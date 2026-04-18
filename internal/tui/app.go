// SPDX-License-Identifier: MIT

package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/recommend"
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
// projectDir is the root directory of the project being configured. It is passed
// to RunSelectModel for tech-stack detection and skills.sh curated skill injection.
// Pass an empty string to skip detection.
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
func Run(projectDir string, catalogSources []string, existing *ExistingConfig) (RunResult, error) {
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

	// Determine the step count BEFORE rendering any subsequent step header
	// so the "N/Total" indicator is consistent across the whole wizard.
	// The SDD model selection step (Step 3) is conditional on at least one
	// qualifying agent (ModelRoutingAgents) being selected.
	hasQualifyingAgent := false
	for _, a := range agents {
		if model.ModelRoutingAgents[a] {
			hasQualifyingAgent = true
			break
		}
	}
	baseSteps := 6
	if hasQualifyingAgent {
		baseSteps = 7
	}
	steps.TotalSteps = baseSteps

	// Step 2 — SDD workflow info
	if err := steps.ShowSDDInfoStep(); err != nil {
		return RunResult{}, mapErr(err)
	}

	// Step 3 (conditional) — SDD model selection, before repositories
	var workflowModels map[string]map[string]string
	if hasQualifyingAgent {
		// Pre-load saved models from existing config for defaults.
		var savedModels map[string]map[string]string
		if existing != nil {
			savedModels = existing.WorkflowModels
		} else if savedManifest := loadExistingManifest(); savedManifest != nil {
			savedModels = mergeWorkflowModels(savedManifest.Workflows)
		}
		// SDD workflow is always auto-selected — pass sddAutoSelected=true
		// so model selection does not skip when no scan results exist yet.
		var emptyManifests []model.WorkflowManifest
		wm, err := steps.RunWorkflowModelSelection(agents, steps.SelectionResult{}, savedModels, emptyManifests, true)
		if err != nil {
			return RunResult{}, mapErr(err)
		}
		workflowModels = wm
	}

	// Step 4 — repository sources (alt screen, step indicator inside form)
	// Pass catalog-detected sources; EnterRepositories merges them with knownSources.
	sources, err := steps.EnterRepositories(catalogSources, preselectedSources)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	// Step 5 — scan + select (alt screen)
	var selection steps.SelectionResult
	var scanned []steps.ScanResult
	var inputs []steps.ScannedRepoInput

	// Separate the skills.sh sentinel from real repo sources before scanning.
	skillsShEnabled := false
	realSources := make([]string, 0, len(sources))
	for _, s := range sources {
		if s == steps.SkillsShCuratedValue {
			skillsShEnabled = true
		} else {
			realSources = append(realSources, s)
		}
	}

	if len(realSources) > 0 || skillsShEnabled {
		cp := cachePath()

		// skillsShInput captures the skills.sh ScannedRepoInput built inside
		// the scan function (which runs under the spinner alt-screen).
		var skillsShInput *steps.ScannedRepoInput

		// The scan function runs inside the bubbletea alt-screen spinner.
		// It scans real repos AND detects the tech stack for skills.sh — all
		// under the same spinner so the user never sees a blank terminal.
		scanFn := func(ctx context.Context, srcs []string, cachePath string) ([]steps.ScanResult, error) {
			var repoResults []steps.ScanResult

			// Scan real repos (skip the skills.sh display entry).
			realSrcs := make([]string, 0, len(srcs))
			for _, s := range srcs {
				if s != "Skills.sh Curated" {
					realSrcs = append(realSrcs, s)
				}
			}

			if len(realSrcs) > 0 {
				s, scanErr := ScanRepositories(ctx, realSrcs, cachePath)
				if scanErr != nil {
					return nil, scanErr
				}
				repoResults = make([]steps.ScanResult, len(s))
				for i, r := range s {
					repoResults[i] = steps.ScanResult{
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
			}

			// Detect tech stack for skills.sh (runs inside the spinner).
			// The result is captured via closure — it needs SkillHeaders which
			// ScanResult cannot carry.
			if skillsShEnabled && projectDir != "" {
				if profile, detectErr := detect.Analyze(projectDir); detectErr == nil {
					catalog := recommend.StaticCatalog{}
					if detected, fetchErr := catalog.FetchByProfile(profile); fetchErr == nil {
						skillsShInput = steps.BuildSkillsShInput(detected)
						// Filter adviser-kind skills to project-relevant subset.
						// Only the skills.sh curated path is filtered; user-imported repos are unchanged.
						if skillsShInput != nil {
							skillsShInput.Skills = recommend.FilterAdvisersByProfile(profile, skillsShInput.Skills)
						}
					}
				}
			}

			return repoResults, nil
		}

		// Build the list of sources to show in the spinner.
		scanSources := make([]string, len(realSources))
		copy(scanSources, realSources)
		if skillsShEnabled {
			scanSources = append(scanSources, "Skills.sh Curated")
		}

		var warnings []string
		scanned, warnings, err = steps.RunScanModel(scanSources, scanFn, cp)
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
				sddWFs, restWFs := partitionSDDWorkflows(r.Workflows)
				inputs = append(inputs, steps.ScannedRepoInput{
					Source:                r.Source,
					Skills:                r.Skills,
					Rules:                 r.Rules,
					MCPs:                  r.MCPs,
					Workflows:             restWFs,
					WorkflowManifests:     r.WorkflowManifests,
					Tools:                 r.Tools,
					Descs:                 r.Descs,
					MCPFiles:              r.MCPFiles,
					AutoSelectedWorkflows: sddWFs,
				})
			}
		}

		// Append skills.sh input (with SkillHeaders preserved).
		if skillsShInput != nil {
			inputs = append(inputs, *skillsShInput)
		}

		if len(inputs) > 0 {
			// Detect whether an AI agent (claude/opencode) is available on PATH.
			// This is fast (exec.LookPath) and safe to call before the select loop.
			_, _, agentDetectErr := recommend.DetectAgent()
			agentAvailable := agentDetectErr == nil

			// Loop: select → (optional) AI recommendations → back to select if user declines.
			var prevSelection *steps.SelectionResult // preserves state on go-back
			for {
				var selectResult steps.SelectModelResult
				var err error
				if prevSelection != nil {
					selectResult, err = steps.RunSelectModel(inputs, recommendEnabled, agentAvailable, projectDir, *prevSelection)
				} else {
					selectResult, err = steps.RunSelectModel(inputs, recommendEnabled, agentAvailable, projectDir)
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

	// Build the full catalog sources list from wizard-selected sources.
	// This includes both CLI-provided catalogs and user-imported repos from Step 2.
	// Filter out the Skills.sh sentinel (not a real catalog source).
	allCatalogSources := make([]string, 0, len(sources))
	for _, s := range sources {
		if s != steps.SkillsShCuratedValue {
			allCatalogSources = append(allCatalogSources, s)
		}
	}

	// Final step — summary & confirm (alt screen, step indicator inside form)
	manifest, err := steps.ConfirmSummary(agents, selection, workflowModels, allCatalogSources)
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

// partitionSDDWorkflows splits a workflow list into SDD workflows (auto-selected)
// and non-SDD workflows (user-visible in TUI). The SDD set matches names that are
// exactly "sdd", have prefix "sdd-", or have prefix "sdd " (case-insensitive).
func partitionSDDWorkflows(workflows []string) (sdd []string, rest []string) {
	for _, wf := range workflows {
		lower := strings.ToLower(wf)
		if lower == "sdd" || strings.HasPrefix(lower, "sdd-") || strings.HasPrefix(lower, "sdd ") {
			sdd = append(sdd, wf)
		} else {
			rest = append(rest, wf)
		}
	}
	return
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
