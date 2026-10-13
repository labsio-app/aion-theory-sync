// Package crashlog turns an otherwise-silent crash in the GUI build into a
// file a member can actually hand over. The GUI ships with -H=windowsgui
// (cmd/gui, .goreleaser.yml) specifically to suppress the console window —
// which also means a panic's stack trace, normally written to stderr, goes
// nowhere a member could ever see it. cmd/gui/main_windows.go recovers any
// panic that escapes its goroutines and writes it here instead.
package crashlog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Dir returns %LOCALAPPDATA%\aion-theory-sync\logs, or a working-directory
// fallback when LOCALAPPDATA is unset — the same convention internal/auth's
// ConfigDir uses for %APPDATA%.
func Dir() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "aion-theory-sync", "logs")
	}
	return filepath.Join(".aion-theory-sync", "logs")
}

// Write records a recovered panic — its value and a stack trace such as
// debug.Stack() returns — to a new, timestamped file under dir, and returns
// that file's path so the caller can point the member at it.
func Write(dir string, recovered any, stack []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	path := filepath.Join(dir, "crash-"+time.Now().Format("20060102-150405.000")+".log")
	content := fmt.Sprintf("%s\npanic: %v\n\n%s", time.Now().Format(time.RFC3339), recovered, stack)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
