// Package tuistyles provides shared lipgloss styles for the DevRune TUI.
// It is a leaf package (no imports from other internal packages) so that both
// the tui package and the tui/steps package can import it without cycles.
package tuistyles

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// ANSI color indices — compatible with any terminal theme (Base16, Dracula,
// Solarized, etc.).  These are the only colors used by the DevRune TUI so that
// it harmonises with huh.ThemeBase16's native ANSI palette.
var (
	// ColorGreen is ANSI green (2) — used for success and completed dots.
	ColorGreen = lipgloss.Color("2")
	// ColorRed is ANSI red (1) — used for error messages.
	ColorRed = lipgloss.Color("1")
	// ColorAccent is ANSI bright green (10) — used for titles, current dot, step labels.
	ColorAccent = lipgloss.Color("10")
	// ColorWhite is ANSI white (7) — used for highlighted text.
	ColorWhite = lipgloss.Color("7")
	// ColorDim is ANSI bright-black / gray (8) — used for muted/info text and future dots.
	ColorDim = lipgloss.Color("8")

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
			Foreground(ColorGreen).
			Bold(true)

	// StyleError renders error messages.
	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)

	// StyleInfo renders informational text.
	StyleInfo = lipgloss.NewStyle().
			Foreground(ColorDim)

	// StyleHighlight renders highlighted/important values.
	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	// StyleBanner renders the DevRune ASCII banner.
	StyleBanner = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			MarginBottom(1)

	// StyleStepIndicator renders the "Step N/M: Name" header.
	StyleStepIndicator = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true).
				MarginBottom(1)

	// StyleSummaryKey renders a summary label.
	StyleSummaryKey = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// StyleSummaryValue renders a summary value.
	StyleSummaryValue = lipgloss.NewStyle().
				Foreground(ColorWhite)

	// StyleVersionBadge renders the version tag.
	StyleVersionBadge = lipgloss.NewStyle().
				Foreground(ColorDim)
)

// DevRuneTheme returns a huh theme based on Base16 with button colors
// overridden to use ANSI green/gray instead of the default pink/magenta.
//
//   - FocusedButton: ANSI green (2) background, black (0) foreground — matches the green accent.
//   - BlurredButton: ANSI bright-black/gray (8) background, white (7) foreground — subtle, neutral.
func DevRuneTheme(isDark bool) *huh.Styles {
	t := huh.ThemeBase16(isDark)
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Background(lipgloss.Color("2")).
		Foreground(lipgloss.Color("0"))
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("7"))
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
