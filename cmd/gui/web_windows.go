//go:build windows

// web_windows.go replaces the old screens.go/widgets.go/theme.go: instead
// of building a walk widget tree from a state.ViewModel, it embeds a
// WebView2 control into a.stage and points it at a local HTTP server
// serving internal/webui's HTML/CSS rendering of that same ViewModel. See
// internal/webui's package comment for why: real CSS instead of owner-
// drawn Win32 composites fixes the layout-deformation and native-control-
// styling complaints the walk version couldn't.
//
// walk is still what creates the window, the tray icon, native dialogs
// (file picker, the technical-log window, message boxes) and the dark
// title bar — none of that was the problem, so none of it changed.
package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/tailscale/walk"
	"github.com/wailsapp/go-webview2/pkg/edge"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
	"github.com/labsio-app/aion-theory-sync/internal/version"
	"github.com/labsio-app/aion-theory-sync/internal/webui"
)

// webuiRefreshScript is what a.render() (app.go) runs in the page after
// every state change: re-fetch "/" and swap in its <div class="app">
// root, rather than a full navigation — see templates/app.js's
// window.__refresh for why (no blank-frame flash, no lost scroll
// position on screens that don't actually need to reset it).
const webuiRefreshScript = "window.__refresh && window.__refresh();"

// setupWebView starts the local HTTP server and embeds WebView2 into
// a.stage, then navigates it to that server's current state — the app's
// first paint.
//
// Must run before walk.App().Run() starts the normal Win32 message loop:
// Chromium.Embed pumps its own message loop internally until the WebView2
// environment and controller finish initializing (both are asynchronous
// COM operations under the hood), and two message loops on the same
// thread can't run at once.
func (a *app) setupWebView() error {
	if err := a.startWebServer(); err != nil {
		return err
	}

	chromium := edge.NewChromium()
	chromium.SetErrorCallback(func(err error) {
		// Chromium's own errorCallback always os.Exit(1)s right after this
		// runs (see go-webview2/pkg/edge) — this is the one chance to tell
		// the member why, rather than the app just vanishing. The most
		// likely cause by far: the WebView2 Runtime isn't installed (it
		// ships with Windows 11 and is normally pushed to Windows 10 via
		// Windows Update, but a locked-down or offline machine may lack
		// it).
		walk.MsgBox(nil, "Aion Theory Sync",
			"Le moteur d'affichage (WebView2) n'a pas pu démarrer : "+err.Error()+
				"\n\nInstalle le WebView2 Runtime depuis "+
				"https://developer.microsoft.com/microsoft-edge/webview2/ puis relance l'application.",
			walk.MsgBoxIconError)
	})
	chromium.Embed(uintptr(a.stage.Handle()))
	chromium.SetBackgroundColour(0x08, 0x07, 0x1a, 255)
	chromium.Resize()
	a.chromium = chromium

	a.stage.SizeChanged().Attach(func() {
		if a.chromium != nil {
			a.chromium.Resize()
		}
	})

	chromium.Navigate(a.webURL)
	return nil
}

// startWebServer binds an ephemeral loopback port (no fixed port to
// collide with another instance, another app, or a previous run that
// didn't exit cleanly) and serves internal/webui's pages from it: "/" and
// "/assets/" via webui.Server (always rendering a.buildPage(), the
// current Machine state), "/action/*" via a.handleAction below.
func (a *app) startWebServer() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	a.webServer = webui.NewServer(a.buildPage)
	mux := http.NewServeMux()
	mux.Handle("/action/", http.HandlerFunc(a.handleAction))
	mux.Handle("/", a.webServer)

	srv := &http.Server{Handler: mux}
	a.httpSrv = srv
	a.webURL = "http://" + listener.Addr().String() + "/"

	go func() {
		defer recoverAndReport()
		_ = srv.Serve(listener) // ends when the process exits; nothing calls Shutdown
	}()
	return nil
}

// buildPage is internal/webui.Server's Provide callback: it's called
// fresh on every request, so the page always reflects whatever a.machine
// says right now — the same "derive, don't cache" rule BuildViewModel
// itself follows.
func (a *app) buildPage() webui.Page {
	vm := state.BuildViewModel(a.machine, time.Now())
	return webui.BuildPage(vm, a.webExtra())
}

// webExtra gathers the app-level fields screens.go used to read directly
// off a — settings, credential presence, self-update progress — into the
// Extra internal/webui.BuildPage needs alongside the ViewModel.
func (a *app) webExtra() webui.Extra {
	extra := webui.Extra{
		LogoURL:            "/assets/img/logo.png",
		A2ToolsLogoURL:     "/assets/img/a2tools.png",
		CredentialPresent:  a.credential != "",
		DPSMeterFound:      a.machine.DPSMeter.Found,
		Version:            version.Version,
		LaunchAtStartup:    a.settings.LaunchAtStartup,
		StartMinimized:     a.settings.StartMinimized,
		AutoSync:           a.settings.AutoSync,
		HistoryDirIsCustom: a.settings.HistoryDirIsCustom,
	}
	if a.updateAvailable {
		extra.Update = webui.UpdateInfo{
			Available:  true,
			Updating:   a.updating,
			Percent:    a.dlProgress.percent(),
			Label:      a.dlProgress.label(time.Now()),
			SyncingNow: a.machine.Current == state.Syncing,
		}
		if a.updateInfo != nil {
			extra.Update.Version = a.updateInfo.Version
		}
	}
	return extra
}

// handleAction is every control's data-action click/change handler
// (templates/app.js): POST /action/<name>, dispatched on the walk UI
// thread (a.ui) since the handlers it calls touch a.machine and the
// webview itself, neither safe to touch off that thread.
func (a *app) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/action/")

	var payload struct {
		Checked bool `json:"checked"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	a.ui(func() { a.dispatchAction(name, payload.Checked) })
	w.WriteHeader(http.StatusNoContent)
}

// dispatchAction maps a data-action name to the same app.go handlers the
// old walk widgets called directly from their OnClicked/OnCheckedChanged
// callbacks — business logic is unchanged, only how a click reaches it.
func (a *app) dispatchAction(name string, checked bool) {
	switch name {
	case "link-requested":
		a.onLinkRequested()
	case "reopen-link":
		if a.machine.Grant.VerificationURL != "" {
			openBrowser(a.machine.Grant.VerificationURL)
		}
	case "cancel-linking":
		a.onCancelLinking()
	case "open-install-dps-meter":
		openBrowser("https://a2tools.app")
	case "open-declare-character":
		openBrowser(defaultServerURL)
	case "choose-folder":
		a.onChooseFolder(false)
	case "choose-folder-and-retry":
		a.onChooseFolder(true)
	case "toggle-launch-at-startup":
		a.settings.LaunchAtStartup = checked
		a.applyAutostart()
		a.saveSettings()
	case "toggle-start-minimized":
		a.settings.StartMinimized = checked
		a.applyAutostart()
		a.saveSettings()
	case "toggle-auto-sync":
		a.onToggleAutoSync(checked)
	case "finish-prerequisites":
		a.onFinishPrerequisites()
	case "sync-now":
		a.onSyncNow()
	case "toggle-pause":
		a.onTogglePause()
	case "close-done":
		a.onCloseDone()
	case "view-online":
		openBrowser(defaultServerURL)
	case "retry-blocked":
		a.onRetryBlocked()
	case "open-technical-log":
		a.onOpenTechnicalLog()
	case "unlink":
		a.onUnlink()
	case "nav-sync":
		a.onNavSync()
	case "nav-settings":
		a.onNavSettings()
	case "update-clicked":
		a.onUpdateClicked()
	}
}
