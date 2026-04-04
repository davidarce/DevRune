// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/davidarce/devrune/internal/cache"
	"github.com/davidarce/devrune/internal/materialize"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/resolve"
	"github.com/davidarce/devrune/internal/state"
)

// RunResolve executes the resolve pipeline: reads the manifest, fetches all
// referenced packages, and writes devrune.lock. It returns the Lockfile on
// success.
//
// manifestPath is the path to devrune.yaml (absolute or relative to workDir).
// If verbose is true, progress lines are written to out.
func RunResolve(ctx context.Context, workDir string, manifestPath string, verbose bool, out io.Writer) (model.Lockfile, error) {
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(workDir, manifestPath)
	}

	if verbose {
		_, _ = fmt.Fprintf(out, "Reading manifest: %s\n", manifestPath)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Lockfile{}, fmt.Errorf("manifest not found: %s", manifestPath)
		}
		return model.Lockfile{}, fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return model.Lockfile{}, fmt.Errorf("parse manifest: %w", err)
	}

	cacheDir := cachePath()
	if verbose {
		_, _ = fmt.Fprintf(out, "Cache directory: %s\n", cacheDir)
	}
	cacheStore := cache.NewFileCacheStore(cacheDir)

	githubFetcher := cache.NewGitHubFetcher("")
	gitlabFetcher := cache.NewGitLabFetcher("")
	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(githubFetcher, gitlabFetcher, localFetcher)

	resolver := resolve.NewResolver(multiFetcher, cacheStore, workDir)

	_, _ = fmt.Fprintf(out, "Resolving packages...")

	lockfile, err := resolver.Resolve(ctx, manifest)
	if err != nil {
		return model.Lockfile{}, fmt.Errorf("resolve: %w", err)
	}

	lockData, err := parse.SerializeLockfile(lockfile)
	if err != nil {
		return model.Lockfile{}, fmt.Errorf("serialize lockfile: %w", err)
	}

	lockPath := filepath.Join(workDir, "devrune.lock")
	if err := os.WriteFile(lockPath, lockData, 0o644); err != nil {
		return model.Lockfile{}, fmt.Errorf("write lockfile %s: %w", lockPath, err)
	}

	_, _ = fmt.Fprintf(out, " done\n")

	// Print summary.
	skillCount, ruleCount := countContents(lockfile)
	_, _ = fmt.Fprintf(out, "  packages: %d, MCPs: %d, workflows: %d\n",
		len(lockfile.Packages), len(lockfile.MCPs), len(lockfile.Workflows))
	_, _ = fmt.Fprintf(out, "  skills: %d, rules: %d\n", skillCount, ruleCount)
	_, _ = fmt.Fprintf(out, "  lockfile: %s\n", lockPath)

	return lockfile, nil
}

// RunInstall executes the install pipeline: reads the lockfile and manifest,
// then materializes the workspace for all configured agents.
func RunInstall(ctx context.Context, workDir string, lockfilePath string, manifest model.UserManifest, verbose bool, out io.Writer) error {
	if !filepath.IsAbs(lockfilePath) {
		lockfilePath = filepath.Join(workDir, lockfilePath)
	}

	lockData, err := os.ReadFile(lockfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("lockfile not found: %s — run 'devrune resolve' first", lockfilePath)
		}
		return fmt.Errorf("read lockfile: %w", err)
	}

	lockfile, err := parse.ParseLockfile(lockData)
	if err != nil {
		return fmt.Errorf("parse lockfile: %w", err)
	}

	if verbose {
		_, _ = fmt.Fprintf(out, "Loaded lockfile: %s\n", lockfilePath)
	}

	cacheDir := cachePath()
	cacheStore := cache.NewFileCacheStore(cacheDir)

	stateMgr := state.NewFileStateManager(workDir)

	linker, err := materialize.NewLinker(manifest.Install.LinkMode)
	if err != nil {
		return fmt.Errorf("create linker: %w", err)
	}

	if verbose {
		_, _ = fmt.Fprintf(out, "Link mode: %s\n", linker.Mode())
	}

	renderers, err := materialize.LoadDefaultRegistry()
	if err != nil {
		return fmt.Errorf("load renderer registry: %w", err)
	}

	materializer := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	_, _ = fmt.Fprintf(out, "Installing workspace...")

	if err := materializer.Install(ctx, lockfile, manifest.Agents, manifest.Install, workDir, extractWorkflowModels(manifest)); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	_, _ = fmt.Fprintf(out, " done\n")

	agentNames := make([]string, 0, len(manifest.Agents))
	for _, a := range manifest.Agents {
		agentNames = append(agentNames, a.Name)
	}

	skillCount, _ := countContents(lockfile)
	_, _ = fmt.Fprintf(out, "  agents: %v\n", agentNames)
	_, _ = fmt.Fprintf(out, "  skills: %d, workflows: %d\n", skillCount, len(lockfile.Workflows))
	_, _ = fmt.Fprintf(out, "\nReady! Your AI agent workspace is configured.\n")

	return nil
}

// extractWorkflowModels merges role model overrides from all workflow entries in the manifest.
// The result is a flat map[agentName]map[roleName]modelValue suitable for passing to the materializer.
// When two workflows define the same agent/role pair, the last one wins (map iteration order).
func extractWorkflowModels(manifest model.UserManifest) map[string]map[string]string {
	var merged map[string]map[string]string
	for _, entry := range manifest.Workflows {
		if len(entry.Roles) == 0 {
			continue
		}
		if merged == nil {
			merged = make(map[string]map[string]string)
		}
		for agent, roles := range entry.Roles {
			if merged[agent] == nil {
				merged[agent] = make(map[string]string)
			}
			for role, modelVal := range roles {
				merged[agent][role] = modelVal
			}
		}
	}
	return merged
}

// countContents tallies skills and rules across all packages in a lockfile.
func countContents(lf model.Lockfile) (skills, rules int) {
	for _, pkg := range lf.Packages {
		for _, item := range pkg.Contents {
			switch item.Kind {
			case "skill":
				skills++
			case "rule":
				rules++
			}
		}
	}
	return
}
