// SPDX-License-Identifier: MIT

package tui

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// autoRecommendEnabled returns true when the auto-recommend feature should run.
// A nil pointer (field absent) means enabled; explicit false disables it.
func autoRecommendEnabled(cfg model.InstallConfig) bool {
	return cfg.AutoRecommend == nil || *cfg.AutoRecommend
}

// RunResult holds all outputs from the interactive TUI wizard.
type RunResult struct {
	Manifest       model.UserManifest
	InstalledTools []string // tool names successfully installed via brew
}

// ExistingConfig holds preselection data loaded from an existing devrune.yaml.
// When non-nil, the wizard uses these values to preselect agents and sources
// that still exist in the current scan results (smart config merge).
// WorkflowModels is the merged per-agent role model map extracted from all workflow entries.
// Recommendations holds AI recommendations from a previous devrune.recommended.yaml run,
// used to pre-select items with AI badges in the selection step.
type ExistingConfig struct {
	Agents          []string
	Sources         []string
	WorkflowModels  map[string]map[string]string
	Recommendations []model.RecommendedItem // AI recommendations for pre-selection
}

// Run executes the interactive TUI wizard and returns the resulting
// RunResult on success.
//
// The wizard walks the user through sequential steps:
//
//	Step 1: Agent selection
//	Step 2: Repository source refs (one at a time)
//	Step 3: Scan + category/item selection
//	Step 4: Summary + confirmation
//
// catalogSources contains source ref strings from the catalogs: key in devrune.yaml
// or from --catalog CLI flags. They appear as pre-selected options
// alongside the built-in known sources in Step 2. Pass nil or an empty slice when
// no catalog config was detected.
//
// existing, when non-nil, provides preselection data from an existing devrune.yaml.
// Agents and sources from existing that still appear in the current catalog/scan
// are preselected in the wizard. Pass nil for a fresh (non-merge) init.
//
// If the user aborts at any step (Ctrl-C or declining confirmation) the
// function returns huh.ErrUserAborted.
func Run(catalogSources []string, existing *ExistingConfig) (RunResult, error) {
	// Determine preselected agents and sources from existing config.
	var preselectedAgents []string
	var preselectedSources []string
	if existing != nil {
		preselectedAgents = existing.Agents
		preselectedSources = existing.Sources
	}

	// Step 1 — agents (alt screen, step indicator inside form)
	agents, err := steps.SelectAgents(preselectedAgents)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	// Determine whether auto-recommend is enabled.
	var installCfg model.InstallConfig
	if m := loadExistingManifest(); m != nil {
		installCfg = m.Install
	}
	recommendEnabled := autoRecommendEnabled(installCfg)

	// Determine early if the workflow model selection step may be active.
	// The step requires at least one qualifying agent (in ModelRoutingAgents)
	// AND at least one workflow selected. We know the agent condition now; workflow
	// selection is only known after Step 3. Set TotalSteps to 5 if qualifying
	// agents exist so that Steps 2 and 3 already show the correct total.
	// After Step 3 we can confirm (or revert to 4) based on the actual selection.
	hasQualifyingAgent := false
	for _, a := range agents {
		if model.ModelRoutingAgents[a] {
			hasQualifyingAgent = true
			break
		}
	}
	baseSteps := 5
	if hasQualifyingAgent {
		baseSteps = 6
	}
	steps.TotalSteps = baseSteps

	// Step 2 — repository sources (alt screen, step indicator inside form)
	// Pass catalog-detected sources; EnterRepositories merges them with knownSources.
	sources, err := steps.EnterRepositories(catalogSources, preselectedSources)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	// Step 3 — scan + select (alt screen)
	var selection steps.SelectionResult
	var scanned []steps.ScanResult
	var inputs []steps.ScannedRepoInput

	if len(sources) > 0 {
		cp := cachePath()

		// Wrap ScanRepositories so the steps package can call it without
		// importing the tui package (avoids import cycle).
		scanFn := func(ctx context.Context, srcs []string, cachePath string) ([]steps.ScanResult, error) {
			s, err := ScanRepositories(ctx, srcs, cachePath)
			if err != nil {
				return nil, err
			}
			out := make([]steps.ScanResult, len(s))
			for i, r := range s {
				out[i] = steps.ScanResult{
					Source:            r.Source,
					Skills:            r.Skills,
					Rules:             r.Rules,
					MCPs:              r.MCPs,
					Workflows:         r.Workflows,
					WorkflowManifests: r.WorkflowManifests,
					Descs:             r.Descs,
					MCPFiles:          r.MCPFiles,
					Tools:             r.Tools,
					Error:             r.Error,
				}
			}
			return out, nil
		}

		var warnings []string
		scanned, warnings, err = steps.RunScanModel(sources, scanFn, cp)
		if err != nil {
			return RunResult{}, fmt.Errorf("scan repositories: %w", err)
		}

		// Report any per-repo scan warnings.
		for _, w := range warnings {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", w)
		}

		// Convert to ScannedRepoInput for the select model.
		inputs = make([]steps.ScannedRepoInput, 0, len(scanned))
		for _, r := range scanned {
			if r.Error == nil {
				inputs = append(inputs, steps.ScannedRepoInput{
					Source:            r.Source,
					Skills:            r.Skills,
					Rules:             r.Rules,
					MCPs:              r.MCPs,
					Workflows:         r.Workflows,
					WorkflowManifests: r.WorkflowManifests,
					Tools:             r.Tools,
					Descs:             r.Descs,
					MCPFiles:          r.MCPFiles,
				})
			}
		}

		if len(inputs) > 0 {
			// Loop: select → (optional) AI recommendations → back to select if user declines.
			var prevSelection *steps.SelectionResult // preserves state on go-back
			for {
				var selectResult steps.SelectModelResult
				var err error
				if prevSelection != nil {
					selectResult, err = steps.RunSelectModel(inputs, recommendEnabled, *prevSelection)
				} else {
					selectResult, err = steps.RunSelectModel(inputs, recommendEnabled)
				}
				if err != nil {
					return RunResult{}, mapErr(err)
				}
				selection = selectResult.Selection
				prevSelection = &selection // save for potential go-back

				if !selectResult.UseRecommendations {
					// User chose "Confirm selection" — done.
					break
				}

				// User chose "AI Recommendations" — run spinner+gate in one program.
				// Keep alt-screen active to avoid flicker between programs.
				fmt.Print("\033[?1049h\033[2J\033[H") // enter alt-screen + clear
				flowResult := steps.RunRecommendFlow(inputs)
				if flowResult.Err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[devrune] recommend error: %v\n", flowResult.Err)
					break
				}
				if flowResult.Accepted && flowResult.Result != nil {
					// Merge recommendations into selection.
					selection = steps.MergeRecommendationsIntoSelection(
						selection, flowResult.Result.Recommendations, 0.7,
					)
					break
				}
				if flowResult.Skipped {
					// User chose "No, go back" — loop back to select step.
					continue
				}
				break
			}
		}
	}

	// Collect workflow manifests from all scanned repos for the model selector.
	var allWorkflowManifests []model.WorkflowManifest
	for _, r := range scanned {
		if r.Error == nil {
			allWorkflowManifests = append(allWorkflowManifests, r.WorkflowManifests...)
		}
	}

	// Now that selection is known, confirm whether the workflow model step will actually run.
	if hasQualifyingAgent {
		hasWorkflow := false
		for _, repo := range selection.Repos {
			if len(repo.SelectedWorkflows) > 0 {
				hasWorkflow = true
				break
			}
		}
		if !hasWorkflow {
			steps.TotalSteps = 5
		}
	}

	// Step 4 (optional) — Workflow model selection
	// Use saved models from ExistingConfig when provided; fall back to disk load.
	var savedModels map[string]map[string]string
	if existing != nil {
		savedModels = existing.WorkflowModels
	} else {
		savedManifest := loadExistingManifest()
		if savedManifest != nil {
			savedModels = mergeWorkflowModels(savedManifest.Workflows)
		}
	}

	workflowModels, err := steps.RunWorkflowModelSelection(agents, selection, savedModels, allWorkflowManifests)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	// Final confirmation: sync TotalSteps with the actual outcome.
	if workflowModels != nil {
		steps.TotalSteps = 6
	} else {
		steps.TotalSteps = 5
	}

	// Collect all tool definitions from scanned repos.
	var allToolDefs []model.ToolDef
	for _, r := range scanned {
		if r.Error == nil {
			allToolDefs = append(allToolDefs, r.Tools...)
		}
	}

	// Filter tools by depends_on conditions and selection.
	activeTools := filterToolsBySelection(allToolDefs, selection)

	// Run tool install step (silently skipped if no tools or no brew).
	toolResult, err := steps.RunToolInstallStep(activeTools)
	if err != nil {
		return RunResult{}, mapErr(err)
	}
	installedTools := toolResult.Installed

	// Final step — summary & confirm (alt screen, step indicator inside form)
	manifest, err := steps.ConfirmSummary(agents, selection, workflowModels, catalogSources)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	return RunResult{Manifest: manifest, InstalledTools: installedTools}, nil
}

// mergeWorkflowModels merges per-agent role model overrides from all workflow entries
// into a flat map[agentName]map[roleName]modelValue for use by the model selection step.
func mergeWorkflowModels(workflows map[string]model.WorkflowEntry) map[string]map[string]string {
	var merged map[string]map[string]string
	for _, entry := range workflows {
		if len(entry.Roles) == 0 {
			continue
		}
		if merged == nil {
			merged = make(map[string]map[string]string)
		}
		for agent, roles := range entry.Roles {
			if merged[agent] == nil {
				merged[agent] = make(map[string]string)
			}
			for role, modelVal := range roles {
				merged[agent][role] = modelVal
			}
		}
	}
	return merged
}

// loadExistingManifest reads devrune.yaml from the current working directory
// and returns the parsed manifest. Returns nil on any error (file not found, parse error).
func loadExistingManifest() *model.UserManifest {
	data, err := os.ReadFile("devrune.yaml")
	if err != nil {
		return nil
	}
	var manifest model.UserManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	return &manifest
}

// cachePath returns the default cache directory: ~/.cache/devrune/packages/
func cachePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = home + "/.cache"
	}
	return base + "/devrune/packages"
}

// mapErr normalises huh.ErrUserAborted so callers can use errors.Is.
// Other errors are returned unchanged.
func mapErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return huh.ErrUserAborted
	}
	// Also map the select model's user aborted error to huh.ErrUserAborted
	// so that the caller can use errors.Is uniformly.
	if err != nil && err.Error() == "user aborted" {
		return huh.ErrUserAborted
	}
	return err
}

// filterToolsBySelection filters tools by the user's category selections and
// depends_on conditions. A tool is included when:
//   - The user selected it in the Tools category (it appears in SelectedTools), AND
//   - Its DependsOn is nil (no conditions), OR at least one DependsOn condition is met
//     (OR logic: matching the specified MCP or workflow is sufficient).
func filterToolsBySelection(tools []model.ToolDef, selection steps.SelectionResult) []model.ToolDef {
	// Build sets from selection across all repos.
	selectedMCPs := make(map[string]bool)
	selectedWorkflows := make(map[string]bool)
	selectedTools := make(map[string]bool)

	for _, repo := range selection.Repos {
		for _, mcp := range repo.SelectedMCPs {
			selectedMCPs[mcp] = true
		}
		for _, wf := range repo.SelectedWorkflows {
			selectedWorkflows[wf] = true
		}
		for _, tool := range repo.SelectedTools {
			selectedTools[tool] = true
		}
	}

	var result []model.ToolDef
	for _, tool := range tools {
		// Tool must be selected by the user in the Tools category.
		if !selectedTools[tool.Name] {
			continue
		}

		// No conditions: always include.
		if tool.DependsOn == nil {
			result = append(result, tool)
			continue
		}

		// OR logic: include if any condition matches.
		if tool.DependsOn.MCP != "" && selectedMCPs[tool.DependsOn.MCP] {
			result = append(result, tool)
			continue
		}
		if tool.DependsOn.Workflow != "" && selectedWorkflows[tool.DependsOn.Workflow] {
			result = append(result, tool)
			continue
		}
		// Neither condition matched — skip.
	}
	return result
}
