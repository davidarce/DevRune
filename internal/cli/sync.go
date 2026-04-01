// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/parse"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Resolve packages and install workspace in one step",
		Long: `Sync is the recommended way to apply changes to your devrune.yaml.
It runs resolve (fetch packages, update devrune.lock) followed by install
(materialize workspace for all agents) in a single command.

Equivalent to running 'devrune resolve && devrune install'.`,
		RunE: runSync,
	}

	cmd.Flags().String("manifest", "devrune.yaml", "Path to the manifest file")
	cmd.Flags().Bool("offline", false, "Fail if any package is not in the local cache")

	return cmd
}

func runSync(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	out := cmd.OutOrStdout()

	manifestFlag, _ := cmd.Flags().GetString("manifest")

	// Check for the case where devrune.catalog.yaml exists but devrune.yaml does not.
	// Suggest running init so users know the next step.
	manifestPath := manifestFlag
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(wd, manifestPath)
	}
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		catalogPath := filepath.Join(wd, "devrune.catalog.yaml")
		if _, cerr := os.Stat(catalogPath); cerr == nil {
			_, _ = fmt.Fprintln(out, "Found devrune.catalog.yaml but no devrune.yaml. Run `devrune init` first.")
		}
	}

	// Step 1: Resolve.
	lockfile, err := RunResolve(ctx, wd, manifestFlag, verbose, out)
	if err != nil {
		return fmt.Errorf("sync resolve: %w", err)
	}

	// Step 2: Read manifest for agent refs and install config.
	// manifestPath is already resolved to an absolute path above.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("sync: read manifest: %w", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("sync: parse manifest: %w", err)
	}

	// Step 3: Install using the lockfile we just resolved.
	// RunInstall reads the lockfile from disk, but we already wrote it in RunResolve.
	// We still need to pass the lockfile path for RunInstall's interface.
	lockPath := filepath.Join(wd, "devrune.lock")

	_ = lockfile // lockfile written to disk by RunResolve; RunInstall reads it back

	if err := RunInstall(ctx, wd, lockPath, manifest, verbose, out); err != nil {
		return fmt.Errorf("sync install: %w", err)
	}

	return nil
}
