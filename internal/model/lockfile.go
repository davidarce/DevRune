package model

import (
	"crypto/sha256"
	"fmt"
)

// Lockfile represents the devrune.lock file.
// It is the single source of truth for deterministic installs.
// All install operations must read from the lockfile, never from the manifest directly.
type Lockfile struct {
	SchemaVersion string           `yaml:"schemaVersion"`
	ManifestHash  string           `yaml:"manifestHash"` // SHA256 of devrune.yaml
	Packages      []LockedPackage  `yaml:"packages"`
	MCPs          []LockedMCP      `yaml:"mcps,omitempty"`
	Workflows     []LockedWorkflow `yaml:"workflows,omitempty"`
}

// LockedPackage is a fully resolved package entry in the lockfile.
type LockedPackage struct {
	Source   SourceRef     `yaml:"source"`
	Hash     string        `yaml:"hash"`     // SHA256 of fetched archive (e.g. "sha256:abc123...")
	Contents []ContentItem `yaml:"contents"` // enumerated content after select filtering
}

// LockedMCP is a fully resolved MCP server definition in the lockfile.
type LockedMCP struct {
	Source SourceRef `yaml:"source"`
	Hash   string    `yaml:"hash"`
	Name   string    `yaml:"name"`
}

// LockedWorkflow is a fully resolved workflow entry in the lockfile.
type LockedWorkflow struct {
	Source SourceRef `yaml:"source"`
	Hash   string    `yaml:"hash"`
	Name   string    `yaml:"name"`          // workflow name from workflow.yaml metadata.name
	Dir    string    `yaml:"dir,omitempty"` // relative path to workflow root within the cached archive dir (empty = root)
}

// Validate checks that the Lockfile has all required fields and is consistent.
func (l Lockfile) Validate() error {
	if l.SchemaVersion == "" {
		return fmt.Errorf("lockfile: schemaVersion is required")
	}
	if l.ManifestHash == "" {
		return fmt.Errorf("lockfile: manifestHash is required")
	}
	for i, pkg := range l.Packages {
		if err := pkg.Source.Validate(); err != nil {
			return fmt.Errorf("lockfile: package[%d] invalid source: %w", i, err)
		}
		if pkg.Hash == "" {
			return fmt.Errorf("lockfile: package[%d] hash is required", i)
		}
	}
	for i, mcp := range l.MCPs {
		if err := mcp.Source.Validate(); err != nil {
			return fmt.Errorf("lockfile: mcp[%d] invalid source: %w", i, err)
		}
		if mcp.Hash == "" {
			return fmt.Errorf("lockfile: mcp[%d] hash is required", i)
		}
		if mcp.Name == "" {
			return fmt.Errorf("lockfile: mcp[%d] name is required", i)
		}
	}
	for i, wf := range l.Workflows {
		if err := wf.Source.Validate(); err != nil {
			return fmt.Errorf("lockfile: workflow[%d] invalid source: %w", i, err)
		}
		if wf.Hash == "" {
			return fmt.Errorf("lockfile: workflow[%d] hash is required", i)
		}
		if wf.Name == "" {
			return fmt.Errorf("lockfile: workflow[%d] name is required", i)
		}
	}
	return nil
}

// ManifestHashMatches returns true if the given manifest bytes produce a SHA256 hash
// that matches the lockfile's stored ManifestHash.
// Expected format: "sha256:<hex>".
func (l Lockfile) ManifestHashMatches(manifestBytes []byte) bool {
	sum := sha256.Sum256(manifestBytes)
	computed := fmt.Sprintf("sha256:%x", sum)
	return computed == l.ManifestHash
}
