// SPDX-License-Identifier: MIT

// Package steps contains the individual wizard step models for the DevRune TUI.
// Each step is a Bubbletea model responsible for one screen of the wizard:
// agent selection, package input, workflow selection, MCP input, and confirmation.
package steps

import (
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	xterm "github.com/charmbracelet/x/term"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ansiStyle is a convenience wrapper around lipgloss.NewStyle with a single
// ANSI foreground color and optional bold.
func ansiStyle(color string, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	if bold {
		s = s.Bold(true)
	}
	return s
}

// TotalSteps controls the total step count shown in the wizard step indicator.
// app.go sets this to 6 when the SDD model selection step is active, 5 otherwise.
var TotalSteps = 5

// termHeight queries the current terminal height.
// Returns 40 as a safe default when the size cannot be determined.
func termHeight() int {
	_, h, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || h <= 0 {
		return 40 // default to full banner
	}
	return h
}

// renderFullBanner builds the full multi-line ASCII art banner string.
// Uses ANSI bright green (10) for a hacker/dev brand color.
// Each line is left-aligned with a 2-space indent.
func renderFullBanner() string {
	artLines := []string{
		`██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗`,
		`██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝`,
		`██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗  `,
		`██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝  `,
		`██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗`,
		`╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	artStyle := ansiStyle("10", true) // ANSI bright green, bold

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range artLines {
		b.WriteString("  ")
		b.WriteString(artStyle.Render(line))
		b.WriteString("\n")
	}

	// Subtitle: plain dim text — no colored blocks.
	subtitleText := "Package manager for AI agent instructions"
	b.WriteString("  ")
	b.WriteString(ansiStyle("8", false).Render(subtitleText))
	b.WriteString("\n")

	// Separator: dim gray.
	shimmerText := "─────────────────────────────────────────────────────────────────"
	b.WriteString("  ")
	b.WriteString(ansiStyle("8", false).Render(shimmerText))

	return b.String()
}

// renderCompactBanner builds a single-line branded banner for medium terminals.
// Uses ANSI bright green (10) to match the full banner style.
// Left-aligned with a 2-space indent.
func renderCompactBanner() string {
	text := "◆ DevRune — Package manager for AI agent instructions"
	return "\n" + "  " + ansiStyle("10", true).Render(text)
}

// responsiveBanner returns the appropriate banner string based on terminal height:
//   - < 25 rows : empty string (no banner)
//   - 25–34 rows: compact single-line banner
//   - ≥ 35 rows : full ASCII art banner
func responsiveBanner() string {
	h := termHeight()
	switch {
	case h < 25:
		return ""
	case h < 35:
		return renderCompactBanner()
	default:
		return renderFullBanner()
	}
}

// BannerNote returns a huh.NewNote with just the banner (no step indicator).
// The banner height is responsive to the current terminal size.
// Use this for standalone dialogs that should show the banner but aren't
// part of the numbered wizard steps.
func BannerNote() *huh.Note {
	return huh.NewNote().Description(responsiveBanner())
}

// stepHeaderString returns the rendered banner + step indicator as a plain string.
// The banner portion is responsive to the current terminal size.
// The step indicator is left-aligned with a 2-space indent.
// This is used when the header needs to be printed directly to stdout rather than
// embedded in a huh.Note (e.g. before a grid-layout form in sdd.go).
func stepHeaderString(step, total int, label string) string {
	var dots []string
	for i := 0; i < total; i++ {
		if i < step-1 {
			// Completed: ANSI green (2)
			dots = append(dots, ansiStyle("2", false).Render("●"))
		} else if i == step-1 {
			// Current: ANSI bright green (10)
			dots = append(dots, ansiStyle("10", false).Render("●"))
		} else {
			// Future: ANSI gray (8)
			dots = append(dots, ansiStyle("8", false).Render("○"))
		}
	}

	stepLabel := tuistyles.StyleStepIndicator.Render(
		fmt.Sprintf("Step %d/%d: %s", step, total, label),
	)
	dotsLine := strings.Join(dots, " ")
	stepLine := "  " + stepLabel + "  " + dotsLine

	return fmt.Sprintf("%s\n%s\n%s",
		responsiveBanner(),
		stepLine,
		"  "+ansiStyle("8", false).Render(""),
	)
}

// stepHeader returns a huh.NewNote with the banner and step indicator.
func stepHeader(step, total int, label string) *huh.Note {
	return huh.NewNote().Description(stepHeaderString(step, total, label))
}
