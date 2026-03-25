// Package tuistyles provides shared lipgloss styles for the DevRune TUI.
// It is a leaf package (no imports from other internal packages) so that both
// the tui package and the tui/steps package can import it without cycles.
package tuistyles

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// ColorPurple is the primary DevRune brand color.
	ColorPurple = lipgloss.Color("#9B59B6")
	// ColorCyan is the secondary accent color.
	ColorCyan = lipgloss.Color("#00BCD4")
	// ColorGreen is used for success messages.
	ColorGreen = lipgloss.Color("#27AE60")
	// ColorRed is used for error messages.
	ColorRed = lipgloss.Color("#E74C3C")
	// ColorGray is used for muted/info text.
	ColorGray = lipgloss.Color("#95A5A6")
	// ColorWhite is used for highlighted text.
	ColorWhite = lipgloss.Color("#FDFEFE")

	// Extended gradient palette for the banner.
	ColorDeepPurple = lipgloss.Color("#6B21A8")
	ColorViolet     = lipgloss.Color("#8B5CF6")
	ColorIndigo     = lipgloss.Color("#6366F1")
	ColorSky        = lipgloss.Color("#38BDF8")
	ColorDim        = lipgloss.Color("#999999")

	// StyleTitle renders the main title text.
	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPurple).
			Bold(true)

	// StyleSubtitle renders subtitle or step description text.
	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorCyan).
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
			Foreground(ColorGray)

	// StyleHighlight renders highlighted/important values.
	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	// StyleBanner renders the DevRune ASCII banner.
	StyleBanner = lipgloss.NewStyle().
			Foreground(ColorPurple).
			Bold(true).
			MarginBottom(1)

	// StyleStepIndicator renders the "Step N/M: Name" header.
	StyleStepIndicator = lipgloss.NewStyle().
				Foreground(ColorCyan).
				Bold(true).
				MarginBottom(1)

	// StyleSummaryKey renders a summary label.
	StyleSummaryKey = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	// StyleSummaryValue renders a summary value.
	StyleSummaryValue = lipgloss.NewStyle().
				Foreground(ColorWhite)

	// StyleVersionBadge renders the version tag.
	StyleVersionBadge = lipgloss.NewStyle().
				Foreground(ColorDim)
)

// hexToRGB parses a "#RRGGBB" string to r, g, b components.
func hexToRGB(hex string) (r, g, b int) {
	if len(hex) == 7 && hex[0] == '#' {
		_, _ = fmt.Sscanf(hex[1:], "%02x%02x%02x", &r, &g, &b)
	}
	return
}

// lerpColor interpolates between two hex colors at fraction t (0.0–1.0).
func lerpColor(a, b string, t float64) string {
	r1, g1, b1 := hexToRGB(a)
	r2, g2, b2 := hexToRGB(b)
	r := int(float64(r1) + t*(float64(r2)-float64(r1)))
	g := int(float64(g1) + t*(float64(g2)-float64(g1)))
	bv := int(float64(b1) + t*(float64(b2)-float64(b1)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, bv)
}

// buildGradient creates a slice of n hex color strings interpolated across
// the given color stops.
func buildGradient(n int, stops []string) []string {
	if n <= 0 || len(stops) == 0 {
		return nil
	}
	if len(stops) == 1 || n == 1 {
		return []string{stops[0]}
	}
	colors := make([]string, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n-1) * float64(len(stops)-1)
		idx := int(t)
		if idx >= len(stops)-1 {
			idx = len(stops) - 2
		}
		frac := t - float64(idx)
		colors[i] = lerpColor(stops[idx], stops[idx+1], frac)
	}
	return colors
}

// GradientLine renders a single line of text where each character is colored
// using the provided gradient hex colors. Spaces are passed through uncolored.
func GradientLine(text string, colors []string) string {
	if len(colors) == 0 || len(text) == 0 {
		return text
	}
	runes := []rune(text)
	n := len(runes)
	var b strings.Builder
	for i, r := range runes {
		if r == ' ' {
			b.WriteRune(' ')
			continue
		}
		idx := 0
		if n > 1 {
			idx = i * (len(colors) - 1) / (n - 1)
		}
		if idx >= len(colors) {
			idx = len(colors) - 1
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[idx])).Bold(true)
		b.WriteString(style.Render(string(r)))
	}
	return b.String()
}

// BannerGradient returns a precomputed gradient palette for banner art lines
// (deep purple → violet → indigo → cyan → sky).
func BannerGradient(width int) []string {
	return buildGradient(width, []string{
		"#6B21A8", "#8B5CF6", "#6366F1", "#00BCD4", "#38BDF8",
	})
}

// SubtitleGradient returns a glow-style gradient (cyan → white → cyan).
func SubtitleGradient(width int) []string {
	return buildGradient(width, []string{
		"#00BCD4", "#FDFEFE", "#00BCD4",
	})
}

// ShimmerGradient returns a shimmer gradient for separators (purple → gray → purple).
func ShimmerGradient(width int) []string {
	return buildGradient(width, []string{
		"#6B21A8", "#95A5A6", "#6B21A8",
	})
}
