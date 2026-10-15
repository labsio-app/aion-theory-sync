//go:build windows

package main

import (
	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
)

// Known, deliberate simplifications versus the UX spec's exact visual
// language — called out here once rather than at every call site:
//
//   - Buttons are plain walk.PushButton (native OS chrome): tailscale/walk's
//     declarative PushButton has no text-color field and Background brushes
//     are unreliable on native button controls, which are theme-drawn, not
//     GDI-filled. A pill-shaped accent button with dark text would need an
//     owner-drawn CustomWidget, a substantial addition not worth building
//     blind with no Windows machine to see it on.
//   - Toggles are plain walk.CheckBox, not a drawn 44×24 switch, for the
//     same reason.
//   - Screen 4's progress ring is a horizontal walk.ProgressBar.
//
// Everything that *is* reliable through plain composites/labels — the
// dark palette, card borders, banners, the stepper, text — follows the
// spec's hex values and copy exactly.

// bordered wraps inner in a 1px colored frame: an outer Composite filled
// with borderColor, padded by exactly 1px (VBox's own margin), holding an
// inner Composite filled with fillColor. walk's Composite has no colored-
// border property of its own, so this is the standard way to fake one
// without owner-drawing anything.
func bordered(borderColor, fillColor walk.Color, innerMargins Margins, spacing int, children []Widget) Composite {
	return Composite{
		Background: solid(borderColor),
		Layout:     VBox{Margins: Margins{Left: 1, Top: 1, Right: 1, Bottom: 1}, SpacingZero: true},
		Children: []Widget{
			Composite{
				Background: solid(fillColor),
				Layout:     VBox{Margins: innerMargins, Spacing: spacing},
				Children:   children,
			},
		},
	}
}

func banner(borderColor, bgColor, textColor walk.Color, text string) Widget {
	return bordered(borderColor, bgColor, Margins{Left: 14, Top: 10, Right: 14, Bottom: 10}, 0, []Widget{
		Label{Text: text, Font: fontUI, TextColor: textColor},
	})
}

func warningBanner(text string) Widget {
	return banner(colorWarningEdge, colorWarningBG, colorWarning, text)
}

// primaryButton is the main call-to-action on a screen: 52px tall, accent
// background where walk's chrome allows it, falling back to disabled
// colors when enabled is false — see the package-level simplifications
// note on why this isn't the spec's rounded accent pill.
func primaryButton(text string, enabled bool, onClick func()) PushButton {
	bg := colorAccent
	if !enabled {
		bg = colorDisabledBG
	}
	return PushButton{
		Text:       text,
		Enabled:    enabled,
		MinSize:    Size{Height: 52},
		Background: solid(bg),
		OnClicked:  onClick,
	}
}

func secondaryButton(text string, onClick func()) PushButton {
	return PushButton{Text: text, MinSize: Size{Height: 40}, OnClicked: onClick}
}

func tertiaryLink(text string, onClick func()) LinkLabel {
	return LinkLabel{
		Text: `<a href="#">` + text + `</a>`,
		Font: fontUI,
		OnLinkActivated: func(*walk.LinkLabelLink) {
			if onClick != nil {
				onClick()
			}
		},
	}
}

func urlLink(text, url string, onOpen func(string)) LinkLabel {
	return LinkLabel{
		Text: `<a href="` + url + `">` + text + `</a>`,
		Font: fontUI,
		OnLinkActivated: func(link *walk.LinkLabelLink) {
			onOpen(link.URL())
		},
	}
}

// toggleRow is screens 2 and 8's on/off preference line: label on the
// left, a checkbox standing in for the spec's drawn switch on the right.
func toggleRow(label string, checked bool, onChanged func(bool)) Widget {
	var cb *walk.CheckBox
	return Composite{
		Layout:     HBox{MarginsZero: true, Spacing: 8},
		Background: solid(colorGround),
		Children: []Widget{
			Label{Text: label, Font: fontUI, TextColor: colorText},
			HSpacer{},
			CheckBox{
				AssignTo: &cb,
				Checked:  checked,
				OnCheckedChanged: func() {
					onChanged(cb.Checked())
				},
			},
		},
	}
}

// stepperWidget is gabarit A's vertical 3-step indicator.
func stepperWidget(steps []state.StepperStep) Widget {
	children := make([]Widget, 0, len(steps)*2)
	for i, step := range steps {
		var pillColor, pillText, textColor walk.Color
		label := step.Label
		switch step.Status {
		case state.StepDone:
			pillColor, pillText, textColor = colorSuccess, colorAccentText, colorTextSoft
		case state.StepCurrent:
			pillColor, pillText, textColor = colorGround, colorAccent, colorText
		default:
			pillColor, pillText, textColor = colorGround, colorMuted, colorMuted
		}
		mark := "✓"
		if step.Status != state.StepDone {
			mark = itoa(i + 1)
		}
		children = append(children, Composite{
			Layout:     HBox{MarginsZero: true, Spacing: 10},
			Background: solid(colorSidebarBG),
			Children: []Widget{
				Composite{
					Background: solid(pillColor),
					MinSize:    Size{Width: 28, Height: 28},
					MaxSize:    Size{Width: 28, Height: 28},
					Layout:     VBox{MarginsZero: true, SpacingZero: true, Alignment: AlignHCenterVCenter},
					Children: []Widget{
						Label{Text: mark, Font: fontUISemibold, TextColor: pillText, TextAlignment: AlignCenter},
					},
				},
				Label{Text: label, Font: fontUI, TextColor: textColor},
			},
		})
		if i != len(steps)-1 {
			children = append(children, VSpacer{Size: 12})
		}
	}
	return Composite{
		Layout:     VBox{MarginsZero: true, SpacingZero: true},
		Background: solid(colorSidebarBG),
		Children:   children,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// prereqCardWidget is one of screen 2's three rows. extraActions are
// appended after the card's one ViewModel-described action (c.ActionLabel)
// — screen 2's missing-meter card needs a second, GUI-only action
// ("Choisir le dossier", a native folder picker) that isn't a navigable
// link the ViewModel needs to know about.
func prereqCardWidget(c state.PrereqCard, onAction func(label string), extraActions ...Widget) Widget {
	statusColor := colorWarning
	mark := "!"
	if c.Done {
		statusColor, mark = colorSuccess, "✓"
	}

	children := []Widget{
		Composite{
			Background: solid(statusColor),
			MinSize:    Size{Width: 24, Height: 24},
			MaxSize:    Size{Width: 24, Height: 24},
			Layout:     VBox{MarginsZero: true, SpacingZero: true, Alignment: AlignHCenterVCenter},
			Children:   []Widget{Label{Text: mark, Font: fontUISemibold, TextColor: colorAccentText, TextAlignment: AlignCenter}},
		},
		Composite{
			Layout:     VBox{MarginsZero: true, SpacingZero: true},
			Background: solid(colorCardBG),
			Children: []Widget{
				Label{Text: c.Title, Font: fontUISemibold, TextColor: colorText},
				Label{Text: c.Subtitle, Font: fontSmall, TextColor: colorTextSoft},
			},
		},
		HSpacer{},
	}
	if !c.Done && c.ActionLabel != "" {
		label := c.ActionLabel
		children = append(children, tertiaryLink(label, func() { onAction(label) }))
	}
	children = append(children, extraActions...)

	return bordered(colorCardBorder, colorCardBG, Margins{Left: 14, Top: 10, Right: 14, Bottom: 10}, 0, []Widget{
		Composite{
			Layout:     HBox{MarginsZero: true, Spacing: 12},
			Background: solid(colorCardBG),
			Children:   children,
		},
	})
}
