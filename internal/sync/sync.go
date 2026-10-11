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

	"github.com/labsio-app/aion-theory-sync/internal/api"
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

		outcome, err := client.UploadCombatLog(ctx, credential, gzipped)
		if err != nil {
			// A revoked/expired credential mid-batch is not a per-file
			// failure (F17): stop here and let the caller relink, rather
			// than printing one generic "upload failed" line per
			// remaining file.
			if errors.Is(err, api.ErrUnauthorized) {
				return summary, err
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
