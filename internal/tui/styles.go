package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// Re-export shared styles for backward compatibility within the tui package.
var (
	StyleTitle         = tuistyles.StyleTitle
	StyleSubtitle      = tuistyles.StyleSubtitle
	StyleSuccess       = tuistyles.StyleSuccess
	StyleError         = tuistyles.StyleError
	StyleInfo          = tuistyles.StyleInfo
	StyleHighlight     = tuistyles.StyleHighlight
	StyleBanner        = tuistyles.StyleBanner
	StyleStepIndicator = tuistyles.StyleStepIndicator
	StyleSummaryKey    = tuistyles.StyleSummaryKey
	StyleSummaryValue  = tuistyles.StyleSummaryValue
)

// Banner returns the DevRune ASCII art banner string.
// Uses ANSI bright green (10) for a hacker/dev brand color.
func Banner() string {
	artLines := []string{
		`  ██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗`,
		`  ██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝`,
		`  ██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗  `,
		`  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝  `,
		`  ██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗`,
		`  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	artStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // ANSI bright green
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))            // ANSI gray

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range artLines {
		b.WriteString(artStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("  AI Agent Configuration Toolkit"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ─────────────────────────────────────────────────────────────────"))

	return b.String()
}

// StepIndicator returns a formatted "Step N/M: Label" string.
func StepIndicator(current, total int, label string) string {
	return StyleStepIndicator.Render(
		"Step " + itoa(current) + "/" + itoa(total) + ": " + label,
	)
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n >= 10 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	pos--
	buf[pos] = byte('0' + n)
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
