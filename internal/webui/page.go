// Package webui replaces cmd/gui's old walk-widget rendering (screens.go,
// widgets.go, theme.go) with server-rendered HTML/CSS: the same
// internal/ui/state Machine/ViewModel drives it, but the actual pixels are
// drawn by a real browser engine (WebView2 on Windows) instead of owner-
// drawn Win32 composites, so cards get real rounded corners, buttons and
// toggles get real colors, and flex layouts hold their shape on resize
// instead of deforming.
//
// Nothing in this package depends on Windows or on walk, so it — and the
// screenshot-based visual-regression harness in tools/visual — runs on any
// platform, the same testability internal/ui/state already has.
package webui

import (
	"fmt"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
)

// Extra is everything a page needs that isn't already in a
// state.ViewModel: app-level settings, asset URLs, and self-update
// progress — the fields cmd/gui's old screens.go pulled from the app
// struct directly rather than from the ViewModel.
type Extra struct {
	LogoURL        string
	A2ToolsLogoURL string
	IconURL        string

	CredentialPresent bool
	DPSMeterFound     bool
	Version           string

	LaunchAtStartup    bool
	StartMinimized     bool
	AutoSync           bool
	HistoryDirIsCustom bool

	Update UpdateInfo
}

// UpdateInfo mirrors app.go's updateAvailable/updating/updateInfo/dlProgress
// fields (main_windows.go, app.go) — see BuildUpdateBanner.
type UpdateInfo struct {
	Available  bool
	Version    string
	Updating   bool
	Percent    int    // -1 when the total size isn't known yet
	Label      string // e.g. "Téléchargement… 42 % · encore environ 12 s"
	SyncingNow bool   // true disables the button: see updateBanner's old doc comment
}

// Page is everything the shell template needs, already reduced to plain
// strings/bools/slices — no business logic or state-package knowledge
// lives in the .html templates themselves, the same boundary
// internal/ui/state's own doc comment draws for cmd/gui.
type Page struct {
	Gabarit string // "a" or "b"
	Screen  string // state.State, used to pick which body template to render

	LogoURL        string
	A2ToolsLogoURL string

	UpdateBanner *UpdateBannerView

	SidebarA *SidebarAView
	SidebarB *SidebarBView

	Unlinked      *state.UnlinkedView
	Linking       *linkingView
	Prerequisites *prerequisitesView
	Ready         *state.ReadyView
	Syncing       *syncingView
	Done          *doneView
	UpToDate      *state.UpToDateView
	Blocked       *blockedView
	Settings      *settingsView
}

type UpdateBannerView struct {
	Text           string
	ButtonDisabled bool
	ButtonLabel    string
	Updating       bool
	ProgressLabel  string
	Percent        int
}

type StepperStepView struct {
	Label string
	Class string // "is-done", "is-current", or ""
	Mark  string
}

type SidebarAView struct {
	Steps []StepperStepView
}

type NavItemView struct {
	Label  string
	Active bool
	Action string
}

type StatusLineView struct {
	IconURL string
	Label   string
	OK      bool
}

type SidebarBView struct {
	Nav    []NavItemView
	Status []StatusLineView
}

type linkingView struct {
	*state.LinkingView
}

// prereqCardView's ActionName is the data-action the card's button posts
// to the local server (see cmd/gui's dispatchAction, web_windows.go) —
// every external navigation goes through a server action and openBrowser
// rather than a real <a href>, so nothing ever navigates the embedded
// WebView2 away from the app itself.
type prereqCardView struct {
	Title            string
	Subtitle         string
	Done             bool
	ActionLabel      string
	ActionName       string // "" when Done or no action
	ExtraActionLabel string // "Choisir le dossier" on the DPS-meter card only
}

type prerequisitesView struct {
	Title                string
	AccountCard          prereqCardView
	DPSMeterCard         prereqCardView
	CharacterCard        prereqCardView
	LaunchAtStartupLabel string
	AutoSyncLabel        string
	FinishButton         string
	FinishEnabled        bool
	LaunchAtStartup      bool
	AutoSync             bool
}

type syncingView struct {
	*state.SyncingView
	MaxValue int
}

type summaryTileView struct {
	Label   string
	Count   int
	Warning bool
}

type doneView struct {
	*state.DoneView
	AlreadyPresentTile      summaryTileView
	NoMatchingCharacterTile summaryTileView
	UnreadableTile          summaryTileView
}

type blockedView struct {
	*state.BlockedView
	ShowPrimary     bool
	ShowInstallLink bool
}

type settingsView struct {
	FolderLabel  string
	AccountLabel string
	VersionLabel string

	LaunchAtStartup bool
	StartMinimized  bool
	AutoSync        bool
}

// BuildPage turns a ViewModel plus Extra into a fully-prepared Page — the
// same role cmd/gui's old screenWidget/gabaritA/gabaritB/screenN functions
// played for walk, just emitting data instead of a widget tree.
func BuildPage(vm state.ViewModel, extra Extra) Page {
	p := Page{
		Screen:         string(vm.Screen),
		LogoURL:        extra.LogoURL,
		A2ToolsLogoURL: extra.A2ToolsLogoURL,
		UpdateBanner:   buildUpdateBanner(extra.Update),
	}

	switch vm.Screen {
	case state.Unlinked, state.Linking, state.Prerequisites:
		p.Gabarit = "a"
		p.SidebarA = &SidebarAView{Steps: buildStepper(vm.Stepper)}
	default:
		p.Gabarit = "b"
		p.SidebarB = buildSidebarB(vm, extra)
	}

	switch vm.Screen {
	case state.Unlinked:
		v := vm.Unlinked
		p.Unlinked = &v
	case state.Linking:
		p.Linking = &linkingView{&vm.Linking}
	case state.Prerequisites:
		p.Prerequisites = buildPrerequisitesView(vm.Prerequisites, extra)
	case state.Ready:
		v := vm.Ready
		p.Ready = &v
	case state.Syncing:
		maxValue := vm.Syncing.Total
		if maxValue <= 0 {
			maxValue = 1
		}
		p.Syncing = &syncingView{&vm.Syncing, maxValue}
	case state.Done:
		p.Done = buildDoneView(vm.Done)
	case state.UpToDate:
		v := vm.UpToDate
		p.UpToDate = &v
	case state.Blocked:
		p.Blocked = buildBlockedView(vm.Blocked)
	case state.Settings:
		p.Settings = buildSettingsView(extra)
	}

	return p
}

func buildUpdateBanner(u UpdateInfo) *UpdateBannerView {
	if !u.Available {
		return nil
	}
	if u.Updating {
		return &UpdateBannerView{
			Updating:      true,
			ProgressLabel: u.Label,
			Percent:       maxInt(u.Percent, 0),
		}
	}
	text := "Une mise à jour (" + u.Version + ") est disponible."
	if u.SyncingNow {
		text += " Disponible une fois la synchro terminée."
	}
	return &UpdateBannerView{
		Text:           text,
		ButtonDisabled: u.SyncingNow,
		ButtonLabel:    "Mettre à jour",
	}
}

func buildStepper(steps []state.StepperStep) []StepperStepView {
	out := make([]StepperStepView, len(steps))
	for i, s := range steps {
		v := StepperStepView{Label: s.Label, Mark: itoa(i + 1)}
		switch s.Status {
		case state.StepDone:
			v.Class, v.Mark = "is-done", "✓"
		case state.StepCurrent:
			v.Class = "is-current"
		}
		out[i] = v
	}
	return out
}

func buildSidebarB(vm state.ViewModel, extra Extra) *SidebarBView {
	characterLabel := "Aucun personnage déclaré"
	if vm.CharacterKnown {
		characterLabel = vm.CharacterSummary
	}

	return &SidebarBView{
		Nav: []NavItemView{
			{Label: "Synchronisation", Active: vm.Screen != state.Settings, Action: "nav-sync"},
			{Label: "Paramètres", Active: vm.Screen == state.Settings, Action: "nav-settings"},
		},
		Status: []StatusLineView{
			{IconURL: extra.LogoURL, Label: "Compte lié", OK: extra.CredentialPresent},
			{IconURL: extra.A2ToolsLogoURL, Label: "DPS Meter A2Tools", OK: extra.DPSMeterFound},
			{Label: characterLabel, OK: vm.CharacterKnown},
		},
	}
}

func buildPrerequisitesView(v state.PrerequisitesView, extra Extra) *prerequisitesView {
	toCard := func(c state.PrereqCard, actionName string) prereqCardView {
		card := prereqCardView{Title: c.Title, Subtitle: c.Subtitle, Done: c.Done, ActionLabel: c.ActionLabel}
		if !c.Done && c.ActionLabel != "" {
			card.ActionName = actionName
		}
		return card
	}

	dpsCard := toCard(v.DPSMeterCard, "open-install-dps-meter")
	if !v.DPSMeterCard.Done {
		dpsCard.ExtraActionLabel = "Choisir le dossier"
	}

	return &prerequisitesView{
		Title:                v.Title,
		AccountCard:          toCard(v.AccountCard, ""),
		DPSMeterCard:         dpsCard,
		CharacterCard:        toCard(v.CharacterCard, "open-declare-character"),
		LaunchAtStartupLabel: v.LaunchAtStartupLabel,
		AutoSyncLabel:        v.AutoSyncLabel,
		FinishButton:         v.FinishButton,
		FinishEnabled:        v.FinishEnabled,
		LaunchAtStartup:      extra.LaunchAtStartup,
		AutoSync:             extra.AutoSync,
	}
}

func buildDoneView(v state.DoneView) *doneView {
	toTile := func(t state.SummaryTile) summaryTileView {
		return summaryTileView{Label: t.Label, Count: t.Count, Warning: t.Warning}
	}
	return &doneView{
		DoneView:                &v,
		AlreadyPresentTile:      toTile(v.AlreadyPresent),
		NoMatchingCharacterTile: toTile(v.NoMatchingCharacter),
		UnreadableTile:          toTile(v.Unreadable),
	}
}

func buildBlockedView(v state.BlockedView) *blockedView {
	return &blockedView{
		BlockedView:     &v,
		ShowPrimary:     v.PrimaryButton != "",
		ShowInstallLink: v.InstallLinkText != "",
	}
}

func buildSettingsView(extra Extra) *settingsView {
	folderLabel := "Dossier du DPS meter · détecté automatiquement"
	if extra.HistoryDirIsCustom {
		folderLabel = "Dossier du DPS meter · personnalisé"
	}
	accountLabel := "Non lié"
	if extra.CredentialPresent {
		accountLabel = "Lié"
	}
	status := "à jour"
	if extra.Update.Available {
		status = "mise à jour disponible"
	}
	return &settingsView{
		FolderLabel:     folderLabel,
		AccountLabel:    "Lié à " + accountLabel,
		VersionLabel:    fmt.Sprintf("Version %s · %s", extra.Version, status),
		LaunchAtStartup: extra.LaunchAtStartup,
		StartMinimized:  extra.StartMinimized,
		AutoSync:        extra.AutoSync,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
