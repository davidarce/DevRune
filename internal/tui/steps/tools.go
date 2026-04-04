// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ToolInstallResult holds the outcome of the tool installation step.
type ToolInstallResult struct {
	// Installed lists the names of tools that were successfully installed.
	Installed []string
	// Failed maps tool name → error message for tools that failed to install.
	Failed map[string]string
}

// toolInstallDoneMsg is sent when all parallel installs complete.
type toolInstallDoneMsg struct {
	result ToolInstallResult
}

// toolInstallModel is a Bubbletea model that displays a spinner while tools
// are installed in parallel in the background.
type toolInstallModel struct {
	spinner spinner.Model
	tools   []model.ToolDef
	result  ToolInstallResult
	done    bool
	err     error
}

func newToolInstallModel(tools []model.ToolDef) toolInstallModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // ANSI bright green

	return toolInstallModel{
		spinner: s,
		tools:   tools,
	}
}

func (m toolInstallModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.doInstall(),
	)
}

// doInstall launches all tool installs in parallel and sends a done message.
func (m toolInstallModel) doInstall() tea.Cmd {
	return func() tea.Msg {
		result := installToolsParallel(m.tools)
		return toolInstallDoneMsg{result: result}
	}
}

func (m toolInstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = fmt.Errorf("user aborted")
			return m, tea.Quit
		}
	case toolInstallDoneMsg:
		m.result = msg.result
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m toolInstallModel) View() tea.View {
	if m.done {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	var sb strings.Builder

	sb.WriteString(ResponsiveBanner())
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	toolStep := TotalSteps - 1
	sb.WriteString(tuistyles.StyleStepIndicator.Render(fmt.Sprintf("Step %d/%d: Installing tools", toolStep, TotalSteps)))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(m.spinner.View())
	sb.WriteString(" ")
	sb.WriteString(tuistyles.StyleInfo.Render("Installing selected tools..."))
	sb.WriteString("\n\n")

	for _, td := range m.tools {
		sb.WriteString("  ")
		sb.WriteString(tuistyles.StyleSummaryValue.Render("  " + td.Name))
		sb.WriteString("\n")
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// installToolsParallel runs brew install for each tool concurrently and
// collects results. It uses a WaitGroup and a results channel.
func installToolsParallel(tools []model.ToolDef) ToolInstallResult {
	type installResult struct {
		name string
		err  error
	}

	ch := make(chan installResult, len(tools))
	var wg sync.WaitGroup

	for _, td := range tools {
		wg.Add(1)
		go func(t model.ToolDef) {
			defer wg.Done()
			// Run the install command via shell so that complex commands
			// (e.g. "brew install foo/tap/bar") work correctly.
			cmd := exec.Command("sh", "-c", t.Command) //nolint:gosec
			err := cmd.Run()
			ch <- installResult{name: t.Name, err: err}
		}(td)
	}

	// Close channel after all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	result := ToolInstallResult{Failed: make(map[string]string)}
	for r := range ch {
		if r.err != nil {
			result.Failed[r.name] = r.err.Error()
		} else {
			result.Installed = append(result.Installed, r.name)
		}
	}
	return result
}

// isToolInstalled checks whether a tool binary is already on PATH.
// Returns true when tool.Binary is non-empty and exec.LookPath finds it.
func isToolInstalled(td model.ToolDef) bool {
	if td.Binary == "" {
		return false
	}
	_, err := exec.LookPath(td.Binary)
	return err == nil
}

// RunToolInstallStep presents a multi-select for available tools, then installs
// the selected ones in parallel with a bubbletea spinner.
//
// Tools whose Binary is already on PATH are shown with a ✓ prefix and
// pre-selected. They are reported as installed in the result but are NOT
// re-installed via brew.
//
// If allTools is empty, the step is skipped and an empty result is returned.
// If brew is not available on PATH, the step is silently skipped.
// If the user selects no tools, the step is also skipped.
func RunToolInstallStep(allTools []model.ToolDef) (ToolInstallResult, error) {
	if len(allTools) == 0 {
		return ToolInstallResult{}, nil
	}

	// Silently skip if brew is not on PATH.
	if _, err := exec.LookPath("brew"); err != nil {
		return ToolInstallResult{}, nil
	}

	// Detect which tools are already installed.
	alreadyInstalled := make(map[string]bool, len(allTools))
	for _, td := range allTools {
		if isToolInstalled(td) {
			alreadyInstalled[td.Name] = true
		}
	}

	// Build selection options — already-installed tools get a ✓ prefix.
	options := make([]huh.Option[string], len(allTools))
	for i, td := range allTools {
		label := td.Name
		if td.Description != "" {
			label = fmt.Sprintf("%s — %s", td.Name, td.Description)
		}
		if alreadyInstalled[td.Name] {
			label = "✓ " + label + " (already installed)"
		}
		options[i] = huh.NewOption(label, td.Name).Selected(true)
	}

	var selected []string

	// The tool step is always the penultimate step (before ConfirmSummary).
	toolStep := TotalSteps - 1

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(toolStep, TotalSteps, "Tool installation"),
			huh.NewMultiSelect[string]().
				Title("Select tools to install").
				Description(responsiveDescription(
					"Use space to toggle, enter to confirm. Selected tools will be installed via brew.",
					"Space to toggle, enter to confirm.",
				)).
				Options(options...).
				Height(dynamicHeight(len(allTools)+2)).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return ToolInstallResult{}, err
	}

	if len(selected) == 0 {
		return ToolInstallResult{}, nil
	}

	// Separate selected tools into already-installed (skip brew) and need-install.
	var toInstall []model.ToolDef
	var preInstalled []string
	for _, td := range allTools {
		if !contains(selected, td.Name) {
			continue
		}
		if alreadyInstalled[td.Name] {
			preInstalled = append(preInstalled, td.Name)
		} else {
			toInstall = append(toInstall, td)
		}
	}

	// If everything was already installed, return immediately — no spinner needed.
	if len(toInstall) == 0 {
		return ToolInstallResult{Installed: preInstalled}, nil
	}

	result, err := waitForResult(toInstall)
	if err != nil {
		return ToolInstallResult{}, err
	}

	// Merge pre-installed names into the result.
	result.Installed = append(preInstalled, result.Installed...)
	return result, nil
}

// contains returns true if s is present in the slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// waitForResult runs the bubbletea spinner model that installs tools in parallel
// and returns the aggregated ToolInstallResult.
func waitForResult(tools []model.ToolDef) (ToolInstallResult, error) {
	m := newToolInstallModel(tools)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return ToolInstallResult{}, fmt.Errorf("tool install model: %w", err)
	}

	result, ok := finalModel.(toolInstallModel)
	if !ok {
		return ToolInstallResult{}, fmt.Errorf("tool install model: unexpected model type")
	}

	if result.err != nil {
		return ToolInstallResult{}, result.err
	}

	return result.result, nil
}
