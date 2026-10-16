//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
	"github.com/wailsapp/go-webview2/pkg/edge"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/auth"
	"github.com/labsio-app/aion-theory-sync/internal/autostart"
	"github.com/labsio-app/aion-theory-sync/internal/settings"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
	"github.com/labsio-app/aion-theory-sync/internal/update"
	"github.com/labsio-app/aion-theory-sync/internal/webui"
)

// colorGround is the one color app.go itself still needs — the MainWindow
// and its stage Composite's background, briefly visible before WebView2
// paints over it (window creation, a resize mid-frame). Every other color
// decision now lives in internal/webui/templates/base.css; see
// web_windows.go's package comment for why the screens themselves moved
// there.
var colorGround = walk.RGB(0x08, 0x07, 0x1a)

func solid(c walk.Color) SolidColorBrush { return SolidColorBrush{Color: c} }

// quickCheckTimeout bounds a background "is the server reachable, what's
// pending" check — short because the window (or a screen's card) is
// waiting on it and should degrade to "found but unknown count" rather
// than hang if the network is slow.
const quickCheckTimeout = 10 * time.Second

// syncTimeout bounds one full sync run: generous (a large first sync can
// be hundreds of uploads, each paced) but still finite.
const syncTimeout = 30 * time.Minute

// linkTimeout bounds the device-flow wait: the device code itself expires
// after 15 minutes (internal/auth), the margin covers the approval step
// and the prerequisites check right after.
const linkTimeout = 20 * time.Minute

type app struct {
	mw    *walk.MainWindow
	stage *walk.Composite

	configDir string
	client    *api.Client
	autoStart autostart.Autostart

	machine    *state.Machine
	settings   settings.Settings
	credential string

	logLines  []string
	logWindow *walk.Dialog

	linkCancel context.CancelFunc
	syncCancel context.CancelFunc

	icon *walk.Icon

	updateChecker   *update.Checker
	updateAvailable bool
	updateInfo      *update.Available
	updating        bool
	dlProgress      downloadProgress

	tray    *walk.NotifyIcon
	watcher *folderWatcher

	// chromium, webServer, httpSrv and webURL are web_windows.go's: the
	// screens themselves render as HTML/CSS in an embedded WebView2
	// control instead of the widget tree cmd/gui used to build directly
	// (see that file's package comment).
	chromium  *edge.Chromium
	webServer *webui.Server
	httpSrv   *http.Server
	webURL    string
}

func runApp(startMinimized bool) {
	a := &app{
		configDir:     auth.ConfigDir(),
		client:        api.New(defaultServerURL),
		autoStart:     autostart.Windows{},
		updateChecker: update.New(),
	}

	loaded, _ := settings.Load(a.configDir)
	a.settings = loaded
	if a.settings.HistoryDir == "" {
		a.settings.HistoryDir = sync.HistoryDir()
	}

	credential, _ := auth.LoadCredential(a.configDir)
	a.credential = credential

	meterFound := folderExists(a.settings.HistoryDir)
	pending := 0
	if credential != "" && meterFound {
		ctx, cancel := context.WithTimeout(context.Background(), quickCheckTimeout)
		// Best effort: a network hiccup at launch just shows 0 pending
		// rather than blocking the window from opening; "Synchroniser
		// maintenant" retries the real count.
		pending, _ = sync.PendingCombats(ctx, a.client, credential, a.settings.HistoryDir)
		cancel()
	}

	initial := state.DeriveIdleState(credential != "", meterFound, pending)
	a.machine = state.NewMachine(initial)
	a.machine.DPSMeter = state.DPSMeterInfo{Found: meterFound, PendingCount: pending}
	if a.settings.LastSyncedAt != "" {
		if t, err := time.Parse(time.RFC3339, a.settings.LastSyncedAt); err == nil {
			a.machine.LastSyncedAt = t
		}
	}
	if initial == state.Blocked {
		a.machine.Block = state.BlockInfo{Reason: state.BlockDPSMeterMissing}
	}

	a.icon, _ = walk.NewIconFromFile(assetPath("icon.ico"))

	err := MainWindow{
		AssignTo:   &a.mw,
		Title:      "Aion Theory Sync",
		Icon:       a.icon,
		Size:       Size{Width: 960, Height: 600},
		MinSize:    Size{Width: 800, Height: 520},
		Background: solid(colorGround),
		Layout:     VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			Composite{
				AssignTo:   &a.stage,
				Background: solid(colorGround),
				Layout:     HBox{MarginsZero: true, SpacingZero: true},
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "Aion Theory Sync", "La fenêtre n'a pas pu s'ouvrir : "+err.Error(), walk.MsgBoxIconError)
		return
	}

	enableDarkTitleBar(a.mw.Handle())
	a.mw.SetExitOnClose(true)
	a.mw.Closing().Attach(a.onWindowClosing)
	a.mw.Activating().Attach(a.onWindowActivated)
	a.setupTray()
	if a.settings.AutoSync {
		a.startAutoSync()
	}

	// setupWebView embeds WebView2 into a.stage and navigates it to the
	// local server's current state — the app's actual first paint, so
	// nothing else needs to call a.render() before this. It must run
	// before walk.App().Run() starts the normal message loop: see its own
	// doc comment (web_windows.go) for why.
	if err := a.setupWebView(); err != nil {
		walk.MsgBox(nil, "Aion Theory Sync", "L'affichage n'a pas pu démarrer : "+err.Error(), walk.MsgBoxIconError)
		return
	}
	if !startMinimized || a.tray == nil {
		a.mw.Show()
	}

	if a.credential != "" {
		a.refreshCharacter()
	}

	go func() {
		defer recoverAndReport()
		checkForUpdate(a.appendLog, func(available update.Available) {
			a.ui(func() {
				a.updateInfo = &available
				a.updateAvailable = true
				a.render() // the banner (web_windows.go) is on every screen, not just Settings
			})
		})
	}()

	walk.App().Run()
}

// enableDarkTitleBar switches the native title bar to dark mode
// (DWMWA_USE_IMMERSIVE_DARK_MODE), best effort: a system predating it
// (pre-20H1 Windows 10) just keeps the default light title bar.
func enableDarkTitleBar(hwnd win.HWND) {
	dark := int32(1)
	win.DwmSetWindowAttribute(hwnd, win.DWMWA_USE_IMMERSIVE_DARK_MODE, unsafe.Pointer(&dark), uint32(unsafe.Sizeof(dark)))
}

// ui runs f on the UI thread — every background goroutine (device-flow
// polling, a sync run) must go through this before touching a.machine or
// calling a.render; walk widgets are not safe to touch from any other
// goroutine.
func (a *app) ui(f func()) {
	walk.App().Synchronize(f)
}

func (a *app) appendLog(line string) {
	a.ui(func() {
		a.logLines = append(a.logLines, line)
		if a.logWindow != nil {
			// best effort: the log window re-reads a.logLines in full the
			// next time it's opened; nothing live-updates it while open,
			// which is an acceptable simplification for a window the
			// member opens to copy-paste a report, not to watch live.
			_ = a.logWindow
		}
	})
}

// render tells the embedded page to re-fetch the current state and swap
// itself in — the one place cmd/gui reacts to a Machine change, and the
// only thing most event handlers below need to call once they've told the
// Machine what happened. See web_windows.go's webuiRefreshScript.
func (a *app) render() {
	if a.chromium != nil {
		a.chromium.Eval(webuiRefreshScript)
	}
}

func (a *app) saveSettings() {
	if err := settings.Save(a.configDir, a.settings); err != nil {
		a.appendLog("Réglages non enregistrés : " + err.Error())
	}
}

func (a *app) onToggleAutoSync(checked bool) {
	a.settings.AutoSync = checked
	a.saveSettings()
	if checked {
		a.startAutoSync()
	} else {
		a.stopAutoSync()
	}
}

func (a *app) applyAutostart() {
	var err error
	if a.settings.LaunchAtStartup {
		err = a.autoStart.Enable(exePath(), a.settings.StartMinimized)
	} else {
		err = a.autoStart.Disable()
	}
	if err != nil {
		a.appendLog("Démarrage automatique : " + err.Error())
	}
}

// --- Screen 0/1: linking --------------------------------------------------

func (a *app) onLinkRequested() {
	if err := a.machine.LinkRequested(); err != nil {
		return
	}
	a.render()

	ctx, cancel := context.WithTimeout(context.Background(), linkTimeout)
	a.linkCancel = cancel

	go func() {
		defer recoverAndReport()
		out := &guiWriter{a.appendLog}
		credential, err := auth.EnsureCredentialWithGrant(ctx, a.client, a.configDir, out, func(grant api.DeviceGrant) {
			a.ui(func() {
				a.machine.GrantOpened(grant)
				a.render()
				openBrowser(grant.VerificationURL)
			})
		})
		if err != nil {
			a.ui(func() {
				if errors.Is(err, context.Canceled) {
					return // "Annuler" already moved back to Unlinked itself
				}
				a.machine.GrantExpired()
				a.render()
			})
			return
		}

		a.ui(func() {
			a.credential = credential
			a.machine.GrantApproved()
			// Screen 2's toggles start pre-checked for a fresh link (the
			// UX spec's "activé par défaut"); applied here rather than in
			// settings.Load so an existing settings.json keeps whatever
			// the member already chose.
			defaults := settings.OnboardingDefaults()
			a.settings.LaunchAtStartup = defaults.LaunchAtStartup
			a.settings.AutoSync = defaults.AutoSync
			a.applyAutostart()
			a.saveSettings()
			if a.settings.AutoSync {
				a.startAutoSync()
			}
			a.render()
			a.refreshPrereqs(false)
			a.refreshCharacter()
		})
	}()
}

func (a *app) onCancelLinking() {
	if a.linkCancel != nil {
		a.linkCancel()
	}
	a.machine.UnlinkRequested()
	a.render()
}

// --- Screen 2/7: prerequisites --------------------------------------------

// refreshPrereqs re-checks the DPS meter folder and, if found, how many
// combats are pending, then either just updates the on-screen card
// (advance == false — screen 2, before "Terminer") or lets the machine
// leave Prerequisites/Blocked right away (advance == true — screen 7's
// "Réessayer", which has no separate confirm step).
func (a *app) refreshPrereqs(advance bool) {
	go func() {
		defer recoverAndReport()
		found := folderExists(a.settings.HistoryDir)
		info := state.DPSMeterInfo{Found: found}
		if found {
			ctx, cancel := context.WithTimeout(context.Background(), quickCheckTimeout)
			pending, err := sync.PendingCombats(ctx, a.client, a.credential, a.settings.HistoryDir)
			cancel()
			if err == nil {
				info.PendingCount = pending
			}
		}
		a.ui(func() {
			switch {
			case advance && found:
				a.machine.PrereqsOK(info)
			case !found:
				a.machine.PrereqsMissing(info)
			default:
				a.machine.DPSMeter = info
			}
			a.render()
		})
	}()
}

// refreshCharacter fetches the member's declared, active characters,
// best effort: any error (including a 404 when the server's sync feature
// flag is off) just leaves screen 2's card, the sidebar and screen 5's
// subtitle hidden until a later call succeeds, same spirit as
// refreshPrereqs' pending-count fetch above.
func (a *app) refreshCharacter() {
	go func() {
		defer recoverAndReport()
		ctx, cancel := context.WithTimeout(context.Background(), quickCheckTimeout)
		characters, err := a.client.Characters(ctx, a.credential)
		cancel()
		if err != nil {
			return
		}
		info := state.CharacterInfo{Known: len(characters) > 0}
		for _, c := range characters {
			info.Characters = append(info.Characters, state.Character{Name: c.CharacterName, Server: c.ServerName})
		}
		a.ui(func() {
			a.machine.Character = info
			a.render()
		})
	}()
}

// onWindowActivated re-checks the declared-character list whenever the
// member brings the window back to the foreground — the same moment
// they'd plausibly be returning from declaring one in their system
// browser (screen 2's "Déclarer un personnage ↗" opens the account page
// externally; see screens.go). A no-op once a character is already known,
// and while unlinked: nothing in the UX needs this to keep polling after
// the card is already filled in, or before there's a credential to call
// the endpoint with.
func (a *app) onWindowActivated() {
	if a.credential == "" || a.machine.Character.Known {
		return
	}
	a.refreshCharacter()
}

func (a *app) onFinishPrerequisites() {
	if err := a.machine.PrereqsOK(a.machine.DPSMeter); err != nil {
		return
	}
	a.render()
}

func (a *app) onChooseFolder(advance bool) {
	dlg := &walk.FileDialog{Title: "Dossier des journaux de combat", InitialDirPath: a.settings.HistoryDir}
	ok, err := dlg.ShowBrowseFolder(a.mw)
	if err != nil || !ok {
		return
	}
	a.settings.HistoryDir = dlg.FilePath
	a.settings.HistoryDirIsCustom = true
	a.saveSettings()
	a.refreshPrereqs(advance)
}

// --- Screens 3/4/5/6: syncing ---------------------------------------------

func (a *app) onSyncNow() {
	if err := a.machine.SyncStarted(); err != nil {
		return
	}
	a.render()

	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	a.syncCancel = cancel

	go func() {
		defer recoverAndReport()
		defer cancel()

		out := &guiWriter{a.appendLog}
		summary, err := sync.RunWithOptions(ctx, a.client, a.credential, a.settings.HistoryDir, out, sync.Options{
			Paused: func() bool { return a.machine.Paused },
			OnProgress: func(e sync.ProgressEvent) {
				a.ui(func() {
					if e.RateLimited > 0 {
						a.machine.RateLimited(e.RateLimited, time.Now())
					} else {
						if a.machine.RateLimit.Active {
							a.machine.Resumed()
						}
						a.machine.Progress(state.ProgressInfo{
							Total: e.Total, Done: e.Done,
							Current: state.CurrentFile{StartedAt: e.Current.StartedAt, Duration: e.Current.Duration},
						})
					}
					a.render()
				})
			},
		})

		a.ui(func() {
			if err != nil {
				if errors.Is(err, api.ErrUnauthorized) {
					_ = auth.ClearCredential(a.configDir)
					a.credential = ""
				}
				a.machine.SyncFailed(err)
				a.render()
				return
			}

			checkCtx, cancelCheck := context.WithTimeout(context.Background(), quickCheckTimeout)
			pending, _ := sync.PendingCombats(checkCtx, a.client, a.credential, a.settings.HistoryDir)
			cancelCheck()

			a.machine.SyncFinished(summary, pending)
			a.machine.LastSyncedAt = time.Now()
			a.settings.LastSyncedAt = a.machine.LastSyncedAt.UTC().Format(time.RFC3339)
			a.saveSettings()
			a.render()
		})
	}()
}

func (a *app) onTogglePause() {
	a.machine.Paused = !a.machine.Paused
	a.render()
}

func (a *app) onCloseDone() {
	a.machine.Current = state.DeriveIdleState(a.credential != "", a.machine.DPSMeter.Found, a.machine.DPSMeter.PendingCount)
	a.render()
}

// --- Screen 7: blocked -----------------------------------------------------

func (a *app) onRetryBlocked() {
	switch a.machine.Block.Reason {
	case state.BlockDPSMeterMissing:
		a.refreshPrereqs(true)
	default:
		a.onSyncNow()
	}
}

// --- Screen 8: settings and account ---------------------------------------

func (a *app) onNavSettings() {
	if err := a.machine.SettingsOpened(); err != nil {
		return
	}
	a.render()
}

func (a *app) onNavSync() {
	if a.machine.Current != state.Settings {
		return
	}
	if err := a.machine.SettingsClosed(); err != nil {
		return
	}
	a.render()
}

func (a *app) onUnlink() {
	if walk.MsgBox(a.mw, "Délier cet appareil",
		"Cet appareil ne pourra plus synchroniser avec ton compte Aion Theory jusqu'à ce que tu le relies à nouveau. Continuer ?",
		walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	_ = auth.ClearCredential(a.configDir)
	a.credential = ""
	_ = a.autoStart.Disable()
	a.stopAutoSync()
	a.settings.LaunchAtStartup = false
	a.settings.AutoSync = false
	a.saveSettings()
	a.machine.UnlinkRequested()
	a.render()
}

func (a *app) onOpenTechnicalLog() {
	if a.logWindow != nil {
		a.logWindow.Show()
		_ = a.logWindow.SetFocus()
		return
	}

	var textEdit *walk.TextEdit
	dlg := Dialog{
		AssignTo: &a.logWindow,
		Title:    "Journal technique",
		Icon:     a.icon,
		MinSize:  Size{Width: 560, Height: 420},
		Layout:   VBox{Margins: Margins{Left: 16, Top: 16, Right: 16, Bottom: 16}, Spacing: 10},
		Children: []Widget{
			TextEdit{AssignTo: &textEdit, ReadOnly: true, VScroll: true, Text: joinLines(a.logLines)},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{Text: "Copier", MinSize: Size{Height: 40}, OnClicked: func() {
						_ = walk.Clipboard().SetText(textEdit.Text())
					}},
					PushButton{Text: "Fermer", MinSize: Size{Height: 40}, OnClicked: func() {
						a.logWindow.Close(walk.DlgCmdCancel)
					}},
				},
			},
		},
	}
	if err := dlg.Create(a.mw); err != nil {
		a.appendLog("Le journal technique n'a pas pu s'ouvrir : " + err.Error())
		return
	}
	a.logWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		a.logWindow = nil
	})
	a.logWindow.Show()
}

// --- Self-update banner (every screen) --------------------------------

// onUpdateClicked starts the actual download for the update the banner is
// showing — see updateBanner in screens.go. Always member-initiated:
// checkForUpdate only ever reports an update is available, never starts
// downloading it on its own, so this is the one place the fetch, the
// in-place swap (update.Apply) and the relaunch (update.Relaunch) happen,
// and only once a member clicks the banner's button.
func (a *app) onUpdateClicked() {
	if a.updating || a.updateInfo == nil || a.machine.Current == state.Syncing {
		return
	}
	a.updating = true
	a.dlProgress = downloadProgress{StartedAt: time.Now()}
	a.render()

	downloadURL := a.updateInfo.DownloadURL
	go func() {
		defer recoverAndReport()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		newExePath, err := a.updateChecker.Download(ctx, downloadURL, func(downloaded, total int64) {
			a.ui(func() {
				before := a.dlProgress.percent()
				a.dlProgress.Downloaded, a.dlProgress.Total = downloaded, total
				// Re-rendering on every chunk (a few hundred for a ~15 MB
				// exe) would needlessly tear down and rebuild the whole
				// screen; only a changed percentage is worth repainting.
				if a.dlProgress.percent() != before {
					a.render()
				}
			})
		})
		if err != nil {
			a.onUpdateFailed("Échec du téléchargement de la mise à jour : " + err.Error())
			return
		}
		if err := update.Apply(newExePath); err != nil {
			a.onUpdateFailed("Échec de l'installation de la mise à jour : " + err.Error())
			return
		}
		if err := update.Relaunch(); err != nil {
			a.onUpdateFailed("Mise à jour installée, mais le redémarrage a échoué : " + err.Error())
			return
		}
		os.Exit(0)
	}()
}

// onUpdateFailed resets the banner back to its "available" state (rather
// than leaving it stuck showing a dead progress bar) and logs why, so a
// member can at least retry or check the technical log.
func (a *app) onUpdateFailed(logLine string) {
	a.ui(func() {
		a.updating = false
		a.appendLog(logLine)
		a.render()
	})
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\r\n"
	}
	return out
}

func folderExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func versionLabel(current string, updateAvailable bool) string {
	status := "à jour"
	if updateAvailable {
		status = "mise à jour disponible"
	}
	return fmt.Sprintf("Version %s · %s", current, status)
}
