package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
)

func TestRunDeviceFlow_waitsForApprovalThenSavesTheCredential(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sync/device":
			json.NewEncoder(w).Encode(map[string]string{
				"grantId":         "grant-1",
				"userCode":        "ABCD-1234",
				"verificationUrl": "https://aion-theory.example/activate",
				"expiresAt":       "2026-10-11T12:15:00.000Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/sync/token":
			if polls.Add(1) < 2 {
				json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "approved",
				"credential": "sync-credential-xyz",
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	client := api.New(server.URL)
	withFastPolling(t)

	var out strings.Builder
	credential, err := RunDeviceFlow(context.Background(), client, dir, &out)
	if err != nil {
		t.Fatalf("RunDeviceFlow: %v", err)
	}
	if credential != "sync-credential-xyz" {
		t.Fatalf("credential = %q, want sync-credential-xyz", credential)
	}
	if !strings.Contains(out.String(), "ABCD-1234") {
		t.Fatalf("output does not show the user code: %q", out.String())
	}

	stored, err := LoadCredential(dir)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if stored != credential {
		t.Fatalf("stored credential = %q, want %q", stored, credential)
	}
}

func TestEnsureCredential_reusesAStoredCredentialWithoutADeviceFlow(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCredential(dir, "already-linked"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no device-flow request expected, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := api.New(server.URL)
	var out strings.Builder
	credential, err := EnsureCredential(context.Background(), client, dir, &out)
	if err != nil {
		t.Fatalf("EnsureCredential: %v", err)
	}
	if credential != "already-linked" {
		t.Fatalf("credential = %q, want already-linked", credential)
	}
}

func TestClearCredential_removesWhatWasStoredSoTheNextRunRelinks(t *testing.T) {
	dir := t.TempDir()
	if err := SaveCredential(dir, "to-be-revoked"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	if err := ClearCredential(dir); err != nil {
		t.Fatalf("ClearCredential: %v", err)
	}
	stored, err := LoadCredential(dir)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if stored != "" {
		t.Fatalf("stored credential = %q, want empty after ClearCredential", stored)
	}
}

func TestRunDeviceFlow_anExpiredGrantFailsWithAClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/device":
			json.NewEncoder(w).Encode(map[string]string{
				"grantId": "grant-1", "userCode": "ABCD-1234",
				"verificationUrl": "https://aion-theory.example/activate",
			})
		case r.URL.Path == "/api/sync/token":
			w.WriteHeader(http.StatusGone)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	client := api.New(server.URL)
	withFastPolling(t)

	var out strings.Builder
	if _, err := RunDeviceFlow(context.Background(), client, dir, &out); err == nil {
		t.Fatal("expected an error for an expired device grant")
	}
}

func TestRunDeviceFlow_retriesATransientPollErrorInsteadOfAborting(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/device":
			json.NewEncoder(w).Encode(map[string]string{
				"grantId": "grant-1", "userCode": "ABCD-1234",
				"verificationUrl": "https://aion-theory.example/activate",
			})
		case r.URL.Path == "/api/sync/token":
			if polls.Add(1) == 1 {
				// A transient failure (F16): not one of the terminal
				// outcomes PollToken names, so the flow must retry rather
				// than abort.
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "approved",
				"credential": "sync-credential-after-retry",
			})
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	client := api.New(server.URL)
	withFastPolling(t)

	var out strings.Builder
	credential, err := RunDeviceFlow(context.Background(), client, dir, &out)
	if err != nil {
		t.Fatalf("RunDeviceFlow: %v", err)
	}
	if credential != "sync-credential-after-retry" {
		t.Fatalf("credential = %q, want sync-credential-after-retry", credential)
	}
}

// withFastPolling shrinks the poll interval for the duration of the test so
// the suite does not wait on the tool's real-world 4s cadence.
func withFastPolling(t *testing.T) {
	t.Helper()
	original := pollInterval
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = original })
}
