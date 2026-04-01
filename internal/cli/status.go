// SPDX-License-Identifier: MIT

package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/parse"
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

	// Check staleness by comparing the stored manifest hash with the current manifest.
	// The stored LockHash is actually a hash of the manifest YAML at install time.
	manifestPath := filepath.Join(wd, "devrune.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(out, "Status: missing (devrune.yaml not found)\n")
		} else {
			_, _ = fmt.Fprintf(out, "Status: error reading manifest\n")
		}
		return nil
	}

	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		_, _ = fmt.Fprintf(out, "Status: error parsing manifest\n")
		return nil
	}

	// Compute manifest hash the same way the resolver does.
	serialized, err := parse.SerializeManifest(manifest)
	if err != nil {
		_, _ = fmt.Fprintf(out, "Status: error serializing manifest\n")
		return nil
	}
	sum := sha256.Sum256(serialized)
	currentHash := fmt.Sprintf("sha256:%x", sum)

	if currentHash == s.LockHash {
		_, _ = fmt.Fprintf(out, "Status: fresh\n")
	} else {
		_, _ = fmt.Fprintf(out, "Status: stale (manifest changed since last install)\n")
	}

	return nil
}
