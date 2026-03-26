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

// copilotToolAliases maps canonical tool names to GitHub Copilot tool aliases.
var copilotToolAliases = map[string]string{
	"read":    "read",
	"edit":    "edit",
	"write":   "edit",
	"bash":    "execute",
	"glob":    "search",
	"grep":    "search",
	"search":  "search",
	"execute": "execute",
}

// copilotDropFields lists canonical frontmatter fields not used by Copilot agents.
var copilotDropFields = []string{
	"argument-hint",
	"disable-model-invocation",
	"mode",
	"reasoning-effort",
	"temperature",
	"tools-mode",
}

// copilotSubAgentTools defines role-specific tool sets for Copilot native sub-agents.
// Keys match the WorkflowRole.Skill field values from workflow.yaml.
var copilotSubAgentTools = map[string][]string{
	"sdd-explore":   {"read", "search"},
	"sdd-plan":      {"read", "search", "edit"},
	"sdd-implement": {"read", "edit"},
	"sdd-review":    {"read"},
}

// CopilotRenderer materializes skills for GitHub Copilot custom agents.
//
// Key transformations:
//   - Skills: .github/skills/{name}/SKILL.md (backing skill tree, adviser skills only)
//   - Orchestrator/entrypoint: .github/agents/sdd-orchestrator.agent.md (native agent)
//   - Sub-agents: .github/agents/{role-name}.agent.md (synthesized from skill content)
//   - Tools: converted to Copilot aliases (read, edit, search, execute)
//   - Colon → hyphen in name (e.g. "git:commit" → "git-commit")
//   - MCPs: written to .vscode/mcp.json at project root (VS Code IDE format)
type CopilotRenderer struct {
	def      matypes.AgentPaths
	agentDef model.AgentDefinition
	// Collected MCP definitions for inline injection.
	mcpDefs          map[string]map[string]interface{}
	registryContents map[string]string // keyed by workflow name
	// Normalized MCPs for catalog instruction injection.
	normalizedMCPs []normalizedMCP
}

// NewCopilotRenderer constructs a CopilotRenderer from the given agent definition.
func NewCopilotRenderer(agentDef model.AgentDefinition) *CopilotRenderer {
	return &CopilotRenderer{
		agentDef: agentDef,
		def: matypes.AgentPaths{
			Workspace:   agentDef.Workspace,
			SkillDir:    agentDef.SkillDir,
			AgentDir:    agentDef.AgentDir,
			CommandDir:  agentDef.CommandDir,
			RulesDir:    agentDef.RulesDir,
			CatalogFile: agentDef.CatalogFile,
		},
		mcpDefs:          make(map[string]map[string]interface{}),
		registryContents: make(map[string]string),
	}
}

// Name returns the agent name.
func (r *CopilotRenderer) Name() string { return r.agentDef.Name }

// AgentType returns "copilot".
func (r *CopilotRenderer) AgentType() string { return "copilot" }

// Definition returns the agent definition.
func (r *CopilotRenderer) Definition() model.AgentDefinition { return r.agentDef }

// WorkspacePaths returns the configured workspace paths.
func (r *CopilotRenderer) WorkspacePaths() matypes.AgentPaths { return r.def }

// NeedsCopyMode returns true — Copilot modifies frontmatter and uses a different filename format.
func (r *CopilotRenderer) NeedsCopyMode() bool { return true }

// RenderSkill converts a canonical skill to GitHub Copilot format.
//
// Two modes:
//   - destDir == "": standalone rendering → writes {workspace}/{skillDir}/{name}.agent.md
//   - destDir != "": workflow skill installation → writes {destDir}/SKILL.md
//     (Copilot stores skills as SKILL.md under .github/skills/{name}/ backing tree,
//     and only surfaces the orchestrator as a native .agent.md file in agentDir)
func (r *CopilotRenderer) RenderSkill(canonicalPath string, destDir string) error {
	skillFile, err := resolveSkillFile(canonicalPath)
	if err != nil {
		return fmt.Errorf("copilot: resolve skill %q: %w", canonicalPath, err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("copilot: read %q: %w", skillFile, err)
	}

	fm, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("copilot: parse frontmatter %q: %w", skillFile, err)
	}

	fm = r.transformFrontmatter(fm)

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("copilot: serialize frontmatter: %w", err)
	}

	if destDir != "" {
		// Workflow skill installation: write SKILL.md inside the provided destDir.
		// (Backing skill tree: .github/skills/{name}/SKILL.md)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("copilot: mkdir %q: %w", destDir, err)
		}
		return os.WriteFile(filepath.Join(destDir, "SKILL.md"), out, 0o644)
	}

	// Standalone rendering: write {name}.agent.md into {workspace}/{skillDir}.
	name := colonToHyphen(getStringField(fm, "name"))
	if name == "" {
		name = filepath.Base(canonicalPath)
	}
	outDir := filepath.Join(r.def.Workspace, r.def.SkillDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir %q: %w", outDir, err)
	}
	outPath := filepath.Join(outDir, name+".agent.md")
	return os.WriteFile(outPath, out, 0o644)
}

// transformFrontmatter applies Copilot-specific frontmatter conversions.
func (r *CopilotRenderer) transformFrontmatter(fm map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(fm))
	for k, v := range fm {
		out[k] = v
	}

	// Drop unsupported fields.
	for _, field := range copilotDropFields {
		delete(out, field)
	}

	// Resolve model short name.
	if modelVal, ok := out["model"]; ok {
		if modelStr, ok := modelVal.(string); ok && modelStr != "" {
			out["model"] = resolveModel(modelStr)
		}
	}

	// Convert allowed-tools to Copilot tool aliases.
	if toolsVal, ok := out["allowed-tools"]; ok {
		aliases := convertToolsToAliases(toolsVal)
		if len(aliases) > 0 {
			out["tools"] = aliases
		}
		delete(out, "allowed-tools")
	}

	// Rename colon → hyphen in name.
	if nameVal, ok := out["name"]; ok {
		if nameStr, ok := nameVal.(string); ok {
			out["name"] = colonToHyphen(nameStr)
		}
	}

	return out
}

// convertToolsToAliases converts a canonical tools list to Copilot tool aliases.
// Duplicates are removed; order follows the alias map.
func convertToolsToAliases(toolsVal interface{}) []string {
	toolsList, ok := toolsVal.([]interface{})
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var result []string
	for _, t := range toolsList {
		ts, ok := t.(string)
		if !ok {
			continue
		}
		// Extract base tool name (strip parameters like "Bash(git:*)").
		base := strings.ToLower(ts)
		if idx := strings.Index(base, "("); idx >= 0 {
			base = base[:idx]
		}
		alias, ok := copilotToolAliases[base]
		if !ok {
			alias = base // keep as-is if no alias
		}
		if !seen[alias] {
			seen[alias] = true
			result = append(result, alias)
		}
	}
	return result
}

// RenderCommand writes a Copilot agent command file.
func (r *CopilotRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	fm := map[string]interface{}{
		"name":        colonToHyphen(cmd.Name),
		"description": cmd.Action,
	}
	body := fmt.Sprintf("# %s\n\n%s\n", cmd.Name, cmd.Action)
	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("copilot: render command %q: %w", cmd.Name, err)
	}

	outDir := filepath.Join(r.def.Workspace, r.def.SkillDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir %q: %w", outDir, err)
	}
	dest := filepath.Join(outDir, colonToHyphen(cmd.Name)+".agent.md")
	return os.WriteFile(dest, out, 0o644)
}

// RenderMCPs generates the MCP config file at the config-derived path with the configured format.
// Also normalizes MCPs to extract agentInstructions for catalog injection.
func (r *CopilotRenderer) RenderMCPs(mcps []model.LockedMCP, cacheStore matypes.CacheStore, workspaceRoot string) error {
	normalized, err := normalizeMCPDefinitions(mcps, cacheStore)
	if err != nil {
		return fmt.Errorf("copilot: normalize MCPs: %w", err)
	}
	r.normalizedMCPs = normalized

	// Resolve effective MCP config from the agent definition.
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)

	// Build the raw mcpDefs map (kept for potential future use).
	for _, n := range normalized {
		r.mcpDefs[n.Name] = n.ServerConfig
	}

	// Write MCP config file at the config-derived path.
	// Use applyMCPEnvTransform for env key rename and placeholder style transformation.
	servers := make(map[string]interface{}, len(normalized))
	for _, n := range normalized {
		servers[n.Name] = applyMCPEnvTransform(n.ServerConfig, mcpConfig)
	}
	mcpJSON := map[string]interface{}{
		mcpConfig.RootKey: servers,
	}
	data, err := json.MarshalIndent(mcpJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("copilot: marshal mcp config: %w", err)
	}
	data = append(data, '\n')

	mcpPath := ResolveMCPOutputPath(workspaceRoot, mcpConfig)
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir for mcp config: %w", err)
	}
	return os.WriteFile(mcpPath, data, 0o644)
}

// RenderCatalog writes the copilot-instructions.md guidance file.
// It injects a minimal workflow section (name, description, orchestrator pointer)
// and MCP-derived Memory/Engram instructions into the catalog.
// The full SDD instructions (Evaluation Gate, Commands, Compaction Recovery) are
// NOT included here because they already exist in the sdd-orchestrator.agent.md file.
func (r *CopilotRenderer) RenderCatalog(skills []model.ContentItem, rules []model.ContentItem, workflows []model.WorkflowManifest, destPath string) error {
	var sb strings.Builder

	sb.WriteString("# GitHub Copilot Custom Agent Instructions\n\n")
	sb.WriteString("This file is auto-generated by DevRune. Do not edit manually.\n\n")

	if len(skills) > 0 {
		sb.WriteString("## Available Agents\n\n")
		for _, s := range skills {
			_, _ = fmt.Fprintf(&sb, "- `%s`\n", colonToHyphen(s.Name))
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

			// Emit a minimal orchestrator pointer instead of injecting the full REGISTRY.md.
			// The full orchestrator instructions already exist in sdd-orchestrator.agent.md —
			// repeating them here is redundant.
			orchRoleName := findWorkflowRoleByKind(wf.Components.Roles, "orchestrator")
			if orchRoleName != nil {
				agentDirName := r.def.AgentDir
				if agentDirName == "" {
					agentDirName = r.def.SkillDir
				}
				agentFile := r.def.Workspace + "/" + agentDirName + "/" + orchRoleName.Name + ".agent.md"
				_, _ = fmt.Fprintf(&sb, "Orchestrator: `%s`\n\n", agentFile)
			}
		}
	}

	// Append MCP agent instructions (e.g. Memory/Engram protocol).
	// Matches Claude renderer behavior: ## <CapitalizedName> + instructions body.
	if sections := generateMCPCatalogSections(r.normalizedMCPs); sections != "" {
		sb.WriteString(sections)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir for catalog: %w", err)
	}
	return os.WriteFile(destPath, []byte(sb.String()), 0o644)
}

// InstallWorkflow materializes a workflow into the Copilot workspace.
//
// Layout (Copilot native sub-agent model):
//   - Adviser skills:  .github/skills/<skill-name>/SKILL.md  (non-SDD skills only)
//   - _shared:         .github/skills/_shared/               (shared contracts)
//   - Orchestrator:    .github/agents/sdd-orchestrator.agent.md (with frontmatter + resolved placeholders)
//   - Sub-agents:      .github/agents/{role-name}.agent.md   (synthesized from skill content)
//   - REGISTRY.md:     captured for catalog injection (not copied loose)
//
// T016: Does NOT create agents/REGISTRY.md or agents/_shared/.
// T017: Does NOT install SDD workflow skills to skills/ — they are embedded in .agent.md.
// T018: Generates native sub-agent .agent.md for each subagent role.
// T019: Installs orchestrator with proper frontmatter and resolved {SKILLS_PATH}/{SDD_MODEL_*}.
func (r *CopilotRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	// Build a set of SDD workflow skills that will become sub-agents (not installed to skills/).
	subagentSkillSet := make(map[string]bool)
	for _, role := range wf.Components.Roles {
		if role.Kind == "subagent" && role.Skill != "" {
			subagentSkillSet[role.Skill] = true
		}
	}

	// skillsSet lists ALL skills from the workflow manifest.
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	skillsBase := filepath.Join(workspaceRoot, r.def.SkillDir)
	if err := os.MkdirAll(skillsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow mkdir skills: %w", err)
	}

	// Determine agentDir: use configured agentDir (e.g. "agents"), fall back to skillDir.
	agentDirName := r.def.AgentDir
	if agentDirName == "" {
		agentDirName = r.def.SkillDir
	}
	agentsBase := filepath.Join(workspaceRoot, agentDirName)
	if err := os.MkdirAll(agentsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow mkdir agents: %w", err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: read workflow dir %q: %w", cachePath, err)
	}

	// Build shared placeholder replacements for this workflow.
	replacements := buildWorkflowPlaceholderReplacements(wf, workspaceRoot, r.def.AgentDir, true)
	if r.def.AgentDir == "" {
		replacements = buildWorkflowPlaceholderReplacements(wf, workspaceRoot, r.def.SkillDir, true)
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

		// T019: Install the orchestrator entrypoint as a native .agent.md with frontmatter.
		if name == wf.Components.Entrypoint {
			orchRoleName := wf.Metadata.Name + "-orchestrator"
			if orchRole := findWorkflowRoleByKind(wf.Components.Roles, "orchestrator"); orchRole != nil {
				orchRoleName = orchRole.Name
			}
			dstPath := filepath.Join(agentsBase, orchRoleName+".agent.md")
			if err := r.installOrchestratorAgent(srcPath, dstPath, wf, replacements); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow orchestrator: %w", err)
			}
			managedPaths = append(managedPaths, dstPath)
			continue
		}

		// T017: Skip SDD workflow sub-agent skills — they are embedded in .agent.md files (T018).
		// Only non-subagent skills (e.g. adviser skills) go into skills/.
		if skillsSet[name] {
			if subagentSkillSet[name] {
				// This skill will be embedded as a native sub-agent — skip skills/ installation.
				continue
			}
			// Non-subagent workflow skill: install as SKILL.md backing tree.
			destDir := filepath.Join(skillsBase, name)
			if err := r.RenderSkill(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow skill %q: %w", name, err)
			}
			if err := copySkillSubdirs(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: copy skill subdirs for %s: %w", name, err)
			}
			managedPaths = append(managedPaths, destDir)
			continue
		}

		// Copy everything else (e.g. _shared/) as-is under skillsBase.
		// T016: _shared/ goes to skills/_shared/, NOT agents/_shared/.
		dstPath := filepath.Join(skillsBase, name)
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow copy %q: %w", name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// T018: Generate native sub-agent .agent.md files for each subagent role.
	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" || role.Skill == "" {
			continue
		}
		skillSrcDir := filepath.Join(cachePath, role.Skill)
		dstPath := filepath.Join(agentsBase, role.Name+".agent.md")
		if err := r.generateSubAgentFile(skillSrcDir, dstPath, role, wf, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: generate sub-agent %q: %w", role.Name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// Capture registry content for catalog injection; apply shared placeholder replacements.
	if wf.Components.Registry != "" {
		content, err := captureRegistryContent(cachePath, wf.Components.Registry, replacements)
		if err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: capture registry: %w", err)
		}
		if content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// Resolve placeholders in all installed .md files under skillsBase.
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: resolve placeholders: %w", err)
	}

	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// installOrchestratorAgent installs ORCHESTRATOR.md as a native .agent.md file with
// Copilot-compatible frontmatter. Placeholders are resolved in the body content.
//
// T019: Adds frontmatter (name, description, user-invocable) and resolves {SKILLS_PATH}
// and {SDD_MODEL_*} placeholders. The orchestrator inherits the session model (no model field).
func (r *CopilotRenderer) installOrchestratorAgent(srcPath, dstPath string, wf model.WorkflowManifest, replacements map[string]string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("copilot: read orchestrator %q: %w", srcPath, err)
	}

	content := string(data)

	// Resolve all placeholders in the orchestrator body.
	for placeholder, value := range replacements {
		content = strings.ReplaceAll(content, placeholder, value)
	}

	// Derive the orchestrator role name from the workflow roles.
	orchRoleName := wf.Metadata.Name + "-orchestrator"
	orchDesc := "SDD Orchestrator — coordinates " + wf.Metadata.Name + " workflow via sub-agents"
	if orchRole := findWorkflowRoleByKind(wf.Components.Roles, "orchestrator"); orchRole != nil {
		orchRoleName = orchRole.Name
	}

	// Build frontmatter for the orchestrator agent.
	// Orchestrator: no model field (inherits session), no disable-model-invocation.
	fm := map[string]interface{}{
		"name":           orchRoleName,
		"description":    orchDesc,
		"user-invocable": true,
	}

	out, err := parse.SerializeFrontmatter(fm, content)
	if err != nil {
		return fmt.Errorf("copilot: serialize orchestrator frontmatter: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir orchestrator agent: %w", err)
	}
	return os.WriteFile(dstPath, out, 0o644)
}

// generateSubAgentFile creates a native Copilot .agent.md for a workflow subagent role.
// The file has role-specific frontmatter (name, description, tools, model,
// user-invocable: false, disable-model-invocation: false) and embeds the full
// skill body (without frontmatter) as the agent instructions.
//
// T018: Role tool sets follow the copilotSubAgentTools mapping. Model is resolved
// from the role.Model field via resolveModel().
func (r *CopilotRenderer) generateSubAgentFile(skillSrcDir, dstPath string, role model.WorkflowRole, wf model.WorkflowManifest, replacements map[string]string) error {
	// Read the skill's SKILL.md to extract its body content.
	skillFile := filepath.Join(skillSrcDir, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("copilot: read skill %q for sub-agent: %w", skillFile, err)
	}

	_, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("copilot: parse skill frontmatter %q: %w", skillFile, err)
	}

	// Resolve placeholders in the body.
	bodyContent := string(body)
	for placeholder, value := range replacements {
		bodyContent = strings.ReplaceAll(bodyContent, placeholder, value)
	}

	// Determine tool set for this role based on the skill name.
	tools := copilotSubAgentTools[role.Skill]
	if len(tools) == 0 {
		// Fallback: default to read-only if skill is not in the known mapping.
		tools = []string{"read"}
	}

	// Build frontmatter: sub-agents are not user-invocable but CAN be invoked
	// by the orchestrator as tools (disable-model-invocation: false).
	fm := map[string]interface{}{
		"name":                     role.Name,
		"description":              skillDescriptionForRole(role.Skill),
		"tools":                    tools,
		"user-invocable":           false,
		"disable-model-invocation": false,
	}

	// Add model if the role specifies one.
	if role.Model != "" {
		fm["model"] = resolveModel(role.Model)
	}

	out, err := parse.SerializeFrontmatter(fm, bodyContent)
	if err != nil {
		return fmt.Errorf("copilot: serialize sub-agent frontmatter for %q: %w", role.Name, err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir sub-agent: %w", err)
	}
	return os.WriteFile(dstPath, out, 0o644)
}

// skillDescriptionForRole returns a human-readable description for a known SDD skill name.
func skillDescriptionForRole(skill string) string {
	switch skill {
	case "sdd-explore":
		return "SDD Explore sub-agent"
	case "sdd-plan":
		return "SDD Plan sub-agent"
	case "sdd-implement":
		return "SDD Implement sub-agent"
	case "sdd-review":
		return "SDD Review sub-agent"
	default:
		return skill + " sub-agent"
	}
}

// Finalize is a no-op for Copilot.
// RenderSettings is a no-op for Copilot (no settings file concept).
func (r *CopilotRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	return nil
}

func (r *CopilotRenderer) Finalize(workspaceRoot string) error { return nil }

// ManagedConfigPaths returns the workspace-level config file paths that the Copilot
// renderer owns and that the materializer should track for cleanup purposes.
// Uses the same config-derived path as RenderMCPs to prevent drift.
func (r *CopilotRenderer) ManagedConfigPaths(workspaceRoot string) []string {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	return []string{ResolveMCPOutputPath(workspaceRoot, mcpConfig)}
}
