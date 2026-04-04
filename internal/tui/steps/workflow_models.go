// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	xterm "github.com/charmbracelet/x/term"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// workflowModelSelectHeight is the number of visible rows for each model Select field.
// Set to show all options without scrolling (inherit sentinel + haiku + sonnet + opus = 4).
const workflowModelSelectHeight = 7

// workflowGridMinHeight is the minimum terminal height required to display role
// groups in a grid layout without clipping. Below this threshold the layout
// falls back to LayoutColumns(2) which paginates groups at a time.
const workflowGridMinHeight = 35

// cardGap is the horizontal gap between agent cards (sequential fallback layout).
const cardGap = 4

// colMinWidth is the minimum per-agent column width for the column layout.
const colMinWidth = 20

// colMaxWidth is the maximum per-agent column width for the column layout.
const colMaxWidth = 35

// colTermMinWidth is the minimum terminal width required to use the column layout.
// Below this threshold the layout falls back to sequential (one agent per line).
const colTermMinWidth = 60

// separatorWidth is the rendered width of the inter-column separator " │ ".
const separatorWidth = 3

// AgentModelConfig holds the agent name and its selectable model options for TUI form building.
// Name is the agent identifier (e.g. "claude", "opencode").
type AgentModelConfig struct {
	Name         string
	ModelOptions []model.ModelOption
}

// openCodeFallbackModels is the static fallback list used when LoadOpenCodeModels
// cannot read the OpenCode models cache (file not found, bad JSON, etc.).
var openCodeFallbackModels = []string{
	"claude-sonnet-4.5",
	"claude-opus-4.5",
	"gpt-4o",
	"gpt-4o-mini",
}

// modelPickedMsg is sent back to the main program after the user selects a model
// in the full-screen huh.Select picker.
type modelPickedMsg struct {
	phaseIdx int
	agentIdx int
	value    int // index into options
	err      error
}

// huhSelectCommand implements tea.ExecCommand and runs a huh.NewSelect form
// for a single agent card's model options. The form is rendered full-screen
// using the same alt-screen approach as other steps.
type huhSelectCommand struct {
	options  []model.ModelOption
	current  int
	title    string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	chosen   int
	err      error
}

func (c *huhSelectCommand) SetStdin(r io.Reader)  { c.stdin = r }
func (c *huhSelectCommand) SetStdout(w io.Writer) { c.stdout = w }
func (c *huhSelectCommand) SetStderr(w io.Writer) { c.stderr = w }

func (c *huhSelectCommand) Run() error {
	huhOpts := make([]huh.Option[int], len(c.options))
	for i, opt := range c.options {
		huhOpts[i] = huh.NewOption(opt.Label, i)
	}

	chosen := c.current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(c.title).
				Options(huhOpts...).
				Value(&chosen).
				Height(workflowModelSelectHeight),
		),
	).
		WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if c.stdin != nil {
		form = form.WithInput(c.stdin)
	}
	if c.stdout != nil {
		form = form.WithOutput(c.stdout)
	}

	if err := form.Run(); err != nil {
		c.err = err
		return err
	}
	c.chosen = chosen
	return nil
}

// workflowTermHeight queries the current terminal height.
// Falls back to workflowGridMinHeight so the grid layout is used by default.
func workflowTermHeight() int {
	_, h, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || h <= 0 {
		return workflowGridMinHeight
	}
	return h
}

// WorkflowModelLayout returns the appropriate huh Layout for the workflow model
// selection form based on the number of roles and terminal height.
// Uses a grid layout when possible, otherwise falls back to columns.
func WorkflowModelLayout(numRoles, termHeight int) huh.Layout {
	if numRoles <= 1 {
		return huh.LayoutDefault
	}
	cols := 2
	rows := int(math.Ceil(float64(numRoles) / float64(cols)))
	if termHeight >= workflowGridMinHeight && rows <= 2 {
		return huh.LayoutGrid(rows, cols)
	}
	return huh.LayoutColumns(cols)
}

// subagentRoles returns the subagent roles that have a Model field from a workflow.
func subagentRoles(wfs []model.WorkflowManifest) []model.WorkflowRole {
	var roles []model.WorkflowRole
	for _, wf := range wfs {
		for _, role := range wf.Components.Roles {
			if role.Kind == "subagent" && role.Model != "" {
				roles = append(roles, role)
			}
		}
	}
	return roles
}

// ---------------------------------------------------------------------------
// cardCell represents one agent card within a phase row.
// ---------------------------------------------------------------------------
type cardCell struct {
	agentName string
	roleName  string
	options   []model.ModelOption
	selected  int // index into options
}

// selectedLabel returns the display label for the currently selected model.
func (c *cardCell) selectedLabel() string {
	if c.selected < 0 || c.selected >= len(c.options) {
		return "—"
	}
	return c.options[c.selected].Label
}

// selectedLabelTruncated returns the display label truncated to maxLen visible chars.
func (c *cardCell) selectedLabelTruncated(maxLen int) string {
	lbl := c.selectedLabel()
	if maxLen < 1 {
		maxLen = 1
	}
	if lipgloss.Width(lbl) > maxLen {
		if maxLen > 1 {
			return lbl[:maxLen-1] + "…"
		}
		return lbl[:maxLen]
	}
	return lbl
}

// ---------------------------------------------------------------------------
// phaseRow groups all agent cards for a single phase.
// ---------------------------------------------------------------------------
type phaseRow struct {
	phase string
	cards []cardCell
}

// ---------------------------------------------------------------------------
// focusTarget identifies which element has focus.
// ---------------------------------------------------------------------------
type focusTarget int

const (
	focusCards   focusTarget = iota // navigating the card grid
	focusButton                    // navigating the confirm/cancel buttons
)

// ---------------------------------------------------------------------------
// modelSelectorModel is the custom bubbletea model for the workflow model selector.
// ---------------------------------------------------------------------------
type modelSelectorModel struct {
	phases []phaseRow

	// Navigation state.
	curPhase int // index into phases
	curAgent int // index into phases[curPhase].cards
	focus    focusTarget

	// Button state.
	buttonIdx int // 0 = Confirm, 1 = Cancel

	// Terminal dimensions for resize handling.
	width  int
	height int

	// Header info.
	stepLabel string

	// Result state.
	confirmed bool
	cancelled bool
}

func newModelSelectorModel(
	agentConfigs []AgentModelConfig,
	roles []model.WorkflowRole,
	savedModels map[string]map[string]string,
	stepLabel string,
) modelSelectorModel {
	phases, phaseGroups := groupRolesByPhase(roles)

	var phaseRows []phaseRow
	for _, phase := range phases {
		phaseRoles := phaseGroups[phase]
		var cards []cardCell
		for _, role := range phaseRoles {
			for _, agent := range agentConfigs {
				// Determine default selection index.
				defaultIdx := 0 // inherit sentinel
				if savedModels != nil {
					if agentMap, ok := savedModels[agent.Name]; ok {
						if saved, ok := agentMap[role.Name]; ok && saved != "" {
							for i, opt := range agent.ModelOptions {
								if opt.Value == saved {
									defaultIdx = i
									break
								}
							}
						}
					}
				}
				cards = append(cards, cardCell{
					agentName: agent.Name,
					roleName:  role.Name,
					options:   agent.ModelOptions,
					selected:  defaultIdx,
				})
			}
		}
		phaseRows = append(phaseRows, phaseRow{
			phase: phase,
			cards: cards,
		})
	}

	// Get initial terminal size.
	w, h := 80, 40
	if tw, th, err := xterm.GetSize(os.Stdout.Fd()); err == nil && tw > 0 {
		w, h = tw, th
	}

	return modelSelectorModel{
		phases:    phaseRows,
		stepLabel: stepLabel,
		width:     w,
		height:    h,
	}
}

func (m modelSelectorModel) Init() tea.Cmd {
	return nil
}

func (m modelSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case modelPickedMsg:
		if msg.err == nil && msg.phaseIdx < len(m.phases) &&
			msg.agentIdx < len(m.phases[msg.phaseIdx].cards) {
			m.phases[msg.phaseIdx].cards[msg.agentIdx].selected = msg.value
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m modelSelectorModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit.
	if key == "ctrl+c" {
		m.cancelled = true
		return m, tea.Quit
	}

	// Card grid / button navigation.
	switch key {
	case "up", "k":
		if m.focus == focusButton {
			m.focus = focusCards
			m.curPhase = len(m.phases) - 1
			m.clampAgent()
		} else if m.curPhase > 0 {
			m.curPhase--
			m.clampAgent()
		}
	case "down", "j":
		if m.focus == focusCards {
			if m.curPhase < len(m.phases)-1 {
				m.curPhase++
				m.clampAgent()
			} else {
				m.focus = focusButton
				m.buttonIdx = 0
			}
		}
	case "left", "h":
		if m.focus == focusButton {
			if m.buttonIdx > 0 {
				m.buttonIdx--
			}
		} else if m.curAgent > 0 {
			m.curAgent--
		}
	case "right", "l":
		if m.focus == focusButton {
			if m.buttonIdx < 1 {
				m.buttonIdx++
			}
		} else if len(m.phases) > 0 && m.curAgent < len(m.phases[m.curPhase].cards)-1 {
			m.curAgent++
		}
	case "enter", " ":
		if m.focus == focusButton {
			if m.buttonIdx == 0 {
				m.confirmed = true
				return m, tea.Quit
			}
			m.cancelled = true
			return m, tea.Quit
		}
		// Launch full-screen model picker for the focused card.
		if len(m.phases) > 0 && len(m.phases[m.curPhase].cards) > 0 {
			card := &m.phases[m.curPhase].cards[m.curAgent]
			phaseIdx := m.curPhase
			agentIdx := m.curAgent
			title := fmt.Sprintf("Model for %s (%s)", card.roleName, card.agentName)
			cmd := &huhSelectCommand{
				options: card.options,
				current: card.selected,
				title:   title,
			}
			return m, tea.Exec(cmd, func(err error) tea.Msg {
				return modelPickedMsg{
					phaseIdx: phaseIdx,
					agentIdx: agentIdx,
					value:    cmd.chosen,
					err:      err,
				}
			})
		}
	case "tab":
		if m.focus == focusCards {
			m.focus = focusButton
			m.buttonIdx = 0
		} else {
			m.focus = focusCards
		}
	case "shift+tab":
		if m.focus == focusButton {
			m.focus = focusCards
		}
	}
	return m, nil
}


func (m *modelSelectorModel) clampAgent() {
	if len(m.phases) == 0 {
		return
	}
	maxAgent := len(m.phases[m.curPhase].cards) - 1
	if m.curAgent > maxAgent {
		m.curAgent = maxAgent
	}
	if m.curAgent < 0 {
		m.curAgent = 0
	}
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func (m modelSelectorModel) View() tea.View {
	var b strings.Builder

	// Header: banner + step indicator.
	b.WriteString(stepHeaderString(4, TotalSteps, m.stepLabel))
	b.WriteString("\n\n")

	// Compute usable content width (terminal width minus left indent of 2).
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Phase label style — bold accent like huh titles.
	phaseStyle := lipgloss.NewStyle().
		Foreground(tuistyles.ColorAccent).
		Bold(true)

	// Determine number of agents (cards in first phase, or 0).
	numAgents := 0
	if len(m.phases) > 0 {
		numAgents = len(m.phases[0].cards)
	}

	useColumns := numAgents > 1 && m.useColumnLayout(numAgents, contentWidth)

	// Render column headers once before the first phase (column layout only).
	if useColumns {
		headerLines := m.renderColumnHeaders(numAgents, contentWidth)
		for _, line := range headerLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	for pi, pr := range m.phases {
		// Phase label.
		b.WriteString("  " + phaseStyle.Render(pr.phase))
		b.WriteString("\n")

		// Render all agent cards for this phase in a horizontal row.
		var cardLines []string
		if useColumns {
			cardLines = m.renderPhaseCardsColumns(pr, pi, contentWidth, numAgents)
		} else {
			cardLines = m.renderPhaseCardsSequential(pr, pi, contentWidth)
		}
		for _, line := range cardLines {
			b.WriteString(line)
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	// Buttons.
	b.WriteString(m.renderButtons())
	b.WriteString("\n\n")

	// Help line.
	helpStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)
	if m.focus == focusButton {
		b.WriteString("  " + helpStyle.Render("←/→ switch button  enter select  tab back to cards"))
	} else {
		b.WriteString("  " + helpStyle.Render("←/→ agents  ↑/↓ phases  enter pick model  tab buttons"))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// useColumnLayout reports whether the column layout (with │ separators) should
// be used given the number of agents and available content width.
func (m modelSelectorModel) useColumnLayout(numAgents, contentWidth int) bool {
	if m.width < colTermMinWidth {
		return false
	}
	// Check that we can fit at least colMinWidth per agent plus separators.
	minRequired := numAgents*colMinWidth + (numAgents-1)*separatorWidth
	return contentWidth >= minRequired
}

// colWidth computes the per-agent column width for the column layout.
func colWidth(numAgents, contentWidth int) int {
	if numAgents <= 0 {
		return colMaxWidth
	}
	totalSep := (numAgents - 1) * separatorWidth
	w := (contentWidth - totalSep) / numAgents
	if w > colMaxWidth {
		w = colMaxWidth
	}
	if w < colMinWidth {
		w = colMinWidth
	}
	return w
}

// renderColumnHeaders renders the agent name header row and a separator line
// that act as a "table header" above all phase rows (column layout only).
//
//	  claude              │  opencode
//	─────────────────────────────────
func (m modelSelectorModel) renderColumnHeaders(numAgents, contentWidth int) []string {
	if numAgents == 0 {
		return nil
	}
	// Gather agent names from the first phase.
	var agentNames []string
	if len(m.phases) > 0 {
		for _, card := range m.phases[0].cards {
			agentNames = append(agentNames, card.agentName)
		}
	} else {
		return nil
	}

	perCol := colWidth(numAgents, contentWidth)
	prefixWidth := 2 // "  " indent before each column

	headerStyle := lipgloss.NewStyle().
		Foreground(tuistyles.ColorAccent).
		Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)
	dividerStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)

	sep := sepStyle.Render("│")

	var headerRow strings.Builder
	headerRow.WriteString("  ") // left margin

	for i, name := range agentNames {
		if i > 0 {
			headerRow.WriteString(sepStyle.Render(" │ "))
		}
		// Left-pad with spaces equivalent to prefix width, then agent name.
		cell := strings.Repeat(" ", prefixWidth) + headerStyle.Render(name)
		// Pad to column width.
		cellVisibleWidth := prefixWidth + lipgloss.Width(name)
		pad := perCol - cellVisibleWidth
		if pad > 0 {
			cell += strings.Repeat(" ", pad)
		}
		headerRow.WriteString(cell)
	}

	// Divider line: total width = numAgents*perCol + (numAgents-1)*separatorWidth
	totalWidth := numAgents*perCol + (numAgents-1)*separatorWidth
	divider := dividerStyle.Render(strings.Repeat("─", totalWidth))

	_ = sep // sep is used inline above

	return []string{
		headerRow.String(),
		"  " + divider,
	}
}

// renderPhaseCardsColumns renders model indicators for a phase using a column layout
// with a dim │ separator between each agent column. Agent names are NOT repeated
// here — they appear once in the column headers rendered by renderColumnHeaders.
//
//	  › sonnet ▾              │    claude-sonnet-4.6 ▾
func (m modelSelectorModel) renderPhaseCardsColumns(pr phaseRow, phaseIdx, contentWidth, numAgents int) []string {
	numCards := len(pr.cards)
	if numCards == 0 {
		return nil
	}

	perCol := colWidth(numAgents, contentWidth)

	focusedStyle := lipgloss.NewStyle().
		Foreground(tuistyles.ColorSecondary).
		Bold(true)
	normalModelStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted)
	focusIndicator := lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary)
	dimStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)
	sepStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)

	sep := sepStyle.Render(" │ ")

	var modelRow strings.Builder
	modelRow.WriteString("  ")

	prefixWidth := 2 // "› " or "  "

	for ci, card := range pr.cards {
		isFocused := m.focus == focusCards && m.curPhase == phaseIdx && m.curAgent == ci

		if ci > 0 {
			modelRow.WriteString(sep)
		}

		// Prefix: "› " focused, "  " blurred.
		var prefix string
		if isFocused {
			prefix = focusIndicator.Render("›") + " "
		} else {
			prefix = dimStyle.Render(" ") + " "
		}

		textWidth := perCol - prefixWidth
		if textWidth < 1 {
			textWidth = 1
		}

		// Model indicator — "label ▾".
		maxModelLen := textWidth - 2
		if maxModelLen < 1 {
			maxModelLen = 1
		}
		modelLabel := card.selectedLabelTruncated(maxModelLen)
		dropText := modelLabel + " \u25be"
		dropPad := textWidth - lipgloss.Width(dropText)
		if dropPad < 0 {
			dropPad = 0
		}
		modelSty := normalModelStyle
		if isFocused {
			modelSty = focusedStyle
		}
		modelRow.WriteString(prefix + modelSty.Render(dropText) + strings.Repeat(" ", dropPad))
	}

	return []string{modelRow.String()}
}

// renderPhaseCardsSequential renders agent cards for a phase in a sequential
// (stacked) layout: one agent per line pair, separated by a blank line.
// Used when the terminal is too narrow for the column layout.
//
//	  claude: sonnet ▾
//	  opencode: claude-sonnet-4.6 ▾
func (m modelSelectorModel) renderPhaseCardsSequential(pr phaseRow, phaseIdx, contentWidth int) []string {
	if len(pr.cards) == 0 {
		return nil
	}

	focusedStyle := lipgloss.NewStyle().
		Foreground(tuistyles.ColorSecondary).
		Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted)
	focusIndicator := lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary)
	dimStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted)

	var lines []string
	for ci, card := range pr.cards {
		isFocused := m.focus == focusCards && m.curPhase == phaseIdx && m.curAgent == ci

		var prefix string
		if isFocused {
			prefix = focusIndicator.Render("›") + " "
		} else {
			prefix = dimStyle.Render(" ") + " "
		}

		agentLabel := labelStyle.Render(card.agentName + ":")
		maxModelLen := contentWidth - lipgloss.Width(card.agentName) - 3 // ": " + ▾
		if maxModelLen < 1 {
			maxModelLen = 1
		}
		modelLabel := card.selectedLabelTruncated(maxModelLen)
		dropText := modelLabel + " \u25be"

		modelSty := normalStyle
		if isFocused {
			modelSty = focusedStyle
		}

		line := "  " + prefix + agentLabel + " " + modelSty.Render(dropText)
		lines = append(lines, line)
	}
	return lines
}

// renderButtons renders the Confirm/Cancel button row.
func (m modelSelectorModel) renderButtons() string {
	focusedBtn := lipgloss.NewStyle().
		Foreground(tuistyles.ColorBg).
		Background(tuistyles.ColorSecondary).
		Bold(true).
		Padding(0, 2)
	blurredBtn := lipgloss.NewStyle().
		Foreground(tuistyles.ColorMuted).
		Background(tuistyles.ColorDim).
		Padding(0, 2)

	confirmStyle := blurredBtn
	cancelStyle := blurredBtn
	if m.focus == focusButton {
		if m.buttonIdx == 0 {
			confirmStyle = focusedBtn
		} else {
			cancelStyle = focusedBtn
		}
	}

	return "  " + confirmStyle.Render("Confirm") + "  " + cancelStyle.Render("Cancel")
}

// ---------------------------------------------------------------------------
// RunWorkflowModelSelection — public entry point (signature unchanged)
// ---------------------------------------------------------------------------

// RunWorkflowModelSelection shows a per-agent model selection step.
//
// The step is skipped (returns nil, nil) when:
//   - no agent in agents is in model.ModelRoutingAgents, OR
//   - no workflow is selected across all repos in selection, OR
//   - no workflow has subagent roles with model fields.
//
// Returns map[agentName]map[roleName]modelValue, or nil if the step was skipped.
func RunWorkflowModelSelection(
	agents []string,
	selection SelectionResult,
	savedModels map[string]map[string]string,
	workflows []model.WorkflowManifest,
) (map[string]map[string]string, error) {
	// Check qualifying agents.
	var qualifyingAgents []string
	for _, a := range agents {
		if model.ModelRoutingAgents[a] {
			qualifyingAgents = append(qualifyingAgents, a)
		}
	}
	if len(qualifyingAgents) == 0 {
		return nil, nil
	}

	// Check that at least one workflow is selected.
	hasWorkflow := false
	for _, repo := range selection.Repos {
		if len(repo.SelectedWorkflows) > 0 {
			hasWorkflow = true
			break
		}
	}
	if !hasWorkflow {
		return nil, nil
	}

	// Get subagent roles that define a model.
	roles := subagentRoles(workflows)
	// When no roles are found from workflow manifests but savedModels has entries,
	// synthesize roles from the saved model keys so the form always shows and
	// lets the user review or change their previous selections.
	if len(roles) == 0 && len(savedModels) > 0 {
		seen := make(map[string]bool)
		for _, roleMap := range savedModels {
			for roleName := range roleMap {
				if !seen[roleName] {
					seen[roleName] = true
					roles = append(roles, model.WorkflowRole{
						Name:  roleName,
						Kind:  "subagent",
						Model: roleName,
					})
				}
			}
		}
	}
	if len(roles) == 0 {
		return nil, nil
	}

	// Build AgentModelConfig for each qualifying agent.
	var agentConfigs []AgentModelConfig
	for _, a := range qualifyingAgents {
		var opts []model.ModelOption
		switch a {
		case "claude":
			opts = model.ClaudeModelOptions()
		case "opencode":
			opts = model.OpenCodeModelOptions(openCodeFallbackModels)
		default:
			opts = model.ClaudeModelOptions()
		}
		agentConfigs = append(agentConfigs, AgentModelConfig{
			Name:         a,
			ModelOptions: opts,
		})
	}

	// Extract workflow name for step header.
	workflowName := ""
	if len(workflows) > 0 {
		workflowName = workflows[0].Metadata.EffectiveDisplayName()
	}

	stepLabel := "Workflow Models"
	if workflowName != "" {
		stepLabel = "Workflow: " + workflowName + " — Model Selection"
	}

	// Create and run the custom bubbletea model.
	mdl := newModelSelectorModel(agentConfigs, roles, savedModels, stepLabel)
	p := tea.NewProgram(mdl)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(modelSelectorModel)
	if final.cancelled {
		return nil, fmt.Errorf("model selection cancelled")
	}

	// Collect results: map[agentName]map[roleName]modelValue.
	result := make(map[string]map[string]string)
	for _, pr := range final.phases {
		for _, card := range pr.cards {
			val := card.options[card.selected].Value
			if val != "" && val != model.ModelInheritOption {
				if result[card.agentName] == nil {
					result[card.agentName] = make(map[string]string)
				}
				result[card.agentName][card.roleName] = val
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Helper functions (kept unchanged for backward compatibility)
// ---------------------------------------------------------------------------

// phaseFromRole extracts a display phase name from a workflow role key.
// It takes the last hyphen-separated segment and title-cases it.
//
// Examples:
//
//	phaseFromRole("sdd-explore")  → "Explore"
//	phaseFromRole("cicd-build")   → "Build"
//	phaseFromRole("review")       → "Review"
func phaseFromRole(roleKey string) string {
	if idx := strings.LastIndex(roleKey, "-"); idx >= 0 {
		segment := roleKey[idx+1:]
		if len(segment) > 0 {
			return strings.ToUpper(segment[:1]) + segment[1:]
		}
		// Trailing hyphen (e.g. "sdd-"): use prefix instead.
		prefix := roleKey[:idx]
		if len(prefix) > 0 {
			return strings.ToUpper(prefix[:1]) + prefix[1:]
		}
	}
	// No hyphen found: title-case the whole key.
	if len(roleKey) > 0 {
		return strings.ToUpper(roleKey[:1]) + roleKey[1:]
	}
	return roleKey
}

// groupRolesByPhase groups roles by their phase (from phaseFromRole).
// Order is preserved: first phase encountered comes first.
func groupRolesByPhase(roles []model.WorkflowRole) ([]string, map[string][]model.WorkflowRole) {
	var phases []string
	groups := make(map[string][]model.WorkflowRole)
	for _, role := range roles {
		phase := phaseFromRole(role.Name)
		if _, exists := groups[phase]; !exists {
			phases = append(phases, phase)
		}
		groups[phase] = append(groups[phase], role)
	}
	return phases, groups
}

// formatRoleLabel converts a workflow role name to a human-readable display label.
//
// When agentPrefix is non-empty, the label is prefixed with "[agentPrefix] ".
//
// The function strips common workflow prefixes (e.g. "sdd-" from "sdd-explorer"),
// capitalises the first letter, replaces hyphens with spaces, and appends " model".
func formatRoleLabel(roleName, agentPrefix string) string {
	// Strip common workflow prefixes (sdd-, cicd-, etc.)
	stripped := roleName
	if idx := strings.Index(roleName, "-"); idx >= 0 {
		stripped = roleName[idx+1:]
	}
	// Replace hyphens with spaces and capitalize.
	stripped = strings.ReplaceAll(stripped, "-", " ")
	if len(stripped) > 0 {
		stripped = strings.ToUpper(stripped[:1]) + stripped[1:]
	}
	label := stripped + " model"
	if agentPrefix != "" {
		return "[" + agentPrefix + "] " + label
	}
	return label
}
