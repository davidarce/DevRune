// SPDX-License-Identifier: MIT

package recommend

// FrameworkSkills maps a framework name (as returned by detect.inferFrameworks)
// to its curated skills.sh skill references.
//
// The Framework field must match the exact string returned by detect.inferFrameworks()
// (e.g., "React", "Next.js", "Spring Boot").
type FrameworkSkills struct {
	Framework string     // must match detect.inferFrameworks() output, e.g. "React", "Next.js", "Spring Boot"
	Skills    []SkillRef // curated skills for this framework
}

// SkillRef is a curated skill reference from skills.sh.
// Path follows the format "owner/repo/skill-name".
type SkillRef struct {
	Path        string // skills.sh path: "owner/repo/skill-name"
	Description string // short description for TUI display
}

// DetectedTech is the result of matching detected frameworks against the registry.
type DetectedTech struct {
	Framework string     // display name (matches FrameworkSkills.Framework)
	Skills    []SkillRef // skills applicable for this framework
}

// CatalogFetcher provides skills grouped by technology.
// StaticCatalog is the default implementation (embedded map).
// Future implementations could call skills.sh API or other registries.
type CatalogFetcher interface {
	FetchByFrameworks(frameworks []string) ([]DetectedTech, error)
}
