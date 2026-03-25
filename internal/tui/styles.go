package tui

import (
	"strings"

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

// Banner returns the DevRune ASCII art banner string with gradient effect.
// Uses the same rendering as the steps package banner for consistency.
func Banner() string {
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

	subtitleText := "  ░▒▓ AI Agent Configuration Toolkit ▓▒░"
	subtitleGrad := tuistyles.SubtitleGradient(40)
	b.WriteString(tuistyles.GradientLine(subtitleText, subtitleGrad))
	b.WriteString("\n")

	shimmerText := "  ─────────────────────────────────────────────────────────────────"
	shimmerGrad := tuistyles.ShimmerGradient(65)
	b.WriteString(tuistyles.GradientLine(shimmerText, shimmerGrad))

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
