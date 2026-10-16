package webui

import (
	"errors"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
)

// Fixture is one named, reproducible (machine, extra, now) triple the
// visual-regression harness (tools/visual) renders and screenshots. Every
// screen the Machine can show gets at least one fixture here, plus the
// notable variants (banners, warnings, an update available) that change a
// screen's layout rather than just its text.
type Fixture struct {
	Name  string
	Now   time.Time
	Build func() (*state.Machine, Extra)
}

var fixtureNow = time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC)

func baseExtra() Extra {
	return Extra{
		LogoURL:        "/assets/img/logo.png",
		A2ToolsLogoURL: "/assets/img/a2tools.png",
		IconURL:        "/assets/img/icon.png",
		Version:        "0.2.8",
	}
}

// Fixtures lists every fixture the harness renders. Order is the display
// order in the preview tool's index and in the screenshot gallery.
func Fixtures() []Fixture {
	return []Fixture{
		{"00-unlinked", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Unlinked)
			return m, baseExtra()
		}},
		{"00-unlinked-expired", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Linking)
			_ = m.GrantExpired()
			return m, baseExtra()
		}},
		{"00-unlinked-revoked", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			_ = m.SyncFailed(api.ErrUnauthorized)
			return m, baseExtra()
		}},
		{"01-linking", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Unlinked)
			_ = m.LinkRequested()
			_ = m.GrantOpened(api.DeviceGrant{
				UserCode:        "ABCD-1234",
				VerificationURL: "https://aion-theory.labsio.app/activate",
				ExpiresAt:       fixtureNow.Add(12 * time.Minute).Format(time.RFC3339),
			})
			return m, baseExtra()
		}},
		{"02-prerequisites-incomplete", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Prerequisites)
			m.DPSMeter = state.DPSMeterInfo{Found: false}
			extra := baseExtra()
			extra.CredentialPresent = true
			extra.LaunchAtStartup, extra.AutoSync = true, true
			return m, extra
		}},
		{"02-prerequisites-complete", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Prerequisites)
			m.DPSMeter = state.DPSMeterInfo{Found: true, PendingCount: 3}
			m.Character = state.CharacterInfo{Known: true, Characters: []state.Character{{Name: "Yume", Server: "Siel"}}}
			extra := baseExtra()
			extra.CredentialPresent = true
			extra.DPSMeterFound = true
			extra.LaunchAtStartup, extra.AutoSync = true, true
			return m, extra
		}},
		{"03-ready", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Ready)
			m.DPSMeter = state.DPSMeterInfo{Found: true, PendingCount: 7}
			m.Character = state.CharacterInfo{Known: true, Characters: []state.Character{{Name: "Yume", Server: "Siel"}}}
			m.LastSyncedAt = fixtureNow.Add(-45 * time.Minute)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"04-syncing", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			m.Character = state.CharacterInfo{Known: true, Characters: []state.Character{{Name: "Yume", Server: "Siel"}}}
			_ = m.Progress(state.ProgressInfo{
				Total: 7, Done: 3,
				Current: state.CurrentFile{StartedAt: fixtureNow.Add(-90 * time.Second), Duration: 3*time.Minute + 12*time.Second},
			})
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"04-syncing-rate-limited", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			_ = m.Progress(state.ProgressInfo{Total: 7, Done: 3})
			_ = m.RateLimited(18*time.Second, fixtureNow)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"05-done", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			m.Character = state.CharacterInfo{Known: true, Characters: []state.Character{{Name: "Yume", Server: "Siel"}}}
			_ = m.SyncFinished(sync.Summary{Recorded: 6, AlreadyRecorded: 2}, 0)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"05-done-with-warnings", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			_ = m.SyncFinished(sync.Summary{
				Recorded:        4,
				AlreadyRecorded: 1,
				Refused:         map[string]int{"NO_MATCHING_CHARACTER": 2},
				Skipped:         map[string]int{"UNREADABLE": 1},
				Failed:          1,
			}, 0)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"06-up-to-date", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.UpToDate)
			m.LastSyncedAt = fixtureNow.Add(-2 * time.Hour)
			m.Character = state.CharacterInfo{Known: true, Characters: []state.Character{{Name: "Yume", Server: "Siel"}}}
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"07-blocked-meter-missing", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Blocked)
			m.Block = state.BlockInfo{Reason: state.BlockDPSMeterMissing}
			extra := baseExtra()
			extra.CredentialPresent = true
			return m, extra
		}},
		{"07-blocked-network", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Syncing)
			_ = m.SyncFailed(errors.New("dial tcp: no route to host"))
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			return m, extra
		}},
		{"08-settings", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.Settings)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			extra.LaunchAtStartup, extra.AutoSync = true, true
			return m, extra
		}},
		{"09-update-banner", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.UpToDate)
			m.LastSyncedAt = fixtureNow.Add(-2 * time.Hour)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			extra.Update = UpdateInfo{Available: true, Version: "0.2.9"}
			return m, extra
		}},
		{"09-update-downloading", fixtureNow, func() (*state.Machine, Extra) {
			m := state.NewMachine(state.UpToDate)
			m.LastSyncedAt = fixtureNow.Add(-2 * time.Hour)
			extra := baseExtra()
			extra.CredentialPresent, extra.DPSMeterFound = true, true
			extra.Update = UpdateInfo{Available: true, Version: "0.2.9", Updating: true, Percent: 42, Label: "Téléchargement… 42 % · encore environ 12 s"}
			return m, extra
		}},
	}
}

// Page builds the Page this fixture renders — the same BuildPage call the
// real app makes, just fed a fixture's Machine/Extra instead of the live
// one.
func (f Fixture) Page() Page {
	m, extra := f.Build()
	vm := state.BuildViewModel(m, f.Now)
	return BuildPage(vm, extra)
}
