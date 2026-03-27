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

// Run executes the interactive TUI wizard and returns the resulting
// UserManifest on success.
//
// The wizard walks the user through four sequential steps:
//
//	Step 1: Agent selection
//	Step 2: Repository source refs (one at a time)
//	Step 3: Scan + category/item selection
//	Step 4: Summary + confirmation
//
// If the user aborts at any step (Ctrl-C or declining confirmation) the
// function returns huh.ErrUserAborted.
func Run() (model.UserManifest, error) {
	// Step 1 — agents (alt screen, step indicator inside form)
	agents, err := steps.SelectAgents()
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	// Determine early if the SDD model selection step may be active.
	// The step requires at least one qualifying agent (in SDDModelRoutingAgents)
	// AND at least one workflow selected. We know the agent condition now; workflow
	// selection is only known after Step 3. Set TotalSteps to 5 if qualifying
	// agents exist so that Steps 2 and 3 already show the correct total.
	// After Step 3 we can confirm (or revert to 4) based on the actual selection.
	hasQualifyingAgent := false
	for _, a := range agents {
		if model.SDDModelRoutingAgents[a] {
			hasQualifyingAgent = true
			break
		}
	}
	if hasQualifyingAgent {
		steps.TotalSteps = 5
	} else {
		steps.TotalSteps = 4
	}

	// Step 2 — repository sources (alt screen, step indicator inside form)
	sources, err := steps.EnterRepositories()
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	// Step 3 — scan + select (alt screen)
	var selection steps.SelectionResult

	if len(sources) > 0 {
		cp := cachePath()

		// Wrap ScanRepositories so the steps package can call it without
		// importing the tui package (avoids import cycle).
		scanFn := func(ctx context.Context, srcs []string, cachePath string) ([]steps.ScanResult, error) {
			scanned, err := ScanRepositories(ctx, srcs, cachePath)
			if err != nil {
				return nil, err
			}
			out := make([]steps.ScanResult, len(scanned))
			for i, r := range scanned {
				out[i] = steps.ScanResult{
					Source:    r.Source,
					Skills:    r.Skills,
					Rules:     r.Rules,
					MCPs:      r.MCPs,
					Workflows: r.Workflows,
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
					Error:     r.Error,
				}
			}
			return out, nil
		}

		scanned, warnings, err := steps.RunScanModel(sources, scanFn, cp)
		if err != nil {
			return model.UserManifest{}, fmt.Errorf("scan repositories: %w", err)
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
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
				})
			}
		}

		if len(inputs) > 0 {
			selection, err = steps.RunSelectModel(inputs)
			if err != nil {
				return model.UserManifest{}, mapErr(err)
			}
		}
	}

	// Now that selection is known, confirm whether the SDD step will actually run.
	// If no workflows were selected, TotalSteps reverts to 4 even for qualifying agents.
	if hasQualifyingAgent {
		hasWorkflow := false
		for _, repo := range selection.Repos {
			if len(repo.SelectedWorkflows) > 0 {
				hasWorkflow = true
				break
			}
		}
		if !hasWorkflow {
			steps.TotalSteps = 4
		}
	}

	// Step 4 (optional) — SDD model selection
	// Load saved models from existing devrune.yaml if present.
	var savedModels map[string]map[string]string
	savedManifest := loadExistingManifest()
	if savedManifest != nil {
		savedModels = savedManifest.SDDModels
	}

	sddModels, err := steps.RunSDDModelSelection(agents, selection, savedModels)
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	// Final confirmation: sync TotalSteps with the actual outcome of RunSDDModelSelection.
	if sddModels != nil {
		steps.TotalSteps = 5
	} else {
		steps.TotalSteps = 4
	}

	// Final step — summary & confirm (alt screen, step indicator inside form)
	manifest, err := steps.ConfirmSummary(agents, selection, sddModels)
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	return manifest, nil
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
