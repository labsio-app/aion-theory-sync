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
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
)

// TestMain shrinks the real inter-upload pacing to near-zero for the whole
// package's test run — every test here uploads against an in-process
// httptest server, so there is nothing for basePause to protect against,
// only real wall-clock time to waste.
func TestMain(m *testing.M) {
	basePause, maxPause = time.Millisecond, 5*time.Millisecond
	os.Exit(m.Run())
}

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

func TestRun_aRateLimitOnOneFileDoesNotStopTheRestOfTheBatch(t *testing.T) {
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
				// A bulk first sync is hundreds of uploads back to back —
				// exactly the kind of burst that trips a server-side
				// rate limit.
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
		}
	}))
	defer server.Close()

	var out strings.Builder
	summary, err := Run(context.Background(), api.New(server.URL), "credential", historyDir, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("Failed = %d, want 1 (the rate-limited file)", summary.Failed)
	}
	if summary.Recorded != 1 {
		t.Fatalf("Recorded = %d, want 1: the rest of the batch should still go through", summary.Recorded)
	}
	if !strings.Contains(out.String(), "rate limited") {
		t.Fatalf("output = %q, want it to mention the rate limit", out.String())
	}
}

func TestRunWithProgress_reportsTotalDoneAndTheCurrentFile(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")
	copyFixtureInto(t, historyDir, "combat-sample-2.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/combat-logs/ids":
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {}})
		case r.URL.Path == "/api/sync/combat-logs":
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
		}
	}))
	defer server.Close()

	var events []ProgressEvent
	var out strings.Builder
	summary, err := RunWithProgress(context.Background(), api.New(server.URL), "credential", historyDir, &out,
		func(e ProgressEvent) { events = append(events, e) })
	if err != nil {
		t.Fatalf("RunWithProgress: %v", err)
	}
	if summary.Recorded != 2 {
		t.Fatalf("Recorded = %d, want 2", summary.Recorded)
	}

	// One event per file (Total=2, Done=0 then Done=1), plus a final event
	// at Done==Total.
	if len(events) != 3 {
		t.Fatalf("got %d progress events, want 3: %+v", len(events), events)
	}
	for _, e := range events[:2] {
		if e.Total != 2 {
			t.Fatalf("event %+v: Total = %d, want 2", e, e.Total)
		}
	}
	if events[0].Done != 0 || events[1].Done != 1 {
		t.Fatalf("Done sequence = %d, %d, want 0, 1", events[0].Done, events[1].Done)
	}
	// filepath.Glob sorts lexically ("combat-sample-2.json" before
	// "combat-sample.json"), so check both fixtures' durations showed up
	// rather than assuming which file is events[0].
	gotDurations := map[time.Duration]bool{events[0].Current.Duration: true, events[1].Current.Duration: true}
	if events[0].Current.StartedAt.IsZero() || events[1].Current.StartedAt.IsZero() {
		t.Fatal("a file event's Current.StartedAt is zero, want it from the fixture's startTimeMs")
	}
	if !gotDurations[42*time.Second] || !gotDurations[30*time.Second] {
		t.Fatalf("Current.Duration across the two file events = %v, want 42s and 30s", gotDurations)
	}
	last := events[2]
	if last.Done != 2 || last.Total != 2 {
		t.Fatalf("final event = %+v, want Done=Total=2", last)
	}
}

func TestRunWithProgress_reportsARateLimitedEvent(t *testing.T) {
	// TestMain shrinks maxPause to 5ms for the whole package's test run,
	// which would otherwise clamp away the point of this test (that the
	// server's own Retry-After, not basePause*2, drives the reported
	// pause): give it room back just for this test.
	originalMaxPause := maxPause
	maxPause = 10 * time.Second
	t.Cleanup(func() { maxPause = originalMaxPause })

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
				w.Header().Set("Retry-After", "3")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			json.NewEncoder(w).Encode(api.SyncOutcome{Kind: "RECORDED"})
		}
	}))
	defer server.Close()

	var rateLimitedEvents []ProgressEvent
	var out strings.Builder
	_, err := RunWithProgress(context.Background(), api.New(server.URL), "credential", historyDir, &out,
		func(e ProgressEvent) {
			if e.RateLimited > 0 {
				rateLimitedEvents = append(rateLimitedEvents, e)
			}
		})
	if err != nil {
		t.Fatalf("RunWithProgress: %v", err)
	}
	if len(rateLimitedEvents) != 1 {
		t.Fatalf("got %d rate-limited events, want 1: %+v", len(rateLimitedEvents), rateLimitedEvents)
	}
	// The server's own Retry-After (3s) must win over basePause*2, which
	// TestMain shrinks to 2ms for this whole package's test run.
	if rateLimitedEvents[0].RateLimited != 3*time.Second {
		t.Fatalf("RateLimited = %v, want 3s (the server's Retry-After)", rateLimitedEvents[0].RateLimited)
	}
}

func TestPendingCombats_countsFilesTheServerDoesNotHaveYet(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")
	copyFixtureInto(t, historyDir, "combat-sample-2.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sync/combat-logs/ids" {
			json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {"auto_90001_1700000000000"}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	pending, err := PendingCombats(context.Background(), api.New(server.URL), "credential", historyDir)
	if err != nil {
		t.Fatalf("PendingCombats: %v", err)
	}
	if pending != 1 {
		t.Fatalf("PendingCombats = %d, want 1 (the one fixture not already known)", pending)
	}
}

func TestPendingCombats_isZeroWhenTheServerAlreadyHasEverything(t *testing.T) {
	historyDir := t.TempDir()
	copyFixtureInto(t, historyDir, "combat-sample.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string][]string{"meterLogIds": {"auto_90001_1700000000000"}})
	}))
	defer server.Close()

	pending, err := PendingCombats(context.Background(), api.New(server.URL), "credential", historyDir)
	if err != nil {
		t.Fatalf("PendingCombats: %v", err)
	}
	if pending != 0 {
		t.Fatalf("PendingCombats = %d, want 0", pending)
	}
}
