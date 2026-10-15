package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_returnsTheZeroValueWhenNothingWasSavedYet(t *testing.T) {
	dir := t.TempDir()

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.HistoryDir != "" {
		t.Fatalf("HistoryDir = %q, want empty", s.HistoryDir)
	}
}

func TestSaveThenLoad_roundTripsTheChosenFolder(t *testing.T) {
	dir := t.TempDir()

	if err := Save(dir, Settings{HistoryDir: `C:\Users\member\AppData\Roaming\com.a2tools.dps-meter\history`}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.HistoryDir != `C:\Users\member\AppData\Roaming\com.a2tools.dps-meter\history` {
		t.Fatalf("HistoryDir = %q, want the saved folder", s.HistoryDir)
	}
}

func TestSaveThenLoad_roundTripsLastSyncedAt(t *testing.T) {
	dir := t.TempDir()

	if err := Save(dir, Settings{HistoryDir: "history", LastSyncedAt: "2026-10-13T20:45:00Z"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.LastSyncedAt != "2026-10-13T20:45:00Z" {
		t.Fatalf("LastSyncedAt = %q, want the saved timestamp", s.LastSyncedAt)
	}
}

func TestSave_overwritesAPreviouslySavedFolder(t *testing.T) {
	dir := t.TempDir()

	if err := Save(dir, Settings{HistoryDir: "first"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Save(dir, Settings{HistoryDir: "second"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.HistoryDir != "second" {
		t.Fatalf("HistoryDir = %q, want second", s.HistoryDir)
	}
}

func TestLoad_anOldSettingsFileWithoutTheNewFieldsStillLoads(t *testing.T) {
	dir := t.TempDir()
	old := `{"historyDir":"C:\\history","lastSyncedAt":"2026-10-13T20:45:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(old), 0o600); err != nil {
		t.Fatalf("writing the old-shaped file: %v", err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.HistoryDir != `C:\history` || s.LastSyncedAt != "2026-10-13T20:45:00Z" {
		t.Fatalf("s = %+v, want the pre-existing fields preserved", s)
	}
	if s.LaunchAtStartup || s.StartMinimized || s.AutoSync || s.HistoryDirIsCustom {
		t.Fatalf("s = %+v, want every new field to default to false for an old file", s)
	}
}

func TestSaveThenLoad_roundTripsTheNewPreferenceFields(t *testing.T) {
	dir := t.TempDir()
	saved := Settings{
		HistoryDir:         "history",
		HistoryDirIsCustom: true,
		LaunchAtStartup:    true,
		StartMinimized:     false,
		AutoSync:           true,
	}
	if err := Save(dir, saved); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != saved {
		t.Fatalf("loaded = %+v, want %+v", loaded, saved)
	}
}

func TestOnboardingDefaults(t *testing.T) {
	d := OnboardingDefaults()
	if !d.LaunchAtStartup {
		t.Fatal("LaunchAtStartup = false, want true (enabled by default)")
	}
	if !d.AutoSync {
		t.Fatal("AutoSync = false, want true (enabled by default)")
	}
	if d.StartMinimized {
		t.Fatal("StartMinimized = true, want false (disabled by default)")
	}
}
