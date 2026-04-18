// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// errConfigureCancelled is returned by runConfigureModelsFromMenu when the user
// cancels the model selector. Callers (e.g. RunMenu) treat this as a silent
// no-op and loop back to the menu without showing any message.
var errConfigureCancelled = errors.New("configure models cancelled")

// updateManifestWorkflowModels writes newModels (map[agent]map[role]string) back into
// all workflow entries in the manifest. Each workflow entry's Roles map is updated
// for every agent/role present in newModels. Entries with no matching roles are left untouched.
// If newModels is nil or empty, existing roles are cleared.
func updateManifestWorkflowModels(manifest *model.UserManifest, newModels map[string]map[string]string) {
	if len(manifest.Workflows) == 0 {
		return
	}
	for name, entry := range manifest.Workflows {
		if len(newModels) == 0 {
			entry.Roles = nil
			manifest.Workflows[name] = entry
			continue
		}
		if entry.Roles == nil {
			entry.Roles = make(map[string]map[string]string)
		}
		for agent, roleMap := range newModels {
			if entry.Roles[agent] == nil {
				entry.Roles[agent] = make(map[string]string)
			}
			for role, val := range roleMap {
				entry.Roles[agent][role] = val
			}
		}
		manifest.Workflows[name] = entry
	}
}

// runConfigureModelsFromMenu runs the standalone model reconfiguration flow from the TUI menu.
// It reads the current manifest, presents the model selector (step 1 of 1), writes the updated
// manifest on confirm, then runs sync+install. On cancel, the manifest is not modified.
// NOTE: steps.TotalSteps must be set to 1 before calling RunWorkflowModelSelection in standalone mode.
func runConfigureModelsFromMenu(cmd *cobra.Command) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	out := cmd.OutOrStdout()
	manifestPath := filepath.Join(wd, "devrune.yaml")

	// 1. Read and parse manifest.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("devrune.yaml not found — run Setup first")
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// 2. Collect qualifying agent names.
	var agentNames []string
	for _, a := range manifest.Agents {
		if model.ModelRoutingAgents[a.Name] {
			agentNames = append(agentNames, a.Name)
		}
	}
	if len(agentNames) == 0 {
		return fmt.Errorf("no model-routing agents configured — run Setup first")
	}

	// 3. Load saved models from existing manifest.
	savedModels := mergeWorkflowModels(manifest.Workflows)

	// 4. Run model selector (standalone: step 1 of 1).
	// NOTE: TotalSteps is a package-level variable; safe for single-threaded CLI use.
	steps.TotalSteps = 1
	newModels, err := steps.RunWorkflowModelSelection(
		agentNames,
		steps.SelectionResult{},
		savedModels,
		nil,  // no workflow manifests in standalone mode
		true, // sddAutoSelected
		1,    // stepNum
	)
	if err != nil {
		if errors.Is(err, steps.ErrModelSelectionCancelled) {
			// User cancelled — return sentinel so RunMenu skips the message.
			return errConfigureCancelled
		}
		return err
	}
	if newModels == nil {
		// No changes made (all inherit — treat as cancel/no-op).
		return errConfigureCancelled
	}

	// 5. Update manifest with new model selections.
	updateManifestWorkflowModels(&manifest, newModels)

	// 6. Serialize and write manifest.
	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		return fmt.Errorf("serialize manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// 7. Sync catalogs, then resolve + install.
	_ = syncCatalogs(manifest, manifestPath)
	lockPath := filepath.Join(wd, "devrune.lock")
	if err := steps.RunInstallSpinner(
		func() error {
			_, resolveErr := RunResolve(ctx, wd, manifestPath, verbose, nopWriter{})
			return resolveErr
		},
		func() error {
			return RunInstall(ctx, wd, lockPath, manifest, verbose, nopWriter{})
		},
	); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	_ = out // reserved for future verbose output
	return nil
}
