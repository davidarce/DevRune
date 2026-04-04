// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
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

	// Step 1: Read manifest for agent refs and install config.
	manifestPath := manifestFlag
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(wd, manifestPath)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("sync: read manifest: %w", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("sync: parse manifest: %w", err)
	}

	// Step 2: Derive catalogs from sources and update manifest before resolving.
	// This must happen before RunResolve so the lock hash matches the installed state.
	if err := syncCatalogs(manifest, manifestPath); err != nil {
		_, _ = fmt.Fprintf(out, "Warning: could not update catalogs: %v\n", err)
	}

	// Step 3: Resolve.
	lockfile, err := RunResolve(ctx, wd, manifestPath, verbose, out)
	if err != nil {
		return fmt.Errorf("sync resolve: %w", err)
	}

	// Step 4: Install using the lockfile we just resolved.
	// RunInstall reads the lockfile from disk, but we already wrote it in RunResolve.
	// We still need to pass the lockfile path for RunInstall's interface.
	lockPath := filepath.Join(wd, "devrune.lock")

	_ = lockfile // lockfile written to disk by RunResolve; RunInstall reads it back

	if err := RunInstall(ctx, wd, lockPath, manifest, verbose, out); err != nil {
		return fmt.Errorf("sync install: %w", err)
	}

	return nil
}

// syncCatalogs derives catalog root refs from all source refs in the manifest
// (packages, mcps, workflows) and writes them to the catalogs: key if any are
// missing. This keeps the manifest self-documenting about which catalogs it uses.
func syncCatalogs(manifest model.UserManifest, manifestPath string) error {
	derived := deriveCatalogRoots(manifest)
	if len(derived) == 0 {
		return nil
	}

	// Check if all derived catalogs are already present.
	existing := make(map[string]bool, len(manifest.Catalogs))
	for _, c := range manifest.Catalogs {
		existing[c] = true
	}

	var missing []string
	for _, d := range derived {
		if !existing[d] {
			missing = append(missing, d)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	// Merge missing catalogs into the manifest and rewrite.
	manifest.Catalogs = append(manifest.Catalogs, missing...)

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Prepend schemaVersion line (yaml.Marshal puts it inline but we want clean output).
	return os.WriteFile(manifestPath, data, 0o644)
}

// deriveCatalogRoots extracts unique catalog root refs from all source refs.
// For "local:/path/to/catalog/mcps/x.yaml" → "local:/path/to/catalog"
// For "github:owner/repo@ref//subpath" → "github:owner/repo@ref"
// For "github:owner/repo@ref" → "github:owner/repo@ref"
func deriveCatalogRoots(manifest model.UserManifest) []string {
	seen := make(map[string]bool)
	var roots []string

	addRoot := func(sourceRef string) {
		root := extractCatalogRoot(sourceRef)
		if root != "" && !seen[root] {
			seen[root] = true
			roots = append(roots, root)
		}
	}

	for _, pkg := range manifest.Packages {
		addRoot(pkg.Source)
	}
	for _, mcp := range manifest.MCPs {
		addRoot(mcp.Source)
	}
	for _, wf := range manifest.Workflows {
		addRoot(wf.Source)
	}

	return roots
}

// extractCatalogRoot extracts the catalog root from a source ref.
//
// Patterns:
//   - "github:owner/repo@ref//subpath" → "github:owner/repo@ref"
//   - "github:owner/repo@ref" → "github:owner/repo@ref"
//   - "gitlab:owner/repo@ref//subpath" → "gitlab:owner/repo@ref"
//   - "local:/path/to/catalog" → "local:/path/to/catalog"
//   - "local:/path/to/catalog/mcps/x.yaml" → strips /mcps/*, /workflows/*, /skills/*
func extractCatalogRoot(sourceRef string) string {
	if sourceRef == "" {
		return ""
	}

	// GitHub/GitLab: strip //subpath
	if strings.HasPrefix(sourceRef, "github:") || strings.HasPrefix(sourceRef, "gitlab:") {
		if idx := strings.Index(sourceRef, "//"); idx != -1 {
			return sourceRef[:idx]
		}
		return sourceRef
	}

	// Local: strip known subdirectories
	if strings.HasPrefix(sourceRef, "local:") {
		path := strings.TrimPrefix(sourceRef, "local:")
		// Strip /mcps/*, /workflows/*, /skills/*, /rules/* suffixes
		for _, marker := range []string{"/mcps/", "/workflows/", "/skills/", "/rules/", "/tools/"} {
			if idx := strings.Index(path, marker); idx != -1 {
				return "local:" + path[:idx]
			}
		}
		return sourceRef
	}

	return sourceRef
}
