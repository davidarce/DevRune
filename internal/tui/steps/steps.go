// Package steps contains the individual wizard step models for the DevRune TUI.
// Each step is a Bubbletea model responsible for one screen of the wizard:
// agent selection, package input, workflow selection, MCP input, and confirmation.
package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// bannerText is the styled banner rendered once and reused across all steps.
var bannerText = renderBanner()

func renderBanner() string {
	// Block-character ASCII art — double-line box-drawing style like purchaseai.
	artLines := []string{
		`  ██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗`,
		`  ██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝`,
		`  ██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗  `,
		`  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝  `,
		`  ██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗`,
		`  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	gradient := tuistyles.BannerGradient(65)

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range artLines {
		b.WriteString(tuistyles.GradientLine(line, gradient))
		b.WriteString("\n")
	}

	// Subtitle with glow effect (cyan → white → cyan).
	subtitleText := "  ░▒▓ AI Agent Configuration Toolkit ▓▒░"
	subtitleGrad := tuistyles.SubtitleGradient(40)
	b.WriteString(tuistyles.GradientLine(subtitleText, subtitleGrad))
	b.WriteString("\n")

	// Separator shimmer.
	shimmerText := "  ─────────────────────────────────────────────────────────────────"
	shimmerGrad := tuistyles.ShimmerGradient(65)
	b.WriteString(tuistyles.GradientLine(shimmerText, shimmerGrad))

	return b.String()
}

// BannerNote returns a huh.NewNote with just the banner (no step indicator).
// Use this for standalone dialogs that should show the banner but aren't
// part of the numbered wizard steps.
func BannerNote() *huh.Note {
	return huh.NewNote().Description(bannerText)
}

// stepHeader returns a huh.NewNote with the banner and step indicator.
func stepHeader(step, total int, label string) *huh.Note {
	// Progress dots: completed = ● (green), current = ● (cyan), future = ○ (dim).
	var dots []string
	for i := 0; i < total; i++ {
		if i < step-1 {
			dots = append(dots, lipgloss.NewStyle().Foreground(tuistyles.ColorGreen).Render("●"))
		} else if i == step-1 {
			dots = append(dots, lipgloss.NewStyle().Foreground(tuistyles.ColorCyan).Render("●"))
		} else {
			dots = append(dots, lipgloss.NewStyle().Foreground(tuistyles.ColorDim).Render("○"))
		}
	}

	header := fmt.Sprintf("%s\n  %s  %s\n  %s",
		bannerText,
		tuistyles.StyleStepIndicator.Render(
			fmt.Sprintf("Step %d/%d: %s", step, total, label),
		),
		strings.Join(dots, " "),
		lipgloss.NewStyle().Foreground(tuistyles.ColorDim).Render(""),
	)
	return huh.NewNote().Description(header)
}
