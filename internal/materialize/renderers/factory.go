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

// RenderCatalog writes the AGENTS.md guidance file for Factory.
// It injects captured workflow registry content (e.g. REGISTRY.md) into
// the catalog instead of copying REGISTRY.md as a loose file.
// It also appends MCP agent instructions (e.g. Memory/Engram guidance) to match
// the Claude catalog format.
func (r *FactoryRenderer) RenderCatalog(skills []model.ContentItem, rules []model.ContentItem, workflows []model.WorkflowManifest, destPath string) error {
	var sb strings.Builder

	sb.WriteString("# Factory AI Agent Catalog\n\n")
	sb.WriteString("This file is auto-generated by DevRune. Do not edit manually.\n\n")

	if len(skills) > 0 {
		sb.WriteString("## Skills\n\n")
		for _, s := range skills {
			_, _ = fmt.Fprintf(&sb, "- `%s`\n", s.Name)
		}
		sb.WriteString("\n")
	}

	if len(workflows) > 0 {
		sb.WriteString("## Workflows\n\n")
		for _, wf := range workflows {
			_, _ = fmt.Fprintf(&sb, "### %s\n\n", wf.Metadata.Name)
			if wf.Metadata.Description != "" {
				sb.WriteString(wf.Metadata.Description + "\n\n")
			}
			if len(wf.Components.Commands) > 0 {
				sb.WriteString("| Command | Action |\n")
				sb.WriteString("|---------|--------|\n")
				for _, cmd := range wf.Components.Commands {
					_, _ = fmt.Fprintf(&sb, "| `%s` | %s |\n", cmd.Name, cmd.Action)
				}
				sb.WriteString("\n")
			}

			// Inject captured registry content instead of copying REGISTRY.md loose.
			if content, ok := r.registryContents[wf.Metadata.Name]; ok && content != "" {
				sb.WriteString(content)
				if !strings.HasSuffix(content, "\n") {
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
		}
	}

	// Append MCP agent instructions (e.g. Memory/Engram guidance) to match Claude catalog format.
	if memSection := generateMCPCatalogSections(r.normalizedMCPs); memSection != "" {
		sb.WriteString(memSection)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("factory: mkdir for catalog: %w", err)
	}
	return os.WriteFile(destPath, []byte(sb.String()), 0o644)
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

	// Build shared placeholder replacements: {SKILLS_PATH} and {SDD_MODEL_*}.
	// Uses buildWorkflowPlaceholderReplacements to avoid double-slash bugs and
	// to ensure {SDD_MODEL_*} markers are resolved from workflow role metadata.
	replacements := buildWorkflowPlaceholderReplacements(wf, workspaceRoot, r.def.SkillDir, true)

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

	// Resolve placeholders in all installed .md files (covers {SKILLS_PATH} and {SDD_MODEL_*}).
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("factory: resolve placeholders: %w", err)
	}

	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// RenderSettings is a no-op for Factory (no settings file concept).
func (r *FactoryRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
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
