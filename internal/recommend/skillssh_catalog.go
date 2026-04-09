// SPDX-License-Identifier: MIT

package recommend

import (
	"github.com/davidarce/devrune/internal/detect"
)

// StaticCatalog is the default implementation of CatalogFetcher.
// It performs pure in-memory lookups against the package-level SkillsRegistry.
// No network calls are made by StaticCatalog itself; skill content is fetched
// only during `devrune resolve` via the existing cached MultiFetcher pipeline.
type StaticCatalog struct{}

// FetchByFrameworks returns DetectedTech entries for each framework name that
// has a matching entry in SkillsRegistry. Frameworks with no registry entry are
// silently ignored. The result preserves registry order for stable TUI display.
func (c StaticCatalog) FetchByFrameworks(frameworks []string) ([]DetectedTech, error) {
	if len(frameworks) == 0 {
		return nil, nil
	}

	// Build a set for O(1) lookups.
	want := make(map[string]struct{}, len(frameworks))
	for _, f := range frameworks {
		want[f] = struct{}{}
	}

	var result []DetectedTech
	for _, entry := range SkillsRegistry {
		if _, ok := want[entry.Framework]; ok {
			result = append(result, DetectedTech{
				Framework: entry.Framework,
				Skills:    entry.Skills,
			})
		}
	}
	return result, nil
}

// FetchByProfile returns DetectedTech entries matching both the detected
// frameworks and the detected languages in the given ProjectProfile.
//
// Framework entries are matched against profile.Frameworks; language entries
// are matched against profile.Languages[].Name. Results are deduplicated so
// that a registry entry matching both a framework name and a language name only
// appears once. The combined result preserves registry order.
func (c StaticCatalog) FetchByProfile(profile *detect.ProjectProfile) ([]DetectedTech, error) {
	if profile == nil {
		return nil, nil
	}

	// Collect all names to match: frameworks + languages.
	names := make(map[string]struct{}, len(profile.Frameworks)+len(profile.Languages))
	for _, f := range profile.Frameworks {
		names[f] = struct{}{}
	}
	for _, l := range profile.Languages {
		names[l.Name] = struct{}{}
	}

	if len(names) == 0 {
		return nil, nil
	}

	// Walk registry in order; deduplicate via seen set.
	seen := make(map[string]struct{})
	var result []DetectedTech
	for _, entry := range SkillsRegistry {
		if _, ok := names[entry.Framework]; !ok {
			continue
		}
		if _, already := seen[entry.Framework]; already {
			continue
		}
		seen[entry.Framework] = struct{}{}
		result = append(result, DetectedTech{
			Framework: entry.Framework,
			Skills:    entry.Skills,
		})
	}
	return result, nil
}

// BuildSkillsCatalogItems converts a slice of DetectedTech values into
// CatalogItem slices suitable for TUI display and AI recommendations.
// Each SkillRef in each DetectedTech produces one CatalogItem with:
//   - Name:        the last path segment of ref.Path (e.g. "vercel-react-best-practices")
//   - Kind:        "skill"
//   - Source:      the full ref.Path (e.g. "vercel-labs/agent-skills/vercel-react-best-practices")
//   - Description: ref.Description
func BuildSkillsCatalogItems(detected []DetectedTech) []CatalogItem {
	var items []CatalogItem
	for _, tech := range detected {
		for _, ref := range tech.Skills {
			items = append(items, CatalogItem{
				Name:        SkillName(ref.Path),
				Kind:        "skill",
				Source:      ref.Path,
				Description: ref.Description,
			})
		}
	}
	return items
}

// SkillName extracts the last path segment from a skills.sh path.
// "vercel-labs/agent-skills/vercel-react-best-practices" → "vercel-react-best-practices"
// "owner/repo" → "repo"
func SkillName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
