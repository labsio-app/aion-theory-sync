//go:build windows

package main

import (
	"github.com/tailscale/walk"
)

// setupTray creates the notification-area icon: left-click or "Ouvrir"
// reopens the window, "Synchroniser maintenant" opens it and starts a
// sync, "Quitter" actually exits (closing the window itself only hides it
// — see onWindowClosing).
func (a *app) setupTray() {
	ni, err := walk.NewNotifyIcon()
	if err != nil {
		a.appendLog("Zone de notification indisponible : " + err.Error())
		return
	}
	a.tray = ni

	if a.icon != nil {
		_ = ni.SetIcon(a.icon)
	}
	_ = ni.SetToolTip("Aion Theory Sync")

	syncNow := walk.NewAction()
	_ = syncNow.SetText("Synchroniser maintenant")
	syncNow.Triggered().Attach(func() {
		a.showWindow()
		a.onSyncNow()
	})

	open := walk.NewAction()
	_ = open.SetText("Ouvrir")
	open.Triggered().Attach(func() { a.showWindow() })

	quit := walk.NewAction()
	_ = quit.SetText("Quitter")
	quit.Triggered().Attach(a.quit)

	_ = ni.ContextMenu().Actions().Add(syncNow)
	_ = ni.ContextMenu().Actions().Add(open)
	_ = ni.ContextMenu().Actions().Add(quit)

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.showWindow()
		}
	})

	_ = ni.SetVisible(true)
}

func (a *app) showWindow() {
	a.mw.Show()
	_ = a.mw.SetFocus()
	_ = a.mw.BringToTop()
}

// quit is the only path that actually ends the process while the tray
// exists: closing the window (the X button) just hides it when auto-sync
// is on (onWindowClosing), so "Quitter" has to bypass that by flipping
// ExitOnClose back on first.
func (a *app) quit() {
	if a.tray != nil {
		a.tray.Dispose()
		a.tray = nil
	}
	a.mw.SetExitOnClose(true)
	_ = a.mw.Close()
}

// onWindowClosing is the MainWindow's Closing handler: with auto-sync on
// and a tray icon up, the X button hides the window instead of exiting —
// the whole point of running in the background. Without auto-sync there
// is nothing to watch for in the background, so closing the window quits
// normally, matching the pre-tray behavior.
func (a *app) onWindowClosing(canceled *bool, reason walk.CloseReason) {
	if !a.settings.AutoSync || a.tray == nil {
		return
	}
	*canceled = true
	a.mw.SetVisible(false)
}
