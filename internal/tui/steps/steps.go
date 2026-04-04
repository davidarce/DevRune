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
func termSize() (int, int) {
	w, h, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 || h <= 0 {
		return 80, 40 // defaults
	}
	return w, h
}

func termHeight() int {
	_, h := termSize()
	return h
}

// renderFullBanner builds the full multi-line ASCII art banner string.
// Uses ANSI bright cyan (14) for the art, with subtitle and separator in dim gray (8).
func renderFullBanner() string {
	artLines := []string{
		`██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗`,
		`██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝`,
		`██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗  `,
		`██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝  `,
		`██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗`,
		`╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	artStyle := ansiStyle("14", true) // ANSI bright cyan, bold
	dimStyle := ansiStyle("8", false) // ANSI dark gray

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range artLines {
		b.WriteString("  ")
		b.WriteString(artStyle.Render(line))
		b.WriteString("\n")
	}

	// Subtitle.
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("Package manager for AI agent instructions"))
	b.WriteString("\n")

	// Separator.
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("─────────────────────────────────────────────────────────────────"))

	return b.String()
}

// renderMinimalBanner builds a minimal 2-line banner for medium-large terminals.
// Used inside wizard steps where the full ASCII art would take too much space.
func renderMinimalBanner() string {
	titleStyle := ansiStyle("15", true)
	dimStyle := ansiStyle("8", false)
	cyanStyle := ansiStyle("14", false)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(cyanStyle.Render("◆ "))
	b.WriteString(titleStyle.Render("DevRune"))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("Package manager for AI agent instructions"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("─────────────────────────────────────────────────────────────────"))

	return b.String()
}

// renderCompactBanner builds a single-line branded banner for medium terminals.
// Uses ANSI bright white (15) to match the full banner style.
// Left-aligned with a 2-space indent.
func renderCompactBanner() string {
	return "\n" + "  " + ansiStyle("14", false).Render("◆ ") + ansiStyle("15", true).Render("DevRune")
}

// responsiveBanner returns the appropriate banner string based on terminal size:
//   - too small (h < 25): empty string
//   - medium (h 25-34 or w < 70): compact single-line banner
//   - large (h >= 35 and w >= 70): full ASCII art banner
func responsiveBanner() string {
	w, h := termSize()
	switch {
	case h < 25:
		return ""
	case h < 35 || w < 70:
		return renderCompactBanner()
	default:
		return renderFullBanner()
	}
}

// responsiveStepBanner returns a banner for wizard steps (minimal, not the full ASCII art).
// The full ASCII art is reserved for the main menu via responsiveBanner/BannerNote.
func responsiveStepBanner() string {
	h := termHeight()
	switch {
	case h < 25:
		return ""
	case h < 35:
		return renderCompactBanner()
	default:
		return renderMinimalBanner()
	}
}

// BannerNote returns a huh.NewNote with just the banner (no step indicator).
// The banner height is responsive to the current terminal size.
// Use this for standalone dialogs that should show the banner but aren't
// part of the numbered wizard steps.
func BannerNote() *huh.Note {
	return huh.NewNote().Title(responsiveBanner())
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
			// Current: ANSI bright cyan (14)
			dots = append(dots, ansiStyle("14", false).Render("●"))
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
		responsiveStepBanner(),
		stepLine,
		"  "+ansiStyle("8", false).Render(""),
	)
}

// stepHeader returns a huh.NewNote with the banner and step indicator.
func stepHeader(step, total int, label string) *huh.Note {
	return huh.NewNote().Description(stepHeaderString(step, total, label))
}
