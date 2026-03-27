// SPDX-License-Identifier: MIT

// Package matypes defines shared interface types for the materialize layer.
// It exists as a separate package to break the import cycle between
// internal/materialize and internal/materialize/renderers.
package matypes

// CacheStore abstracts content-addressed storage access for renderers.
// Matches the cache.CacheStore interface to allow direct assignment.
type CacheStore interface {
	// Has reports whether a cached entry exists for the given SHA256 hash.
	Has(hash string) bool

	// Get returns the filesystem path to the extracted directory for the
	// given SHA256 hash. Returns ("", false) if not cached.
	Get(hash string) (dir string, ok bool)

	// Store writes the archive bytes to the cache and returns the extracted path.
	Store(key string, data []byte) (dir string, err error)
}

// AgentPaths bundles the workspace-relative directory paths for a given agent.
type AgentPaths struct {
	Workspace   string // e.g. ".claude"
	SkillDir    string // e.g. "skills" — reusable backing skill tree
	AgentDir    string // e.g. "agents" — surfaced native agent entry files (optional; empty means agent uses SkillDir for everything)
	CommandDir  string // e.g. "commands" (optional)
	RulesDir    string // e.g. "rules"
	CatalogFile string // e.g. "CLAUDE.md"
}

// WorkflowInstallResult is returned by InstallWorkflow to report the paths that
// the renderer created or owns for this workflow. The materializer uses these
// paths for managed-path tracking and stale-layout cleanup instead of guessing
// workflow roots from agent definitions.
type WorkflowInstallResult struct {
	ManagedPaths []string
}
