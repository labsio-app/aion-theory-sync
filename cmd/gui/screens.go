//go:build windows

package main

import (
	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
	"github.com/labsio-app/aion-theory-sync/internal/version"
)

// screenWidget is the one place that turns a ViewModel into a widget
// tree: every event handler calls a.render(), which calls this and nothing
// else, so there is no business "if" anywhere outside internal/ui/state —
// only "which gabarit, which fields" layout decisions.
func (a *app) screenWidget(vm state.ViewModel) Widget {
	switch vm.Screen {
	case state.Unlinked, state.Linking, state.Prerequisites:
		return a.gabaritA(vm)
	default:
		return a.gabaritB(vm)
	}
}

// --- Gabarit A: onboarding (screens 0-2) ----------------------------------

func (a *app) gabaritA(vm state.ViewModel) Widget {
	var content []Widget
	switch vm.Screen {
	case state.Unlinked:
		content = a.screen0(vm.Unlinked)
	case state.Linking:
		content = a.screen1(vm.Linking)
	case state.Prerequisites:
		content = a.screen2(vm.Prerequisites)
	}

	return Composite{
		Layout:     HBox{MarginsZero: true, SpacingZero: true},
		Background: solid(colorGround),
		Children: []Widget{
			Composite{
				Background: solid(colorSidebarBG),
				MinSize:    Size{Width: 320},
				MaxSize:    Size{Width: 320},
				Layout:     VBox{Margins: Margins{Left: 40, Top: 40, Right: 32, Bottom: 40}, Spacing: 16},
				Children: []Widget{
					ImageView{Image: a.logo, MinSize: Size{Width: 88, Height: 88}, MaxSize: Size{Width: 88, Height: 88}, Mode: ImageViewModeShrink},
					Label{Text: "Aion Theory Sync", Font: fontTitleMedium, TextColor: colorText},
					Label{Text: "Tes combats du DPS meter, rattachés à ton personnage sur Aion Theory.", Font: fontSmall, TextColor: colorMuted},
					VSpacer{Size: 8},
					stepperWidget(vm.Stepper),
					VSpacer{},
				},
			},
			Composite{
				Background: solid(colorGround),
				Layout:     VBox{Margins: Margins{Left: 56, Top: 56, Right: 56, Bottom: 56}, Spacing: 14},
				Children:   content,
			},
		},
	}
}

func (a *app) screen0(v state.UnlinkedView) []Widget {
	children := []Widget{}
	if v.ExpiredBanner {
		children = append(children, warningBanner(v.ExpiredBannerText))
	}
	if v.RevokedBanner {
		children = append(children, warningBanner(v.RevokedBannerText))
	}
	children = append(children,
		Label{Text: v.Title, Font: fontTitleLarge, TextColor: colorText},
		Label{Text: v.Body, Font: fontUI, TextColor: colorTextSoft},
		VSpacer{},
		Composite{
			Layout:     HBox{MarginsZero: true, Spacing: 16},
			Background: solid(colorGround),
			Children: []Widget{
				primaryButton(v.PrimaryButton, true, a.onLinkRequested),
				Label{Text: v.Hint, Font: fontSmall, TextColor: colorMuted},
				HSpacer{},
			},
		},
	)
	return children
}

func (a *app) screen1(v state.LinkingView) []Widget {
	return []Widget{
		Label{Text: v.Title, Font: fontTitleLarge, TextColor: colorText},
		Label{Text: v.Subtitle, Font: fontUI, TextColor: colorTextSoft},
		bordered(colorCardBorder, colorCardBG, Margins{Left: 24, Top: 24, Right: 24, Bottom: 24}, 0, []Widget{
			Composite{
				Layout:     HBox{MarginsZero: true, Spacing: 12},
				Background: solid(colorCardBG),
				Children: []Widget{
					Label{Text: v.UserCode, Font: fontCode, TextColor: colorText},
					HSpacer{},
					secondaryButton(v.CopyButton, func() { _ = walk.Clipboard().SetText(v.UserCode) }),
				},
			},
		}),
		Composite{
			Layout:     HBox{MarginsZero: true, Spacing: 8},
			Background: solid(colorGround),
			Children: []Widget{
				Composite{Background: solid(colorAccent), MinSize: Size{Width: 8, Height: 8}, MaxSize: Size{Width: 8, Height: 8}},
				Label{Text: v.WaitingText, Font: fontSmall, TextColor: colorTextSoft},
				HSpacer{},
			},
		},
		VSpacer{},
		Composite{
			Layout:     HBox{MarginsZero: true, Spacing: 16},
			Background: solid(colorGround),
			Children: []Widget{
				primaryButton(v.ReopenButton, true, func() { openBrowser(v.VerificationURL) }),
				tertiaryLink(v.CancelText, a.onCancelLinking),
				HSpacer{},
			},
		},
	}
}

func (a *app) screen2(v state.PrerequisitesView) []Widget {
	children := []Widget{
		Label{Text: v.Title, Font: fontTitleLarge, TextColor: colorText},
		prereqCardWidget(v.AccountCard, nil),
	}

	var dpsExtra []Widget
	if !v.DPSMeterCard.Done {
		dpsExtra = append(dpsExtra, tertiaryLink("Choisir le dossier", func() { a.onChooseFolder(false) }))
	}
	children = append(children, prereqCardWidget(v.DPSMeterCard, func(label string) {
		if label == v.DPSMeterCard.ActionLabel {
			openBrowser(v.DPSMeterCard.ActionURL)
		}
	}, dpsExtra...))

	children = append(children,
		HSeparator{MaxSize: Size{Height: 1}, Background: solid(colorSeparator)},
		toggleRow(v.LaunchAtStartupLabel, a.settings.LaunchAtStartup, func(checked bool) {
			a.settings.LaunchAtStartup = checked
			a.applyAutostart()
			a.saveSettings()
		}),
		toggleRow(v.AutoSyncLabel, a.settings.AutoSync, func(checked bool) {
			a.settings.AutoSync = checked
			a.saveSettings()
		}),
		VSpacer{},
		Composite{
			Layout:     HBox{MarginsZero: true},
			Background: solid(colorGround),
			Children:   []Widget{HSpacer{}, primaryButton(v.FinishButton, v.FinishEnabled, a.onFinishPrerequisites)},
		},
	)
	return children
}

// --- Gabarit B: usage (screens 3-8) ---------------------------------------

func (a *app) gabaritB(vm state.ViewModel) Widget {
	var content []Widget
	switch vm.Screen {
	case state.Ready:
		content = a.screen3(vm.Ready)
	case state.Syncing:
		content = a.screen4(vm.Syncing)
	case state.Done:
		content = a.screen5(vm.Done)
	case state.UpToDate:
		content = a.screen6(vm.UpToDate)
	case state.Blocked:
		content = a.screen7(vm.Blocked)
	case state.Settings:
		content = a.screen8()
	}

	return Composite{
		Layout:     HBox{MarginsZero: true, SpacingZero: true},
		Background: solid(colorGround),
		Children: []Widget{
			a.sidebarB(vm),
			Composite{
				Background: solid(colorGround),
				Layout:     VBox{Margins: Margins{Left: 48, Top: 40, Right: 48, Bottom: 40}, Spacing: 12},
				Children:   content,
			},
		},
	}
}

func (a *app) sidebarB(vm state.ViewModel) Widget {
	navItem := func(label string, active bool, onClick func()) Widget {
		bg, barColor := colorSidebarBG, colorSidebarBG
		if active {
			bg, barColor = colorNavActiveBG, colorAccent
		}
		return Composite{
			Background: solid(bg),
			MinSize:    Size{Height: 40},
			Layout:     HBox{MarginsZero: true, Spacing: 10},
			Children: []Widget{
				Composite{Background: solid(barColor), MinSize: Size{Width: 3}, MaxSize: Size{Width: 3}},
				tertiaryLink(label, onClick),
				HSpacer{},
			},
		}
	}

	statusLine := func(logo walk.Image, label string, ok bool) Widget {
		dot := colorWarning
		if ok {
			dot = colorSuccess
		}
		children := []Widget{}
		if logo != nil {
			children = append(children, ImageView{Image: logo, MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeShrink})
		}
		children = append(children,
			Label{Text: label, Font: fontSmall, TextColor: colorTextSoft},
			HSpacer{},
			Composite{Background: solid(dot), MinSize: Size{Width: 8, Height: 8}, MaxSize: Size{Width: 8, Height: 8}},
		)
		return Composite{
			Background: solid(colorSidebarBG),
			MinSize:    Size{Height: 20},
			Layout:     HBox{MarginsZero: true, Spacing: 6},
			Children:   children,
		}
	}

	return Composite{
		Background: solid(colorSidebarBG),
		MinSize:    Size{Width: 280},
		MaxSize:    Size{Width: 280},
		Layout:     VBox{Margins: Margins{Left: 24, Top: 24, Right: 24, Bottom: 24}, Spacing: 4},
		Children: []Widget{
			Composite{
				Layout:     HBox{MarginsZero: true, Spacing: 8},
				Background: solid(colorSidebarBG),
				Children: []Widget{
					ImageView{Image: a.logo, MinSize: Size{Width: 40, Height: 40}, MaxSize: Size{Width: 40, Height: 40}, Mode: ImageViewModeShrink},
					Label{Text: "Aion Theory Sync", Font: fontUISemibold, TextColor: colorText},
				},
			},
			VSpacer{Size: 16},
			navItem("Synchronisation", vm.Screen != state.Settings, a.onNavSync),
			navItem("Paramètres", vm.Screen == state.Settings, a.onNavSettings),
			VSpacer{},
			statusLine(a.logo, "Compte lié", a.credential != ""),
			statusLine(a.a2toolsLogo, "DPS Meter A2Tools", a.machine.DPSMeter.Found),
		},
	}
}

func (a *app) screen3(v state.ReadyView) []Widget {
	return []Widget{
		Label{Text: v.LastSyncedLabel, Font: fontSmall, TextColor: colorMuted},
		VSpacer{},
		Label{Text: itoa(v.PendingCount), Font: fontHero, TextColor: colorText, TextAlignment: AlignCenter},
		Label{Text: v.PendingLabel, Font: fontUI, TextColor: colorTextSoft, TextAlignment: AlignCenter},
		VSpacer{},
		primaryButton(v.SyncNowButton, true, a.onSyncNow),
	}
}

func (a *app) screen4(v state.SyncingView) []Widget {
	maxValue := v.Total
	if maxValue <= 0 {
		maxValue = 1
	}
	children := []Widget{
		VSpacer{},
		ProgressBar{MinValue: 0, MaxValue: maxValue, Value: v.Done, MinSize: Size{Height: 16}},
		Label{Text: itoa(v.Done) + "/" + itoa(v.Total), Font: fontTitleMedium, TextColor: colorText, TextAlignment: AlignCenter},
		Label{Text: v.StatusText, Font: fontUI, TextColor: colorTextSoft, TextAlignment: AlignCenter},
	}
	if v.CurrentFileLabel != "" {
		children = append(children, Label{Text: v.CurrentFileLabel, Font: fontSmall, TextColor: colorMuted, TextAlignment: AlignCenter})
	}
	if v.RateLimitActive {
		children = append(children, warningBanner(v.RateLimitText))
	}
	children = append(children, VSpacer{}, Composite{
		Layout:     HBox{MarginsZero: true},
		Background: solid(colorGround),
		Children:   []Widget{HSpacer{}, secondaryButton(v.PauseResumeButton, a.onTogglePause)},
	})
	return children
}

func (a *app) screen5(v state.DoneView) []Widget {
	children := []Widget{
		Label{Text: "✓", Font: fontTitleLarge, TextColor: colorSuccess, TextAlignment: AlignCenter},
		Label{Text: v.Title, Font: fontTitleLarge, TextColor: colorText, TextAlignment: AlignCenter},
	}
	if v.SubtitleVisible {
		children = append(children, Label{Text: v.Subtitle, Font: fontUI, TextColor: colorTextSoft, TextAlignment: AlignCenter})
	}
	children = append(children, Composite{
		Layout:     HBox{MarginsZero: true, Spacing: 12},
		Background: solid(colorGround),
		Children: []Widget{
			summaryTile(v.AlreadyPresent),
			summaryTile(v.NoMatchingCharacter),
			summaryTile(v.Unreadable),
		},
	})
	if v.FollowupVisible {
		children = append(children, Label{Text: v.FollowupText, Font: fontSmall, TextColor: colorTextSoft})
	}
	if v.FailedBannerVisible {
		children = append(children, warningBanner(v.FailedBannerText))
	}
	children = append(children, VSpacer{}, Composite{
		Layout:     HBox{MarginsZero: true, Spacing: 12},
		Background: solid(colorGround),
		Children: []Widget{
			HSpacer{},
			secondaryButton(v.CloseButton, a.onCloseDone),
			primaryButton(v.ViewOnlineButton, true, func() { openBrowser(defaultServerURL) }),
		},
	})
	return children
}

func summaryTile(t state.SummaryTile) Widget {
	border := colorCardBorder
	if t.Warning {
		border = colorWarningEdge
	}
	return bordered(border, colorCardBG, Margins{Left: 16, Top: 12, Right: 16, Bottom: 12}, 4, []Widget{
		Label{Text: itoa(t.Count), Font: fontTitleMedium, TextColor: colorText, TextAlignment: AlignCenter},
		Label{Text: t.Label, Font: fontSmall, TextColor: colorTextSoft, TextAlignment: AlignCenter},
	})
}

func (a *app) screen6(v state.UpToDateView) []Widget {
	return []Widget{
		Label{Text: v.LastSyncedLabel, Font: fontSmall, TextColor: colorMuted},
		VSpacer{},
		Label{Text: "✓", Font: fontTitleLarge, TextColor: colorSuccess, TextAlignment: AlignCenter},
		Label{Text: v.Title, Font: fontTitleMedium, TextColor: colorText, TextAlignment: AlignCenter},
		Label{Text: v.Body, Font: fontUI, TextColor: colorTextSoft, TextAlignment: AlignCenter},
		VSpacer{},
		primaryButton(v.SyncNowButton, !v.SyncNowDisabled, func() {}),
	}
}

func (a *app) screen7(v state.BlockedView) []Widget {
	children := []Widget{
		Label{Text: "!", Font: fontTitleLarge, TextColor: colorWarning, TextAlignment: AlignCenter},
		Label{Text: v.Title, Font: fontTitleLarge, TextColor: colorText},
		Label{Text: v.Body, Font: fontUI, TextColor: colorTextSoft},
		VSpacer{},
	}

	var row []Widget
	if v.PrimaryButton != "" {
		row = append(row, primaryButton(v.PrimaryButton, true, func() { a.onChooseFolder(true) }))
	}
	row = append(row, secondaryButton(v.RetryButton, a.onRetryBlocked))
	if v.InstallLinkText != "" {
		row = append(row, urlLink(v.InstallLinkText, "https://a2tools.app", openBrowser))
	}
	row = append(row, HSpacer{}, tertiaryLink(v.TechnicalLogLink, a.onOpenTechnicalLog))

	children = append(children, Composite{Layout: HBox{MarginsZero: true, Spacing: 16}, Background: solid(colorGround), Children: row})
	return children
}

func (a *app) screen8() []Widget {
	folderLabel := "Dossier du DPS meter · détecté automatiquement"
	if a.settings.HistoryDirIsCustom {
		folderLabel = "Dossier du DPS meter · personnalisé"
	}

	accountLabel := "Non lié"
	if a.credential != "" {
		accountLabel = "Lié"
	}

	return []Widget{
		Label{Text: "Paramètres", Font: fontTitleLarge, TextColor: colorText},
		VSpacer{Size: 4},
		sectionHeader("DÉMARRAGE"),
		toggleRow("Lancer au démarrage de Windows", a.settings.LaunchAtStartup, func(checked bool) {
			a.settings.LaunchAtStartup = checked
			a.applyAutostart()
			a.saveSettings()
		}),
		HSeparator{MaxSize: Size{Height: 1}, Background: solid(colorSeparator)},
		toggleRow("Démarrer réduit dans la zone de notification", a.settings.StartMinimized, func(checked bool) {
			a.settings.StartMinimized = checked
			a.applyAutostart()
			a.saveSettings()
		}),
		VSpacer{Size: 12},
		sectionHeader("SYNCHRONISATION"),
		toggleRow("Envoyer automatiquement les nouveaux combats", a.settings.AutoSync, func(checked bool) {
			a.settings.AutoSync = checked
			a.saveSettings()
		}),
		HSeparator{MaxSize: Size{Height: 1}, Background: solid(colorSeparator)},
		settingsLinkRow(a.a2toolsLogo, folderLabel, "Modifier", func() { a.onChooseFolder(false) }),
		VSpacer{Size: 12},
		sectionHeader("COMPTE"),
		settingsLinkRow(a.logo, "Lié à "+accountLabel, "Délier ce PC", a.onUnlink),
		VSpacer{},
		Composite{
			Layout:     HBox{MarginsZero: true},
			Background: solid(colorGround),
			Children: []Widget{
				tertiaryLink("Ouvrir le journal technique", a.onOpenTechnicalLog),
				HSpacer{},
				Label{Text: versionLabel(version.Version, a.updateAvailable), Font: fontSmall, TextColor: colorMuted},
			},
		},
	}
}

func sectionHeader(text string) Widget {
	return Label{Text: text, Font: fontSmall, TextColor: colorMuted}
}

// settingsLinkRow is one of screen 8's 44px rows: an optional logo, a
// label, and a trailing action link. The action always renders in the
// SysLink control's own system link color — walk's LinkLabel wraps the
// native Windows SysLink control, which only understands a bare <a> tag,
// not arbitrary styling, so "Délier ce PC"'s orange from the spec isn't
// reproduced here (another of the simplifications widgets.go calls out).
func settingsLinkRow(logo walk.Image, label, action string, onClick func()) Widget {
	children := []Widget{}
	if logo != nil {
		children = append(children, ImageView{Image: logo, MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeShrink})
	}
	children = append(children,
		Label{Text: label, Font: fontUI, TextColor: colorText},
		HSpacer{},
		tertiaryLink(action, onClick),
	)
	return Composite{
		MinSize:    Size{Height: 44},
		Layout:     HBox{MarginsZero: true, Spacing: 8},
		Background: solid(colorGround),
		Children:   children,
	}
}
