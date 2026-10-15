// Package sync scans the A2Tools DPS meter's history folder and uploads
// combat logs the server does not have yet (T2).
package sync

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
)

// basePause and maxPause bound the delay this package waits between
// uploads. basePause is a proactive throttle: a first sync can be hundreds
// of files back to back, more than enough to trip a server-side rate
// limit that an occasional retry run never would. maxPause caps how far a
// run backs off after a 429 (uploadPause grows from basePause toward it,
// see Run) so a long Retry-After on one file can't single-handedly blow
// the caller's context budget across the rest of a large batch.
//
// Both are vars, not consts, so tests can shrink them instead of actually
// sleeping real wall-clock time.
var (
	basePause = 200 * time.Millisecond
	maxPause  = 5 * time.Second
)

// reasonSentences turns the platform's machine codes into the sentence a
// lay player reads in the run's summary (F19) — the server speaks in enum
// codes, this tool is the one place a human reads the output.
var reasonSentences = map[string]string{
	"NO_DECLARED_CHARACTERS": "declare a character on Aion Theory first",
	"NO_MATCHING_CHARACTER":  "no declared character matched an unmasked player in this fight",
	"AMBIGUOUS_MATCH":        "more than one declared character could match this fight",
	"UNREADABLE":             "the file is not a readable combat log",
	"MISSING_LOG_ID":         "the file carries no combat id",
	"MISSING_ACTORS":         "the file names no valid player",
	"OVERSIZED":              "the file is larger than the server accepts",
	"CONTENT_MISMATCH":       "already synced under this id with different content",
}

func describeReason(reason string) string {
	if sentence, ok := reasonSentences[reason]; ok {
		return sentence
	}
	return reason
}

// HistoryDir is the A2Tools DPS meter's own log folder — a sibling of this
// tool's config folder, not inside it (T2: "%APPDATA%\com.a2tools.dps-meter\history").
func HistoryDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "com.a2tools.dps-meter", "history")
	}
	return ""
}

type Summary struct {
	Recorded        int
	AlreadyRecorded int
	Skipped         map[string]int
	Refused         map[string]int
	Failed          int
}

func newSummary() Summary {
	return Summary{Skipped: map[string]int{}, Refused: map[string]int{}}
}

func (s Summary) String() string {
	total := s.Recorded + s.AlreadyRecorded + s.Failed
	for _, n := range s.Skipped {
		total += n
	}
	for _, n := range s.Refused {
		total += n
	}
	out := fmt.Sprintf("%d combat log(s) seen: %d recorded, %d already recorded", total, s.Recorded, s.AlreadyRecorded)
	for reason, n := range s.Skipped {
		out += fmt.Sprintf(", %d skipped (%s)", n, describeReason(reason))
	}
	for reason, n := range s.Refused {
		out += fmt.Sprintf(", %d refused (%s)", n, describeReason(reason))
	}
	if s.Failed > 0 {
		out += fmt.Sprintf(", %d failed to upload (retry by running again)", s.Failed)
	}
	return out
}

// meterLogID is the handful of fields Run and PendingCombats read straight
// off a history file's own JSON, without decoding the rest of it (skills,
// actors...): the meter log id to dedupe against the server, plus the two
// fields RunWithProgress's current-file events echo back to the caller.
type meterLogID struct {
	ID          string `json:"id"`
	StartTimeMs int64  `json:"startTimeMs"`
	DurationMs  int64  `json:"durationMs"`
}

// FileProgress is the combat log a running RunWithProgress call is
// currently uploading.
type FileProgress struct {
	StartedAt time.Time
	Duration  time.Duration
}

// ProgressEvent is one step of a RunWithProgress call: either a file about
// to upload (Total/Done/Current set) or a rate limit the server just
// returned (RateLimited set, everything else zero). Total/Done still
// accompany a RateLimited event, so a caller only watching Done/Total
// never has to merge two event shapes to track progress.
type ProgressEvent struct {
	Total       int
	Done        int
	Current     FileProgress
	RateLimited time.Duration // 0 unless this event reports a fresh rate limit
}

// ProgressFunc receives every ProgressEvent a RunWithProgress call emits,
// in order, on the same goroutine that called it.
type ProgressFunc func(ProgressEvent)

// Options configures RunWithOptions. The zero value matches Run's plain
// behavior: no progress callback, never paused.
type Options struct {
	// OnProgress, if set, receives every ProgressEvent as RunWithOptions
	// goes (see ProgressEvent's doc comment).
	OnProgress ProgressFunc

	// Paused, if set, is polled before every file: RunWithOptions waits
	// (checking ctx for cancellation every pausePollInterval) for as long
	// as it reports true. This is screen 4's "Mettre en pause" — a local,
	// cooperative pause the GUI drives, not a server-requested one (that's
	// RateLimited, which never waits on this).
	Paused func() bool
}

// pausePollInterval bounds how long a paused RunWithOptions call can take
// to notice ctx was cancelled. A var, not a const, so tests can shrink it.
var pausePollInterval = 200 * time.Millisecond

// Run uploads every history file whose meter log id is not already in
// knownIDs. The tool keeps no state of its own (F3/D9): knownIDs comes from
// the server on every run, so a retry or a restore never duplicates
// anything and a file for a character declared later is picked up as soon
// as it is re-offered.
func Run(ctx context.Context, client *api.Client, credential, historyDir string, out io.Writer) (Summary, error) {
	return RunWithOptions(ctx, client, credential, historyDir, out, Options{})
}

// RunWithProgress is Run, but also reports structured progress through
// onProgress as it goes — the GUI's syncing screen uses this to drive its
// progress ring, current-file line and rate-limit countdown without
// having to parse out's plain-text lines. onProgress may be nil.
func RunWithProgress(ctx context.Context, client *api.Client, credential, historyDir string, out io.Writer, onProgress ProgressFunc) (Summary, error) {
	return RunWithOptions(ctx, client, credential, historyDir, out, Options{OnProgress: onProgress})
}

// RunWithOptions is Run with every optional behavior (progress reporting,
// pausing) spelled out in opts.
func RunWithOptions(ctx context.Context, client *api.Client, credential, historyDir string, out io.Writer, opts Options) (Summary, error) {
	onProgress := opts.OnProgress
	if onProgress == nil {
		onProgress = func(ProgressEvent) {}
	}

	summary := newSummary()

	knownIDs, err := client.RecordedMeterLogIDs(ctx, credential)
	if err != nil {
		return summary, fmt.Errorf("listing already-recorded combat logs: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		return summary, fmt.Errorf("scanning %s: %w", historyDir, err)
	}

	// Grows past basePause only after a 429 (see below), and never past
	// maxPause — a plain proactive throttle the rest of the time.
	pause := basePause

	for i, file := range files {
		if err := waitWhilePaused(ctx, opts.Paused); err != nil {
			return summary, err
		}

		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(out, "%s: could not read (%v)\n", filepath.Base(file), err)
			summary.Failed++
			continue
		}

		var parsed meterLogID
		json.Unmarshal(raw, &parsed) // best-effort: a parse failure leaves parsed at its zero value
		if parsed.ID != "" && knownIDs[parsed.ID] {
			summary.AlreadyRecorded++
			continue
		}

		onProgress(ProgressEvent{Total: len(files), Done: i, Current: fileProgress(parsed)})

		gzipped, err := gzipBytes(raw)
		if err != nil {
			fmt.Fprintf(out, "%s: could not compress (%v)\n", filepath.Base(file), err)
			summary.Failed++
			continue
		}

		if err := sleepCtx(ctx, pause); err != nil {
			return summary, err
		}

		outcome, err := client.UploadCombatLog(ctx, credential, gzipped)
		if err != nil {
			// A revoked/expired credential mid-batch is not a per-file
			// failure (F17): stop here and let the caller relink, rather
			// than printing one generic "upload failed" line per
			// remaining file.
			if errors.Is(err, api.ErrUnauthorized) {
				return summary, err
			}

			var rl *api.RateLimitedError
			if errors.As(err, &rl) {
				// The server is telling the whole batch to slow down,
				// not just complaining about this one file: grow the
				// pause used for every upload still to come, instead of
				// retrying this file inline and hammering the same limit
				// again immediately. A fresh Run (the existing "retry by
				// running again" path, already relied on for ordinary
				// failures) picks this file back up.
				next := pause * 2
				if rl.RetryAfter > next {
					next = rl.RetryAfter
				}
				if next > maxPause {
					next = maxPause
				}
				pause = next
				onProgress(ProgressEvent{Total: len(files), Done: i, RateLimited: next})
			}

			fmt.Fprintf(out, "%s: upload failed (%v)\n", filepath.Base(file), err)
			summary.Failed++
			continue
		}

		switch outcome.Kind {
		case "RECORDED":
			summary.Recorded++
		case "ALREADY_RECORDED":
			summary.AlreadyRecorded++
		case "SKIPPED":
			summary.Skipped[outcome.Reason]++
		case "REFUSED":
			summary.Refused[outcome.Reason]++
		default:
			fmt.Fprintf(out, "%s: unexpected server outcome %q\n", filepath.Base(file), outcome.Kind)
			summary.Failed++
		}
	}

	onProgress(ProgressEvent{Total: len(files), Done: len(files)})
	return summary, nil
}

// waitWhilePaused blocks for as long as paused reports true, rechecking
// every pausePollInterval so a cancelled ctx is noticed promptly rather
// than only once the member resumes.
func waitWhilePaused(ctx context.Context, paused func() bool) error {
	if paused == nil {
		return nil
	}
	for paused() {
		if err := sleepCtx(ctx, pausePollInterval); err != nil {
			return err
		}
	}
	return nil
}

func fileProgress(parsed meterLogID) FileProgress {
	var fp FileProgress
	if parsed.StartTimeMs > 0 {
		fp.StartedAt = time.UnixMilli(parsed.StartTimeMs)
	}
	fp.Duration = time.Duration(parsed.DurationMs) * time.Millisecond
	return fp
}

// PendingCombats counts the history files Run would attempt to upload
// right now — everything under historyDir whose meter log id the server
// doesn't already have. It makes the same RecordedMeterLogIDs round trip
// Run does, but no upload, so the GUI can show "N combats en attente"
// (screens 2/3/6) before the member ever clicks sync.
func PendingCombats(ctx context.Context, client *api.Client, credential, historyDir string) (int, error) {
	knownIDs, err := client.RecordedMeterLogIDs(ctx, credential)
	if err != nil {
		return 0, fmt.Errorf("listing already-recorded combat logs: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		return 0, fmt.Errorf("scanning %s: %w", historyDir, err)
	}

	pending := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			// Run counts an unreadable file as Failed rather than
			// skipping it silently; PendingCombats treats it the same
			// way, as "something Run will have to deal with".
			pending++
			continue
		}
		var parsed meterLogID
		json.Unmarshal(raw, &parsed)
		if parsed.ID != "" && knownIDs[parsed.ID] {
			continue
		}
		pending++
	}
	return pending, nil
}

// sleepCtx pauses for d, or returns ctx's error early if it's cancelled
// first — so a very long Retry-After can't outlast the caller's own
// timeout (runTimeout in cmd/gui) silently.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
