package model

import "fmt"

// ContentKind identifies the type of a content item within a package.
type ContentKind string

const (
	KindSkill  ContentKind = "skill"
	KindRule   ContentKind = "rule"
	KindPrompt ContentKind = "prompt"
	KindMemory ContentKind = "memory"
)

// RuleMeta holds metadata parsed from a rule's frontmatter.
// It is populated only for ContentItems with KindRule; nil for skills/prompts/memory.
type RuleMeta struct {
	Scope       string `yaml:"scope"`       // e.g. "architecture", "testing", "tech", "api"
	Technology  string `yaml:"technology"`  // e.g. "java", "any"
	AppliesTo   string `yaml:"applies_to"`  // comma-separated skill names
	Description string `yaml:"description"`  // human-readable description
	DisplayName  string `yaml:"display_name"` // optional display name from frontmatter; empty string falls back to ContentItem.Name
}

// ContentItem describes a single discoverable item within a resolved package.
// Each item has a kind, a logical name, and a relative path within the package.
type ContentItem struct {
	Kind        ContentKind `yaml:"kind"`
	Name        string      `yaml:"name"`                  // logical name, e.g. "git-commit" or "architecture/clean-architecture"
	Path        string      `yaml:"path"`                  // relative path within the package, e.g. "skills/git-commit/"
	Description string      `yaml:"description,omitempty"` // human-readable description from frontmatter
	RuleMeta    *RuleMeta   `yaml:"ruleMeta,omitempty"`    // rule metadata; nil for non-rule content items
}

// Validate checks that the ContentItem has all required fields.
func (c ContentItem) Validate() error {
	if c.Kind == "" {
		return fmt.Errorf("content item: kind is required")
	}
	switch c.Kind {
	case KindSkill, KindRule, KindPrompt, KindMemory:
		// valid
	default:
		return fmt.Errorf("content item: unknown kind %q", c.Kind)
	}
	if c.Name == "" {
		return fmt.Errorf("content item: name is required")
	}
	if c.Path == "" {
		return fmt.Errorf("content item: path is required")
	}
	return nil
}
