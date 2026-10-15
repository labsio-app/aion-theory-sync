package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestOpenDevice_returnsTheGrantTheServerIssued(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/sync/device" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(DeviceGrant{
			GrantID: "grant-1", UserCode: "ABCD-1234",
			VerificationURL: "https://aion-theory.example/activate",
			ExpiresAt:       "2026-10-11T12:15:00.000Z",
		})
	}))
	defer server.Close()

	grant, err := New(server.URL).OpenDevice(context.Background())
	if err != nil {
		t.Fatalf("OpenDevice: %v", err)
	}
	if grant.UserCode != "ABCD-1234" {
		t.Fatalf("UserCode = %q, want ABCD-1234", grant.UserCode)
	}
}

func TestPollToken_mapsTerminalStatusesToDistinctErrors(t *testing.T) {
	cases := []struct {
		status  int
		wantErr error
	}{
		{http.StatusGone, ErrExpired},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrAlreadyUsed},
	}
	for _, c := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
		}))
		_, err := New(server.URL).PollToken(context.Background(), "grant-1")
		server.Close()
		if err != c.wantErr {
			t.Fatalf("status %d: err = %v, want %v", c.status, err, c.wantErr)
		}
	}
}

func TestUploadCombatLog_returnsTheServerOutcome(t *testing.T) {
	var gotAuth, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("authorization")
		gotContentType = r.Header.Get("content-type")
		json.NewEncoder(w).Encode(SyncOutcome{Kind: "RECORDED", CombatLogID: "log-1"})
	}))
	defer server.Close()

	outcome, err := New(server.URL).UploadCombatLog(context.Background(), "sync-credential", []byte("gzip-bytes"))
	if err != nil {
		t.Fatalf("UploadCombatLog: %v", err)
	}
	if outcome.Kind != "RECORDED" {
		t.Fatalf("Kind = %q, want RECORDED", outcome.Kind)
	}
	if gotAuth != "Bearer sync-credential" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
	if gotContentType != "application/gzip" {
		t.Fatalf("content-type header = %q", gotContentType)
	}
}

func TestUploadCombatLog_401And413BecomeNamedErrors(t *testing.T) {
	cases := []struct {
		status  int
		wantErr error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusRequestEntityTooLarge, ErrTooLarge},
	}
	for _, c := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
		}))
		_, err := New(server.URL).UploadCombatLog(context.Background(), "x", []byte("y"))
		server.Close()
		if err != c.wantErr {
			t.Fatalf("status %d: err = %v, want %v", c.status, err, c.wantErr)
		}
	}
}

func TestUploadCombatLog_429BecomesARateLimitedErrorCarryingRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := New(server.URL).UploadCombatLog(context.Background(), "x", []byte("y"))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want it to wrap ErrRateLimited", err)
	}
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want a *RateLimitedError", err)
	}
	if rl.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", rl.RetryAfter)
	}
}

func TestUploadCombatLog_429WithNoRetryAfterHeaderLeavesItZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := New(server.URL).UploadCombatLog(context.Background(), "x", []byte("y"))
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want a *RateLimitedError", err)
	}
	if rl.RetryAfter != 0 {
		t.Fatalf("RetryAfter = %v, want 0", rl.RetryAfter)
	}
}

func TestRecordedMeterLogIDs_returnsASetForFastLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {"auto_1_1", "auto_2_2"}})
	}))
	defer server.Close()

	ids, err := New(server.URL).RecordedMeterLogIDs(context.Background(), "credential")
	if err != nil {
		t.Fatalf("RecordedMeterLogIDs: %v", err)
	}
	if !ids["auto_1_1"] || !ids["auto_2_2"] || ids["auto_3_3"] {
		t.Fatalf("ids = %v", ids)
	}
}

func TestCharacters_decodesEveryField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"characters":[`+
			`{"characterId":"c1","characterName":"Yume","serverName":"Siel"},`+
			`{"characterId":"c2","characterName":"Lira","serverName":null}`+
			`]}`)
	}))
	defer server.Close()

	characters, err := New(server.URL).Characters(context.Background(), "credential")
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	want := []Character{
		{CharacterID: "c1", CharacterName: "Yume", ServerName: "Siel"},
		{CharacterID: "c2", CharacterName: "Lira", ServerName: ""},
	}
	if !reflect.DeepEqual(characters, want) {
		t.Fatalf("characters = %+v, want %+v", characters, want)
	}
}

func TestCharacters_404BecomesErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := New(server.URL).Characters(context.Background(), "credential")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
