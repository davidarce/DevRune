// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// runBackupsFromMenu is the dispatcher for the Backups menu action.
// It wires the menu selection to steps.RunRestoreStep by constructing
// the working directory, manifest path, and an installFn that runs
// resolve + install against the (possibly just-restored) devrune.yaml.
func runBackupsFromMenu(cmd *cobra.Command) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	manifestPath := filepath.Join(wd, "devrune.yaml")
	lockPath := filepath.Join(wd, "devrune.lock")

	// installFn runs resolve then install after the manifest has been replaced.
	// It re-reads the manifest from disk so it reflects the restored content.
	installFn := func() error {
		// Re-resolve so the lockfile matches the restored manifest.
		_, err := RunResolve(ctx, wd, manifestPath, verbose, nopWriter{})
		if err != nil {
			return fmt.Errorf("resolve after restore: %w", err)
		}

		// Re-read manifest for RunInstall (the lockfile was just regenerated).
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("read manifest after restore: %w", err)
		}
		manifest, err := parse.ParseManifest(manifestData)
		if err != nil {
			return fmt.Errorf("parse manifest after restore: %w", err)
		}

		if err := RunInstall(ctx, wd, lockPath, manifest, verbose, nopWriter{}); err != nil {
			return fmt.Errorf("install after restore: %w", err)
		}
		return nil
	}

	_, err := steps.RunRestoreStep(wd, manifestPath, installFn)
	return err
}
