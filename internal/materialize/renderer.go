// SPDX-License-Identifier: MIT

package materialize

import (
	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
)

// CacheStore is re-exported from matypes for use within the materialize package.
type CacheStore = matypes.CacheStore

// AgentPaths is re-exported from matypes for use within the materialize package.
type AgentPaths = matypes.AgentPaths

// WorkflowInstallResult is re-exported from matypes for use within the materialize package.
type WorkflowInstallResult = matypes.WorkflowInstallResult

// AgentRenderer is the central abstraction in DevRune v3.3. Each built-in agent
// type (Claude, OpenCode, Copilot, Factory) has a compiled Go implementation.
// ALL agent-specific logic lives here — no YAML transform config needed.
//
// Adding a new agent means implementing this interface in Go. No YAML complexity.
type AgentRenderer interface {
	// Name returns the agent name (e.g. "claude", "opencode").
	Name() string

	// Definition returns the agent's workspace paths and config.
	Definition() model.AgentDefinition

	// RenderSkill converts canonical skill frontmatter to agent-native format.
	// canonicalPath may be a directory (containing SKILL.md) or a file path.
	// Output is written to destDir in agent-native naming/format.
	RenderSkill(canonicalPath string, destDir string) error

	// RenderCommand converts a canonical WorkflowCommand to agent-native format
	// and writes the result to destDir.
	RenderCommand(cmd model.WorkflowCommand, destDir string) error

	// RenderMCPs writes MCP configuration in agent-native format.
	RenderMCPs(mcps []model.LockedMCP, cacheStore CacheStore, workspaceRoot string) error

	// RenderSettings generates the agent's settings file (e.g. .claude/settings.json).
	// Agents that do not support settings generation return nil.
	RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error

	// InstallWorkflow materializes a workflow into the agent's workspace.
	// It returns a WorkflowInstallResult reporting the renderer-owned paths so
	// the materializer can track managed paths without guessing layout roots.
	//
	// cachePath is the absolute path to the workflow source directory (e.g.
	// "/cache/<hash>/workflows/sdd"). catalogRoot is the absolute path to the
	// containing catalog root (e.g. "/cache/<hash>"), which the renderer uses
	// to resolve skills referenced in `components.skills` that live outside
	// the workflow directory (catalog-level `skills/<name>/`). When the
	// workflow ships standalone with no catalog wrapper, catalogRoot may
	// equal cachePath; the renderer treats that as "no external lookup
	// available" and only resolves workflow-internal skill names.
	InstallWorkflow(wf model.WorkflowManifest, cachePath string, catalogRoot string, workspaceRoot string) (WorkflowInstallResult, error)

	// Finalize runs agent-specific post-processing after all content is installed.
	Finalize(workspaceRoot string) error

	// NeedsCopyMode returns true if this renderer modifies frontmatter.
	NeedsCopyMode() bool

	// AgentType returns the type string matching agent.yaml's "type" field.
	AgentType() string

	// WorkspacePaths returns the configured workspace paths for this renderer.
	WorkspacePaths() AgentPaths
}
