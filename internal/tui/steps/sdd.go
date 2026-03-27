// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	xterm "github.com/charmbracelet/x/term"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// sddModelSelectHeight is the number of visible rows for each model Select field.
// Set to show all options without scrolling (inherit sentinel + haiku + sonnet + opus = 4).
const sddModelSelectHeight = 7

// sddGridMinHeight is the minimum terminal height required to display all four
// SDD phase groups in a 2×2 grid without clipping. Below this threshold the
// layout falls back to LayoutColumns(2) which paginates 2 groups at a time.
const sddGridMinHeight = 35

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

// sddTermHeight queries the current terminal height.
// Falls back to sddGridMinHeight so the grid layout is used by default.
func sddTermHeight() int {
	_, h, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || h <= 0 {
		return sddGridMinHeight
	}
	return h
}

// SDDLayout returns the appropriate huh Layout for the SDD model selection
// form based on the current terminal height. Uses LayoutGrid(2, 2) when the
// terminal is tall enough, otherwise falls back to LayoutColumns(2).
func SDDLayout(termHeight int) huh.Layout {
	if termHeight >= sddGridMinHeight {
		return huh.LayoutGrid(2, 2)
	}
	return huh.LayoutColumns(2)
}

// RunSDDModelSelection shows a per-agent model selection step.
//
// The step is skipped (returns nil, nil) when:
//   - no agent in agents is in model.SDDModelRoutingAgents, OR
//   - no workflow is selected across all repos in selection.
//
// For each qualifying agent (claude, opencode) a separate huh form is shown
// with four Select fields arranged in a 2-column layout:
//
//	[Explore model]     [Plan model]
//	[Implement model]   [Review model]
//
// savedModels pre-populates form defaults from a previously saved devrune.yaml.
//
// Returns map[agentName]map[roleName]modelValue, or nil if the step was skipped.
func RunSDDModelSelection(
	agents []string,
	selection SelectionResult,
	savedModels map[string]map[string]string,
) (map[string]map[string]string, error) {
	// Check qualifying agents.
	var qualifyingAgents []string
	for _, a := range agents {
		if model.SDDModelRoutingAgents[a] {
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

	totalForms := len(agentConfigs)
	result := make(map[string]map[string]string)

	for formIdx, agentCfg := range agentConfigs {
		var savedSelections map[string]string
		if savedModels != nil {
			savedSelections = savedModels[agentCfg.Name]
		}

		// stepNum is the form number within the SDD step (1-indexed).
		// We show "Step X/Y" where X is the overall wizard step number.
		// Since we don't know the total wizard steps here, we use form index.
		stepNum := formIdx + 1
		selections, err := newSDDModelForm(agentCfg, savedSelections, stepNum, totalForms)
		if err != nil {
			return nil, err
		}
		if selections != nil {
			result[agentCfg.Name] = selections
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// newSDDModelForm creates and runs a huh form for one agent's model selection.
// The four phase selects are displayed in a 2-column layout:
//
//	[Explore model]     [Plan model]
//	[Implement model]   [Review model]
//
// Returns a map of roleName → selectedModelValue for the agent, or nil on skip.
func newSDDModelForm(
	agent AgentModelConfig,
	savedSelections map[string]string,
	formNum, totalForms int,
) (map[string]string, error) {
	if len(agent.ModelOptions) == 0 {
		return nil, nil
	}

	// Build a string pointer per role to capture the selected value.
	// We align with SDDPhaseRoleNames order.
	roleValues := make(map[string]*string, len(model.SDDPhaseRoleNames))
	for _, roleName := range model.SDDPhaseRoleNames {
		// Default to inherit sentinel; override with saved selection if present.
		defaultVal := model.SDDModelInheritOption
		if savedSelections != nil {
			if saved, ok := savedSelections[roleName]; ok && saved != "" {
				// Verify saved value is a valid option.
				for _, opt := range agent.ModelOptions {
					if opt.Value == saved {
						defaultVal = saved
						break
					}
				}
			}
		}
		val := defaultVal
		roleValues[roleName] = &val
	}

	// Build step label with agent name.
	stepLabel := "SDD Models"
	if totalForms > 1 {
		stepLabel = "SDD Models — " + agent.Name
	}

	// Groups 1–N: one group per SDD phase so that LayoutGrid(2, 2) / LayoutColumns(2)
	// can arrange them in a 2-column grid showing all four phases simultaneously.
	var phaseGroups []*huh.Group
	for _, roleName := range model.SDDPhaseRoleNames {
		localRoleName := roleName
		targetVal := roleValues[localRoleName]

		huhOpts := make([]huh.Option[string], len(agent.ModelOptions))
		for i, opt := range agent.ModelOptions {
			huhOpts[i] = huh.NewOption(opt.Label, opt.Value)
		}

		field := huh.NewSelect[string]().
			Title(formatPhaseLabel(localRoleName, "")).
			Description("Model to use for this SDD phase").
			Height(sddModelSelectHeight).
			Options(huhOpts...).
			Value(targetVal)

		phaseGroups = append(phaseGroups, huh.NewGroup(field))
	}

	termH := sddTermHeight()

	// Clear screen + cursor home before printing the header so that
	// previous form output (e.g. the Claude form) doesn't stack with
	// the next agent's header (e.g. OpenCode).
	_, _ = fmt.Fprint(os.Stdout, "\033[2J\033[H")
	_, _ = fmt.Fprintln(os.Stdout, stepHeaderString(4, TotalSteps, stepLabel))

	// Run the four phase selects in a 2-column grid layout.
	// No AltScreen here: the form renders inline so the header printed via
	// fmt.Fprintln above remains visible above the grid form.
	form := huh.NewForm(phaseGroups...).
		WithTheme(tuistyles.DevRuneThemeFunc).
		WithLayout(SDDLayout(termH))

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Collect results, filtering out inherit sentinels.
	result := make(map[string]string, len(model.SDDPhaseRoleNames))
	for _, roleName := range model.SDDPhaseRoleNames {
		val := *roleValues[roleName]
		if val != "" && val != model.SDDModelInheritOption {
			result[roleName] = val
		}
	}
	return result, nil
}

// formatPhaseLabel converts an SDD role name to a human-readable display label.
//
// When agentPrefix is non-empty, the label is prefixed with "[agentPrefix] ".
//
// Known mappings (without prefix):
//
//	"sdd-explorer"    → "Explore model"
//	"sdd-planner"     → "Plan model"
//	"sdd-implementer" → "Implement model"
//	"sdd-reviewer"    → "Review model"
//
// Unknown names: strips the "sdd-" prefix (if present), capitalises the first
// letter, and appends " model".
func formatPhaseLabel(roleName, agentPrefix string) string {
	label, ok := model.SDDPhaseLabels[roleName]
	if !ok {
		stripped := strings.TrimPrefix(roleName, "sdd-")
		if len(stripped) > 0 {
			stripped = strings.ToUpper(stripped[:1]) + stripped[1:]
		}
		label = stripped + " model"
	}
	if agentPrefix != "" {
		return "[" + agentPrefix + "] " + label
	}
	return label
}
