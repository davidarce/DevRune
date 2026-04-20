// SPDX-License-Identifier: MIT

package renderers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	"sdd-explore":   {"read", "search", "edit", "execute"},
	"sdd-plan":      {"read", "search", "edit", "execute"},
	"sdd-implement": {"read", "search", "edit", "execute"},
	"sdd-review":    {"read", "search", "execute"},
	// Adviser roles — read and search only; they analyse the plan and their SKILL.md.
	"architect-adviser":         {"read", "search"},
	"api-first-adviser":         {"read", "search"},
	"unit-test-adviser":         {"read", "search"},
	"integration-test-adviser":  {"read", "search"},
	"component-adviser":         {"read", "search"},
	"frontend-test-adviser":     {"read", "search"},
	"web-accessibility-adviser": {"read", "search"},
}

// copilotBodyReplacements maps Claude Code MCP shorthand tool calls to Copilot
// mcp__<server>__<tool>( format. Keys include the opening paren to avoid false
// positives (e.g. "mem_save_prompt(" won't be matched by the "mem_save(" key).
// Order of iteration is irrelevant since keys are disjoint prefixes.
var copilotBodyReplacements = map[string]string{
	"mem_save(":              "mcp__engram__mem_save(",
	"mem_search(":            "mcp__engram__mem_search(",
	"mem_context(":           "mcp__engram__mem_context(",
	"mem_session_summary(":   "mcp__engram__mem_session_summary(",
	"mem_get_observation(":   "mcp__engram__mem_get_observation(",
	"mem_timeline(":          "mcp__engram__mem_timeline(",
	"mem_save_prompt(":       "mcp__engram__mem_save_prompt(",
	"mem_stats(":             "mcp__engram__mem_stats(",
	"mem_update(":            "mcp__engram__mem_update(",
	"mem_delete(":            "mcp__engram__mem_delete(",
	"mem_suggest_topic_key(": "mcp__engram__mem_suggest_topic_key(",
	"mem_capture_passive(":   "mcp__engram__mem_capture_passive(",
	"mem_session_start(":     "mcp__engram__mem_session_start(",
	"mem_session_end(":       "mcp__engram__mem_session_end(",
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
	modelOverrides map[string]string // role-name → model-value from TUI SDD model selection
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

// SetModelOverrides stores per-role model overrides selected in the TUI SDD model
// selection step. When set, these override the role.Model values from workflow.yaml
// when building {SDD_MODEL_*} placeholder replacements.
func (r *CopilotRenderer) SetModelOverrides(overrides map[string]string) {
	r.modelOverrides = overrides
}

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

	// Resolve model short name. Use identity resolver — Copilot frontmatter requires bare IDs
	// like "claude-sonnet-4.6", not the "anthropic/..." format that resolveModel() produces.
	if modelVal, ok := out["model"]; ok {
		if modelStr, ok := modelVal.(string); ok && modelStr != "" {
			out["model"] = copilotModelResolver(modelStr)
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

// RegistryContents returns the registry contents map (keyed by workflow name).
// Used by the materializer to pass to RenderRootCatalog.
func (r *CopilotRenderer) RegistryContents() map[string]string {
	return r.registryContents
}

// MCPAgentInstructions returns the MCP agent instructions extracted from normalizedMCPs.
// Used by the materializer to pass to RenderRootCatalog.
func (r *CopilotRenderer) MCPAgentInstructions() map[string]string {
	result := make(map[string]string)
	for _, mcp := range r.normalizedMCPs {
		if mcp.AgentInstructions != "" {
			result[mcp.Name] = mcp.AgentInstructions
		}
	}
	return result
}

// copilotModelResolver is a no-op model resolver for the Copilot path.
// Copilot .agent.md frontmatter uses bare model IDs ("claude-sonnet-4.6"),
// NOT the "anthropic/claude-sonnet-4-20250514" format produced by resolveModel().
// The TUI stores bare IDs directly for Copilot, so no transformation is needed.
func copilotModelResolver(id string) string { return id }

// InstallWorkflow materializes a workflow into the Copilot workspace.
//
// Layout (Copilot native sub-agent model):
//   - Adviser skills:  .github/skills/<skill-name>/SKILL.md  (non-SDD skills only)
//   - _shared:         .github/skills/sdd-orchestrator/_shared/ (shared contracts)
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

	// Resolve the orchestrator role name upfront so non-skill auxiliary files
	// (e.g. _shared/) land under skillsBase/<orchRoleName>/ rather than in agentsBase.
	// In Copilot's model, agents/ contains only flat .agent.md files; skills/ holds
	// content, resources, and references (including _shared/ templates).
	// This matches the paths the orchestrator .agent.md references (e.g.
	// .github/skills/sdd-orchestrator/_shared/launch-templates.md).
	orchRoleName := wf.Metadata.EffectiveWorkingDir()
	if orchRole := findWorkflowRoleByKind(wf.Components.Roles, "orchestrator"); orchRole != nil {
		orchRoleName = orchRole.Name
	}
	orchSkillDir := filepath.Join(skillsBase, orchRoleName)

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: read workflow dir %q: %w", cachePath, err)
	}

	// Build shared placeholder replacements for this workflow including model routing.
	// copilotModelResolver is an identity pass-through: Copilot .agent.md frontmatter
	// requires bare IDs like "claude-sonnet-4.6", NOT the "anthropic/..." format that
	// resolveModel() would produce. TUI-selected model overrides (r.modelOverrides) contain
	// bare IDs stored directly from CopilotModelOptions() values — no transformation needed.
	//
	// {SKILLS_PATH} resolves to the skills/ directory (not agents/) because in Copilot's
	// model, agents/ contains only flat .agent.md files while content (_shared/, templates,
	// references) lives under skills/. The orchestrator .agent.md references resources via
	// paths like .github/skills/sdd-orchestrator/_shared/launch-templates.md.
	skillDirName := r.def.SkillDir
	replacements := buildWorkflowPlaceholderReplacements(
		wf, workspaceRoot, skillDirName,
		copilotModelResolver, // identity: bare ID passes through unchanged to .agent.md frontmatter
		                      // DO NOT use resolveModel here — that produces "anthropic/..." format,
		                      // which is invalid for Copilot .agent.md frontmatter.
		r.modelOverrides,     // TUI-selected per-role bare model IDs from CopilotModelOptions()
		nil,                  // no custom subagent resolver; Copilot uses native @agent-name
	)
	// Override subagent placeholders to use role names (Copilot has native .agent.md agents).
	// buildWorkflowPlaceholderReplacements with subagentResolver=nil defaults subagent entries
	// to "general"; this loop overwrites them with native Copilot agent names (e.g. "sdd-explorer").
	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" {
			continue
		}
		key := model.PlaceholderKeyFromRole(wf.Metadata.Name, role.Name, role.Placeholder)
		replacements["{WORKFLOW_SUBAGENT_"+key+"}"] = role.Name
	}

	var managedPaths []string

	// Resolve agent-specific orchestrator variant if present.
	// ORCHESTRATOR.copilot.md takes precedence over the generic ORCHESTRATOR.md.
	const variantEntrypointName = "ORCHESTRATOR.copilot.md"
	variantOrchPath := ""
	if _, statErr := os.Stat(filepath.Join(cachePath, variantEntrypointName)); statErr == nil {
		variantOrchPath = filepath.Join(cachePath, variantEntrypointName)
	}

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
		// Each renderer uses only its own variant (resolved in the pre-loop probe above).
		if orchestratorVariantNames[name] {
			continue
		}

		// T019: Install the orchestrator entrypoint as a native .agent.md with frontmatter.
		if name == wf.Components.Entrypoint {
			effectiveSrc := srcPath
			if variantOrchPath != "" {
				effectiveSrc = variantOrchPath // use variant if found
			}
			dstPath := filepath.Join(agentsBase, orchRoleName+".agent.md")
			if err := r.installOrchestratorAgent(effectiveSrc, dstPath, wf, replacements); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow orchestrator: %w", err)
			}
			managedPaths = append(managedPaths, dstPath)
			continue
		}

		// T017: Skip SDD workflow sub-agent skills — they are embedded in .agent.md files (T018).
		// Also skip the orchestrator skill directory: its content is already in the .agent.md
		// (installed by installOrchestratorAgent above). Rendering it would create a SKILL.md
		// with Claude Code Task()/Skill() syntax that is invalid for Copilot.
		// Only non-subagent, non-orchestrator skills (e.g. adviser skills) go into skills/.
		if skillsSet[name] {
			if subagentSkillSet[name] || name == orchRoleName {
				// Subagent skills are embedded in .agent.md; orchestrator is already installed.
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

		// Skip hook/plugin asset directories — only copied by the agent that declares them.
		if hookAssetDirNames[name] {
			continue
		}

		// Copy everything else (e.g. _shared/) under skillsBase/<orchRoleName>/
		// so that the orchestrator .agent.md can reference them via paths such as
		// .github/skills/sdd-orchestrator/_shared/launch-templates.md.
		// In Copilot's model, agents/ is flat (.agent.md files only); content lives in skills/.
		dstPath := filepath.Join(orchSkillDir, name)
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: workflow copy %q: %w", name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// T018: Generate native sub-agent .agent.md files for each subagent role.
	// Skip sentinel roles (e.g. sdd-adviser with skill "*-adviser") — these exist
	// only for model placeholder generation, not for .agent.md synthesis.
	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" || role.Skill == "" || strings.Contains(role.Skill, "*") {
			continue
		}
		skillSrcDir := filepath.Join(cachePath, role.Skill)
		dstPath := filepath.Join(agentsBase, role.Name+".agent.md")
		if err := r.generateSubAgentFile(skillSrcDir, dstPath, role, wf, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: generate sub-agent %q: %w", role.Name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// Generate lightweight .agent.md wrappers for adviser skills installed in skillsBase.
	// Advisers are not workflow roles but need to be invocable as @agent-name in
	// Copilot's guidance loop. The wrapper references the SKILL.md in skills/ rather
	// than embedding the full content (avoiding duplication).
	if adviserEntries, err := os.ReadDir(skillsBase); err == nil {
		for _, entry := range adviserEntries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), "-adviser") {
				continue
			}
			adviserName := entry.Name()
			agentPath := filepath.Join(agentsBase, adviserName+".agent.md")
			// Skip if already generated as a workflow role.
			if _, err := os.Stat(agentPath); err == nil {
				continue
			}
			description := skillDescriptionForRole(adviserName)
			if description == "" {
				description = adviserName + " specialist adviser"
			}
			tools := copilotSubAgentTools[adviserName]
			if len(tools) == 0 {
				tools = []string{"read"}
			}
			fm := map[string]interface{}{
				"name":                     adviserName,
				"description":              description,
				"tools":                    tools,
				"user-invocable":           false,
				"disable-model-invocation": false,
			}
			// Add model: check for an exact override first, then fall back to the SDD adviser sentinel.
			// sdd-adviser (skill: "*-adviser") is the sentinel that represents all advisers in the SDD
			// workflow, so its model override applies to any adviser wrapper without its own override.
			adviserModel := r.modelOverrides[adviserName]
			if adviserModel == "" || adviserModel == model.ModelInheritOption {
				adviserModel = r.modelOverrides["sdd-adviser"]
			}
			if adviserModel != "" && adviserModel != model.ModelInheritOption {
				fm["model"] = adviserModel
			}
			// Use workspace-relative path (not absolute) so the .agent.md is portable.
			skillRelPath := filepath.Join(workspaceRoot, r.def.SkillDir, adviserName, "SKILL.md")
			body := fmt.Sprintf("You are a specialist adviser. Read your skill file at `%s` and follow its instructions.\n", skillRelPath)
			out, err := parse.SerializeFrontmatter(fm, body)
			if err != nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(agentPath), 0o755); err != nil {
				continue
			}
			if err := os.WriteFile(agentPath, out, 0o644); err != nil {
				continue
			}
			managedPaths = append(managedPaths, agentPath)
		}
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

	// Resolve placeholders in all installed .md files under skillsBase and agentsBase.
	// agentsBase now contains the orchestrator .agent.md and the _shared/ directory.
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: resolve placeholders (skills): %w", err)
	}
	if err := resolvePlaceholders(agentsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: resolve placeholders (agents): %w", err)
	}

	// Remove any lines containing unresolved {SDD_MODEL_*} or {WORKFLOW_MODEL_*} placeholders.
	// These correspond to phases where the user selected "inherit from session" (no override).
	// Lines with resolved model values (e.g. "model: claude-sonnet-4.6") are untouched.
	// Apply to both skillsBase and agentsBase since agent files are written to agentsBase.
	if err := removeModelPlaceholderLines(skillsBase); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: remove model placeholder lines (skills): %w", err)
	}
	if err := removeModelPlaceholderLines(agentsBase); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("copilot: remove model placeholder lines (agents): %w", err)
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
	content = transformBodyForCopilot(content)

	// Derive the orchestrator role name from the workflow roles.
	orchRoleName := wf.Metadata.EffectiveWorkingDir()
	orchDesc := "SDD Orchestrator — coordinates " + wf.Metadata.EffectiveDisplayName() + " workflow via sub-agents"
	if orchRole := findWorkflowRoleByKind(wf.Components.Roles, "orchestrator"); orchRole != nil {
		orchRoleName = orchRole.Name
	}

	// Build frontmatter for the orchestrator agent.
	fm := map[string]interface{}{
		"name":           orchRoleName,
		"description":    orchDesc,
		"user-invocable": true,
	}
	if orchModel := r.modelOverrides["sdd-orchestrator"]; orchModel != "" && orchModel != model.ModelInheritOption {
		fm["model"] = orchModel
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
// T018: Role tool sets follow the copilotSubAgentTools mapping. Model is looked up
// from the replacements map (pre-resolved by copilotModelResolver — bare ID passthrough).
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
	bodyContent = transformBodyForCopilot(bodyContent)

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

	// Add model: prefer TUI-selected override from replacements (pre-resolved via copilotModelResolver),
	// fall back to bare role.Model. Never use resolveModel() — that produces "anthropic/..." format
	// which is invalid for Copilot .agent.md frontmatter.
	placeholderKey := "{WORKFLOW_MODEL_" + model.PlaceholderKeyFromRole(wf.Metadata.Name, role.Name, role.Placeholder) + "}"
	if modelVal, ok := replacements[placeholderKey]; ok && modelVal != "" {
		fm["model"] = modelVal
	} else if role.Model != "" {
		fm["model"] = copilotModelResolver(role.Model)
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
	case "architect-adviser":
		return "Clean architecture adviser: hexagonal, DDD, ports and adapters"
	case "api-first-adviser":
		return "API-first design adviser: OpenAPI, REST conventions, error models"
	case "unit-test-adviser":
		return "Unit test adviser: test structure, mocking, Given-When-Then"
	case "integration-test-adviser":
		return "Integration test adviser: adapter testing, external service mocking"
	case "component-adviser":
		return "React component adviser: composition, hooks, state management"
	case "frontend-test-adviser":
		return "Frontend test adviser: React Testing Library, Vitest, Cypress"
	case "web-accessibility-adviser":
		return "Web accessibility adviser: WCAG 2.1 AA, ARIA, keyboard navigation"
	default:
		return skill + " sub-agent"
	}
}

// RenderSettings writes .vscode/settings.json with Copilot MCP tool auto-approve entries.
// Only MCPs with permission level "allow" are added; "ask" and "deny" are no-ops for Copilot.
// Existing VS Code settings are preserved (read-merge-write).
func (r *CopilotRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	if r.agentDef.Settings == nil {
		return nil
	}

	// .vscode/settings.json lives at the project root (one level up from .github).
	vscodeDir := filepath.Join(workspaceRoot, "..", ".vscode")
	settingsPath := filepath.Join(vscodeDir, "settings.json")

	// Read existing settings if present (preserve other VS Code settings).
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("copilot: parse .vscode/settings.json: %w", err)
		}
	}

	// Get or create the autoApprove array.
	const autoApproveKey = "github.copilot.chat.tools.autoApprove"
	var autoApprove []string
	if raw, ok := existing[autoApproveKey]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					autoApprove = append(autoApprove, s)
				}
			}
		}
	}

	// Add MCP allow-level permissions.
	for _, mcp := range r.normalizedMCPs {
		level, ok := mcp.Permissions["level"]
		if !ok {
			continue
		}
		if level != "allow" {
			// ask/deny are no-ops for Copilot.
			continue
		}
		pattern := fmt.Sprintf("mcp__%s__*", mcp.Name)
		autoApprove = append(autoApprove, pattern)
	}

	// Deduplicate while preserving order.
	seen := make(map[string]bool, len(autoApprove))
	deduped := autoApprove[:0]
	for _, p := range autoApprove {
		if !seen[p] {
			seen[p] = true
			deduped = append(deduped, p)
		}
	}

	existing[autoApproveKey] = deduped

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("copilot: marshal .vscode/settings.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		return fmt.Errorf("copilot: mkdir .vscode: %w", err)
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

func (r *CopilotRenderer) Finalize(workspaceRoot string) error { return nil }

// SettingsManagedPaths returns the settings file path that RenderSettings writes for Copilot.
// Copilot writes .vscode/settings.json at the project root (one level up from .github).
// Used by the materializer to track the file for cleanup on reinstall.
func (r *CopilotRenderer) SettingsManagedPaths(workspaceRoot string) []string {
	if r.agentDef.Settings == nil {
		return nil
	}
	return []string{filepath.Join(workspaceRoot, "..", ".vscode", "settings.json")}
}

// ManagedConfigPaths returns the workspace-level config file paths that the Copilot
// renderer owns and that the materializer should track for cleanup purposes.
// Uses the same config-derived path as RenderMCPs to prevent drift.
func (r *CopilotRenderer) ManagedConfigPaths(workspaceRoot string) []string {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	return []string{ResolveMCPOutputPath(workspaceRoot, mcpConfig)}
}

// transformBodyForCopilot applies Claude Code → Copilot body text transformations.
// It converts tool references, MCP shorthand, shebang lines, and Skill/Task blocks
// to their Copilot-native equivalents. Safe to call on any markdown body.
//
// Transformations applied in order:
//  1. Shebang conversion: lines matching "- **Label**: !`cmd`" → "Pre-flight Commands" section
//  2. MCP normalization: mem_save( → mcp__engram__mem_save( etc. (via copilotBodyReplacements)
//  3. AskUserQuestion replacement: → prose equivalent
//  4. Skill()/Task() block stripping: → @agent-name prose instructions
//  5. Tool name replacement: backtick-wrapped Write/Edit/Read/Glob/Grep/Bash → prose
func transformBodyForCopilot(body string) string {
	body = transformCopilotShebangs(body)
	body = transformCopilotMCPCalls(body)
	body = transformCopilotAskUserQuestion(body)
	body = transformCopilotSkillTaskBlocks(body)
	body = transformCopilotToolNames(body)
	return body
}

// reCopilotShebang matches Dynamic Context shebang lines of the form:
//
//   - **Label**: !`command`
var reCopilotShebang = regexp.MustCompile("(?m)^(- \\*\\*[^*]+\\*\\*): !`(.+)`$")

// transformCopilotShebangs converts Dynamic Context shebang lines to a
// "Pre-flight Commands" prose section that Copilot can act on.
func transformCopilotShebangs(body string) string {
	matches := reCopilotShebang.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body
	}

	// Collect all shebang entries (label + command) and track their span.
	type shebangEntry struct {
		label   string
		command string
		start   int
		end     int
	}
	var entries []shebangEntry
	for _, m := range matches {
		entries = append(entries, shebangEntry{
			label:   body[m[2]:m[3]],
			command: body[m[4]:m[5]],
			start:   m[0],
			end:     m[1],
		})
	}

	// Build the replacement "Pre-flight Commands" section from all matched entries.
	var sb strings.Builder
	sb.WriteString("## Pre-flight Commands\n\n")
	sb.WriteString("Before proceeding, execute these commands and include their output in your context:\n\n")
	for _, e := range entries {
		sb.WriteString("- ")
		sb.WriteString(e.label)
		sb.WriteString(": run `")
		sb.WriteString(e.command)
		sb.WriteString("`\n")
	}

	// Replace each matched line with an empty string first, then prepend the section.
	// Work backwards through entries to preserve indices.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		body = body[:e.start] + body[e.end:]
	}

	// Insert the pre-flight section before the first non-empty line.
	idx := strings.Index(body, "\n")
	if idx < 0 {
		return sb.String() + "\n" + body
	}
	return body[:idx+1] + sb.String() + "\n" + strings.TrimLeft(body[idx+1:], "\n")
}

// transformCopilotMCPCalls replaces Claude Code MCP shorthand calls (mem_save( etc.)
// with Copilot mcp__engram__* equivalents using the copilotBodyReplacements map.
// It is idempotent: already-prefixed occurrences (e.g. mcp__engram__mem_save() are
// not double-prefixed.
func transformCopilotMCPCalls(body string) string {
	const sentinel = "\x00ENGRAM\x00"
	for old, newVal := range copilotBodyReplacements {
		// Guard against double-prefixing: temporarily replace already-transformed
		// occurrences with a sentinel, do the plain substitution, then restore.
		body = strings.ReplaceAll(body, newVal, sentinel)
		body = strings.ReplaceAll(body, old, newVal)
		body = strings.ReplaceAll(body, sentinel, newVal)
	}
	return body
}

// transformCopilotAskUserQuestion replaces AskUserQuestion references with prose.
func transformCopilotAskUserQuestion(body string) string {
	// Replace backtick-wrapped variant first (more specific).
	body = strings.ReplaceAll(body, "`AskUserQuestion`", "ask the user directly")
	// Replace bare variant.
	body = strings.ReplaceAll(body, "AskUserQuestion", "ask the user directly (no special tool needed)")
	return body
}

// reSkillInline matches inline Skill("name") or Skill(skill: "name") references.
var reSkillInline = regexp.MustCompile(`Skill\((?:skill:\s*)?"([^"]+)"\)`)

// transformCopilotSkillTaskBlocks replaces Skill() references with @agent-name prose,
// and strips multi-line Task(...) blocks with a simplified prose instruction.
func transformCopilotSkillTaskBlocks(body string) string {
	// Replace inline Skill() references.
	body = reSkillInline.ReplaceAllStringFunc(body, func(match string) string {
		sub := reSkillInline.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		return "@" + sub[1] + " agent"
	})

	// Strip multi-line Task(...) blocks using a line-by-line state machine
	// that tracks paren depth to find the closing paren.
	body = stripTaskBlocks(body)
	return body
}

// stripTaskBlocks removes multi-line Task(...) call blocks from body text,
// replacing each with a prose instruction. Uses a paren-depth state machine
// to handle nested parens robustly.
func stripTaskBlocks(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	depth := 0
	inTask := false

	for _, line := range lines {
		if !inTask {
			// Detect start of a Task( block (not inside a code block, just plain text).
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Task(") || strings.Contains(line, " Task(") {
				inTask = true
				depth = strings.Count(line, "(") - strings.Count(line, ")")
				if depth <= 0 {
					// Single-line Task() — replace inline.
					out = append(out, "Invoke the appropriate sub-agent with the context above.")
					inTask = false
					depth = 0
				}
				// Multi-line: consume the opening line, emit replacement on close.
				continue
			}
			out = append(out, line)
		} else {
			// Inside a Task block: track paren depth.
			depth += strings.Count(line, "(") - strings.Count(line, ")")
			if depth <= 0 {
				// Closing line reached — emit replacement prose.
				out = append(out, "Invoke the appropriate sub-agent with the context above.")
				inTask = false
				depth = 0
			}
			// Consume the line (don't emit).
		}
	}

	// If we hit end-of-body while still in a Task block, emit replacement.
	if inTask {
		out = append(out, "Invoke the appropriate sub-agent with the context above.")
	}

	return strings.Join(out, "\n")
}

// copilotToolNameReplacements maps backtick-wrapped Claude Code tool names to
// Copilot-friendly prose equivalents. Only exact backtick-wrapped names are replaced
// to avoid matching prose words like "Read the file" or "Edit the plan".
var copilotToolNameReplacements = [][2]string{
	{"`Write`", "the file write tool"},
	{"`Edit`", "the file edit tool"},
	{"`Read`", "the file read tool"},
	{"`Glob`", "the file search tool"},
	{"`Grep`", "the content search tool"},
	{"`Bash`", "the terminal tool"},
}

// reBacktickToolWithArgs matches backtick-wrapped tool names that include parameters,
// e.g. `Bash(git:*)` or `Write(path, content)`.
var reBacktickToolWithArgs = regexp.MustCompile("`(Write|Edit|Read|Glob|Grep|Bash)\\([^`]*\\)`")

// transformCopilotToolNames replaces backtick-wrapped tool name references with
// Copilot-native prose equivalents. Only exact matches are replaced to avoid
// mangling prose sentences containing these words.
func transformCopilotToolNames(body string) string {
	// Replace parameterized forms first (e.g. `Bash(git:*)`) before the plain form.
	body = reBacktickToolWithArgs.ReplaceAllStringFunc(body, func(match string) string {
		sub := reBacktickToolWithArgs.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		switch sub[1] {
		case "Write":
			return "the file write tool"
		case "Edit":
			return "the file edit tool"
		case "Read":
			return "the file read tool"
		case "Glob":
			return "the file search tool"
		case "Grep":
			return "the content search tool"
		case "Bash":
			return "the terminal tool"
		}
		return match
	})
	// Replace exact backtick-wrapped names.
	for _, pair := range copilotToolNameReplacements {
		body = strings.ReplaceAll(body, pair[0], pair[1])
	}
	return body
}

// stripEmptyPhaseToModelTable removes the "## Phase-to-Model Table" section when
// it contains only the header row and no data rows. This avoids rendering an empty
// table in agents that don't support per-phase model routing (e.g. Copilot).
func stripEmptyPhaseToModelTable(content string) string {
	const marker = "## Phase-to-Model Table"
	idx := strings.Index(content, marker)
	if idx < 0 {
		return content
	}
	// Find the end of the section: next "## " heading or end of string.
	rest := content[idx+len(marker):]
	endIdx := strings.Index(rest, "\n## ")
	if endIdx < 0 {
		endIdx = len(rest)
	}
	section := rest[:endIdx]
	// Count non-empty, non-header lines in the table. The table has:
	//   | Phase | Skill | Model |
	//   |-------|-------|-------|
	// If there are no data rows after the separator, strip the whole section.
	lines := strings.Split(section, "\n")
	dataRows := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "| Phase") || strings.HasPrefix(trimmed, "|---") {
			continue
		}
		if strings.HasPrefix(trimmed, "|") {
			dataRows++
		}
	}
	if dataRows > 0 {
		return content // Table has data, keep it.
	}
	// Strip the section including trailing newlines.
	return content[:idx] + strings.TrimLeft(rest[endIdx:], "\n")
}
