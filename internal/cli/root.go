// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates and configures the Cobra root command with all subcommands.
// version and commit are injected at build time via ldflags.
func NewRootCmd(version, commit string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "devrune",
		Short: "DevRune — AI agent configuration manager",
		Long: `DevRune configures AI development agents (Claude, OpenCode, Copilot, Factory)
by resolving, fetching, and materializing packages of skills, rules, MCP server
definitions, and workflows into developer workspaces.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isNonInteractive(cmd) {
				return cmd.Help()
			}
			return RunMenu(cmd)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().Bool("non-interactive", false, "Disable interactive prompts (for CI/automation)")
	rootCmd.PersistentFlags().String("dir", ".", "Working directory (where devrune.yaml is located)")

	// Register subcommands
	rootCmd.AddCommand(
		newInitCmd(),
		newResolveCmd(),
		newInstallCmd(),
		newSyncCmd(),
		newStatusCmd(),
		newVersionCmd(version, commit),
		newUninstallCmd(),
		newUpgradeCmd(),
		newCacheCmd(),
	)

	return rootCmd
}
