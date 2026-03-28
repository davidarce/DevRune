// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"math"
	"os"
	"strings"

	"charm.land/huh/v2"
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

// RunWorkflowModelSelection shows a per-agent model selection step.
//
// The step is skipped (returns nil, nil) when:
//   - no agent in agents is in model.ModelRoutingAgents, OR
//   - no workflow is selected across all repos in selection, OR
//   - no workflow has subagent roles with model fields.
//
// For each qualifying agent (claude, opencode) a separate huh form is shown
// with Select fields for each subagent role that has a model, arranged in a
// grid layout.
//
// savedModels pre-populates form defaults from a previously saved devrune.yaml.
//
// workflows is the list of resolved workflow manifests for the selected workflows.
// When nil, the function collects roles from the selection metadata (backward compat
// is handled by the caller providing workflow manifests).
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

	totalForms := len(agentConfigs)
	result := make(map[string]map[string]string)

	for formIdx, agentCfg := range agentConfigs {
		var savedSelections map[string]string
		if savedModels != nil {
			savedSelections = savedModels[agentCfg.Name]
		}

		stepNum := formIdx + 1
		selections, err := newWorkflowModelForm(agentCfg, roles, savedSelections, stepNum, totalForms)
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

// newWorkflowModelForm creates and runs a huh form for one agent's model selection.
// Roles are displayed dynamically based on the workflow's subagent roles.
func newWorkflowModelForm(
	agent AgentModelConfig,
	roles []model.WorkflowRole,
	savedSelections map[string]string,
	formNum, totalForms int,
) (map[string]string, error) {
	if len(agent.ModelOptions) == 0 || len(roles) == 0 {
		return nil, nil
	}

	// Build a string pointer per role to capture the selected value.
	roleValues := make(map[string]*string, len(roles))
	for _, role := range roles {
		// Default to inherit sentinel; override with saved selection if present.
		defaultVal := model.ModelInheritOption
		if savedSelections != nil {
			if saved, ok := savedSelections[role.Name]; ok && saved != "" {
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
		roleValues[role.Name] = &val
	}

	// Build step label.
	stepLabel := "Workflow Models"
	if totalForms > 1 {
		stepLabel = "Workflow Models — " + agent.Name
	}

	// One group per role so grid layout can arrange them.
	var roleGroups []*huh.Group
	for _, role := range roles {
		localRole := role
		targetVal := roleValues[localRole.Name]

		huhOpts := make([]huh.Option[string], len(agent.ModelOptions))
		for i, opt := range agent.ModelOptions {
			huhOpts[i] = huh.NewOption(opt.Label, opt.Value)
		}

		field := huh.NewSelect[string]().
			Title(formatRoleLabel(localRole.Name, "")).
			Description("Model to use for this role").
			Height(workflowModelSelectHeight).
			Options(huhOpts...).
			Value(targetVal)

		roleGroups = append(roleGroups, huh.NewGroup(field))
	}

	termH := workflowTermHeight()

	// Clear screen + cursor home before printing the header.
	_, _ = fmt.Fprint(os.Stdout, "\033[2J\033[H")
	_, _ = fmt.Fprintln(os.Stdout, stepHeaderString(4, TotalSteps, stepLabel))

	form := huh.NewForm(roleGroups...).
		WithTheme(tuistyles.DevRuneThemeFunc).
		WithLayout(WorkflowModelLayout(len(roles), termH))

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Collect results, filtering out inherit sentinels.
	result := make(map[string]string, len(roles))
	for _, role := range roles {
		val := *roleValues[role.Name]
		if val != "" && val != model.ModelInheritOption {
			result[role.Name] = val
		}
	}
	return result, nil
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
