// SPDX-License-Identifier: MIT

// Package tuistyles provides shared lipgloss styles for the DevRune TUI.
// It is a leaf package (no imports from other internal packages) so that both
// the tui package and the tui/steps package can import it without cycles.
package tuistyles

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// ANSI color indices — ANSI 0-15 only for maximum terminal compatibility.
// These are the only colors used by the DevRune TUI so that it harmonises
// with huh.ThemeBase16's native ANSI palette across iTerm2, Terminal.app,
// Windows Terminal, and Linux VTs.
var (
	// ColorAccent is ANSI bright white (15) — primary accent for titles, labels.
	ColorAccent = lipgloss.Color("15")
	// ColorSecondary is ANSI bright cyan (14) — highlights, step indicators, links.
	ColorSecondary = lipgloss.Color("14")
	// ColorSuccess is ANSI green (2) — checkmarks, success states, completed dots.
	ColorSuccess = lipgloss.Color("2")
	// ColorError is ANSI red (1) — error messages.
	ColorError = lipgloss.Color("1")
	// ColorDim is ANSI bright-black / dark gray (8) — muted text, borders, future dots.
	ColorDim = lipgloss.Color("8")
	// ColorMuted is ANSI light gray (7) — secondary text, summary values.
	ColorMuted = lipgloss.Color("7")
	// ColorBg is ANSI black (0) — backgrounds where needed.
	ColorBg = lipgloss.Color("0")

	// StyleTitle renders the main title text.
	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// StyleSubtitle renders subtitle or step description text.
	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Italic(true)

	// StyleSuccess renders success messages.
	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	// StyleError renders error messages.
	StyleError = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// StyleInfo renders informational text.
	StyleInfo = lipgloss.NewStyle().
			Foreground(ColorDim)

	// StyleHighlight renders highlighted/important values.
	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// StyleBanner renders the DevRune banner.
	StyleBanner = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			MarginBottom(1)

	// StyleStepIndicator renders the "Step N/M: Name" header.
	StyleStepIndicator = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true).
				MarginBottom(1)

	// StyleSummaryKey renders a summary label.
	StyleSummaryKey = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// StyleSummaryValue renders a summary value.
	StyleSummaryValue = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// StyleVersionBadge renders the version tag.
	StyleVersionBadge = lipgloss.NewStyle().
				Foreground(ColorDim)
)

// DevRuneTheme returns a huh theme based on Base16 with button colors
// overridden to use the ANSI-16 neutral palette.
//
//   - FocusedButton: black (0) foreground on bright cyan (14) background — clearly visible primary action.
//   - BlurredButton: light gray (7) foreground on dark gray (8) background — dimmer secondary action.
func DevRuneTheme(isDark bool) *huh.Styles {
	t := huh.ThemeBase16(isDark)
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Background(ColorSecondary).
		Foreground(ColorBg).
		Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Background(ColorDim).
		Foreground(ColorMuted)
	// t.Blurred is a copy of t.Focused set inside ThemeBase16, so button
	// styles on t.Blurred already inherited the Focused values. Override them
	// explicitly to be sure they stay consistent.
	t.Blurred.FocusedButton = t.Focused.FocusedButton
	t.Blurred.BlurredButton = t.Focused.BlurredButton
	return t
}

// DevRuneThemeFunc is a huh.ThemeFunc wrapper around DevRuneTheme.
// Use it wherever huh.ThemeFunc(huh.ThemeBase16) would otherwise appear.
var DevRuneThemeFunc huh.ThemeFunc = DevRuneTheme
