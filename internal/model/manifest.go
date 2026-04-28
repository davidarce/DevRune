// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"strings"
)

// WorkflowEntry represents a single workflow declaration in devrune.yaml.
// It combines the workflow source ref with optional per-agent role model overrides.
type WorkflowEntry struct {
	Source string                       `yaml:"source"`
	Roles  map[string]map[string]string `yaml:"roles,omitempty"`
}

// AdvisorDef is the runtime representation of a single advisor. It is
// populated from advisorcatalog.Scanner output (which reads SKILL.md
// frontmatter from the resolved source directory) and consumed by
// recommend, materialize, and the TUI.
//
// AdvisorDef is NEVER persisted to devrune.yaml. The persisted shape is
// UserManifest.Advisors []AdvisorSource — one entry per source with an
// optional Select list of names to install. All AdvisorDef fields are
// derived at runtime from the resolved source's SKILL.md files.
//
// Name is the canonical identifier (must end in "-advisor" and match the
// directory under .claude/skills/). Description populates the generated
// agent wrapper's frontmatter. Scope is the set of project domains this
// advisor applies to (empty slice = universal). Values come from the
// controlled vocabulary defined by the AdvisorScope* constants.
type AdvisorDef struct {
	Name        string
	Description string
	Scope       []string
}

// AdvisorScope controlled vocabulary constants.
// These are the recognized domain tags for the Scope field on AdvisorDef.
// Vocabulary matching is case-sensitive; canonical form is lower-case.
const (
	AdvisorScopeFrontend      = "frontend"
	AdvisorScopeBackend       = "backend"
	AdvisorScopeTesting       = "testing"
	AdvisorScopeArchitecture  = "architecture"
	AdvisorScopeAPI           = "api"
	AdvisorScopeSecurity      = "security"
	AdvisorScopePerformance   = "performance"
	AdvisorScopeAccessibility = "accessibility"
)

// validAdvisorScopes is the recognized vocabulary used by NormalizeAdvisorScope.
// Values not in this set are silently dropped at normalization time (soft fallback).
var validAdvisorScopes = map[string]bool{
	AdvisorScopeFrontend:      true,
	AdvisorScopeBackend:       true,
	AdvisorScopeTesting:       true,
	AdvisorScopeArchitecture:  true,
	AdvisorScopeAPI:           true,
	AdvisorScopeSecurity:      true,
	AdvisorScopePerformance:   true,
	AdvisorScopeAccessibility: true,
}

// Validate checks that the AdvisorDef name is well-formed.
//
// Rules:
//   - Name must be non-empty and end in "-advisor" (case-sensitive).
//
// Scope content is intentionally NOT validated here. Vocabulary checking and
// deduplication are performed by NormalizeAdvisorScope at the loader boundary
// (advisormeta ingestion paths). Unknown scope values are silently dropped
// — never rejected — to preserve forward compatibility. By the time Validate
// runs, scope is already normalized.
func (a AdvisorDef) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("advisor: name must not be empty")
	}
	// Reject names that contain leading or trailing whitespace (without trimming),
	// require a non-trivial prefix before the suffix (bare "-advisor" is rejected),
	// and require the canonical lowercase "-advisor" suffix (case-sensitive).
	if strings.TrimSpace(a.Name) != a.Name || !strings.HasSuffix(a.Name, "-advisor") || len(a.Name) == len("-advisor") {
		return fmt.Errorf("advisor: name %q must end in \"-advisor\"", a.Name)
	}
	return nil
}

// AdvisorSource is one entry in UserManifest.Advisors. It declares a
// catalog source (local:/github:/gitlab:) and optionally selects which
// advisors from that source to install. When Select is empty or nil, ALL
// "-advisor" directories discovered by Scanner under the resolved source
// are installed.
//
// The shape mirrors UserManifest.Packages so users have one mental model:
// a source plus an optional select list.
//
//   - Source is scheme-prefixed (local:/path, local:./relative,
//     github:owner/repo[@ref], gitlab:owner/repo[@ref]). It is the same
//     URL grammar as CatalogSource.URL — see CatalogSource.Validate for
//     scheme rules.
//   - LastFetched is set by the fetcher after successful github:/gitlab:
//     fetches. local: sources never need a fetch and leave it empty.
//   - Select lists advisor names to install from this source (each must
//     end in "-advisor"). Empty/nil means "install everything".
type AdvisorSource struct {
	Source      string   `yaml:"source"`
	LastFetched string   `yaml:"lastFetched,omitempty"`
	Select      []string `yaml:"select,omitempty"`
}

// AsCatalogSource converts an AdvisorSource into the CatalogSource shape
// that the advisorcatalog package's Fetcher accepts. The conversion is
// lossless because both share the URL+LastFetched fields.
func (a AdvisorSource) AsCatalogSource() CatalogSource {
	return CatalogSource{URL: a.Source, LastFetched: a.LastFetched}
}

// Validate checks that the AdvisorSource is internally consistent.
// Rules:
//   - Source must satisfy CatalogSource.Validate (scheme + body grammar).
//   - Each entry in Select must end in "-advisor".
func (a AdvisorSource) Validate() error {
	cs := a.AsCatalogSource()
	if err := cs.Validate(); err != nil {
		return err
	}
	for _, name := range a.Select {
		if name == "" {
			return fmt.Errorf("advisor source %q: select entry must not be empty", a.Source)
		}
		if strings.TrimSpace(name) != name || !strings.HasSuffix(name, "-advisor") || len(name) == len("-advisor") {
			return fmt.Errorf("advisor source %q: select entry %q must end in \"-advisor\"", a.Source, name)
		}
	}
	return nil
}

// NormalizeAdvisorScope sanitises a raw scope slice. It:
//   - trims whitespace from each element,
//   - drops empty (post-trim) elements,
//   - drops elements not in validAdvisorScopes (soft fallback — unknown values
//     are NEVER an error; this preserves forward compatibility for new vocab tags),
//   - dedupes while preserving first-seen order,
//   - returns nil if no recognized values remain (= universal).
//
// Vocabulary matching is case-sensitive. Authors are expected to use the
// canonical lower-case tags. A mixed-case input like "Frontend" is treated as
// unknown and silently dropped — same forward-compat policy.
//
// NormalizeAdvisorScope never returns a non-nil empty slice; callers should
// treat a nil return as "universal" (applies to every project).
func NormalizeAdvisorScope(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue // soft drop: empty element
		}
		if !validAdvisorScopes[s] {
			continue // soft drop: unknown vocab — forward-compat
		}
		if seen[s] {
			continue // soft drop: duplicate
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil // all values dropped → universal
	}
	return out
}

// CatalogSource is a persisted reference to an advisor catalog. The TUI
// "Add advisor" flow uses these; "Refresh catalogs" re-fetches them.
// URL is scheme-prefixed:
//
//	local:/abs/path         — filesystem directory (no fetch, no cache)
//	local:./relative/path   — resolved against devrune.yaml directory
//	github:owner/repo       — GitHub repo (default branch); fetched via git clone --depth=1
//	github:owner/repo@ref   — GitHub repo at a specific branch/tag/SHA
//	gitlab:owner/repo[@ref] — GitLab repo
//
// CatalogSource is the primitive consumed by advisorcatalog.Fetcher. It is
// no longer persisted directly in devrune.yaml — the manifest now uses
// AdvisorSource (which embeds the same URL grammar) as the persisted shape.
// CatalogSource lives on as the runtime input to Fetcher / Scanner.
type CatalogSource struct {
	URL         string `yaml:"url"`
	Name        string `yaml:"name,omitempty"`        // human-readable alias, optional
	LastFetched string `yaml:"lastFetched,omitempty"` // RFC3339; blank if never fetched
}

// validCatalogSchemes is the set of accepted URL scheme prefixes for CatalogSource.
var validCatalogSchemes = []string{"local:", "github:", "gitlab:"}

// Validate checks that the CatalogSource is internally consistent.
// Rules:
//   - URL must be non-empty.
//   - URL must have one of the accepted scheme prefixes: local:, github:, gitlab:.
//   - For github: and gitlab:, the body must match "<owner>/<repo>" or "<owner>/<repo>@<ref>"
//     where <ref> is non-empty (trailing "@" with no ref is rejected).
//   - For local:, body presence is validated by the fetcher; only the prefix is checked here.
func (c CatalogSource) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("catalogSource: url must not be empty")
	}

	var scheme, body string
	for _, prefix := range validCatalogSchemes {
		if strings.HasPrefix(c.URL, prefix) {
			scheme = prefix
			body = c.URL[len(prefix):]
			break
		}
	}
	if scheme == "" {
		return fmt.Errorf("catalogSource: url %q has an unrecognised scheme (must be one of local:, github:, gitlab:)", c.URL)
	}

	if scheme == "github:" || scheme == "gitlab:" {
		if body == "" {
			return fmt.Errorf("catalogSource: url %q: %s body must not be empty", c.URL, scheme)
		}
		// Must contain at least one slash (owner/repo).
		slashIdx := strings.Index(body, "/")
		if slashIdx < 0 {
			return fmt.Errorf("catalogSource: url %q: %s body must be in the form \"owner/repo\" or \"owner/repo@ref\"", c.URL, scheme)
		}
		// If an "@" is present it must have a non-empty ref after it.
		if atIdx := strings.Index(body, "@"); atIdx >= 0 {
			ref := body[atIdx+1:]
			if ref == "" {
				return fmt.Errorf("catalogSource: url %q: ref after \"@\" must not be empty", c.URL)
			}
		}
	}

	return nil
}

// reservedAdvisorNames is the canonical list of native advisor names that ship
// with DevRune via the starter catalog (devrune-starter-catalog/skills/*-advisor).
// User-installed custom advisors (manifest.advisors[].select) are NOT in this list
// even when their copy lives under .claude/skills/. The integrity test in
// internal/advisormeta/ verifies every name in this slice exists on disk; it
// does NOT fail on extra *-advisor directories (those are legitimate user
// customs). When DevRune adds or removes a SHIPPED native, update this slice.
var reservedAdvisorNames = []string{
	"architect-advisor",
	"api-first-advisor",
	"unit-test-advisor",
	"integration-test-advisor",
	"frontend-test-advisor",
	"component-advisor",
	"web-accessibility-advisor",
}

// ReservedAdvisorNames returns a copy of the canonical reserved advisor name list.
// The slice itself stays unexported so callers cannot mutate it. The integrity
// test in internal/advisormeta/ uses this to diff against the on-disk catalog.
func ReservedAdvisorNames() []string {
	out := make([]string, len(reservedAdvisorNames))
	copy(out, reservedAdvisorNames)
	return out
}

// UserManifest represents the user's devrune.yaml file.
// It declares packages, MCP servers, agents, and optional workflows to install.
type UserManifest struct {
	SchemaVersion string                   `yaml:"schemaVersion"`
	Packages      []PackageRef             `yaml:"packages"`
	MCPs          []MCPRef                 `yaml:"mcps,omitempty"`
	Agents        []AgentRef               `yaml:"agents"`
	Workflows     map[string]WorkflowEntry `yaml:"workflows,omitempty"` // name -> WorkflowEntry
	// Catalogs holds refs to the PRIMARY package catalog (e.g. github:owner/devrune-starter-catalog).
	// This is distinct from Advisors — see Advisors comment for details.
	Catalogs []string      `yaml:"catalogs,omitempty"`
	Install  InstallConfig `yaml:"install,omitempty"`
	// Advisors lists external advisor sources (local directories, GitHub
	// repos, GitLab repos) and which advisors to install from each. Each
	// entry mirrors the Packages shape — a Source plus an optional Select
	// list of names. When Select is empty/nil, ALL "-advisor" directories
	// discovered by Scanner under the resolved source are installed.
	//
	// NOTE: This is distinct from the top-level Catalogs field, which
	// references the primary DevRune package catalog (where DevRune
	// packages come from). Advisors holds advisor-only sources.
	Advisors []AdvisorSource `yaml:"advisors,omitempty"`
}

// PackageRef is a reference to a package in the user manifest.
type PackageRef struct {
	Source string        `yaml:"source"` // raw source ref string, e.g. "github:owner/repo@ref//subpath"
	Select *SelectFilter `yaml:"select,omitempty"`
}

// MCPRef is a reference to an MCP server definition.
type MCPRef struct {
	Source string `yaml:"source"` // source ref to MCP definition YAML
}

// AgentRef names an agent to configure during installation.
type AgentRef struct {
	Name string `yaml:"name"` // e.g. "claude", "opencode", "copilot", "factory"
}

// InstallConfig holds installation preferences declared in the user manifest.
type InstallConfig struct {
	LinkMode      string            `yaml:"linkMode,omitempty"`      // "symlink" | "copy" | "hardlink"
	RulesMode     map[string]string `yaml:"rulesMode,omitempty"`     // agent -> "concat" | "individual" | "both"
	AutoRecommend *bool             `yaml:"autoRecommend,omitempty"` // nil = enabled; explicit false disables auto-recommend
}

// SelectFilter allows the user to select a subset of a package's content.
type SelectFilter struct {
	Skills []string `yaml:"skills,omitempty"`
	Rules  []string `yaml:"rules,omitempty"`
}

// Validate checks that the UserManifest has all required fields and is consistent.
func (m UserManifest) Validate() error {
	if m.SchemaVersion == "" {
		return fmt.Errorf("manifest: schemaVersion is required")
	}
	if len(m.Agents) == 0 {
		return fmt.Errorf("manifest: at least one agent must be specified")
	}

	// Check for duplicate package sources
	seen := make(map[string]bool, len(m.Packages))
	for _, pkg := range m.Packages {
		if pkg.Source == "" {
			return fmt.Errorf("manifest: package source must not be empty")
		}
		if seen[pkg.Source] {
			return fmt.Errorf("manifest: duplicate package source %q", pkg.Source)
		}
		seen[pkg.Source] = true
	}

	// Build a set of reserved native advisor names for collision detection.
	reserved := make(map[string]bool, len(reservedAdvisorNames))
	for _, name := range reservedAdvisorNames {
		reserved[name] = true
	}

	// Validate Advisors: no duplicate sources, each entry valid, no select
	// names that collide with native DevRune advisor names.
	seenSources := make(map[string]bool, len(m.Advisors))
	for _, src := range m.Advisors {
		if err := src.Validate(); err != nil {
			return fmt.Errorf("manifest: %w", err)
		}
		if seenSources[src.Source] {
			return fmt.Errorf("manifest: duplicate advisor source %q", src.Source)
		}
		seenSources[src.Source] = true
		for _, name := range src.Select {
			if reserved[name] {
				return fmt.Errorf("manifest: advisor source %q select entry %q conflicts with a native DevRune advisor", src.Source, name)
			}
		}
	}

	return nil
}
