// SPDX-License-Identifier: MIT

package resolve

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// EnumerateContents discovers skills, rules, and prompts in an extracted package directory.
//
// Convention-based discovery:
//
//   - skills/  — each subdirectory that contains a SKILL.md is a skill.
//     Name: subdirectory name (e.g. "git-commit")
//     Path: "skills/{name}/"
//
//   - rules/   — each *.md file anywhere under rules/ is a rule.
//     Name: path relative to rules/ without .md suffix, using "/" separator
//     e.g. "architecture/clean-architecture/clean-architecture-rules"
//     Path: "rules/{relative-dir}/"
//
//   - prompts/ — each *.md file anywhere under prompts/ is a prompt.
//     Name: path relative to prompts/ without .md suffix
//     Path: "prompts/{relative-dir}/"
//
// Hidden files and directories (names starting with ".") are ignored.
// Returns an empty slice (not an error) for a non-existent or empty extractedDir.
func EnumerateContents(extractedDir string) ([]model.ContentItem, error) {
	var items []model.ContentItem

	skills, err := enumerateSkills(extractedDir)
	if err != nil {
		return nil, err
	}
	items = append(items, skills...)

	rules, err := enumerateMarkdownDir(extractedDir, "rules", model.KindRule)
	if err != nil {
		return nil, err
	}
	items = append(items, rules...)

	prompts, err := enumerateMarkdownDir(extractedDir, "prompts", model.KindPrompt)
	if err != nil {
		return nil, err
	}
	items = append(items, prompts...)

	return items, nil
}

// enumerateSkills discovers skills under extractedDir/skills/.
// A skill is a directory that directly contains a SKILL.md file.
func enumerateSkills(extractedDir string) ([]model.ContentItem, error) {
	skillsDir := filepath.Join(extractedDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var items []model.ContentItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		// A skill directory must contain SKILL.md directly.
		skillFile := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue // not a valid skill directory
		}

		// Read SKILL.md to extract description from frontmatter.
		description := ""
		if data, readErr := os.ReadFile(skillFile); readErr == nil {
			if fm, _, parseErr := parse.ParseFrontmatter(data); parseErr == nil {
				if v, ok := fm["description"]; ok {
					if s, ok := v.(string); ok {
						description = s
					}
				}
			}
		}

		items = append(items, model.ContentItem{
			Kind:        model.KindSkill,
			Name:        name,
			Path:        "skills/" + name + "/",
			Description: description,
		})
	}
	return items, nil
}

// enumerateMarkdownDir discovers markdown files under extractedDir/{subdir}/.
// Each *.md file becomes a content item with the given kind.
// Name is the relative path within subdir, stripped of the .md extension,
// using "/" separators. Empty subdirectories are silently skipped.
func enumerateMarkdownDir(extractedDir, subdir string, kind model.ContentKind) ([]model.ContentItem, error) {
	base := filepath.Join(extractedDir, subdir)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}

	var items []model.ContentItem

	err := filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		name := d.Name()

		// Skip hidden entries.
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(name, ".md") {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Logical name: relative path without .md suffix.
		logicalName := strings.TrimSuffix(rel, ".md")

		// Path is the relative path to the specific file within the package.
		itemPath := subdir + "/" + rel

		// Read the markdown file to extract frontmatter metadata for rules.
		item := model.ContentItem{
			Kind: kind,
			Name: logicalName,
			Path: itemPath,
		}
		if data, readErr := os.ReadFile(path); readErr == nil {
			if fm, _, parseErr := parse.ParseFrontmatter(data); parseErr == nil {
				if desc, ok := fm["description"]; ok {
					if s, ok := desc.(string); ok {
						item.Description = s
					}
				}
				if kind == model.KindRule {
					meta := &model.RuleMeta{}
					hasData := false
					if v, ok := fm["scope"]; ok {
						if s, ok := v.(string); ok {
							meta.Scope = s
							hasData = true
						}
					}
					if v, ok := fm["technology"]; ok {
						if s, ok := v.(string); ok {
							meta.Technology = s
							hasData = true
						}
					}
					// Try both "applies-to" (canonical YAML) and "applies_to" (legacy)
					if v, ok := fm["applies-to"]; ok {
						meta.AppliesTo = normalizeAppliesToValue(v)
						hasData = true
					} else if v, ok := fm["applies_to"]; ok {
						meta.AppliesTo = normalizeAppliesToValue(v)
						hasData = true
					}
					if v, ok := fm["description"]; ok {
						if s, ok := v.(string); ok {
							meta.Description = s
							hasData = true
						}
					}
					if v, ok := fm["name"]; ok {
						if s, ok := v.(string); ok {
							meta.DisplayName = s
							hasData = true
						}
					}
					if hasData {
						item.RuleMeta = meta
					}
				}
			}
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

// normalizeAppliesToValue converts a YAML applies-to value to a comma-separated string.
// Handles both []interface{} (YAML list) and string (legacy comma-separated) formats.
func normalizeAppliesToValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

// ApplyFilter returns only the content items that match the given SelectFilter.
// If filter is nil, all items are returned unchanged.
// Matching rules:
//   - A skill is included if its Name appears in filter.Skills (or filter.Skills is empty).
//   - A rule is included if its Name appears in filter.Rules (or filter.Rules is empty).
//   - When one kind has explicit selections and the other is empty, the empty kind
//     is excluded (not "include all"). This prevents a skills-only filter from
//     accidentally pulling in all rules from a large repo.
//   - Prompts are always included (not filterable in MVP).
func ApplyFilter(items []model.ContentItem, filter *model.SelectFilter) []model.ContentItem {
	if filter == nil {
		return items
	}

	skillSet := toSet(filter.Skills)
	ruleSet := toSet(filter.Rules)

	// When one kind has explicit selections and the other is empty,
	// treat the empty kind as "exclude all" rather than "include all".
	hasExplicitSkills := len(skillSet) > 0
	hasExplicitRules := len(ruleSet) > 0

	result := make([]model.ContentItem, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case model.KindSkill:
			if !hasExplicitSkills || skillSet[item.Name] {
				result = append(result, item)
			}
		case model.KindRule:
			if !hasExplicitRules {
				// If no explicit rule selection and no explicit skill selection either,
				// include all rules (backwards-compatible: nil-like filter).
				// But if skills are explicitly selected, exclude unselected rules.
				if !hasExplicitSkills {
					result = append(result, item)
				}
			} else if ruleSet[item.Name] {
				result = append(result, item)
			}
		default:
			// Prompts, memory — always include.
			result = append(result, item)
		}
	}
	return result
}

// toSet converts a string slice into a set (map[string]bool) for O(1) lookup.
func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
