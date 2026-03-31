// SPDX-License-Identifier: MIT

package renderers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// factoryDropFields lists fields not used by the Factory agent format.
var factoryDropFields = []string{
	"argument-hint",
	"disable-model-invocation",
	"reasoning-effort", // renamed to reasoningEffort
}

// FactoryRenderer materializes skills and config for the Factory AI agent.
//
// Key transformations applied to canonical frontmatter:
//   - Injects: user-invocable: false (for workflow skills)
//   - Resolves: model short name → full Factory model ID
//   - Renames: reasoning-effort → reasoningEffort (camelCase)
//   - Entrypoints: installed as .factory/skills/sdd-orchestrator/ORCHESTRATOR.md
//   - MCP: written to .factory/mcp.json (workspace root), sanitized (no agentInstructions/name)
//   - Registry: captured for catalog injection (not copied as loose file)
//   - Memory/Engram: MCP agentInstructions injected into AGENTS.md catalog
type FactoryRenderer struct {
	def              matypes.AgentPaths
	agentDef         model.AgentDefinition
	registryContents map[string]string // keyed by workflow name
	normalizedMCPs   []normalizedMCP   // MCP entries with agentInstructions for catalog injection
	modelOverrides   map[string]string // role-name → model-value from TUI SDD model selection
}

// NewFactoryRenderer constructs a FactoryRenderer from the given agent definition.
func NewFactoryRenderer(agentDef model.AgentDefinition) *FactoryRenderer {
	return &FactoryRenderer{
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
func (r *FactoryRenderer) Name() string { return r.agentDef.Name }

// AgentType returns "factory".
func (r *FactoryRenderer) AgentType() string { return "factory" }

// SetModelOverrides stores per-role model overrides selected in the TUI SDD model
// selection step. When set, these override the role.Model values from workflow.yaml
// when building {SDD_MODEL_*} placeholder replacements.
func (r *FactoryRenderer) SetModelOverrides(overrides map[string]string) {
	r.modelOverrides = overrides
}

// Definition returns the agent definition.
func (r *FactoryRenderer) Definition() model.AgentDefinition { return r.agentDef }

// WorkspacePaths returns the configured workspace paths.
func (r *FactoryRenderer) WorkspacePaths() matypes.AgentPaths { return r.def }

// NeedsCopyMode returns true — Factory modifies frontmatter.
func (r *FactoryRenderer) NeedsCopyMode() bool { return true }

// RenderSkill converts a canonical skill to Factory format and writes SKILL.md to destDir.
func (r *FactoryRenderer) RenderSkill(canonicalPath string, destDir string) error {
	return r.renderSkillInternal(canonicalPath, destDir, false)
}

// renderSkillInternal is the shared implementation; isWorkflowSkill controls user-invocable injection.
func (r *FactoryRenderer) renderSkillInternal(canonicalPath, destDir string, isWorkflowSkill bool) error {
	skillFile, err := resolveSkillFile(canonicalPath)
	if err != nil {
		return fmt.Errorf("factory: resolve skill %q: %w", canonicalPath, err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("factory: read %q: %w", skillFile, err)
	}

	fm, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("factory: parse frontmatter %q: %w", skillFile, err)
	}

	fm = r.transformFrontmatter(fm, isWorkflowSkill)

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("factory: serialize frontmatter: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("factory: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(dest, out, 0o644)
}

// transformFrontmatter applies all Factory-specific frontmatter conversions.
func (r *FactoryRenderer) transformFrontmatter(fm map[string]interface{}, isWorkflowSkill bool) map[string]interface{} {
	out := make(map[string]interface{}, len(fm)+2)
	for k, v := range fm {
		out[k] = v
	}

	// Drop fields not used by Factory.
	for _, field := range factoryDropFields {
		delete(out, field)
	}

	// Resolve model short name.
	if modelVal, ok := out["model"]; ok {
		if modelStr, ok := modelVal.(string); ok && modelStr != "" {
			out["model"] = resolveModel(modelStr)
		}
	}

	// Rename reasoning-effort → reasoningEffort (camelCase).
	if val, ok := fm["reasoning-effort"]; ok {
		out["reasoningEffort"] = val
		delete(out, "reasoning-effort")
	}

	// Inject user-invocable: false for workflow skills (subagents).
	if isWorkflowSkill {
		out["user-invocable"] = false
	}

	return out
}

// RenderCommand writes a Factory command file to destDir.
func (r *FactoryRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	fm := map[string]interface{}{
		"name":        cmd.Name,
		"description": cmd.Action,
	}
	body := fmt.Sprintf("# %s\n\n%s\n", cmd.Name, cmd.Action)
	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("factory: render command %q: %w", cmd.Name, err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("factory: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(dest, out, 0o644)
}

// RenderMCPs writes sanitized MCP configuration to the path defined by the agent's MCP config.
// Only runtime/transport fields are written (agentInstructions and name are stripped).
// Extracted agentInstructions are stored for later catalog injection by RenderCatalog.
// ${VAR} placeholders are resolved from environment variables during Finalize().
func (r *FactoryRenderer) RenderMCPs(mcps []model.LockedMCP, cacheStore matypes.CacheStore, workspaceRoot string) error {
	normalized, err := normalizeMCPDefinitions(mcps, cacheStore)
	if err != nil {
		return fmt.Errorf("factory: normalize MCP definitions: %w", err)
	}

	// Store normalized MCPs for catalog memory section injection.
	r.normalizedMCPs = normalized

	// Resolve effective MCP config from the agent definition.
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)

	servers := make(map[string]interface{})
	for _, n := range normalized {
		servers[n.Name] = applyMCPEnvTransform(n.ServerConfig, mcpConfig)
	}

	mcpJSON := map[string]interface{}{
		mcpConfig.RootKey: servers,
	}

	data, err := json.MarshalIndent(mcpJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("factory: marshal MCP JSON: %w", err)
	}
	data = append(data, '\n')

	// Write mcp config file at the config-derived path.
	mcpPath := ResolveMCPOutputPath(workspaceRoot, mcpConfig)
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return fmt.Errorf("factory: mkdir for mcp config: %w", err)
	}
	return os.WriteFile(mcpPath, data, 0o644)
}

// RegistryContents returns the registry contents map (keyed by workflow name).
// Used by the materializer to pass to RenderRootCatalog.
func (r *FactoryRenderer) RegistryContents() map[string]string {
	return r.registryContents
}

// MCPAgentInstructions returns the MCP agent instructions extracted from normalizedMCPs.
// Used by the materializer to pass to RenderRootCatalog.
func (r *FactoryRenderer) MCPAgentInstructions() map[string]string {
	result := make(map[string]string)
	for _, mcp := range r.normalizedMCPs {
		if mcp.AgentInstructions != "" {
			result[mcp.Name] = mcp.AgentInstructions
		}
	}
	return result
}

// InstallWorkflow materializes a workflow into the Factory workspace.
//
// Layout (matches real working .factory installation):
//   - Skills: .factory/skills/<skill-name>/SKILL.md
//   - Orchestrator: .factory/skills/sdd-orchestrator/ORCHESTRATOR.md
//   - _shared: .factory/skills/_shared/
//   - REGISTRY.md: captured for catalog injection (not copied loose)
//   - {SKILLS_PATH}: resolved in installed .md files via resolvePlaceholders()
func (r *FactoryRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	skillsBase := filepath.Join(workspaceRoot, r.def.SkillDir)

	if err := os.MkdirAll(skillsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: workflow mkdir skills: %w", err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: read workflow dir %q: %w", cachePath, err)
	}

	var managedPaths []string

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(cachePath, name)

		if name == "workflow.yaml" {
			continue
		}

		// Skip the registry file — it is captured below, not copied loose.
		if wf.Components.Registry != "" && name == wf.Components.Registry {
			continue
		}

		if skillsSet[name] {
			// Workflow skills: .factory/skills/<skill-name>/SKILL.md
			destDir := filepath.Join(skillsBase, name)
			if err := r.renderSkillInternal(srcPath, destDir, true); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: workflow skill %q: %w", name, err)
			}
			if err := copySkillSubdirs(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: copy skill subdirs for %s: %w", name, err)
			}
			managedPaths = append(managedPaths, destDir)
			continue
		}

		if name == wf.Components.Entrypoint {
			// Orchestrator: .factory/skills/<wf-name>-orchestrator/ORCHESTRATOR.md
			orchDirName := wf.Metadata.Name + "-orchestrator"
			orchDir := filepath.Join(skillsBase, orchDirName)
			if err := os.MkdirAll(orchDir, 0o755); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: mkdir orchestrator: %w", err)
			}
			dstPath := filepath.Join(orchDir, name)
			if err := copySingleFile(srcPath, dstPath, 0o644); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: workflow entrypoint: %w", err)
			}
			managedPaths = append(managedPaths, orchDir)
			continue
		}

		// Copy everything else (e.g. _shared/) as-is under skillsBase.
		dstPath := filepath.Join(skillsBase, name)
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: workflow copy %q: %w", name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// Build shared placeholder replacements: {SKILLS_PATH} only.
	// Factory does not support model routing in the TUI, so {SDD_MODEL_*} placeholders
	// are intentionally left unresolved here and removed entirely by
	// removeModelPlaceholderLines below. This prevents invalid model IDs from being
	// written into the installed ORCHESTRATOR.md, allowing sub-agents to inherit the
	// session's active model instead.
	replacements := buildWorkflowPathReplacements(workspaceRoot, r.def.SkillDir)

	// Capture registry content for catalog injection; apply shared replacements.
	if wf.Components.Registry != "" {
		content, err := captureRegistryContent(cachePath, wf.Components.Registry, replacements)
		if err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: capture registry: %w", err)
		}
		if content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// Resolve placeholders in all installed .md files (covers {SKILLS_PATH}).
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: resolve placeholders: %w", err)
	}

	// Remove any lines containing unresolved {SDD_MODEL_*} placeholders.
	// Factory does not support model routing; sub-agents must inherit the session model.
	if err := removeModelPlaceholderLines(skillsBase); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: remove model placeholder lines: %w", err)
	}

	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// RenderSettings generates .factory/settings.json with commandAllowlist and commandDenylist
// derived from MCP permissions and workflow permissions.
// If agentDef.Settings is nil, settings generation is skipped (returns nil).
func (r *FactoryRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	if r.agentDef.Settings == nil {
		return nil
	}

	settingsPath := filepath.Join(workspaceRoot, "settings.json")

	// Read existing settings.json to merge (read-merge-write pattern).
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Get or create commandAllowlist and commandDenylist.
	allowlist := toStringSlice(existing["commandAllowlist"])
	denylist := toStringSlice(existing["commandDenylist"])

	seenAllow := make(map[string]bool)
	seenDeny := make(map[string]bool)
	for _, v := range allowlist {
		seenAllow[v] = true
	}
	for _, v := range denylist {
		seenDeny[v] = true
	}

	// Add base permissions from agent definition to allowlist.
	for _, p := range r.agentDef.Settings.Permissions {
		if !seenAllow[p] {
			seenAllow[p] = true
			allowlist = append(allowlist, p)
		}
	}

	// Iterate normalizedMCPs: allow → commandAllowlist, deny → commandDenylist, ask is no-op.
	for _, mcp := range r.normalizedMCPs {
		level := mcp.Permissions["level"]
		pattern := fmt.Sprintf("mcp__%s__*", mcp.Name)
		switch level {
		case "allow":
			if !seenAllow[pattern] {
				seenAllow[pattern] = true
				allowlist = append(allowlist, pattern)
			}
		case "deny":
			if !seenDeny[pattern] {
				seenDeny[pattern] = true
				denylist = append(denylist, pattern)
			}
		}
	}

	// Iterate workflow permissions and add to allowlist.
	for _, wf := range workflows {
		for _, p := range wf.Components.Permissions {
			if !seenAllow[p] {
				seenAllow[p] = true
				allowlist = append(allowlist, p)
			}
		}
	}

	// Merge into existing settings map.
	existing["commandAllowlist"] = allowlist
	existing["commandDenylist"] = denylist

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("factory: marshal settings.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("factory: mkdir for settings: %w", err)
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

// toStringSlice converts an interface{} value to []string.
// Accepts []interface{} (from JSON unmarshal) or []string directly.
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ManagedConfigPaths returns the workspace-level config file paths that the Factory
// renderer owns and that the materializer should track for cleanup purposes.
// Uses the same config-derived path as RenderMCPs to prevent drift.
func (r *FactoryRenderer) ManagedConfigPaths(workspaceRoot string) []string {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	return []string{ResolveMCPOutputPath(workspaceRoot, mcpConfig)}
}

// Finalize resolves ${VAR} patterns in the config-derived mcp file using os.Getenv().
func (r *FactoryRenderer) Finalize(workspaceRoot string) error {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	mcpPath := ResolveMCPOutputPath(workspaceRoot, mcpConfig)
	data, err := os.ReadFile(mcpPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("factory: finalize read mcp.json: %w", err)
	}

	resolved := resolveEnvPlaceholders(string(data))
	return os.WriteFile(mcpPath, []byte(resolved), 0o644)
}

// resolveEnvPlaceholders replaces ${VAR} patterns in s with os.Getenv("VAR").
// If the environment variable is not set, the placeholder is left unchanged.
func resolveEnvPlaceholders(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := strings.Index(s[i+2:], "}")
			if end >= 0 {
				varName := s[i+2 : i+2+end]
				val := os.Getenv(varName)
				if val == "" {
					// Keep placeholder if env var not set.
					sb.WriteString(s[i : i+2+end+1])
				} else {
					sb.WriteString(val)
				}
				i += 2 + end + 1
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
