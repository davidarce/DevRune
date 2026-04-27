// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/advisorcatalog"
	"github.com/davidarce/devrune/internal/advisormeta"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

const addNewCatalogSentinel = "__add_new__"

// runAddAdvisorFlow implements the interactive "Add advisor" flow (Screens 3a/3b/3c).
// It mutates manifest in place but does NOT persist it — the caller is responsible
// for calling persistManifest and SyncAdvisors after this returns nil.
func runAddAdvisorFlow(ctx context.Context, wd string, manifest *model.UserManifest) error {
	for {
		// Screen 3a — pick existing catalog source or add a new one.
		selectedCatalogURL, err := runPickCatalogForm(manifest)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				// User cancelled — bubble up so the parent loop returns to the
				// top-level advisors menu.
				return huh.ErrUserAborted
			}
			return err
		}

		var catalogSrc model.CatalogSource
		if selectedCatalogURL == addNewCatalogSentinel {
			// Screen 3b — collect and validate a new catalog source.
			newSrc, back, addErr := runAddCatalogSourceForm(ctx, wd, manifest)
			if addErr != nil {
				return addErr
			}
			if back {
				// User hit "Back" — return to Screen 3a.
				continue
			}
			catalogSrc = newSrc
		} else {
			// Locate the existing CatalogSource by URL.
			for _, cs := range manifest.AdvisorCatalogs {
				if cs.URL == selectedCatalogURL {
					catalogSrc = cs
					break
				}
			}
		}

		// Fetch (or resolve for local:) the catalog root directory.
		rootDir, fetchErr := fetchCatalogSource(ctx, wd, catalogSrc)
		if fetchErr != nil {
			return fmt.Errorf("add-advisor: fetch catalog %q: %w", catalogSrc.URL, fetchErr)
		}

		// Single-advisor-mode detection: if the resolved dir itself contains SKILL.md
		// at root AND the basename ends in "-advisor", treat it as a single-advisor source.
		singleAdvisorMode := isSingleAdvisorDir(rootDir)

		// Scan for catalog entries.
		var entries []advisorcatalog.CatalogEntry
		if singleAdvisorMode {
			// Build a synthetic entry for the single advisor.
			entry, scanErr := singleAdvisorEntry(rootDir)
			if scanErr != nil {
				return fmt.Errorf("add-advisor: scan single advisor %q: %w", rootDir, scanErr)
			}
			entries = []advisorcatalog.CatalogEntry{entry}
		} else {
			var scanErr error
			entries, scanErr = advisorcatalog.DirScanner{}.Scan(rootDir)
			if scanErr != nil {
				return fmt.Errorf("add-advisor: scan catalog %q: %w", rootDir, scanErr)
			}
		}

		if len(entries) == 0 {
			return fmt.Errorf("add-advisor: no advisors found in %q", rootDir)
		}

		// Build the installed set (native + already-registered) for Screen 3c labels.
		installedNames := buildInstalledSet(manifest)

		// Screen 3c — multi-select which entries to import (auto-select if only one
		// AND it is not already installed). Entries are shown with "(already installed)"
		// labels; the form filters them out from the final selection so the user can
		// see the full catalog state without being blocked by collisions.
		var selected []advisorcatalog.CatalogEntry
		if (len(entries) == 1 || singleAdvisorMode) && !installedNames[entries[0].Name] {
			selected = entries
		} else {
			var back bool
			var selErr error
			selected, back, selErr = runPickAdvisorEntriesForm(entries, installedNames)
			if selErr != nil {
				return selErr
			}
			if back {
				continue
			}
			if len(selected) == 0 {
				// User submitted with no NEW selections (only pre-installed
				// advisors were toggled, which are filtered out). Show a
				// Note inside the TUI then exit the flow — printing to stdout
				// is invisible while the altscreen is active.
				if err := showInfoNote("Add advisor", "No new advisors selected."); err != nil {
					return err
				}
				return nil
			}
		}

		// Determine origin.
		origin := model.AdvisorOriginCatalog
		if singleAdvisorMode {
			scheme, _, _, _ := advisorcatalog.ParseCatalogURL(catalogSrc.URL)
			if scheme == "local" {
				origin = model.AdvisorOriginLocal
			}
		}

		// Copy each selected advisor into .claude/skills/ and register it.
		for _, entry := range selected {
			dst := filepath.Join(wd, ".claude", "skills", entry.Name)
			if _, copyErr := copyAdvisorDir(entry.DirPath, dst); copyErr != nil {
				return fmt.Errorf("add-advisor: copy %q: %w", entry.Name, copyErr)
			}

			// Determine skillSource: for catalog entries use the catalog URL, for local single-advisor use the dir path.
			skillSrc := catalogSrc.URL
			if origin == model.AdvisorOriginLocal {
				skillSrc = entry.DirPath
			}

			// Note: Scope is intentionally NOT copied into the manifest entry.
			// SKILL.md on disk is the single source of truth for scope; the TUI
			// and filter code load it on demand via advisormeta.
			def := model.AdvisorDef{
				Name:        entry.Name,
				Description: entry.Description,
				SkillSource: skillSrc,
				Origin:      origin,
			}
			manifest.CustomAdvisors = append(manifest.CustomAdvisors, def)
		}

		// Update SelectFilter.Skills for installed advisors.
		addedNames := make([]string, 0, len(selected))
		for _, e := range selected {
			addedNames = append(addedNames, e.Name)
		}
		if err := applyManifestDiff(manifest, addedNames, nil); err != nil {
			return fmt.Errorf("add-advisor: apply diff: %w", err)
		}

		return nil
	}
}

// runRefreshCatalogsFlow re-fetches every registered catalog source, re-copies
// any catalog-imported advisors whose SKILL.md changed (drift detection via SHA256),
// persists updated LastFetched timestamps, and calls SyncAdvisors.
//
// Returns a CatalogRefreshResult with structured messages for the TUI to
// render (status per catalog, warnings, updates). All output is captured —
// nothing is written to stdout/stderr while the TUI is active.
//
// An error is returned if ANY catalog fetch fails; per-catalog errors are
// aggregated. The result is still populated for the catalogs that succeeded.
func runRefreshCatalogsFlow(ctx context.Context, wd string, manifest *model.UserManifest) (CatalogRefreshResult, error) {
	var refreshResult CatalogRefreshResult

	if len(manifest.AdvisorCatalogs) == 0 {
		return refreshResult, fmt.Errorf("refresh-catalogs: no advisor catalogs registered (use 'Add advisor' to add one first)")
	}

	var fetchErrs []error

	for i := range manifest.AdvisorCatalogs {
		cat := &manifest.AdvisorCatalogs[i]

		// Fetch (re-clone or fast-forward).
		rootDir, fetchErr := fetchCatalogSource(ctx, wd, *cat)
		if fetchErr != nil {
			refreshResult.Errors = append(refreshResult.Errors,
				fmt.Sprintf("%s: fetch error: %v", cat.URL, fetchErr))
			fetchErrs = append(fetchErrs, fmt.Errorf("catalog %q: %w", cat.URL, fetchErr))
			continue
		}

		// Update LastFetched.
		cat.LastFetched = time.Now().UTC().Format(time.RFC3339)

		// Re-scan the catalog.
		catalogEntries, scanErr := advisorcatalog.DirScanner{}.Scan(rootDir)
		if scanErr != nil {
			refreshResult.Warnings = append(refreshResult.Warnings,
				fmt.Sprintf("%s: scan error: %v (skipped)", cat.URL, scanErr))
			continue
		}

		// Build a lookup of scanned entries by name for fast access.
		entryByName := make(map[string]advisorcatalog.CatalogEntry, len(catalogEntries))
		for _, e := range catalogEntries {
			entryByName[e.Name] = e
		}

		// Re-copy advisors imported from this catalog whose SKILL.md has changed.
		updatedCount := 0
		for _, def := range manifest.CustomAdvisors {
			if def.Origin != model.AdvisorOriginCatalog {
				continue
			}
			if !advisorBelongsToCatalog(def, *cat, manifest.AdvisorCatalogs, wd) {
				continue
			}

			newEntry, ok := entryByName[def.Name]
			if !ok {
				continue
			}

			dst := filepath.Join(wd, ".claude", "skills", def.Name)
			dstSkillMD := filepath.Join(dst, "SKILL.md")

			hashBefore := fileSHA256(dstSkillMD)
			hashAfter := fileSHA256(newEntry.SKILLPath)

			if hashBefore == hashAfter {
				continue
			}

			if _, copyErr := copyAdvisorDir(newEntry.DirPath, dst); copyErr != nil {
				refreshResult.Warnings = append(refreshResult.Warnings,
					fmt.Sprintf("%s: could not re-copy %q: %v", cat.URL, def.Name, copyErr))
				continue
			}
			refreshResult.Updated = append(refreshResult.Updated, def.Name)
			updatedCount++
		}

		if updatedCount == 0 {
			refreshResult.NoChanges = append(refreshResult.NoChanges, cat.URL)
		}
	}

	if len(fetchErrs) > 0 {
		return refreshResult, fmt.Errorf("refresh-catalogs: %d catalog(s) failed to fetch: %w",
			len(fetchErrs), errors.Join(fetchErrs...))
	}
	return refreshResult, nil
}

// CatalogRefreshResult collects structured per-catalog output from a refresh
// operation so it can be rendered inside the TUI instead of streamed to
// stdout (which corrupts the altscreen).
type CatalogRefreshResult struct {
	// Updated holds names of advisors that were re-copied due to detected drift.
	Updated []string
	// NoChanges holds catalog URLs that produced no advisor updates.
	NoChanges []string
	// Warnings holds non-fatal notices (scan failures, copy failures).
	Warnings []string
	// Errors holds fatal per-catalog errors (e.g. fetch failures).
	Errors []string
}

// ── Screen helpers ────────────────────────────────────────────────────────────

// runPickCatalogForm shows Screen 3a: select an existing catalog source or add a new one.
// Returns the selected catalog URL or addNewCatalogSentinel.
//
// Single-choice Select: Enter drills directly into the highlighted catalog
// (no separate Confirm step — the selection IS the action). Esc cancels.
// Returns huh.ErrUserAborted on cancel.
func runPickCatalogForm(manifest *model.UserManifest) (string, error) {
	options := make([]huh.Option[string], 0, len(manifest.AdvisorCatalogs)+1)
	for _, cat := range manifest.AdvisorCatalogs {
		label := cat.URL
		if cat.Name != "" {
			label = cat.Name + " (" + cat.URL + ")"
		}
		options = append(options, huh.NewOption(label, cat.URL))
	}
	options = append(options, huh.NewOption("+ Add a new catalog source", addNewCatalogSentinel))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewSelect[string]().
				Title("Add advisor — select catalog source").
				Description("Enter opens the highlighted catalog. Esc cancels.").
				Options(options...).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", huh.ErrUserAborted
		}
		return "", err
	}
	return selected, nil
}

// runAddCatalogSourceForm shows Screen 3b: collect scheme + path for a new catalog source.
// Returns (newSource, back=true, nil) when the user hits Back, or (newSource, false, nil) on Confirm.
func runAddCatalogSourceForm(ctx context.Context, wd string, manifest *model.UserManifest) (model.CatalogSource, bool, error) {
	var scheme string
	var path string
	var confirmed bool

	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewSelect[string]().
				Title("Catalog source type").
				Description("Choose the scheme for the new catalog source.").
				Options(
					huh.NewOption("local: (filesystem path)", "local"),
					huh.NewOption("github: (GitHub repository)", "github"),
					huh.NewOption("gitlab: (GitLab repository)", "gitlab"),
				).
				Value(&scheme),
			huh.NewInput().
				Title("Catalog path or repository").
				Description("For local: enter a filesystem path. For github:/gitlab: enter owner/repo[@ref].").
				Value(&path),
			huh.NewConfirm().
				Affirmative("Confirm").
				Negative("Back").
				Value(&confirmed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return model.CatalogSource{}, false, err
	}

	if !confirmed {
		return model.CatalogSource{}, true, nil
	}

	url := scheme + ":" + strings.TrimSpace(path)
	src := model.CatalogSource{URL: url}
	if err := src.Validate(); err != nil {
		return model.CatalogSource{}, false, fmt.Errorf("invalid catalog source: %w", err)
	}

	// Test fetch immediately so we fail early.
	if _, err := fetchCatalogSource(ctx, wd, src); err != nil {
		return model.CatalogSource{}, false, fmt.Errorf("catalog source %q: %w", url, err)
	}

	// Deduplicate before appending.
	for _, existing := range manifest.AdvisorCatalogs {
		if existing.URL == url {
			return src, false, nil
		}
	}
	manifest.AdvisorCatalogs = append(manifest.AdvisorCatalogs, src)
	return src, false, nil
}

// runPickAdvisorEntriesForm shows Screen 3c: multi-select which catalog entries to import.
// Entries whose names are in installedNames are pre-selected and shown with
// "(already installed)" appended to their label; they are excluded from the
// returned selection regardless of toggle state.
//
// Navigation: space toggles items, Enter submits the current selection,
// Esc cancels.
//
// Returns (selected entries, back=true, nil) when the user cancels (Esc).
// Returns (selected entries, false, nil) on Enter; selected may be empty if
// the user only kept already-installed entries selected.
func runPickAdvisorEntriesForm(entries []advisorcatalog.CatalogEntry, installedNames map[string]bool) ([]advisorcatalog.CatalogEntry, bool, error) {
	options := make([]huh.Option[string], len(entries))
	for i, e := range entries {
		label := e.Name
		if e.Description != "" {
			label += " — " + e.Description
		}
		if installedNames[e.Name] {
			label += " (already installed)"
		}
		opt := huh.NewOption(label, e.Name)
		if installedNames[e.Name] {
			opt = opt.Selected(true)
		}
		options[i] = opt
	}

	var selectedNames []string

	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewMultiSelect[string]().
				Title("Select advisors to install").
				Description("Space toggles items. Enter submits the current selection. Esc cancels.").
				Options(options...).
				Height(steps.DynamicHeight(len(entries)+2)).
				Value(&selectedNames),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, true, nil
		}
		return nil, false, err
	}

	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}

	var selected []advisorcatalog.CatalogEntry
	for _, e := range entries {
		if nameSet[e.Name] && !installedNames[e.Name] {
			selected = append(selected, e)
		}
	}
	return selected, false, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// fetchCatalogSource dispatches to the appropriate fetcher based on the URL scheme.
func fetchCatalogSource(ctx context.Context, wd string, src model.CatalogSource) (string, error) {
	scheme, _, _, err := advisorcatalog.ParseCatalogURL(src.URL)
	if err != nil {
		return "", err
	}
	switch scheme {
	case "local":
		return advisorcatalog.LocalFetcher{WorkspaceRoot: wd}.Fetch(ctx, src)
	case "github", "gitlab":
		return advisorcatalog.GitFetcher{WorkspaceRoot: wd}.Fetch(ctx, src)
	default:
		return "", fmt.Errorf("unsupported catalog scheme %q", scheme)
	}
}

// isSingleAdvisorDir reports whether dir itself is a single-advisor directory:
// it must contain a SKILL.md at its root and its basename must end in "-advisor".
func isSingleAdvisorDir(dir string) bool {
	if !strings.HasSuffix(filepath.Base(dir), "-advisor") {
		return false
	}
	skillMD := filepath.Join(dir, "SKILL.md")
	_, err := os.Stat(skillMD)
	return err == nil
}

// singleAdvisorEntry builds a synthetic CatalogEntry for a single-advisor directory.
func singleAdvisorEntry(dir string) (advisorcatalog.CatalogEntry, error) {
	skillMD := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return advisorcatalog.CatalogEntry{}, fmt.Errorf("read SKILL.md: %w", err)
	}

	fm, _, parseErr := parse.ParseFrontmatter(data)
	if parseErr != nil {
		return advisorcatalog.CatalogEntry{}, fmt.Errorf("parse SKILL.md frontmatter: %w", parseErr)
	}

	description := ""
	if v, ok := fm["description"]; ok {
		if str, ok := v.(string); ok {
			description = str
		}
	}
	// Extract scope list from frontmatter via the shared helper.
	// Hard YAML-shape errors (scalar instead of list, null value, non-string
	// element) are surfaced as errors. Unknown vocabulary values are dropped
	// silently by model.NormalizeAdvisorScope (soft-fallback policy).
	rawScope, err := advisormeta.FrontmatterStringList(fm, "scope")
	if err != nil {
		return advisorcatalog.CatalogEntry{}, fmt.Errorf("advisor %q: %w", filepath.Base(dir), err)
	}

	return advisorcatalog.CatalogEntry{
		Name:        filepath.Base(dir),
		Description: description,
		Scope:       model.NormalizeAdvisorScope(rawScope),
		SKILLPath:   skillMD,
		DirPath:     dir,
	}, nil
}

// buildInstalledSet returns a set of advisor names that are already installed:
// native advisors from model.ReservedAdvisorNames plus custom advisors in the manifest.
func buildInstalledSet(manifest *model.UserManifest) map[string]bool {
	installed := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		installed[n] = true
	}
	for _, def := range manifest.CustomAdvisors {
		installed[def.Name] = true
	}
	return installed
}

// filterAvailableEntries removes entries that collide with native advisors or
// already-registered custom/catalog advisors in the manifest.
func filterAvailableEntries(entries []advisorcatalog.CatalogEntry, manifest *model.UserManifest) []advisorcatalog.CatalogEntry {
	nativeSet := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		nativeSet[n] = true
	}
	registeredSet := make(map[string]bool)
	for _, def := range manifest.CustomAdvisors {
		registeredSet[def.Name] = true
	}

	var filtered []advisorcatalog.CatalogEntry
	for _, e := range entries {
		if nativeSet[e.Name] || registeredSet[e.Name] {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// advisorBelongsToCatalog returns true if def is attributable to cat.
//
// Matching strategy (in priority order):
//  1. Exact URL match: def.SkillSource == cat.URL (set by the Add-advisor flow for
//     multi-advisor catalog sources).
//  2. Path prefix match: def.SkillSource is a filesystem path that lies inside the
//     catalog's resolved cache directory (.devrune/advisor-catalogs/<CacheKey>/…).
//     This covers catalog-imported advisors whose SkillSource was set to the copied
//     cache path rather than the catalog URL.
//
// The old single-catalog heuristic ("if only 1 catalog is registered, attribute
// everything to it") is intentionally removed: it would mis-attribute local-origin
// advisors to a github: catalog when exactly one catalog is registered.
func advisorBelongsToCatalog(def model.AdvisorDef, cat model.CatalogSource, _ []model.CatalogSource, wd string) bool {
	// Strategy 1: exact URL match.
	if def.SkillSource == cat.URL {
		return true
	}

	// Strategy 2: path-prefix match against the catalog's cache directory.
	// Local catalogs (CacheKey == "") have no cache dir — they are used in place,
	// so a path-prefix match is not meaningful; skip.
	cacheKey := advisorcatalog.CacheKey(cat)
	if cacheKey == "" {
		return false
	}

	cacheDir := filepath.Clean(filepath.Join(wd, ".devrune", "advisor-catalogs", cacheKey))
	skillSrc := filepath.Clean(def.SkillSource)

	// HasPrefix via Rel: if skillSrc is under cacheDir, Rel won't start with "..".
	rel, err := filepath.Rel(cacheDir, skillSrc)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// fileSHA256 returns the hex-encoded SHA256 of the named file, or an empty
// string if the file cannot be read (treated as "missing / unknown").
func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
