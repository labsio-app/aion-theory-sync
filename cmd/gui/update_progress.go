package main

import (
	"fmt"
	"time"
)

// downloadProgress is the state cmd/gui tracks while a self-update
// download is running — see app.onUpdateClicked (app.go, Windows-only)
// and the banner it drives (screens.go). Kept in its own file with no
// walk import and no build tag so its formatting is unit-testable on any
// platform, the same reason autosync_logic.go stays untagged.
type downloadProgress struct {
	Downloaded, Total int64
	StartedAt         time.Time
}

// percent is -1 when Total is unknown (<= 0 — update.Checker.Download's
// own doc comment: the server didn't report a Content-Length) rather than
// a misleading 0%.
func (p downloadProgress) percent() int {
	if p.Total <= 0 {
		return -1
	}
	return int(p.Downloaded * 100 / p.Total)
}

// label renders the update banner's single line while the download runs:
// "Téléchargement… 42 % · encore environ 12 s", with an ETA extrapolated
// from the transfer rate observed so far. Falls back to a plain
// "Téléchargement en cours…" when the total size, or a rate, isn't known
// yet — never a 0 % or a bogus ETA.
func (p downloadProgress) label(now time.Time) string {
	pct := p.percent()
	if pct < 0 {
		return "Téléchargement en cours…"
	}

	elapsed := now.Sub(p.StartedAt)
	if elapsed <= 0 || p.Downloaded <= 0 {
		return fmt.Sprintf("Téléchargement… %d %%", pct)
	}

	rate := float64(p.Downloaded) / elapsed.Seconds()
	remaining := float64(p.Total-p.Downloaded) / rate
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("Téléchargement… %d %% · encore environ %s", pct, etaLabel(int(remaining)))
}

func etaLabel(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d s", seconds)
	}
	return fmt.Sprintf("%d min", seconds/60)
}
