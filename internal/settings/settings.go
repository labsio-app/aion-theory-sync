// Package settings owns the member's own preferences for this tool — which
// folder to read combat logs from, and when the last sync completed — kept
// separate from internal/auth's credential so a corrupt or cleared
// preference never costs the device's link (single responsibility, same
// config directory).
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings is the member's saved preference, chosen once in the GUI and
// reused on every later launch until they pick a different folder.
type Settings struct {
	HistoryDir string `json:"historyDir"`

	// HistoryDirIsCustom is false while HistoryDir is still whatever
	// sync.HistoryDir() auto-detected, true once the member has picked a
	// folder themselves (screen 2's "Choisir le dossier", screen 8's
	// "Modifier") — the settings screen shows "détecté automatiquement" or
	// "personnalisé" from this rather than comparing paths on every launch.
	HistoryDirIsCustom bool `json:"historyDirIsCustom,omitempty"`

	// LastSyncedAt is RFC3339, set after a sync completes successfully —
	// the GUI's onboarding status reads it to show "Dernière synchro : …"
	// instead of re-deriving it from the uploaded combat logs themselves.
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`

	// LaunchAtStartup and StartMinimized drive internal/autostart's Run
	// registry key; AutoSync drives the history-folder watcher. All three
	// are plain saved preferences (Load returns Go's zero value, false,
	// for a settings.json predating these fields, or for one that simply
	// never set them) - the UX spec's "activé par défaut" for a fresh
	// onboarding is cmd/gui's job to apply once, when it first saves
	// Settings after screen 2, not something Load should guess at every
	// read of an existing file.
	LaunchAtStartup bool `json:"launchAtStartup,omitempty"`
	StartMinimized  bool `json:"startMinimized,omitempty"`
	AutoSync        bool `json:"autoSync,omitempty"`
}

// OnboardingDefaults is the toggle state screen 2 shows before the member
// has touched anything, for a fresh link: launch at startup and auto-sync
// on, start-minimized off (the UX spec's "activé par défaut" / "désactivé
// par défaut"). cmd/gui applies this once, when saving Settings right
// after a successful link — Load itself never assumes it (see the
// Settings doc comment above).
func OnboardingDefaults() Settings {
	return Settings{LaunchAtStartup: true, AutoSync: true}
}

func settingsPath(dir string) string {
	return filepath.Join(dir, "settings.json")
}

// Load returns the saved settings, or the zero value if none were saved yet.
func Load(dir string) (Settings, error) {
	raw, err := os.ReadFile(settingsPath(dir))
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save persists the member's choice of combat-log history folder.
func Save(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(dir), raw, 0o600)
}
