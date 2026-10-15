// Auto-sync's decision logic: no build tag, so it's testable on any
// platform even though only the windows-tagged autosync_windows.go ever
// calls it for real.
package main

import (
	"os"
	"path/filepath"
	"time"
)

// autoSyncPollInterval and autoSyncDebounce match the UX spec's own
// numbers ("polling 30 s", "debounce de 10 s après le dernier fichier").
const (
	autoSyncPollInterval = 30 * time.Second
	autoSyncDebounce     = 10 * time.Second
)

// newestHistoryFileModTime returns the most recent modification time among
// dir's *.json history files, or the zero Time if there are none, or dir
// can't be read.
func newestHistoryFileModTime(dir string) time.Time {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return time.Time{}
	}
	var newest time.Time
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// shouldTriggerAutoSync decides whether the folder watcher should start a
// sync right now: there is a history file newer than the last trigger
// (lastTriggered starts at the zero Time, so the very first file ever
// found always qualifies), and it has stayed unchanged for at least
// debounce — the UX spec's margin for the DPS meter to finish writing it
// before this tool reads it.
func shouldTriggerAutoSync(newestModTime, lastTriggered, now time.Time, debounce time.Duration) bool {
	if newestModTime.IsZero() {
		return false
	}
	if !newestModTime.After(lastTriggered) {
		return false
	}
	return now.Sub(newestModTime) >= debounce
}
