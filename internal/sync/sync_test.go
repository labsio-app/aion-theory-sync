package sync

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labsio-app/aion-theory-sync/internal/api"
)

func copyFixtureInto(t *testing.T, dir, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", name, err)
	}
}

func TestRun_uploadsAFileNotYetKnownToTheServer(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")

	var uploadedGzip []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/sync/combat-logs":
			uploadedGzip, _ = io.ReadAll(r.Body)
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED", CombatLogID: "log-1"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var out strings.Builder
	summary, err := Run(context.Background(), api.New(server.URL), "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Recorded != 1 {
		t.Fatalf("Recorded = %d, want 1", summary.Recorded)
	}

	reader, err := gzip.NewReader(strings.NewReader(string(uploadedGzip)))
	if err != nil {
		t.Fatalf("the upload body was not valid gzip: %v", err)
	}
	inflated, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("inflating the upload body: %v", err)
	}
	if !strings.Contains(string(inflated), "auto_90001_1700000000000") {
		t.Fatalf("inflated body missing the combat's id: %s", inflated)
	}
}

func TestRun_skipsAFileWhoseIdTheServerAlreadyHas(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")

	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {"auto_90001_1700000000000"}})
		case r.URL.Path == "/api/sync/combat-logs":
			uploadCalled = true
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
		}
	}))
	defer server.Close()

	var out strings.Builder
	summary, err := Run(context.Background(), api.New(server.URL), "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if uploadCalled {
		t.Fatal("a known meter log id was re-uploaded (F3/D9 violated)")
	}
	if summary.AlreadyRecorded != 1 {
		t.Fatalf("AlreadyRecorded = %d, want 1", summary.AlreadyRecorded)
	}
}

func TestRun_aRetryAfterAFailureNeverDuplicatesAnUpload(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {}})
		case r.URL.Path == "/api/sync/combat-logs":
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// The server is idempotent on (member, meter log id): a
			// second attempt for the same file still ends RECORDED,
			// never a duplicate.
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
		}
	}))
	defer server.Close()

	client := api.New(server.URL)
	var out strings.Builder

	first, err := Run(context.Background(), client, "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.Failed != 1 || first.Recorded != 0 {
		t.Fatalf("first attempt = %+v, want one failure and no recording", first)
	}

	second, err := Run(context.Background(), client, "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.Recorded != 1 {
		t.Fatalf("second attempt = %+v, want the retry to record it", second)
	}
}

func TestRun_reportsSkippedAndRefusedReasonsInTheSummary(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {}})
		case r.URL.Path == "/api/sync/combat-logs":
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "SKIPPED", Reason: "NO_MATCHING_CHARACTER"})
		}
	}))
	defer server.Close()

	var out strings.Builder
	summary, err := Run(context.Background(), api.New(server.URL), "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Skipped["NO_MATCHING_CHARACTER"] != 1 {
		t.Fatalf("Skipped = %v, want NO_MATCHING_CHARACTER: 1", summary.Skipped)
	}
	// The code is kept inside Summary (programmatic use), but the printed
	// text is the human sentence a lay player reads (F19) — not the raw
	// server enum.
	if !strings.Contains(summary.String(), "no declared character matched") {
		t.Fatalf("summary text is not the human sentence: %q", summary.String())
	}
	if strings.Contains(summary.String(), "NO_MATCHING_CHARACTER") {
		t.Fatalf("summary text leaked the raw code: %q", summary.String())
	}
}

func TestRun_stopsTheBatchOnARevokedCredentialInsteadOfFailingEveryRemainingFile(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")
	copyFixtureInto(t, historyDir, "combat-sample-2.json")

	uploads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {}})
		case r.URL.Path == "/api/sync/combat-logs":
			uploads++
			if uploads == 1 {
				json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
				return
			}
			// The device was revoked partway through the batch (F17).
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	var out strings.Builder
	summary, err := Run(context.Background(), api.New(server.URL), "credential", historyDir, &out)

	if !errors.Is(err, api.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if summary.Recorded != 1 {
		t.Fatalf("Recorded = %d, want 1 (the file uploaded before the credential was revoked)", summary.Recorded)
	}
	if summary.Failed != 0 {
		t.Fatalf("Failed = %d, want 0: a revoked credential stops the batch, it is not a per-file failure", summary.Failed)
	}
}
