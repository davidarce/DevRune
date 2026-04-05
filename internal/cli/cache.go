// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local DevRune cache",
		Long:  `Commands for inspecting and managing the local DevRune cache.`,
	}

	cmd.AddCommand(newCacheCleanCmd())

	return cmd
}

func newCacheCleanCmd() *cobra.Command {
	var recommendOnly bool
	var packagesOnly bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove cached packages and/or recommendation data",
		Long: `Clean removes cached data from the local DevRune cache.

By default both the package cache and the recommendation cache are removed.
Use --packages-only or --recommend-only to limit the scope.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheClean(cmd, recommendOnly, packagesOnly)
		},
	}

	cmd.Flags().BoolVar(&recommendOnly, "recommend-only", false, "Only clean the recommendation cache")
	cmd.Flags().BoolVar(&packagesOnly, "packages-only", false, "Only clean the package cache")

	return cmd
}

// cacheBasePath returns the devrune base cache directory: ~/.cache/devrune/
func cacheBasePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "devrune")
}

func runCacheClean(cmd *cobra.Command, recommendOnly, packagesOnly bool) error {
	out := cmd.OutOrStdout()

	base := cacheBasePath()
	packagesDir := filepath.Join(base, "packages")
	recommendDir := filepath.Join(base, "recommend")

	cleanPackages := !recommendOnly
	cleanRecommend := !packagesOnly

	var totalFreed int64

	if cleanPackages {
		freed, err := removeDir(packagesDir)
		if err != nil {
			return fmt.Errorf("clean packages cache: %w", err)
		}
		if freed >= 0 {
			_, _ = fmt.Fprintf(out, "Cleaned package cache (%s): %s freed\n", packagesDir, formatBytes(freed))
		} else {
			_, _ = fmt.Fprintf(out, "Package cache not found, skipping.\n")
		}
		totalFreed += max64(freed, 0)
	}

	if cleanRecommend {
		freed, err := removeDir(recommendDir)
		if err != nil {
			return fmt.Errorf("clean recommendation cache: %w", err)
		}
		if freed >= 0 {
			_, _ = fmt.Fprintf(out, "Cleaned recommendation cache (%s): %s freed\n", recommendDir, formatBytes(freed))
		} else {
			_, _ = fmt.Fprintf(out, "Recommendation cache not found, skipping.\n")
		}
		totalFreed += max64(freed, 0)
	}

	_, _ = fmt.Fprintf(out, "Total space freed: %s\n", formatBytes(totalFreed))
	return nil
}

// removeDir calculates the total size of dir, removes it, and returns the bytes freed.
// Returns -1 if the directory does not exist (nothing to clean).
func removeDir(dir string) (int64, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return -1, nil
	}

	size, err := dirSize(dir)
	if err != nil {
		return 0, fmt.Errorf("calculate size of %s: %w", dir, err)
	}

	if err := os.RemoveAll(dir); err != nil {
		return 0, fmt.Errorf("remove %s: %w", dir, err)
	}

	return size, nil
}

// dirSize returns the total size in bytes of all files under root.
func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// formatBytes returns a human-readable representation of a byte count.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
