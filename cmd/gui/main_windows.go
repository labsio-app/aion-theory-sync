//go:build windows

// Command aion-theory-sync-gui is the installable desktop app: pick the
// A2Tools history folder once, link the device, and sync with one click. It
// wraps the exact same internal/auth, internal/api and internal/sync
// packages the console tool (../../main.go) already uses and already
// tests — nothing about the sync contract changes here, only how a member
// drives it.
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
	. "github.com/tailscale/walk/declarative"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/auth"
	"github.com/labsio-app/aion-theory-sync/internal/crashlog"
	"github.com/labsio-app/aion-theory-sync/internal/settings"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
	"github.com/labsio-app/aion-theory-sync/internal/update"
	"github.com/labsio-app/aion-theory-sync/internal/version"
)

const defaultServerURL = "https://aion-theory.labsio.app"

// runTimeout bounds one connect-and-sync attempt: the device code itself
// expires after 15 minutes (internal/auth), the extra time covers the
// upload pass that follows approval.
const runTimeout = 20 * time.Minute

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

	runGUI()
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
// goroutine this package spawns (see runGUI) — not just once.
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

func runGUI() {
	configDir := auth.ConfigDir()
	saved, _ := settings.Load(configDir)
	historyDir := saved.HistoryDir
	if historyDir == "" {
		historyDir = sync.HistoryDir()
	}

	var mw *walk.MainWindow
	var folderEdit *walk.LineEdit
	var statusLabel *walk.Label
	var logBox *walk.TextEdit
	var actionButton *walk.PushButton

	// Every background goroutine below touches widgets only through these
	// three functions, each hopping back onto the UI thread via
	// Synchronize — walk widgets are not safe to touch from any other
	// goroutine.
	appendLog := func(line string) {
		walk.App().Synchronize(func() { logBox.AppendText(line + "\r\n") })
	}
	setStatus := func(text string) {
		walk.App().Synchronize(func() { statusLabel.SetText(text) })
	}
	setBusy := func(busy bool) {
		walk.App().Synchronize(func() {
			actionButton.SetEnabled(!busy)
			folderEdit.SetEnabled(!busy)
		})
	}

	connectAndSync := func() {
		dir := strings.TrimSpace(folderEdit.Text())
		if dir == "" {
			setStatus("Choisissez d'abord le dossier des journaux de combat.")
			return
		}
		if err := settings.Save(configDir, settings.Settings{HistoryDir: dir}); err != nil {
			appendLog("Le dossier choisi n'a pas pu être enregistré : " + err.Error())
		}

		setBusy(true)
		defer setBusy(false)

		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()

		client := api.New(defaultServerURL)
		out := &guiWriter{appendLog}

		setStatus("Connexion…")
		credential, err := auth.EnsureCredentialWithGrant(ctx, client, configDir, out, func(grant api.DeviceGrant) {
			setStatus("Ouvrez le lien et entrez ce code : " + grant.UserCode)
			openBrowser(grant.VerificationURL)
		})
		if err != nil {
			setStatus("Échec de la connexion.")
			appendLog("Erreur : " + err.Error())
			return
		}

		setStatus("Synchronisation des combats…")
		summary, err := sync.Run(ctx, client, credential, dir, out)
		if err != nil {
			setStatus("Échec de la synchronisation.")
			appendLog("Erreur : " + err.Error())
			return
		}
		setStatus("Synchronisé.")
		appendLog(summary.String())
	}

	// Both best-effort and read from the install directory (the installer
	// places assets/ next to the exe): a missing icon or logo leaves the
	// title bar or header blank rather than failing the whole window. The
	// .exe itself also carries this same icon as a Windows resource
	// (versioninfo.json, embedded at build time), for Explorer and the
	// taskbar before the window ever opens.
	icon, _ := walk.NewIconFromFile(assetPath("icon.ico"))
	logo, _ := walk.NewImageFromFile(assetPath("logo.png"))

	err := MainWindow{
		AssignTo: &mw,
		Title:    "Aion Theory Sync",
		Icon:     icon,
		MinSize:  Size{Width: 440, Height: 380},
		Layout:   VBox{},
		Children: []Widget{
			ImageView{Image: logo, MinSize: Size{Height: 96}, Mode: ImageViewModeShrink},
			Label{Text: "Dossier des journaux de combat (A2Tools) :"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{AssignTo: &folderEdit, Text: historyDir},
					PushButton{Text: "Parcourir…", OnClicked: func() {
						dlg := &walk.FileDialog{Title: "Dossier des journaux de combat", InitialDirPath: folderEdit.Text()}
						if ok, _ := dlg.ShowBrowseFolder(mw); ok {
							folderEdit.SetText(dlg.FilePath)
						}
					}},
				},
			},
			VSpacer{Size: 8},
			PushButton{
				AssignTo: &actionButton,
				Text:     "Se connecter et synchroniser",
				OnClicked: func() {
					go func() {
						defer recoverAndReport()
						connectAndSync()
					}()
				},
			},
			Label{AssignTo: &statusLabel, Text: "Prêt."},
			TextEdit{AssignTo: &logBox, ReadOnly: true, VScroll: true, MinSize: Size{Height: 140}},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "Aion Theory Sync", "La fenêtre n'a pas pu s'ouvrir : "+err.Error(), walk.MsgBoxIconError)
		return
	}
	mw.SetExitOnClose(true)
	mw.Show()
	go func() {
		defer recoverAndReport()
		checkForUpdate(mw, appendLog, setStatus)
	}()
	walk.App().Run()
}

// checkForUpdate runs once per launch, in the background so it never
// delays the window opening. version.Version is "dev" for any build that
// skipped -ldflags (ci.yml, a local `go build`), which update.Check
// always treats as "nothing to check" — so this is a no-op outside a
// release build installed via Setup.exe or the raw goreleaser binary.
func checkForUpdate(mw *walk.MainWindow, appendLog func(string), setStatus func(string)) {
	// A leftover from a previous update (see update.Apply) only ever
	// blocks a future update's own rename dance, never today's launch,
	// so cleaning it up here (rather than failing anything) is enough.
	update.RemoveOldVersion()

	checkCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	checker := update.New()
	available, err := checker.Check(checkCtx, version.Version)
	if err != nil {
		appendLog("Vérification des mises à jour : " + err.Error())
		return
	}
	if available == nil {
		return
	}

	var install bool
	walk.App().Synchronize(func() {
		msg := fmt.Sprintf("Une nouvelle version (%s) est disponible. L'installer maintenant ?", available.Version)
		install = walk.MsgBox(mw, "Mise à jour disponible", msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes
	})
	if !install {
		return
	}

	setStatus("Téléchargement de la mise à jour…")
	// Downloaded exe is typically only a few MB, but a generous timeout
	// costs nothing on a fast connection and avoids failing a slow one.
	downloadCtx, cancelDownload := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelDownload()

	newExePath, err := checker.Download(downloadCtx, available.DownloadURL)
	if err != nil {
		setStatus("Échec du téléchargement de la mise à jour.")
		appendLog("Erreur : " + err.Error())
		return
	}
	if err := update.Apply(newExePath); err != nil {
		setStatus("Échec de l'installation de la mise à jour.")
		appendLog("Erreur : " + err.Error())
		return
	}
	if err := update.Relaunch(); err != nil {
		setStatus("Mise à jour installée, mais le redémarrage a échoué.")
		appendLog("Relancez l'application manuellement. Erreur : " + err.Error())
		return
	}
	os.Exit(0)
}

// guiWriter adapts the plain-text progress lines internal/auth and
// internal/sync already write to an io.Writer (and already test against)
// onto the log box, one line at a time instead of one io.Writer.Write call
// that may bundle several.
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
