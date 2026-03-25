package cli

import (
	"github.com/spf13/cobra"
)

func newResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve packages and produce devrune.lock",
		Long: `Resolve reads devrune.yaml, fetches all referenced packages and workflows
from their source refs (github:, gitlab:, local:), computes content hashes,
and writes the resolved lockfile to devrune.lock.

This is the only command that touches the network.`,
		RunE: runResolve,
	}

	cmd.Flags().String("manifest", "devrune.yaml", "Path to the manifest file")
	cmd.Flags().Bool("offline", false, "Fail if any package is not in the local cache")

	return cmd
}

func runResolve(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)

	manifestFlag, _ := cmd.Flags().GetString("manifest")

	_, err := RunResolve(ctx, wd, manifestFlag, verbose, cmd.OutOrStdout())
	return err
}
