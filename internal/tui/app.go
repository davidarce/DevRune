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

// RunResult holds all outputs from the interactive TUI wizard.
type RunResult struct {
	Manifest       model.UserManifest
	InstalledTools []string // tool names successfully installed via brew
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
// If the user aborts at any step (Ctrl-C or declining confirmation) the
// function returns huh.ErrUserAborted.
func Run() (RunResult, error) {
	// Step 1 — agents (alt screen, step indicator inside form)
	agents, err := steps.SelectAgents()
	if err != nil {
		return RunResult{}, mapErr(err)
	}

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
	if hasQualifyingAgent {
		steps.TotalSteps = 6
	} else {
		steps.TotalSteps = 5
	}

	// Step 2 — repository sources (alt screen, step indicator inside form)
	sources, err := steps.EnterRepositories()
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	// Step 3 — scan + select (alt screen)
	var selection steps.SelectionResult
	var scanned []steps.ScanResult

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
					Source:    r.Source,
					Skills:    r.Skills,
					Rules:     r.Rules,
					MCPs:      r.MCPs,
					Workflows: r.Workflows,
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
					Tools:     r.Tools,
					Error:     r.Error,
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
		inputs := make([]steps.ScannedRepoInput, 0, len(scanned))
		for _, r := range scanned {
			if r.Error == nil {
				inputs = append(inputs, steps.ScannedRepoInput{
					Source:    r.Source,
					Skills:    r.Skills,
					Rules:     r.Rules,
					MCPs:      r.MCPs,
					Workflows: r.Workflows,
					Tools:     r.Tools,
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
				})
			}
		}

		if len(inputs) > 0 {
			selection, err = steps.RunSelectModel(inputs)
			if err != nil {
				return RunResult{}, mapErr(err)
			}
		}
	}

	// Now that selection is known, confirm whether the SDD step will actually run.
	// If no workflows were selected, TotalSteps reverts to 5 even for qualifying agents.
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
	// Load saved models from existing devrune.yaml if present.
	var savedModels map[string]map[string]string
	savedManifest := loadExistingManifest()
	if savedManifest != nil {
		savedModels = savedManifest.WorkflowModels
	}

	// TODO: resolve workflow manifests from selection to pass to RunWorkflowModelSelection.
	// For now, pass nil — the function will skip if no roles have models.
	workflowModels, err := steps.RunWorkflowModelSelection(agents, selection, savedModels, nil)
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
	manifest, err := steps.ConfirmSummary(agents, selection, workflowModels)
	if err != nil {
		return RunResult{}, mapErr(err)
	}

	return RunResult{Manifest: manifest, InstalledTools: installedTools}, nil
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
