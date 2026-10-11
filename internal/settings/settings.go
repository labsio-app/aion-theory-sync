// Package settings owns the member's own preferences for this tool — today,
// only which folder to read combat logs from — kept separate from
// internal/auth's credential so a corrupt or cleared preference never costs
// the device's link (single responsibility, same config directory).
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
