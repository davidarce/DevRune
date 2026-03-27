// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// CompletionInfo holds the data to display on the completion screen.
type CompletionInfo struct {
	Agents    []string
	Packages  int
	MCPs      int
	Workflows int
	Skills    int
	Rules     int
	Manifest  string
	Lockfile  string
}

// completionModel is a Bubbletea model that renders the completion screen
// and waits for Q or Ctrl-C to exit.
type completionModel struct {
	info CompletionInfo
}

func (m completionModel) Init() tea.Cmd {
	return nil
}

func (m completionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m completionModel) View() tea.View {
	var sb strings.Builder

	// Box styles.
	boxBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")). // ANSI green
		Padding(1, 3).
		MarginTop(1).
		MarginLeft(2)

	// Title.
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")). // ANSI green
		Bold(true).
		Render("  Installation Complete!")

	// Divider.
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // ANSI gray
		Render("  ─────────────────────────────────────")

	// Summary lines.
	var lines []string
	lines = append(lines, fmtLine("Agents", strings.Join(m.info.Agents, ", ")))
	if m.info.Packages > 0 {
		lines = append(lines, fmtLine("Repos", fmt.Sprintf("%d", m.info.Packages)))
	}
	if m.info.MCPs > 0 {
		lines = append(lines, fmtLine("MCPs", fmt.Sprintf("%d", m.info.MCPs)))
	}
	if m.info.Workflows > 0 {
		lines = append(lines, fmtLine("Workflows", fmt.Sprintf("%d", m.info.Workflows)))
	}
	if m.info.Skills > 0 {
		lines = append(lines, fmtLine("Skills", fmt.Sprintf("%d", m.info.Skills)))
	}
	if m.info.Rules > 0 {
		lines = append(lines, fmtLine("Rules", fmt.Sprintf("%d", m.info.Rules)))
	}
	lines = append(lines, "")
	lines = append(lines, fmtLine("Manifest", m.info.Manifest))
	lines = append(lines, fmtLine("Lockfile", m.info.Lockfile))

	boxContent := strings.Join(lines, "\n")
	box := boxBorder.Render(boxContent)

	// Footer hint.
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // ANSI gray
		Italic(true).
		MarginLeft(4).
		MarginTop(1).
		Render("Press q to exit")

	sb.WriteString(Banner())
	sb.WriteString("\n\n")
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n")
	sb.WriteString(box)
	sb.WriteString("\n")
	sb.WriteString(hint)
	sb.WriteString("\n\n")

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// fmtLine formats a key-value pair for the summary.
func fmtLine(key, value string) string {
	padded := fmt.Sprintf("%-12s", key+":")
	return tuistyles.StyleSummaryKey.Render(padded) + tuistyles.StyleSummaryValue.Render(value)
}

// RunCompletion shows the interactive completion screen and blocks until Q is pressed.
func RunCompletion(info CompletionInfo) error {
	m := completionModel{info: info}
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
