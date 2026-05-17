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
func newSddAdvisorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdd-advisors",
		Short: "Manage SDD advisors — list, install, uninstall, and register custom advisors",
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

  # Add an advisor from GitHub (selecting one specific advisor by name):
  devrune sdd-advisors --add-advisor source=github:acme/advisor-catalog,name=security-advisor

  # Remove an advisor by name (strips it from the matching AdvisorSource.Select):
  devrune sdd-advisors --remove-advisor security-advisor

  # Add an advisor source (no Select — installs everything in the source):
  devrune sdd-advisors --add-catalog github:acme/advisor-catalog

  # Remove an advisor source (drops the entire AdvisorSource entry):
  devrune sdd-advisors --remove-catalog github:acme/advisor-catalog

  # Re-fetch all registered advisor sources:
  devrune sdd-advisors --refresh-catalogs

  Note: --refresh-catalogs returns an error if any source fetch fails.
  All sources are attempted; per-source errors are aggregated and reported together.`,
		RunE:          runSddAdvisors,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Native toggle flags (install / uninstall by name).
	cmd.Flags().StringArray("install", nil, "Advisor name(s) to install (repeatable)")
	cmd.Flags().StringArray("uninstall", nil, "Advisor name(s) to uninstall (repeatable)")

	// Advisor source flags.
	cmd.Flags().StringArray("add-advisor", nil,
		`Add an advisor by source. Value format: source=SCHEME:PATH[,name=NAME]
  Schemes: local: (filesystem path), github: (owner/repo[@ref]), gitlab: (owner/repo[@ref])
  Without name=: install everything discovered in the source (Select empty).
  With name=:    install only that name (added to the matching source's Select).
  Example: --add-advisor source=local:./my-advisor,name=security-advisor`)
	cmd.Flags().StringArray("remove-advisor", nil, "Remove an advisor by name (repeatable). Strips it from any AdvisorSource.Select.")

	// Bulk source flags.
	cmd.Flags().StringArray("add-catalog", nil,
		`Add an advisor source with no Select filter (installs everything in the source). Repeatable.
  Example: --add-catalog github:acme/advisor-catalog`)
	cmd.Flags().StringArray("remove-catalog", nil, "Remove an advisor source by URL — drops the entire AdvisorSource entry (repeatable).")
	cmd.Flags().Bool("refresh-catalogs", false, "Re-fetch all registered advisor sources")

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
		entry, parseErr := parseAddAdvisorFlag(val, wd)
		if parseErr != nil {
			return fmt.Errorf("sdd-advisors: --add-advisor %q: %w", val, parseErr)
		}

		// Find or create the AdvisorSource entry for entry.source.
		existing := findAdvisorSource(&manifest, entry.source)
		if existing == nil {
			newSrc := model.AdvisorSource{Source: entry.source}
			if entry.name != "" {
				newSrc.Select = []string{entry.name}
			}
			if err := newSrc.Validate(); err != nil {
				return fmt.Errorf("sdd-advisors: --add-advisor %q: %w", val, err)
			}
			manifest.Advisors = append(manifest.Advisors, newSrc)
			changed = true
			continue
		}

		// Source exists. If the user asked for a specific name, append it
		// (preserving the existing Select shape: extending an existing list,
		// or staying empty if the source already says "install everything").
		if entry.name == "" {
			// User wants the whole source — collapse Select to empty.
			if len(existing.Select) > 0 {
				existing.Select = nil
				changed = true
			}
			continue
		}
		if len(existing.Select) == 0 {
			// Source already installs everything — adding a specific name is
			// redundant. No-op.
			continue
		}
		if !containsString(existing.Select, entry.name) {
			existing.Select = append(existing.Select, entry.name)
			changed = true
		}
	}

	// ── --remove-advisor ─────────────────────────────────────────────────────
	removeAdvisorVals, _ := cmd.Flags().GetStringArray("remove-advisor")
	for _, name := range removeAdvisorVals {
		// Walk all sources, strip name from Select. Drop sources whose
		// previously-non-empty Select becomes empty (mirrors applyManifestDiff).
		found := false
		nextAdvisors := manifest.Advisors[:0]
		for _, src := range manifest.Advisors {
			if len(src.Select) == 0 {
				// Empty Select = install everything — name is implicitly
				// included, but we cannot drop it from this source without
				// resolving + materializing a new explicit Select. Surface
				// this to the user instead of doing it silently.
				nextAdvisors = append(nextAdvisors, src)
				continue
			}
			filtered := src.Select[:0]
			for _, n := range src.Select {
				if n == name {
					found = true
					continue
				}
				filtered = append(filtered, n)
			}
			src.Select = filtered
			if len(src.Select) == 0 {
				// Whole source falls away — its Select was non-empty and now is.
				continue
			}
			nextAdvisors = append(nextAdvisors, src)
		}
		manifest.Advisors = nextAdvisors

		if !found {
			return fmt.Errorf("sdd-advisors: --remove-advisor: advisor %q not found in any AdvisorSource.Select", name)
		}
		changed = true
	}

	// ── --add-catalog ─────────────────────────────────────────────────────────
	addCatalogVals, _ := cmd.Flags().GetStringArray("add-catalog")
	for _, url := range addCatalogVals {
		newSrc := model.AdvisorSource{Source: url}
		if err := newSrc.Validate(); err != nil {
			return fmt.Errorf("sdd-advisors: --add-catalog %q: %w", url, err)
		}
		// Deduplicate by source URL.
		if findAdvisorSource(&manifest, url) != nil {
			continue
		}
		manifest.Advisors = append(manifest.Advisors, newSrc)
		changed = true
	}

	// ── --remove-catalog ─────────────────────────────────────────────────────
	removeCatalogVals, _ := cmd.Flags().GetStringArray("remove-catalog")
	for _, url := range removeCatalogVals {
		filtered := manifest.Advisors[:0]
		for _, src := range manifest.Advisors {
			if src.Source != url {
				filtered = append(filtered, src)
			}
		}
		if len(filtered) != len(manifest.Advisors) {
			changed = true
		}
		manifest.Advisors = filtered
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
		if err := writeManifestSafe(manifestPath, data); err != nil {
			return fmt.Errorf("sdd-advisors: %w", err)
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
	_ = out
	for {
		rows := buildAdvisorInventory(filepath.Join(wd, ".claude", "skills"), manifest)
		action, err := runTopLevelActionForm(rows, len(manifest.Advisors))
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
// It creates a backup of the current state before writing (no-op on first write).
func persistManifest(manifest model.UserManifest, manifestPath string) error {
	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		return fmt.Errorf("serialize manifest: %w", err)
	}
	return writeManifestSafe(manifestPath, data)
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

// addAdvisorFlagEntry is the internal struct returned by parseAddAdvisorFlag.
// It is a light, runtime-only triple used by the non-interactive flow to
// drive the manifest mutation. It does NOT persist anywhere — the caller
// translates it into the appropriate AdvisorSource changes.
type addAdvisorFlagEntry struct {
	source string // scheme-prefixed URL (local:/github:/gitlab:)
	name   string // optional advisor name; empty = install everything in the source
}

// parseAddAdvisorFlag parses a value in the format:
//
//	source=SCHEME:PATH[,name=NAME]
//
// (description and tier are no longer accepted — description is read from the
// SKILL.md frontmatter on disk; tier was removed earlier.)
//
// Returns the source URL plus the optional advisor name.
func parseAddAdvisorFlag(val, wd string) (addAdvisorFlagEntry, error) {
	parts := strings.Split(val, ",")
	kv := make(map[string]string, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key, value, found := strings.Cut(p, "=")
		if !found {
			return addAdvisorFlagEntry{}, fmt.Errorf("expected key=value, got %q", p)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		kv[key] = value
	}

	sourceVal, ok := kv["source"]
	if !ok || sourceVal == "" {
		return addAdvisorFlagEntry{}, fmt.Errorf("source= is required (e.g. source=local:./my-advisor)")
	}

	switch {
	case strings.HasPrefix(sourceVal, "local:"):
		// For local: we resolve and verify the path eagerly so the user gets
		// an immediate error rather than a deferred fetch failure later.
		localPath := strings.TrimPrefix(sourceVal, "local:")
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(wd, localPath)
		}
		if fi, err := os.Stat(localPath); err != nil {
			return addAdvisorFlagEntry{}, fmt.Errorf("local path %q: %w", localPath, err)
		} else if !fi.IsDir() {
			return addAdvisorFlagEntry{}, fmt.Errorf("local path %q is not a directory", localPath)
		}
		// Normalize the source URL to use the absolute path so subsequent
		// lookups against the manifest are deterministic.
		sourceVal = "local:" + localPath

	case strings.HasPrefix(sourceVal, "github:"), strings.HasPrefix(sourceVal, "gitlab:"):
		// nothing to do — the fetcher will validate on first use.

	default:
		return addAdvisorFlagEntry{}, fmt.Errorf("unrecognised scheme in %q (must be local:, github:, or gitlab:)", sourceVal)
	}

	// Validate the resulting source URL via CatalogSource (same scheme grammar).
	if err := (model.CatalogSource{URL: sourceVal}).Validate(); err != nil {
		return addAdvisorFlagEntry{}, err
	}

	return addAdvisorFlagEntry{
		source: sourceVal,
		name:   kv["name"],
	}, nil
}
