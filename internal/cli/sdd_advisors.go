// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// newSddAdvisorsCmd constructs the "sdd-advisors" Cobra subcommand.
// An alias "sdd-advisers" is registered for one-release backward compat.
func newSddAdvisorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sdd-advisors",
		Aliases: []string{"sdd-advisers"},
		Short:   "Manage SDD advisors — list, install, uninstall, and register custom advisors",
		Long: `Manage SDD advisors interactively or via flags.

devrune sdd-advisors provides a TUI to list, toggle (install/uninstall),
add, and remove SDD advisors without running a full 'devrune sync'. After
any change the relevant agent files (.claude/agents/*.md, CLAUDE.md,
AGENTS.md) and SDD skill files are regenerated automatically.

Note: SDD model configuration (per-phase model overrides) is separate.
Use 'Configure role models' from the main menu for that.

Examples:

  # Interactive TUI (recommended):
  devrune sdd-advisors

  # Toggle specific advisors (non-interactive):
  devrune sdd-advisors --install architect-advisor --uninstall unit-test-advisor

  # Add a custom advisor from a local directory:
  devrune sdd-advisors --add-advisor source=local:./my-advisor

  # Add a custom advisor from GitHub with explicit name:
  devrune sdd-advisors --add-advisor source=github:acme/advisor-catalog,name=security-advisor

  # Remove a custom or catalog-imported advisor:
  devrune sdd-advisors --remove-advisor security-advisor

  # Add an advisor catalog source:
  devrune sdd-advisors --add-catalog github:acme/advisor-catalog

  # Remove an advisor catalog source:
  devrune sdd-advisors --remove-catalog github:acme/advisor-catalog

  # Re-fetch all registered advisor catalog sources:
  devrune sdd-advisors --refresh-catalogs

  Note: --refresh-catalogs returns an error if any catalog fetch fails.
  All catalogs are attempted; per-catalog errors are aggregated and reported together.

  Note: the alias 'sdd-advisers' (British spelling) is accepted for one
  release as a backward-compat shim and will be removed in a future version.`,
		RunE:          runSddAdvisors,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Native toggle flags (install / uninstall by name).
	cmd.Flags().StringArray("install", nil, "Advisor name(s) to install (repeatable)")
	cmd.Flags().StringArray("uninstall", nil, "Advisor name(s) to uninstall (repeatable)")

	// Custom / catalog advisor flags.
	cmd.Flags().StringArray("add-advisor", nil,
		`Add a custom or catalog advisor. Value format: source=SCHEME:PATH[,name=NAME,description=DESC,tier=TIER]
  Schemes: local: (filesystem path), github: (owner/repo[@ref]), gitlab: (owner/repo[@ref])
  Example: --add-advisor source=local:./my-advisor,name=security-advisor`)
	cmd.Flags().StringArray("remove-advisor", nil, "Remove a custom or catalog-imported advisor by name (repeatable)")

	// Catalog source flags.
	cmd.Flags().StringArray("add-catalog", nil,
		`Add an advisor catalog source (SCHEME:PATH, repeatable).
  Example: --add-catalog github:acme/advisor-catalog`)
	cmd.Flags().StringArray("remove-catalog", nil, "Remove an advisor catalog source by URL (repeatable)")
	cmd.Flags().Bool("refresh-catalogs", false, "Re-fetch all registered advisor catalog sources")

	return cmd
}

// runSddAdvisors is the RunE handler for the sdd-advisors command.
// It loads the manifest, determines whether to run interactively or via flags,
// and dispatches accordingly.
func runSddAdvisors(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	nonInteractive := isNonInteractive(cmd)
	out := cmd.OutOrStdout()

	manifestPath := filepath.Join(wd, "devrune.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("devrune.yaml not found — run 'devrune init' first")
		}
		return fmt.Errorf("sdd-advisors: read manifest: %w", err)
	}

	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("sdd-advisors: parse manifest: %w", err)
	}

	// Detect whether any advisor-related flags were explicitly provided.
	anyFlagChanged := cmd.Flags().Changed("install") ||
		cmd.Flags().Changed("uninstall") ||
		cmd.Flags().Changed("add-advisor") ||
		cmd.Flags().Changed("remove-advisor") ||
		cmd.Flags().Changed("add-catalog") ||
		cmd.Flags().Changed("remove-catalog") ||
		cmd.Flags().Changed("refresh-catalogs")

	if nonInteractive || anyFlagChanged {
		return runSddAdvisorsNonInteractive(ctx, cmd, wd, manifest, manifestPath, out)
	}

	return runSddAdvisorsInteractive(ctx, cmd, wd, manifest, manifestPath, out)
}

// runSddAdvisorsNonInteractive processes flag-driven mutations and calls SyncAdvisors.
func runSddAdvisorsNonInteractive(
	ctx context.Context,
	cmd *cobra.Command,
	wd string,
	manifest model.UserManifest,
	manifestPath string,
	out interface{ Write([]byte) (int, error) },
) error {
	changed := false

	// ── --install / --uninstall ──────────────────────────────────────────────
	installNames, _ := cmd.Flags().GetStringArray("install")
	uninstallNames, _ := cmd.Flags().GetStringArray("uninstall")
	if len(installNames) > 0 || len(uninstallNames) > 0 {
		if err := applyManifestDiff(&manifest, installNames, uninstallNames); err != nil {
			return fmt.Errorf("sdd-advisors: %w", err)
		}
		changed = true
	}

	// ── --add-advisor ─────────────────────────────────────────────────────────
	addAdvisorVals, _ := cmd.Flags().GetStringArray("add-advisor")
	for _, val := range addAdvisorVals {
		def, parseErr := parseAddAdvisorFlag(val, wd)
		if parseErr != nil {
			return fmt.Errorf("sdd-advisors: --add-advisor %q: %w", val, parseErr)
		}
		if err := def.Validate(); err != nil {
			return fmt.Errorf("sdd-advisors: --add-advisor %q: %w", val, err)
		}
		// Deduplicate by name.
		already := false
		for _, existing := range manifest.CustomAdvisors {
			if existing.Name == def.Name {
				already = true
				break
			}
		}
		if !already {
			manifest.CustomAdvisors = append(manifest.CustomAdvisors, def)
			changed = true
		}
	}

	// ── --remove-advisor ─────────────────────────────────────────────────────
	removeAdvisorVals, _ := cmd.Flags().GetStringArray("remove-advisor")
	for _, name := range removeAdvisorVals {
		found := false
		for _, def := range manifest.CustomAdvisors {
			if def.Name == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("sdd-advisors: --remove-advisor: advisor %q not found in customAdvisors", name)
		}
		filtered := manifest.CustomAdvisors[:0]
		for _, def := range manifest.CustomAdvisors {
			if def.Name != name {
				filtered = append(filtered, def)
			}
		}
		manifest.CustomAdvisors = filtered
		changed = true
	}

	// ── --add-catalog ─────────────────────────────────────────────────────────
	addCatalogVals, _ := cmd.Flags().GetStringArray("add-catalog")
	for _, url := range addCatalogVals {
		cat := model.CatalogSource{URL: url}
		if err := cat.Validate(); err != nil {
			return fmt.Errorf("sdd-advisors: --add-catalog %q: %w", url, err)
		}
		// Deduplicate by URL.
		already := false
		for _, existing := range manifest.AdvisorCatalogs {
			if existing.URL == url {
				already = true
				break
			}
		}
		if !already {
			manifest.AdvisorCatalogs = append(manifest.AdvisorCatalogs, cat)
			changed = true
		}
	}

	// ── --remove-catalog ─────────────────────────────────────────────────────
	removeCatalogVals, _ := cmd.Flags().GetStringArray("remove-catalog")
	for _, url := range removeCatalogVals {
		filtered := manifest.AdvisorCatalogs[:0]
		for _, cat := range manifest.AdvisorCatalogs {
			if cat.URL != url {
				filtered = append(filtered, cat)
			}
		}
		manifest.AdvisorCatalogs = filtered
		changed = true
	}

	// ── --refresh-catalogs ────────────────────────────────────────────────────
	refreshCatalogs, _ := cmd.Flags().GetBool("refresh-catalogs")
	if refreshCatalogs {
		refreshResult, err := runRefreshCatalogsFlow(ctx, wd, &manifest)
		if err != nil {
			return fmt.Errorf("sdd-advisors: refresh-catalogs: %w", err)
		}
		_, _ = fmt.Fprint(out, formatCatalogRefreshSummary(refreshResult))
		changed = true
	}

	// Persist manifest if mutated.
	if changed {
		data, err := parse.SerializeManifest(manifest)
		if err != nil {
			return fmt.Errorf("sdd-advisors: serialize manifest: %w", err)
		}
		if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
			return fmt.Errorf("sdd-advisors: write manifest: %w", err)
		}
	}

	// Run SyncAdvisors unconditionally (re-renders even with no manifest change).
	result, err := SyncAdvisors(ctx, wd, manifest)
	if err != nil {
		return fmt.Errorf("sdd-advisors: sync: %w", err)
	}

	printAdvisorsSyncSummary(out, result)
	return nil
}

// runSddAdvisorsInteractive runs the interactive TUI loop.
func runSddAdvisorsInteractive(
	ctx context.Context,
	_ *cobra.Command,
	wd string,
	manifest model.UserManifest,
	manifestPath string,
	out interface{ Write([]byte) (int, error) },
) error {
	for {
		rows := buildAdvisorInventory(filepath.Join(wd, ".claude", "skills"), manifest)
		action, err := runTopLevelActionForm(rows, len(manifest.AdvisorCatalogs))
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}

		switch action {
		case advisorsActionQuit:
			return nil

		case advisorsActionToggle:
			toAdd, toRemove, formErr := runToggleForm(rows)
			if errors.Is(formErr, huh.ErrUserAborted) {
				continue
			}
			if formErr != nil {
				return formErr
			}

			if len(toAdd) == 0 && len(toRemove) == 0 {
				// No change — loop back to menu.
				continue
			}

			if diffErr := applyManifestDiff(&manifest, toAdd, toRemove); diffErr != nil {
				return diffErr
			}

			if persistErr := persistManifest(manifest, manifestPath); persistErr != nil {
				return persistErr
			}

			result, syncErr := SyncAdvisors(ctx, wd, manifest)
			if syncErr != nil {
				return syncErr
			}
			if err := showAdvisorsSyncSummary(result); err != nil {
				return err
			}

		case advisorsActionAddAdvisor:
			if addErr := runAddAdvisorFlow(ctx, wd, &manifest); addErr != nil {
				if errors.Is(addErr, huh.ErrUserAborted) {
					continue
				}
				if err := showInfoNote("Add advisor failed", addErr.Error()); err != nil {
					return err
				}
				continue
			}

			if persistErr := persistManifest(manifest, manifestPath); persistErr != nil {
				return persistErr
			}

			result, syncErr := SyncAdvisors(ctx, wd, manifest)
			if syncErr != nil {
				return syncErr
			}
			if err := showAdvisorsSyncSummary(result); err != nil {
				return err
			}

		case advisorsActionRefreshCatalogs:
			refreshResult, refreshErr := runRefreshCatalogsFlow(ctx, wd, &manifest)
			if refreshErr != nil {
				if errors.Is(refreshErr, huh.ErrUserAborted) {
					continue
				}
				if err := showInfoNote("Refresh catalogs failed", refreshErr.Error()); err != nil {
					return err
				}
				continue
			}

			if persistErr := persistManifest(manifest, manifestPath); persistErr != nil {
				return persistErr
			}

			syncResult, syncErr := SyncAdvisors(ctx, wd, manifest)
			if syncErr != nil {
				return syncErr
			}
			if err := showCatalogRefreshSummary(refreshResult, syncResult); err != nil {
				return err
			}
		}
	}
}

// runSddAdvisorsFromMenu is the entry point called from RunMenu.
// It mirrors the pattern used by runSyncFromMenu / runConfigureModelsFromMenu.
func runSddAdvisorsFromMenu(cmd *cobra.Command) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	out := nopWriter{} // suppress progress noise inside the TUI session

	manifestPath := filepath.Join(wd, "devrune.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("devrune.yaml not found — run New setup first")
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	return runSddAdvisorsInteractive(ctx, cmd, wd, manifest, manifestPath, out)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// persistManifest serializes and writes the manifest to disk.
func persistManifest(manifest model.UserManifest, manifestPath string) error {
	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		return fmt.Errorf("serialize manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// showInfoNote renders an informational message inside the TUI as a Note,
// waits for the user to dismiss it, then returns. Use for short user-facing
// notices ("nothing was changed", "operation cancelled") that would otherwise
// be invisible if printed to stdout while the altscreen is active.
func showInfoNote(title, body string) error {
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title(title).
				Description(body),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	return nil
}

// showAdvisorsSyncSummary renders the sync result inside the TUI as a Note
// form, then waits for the user to dismiss it. All output stays on the
// altscreen — nothing is written to stdout/stderr.
func showAdvisorsSyncSummary(result AdvisorsSyncResult) error {
	body := formatAdvisorsSyncSummary(result)

	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title("Advisors sync — done").
				Description(body),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	return nil
}

// showCatalogRefreshSummary renders the refresh-catalogs result inside the
// TUI as a Note form. Like showAdvisorsSyncSummary, all output stays on the
// altscreen.
func showCatalogRefreshSummary(refresh CatalogRefreshResult, sync AdvisorsSyncResult) error {
	body := formatCatalogRefreshSummary(refresh) + "\n\n" + formatAdvisorsSyncSummary(sync)

	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title("Refresh catalogs — done").
				Description(body),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	return nil
}

// formatAdvisorsSyncSummary builds the multi-line summary body shown in the
// sync-summary Note. Used by both the TUI Note and the CLI text path.
func formatAdvisorsSyncSummary(result AdvisorsSyncResult) string {
	var b strings.Builder
	total := len(result.Written) + len(result.Deleted) + len(result.WrittenCatalogDocs) + len(result.WrittenSkillDocs)
	if total == 0 {
		b.WriteString("No files changed.\n")
	} else {
		if len(result.Written) > 0 {
			fmt.Fprintf(&b, "• Written:  %d agent file(s)\n", len(result.Written))
		}
		if len(result.Deleted) > 0 {
			fmt.Fprintf(&b, "• Removed:  %d agent file(s)\n", len(result.Deleted))
		}
		if len(result.WrittenCatalogDocs) > 0 {
			fmt.Fprintf(&b, "• Updated:  %d catalog doc(s)\n", len(result.WrittenCatalogDocs))
		}
		if len(result.WrittenSkillDocs) > 0 {
			fmt.Fprintf(&b, "• Updated:  %d SDD skill file(s)\n", len(result.WrittenSkillDocs))
		}
	}
	if len(result.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, w := range result.Warnings {
			fmt.Fprintf(&b, "  ! %s\n", w)
		}
	}
	return b.String()
}

// formatCatalogRefreshSummary builds the multi-line summary body for the
// refresh-catalogs Note.
func formatCatalogRefreshSummary(r CatalogRefreshResult) string {
	var b strings.Builder
	if len(r.Updated) == 0 && len(r.NoChanges) == 0 && len(r.Warnings) == 0 && len(r.Errors) == 0 {
		b.WriteString("No catalogs processed.")
		return b.String()
	}
	for _, name := range r.Updated {
		fmt.Fprintf(&b, "• [updated] %s\n", name)
	}
	for _, url := range r.NoChanges {
		fmt.Fprintf(&b, "• [ok] %s — no changes\n", url)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "  ! %s\n", w)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "  ✗ %s\n", e)
	}
	return b.String()
}

// printAdvisorsSyncSummary writes a brief sync result to out.
// Used by the non-interactive CLI flag path; the TUI uses showAdvisorsSyncSummary.
func printAdvisorsSyncSummary(out interface{ Write([]byte) (int, error) }, result AdvisorsSyncResult) {
	total := len(result.Written) + len(result.Deleted) + len(result.WrittenCatalogDocs) + len(result.WrittenSkillDocs)
	if total == 0 {
		_, _ = fmt.Fprintln(out, "  No files changed.")
		return
	}
	if len(result.Written) > 0 {
		_, _ = fmt.Fprintf(out, "  Written:  %d agent file(s)\n", len(result.Written))
	}
	if len(result.Deleted) > 0 {
		_, _ = fmt.Fprintf(out, "  Removed:  %d agent file(s)\n", len(result.Deleted))
	}
	if len(result.WrittenCatalogDocs) > 0 {
		_, _ = fmt.Fprintf(out, "  Updated:  %d catalog doc(s)\n", len(result.WrittenCatalogDocs))
	}
	if len(result.WrittenSkillDocs) > 0 {
		_, _ = fmt.Fprintf(out, "  Updated:  %d SDD skill file(s)\n", len(result.WrittenSkillDocs))
	}
}

// parseAddAdvisorFlag parses a value in the format:
//
//	source=SCHEME:PATH[,name=NAME,description=DESC,tier=TIER]
//
// It returns a fully-populated AdvisorDef. The name, if not provided, is
// derived from the last path component of SCHEME:PATH.
func parseAddAdvisorFlag(val, wd string) (model.AdvisorDef, error) {
	parts := strings.Split(val, ",")
	kv := make(map[string]string, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key, value, found := strings.Cut(p, "=")
		if !found {
			return model.AdvisorDef{}, fmt.Errorf("expected key=value, got %q", p)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		kv[key] = value
	}

	sourceVal, ok := kv["source"]
	if !ok || sourceVal == "" {
		return model.AdvisorDef{}, fmt.Errorf("source= is required (e.g. source=local:./my-advisor)")
	}

	// Determine origin and resolve local paths.
	var origin model.AdvisorOrigin
	var skillSource string

	switch {
	case strings.HasPrefix(sourceVal, "local:"):
		origin = model.AdvisorOriginLocal
		localPath := strings.TrimPrefix(sourceVal, "local:")
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(wd, localPath)
		}
		// Validate it exists and is a directory.
		if fi, err := os.Stat(localPath); err != nil {
			return model.AdvisorDef{}, fmt.Errorf("local path %q: %w", localPath, err)
		} else if !fi.IsDir() {
			return model.AdvisorDef{}, fmt.Errorf("local path %q is not a directory", localPath)
		}
		skillSource = localPath

	case strings.HasPrefix(sourceVal, "github:"), strings.HasPrefix(sourceVal, "gitlab:"):
		origin = model.AdvisorOriginCatalog
		skillSource = sourceVal

	default:
		return model.AdvisorDef{}, fmt.Errorf("unrecognised scheme in %q (must be local:, github:, or gitlab:)", sourceVal)
	}

	// Derive name from last path component if not provided.
	name := kv["name"]
	if name == "" {
		// Strip scheme prefix and any ref (@ref), then take the last segment.
		rawPath := sourceVal
		for _, pfx := range []string{"local:", "github:", "gitlab:"} {
			rawPath = strings.TrimPrefix(rawPath, pfx)
		}
		// Strip @ref.
		if atIdx := strings.Index(rawPath, "@"); atIdx >= 0 {
			rawPath = rawPath[:atIdx]
		}
		rawPath = strings.TrimSuffix(rawPath, "/")
		if idx := strings.LastIndex(rawPath, "/"); idx >= 0 {
			name = rawPath[idx+1:]
		} else {
			name = rawPath
		}
	}

	return model.AdvisorDef{
		Name:        name,
		Description: kv["description"],
		SkillSource: skillSource,
		Origin:      origin,
	}, nil
}
