// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
)

// WorkflowEntry represents a single workflow declaration in devrune.yaml.
// It combines the workflow source ref with optional per-agent role model overrides.
type WorkflowEntry struct {
	Source string                       `yaml:"source"`
	Roles  map[string]map[string]string `yaml:"roles,omitempty"`
}

// UserManifest represents the user's devrune.yaml file.
// It declares packages, MCP servers, agents, and optional workflows to install.
type UserManifest struct {
	SchemaVersion string                   `yaml:"schemaVersion"`
	Packages      []PackageRef             `yaml:"packages"`
	MCPs          []MCPRef                 `yaml:"mcps,omitempty"`
	Agents        []AgentRef               `yaml:"agents"`
	Workflows     map[string]WorkflowEntry `yaml:"workflows,omitempty"` // name -> WorkflowEntry
	Catalogs      []string                 `yaml:"catalogs,omitempty"`  // catalog source refs
	Install       InstallConfig            `yaml:"install,omitempty"`
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

	return nil
}
