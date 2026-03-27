// SPDX-License-Identifier: MIT

package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/state"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workspace installation state",
		Long: `Status reads .devrune/state.yaml and prints the current workspace state:
installed agents, managed paths, active workflows, and whether the lockfile
is in sync with the installed state.`,
		RunE: runStatus,
	}

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	wd := workingDir(cmd)
	out := cmd.OutOrStdout()

	stateMgr := state.NewFileStateManager(wd)

	s, err := stateMgr.Read()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	// Zero-value state means no installation has been performed yet.
	if s.SchemaVersion == "" && len(s.ActiveAgents) == 0 {
		_, _ = fmt.Fprintln(out, "No installation found.")
		_, _ = fmt.Fprintf(out, "Run 'devrune resolve' then 'devrune install' to get started.\n")
		return nil
	}

	_, _ = fmt.Fprintf(out, "Schema version:  %s\n", s.SchemaVersion)
	if s.InstalledAt != "" {
		_, _ = fmt.Fprintf(out, "Installed at:    %s\n", s.InstalledAt)
	}
	_, _ = fmt.Fprintf(out, "Lock hash:       %s\n", s.LockHash)

	if len(s.ActiveAgents) > 0 {
		_, _ = fmt.Fprintf(out, "Active agents:   %v\n", s.ActiveAgents)
	} else {
		_, _ = fmt.Fprintf(out, "Active agents:   (none)\n")
	}

	if len(s.ActiveWorkflows) > 0 {
		_, _ = fmt.Fprintf(out, "Active workflows:%v\n", s.ActiveWorkflows)
	} else {
		_, _ = fmt.Fprintf(out, "Active workflows:(none)\n")
	}

	_, _ = fmt.Fprintf(out, "Managed paths:   %d\n", len(s.ManagedPaths))

	// Check lockfile staleness.
	lockPath := filepath.Join(wd, "devrune.lock")
	lockData, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(out, "Lockfile status: missing (devrune.lock not found)\n")
		} else {
			_, _ = fmt.Fprintf(out, "Lockfile status: error reading lockfile\n")
		}
		return nil
	}

	// Compute SHA256 of the lockfile content and compare with the stored LockHash.
	sum := sha256.Sum256(lockData)
	lockfileHash := fmt.Sprintf("sha256:%x", sum)
	if lockfileHash == s.LockHash {
		_, _ = fmt.Fprintf(out, "Status: fresh\n")
	} else {
		_, _ = fmt.Fprintf(out, "Status: stale (lockfile changed since last install)\n")
	}

	return nil
}
