// Package autostart owns the "launch at Windows startup" preference: one
// value under HKCU\Software\Microsoft\Windows\CurrentVersion\Run, named
// AionTheorySync, holding the quoted path to this exe and, when the member
// also wants it to start minimized, a trailing --minimized.
package autostart

// Autostart is implemented by Windows (windows.go, the real registry
// write) and Fake (below, for tests and for other platforms' build).
type Autostart interface {
	// Enable points the Run key at exePath, appending --minimized to the
	// command line when minimized is true.
	Enable(exePath string, minimized bool) error
	Disable() error
	IsEnabled() (bool, error)
}

// commandLine builds the Run value's data: the exe path, quoted so a path
// containing spaces (Program Files, most installs) still parses as one
// argument, plus --minimized when asked. Kept as its own pure function so
// it's testable without touching the registry, on any platform.
func commandLine(exePath string, minimized bool) string {
	line := `"` + exePath + `"`
	if minimized {
		line += " --minimized"
	}
	return line
}

// Fake is an in-memory Autostart for tests: it records whatever was last
// enabled instead of touching the registry.
type Fake struct {
	Enabled   bool
	ExePath   string
	Minimized bool

	// EnableErr/DisableErr, when set, make the matching method fail
	// instead of recording anything - for exercising a caller's own error
	// handling.
	EnableErr  error
	DisableErr error
}

func (f *Fake) Enable(exePath string, minimized bool) error {
	if f.EnableErr != nil {
		return f.EnableErr
	}
	f.Enabled = true
	f.ExePath = exePath
	f.Minimized = minimized
	return nil
}

func (f *Fake) Disable() error {
	if f.DisableErr != nil {
		return f.DisableErr
	}
	f.Enabled = false
	f.ExePath = ""
	f.Minimized = false
	return nil
}

func (f *Fake) IsEnabled() (bool, error) {
	return f.Enabled, nil
}
