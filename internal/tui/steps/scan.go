// SPDX-License-Identifier: MIT

package steps

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ScanFunc is the function signature for repository scanning.
// It receives a context, source list, and cache path and returns scan results.
type ScanFunc func(ctx context.Context, sources []string, cachePath string) ([]ScanResult, error)

// ScanResult mirrors tui.ScannedRepo but lives in the steps package to avoid
// import cycles. The caller maps between the two.
type ScanResult struct {
	Source    string
	Skills    []string
	Rules     []string
	MCPs      []string
	Workflows []string
	Tools     []model.ToolDef
	Descs     map[string]string
	MCPFiles  map[string]string
	Error     error
}

// scanDoneMsg is sent when the background scan completes.
type scanDoneMsg struct {
	results []ScanResult
	err     error
}

// scanModel is a Bubbletea model that displays a spinner while scanning
// repositories in the background.
type scanModel struct {
	spinner  spinner.Model
	sources  []string
	scanFn   ScanFunc
	cache    string
	results  []ScanResult
	err      error
	done     bool
	warnings []string
}

func newScanModel(sources []string, scanFn ScanFunc, cachePath string) scanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // ANSI bright green

	return scanModel{
		spinner: s,
		sources: sources,
		scanFn:  scanFn,
		cache:   cachePath,
	}
}

func (m scanModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.doScan(),
	)
}

func (m scanModel) doScan() tea.Cmd {
	return func() tea.Msg {
		results, err := m.scanFn(context.Background(), m.sources, m.cache)
		return scanDoneMsg{results: results, err: err}
	}
}

func (m scanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = fmt.Errorf("user aborted")
			return m, tea.Quit
		}
	case scanDoneMsg:
		m.results = msg.results
		m.err = msg.err
		m.done = true

		// Collect warnings from individual repo errors.
		if msg.err == nil {
			for _, r := range msg.results {
				if r.Error != nil {
					m.warnings = append(m.warnings, fmt.Sprintf("  Warning: %s — %v", r.Source, r.Error))
				}
			}
		}

		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m scanModel) View() tea.View {
	if m.done {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	var sb strings.Builder

	sb.WriteString(responsiveBanner())
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(tuistyles.StyleStepIndicator.Render(fmt.Sprintf("Step 3/%d: Select content", TotalSteps)))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(m.spinner.View())
	sb.WriteString(" ")
	sb.WriteString(tuistyles.StyleInfo.Render("Scanning repositories..."))
	sb.WriteString("\n\n")

	// Show which sources are being scanned.
	for _, src := range m.sources {
		sb.WriteString("  ")
		sb.WriteString(tuistyles.StyleSummaryValue.Render("  " + src))
		sb.WriteString("\n")
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// RunScanModel runs the scanning spinner within an alt-screen bubbletea program.
// It returns the scan results (with per-repo errors embedded) and any warnings
// that should be shown to the user.
func RunScanModel(sources []string, scanFn ScanFunc, cachePath string) ([]ScanResult, []string, error) {
	m := newScanModel(sources, scanFn, cachePath)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("scan model: %w", err)
	}

	result, ok := finalModel.(scanModel)
	if !ok {
		return nil, nil, fmt.Errorf("scan model: unexpected model type")
	}

	if result.err != nil {
		return nil, nil, result.err
	}

	return result.results, result.warnings, nil
}
