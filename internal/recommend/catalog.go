// SPDX-License-Identifier: MIT

package recommend

// ScannedSource represents a scanned catalog source with its items.
// Defined here (not in tui) to avoid import cycles — tui converts its
// internal repo types to ScannedSource before calling BuildCatalogItems.
type ScannedSource struct {
	Source    string
	Skills    []string
	Rules     []string
	MCPs      []string
	Workflows []string
	Descs     map[string]string // item name → human-readable description
}

// BuildCatalogItems converts scanned sources into CatalogItem slices for the AI.
func BuildCatalogItems(sources []ScannedSource) []CatalogItem {
	var items []CatalogItem
	for _, src := range sources {
		for _, name := range src.Skills {
			items = append(items, CatalogItem{
				Name:        name,
				Kind:        "skill",
				Source:      src.Source,
				Description: src.Descs[name],
			})
		}
		for _, name := range src.Rules {
			items = append(items, CatalogItem{
				Name:        name,
				Kind:        "rule",
				Source:      src.Source,
				Description: src.Descs[name],
			})
		}
		for _, name := range src.MCPs {
			items = append(items, CatalogItem{
				Name:        name,
				Kind:        "mcp",
				Source:      src.Source,
				Description: src.Descs[name],
			})
		}
		for _, name := range src.Workflows {
			items = append(items, CatalogItem{
				Name:        name,
				Kind:        "workflow",
				Source:      src.Source,
				Description: src.Descs[name],
			})
		}
	}
	return items
}
