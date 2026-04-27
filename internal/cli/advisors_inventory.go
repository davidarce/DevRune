// SPDX-License-Identifier: MIT

package cli

import (
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
	Origin      string // "", "local", "catalog", "github:acme", etc.
	CatalogURL  string // set only for catalog-imported advisors
	Installed   bool
}

// buildAdvisorInventory returns a fully annotated list of every known advisor —
// native, custom-local, and catalog-imported — merged and deduplicated.
//
// skillsRoot is the path to the on-disk skills directory (e.g.
// filepath.Join(wd, ".claude", "skills")). It is used to load native advisor
// scope declarations from each *-advisor/SKILL.md frontmatter. If skillsRoot
// is empty or the directory is inaccessible, native advisors are returned with
// nil scope (universal) and no error is propagated — the inventory degrades
// gracefully rather than crashing the TUI.
//
// Ordering guarantee (deterministic):
//  1. Native advisors, sorted by name.
//  2. Custom (local+catalog) advisors not already listed as native, sorted by
//     CatalogURL (empty first) then Name within each group.
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

	// ── build a lookup: advisor name → catalog URL (from AdvisorCatalogs) ────
	// We rely on the advisor's SkillSource matching the catalog URL (best-effort).
	// The SkillSource field for catalog-imported advisors stores the catalog-ref URL.
	catalogURLByName := make(map[string]string, len(m.CustomAdvisors))
	for _, def := range m.CustomAdvisors {
		if def.Origin == model.AdvisorOriginCatalog {
			// Try to find the matching catalog source by checking if the advisor's
			// SkillSource starts with (or equals) any catalog URL.
			matched := false
			for _, cat := range m.AdvisorCatalogs {
				if cat.URL != "" && def.SkillSource == cat.URL {
					catalogURLByName[def.Name] = cat.URL
					matched = true
					break
				}
			}
			if !matched && len(m.AdvisorCatalogs) > 0 {
				// best-effort: if only one catalog, attribute to it
				if len(m.AdvisorCatalogs) == 1 {
					catalogURLByName[def.Name] = m.AdvisorCatalogs[0].URL
				}
				// if multiple catalogs and no match, CatalogURL stays ""
			}
		}
	}

	// ── build a set of custom advisor names (to deduplicate against native) ───
	customByName := make(map[string]model.AdvisorDef, len(m.CustomAdvisors))
	for _, def := range m.CustomAdvisors {
		customByName[def.Name] = def
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

	// ── 2. Custom advisors (local then catalog, sorted) ───────────────────────
	// Separate into local and catalog groups for ordered output.
	type customEntry struct {
		def        model.AdvisorDef
		catalogURL string
	}

	var customEntries []customEntry
	for _, def := range m.CustomAdvisors {
		ce := customEntry{
			def:        def,
			catalogURL: catalogURLByName[def.Name],
		}
		customEntries = append(customEntries, ce)
	}

	// Sort: by CatalogURL (empty/local first) then by Name.
	sort.Slice(customEntries, func(i, j int) bool {
		ci, cj := customEntries[i], customEntries[j]
		if ci.catalogURL != cj.catalogURL {
			return ci.catalogURL < cj.catalogURL
		}
		return ci.def.Name < cj.def.Name
	})

	for _, ce := range customEntries {
		// Scope comes from the on-disk SKILL.md frontmatter — the single
		// source of truth. nativeScopes[name] returns nil (= universal) when
		// the advisor's SKILL.md is missing or has no scope; that is the
		// correct fallback.
		scope := append([]string(nil), nativeScopes[ce.def.Name]...)
		rows = append(rows, advisorRow{
			Name:        ce.def.Name,
			Description: ce.def.Description,
			Scope:       scope,
			Origin:      string(ce.def.Origin),
			CatalogURL:  ce.catalogURL,
			Installed:   true, // custom advisors are always explicitly registered
		})
	}

	return rows
}

// applyManifestDiff mutates m in place, adding and removing advisor skill
// references. It is a pure function with respect to the filesystem — no reads
// or writes occur.
//
// On add:
//   - The name must be a known native advisor (via model.ReservedAdvisorNames)
//     OR already present in m.CustomAdvisors. Unknown names → error.
//   - The skill is appended to m.Packages[primaryIdx].Select.Skills where
//     primaryIdx is the index of the first package that already has any advisor
//     skill; falls back to 0 when no package has advisors yet.
//   - Skills are kept sorted lexicographically for determinism.
//   - m.Packages must not be empty; error if it is.
//
// On remove:
//   - The skill name is stripped from every package's Select.Skills.
//   - Also stripped from m.CustomAdvisors (for custom/catalog entries).
//   - Removing a name not present anywhere is a no-op (no error).
func applyManifestDiff(m *model.UserManifest, toAdd, toRemove []string) error {
	// ── build lookup sets ─────────────────────────────────────────────────────
	nativeSet := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		nativeSet[n] = true
	}

	customSet := make(map[string]bool, len(m.CustomAdvisors))
	for _, def := range m.CustomAdvisors {
		customSet[def.Name] = true
	}

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

		// Strip from CustomAdvisors.
		filtered := m.CustomAdvisors[:0]
		for _, def := range m.CustomAdvisors {
			if def.Name != name {
				filtered = append(filtered, def)
			}
		}
		m.CustomAdvisors = filtered
	}

	return nil
}
