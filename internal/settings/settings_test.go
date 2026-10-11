package settings

import "testing"

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
