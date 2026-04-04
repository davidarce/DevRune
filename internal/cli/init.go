// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize, resolve, and install in one step (interactive TUI wizard or --non-interactive)",
		Long: `Initialize a new devrune.yaml manifest for the current project, then automatically
resolve packages and install the workspace.

In interactive mode, a TUI wizard guides you through selecting agents and repository
sources. In non-interactive mode, use flags to specify all options.`,
		RunE:          runInit,
		SilenceErrors: true, // we handle error display ourselves
		SilenceUsage:  true,
	}

	cmd.Flags().StringSlice("agents", nil, "Agent names to configure (e.g. claude,opencode)")
	cmd.Flags().StringArray("source", nil, "Repository source refs (repeatable, e.g. github:owner/repo@v1)")
	cmd.Flags().StringArray("mcp", nil, "MCP server source refs (repeatable)")
	cmd.Flags().StringArray("workflow", nil, "Workflow source refs (repeatable)")
	cmd.Flags().Bool("force", false, "Overwrite existing devrune.yaml without prompting")
	cmd.Flags().StringArray("catalog", nil, "Catalog source refs (repeatable, e.g. github:org/catalog)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	nonInteractive := isNonInteractive(cmd)
	verbose := isVerbose(cmd)
	force, _ := cmd.Flags().GetBool("force")

	agentNames, _ := cmd.Flags().GetStringSlice("agents")
	sources, _ := cmd.Flags().GetStringArray("source")
	mcps, _ := cmd.Flags().GetStringArray("mcp")
	workflows, _ := cmd.Flags().GetStringArray("workflow")
	catalogFlags, _ := cmd.Flags().GetStringArray("catalog")

	// Determine whether any flags were explicitly provided.
	hasFlags := cmd.Flags().Changed("agents") ||
		cmd.Flags().Changed("source") ||
		cmd.Flags().Changed("mcp") ||
		cmd.Flags().Changed("workflow") ||
		cmd.Flags().Changed("catalog")

	out := cmd.OutOrStdout()
	destPath := filepath.Join(wd, "devrune.yaml")

	// Resolve catalog sources: merge --catalog flag values with catalogs from
	// the existing manifest (if devrune.yaml already exists). CLI flags take
	// precedence; existing manifest catalogs are appended if not already present.
	var catalogSources []string
	{
		seen := make(map[string]bool)
		for _, src := range catalogFlags {
			if src != "" && !seen[src] {
				seen[src] = true
				catalogSources = append(catalogSources, src)
			}
		}
		// Read catalogs from existing manifest if present.
		if existingData, err := os.ReadFile(destPath); err == nil {
			var existing model.UserManifest
			if err := yaml.Unmarshal(existingData, &existing); err == nil {
				for _, src := range existing.Catalogs {
					if src != "" && !seen[src] {
						seen[src] = true
						catalogSources = append(catalogSources, src)
					}
				}
			}
		}
	}

	// Build ExistingConfig from existing manifest if present (for smart config merge).
	// In interactive mode this replaces the old "Overwrite?" confirm prompt.
	var existing *tui.ExistingConfig
	if existingData, err := os.ReadFile(destPath); err == nil {
		var existingManifest model.UserManifest
		if err := yaml.Unmarshal(existingData, &existingManifest); err == nil {
			existingAgents := make([]string, 0, len(existingManifest.Agents))
			for _, a := range existingManifest.Agents {
				if a.Name != "" {
					existingAgents = append(existingAgents, a.Name)
				}
			}
			existingSources := make([]string, 0, len(existingManifest.Packages))
			for _, p := range existingManifest.Packages {
				if p.Source != "" {
					existingSources = append(existingSources, p.Source)
				}
			}
			existing = &tui.ExistingConfig{
				Agents:         existingAgents,
				Sources:        existingSources,
				WorkflowModels: mergeWorkflowModels(existingManifest.Workflows),
			}
		}
	}

	// Check if devrune.yaml already exists in non-interactive / force scenarios.
	if _, statErr := os.Stat(destPath); statErr == nil {
		if nonInteractive || hasFlags {
			if !force {
				printError(out, fmt.Sprintf("%s already exists — use --force to overwrite", destPath))
				return fmt.Errorf("%s already exists", destPath)
			}
		}
		// Interactive mode: no "Overwrite?" prompt — wizard re-runs with preselected values.
	}

	var manifest model.UserManifest

	if !nonInteractive && !hasFlags {
		// Interactive mode: launch TUI wizard with preselection from existing config (if any).
		result, err := tui.Run(catalogSources, existing)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				_, _ = fmt.Fprintln(out, "Aborted.")
				return nil
			}
			printError(out, "Wizard failed: "+err.Error())
			return err
		}
		manifest = result.Manifest
	} else {
		// Non-interactive / flag-based path.
		// Merge catalog sources into --source flag values. Explicit --source flags are
		// added first; catalog sources are appended if not already present.
		mergedSources := make([]string, 0, len(sources)+len(catalogSources))
		seen := make(map[string]bool, len(sources))
		for _, src := range sources {
			if src != "" {
				seen[src] = true
				mergedSources = append(mergedSources, src)
			}
		}
		for _, src := range catalogSources {
			if src != "" && !seen[src] {
				mergedSources = append(mergedSources, src)
			}
		}
		sources = mergedSources

		agentRefs := make([]model.AgentRef, 0, len(agentNames))
		for _, name := range agentNames {
			if name != "" {
				agentRefs = append(agentRefs, model.AgentRef{Name: name})
			}
		}

		pkgRefs := make([]model.PackageRef, 0, len(sources))
		for _, src := range sources {
			if src != "" {
				pkgRefs = append(pkgRefs, model.PackageRef{Source: src})
			}
		}

		mcpRefs := make([]model.MCPRef, 0, len(mcps))
		for _, m := range mcps {
			if m != "" {
				mcpRefs = append(mcpRefs, model.MCPRef{Source: m})
			}
		}

		workflowEntries := buildWorkflowEntries(workflows)
		manifest = model.UserManifest{
			SchemaVersion: "devrune/v1",
			Agents:        agentRefs,
			Packages:      pkgRefs,
			MCPs:          mcpRefs,
			Workflows:     workflowEntries,
			Catalogs:      catalogSources,
		}
	}

	if err := manifest.Validate(); err != nil {
		printError(out, "Validation failed: "+err.Error())
		return err
	}

	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		printError(out, "Serialize manifest: "+err.Error())
		return err
	}

	// --- Styled installation output ---
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSubtitle.Render("  Installing..."))
	_, _ = fmt.Fprintln(out)

	// Write manifest.
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		printError(out, "Write manifest: "+err.Error())
		return err
	}
	printDone(out, "Manifest written: "+destPath)

	agentSummary := make([]string, 0, len(manifest.Agents))
	for _, a := range manifest.Agents {
		agentSummary = append(agentSummary, a.Name)
	}

	// Resolve + install if there are packages to fetch.
	if len(manifest.Packages) > 0 || len(manifest.MCPs) > 0 || len(manifest.Workflows) > 0 {
		// Derive catalogs from sources and update manifest before resolving.
		// This must happen before RunResolve so the lock hash matches the installed state.
		_ = syncCatalogs(manifest, destPath)

		// Resolve + install via bubbletea spinner.
		var lockfile model.Lockfile
		lockPath := filepath.Join(wd, "devrune.lock")
		if err := steps.RunInstallSpinner(
			func() error {
				lf, resolveErr := RunResolve(ctx, wd, destPath, verbose, nopWriter{})
				if resolveErr != nil {
					return resolveErr
				}
				lockfile = lf
				return nil
			},
			func() error {
				return RunInstall(ctx, wd, lockPath, manifest, verbose, nopWriter{})
			},
		); err != nil {
			return err
		}
		_ = lockfile
	} else {
		_, _ = fmt.Fprintln(out, tuistyles.StyleInfo.Render("  No packages to resolve."))
	}

	// Non-interactive / flag-based: plain text summary.
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Render("  Installation complete!"))
	_, _ = fmt.Fprintln(out)
	printSummaryLine(out, "Agents", strings.Join(agentSummary, ", "))
	if len(manifest.Packages) > 0 {
		printSummaryLine(out, "Repos", fmt.Sprintf("%d", len(manifest.Packages)))
	}
	if len(manifest.MCPs) > 0 {
		printSummaryLine(out, "MCPs", fmt.Sprintf("%d", len(manifest.MCPs)))
	}
	if len(manifest.Workflows) > 0 {
		printSummaryLine(out, "Workflows", fmt.Sprintf("%d", len(manifest.Workflows)))
	}
	printSummaryLine(out, "Manifest", destPath)
	printSummaryLine(out, "Lockfile", filepath.Join(wd, "devrune.lock"))
	_, _ = fmt.Fprintln(out)

	return nil
}


// printDone writes a styled "completed" step line with a green checkmark.
func printDone(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Foreground(tuistyles.ColorSuccess).Render("  ✓ ")+tuistyles.StyleSummaryValue.Render(msg))
}

// printError writes a styled error line with a red cross.
func printError(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, tuistyles.StyleError.Render("  ✗ "+msg))
}

// printSummaryLine writes a styled key-value summary line.
func printSummaryLine(out io.Writer, key, value string) {
	padded := fmt.Sprintf("%-12s", key+":")
	_, _ = fmt.Fprintln(out, "  "+tuistyles.StyleSummaryKey.Render(padded)+tuistyles.StyleSummaryValue.Render(value))
}

// buildWorkflowEntries converts a list of workflow source ref strings (from --workflow flags)
// into a map[name]WorkflowEntry suitable for UserManifest.Workflows.
// The workflow name is derived from the last path component of the source ref.
func buildWorkflowEntries(sources []string) map[string]model.WorkflowEntry {
	if len(sources) == 0 {
		return nil
	}
	entries := make(map[string]model.WorkflowEntry, len(sources))
	for _, src := range sources {
		if src == "" {
			continue
		}
		name := workflowNameFromSource(src)
		entries[name] = model.WorkflowEntry{Source: src}
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// workflowNameFromSource derives a workflow name from a source ref string.
// Uses the last path component after the "//" subpath separator, or the last
// slash-separated segment otherwise.
func workflowNameFromSource(src string) string {
	// Strip subpath after "//".
	if idx := strings.Index(src, "//"); idx >= 0 {
		sub := src[idx+2:]
		// e.g. "workflows/sdd" → last component "sdd"
		if last := strings.LastIndex(sub, "/"); last >= 0 {
			return sub[last+1:]
		}
		return sub
	}
	// No subpath: use last segment before any "@" ref.
	noRef := src
	if at := strings.Index(src, "@"); at >= 0 {
		noRef = src[:at]
	}
	if last := strings.LastIndex(noRef, "/"); last >= 0 {
		return noRef[last+1:]
	}
	return noRef
}

// mergeWorkflowModels merges per-agent role model overrides from all workflow entries
// into a flat map[agentName]map[roleName]modelValue.
func mergeWorkflowModels(workflows map[string]model.WorkflowEntry) map[string]map[string]string {
	var merged map[string]map[string]string
	for _, entry := range workflows {
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

// nopWriter discards all writes. Used to suppress pipeline output when the
// init command provides its own styled output.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
