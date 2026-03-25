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

	// RenderCatalog generates the agent's guidance/catalog file.
	// skills is the list of installed skills; rules is the list of installed rules;
	// workflows is the list of installed workflow manifests.
	RenderCatalog(skills []model.ContentItem, rules []model.ContentItem, workflows []model.WorkflowManifest, destPath string) error

	// RenderSettings generates the agent's settings file (e.g. .claude/settings.json).
	// Agents that do not support settings generation return nil.
	RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error

	// InstallWorkflow materializes a workflow into the agent's workspace.
	// It returns a WorkflowInstallResult reporting the renderer-owned paths so
	// the materializer can track managed paths without guessing layout roots.
	InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (WorkflowInstallResult, error)

	// Finalize runs agent-specific post-processing after all content is installed.
	Finalize(workspaceRoot string) error

	// NeedsCopyMode returns true if this renderer modifies frontmatter.
	NeedsCopyMode() bool

	// AgentType returns the type string matching agent.yaml's "type" field.
	AgentType() string

	// WorkspacePaths returns the configured workspace paths for this renderer.
	WorkspacePaths() AgentPaths
}
