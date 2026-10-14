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

type meterLogID struct {
	ID string `json:"id"`
}

// Run uploads every history file whose meter log id is not already in
// knownIDs. The tool keeps no state of its own (F3/D9): knownIDs comes from
// the server on every run, so a retry or a restore never duplicates
// anything and a file for a character declared later is picked up as soon
// as it is re-offered.
func Run(ctx context.Context, client *api.Client, credential, historyDir string, out io.Writer) (Summary, error) {
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

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(out, "%s: could not read (%v)\n", filepath.Base(file), err)
			summary.Failed++
			continue
		}

		var parsed meterLogID
		if err := json.Unmarshal(raw, &parsed); err == nil && parsed.ID != "" && knownIDs[parsed.ID] {
			summary.AlreadyRecorded++
			continue
		}

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

	return summary, nil
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
