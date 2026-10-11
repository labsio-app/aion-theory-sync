// Package auth owns the device authorization flow and the credential it
// yields: requesting one, waiting for member approval, and keeping it on
// disk between runs (T1, F16).
package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type config struct {
	// Credential is protected at rest by Protect/Unprotect (DPAPI on
	// Windows, plain on other platforms — the tool only ships for
	// windows/amd64, T3).
	Credential []byte `json:"credential"`
}

// ConfigDir returns %APPDATA%\aion-theory-sync (T1), or a working-directory
// fallback when APPDATA is unset, so the tool still runs for local testing
// off Windows.
func ConfigDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "aion-theory-sync")
	}
	return ".aion-theory-sync"
}

func configPath(dir string) string {
	return filepath.Join(dir, "config.json")
}

// LoadCredential returns the stored credential, or "" if none is stored yet.
func LoadCredential(dir string) (string, error) {
	raw, err := os.ReadFile(configPath(dir))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", err
	}
	if len(cfg.Credential) == 0 {
		return "", nil
	}
	plain, err := Unprotect(cfg.Credential)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// SaveCredential persists the credential issued by the device flow.
func SaveCredential(dir, credential string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	protected, err := Protect([]byte(credential))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(config{Credential: protected})
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(dir), raw, 0o600)
}

// ClearCredential removes a credential the server rejected (401), so the
// next run starts a fresh device flow instead of retrying it forever.
func ClearCredential(dir string) error {
	err := os.Remove(configPath(dir))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
