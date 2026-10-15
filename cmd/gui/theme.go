//go:build windows

package main

import (
	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

// Palette, exactly the hex values the UX spec names.
var (
	colorGround      = walk.RGB(0x08, 0x07, 0x1a) // fond principal
	colorSidebarBG   = walk.RGB(0x0e, 0x0c, 0x24) // fond barre latérale
	colorCardBG      = walk.RGB(0x12, 0x0f, 0x2b) // fond carte
	colorCardBorder  = walk.RGB(0x24, 0x1f, 0x45) // bordure carte
	colorSeparator   = walk.RGB(0x1e, 0x1a, 0x3d) // séparateur
	colorText        = walk.RGB(0xf2, 0xee, 0xfa) // texte principal
	colorTextSoft    = walk.RGB(0xcf, 0xc7, 0xe6) // texte secondaire
	colorMuted       = walk.RGB(0x9a, 0x91, 0xb8) // texte discret
	colorAccent      = walk.RGB(0x6b, 0xd5, 0xff) // accent
	colorAccentText  = walk.RGB(0x04, 0x03, 0x0e) // texte sur fond accent
	colorSuccess     = walk.RGB(0x5f, 0xd9, 0x8a)
	colorWarning     = walk.RGB(0xff, 0x9d, 0x6b)
	colorWarningBG   = walk.RGB(0x2a, 0x1a, 0x12)
	colorWarningEdge = walk.RGB(0x6b, 0x4a, 0x30)
	colorNavActiveBG = walk.RGB(0x1c, 0x18, 0x40)

	// colorSecondaryEdge (bordure bouton secondaire, #3A3466) has no
	// consumer: walk.PushButton is a native, theme-drawn control with no
	// border-color field (see widgets.go's simplifications note). Kept
	// here so the full spec palette stays documented in one place.
	colorSecondaryEdge = walk.RGB(0x3a, 0x34, 0x66)

	// Disabled "Synchroniser maintenant" on screen 6, per the spec.
	colorDisabledBG   = walk.RGB(0x16, 0x13, 0x2f)
	colorDisabledText = walk.RGB(0x8f, 0x87, 0xae)
)

// Fonts. The spec asks for Segoe UI Variable / Segoe UI Variable Display /
// Cascadia Mono — all native Windows fonts (Segoe UI Variable since the
// Windows 11 22H2 MCDM update, Cascadia Mono shipped with Windows Terminal
// and, since Windows 11, with the OS itself), so nothing is embedded. A
// system lacking a listed family silently falls back to its next one
// (how GDI font substitution already behaves), rather than failing to
// render.
var (
	fontUI          = Font{Family: "Segoe UI Variable Text", PointSize: 10}
	fontUISemibold  = Font{Family: "Segoe UI Variable Text", PointSize: 10, SemiBold: true}
	fontSmall       = Font{Family: "Segoe UI Variable Small", PointSize: 9}
	fontTitleLarge  = Font{Family: "Segoe UI Variable Display", PointSize: 20, SemiBold: true}
	fontTitleMedium = Font{Family: "Segoe UI Variable Display", PointSize: 16, SemiBold: true}
	fontHero        = Font{Family: "Segoe UI Variable Display", PointSize: 44, SemiBold: true}
	fontCode        = Font{Family: "Cascadia Mono", PointSize: 22, Bold: true}
)

func solid(c walk.Color) SolidColorBrush { return SolidColorBrush{Color: c} }
