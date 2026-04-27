// SPDX-License-Identifier: MIT

package advisorcatalog

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// DirScanner implements Scanner by walking a single level of a catalog root
// directory and collecting every subdirectory whose name ends in "-advisor".
type DirScanner struct{}

// Scan reads the top-level subdirectories of catalogRoot and returns a
// CatalogEntry for each directory that:
//  1. Has a name ending in "-advisor".
//  2. Contains a SKILL.md file.
//
// Directories whose names do not end in "-advisor" are skipped with a warning
// log. Directories that are missing SKILL.md are also skipped with a warning.
// Regular files at the root level are silently ignored.
//
// The returned slice is sorted by CatalogEntry.Name in ascending order.
func (s DirScanner) Scan(catalogRoot string) ([]CatalogEntry, error) {
	entries, err := os.ReadDir(catalogRoot)
	if err != nil {
		return nil, err
	}

	var result []CatalogEntry

	for _, entry := range entries {
		// Skip regular files silently.
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Only process directories that end with "-advisor".
		if !strings.HasSuffix(name, "-advisor") {
			log.Printf("advisorcatalog: skipping %s (not an advisor directory — name must end in -advisor)", name)
			continue
		}

		dirPath := filepath.Join(catalogRoot, name)
		skillPath := filepath.Join(dirPath, "SKILL.md")

		// Check that SKILL.md exists.
		if _, statErr := os.Stat(skillPath); os.IsNotExist(statErr) {
			log.Printf("advisorcatalog: skipping %s: missing SKILL.md", filepath.Join(catalogRoot, name))
			continue
		}

		// Read and parse the frontmatter from SKILL.md.
		data, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			log.Printf("advisorcatalog: skipping %s: cannot read SKILL.md: %v", filepath.Join(catalogRoot, name), readErr)
			continue
		}

		fm, _, parseErr := parse.ParseFrontmatter(data)
		if parseErr != nil {
			log.Printf("advisorcatalog: skipping %s: invalid SKILL.md frontmatter: %v", filepath.Join(catalogRoot, name), parseErr)
			continue
		}

		description := ""
		if v, ok := fm["description"]; ok {
			if str, ok := v.(string); ok {
				description = str
			}
		}

		var rawScope []string
		if v, ok := fm["scope"]; ok {
			if list, ok := v.([]interface{}); ok {
				for _, elem := range list {
					if s, ok := elem.(string); ok {
						rawScope = append(rawScope, strings.TrimSpace(s))
					}
				}
			}
		}
		scope := model.NormalizeAdvisorScope(rawScope)

		result = append(result, CatalogEntry{
			Name:        name,
			Description: description,
			Scope:       scope,
			SKILLPath:   skillPath,
			DirPath:     dirPath,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}
