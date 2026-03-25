package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/parse"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Materialize the workspace from devrune.lock",
		Long: `Install reads devrune.lock and materializes the workspace for all configured
agents. Skills, rules, MCP configurations, and workflows are installed into each
agent's workspace directory.

Always works from the lockfile — run 'devrune resolve' first to update it.`,
		RunE: runInstall,
	}

	cmd.Flags().Bool("locked", false, "Fail if devrune.lock does not exist (default: auto-resolve if missing)")
	cmd.Flags().Bool("offline", false, "Do not attempt network access during install")

	return cmd
}

func runInstall(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	locked, _ := cmd.Flags().GetBool("locked")

	lockPath := filepath.Join(wd, "devrune.lock")
	manifestPath := filepath.Join(wd, "devrune.yaml")

	// Validate lockfile exists before proceeding.
	if _, err := os.Stat(lockPath); err != nil {
		if os.IsNotExist(err) {
			if locked {
				return fmt.Errorf("devrune.lock not found in %s (run 'devrune resolve' first)", wd)
			}
			return fmt.Errorf("devrune.lock not found in %s — run 'devrune resolve' first", wd)
		}
		return fmt.Errorf("stat lockfile: %w", err)
	}

	// Read manifest for agent refs and install config.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("devrune.yaml not found in %s", wd)
		}
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	if verbose {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Loaded manifest: %s\n", manifestPath)
	}

	return RunInstall(ctx, wd, lockPath, manifest, verbose, cmd.OutOrStdout())
}
