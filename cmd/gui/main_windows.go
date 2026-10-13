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

// Brand palette, mirrored from the website's apps/web/app/assets/tokens.css
// (confirmed with the site's own team rather than guessed from a screenshot):
// ground/ground-deep for backgrounds, elyos/asmodian for the two faction
// accents, text/text-soft/muted/faint for the type scale. Kept as package
// vars, not consts: walk.RGB is a function, not a constant expression.
var (
	colorGround      = walk.RGB(0x08, 0x07, 0x1a)
	colorGroundDeep  = walk.RGB(0x04, 0x03, 0x0e)
	colorAccentElyos = walk.RGB(0x6b, 0xd5, 0xff)
	colorText        = walk.RGB(0xf2, 0xee, 0xfa)
	colorTextSoft    = walk.RGB(0xcf, 0xc7, 0xe6)
	colorMuted       = walk.RGB(0x9a, 0x91, 0xb8)
	colorFaint       = walk.RGB(0x6c, 0x64, 0x89)

	// Not part of the brand's two-accent palette (elyos/asmodian carry no
	// success/warning meaning on the site) — conventional done/action-needed
	// colors, chosen only to read clearly against colorGround.
	colorStatusOK   = walk.RGB(0x5f, 0xd9, 0x8a)
	colorStatusWarn = walk.RGB(0xff, 0x9d, 0x6b)
)

var (
	fontUI    = Font{Family: "Inter", PointSize: 9}
	fontTitle = Font{Family: "Cormorant Garamond", PointSize: 18, SemiBold: true}
	fontSmall = Font{Family: "Inter", PointSize: 8}
)

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

// statusRow is one line of the onboarding checklist: a small icon (✓ / •)
// followed by a short description, both recolored by setOK/setPending as
// the underlying check's result changes — at startup, and again after the
// member acts (changes the folder, connects, syncs).
type statusRow struct {
	icon *walk.Label
	text *walk.Label
}

func (r *statusRow) setOK(text string) {
	r.icon.SetTextColor(colorStatusOK)
	r.icon.SetText("✓")
	r.text.SetTextColor(colorTextSoft)
	r.text.SetText(text)
}

func (r *statusRow) setPending(text string) {
	r.icon.SetTextColor(colorStatusWarn)
	r.icon.SetText("•")
	r.text.SetTextColor(colorMuted)
	r.text.SetText(text)
}

// statusRowWidget declares one onboarding checklist line. The icon and text
// labels start as placeholders — the caller fills them in with an initial
// setOK/setPending right after the window is created, so there is no
// flash of unstyled content.
func statusRowWidget(row *statusRow, label string, trailing ...Widget) Composite {
	children := []Widget{
		Label{AssignTo: &row.icon, Text: "•", MinSize: Size{Width: 16}, Font: fontUI, TextColor: colorMuted},
		Label{Text: label, MinSize: Size{Width: 170}, Font: fontUI, TextColor: colorTextSoft},
		Label{AssignTo: &row.text, Text: "…", Font: fontUI, TextColor: colorMuted},
	}
	children = append(children, trailing...)
	children = append(children, HSpacer{})
	return Composite{
		Layout:     HBox{MarginsZero: true, Spacing: 8},
		Background: SolidColorBrush{Color: colorGround},
		Children:   children,
	}
}

// a2ToolsInstalled reports whether A2Tools DPS Meter has ever run on this
// machine — its own app-data folder (the parent of sync.HistoryDir, the
// one fixed location it always uses: confirmed by reading a real member's
// settings.json, which lets the screenshot folder be customized but never
// the history folder) exists only once A2Tools itself has created it.
func a2ToolsInstalled() bool {
	info, err := os.Stat(filepath.Dir(sync.HistoryDir()))
	return err == nil && info.IsDir()
}

func folderExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func runGUI() {
	configDir := auth.ConfigDir()
	current, _ := settings.Load(configDir)
	if current.HistoryDir == "" {
		current.HistoryDir = sync.HistoryDir()
	}

	credential, _ := auth.LoadCredential(configDir)
	connected := credential != ""

	var mw *walk.MainWindow
	var statusLabel *walk.Label
	var logBox *walk.TextEdit
	var actionButton *walk.PushButton
	var changeFolderLink *walk.LinkLabel
	var a2toolsDownloadLink *walk.LinkLabel

	var a2toolsRow, folderRow, accountRow, syncRow statusRow

	// Every background goroutine below touches widgets only through these
	// functions, each hopping back onto the UI thread via Synchronize —
	// walk widgets are not safe to touch from any other goroutine.
	appendLog := func(line string) {
		walk.App().Synchronize(func() { logBox.AppendText(line + "\r\n") })
	}
	setStatus := func(text string) {
		walk.App().Synchronize(func() { statusLabel.SetText(text) })
	}
	setBusy := func(busy bool) {
		walk.App().Synchronize(func() {
			actionButton.SetEnabled(!busy)
			changeFolderLink.SetEnabled(!busy)
		})
	}

	refreshA2ToolsRow := func() {
		if a2ToolsInstalled() {
			a2toolsRow.setOK("Détecté")
			a2toolsDownloadLink.SetVisible(false)
		} else {
			a2toolsRow.setPending("Introuvable")
			a2toolsDownloadLink.SetVisible(true)
		}
	}
	refreshFolderRow := func() {
		if folderExists(current.HistoryDir) {
			folderRow.setOK(current.HistoryDir)
		} else {
			folderRow.setPending("Introuvable : " + current.HistoryDir)
		}
	}
	refreshAccountRow := func(connected bool) {
		if connected {
			accountRow.setOK("Connecté")
		} else {
			accountRow.setPending("Pas encore connecté")
		}
	}
	refreshSyncRow := func() {
		if current.LastSyncedAt == "" {
			syncRow.setPending("Jamais synchronisé")
			return
		}
		t, err := time.Parse(time.RFC3339, current.LastSyncedAt)
		if err != nil {
			syncRow.setPending("Jamais synchronisé")
			return
		}
		syncRow.setOK("Dernière synchro : " + t.Local().Format("02/01/2006 15:04"))
	}

	connectAndSync := func() {
		dir := current.HistoryDir
		if dir == "" {
			setStatus("Choisissez d'abord le dossier des journaux de combat.")
			return
		}
		if err := settings.Save(configDir, current); err != nil {
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
		walk.App().Synchronize(func() {
			accountRow.setOK("Connecté")
			actionButton.SetText("Synchroniser maintenant")
		})

		setStatus("Synchronisation des combats…")
		summary, err := sync.Run(ctx, client, credential, dir, out)
		if err != nil {
			setStatus("Échec de la synchronisation.")
			appendLog("Erreur : " + err.Error())
			return
		}

		current.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
		if err := settings.Save(configDir, current); err != nil {
			appendLog("La date de synchronisation n'a pas pu être enregistrée : " + err.Error())
		}
		walk.App().Synchronize(refreshSyncRow)

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

	actionButtonText := "Se connecter et synchroniser"
	if connected {
		actionButtonText = "Synchroniser maintenant"
	}

	err := MainWindow{
		AssignTo:   &mw,
		Title:      "Aion Theory Sync",
		Icon:       icon,
		MinSize:    Size{Width: 480, Height: 480},
		Background: SolidColorBrush{Color: colorGround},
		Font:       fontUI,
		Layout:     VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			// Header band: brand-dark frame around the logo and wordmark,
			// the one deliberately "designed" element the site's own
			// colors and serif wordmark font carry over to.
			Composite{
				Background: SolidColorBrush{Color: colorGroundDeep},
				Layout:     HBox{Margins: Margins{Left: 20, Top: 14, Right: 20, Bottom: 14}, Spacing: 12},
				Children: []Widget{
					ImageView{Image: logo, MinSize: Size{Width: 48, Height: 48}, MaxSize: Size{Width: 48, Height: 48}, Mode: ImageViewModeShrink},
					Label{Text: "Aion Theory Sync", Font: fontTitle, TextColor: colorText},
					HSpacer{},
				},
			},
			Composite{
				Background: SolidColorBrush{Color: colorGround},
				Layout:     VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 16}, Spacing: 10},
				Children: []Widget{
					Label{Text: "ÉTAT", Font: fontSmall, TextColor: colorFaint},
					statusRowWidget(&a2toolsRow, "DPS Meter A2Tools",
						LinkLabel{
							AssignTo: &a2toolsDownloadLink,
							Text:     `<a id="dl" href="https://a2tools.app">Télécharger</a>`,
							Font:     fontUI,
							OnLinkActivated: func(link *walk.LinkLabelLink) {
								openBrowser(link.URL())
							},
						},
					),
					statusRowWidget(&folderRow, "Dossier de logs",
						LinkLabel{
							AssignTo: &changeFolderLink,
							Text:     `<a id="change">Changer…</a>`,
							Font:     fontUI,
							OnLinkActivated: func(link *walk.LinkLabelLink) {
								dlg := &walk.FileDialog{Title: "Dossier des journaux de combat", InitialDirPath: current.HistoryDir}
								if ok, _ := dlg.ShowBrowseFolder(mw); ok {
									current.HistoryDir = dlg.FilePath
									if err := settings.Save(configDir, current); err != nil {
										appendLog("Le dossier choisi n'a pas pu être enregistré : " + err.Error())
									}
									refreshFolderRow()
								}
							},
						},
					),
					statusRowWidget(&accountRow, "Compte Aion Theory"),
					statusRowWidget(&syncRow, "Synchronisation"),
					VSpacer{Size: 6},
					PushButton{
						AssignTo: &actionButton,
						Text:     actionButtonText,
						OnClicked: func() {
							go func() {
								defer recoverAndReport()
								connectAndSync()
							}()
						},
					},
					Label{AssignTo: &statusLabel, Text: "Prêt.", Font: fontUI, TextColor: colorTextSoft},
					HSeparator{},
					Label{Text: "DÉTAILS TECHNIQUES", Font: fontSmall, TextColor: colorFaint},
					TextEdit{AssignTo: &logBox, ReadOnly: true, VScroll: true, MinSize: Size{Height: 110}, Font: fontSmall},
				},
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "Aion Theory Sync", "La fenêtre n'a pas pu s'ouvrir : "+err.Error(), walk.MsgBoxIconError)
		return
	}

	refreshA2ToolsRow()
	refreshFolderRow()
	refreshAccountRow(connected)
	refreshSyncRow()

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
