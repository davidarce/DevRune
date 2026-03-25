package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// workingDir reads the --dir flag and resolves it to an absolute path.
// Falls back to the current working directory if resolution fails.
func workingDir(cmd *cobra.Command) string {
	dir, _ := cmd.Root().PersistentFlags().GetString("dir")
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// isVerbose reads the --verbose flag from the root command.
func isVerbose(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
	return v
}

// isNonInteractive reads the --non-interactive flag from the root command.
func isNonInteractive(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("non-interactive")
	return v
}

// cachePath returns the default cache directory: ~/.cache/devrune/packages/
func cachePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		// Fallback to ~/.cache if os.UserCacheDir fails.
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "devrune", "packages")
}
