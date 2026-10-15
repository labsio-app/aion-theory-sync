package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldTriggerAutoSync(t *testing.T) {
	now := time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC)
	cases := []struct {
		name          string
		newestModTime time.Time
		lastTriggered time.Time
		want          bool
	}{
		{"no history files at all", time.Time{}, time.Time{}, false},
		{"first file ever, already quiet long enough", now.Add(-15 * time.Second), time.Time{}, true},
		{"first file ever, still being written", now.Add(-2 * time.Second), time.Time{}, false},
		{"nothing newer than the last trigger", now.Add(-20 * time.Second), now.Add(-20 * time.Second), false},
		{"a newer file landed but debounce hasn't elapsed", now.Add(-5 * time.Second), now.Add(-30 * time.Second), false},
		{"a newer file landed and has been quiet for the full debounce", now.Add(-10 * time.Second), now.Add(-40 * time.Second), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldTriggerAutoSync(c.newestModTime, c.lastTriggered, now, 10*time.Second); got != c.want {
				t.Fatalf("shouldTriggerAutoSync = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNewestHistoryFileModTime(t *testing.T) {
	dir := t.TempDir()

	if got := newestHistoryFileModTime(dir); !got.IsZero() {
		t.Fatalf("empty dir: got %v, want the zero time", got)
	}

	older := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	writeFileAt(t, filepath.Join(dir, "a.json"), older)
	writeFileAt(t, filepath.Join(dir, "b.json"), newer)
	// Not a *.json file: must not count even though it's the newest file
	// on disk.
	writeFileAt(t, filepath.Join(dir, "c.txt"), newer.Add(time.Hour))

	got := newestHistoryFileModTime(dir)
	if !got.Equal(newer) {
		t.Fatalf("newestHistoryFileModTime = %v, want %v", got, newer)
	}
}

func writeFileAt(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("setting mtime on %s: %v", path, err)
	}
}
