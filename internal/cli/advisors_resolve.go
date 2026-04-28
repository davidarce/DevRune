// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/davidarce/devrune/internal/advisorcatalog"
	"github.com/davidarce/devrune/internal/model"
)

// ResolvedAdvisor pairs the runtime AdvisorDef with the source-info needed by
// callers that previously inspected def.Origin / def.SkillSource directly.
//
//   - Def is the runtime view (Name, Description, Scope) populated from
//     SKILL.md frontmatter via the Scanner.
//   - Source is the scheme-prefixed URL of the AdvisorSource entry that
//     produced this advisor (e.g. "local:/path" or "github:owner/repo@ref").
//   - DirPath is the absolute path to the advisor's directory on disk
//     (the parent of SKILL.md), ready to feed copyAdvisorDir.
//   - Origin is "local" or "catalog", derived from Source's scheme. It is
//     a convenience for callers — never persisted.
type ResolvedAdvisor struct {
	Def     model.AdvisorDef
	Source  string
	DirPath string
	Origin  string
}

// resolveAdvisorOriginFromSource reports "local" for "local:" sources, and
// "catalog" for "github:"/"gitlab:" sources. Empty otherwise.
func resolveAdvisorOriginFromSource(source string) string {
	switch {
	case strings.HasPrefix(source, "local:"):
		return "local"
	case strings.HasPrefix(source, "github:"), strings.HasPrefix(source, "gitlab:"):
		return "catalog"
	default:
		return ""
	}
}

// resolveAdvisors walks manifest.Advisors[], fetches+scans each source, and
// returns the flat list of ResolvedAdvisor tuples after applying each entry's
// Select filter (empty Select = include all advisors discovered in source).
//
// Single-advisor-mode: when the resolved root directory itself is a single
// advisor directory (basename ends in "-advisor" AND a SKILL.md sits at root),
// a synthetic CatalogEntry is built for it. This mirrors the behavior of
// runAddAdvisorFlow.
//
// Errors fail-fast: any fetch/scan/parse error aborts the resolution.
//
// Pure helper — no manifest mutation, no side effects beyond the network/fs
// calls performed by the Fetcher.
func resolveAdvisors(ctx context.Context, wd string, m model.UserManifest) ([]ResolvedAdvisor, error) {
	if len(m.Advisors) == 0 {
		return nil, nil
	}

	var out []ResolvedAdvisor
	for _, src := range m.Advisors {
		cs := src.AsCatalogSource()
		rootDir, err := fetchCatalogSource(ctx, wd, cs)
		if err != nil {
			return nil, fmt.Errorf("resolveAdvisors: fetch %q: %w", src.Source, err)
		}

		var entries []advisorcatalog.CatalogEntry
		if isSingleAdvisorDir(rootDir) {
			entry, scanErr := singleAdvisorEntry(rootDir)
			if scanErr != nil {
				return nil, fmt.Errorf("resolveAdvisors: scan single-advisor %q: %w", src.Source, scanErr)
			}
			entries = []advisorcatalog.CatalogEntry{entry}
		} else {
			entries, err = advisorcatalog.DirScanner{}.Scan(rootDir)
			if err != nil {
				return nil, fmt.Errorf("resolveAdvisors: scan %q: %w", src.Source, err)
			}
		}

		// Apply Select filter (empty = include all).
		var keep map[string]bool
		if len(src.Select) > 0 {
			keep = make(map[string]bool, len(src.Select))
			for _, n := range src.Select {
				keep[n] = true
			}
		}

		origin := resolveAdvisorOriginFromSource(src.Source)
		for _, e := range entries {
			if keep != nil && !keep[e.Name] {
				continue
			}
			out = append(out, ResolvedAdvisor{
				Def: model.AdvisorDef{
					Name:        e.Name,
					Description: e.Description,
					Scope:       append([]string(nil), e.Scope...),
				},
				Source:  src.Source,
				DirPath: e.DirPath,
				Origin:  origin,
			})
		}
	}

	return out, nil
}

// findAdvisorSource returns a pointer to the AdvisorSource in m.Advisors with
// the given Source URL, or nil if none exists. Used by the add/remove flows
// that need to mutate (or extend) an existing entry's Select list.
func findAdvisorSource(m *model.UserManifest, source string) *model.AdvisorSource {
	for i := range m.Advisors {
		if m.Advisors[i].Source == source {
			return &m.Advisors[i]
		}
	}
	return nil
}
