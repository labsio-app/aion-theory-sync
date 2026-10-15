//go:build windows

// Command aion-theory-sync-gui is the installable desktop app: link the
// device once, then let it sync — a guided onboarding flow (app.go,
// screens.go, widgets.go, theme.go), its screens driven entirely by
// internal/ui/state's Machine/ViewModel rather than by business logic
// living in this package. It wraps the exact same internal/auth,
// internal/api and internal/sync packages the console tool (../../main.go)
// already uses and already tests — nothing about the sync contract
// changes here, only how a member drives it.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tailscale/walk"

	"github.com/labsio-app/aion-theory-sync/internal/crashlog"
	"github.com/labsio-app/aion-theory-sync/internal/update"
	"github.com/labsio-app/aion-theory-sync/internal/version"
)

const defaultServerURL = "https://aion-theory.labsio.app"

func main() {
	// walk.InitApp must be the very first walk call (tailscale/walk's own
	// doc comment on InitApp says so): every widget's layout pass reaches
	// walk.App() on a background goroutine the library spawns for it, and
	// walk.App() panics outright — "walk.InitApp must be called first" —
	// until InitApp has run once. Skipping this crashed the window before
	// it ever appeared, on every single launch of v0.2.2 — the "the app
	// doesn't open" report.
	if _, err := walk.InitApp(); err != nil {
		walk.MsgBox(nil, "Aion Theory Sync", "Le moteur graphique n'a pas pu démarrer : "+err.Error(), walk.MsgBoxIconError)
		os.Exit(1)
	}
	defer recoverAndReport()

	startMinimized := len(os.Args) > 1 && os.Args[1] == "--minimized"
	runApp(startMinimized)
}

// recoverAndReport turns a panic that would otherwise vanish without a
// trace into a crash-log file and a message box a member can act on. This
// build is -H=windowsgui (.goreleaser.yml, release.yml), specifically to
// suppress the console window — which also means an unrecovered panic's
// stack trace, normally printed to stderr, goes nowhere anyone could ever
// see it; the process just disappears.
//
// A deferred recover only catches a panic on the same goroutine it runs
// on, so this must be deferred in main() *and* at the top of every
// goroutine this package spawns — not just once.
func recoverAndReport() {
	r := recover()
	if r == nil {
		return
	}
	path, err := crashlog.Write(crashlog.Dir(), r, debug.Stack())
	msg := fmt.Sprintf("Aion Theory Sync a rencontré une erreur inattendue et doit se fermer.\n\nErreur : %v", r)
	if err == nil {
		msg += "\n\nDétails enregistrés dans :\n" + path
	}
	walk.MsgBox(nil, "Aion Theory Sync", msg, walk.MsgBoxIconError)
}

// checkForUpdate runs once per launch, in the background so it never
// delays the window opening. version.Version is "dev" for any build that
// skipped -ldflags (ci.yml, a local `go build`), which update.Check
// always treats as "nothing to check" — so this is a no-op outside a
// release build installed via Setup.exe or the raw goreleaser binary.
//
// It only detects and reports: onAvailable (app.go) shows the banner
// every screen now carries instead of popping a blocking dialog here, so
// a member decides for themselves when to click "Mettre à jour" — never
// the instant this check happens to finish. The actual download/install/
// relaunch lives in app.go's onUpdateClicked, triggered from that banner.
func checkForUpdate(appendLog func(string), onAvailable func(update.Available)) {
	// A leftover from a previous update (see update.Apply) only ever
	// blocks a future update's own rename dance, never today's launch,
	// so cleaning it up here (rather than failing anything) is enough.
	update.RemoveOldVersion()

	checkCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	available, err := update.New().Check(checkCtx, version.Version)
	if err != nil {
		appendLog("Vérification des mises à jour : " + err.Error())
		return
	}
	if available == nil || onAvailable == nil {
		return
	}
	onAvailable(*available)
}

// guiWriter adapts the plain-text progress lines internal/auth and
// internal/sync already write to an io.Writer (and already test against)
// onto the technical log, one line at a time instead of one io.Writer.Write
// call that may bundle several.
type guiWriter struct {
	appendLog func(string)
}

func (w *guiWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			w.appendLog(line)
		}
	}
	return len(p), nil
}

func openBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", "", url).Start()
}

// assetPath resolves a file under the installer's assets/ folder (see
// installer/aion-theory-sync.iss) relative to the running exe's own
// directory, not the current working directory — which Explorer normally
// sets to match, but a custom shortcut or a launch from a pinned taskbar
// entry isn't guaranteed to. Falls back to the plain relative path if the
// exe's own location can't be determined, matching the previous behavior.
func assetPath(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join("assets", name)
	}
	return filepath.Join(filepath.Dir(exe), "assets", name)
}

func exePath() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return exe
}
