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
	"os/exec"
	"strings"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/auth"
	"github.com/labsio-app/aion-theory-sync/internal/settings"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
)

const defaultServerURL = "https://aion-theory.labsio.app"

// runTimeout bounds one connect-and-sync attempt: the device code itself
// expires after 15 minutes (internal/auth), the extra time covers the
// upload pass that follows approval.
const runTimeout = 20 * time.Minute

func main() {
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
	icon, _ := walk.NewIconFromFile("assets/icon.ico")
	logo, _ := walk.NewImageFromFile("assets/logo.png")

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
					go connectAndSync()
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
	walk.App().Run()
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
