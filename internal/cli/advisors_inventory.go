// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/davidarce/devrune/internal/advisormeta"
	"github.com/davidarce/devrune/internal/model"
)

// advisorRow is a display-ready record for a single advisor in the inventory
// table. It is produced by buildAdvisorInventory and consumed by the TUI.
type advisorRow struct {
	Name        string
	Description string
	Scope       []string
	Origin      string // "", "local", "catalog"
	CatalogURL  string // set only for catalog-imported advisors (the AdvisorSource.Source)
	Installed   bool
}

// buildAdvisorInventory returns a fully annotated list of every known advisor —
// native and externally-sourced — merged and deduplicated.
//
// skillsRoot is the path to the on-disk skills directory (e.g.
// filepath.Join(wd, ".claude", "skills")). It is used to load native advisor
// scope declarations from each *-advisor/SKILL.md frontmatter. If skillsRoot
// is empty or the directory is inaccessible, native advisors are returned with
// nil scope (universal) and no error is propagated — the inventory degrades
// gracefully rather than crashing the TUI.
//
// External advisors are discovered by walking m.Advisors[] and scanning each
// resolved source via advisorcatalog.Scanner. The Select filter on each entry
// is applied (empty Select = include all advisors discovered in the source).
//
// Resolution failures (fetch / scan / parse errors) degrade gracefully: the
// failing source is skipped silently, leaving its advisors absent from the
// inventory. The TUI must remain responsive even when a remote catalog is
// temporarily unreachable.
//
// Ordering guarantee (deterministic):
//  1. Native advisors, sorted by name.
//  2. External advisors not already listed as native, sorted by CatalogURL
//     (empty/local first) then Name within each group.
func buildAdvisorInventory(skillsRoot string, m model.UserManifest) []advisorRow {
	// ── load native advisor scopes from disk ──────────────────────────────────
	// Errors are non-fatal: native advisors without scope are treated as
	// universal (nil Scope). The TUI degrades gracefully if the skills
	// directory is missing or a SKILL.md is malformed.
	nativeScopes, _ := advisormeta.LoadNativeAdvisorScopes(skillsRoot)

	// ── build a set of all installed skill names across all packages ──────────
	installedSkills := make(map[string]bool)
	for _, pkg := range m.Packages {
		if pkg.Select == nil {
			continue
		}
		for _, s := range pkg.Select.Skills {
			installedSkills[s] = true
		}
	}

	// ── resolve external advisors via the new Advisors[] schema ───────────────
	// On any per-source failure we silently drop that source's contributions —
	// the inventory is a best-effort view; it must not crash the TUI when a
	// remote catalog is unreachable. Fetcher operations are network calls, so
	// we use a fresh context here (no upstream cancellation token at this layer).
	resolved, _ := resolveAdvisors(context.Background(), "", m)

	// ── deduplicate against native names: external entry wins for same name ──
	customByName := make(map[string]ResolvedAdvisor, len(resolved))
	for _, r := range resolved {
		customByName[r.Def.Name] = r
	}

	var rows []advisorRow

	// ── 1. Native advisors ────────────────────────────────────────────────────
	nativeNames := model.ReservedAdvisorNames() // already sorted
	for _, name := range nativeNames {
		// If a custom entry with the same name exists, skip — custom wins.
		if _, isCustom := customByName[name]; isCustom {
			continue
		}
		// Defensive copy of the scope slice from the loaded native scopes.
		// nativeScopes may be nil (e.g. skillsRoot not found); in that case
		// the scope defaults to nil (universal) via the zero value.
		scope := append([]string(nil), nativeScopes[name]...)
		rows = append(rows, advisorRow{
			Name:      name,
			Scope:     scope,
			Origin:    "",
			Installed: installedSkills[name],
		})
	}

	// ── 2. External advisors (sorted: empty/local source first, then catalog) ─
	type customEntry struct {
		row     advisorRow
		sortKey string // CatalogURL (empty for local: sources sorts first)
	}

	var customEntries []customEntry
	for _, r := range resolved {
		var catalogURL string
		if r.Origin == "catalog" {
			catalogURL = r.Source
		}

		// Scope: prefer the SKILL.md frontmatter parsed by the Scanner (carried
		// in r.Def.Scope). For native-name overrides we may also have a value in
		// nativeScopes — Scanner output wins because it reflects the on-disk
		// SKILL.md inside the source dir, which is the new single source of
		// truth.
		scope := append([]string(nil), r.Def.Scope...)

		row := advisorRow{
			Name:        r.Def.Name,
			Description: r.Def.Description,
			Scope:       scope,
			Origin:      r.Origin,
			CatalogURL:  catalogURL,
			Installed:   true, // resolved advisors are explicitly registered
		}
		customEntries = append(customEntries, customEntry{row: row, sortKey: catalogURL})
	}

	sort.Slice(customEntries, func(i, j int) bool {
		ci, cj := customEntries[i], customEntries[j]
		if ci.sortKey != cj.sortKey {
			return ci.sortKey < cj.sortKey
		}
		return ci.row.Name < cj.row.Name
	})

	for _, ce := range customEntries {
		rows = append(rows, ce.row)
	}

	return rows
}

// applyManifestDiff mutates m in place, adding and removing advisor skill
// references. It is a pure function with respect to the filesystem — no reads
// or writes occur.
//
// On add:
//   - The name must be a known native advisor (via model.ReservedAdvisorNames)
//     OR present in any AdvisorSource.Select (or implied by an empty Select).
//     For non-native names not yet present anywhere, we accept the add only
//     when the name is already known (i.e. registered explicitly under some
//     AdvisorSource). Pure native names are always accepted.
//   - The skill is appended to m.Packages[primaryIdx].Select.Skills where
//     primaryIdx is the index of the first package that already has any advisor
//     skill; falls back to 0 when no package has advisors yet.
//   - Skills are kept sorted lexicographically for determinism.
//   - m.Packages must not be empty; error if it is.
//
// On remove:
//   - The skill name is stripped from every package's Select.Skills.
//   - The name is also stripped from every AdvisorSource.Select. If a Select
//     becomes empty AND was previously non-empty, the AdvisorSource entry is
//     removed entirely (an empty Select on a still-present source would mean
//     "install everything", which is the wrong intent after an explicit remove).
//   - Removing a name not present anywhere is a no-op (no error).
func applyManifestDiff(m *model.UserManifest, toAdd, toRemove []string) error {
	// ── build lookup sets ─────────────────────────────────────────────────────
	nativeSet := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		nativeSet[n] = true
	}

	customSet := buildAdvisorSourceSelectionSet(m)

	// ── process additions ─────────────────────────────────────────────────────
	for _, name := range toAdd {
		if !nativeSet[name] && !customSet[name] {
			return fmt.Errorf("unknown advisor %q", name)
		}

		if len(m.Packages) == 0 {
			return fmt.Errorf("cannot add advisor %q: manifest has no packages", name)
		}

		// Find primaryIdx: first package that already contains any advisor skill.
		primaryIdx := -1
		for i, pkg := range m.Packages {
			if pkg.Select == nil {
				continue
			}
			for _, s := range pkg.Select.Skills {
				if nativeSet[s] || customSet[s] {
					primaryIdx = i
					break
				}
			}
			if primaryIdx >= 0 {
				break
			}
		}
		if primaryIdx < 0 {
			primaryIdx = 0
		}

		// Initialise Select and Skills if needed.
		if m.Packages[primaryIdx].Select == nil {
			m.Packages[primaryIdx].Select = &model.SelectFilter{}
		}
		if m.Packages[primaryIdx].Select.Skills == nil {
			m.Packages[primaryIdx].Select.Skills = []string{}
		}

		// Append only if absent.
		alreadyPresent := false
		for _, s := range m.Packages[primaryIdx].Select.Skills {
			if s == name {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			m.Packages[primaryIdx].Select.Skills = append(m.Packages[primaryIdx].Select.Skills, name)
			sort.Strings(m.Packages[primaryIdx].Select.Skills)
		}
	}

	// ── process removals ──────────────────────────────────────────────────────
	for _, name := range toRemove {
		// Strip from every package.
		for i := range m.Packages {
			if m.Packages[i].Select == nil {
				continue
			}
			filtered := m.Packages[i].Select.Skills[:0]
			for _, s := range m.Packages[i].Select.Skills {
				if s != name {
					filtered = append(filtered, s)
				}
			}
			m.Packages[i].Select.Skills = filtered
		}

		// Strip from each AdvisorSource.Select. If a source had a non-empty
		// Select that now becomes empty, drop the source entirely (empty Select
		// would otherwise mean "install everything", which is the wrong intent
		// after an explicit removal).
		nextAdvisors := m.Advisors[:0]
		for _, src := range m.Advisors {
			if len(src.Select) == 0 {
				// Source installs everything — leave intact. The remove is a
				// no-op for this source (the user can drop the whole source
				// via the dedicated remove-advisor flow if that is the intent).
				nextAdvisors = append(nextAdvisors, src)
				continue
			}
			filtered := src.Select[:0]
			for _, n := range src.Select {
				if n != name {
					filtered = append(filtered, n)
				}
			}
			src.Select = filtered
			if len(src.Select) == 0 {
				// All explicit selections removed → drop the source.
				continue
			}
			nextAdvisors = append(nextAdvisors, src)
		}
		m.Advisors = nextAdvisors
	}

	return nil
}

// buildAdvisorSourceSelectionSet returns the set of advisor names that are
// EXPLICITLY listed in any AdvisorSource.Select. Sources with empty Select
// (= install everything) do not contribute to this set because we cannot
// enumerate their contents without I/O — applyManifestDiff is pure.
//
// This is a tradeoff: applyManifestDiff cannot know about advisors only
// implied by an empty Select. Callers that need that resolution must consult
// resolveAdvisors which DOES perform fetch+scan.
func buildAdvisorSourceSelectionSet(m *model.UserManifest) map[string]bool {
	set := make(map[string]bool)
	for _, src := range m.Advisors {
		for _, n := range src.Select {
			set[n] = true
		}
	}
	return set
}
