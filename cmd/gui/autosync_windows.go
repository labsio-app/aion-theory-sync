//go:build windows

package main

import (
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
)

// folderWatcher is the handle a.watcher holds while the auto-sync loop is
// running; closing stop ends it.
type folderWatcher struct {
	stop chan struct{}
}

// startAutoSync is a no-op if a loop is already running — settings.AutoSync
// flipping on twice in a row (or being true already at launch) must not
// spawn a second one.
func (a *app) startAutoSync() {
	if a.watcher != nil {
		return
	}
	stop := make(chan struct{})
	a.watcher = &folderWatcher{stop: stop}
	go a.runAutoSyncLoop(stop)
}

func (a *app) stopAutoSync() {
	if a.watcher == nil {
		return
	}
	close(a.watcher.stop)
	a.watcher = nil
}

func (a *app) runAutoSyncLoop(stop <-chan struct{}) {
	defer recoverAndReport()

	ticker := time.NewTicker(autoSyncPollInterval)
	defer ticker.Stop()

	var lastTriggered time.Time
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		newest := newestHistoryFileModTime(a.settings.HistoryDir)
		if !shouldTriggerAutoSync(newest, lastTriggered, time.Now(), autoSyncDebounce) {
			continue
		}
		lastTriggered = newest

		a.ui(func() {
			// Only from an idle screen: a sync already running, or the
			// member mid-onboarding/mid-settings, is left alone rather
			// than interrupted by a background trigger.
			if a.machine.Current == state.Ready || a.machine.Current == state.UpToDate {
				a.onSyncNow()
			}
		})
	}
}
