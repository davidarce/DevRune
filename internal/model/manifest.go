// SPDX-License-Identifier: MIT

package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// UserManifest represents the user's devrune.yaml file.
// It declares packages, MCP servers, agents, and optional workflows to install.
type UserManifest struct {
	SchemaVersion string        `yaml:"schemaVersion"`
	Packages      []PackageRef  `yaml:"packages"`
	MCPs          []MCPRef      `yaml:"mcps,omitempty"`
	Agents        []AgentRef    `yaml:"agents"`
	Workflows     []string      `yaml:"workflows,omitempty"` // source ref strings
	Install       InstallConfig                `yaml:"install,omitempty"`
	// WorkflowModels holds per-agent, per-workflow role model selections.
	// Structure: WorkflowModels[agentName][roleName] = modelValue
	// The YAML key is "workflowModels" but we also accept the legacy "sddModels"
	// key via custom UnmarshalYAML for backward compatibility.
	WorkflowModels map[string]map[string]string `yaml:"workflowModels,omitempty"`
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
	LinkMode  string            `yaml:"linkMode,omitempty"`  // "symlink" | "copy" | "hardlink"
	RulesMode map[string]string `yaml:"rulesMode,omitempty"` // agent -> "concat" | "individual" | "both"
}

// SelectFilter allows the user to select a subset of a package's content.
type SelectFilter struct {
	Skills []string `yaml:"skills,omitempty"`
	Rules  []string `yaml:"rules,omitempty"`
}

// userManifestRaw is an alias used by UnmarshalYAML to decode without infinite recursion.
type userManifestRaw UserManifest

// legacyManifestOverlay captures only the legacy sddModels key from YAML.
type legacyManifestOverlay struct {
	SDDModels map[string]map[string]string `yaml:"sddModels,omitempty"`
}

// UnmarshalYAML implements custom YAML decoding for UserManifest.
// It migrates the legacy "sddModels" key to WorkflowModels when the new key is absent.
func (m *UserManifest) UnmarshalYAML(value *yaml.Node) error {
	// Decode into the raw alias to get all standard fields.
	var raw userManifestRaw
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*m = UserManifest(raw)

	// If WorkflowModels is already populated, nothing to migrate.
	if len(m.WorkflowModels) > 0 {
		return nil
	}

	// Check for legacy sddModels key.
	var legacy legacyManifestOverlay
	if err := value.Decode(&legacy); err != nil {
		return nil // non-fatal: ignore decode errors for legacy overlay
	}
	if len(legacy.SDDModels) > 0 {
		m.WorkflowModels = legacy.SDDModels
	}

	return nil
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
