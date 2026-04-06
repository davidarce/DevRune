// SPDX-License-Identifier: MIT

package renderers

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// codexDropFields lists frontmatter fields not supported by the Codex agent format.
// Codex skills only use name and description; all other Claude/Factory-specific
// fields are dropped during rendering.
var codexDropFields = []string{
	"allowed-tools",
	"argument-hint",
	"disable-model-invocation",
	"tools-mode",
	"mode",
}

// CodexRenderer materializes skills and config for the OpenAI Codex CLI agent.
//
// Key characteristics:
//   - Skills output to .agents/skills/ (shared directory with OpenCode, Factory)
//   - MCP config written in TOML format to .codex/config.toml
//   - Frontmatter: keeps name and description only, drops unsupported fields
//   - Catalog: AGENTS.md at project root
type CodexRenderer struct {
	def              matypes.AgentPaths
	agentDef         model.AgentDefinition
	registryContents map[string]string // keyed by workflow name
	normalizedMCPs   []normalizedMCP   // MCP entries with agentInstructions for catalog injection
	modelOverrides   map[string]string // role-name → model-value from TUI SDD model selection
}

// NewCodexRenderer constructs a CodexRenderer from the given agent definition.
func NewCodexRenderer(agentDef model.AgentDefinition) *CodexRenderer {
	return &CodexRenderer{
		agentDef: agentDef,
		def: matypes.AgentPaths{
			Workspace:   agentDef.Workspace,
			SkillDir:    agentDef.SkillDir,
			CommandDir:  agentDef.CommandDir,
			RulesDir:    agentDef.RulesDir,
			CatalogFile: agentDef.CatalogFile,
		},
		registryContents: make(map[string]string),
		normalizedMCPs:   nil,
	}
}

// Name returns the agent name.
func (r *CodexRenderer) Name() string { return r.agentDef.Name }

// AgentType returns "codex".
func (r *CodexRenderer) AgentType() string { return "codex" }

// SetModelOverrides stores per-role model overrides. Codex does not support model
// routing but the field is stored for interface completeness.
func (r *CodexRenderer) SetModelOverrides(overrides map[string]string) {
	r.modelOverrides = overrides
}

// Definition returns the agent definition.
func (r *CodexRenderer) Definition() model.AgentDefinition { return r.agentDef }

// WorkspacePaths returns the configured workspace paths.
func (r *CodexRenderer) WorkspacePaths() matypes.AgentPaths { return r.def }

// NeedsCopyMode returns true — Codex renderer modifies frontmatter.
func (r *CodexRenderer) NeedsCopyMode() bool { return true }

// RegistryContents returns the registry contents map (keyed by workflow name).
func (r *CodexRenderer) RegistryContents() map[string]string {
	return r.registryContents
}

// MCPAgentInstructions returns MCP agent instructions extracted from normalizedMCPs.
func (r *CodexRenderer) MCPAgentInstructions() map[string]string {
	result := make(map[string]string)
	for _, mcp := range r.normalizedMCPs {
		if mcp.AgentInstructions != "" {
			result[mcp.Name] = mcp.AgentInstructions
		}
	}
	return result
}

// RenderSkill converts a canonical skill to Codex format and writes SKILL.md to destDir.
func (r *CodexRenderer) RenderSkill(canonicalPath string, destDir string) error {
	return r.renderSkillInternal(canonicalPath, destDir, false)
}

// renderSkillInternal is the shared implementation; isWorkflowSkill is unused for Codex
// (no user-invocable injection needed) but kept for symmetry with Factory pattern.
func (r *CodexRenderer) renderSkillInternal(canonicalPath, destDir string, _ bool) error {
	skillFile, err := resolveSkillFile(canonicalPath)
	if err != nil {
		return fmt.Errorf("codex: resolve skill %q: %w", canonicalPath, err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("codex: read %q: %w", skillFile, err)
	}

	fm, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("codex: parse frontmatter %q: %w", skillFile, err)
	}

	fm = r.transformFrontmatter(fm)

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("codex: serialize frontmatter: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("codex: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(dest, out, 0o644)
}

// transformFrontmatter applies Codex-specific frontmatter transformations.
// Drops all fields not supported by Codex; keeps name and description.
func (r *CodexRenderer) transformFrontmatter(fm map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(fm))
	for k, v := range fm {
		out[k] = v
	}
	for _, field := range codexDropFields {
		delete(out, field)
	}
	return out
}

// RenderCommand writes a Codex command file (as a skill) to destDir.
func (r *CodexRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	fm := map[string]interface{}{
		"name":        cmd.Name,
		"description": cmd.Action,
	}
	body := fmt.Sprintf("# %s\n\n%s\n", cmd.Name, cmd.Action)
	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("codex: render command %q: %w", cmd.Name, err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("codex: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(dest, out, 0o644)
}

// RenderMCPs writes MCP configuration in TOML format to .codex/config.toml.
//
// The TOML format uses [mcp_servers.<name>] table structure as required by the
// Codex CLI agent. ${VAR} placeholders are resolved from environment variables
// during Finalize().
func (r *CodexRenderer) RenderMCPs(mcps []model.LockedMCP, cacheStore matypes.CacheStore, workspaceRoot string) error {
	normalized, err := normalizeMCPDefinitions(mcps, cacheStore)
	if err != nil {
		return fmt.Errorf("codex: normalize MCP definitions: %w", err)
	}

	// Store normalized MCPs for catalog memory section injection.
	r.normalizedMCPs = normalized

	if len(normalized) == 0 {
		return nil
	}

	// Resolve effective MCP config from the agent definition.
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)

	// Build the mcp_servers map.
	servers := make(map[string]interface{})
	for _, n := range normalized {
		servers[n.Name] = applyMCPEnvTransform(n.ServerConfig, mcpConfig)
	}

	// Wrap in the root key for TOML serialization.
	mcpDoc := map[string]interface{}{
		mcpConfig.RootKey: servers,
	}

	data, err := toml.Marshal(mcpDoc)
	if err != nil {
		return fmt.Errorf("codex: marshal MCP TOML: %w", err)
	}

	// Write config file at the config-derived path.
	mcpPath := ResolveMCPOutputPath(workspaceRoot, mcpConfig)
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return fmt.Errorf("codex: mkdir for mcp config: %w", err)
	}
	return os.WriteFile(mcpPath, data, 0o644)
}

// RenderSettings is a no-op for Codex v1. Codex permissions are managed separately
// via config.toml and will be supported in a future enhancement.
func (r *CodexRenderer) RenderSettings(_ string, _ []model.ContentItem, _ []model.WorkflowManifest) error {
	if r.agentDef.Settings == nil {
		return nil
	}
	// TODO: Codex permissions in config.toml (future enhancement)
	return nil
}

// InstallWorkflow materializes a workflow into the Codex workspace.
//
// Layout:
//   - Skills: .agents/skills/<skill-name>/SKILL.md
//   - Orchestrator: .agents/skills/<wf-name>-orchestrator/ORCHESTRATOR.md
//   - _shared: .agents/skills/_shared/
//   - REGISTRY.md: captured for catalog injection (not copied loose)
func (r *CodexRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	skillsBase := filepath.Join(workspaceRoot, r.def.SkillDir)
	workflowDir := filepath.Join(skillsBase, wf.Metadata.EffectiveWorkingDir())

	if err := os.MkdirAll(skillsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: workflow mkdir skills: %w", err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: read workflow dir %q: %w", cachePath, err)
	}

	var managedPaths []string

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(cachePath, name)

		if name == "workflow.yaml" {
			continue
		}

		// Skip the registry file — captured below, not copied loose.
		if wf.Components.Registry != "" && name == wf.Components.Registry {
			continue
		}

		// Skip all orchestrator variant files — none should be copied as loose files.
		// Codex uses the generic ORCHESTRATOR.md entrypoint; agent-specific variants
		// are for OpenCode and Copilot only.
		if orchestratorVariantNames[name] {
			continue
		}

		if skillsSet[name] {
			// Workflow skills: .agents/skills/<skill-name>/SKILL.md
			destDir := filepath.Join(skillsBase, name)
			if err := r.renderSkillInternal(srcPath, destDir, true); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: workflow skill %q: %w", name, err)
			}
			if err := copySkillSubdirs(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: copy skill subdirs for %s: %w", name, err)
			}
			managedPaths = append(managedPaths, destDir)
			continue
		}

		if name == wf.Components.Entrypoint {
			// Orchestrator: .agents/skills/<workingDir>/ORCHESTRATOR.md
			if err := os.MkdirAll(workflowDir, 0o755); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: mkdir orchestrator: %w", err)
			}
			dstPath := filepath.Join(workflowDir, name)
			if err := copySingleFile(srcPath, dstPath, 0o644); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: workflow entrypoint: %w", err)
			}
			managedPaths = append(managedPaths, workflowDir)
			continue
		}

		// Copy everything else (e.g. _shared/) as-is under workflowDir.
		dstPath := filepath.Join(workflowDir, name)
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: workflow copy %q: %w", name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// Build shared placeholder replacements: {SKILLS_PATH} only.
	// Codex does not support model routing in the TUI, so {SDD_MODEL_*} placeholders
	// are removed entirely by removeModelPlaceholderLines below.
	replacements := buildWorkflowPathReplacements(wf, workspaceRoot, r.def.SkillDir)

	// Capture registry content for catalog injection; apply shared replacements.
	if wf.Components.Registry != "" {
		content, err := captureRegistryContent(cachePath, wf.Components.Registry, replacements)
		if err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: capture registry: %w", err)
		}
		if content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// Resolve placeholders in all installed .md files (covers {SKILLS_PATH}).
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: resolve placeholders: %w", err)
	}

	// Remove any lines containing unresolved {SDD_MODEL_*} placeholders.
	if err := removeModelPlaceholderLines(skillsBase); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("codex: remove model placeholder lines: %w", err)
	}

	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// ManagedConfigPaths returns the workspace-level config file paths that the Codex
// renderer owns and that the materializer should track for cleanup purposes.
func (r *CodexRenderer) ManagedConfigPaths(workspaceRoot string) []string {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	return []string{ResolveMCPOutputPath(workspaceRoot, mcpConfig)}
}

// Finalize resolves ${VAR} patterns in .codex/config.toml using os.Getenv().
// The TOML file is treated as raw text for placeholder resolution — no
// parse/reserialize round-trip needed.
func (r *CodexRenderer) Finalize(workspaceRoot string) error {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	mcpPath := ResolveMCPOutputPath(workspaceRoot, mcpConfig)
	data, err := os.ReadFile(mcpPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("codex: finalize read config.toml: %w", err)
	}

	resolved := resolveEnvPlaceholders(string(data))
	return os.WriteFile(mcpPath, []byte(resolved), 0o644)
}
