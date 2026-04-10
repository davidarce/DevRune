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

// openCodeDropFields lists canonical frontmatter fields that OpenCode does not support.
var openCodeDropFields = []string{
	"allowed-tools",
	"argument-hint",
	"disable-model-invocation",
}

// openCodeAgentTools is the standard tool set for all synthesized OpenCode SDD agents.
var openCodeAgentTools = map[string]bool{
	"bash":  true,
	"glob":  true,
	"grep":  true,
	"read":  true,
	"task":  true,
	"write": true,
}

// OpenCodeRenderer materializes skills and config for OpenCode.
//
// Key transformations applied to canonical frontmatter:
//   - Drops: allowed-tools, argument-hint, disable-model-invocation
//   - Resolves: model short name → full anthropic ID (e.g. "sonnet" → "anthropic/claude-sonnet-4-20250514")
//   - Converts: tools list → boolean map { "write": true, "read": true, ... }
//   - Injects: mode: subagent
//   - Renames: colon → hyphen in skill name (e.g. "git:commit" → "git-commit")
//
// Output location: .opencode/skills/{name}/SKILL.md
type OpenCodeRenderer struct {
	def              matypes.AgentPaths
	agentDef         model.AgentDefinition
	registryContents map[string]string // keyed by workflow name
	normalizedMCPs   []normalizedMCP   // MCP entries with agentInstructions for catalog injection
	modelOverrides   map[string]string // role-name → model-value from TUI SDD model selection
}

// NewOpenCodeRenderer constructs an OpenCodeRenderer from the given agent definition.
func NewOpenCodeRenderer(agentDef model.AgentDefinition) *OpenCodeRenderer {
	return &OpenCodeRenderer{
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
func (r *OpenCodeRenderer) Name() string { return r.agentDef.Name }

// AgentType returns "opencode".
func (r *OpenCodeRenderer) AgentType() string { return "opencode" }

// SetModelOverrides stores per-role model overrides selected in the TUI SDD model
// selection step. When set, these override the role.Model values from workflow.yaml
// when building {SDD_MODEL_*} placeholder replacements.
func (r *OpenCodeRenderer) SetModelOverrides(overrides map[string]string) {
	r.modelOverrides = overrides
}

// Definition returns the agent definition.
func (r *OpenCodeRenderer) Definition() model.AgentDefinition { return r.agentDef }

// WorkspacePaths returns the configured workspace paths.
func (r *OpenCodeRenderer) WorkspacePaths() matypes.AgentPaths { return r.def }

// NeedsCopyMode returns true — OpenCode modifies frontmatter significantly.
func (r *OpenCodeRenderer) NeedsCopyMode() bool { return true }

// RenderSkill converts canonical frontmatter to OpenCode format and writes
// the result as {destDir}/SKILL.md (OpenCode uses SKILL.md inside a skill directory).
func (r *OpenCodeRenderer) RenderSkill(canonicalPath string, destDir string) error {
	skillFile, err := resolveSkillFile(canonicalPath)
	if err != nil {
		return fmt.Errorf("opencode: resolve skill %q: %w", canonicalPath, err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("opencode: read %q: %w", skillFile, err)
	}

	fm, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("opencode: parse frontmatter %q: %w", skillFile, err)
	}

	// Transform frontmatter to OpenCode native format.
	fm = r.transformFrontmatter(fm)

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("opencode: serialize frontmatter: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("opencode: mkdir %q: %w", destDir, err)
	}

	// OpenCode uses SKILL.md inside the skill directory (same as Claude/Factory).
	outPath := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(outPath, out, 0o644)
}

// transformFrontmatter applies all OpenCode-specific frontmatter conversions.
func (r *OpenCodeRenderer) transformFrontmatter(fm map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(fm))
	for k, v := range fm {
		out[k] = v
	}

	// Drop unsupported fields.
	for _, field := range openCodeDropFields {
		delete(out, field)
	}

	// Resolve model short name to OpenCode format (github-copilot/...).
	if modelVal, ok := out["model"]; ok {
		if modelStr, ok := modelVal.(string); ok && modelStr != "" {
			out["model"] = resolveOpenCodeModel(modelStr)
		}
	}

	// Convert allowed-tools list to boolean map (tools already dropped above,
	// but if caller passes tools field separately, convert it).
	if toolsVal, ok := out["tools"]; ok {
		if toolsList, ok := toolsVal.([]interface{}); ok {
			toolsMap := make(map[string]bool, len(toolsList))
			for _, t := range toolsList {
				if ts, ok := t.(string); ok {
					toolsMap[normalizeToolName(ts)] = true
				}
			}
			out["tools"] = toolsMap
		}
	}

	// Inject mode: subagent.
	out["mode"] = "subagent"

	// Rename colon to hyphen in the name field.
	if nameVal, ok := out["name"]; ok {
		if nameStr, ok := nameVal.(string); ok {
			out["name"] = colonToHyphen(nameStr)
		}
	}

	return out
}

// normalizeToolName converts a tool name to a simple key.
// Example: "Bash(git:*)" → "bash", "Read" → "read".
func normalizeToolName(tool string) string {
	tool = strings.ToLower(tool)
	if idx := strings.Index(tool, "("); idx >= 0 {
		tool = tool[:idx]
	}
	return tool
}

// RenderCommand writes an OpenCode command file to destDir.
func (r *OpenCodeRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	fm := map[string]interface{}{
		"name":        colonToHyphen(cmd.Name),
		"description": cmd.Action,
		"mode":        "subagent",
	}
	body := fmt.Sprintf("# %s\n\n%s\n", cmd.Name, cmd.Action)
	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("opencode: render command %q: %w", cmd.Name, err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("opencode: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, colonToHyphen(cmd.Name)+".md")
	return os.WriteFile(dest, out, 0o644)
}

// RenderMCPs writes or merges MCP configuration into the config-derived file (opencode.json).
// Format: { "<rootKey>": { "<name>": { "<envKey>": { ... }, ... } } }
// Only sanitized runtime fields (command, args, env, environment, type, url, headers) are
// written; renderer-external metadata fields (agentInstructions, name) are stripped.
// The env key and placeholder format are controlled by the agent's MCPConfig.
func (r *OpenCodeRenderer) RenderMCPs(mcps []model.LockedMCP, cacheStore matypes.CacheStore, workspaceRoot string) error {
	// Resolve effective MCP config from the agent definition.
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	opencodeJSON := ResolveMCPOutputPath(workspaceRoot, mcpConfig)

	// Normalize all MCP definitions: strip metadata, extract catalog instructions.
	normalized, err := normalizeMCPDefinitions(mcps, cacheStore)
	if err != nil {
		return fmt.Errorf("opencode: normalize MCP definitions: %w", err)
	}

	// Load existing opencode.json or start fresh.
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(opencodeJSON); err == nil {
		_ = parseYAML(data, &existing) // best-effort; ignore parse errors
	}

	mcpSection, _ := existing[mcpConfig.RootKey].(map[string]interface{})
	if mcpSection == nil {
		mcpSection = make(map[string]interface{})
	}

	// Store normalized MCPs for catalog memory section injection.
	r.normalizedMCPs = normalized

	for _, mcp := range normalized {
		// Transform the canonical MCP config to OpenCode native format, then apply
		// env key and placeholder style.
		transformed := transformMCPForOpenCode(mcp.ServerConfig)
		mcpSection[mcp.Name] = applyMCPEnvTransform(transformed, mcpConfig)
	}
	existing[mcpConfig.RootKey] = mcpSection

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("opencode: marshal opencode.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(opencodeJSON), 0o755); err != nil {
		return fmt.Errorf("opencode: mkdir %q: %w", filepath.Dir(opencodeJSON), err)
	}
	return os.WriteFile(opencodeJSON, data, 0o644)
}

// RegistryContents returns the registry contents map (keyed by workflow name).
// Used by the materializer to pass to RenderRootCatalog.
func (r *OpenCodeRenderer) RegistryContents() map[string]string {
	return r.registryContents
}

// MCPAgentInstructions returns the MCP agent instructions extracted from normalizedMCPs.
// Used by the materializer to pass to RenderRootCatalog.
func (r *OpenCodeRenderer) MCPAgentInstructions() map[string]string {
	result := make(map[string]string)
	for _, mcp := range r.normalizedMCPs {
		if mcp.AgentInstructions != "" {
			result[mcp.Name] = mcp.AgentInstructions
		}
	}
	return result
}

// InstallWorkflow materializes a workflow into the OpenCode workspace.
//
// Layout (matches real working .opencode installation):
//   - Skills: .opencode/skills/<skill-name>/SKILL.md
//   - _shared: .opencode/skills/_shared/
//   - REGISTRY.md: captured for catalog injection (not copied loose)
//   - SDD agents: synthesized into opencode.json from components.roles
//   - {SKILLS_PATH} and {SDD_MODEL_*}: resolved in installed .md files and opencode.json
//
// NOT created:
//   - agents/ directory (redundant — agent entries go into opencode.json)
//   - commands/ directory (empty; not needed by OpenCode)
func (r *OpenCodeRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	skillsBase := filepath.Join(workspaceRoot, r.def.SkillDir)
	// OpenCode installs workflow files (orchestrator, _shared) in its own workspace
	// to avoid conflicts with shared .agents/skills/ and to allow platform-specific
	// placeholder resolution (subagent types = role names instead of "general").
	workflowDir := filepath.Join(workspaceRoot, wf.Metadata.EffectiveWorkingDir())
	if err := os.MkdirAll(skillsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: workflow mkdir: %w", err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: read workflow dir %q: %w", cachePath, err)
	}

	var managedPaths []string
	var orchPath string // path to the installed entrypoint for orchestrator prompt

	// Resolve agent-specific orchestrator variant if present.
	// ORCHESTRATOR.opencode.md takes precedence over the generic ORCHESTRATOR.md.
	const variantEntrypointName = "ORCHESTRATOR.opencode.md"
	if _, statErr := os.Stat(filepath.Join(cachePath, variantEntrypointName)); statErr == nil {
		orchPath = filepath.Join(cachePath, variantEntrypointName)
	}

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

		// Skip all orchestrator variant files — none should be copied as loose files.
		// Each renderer uses only its own variant (resolved in the pre-loop probe above).
		if orchestratorVariantNames[name] {
			continue
		}

		if skillsSet[name] {
			// Workflow skills: .opencode/skills/<skill-name>/SKILL.md
			destDir := filepath.Join(skillsBase, name)
			if err := r.RenderSkill(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: workflow skill %q: %w", name, err)
			}
			if err := copySkillSubdirs(srcPath, destDir); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: copy skill subdirs for %s: %w", name, err)
			}
			managedPaths = append(managedPaths, destDir)
			continue
		}

		if name == wf.Components.Entrypoint {
			// Entrypoint (ORCHESTRATOR.md) is NOT installed as a file in OpenCode.
			// Instead, its content is read and synthesized into opencode.json as the
			// orchestrator agent prompt. Record the src path for synthesis below.
			if orchPath == "" {
				// No variant found: use the generic entrypoint.
				orchPath = srcPath
			}
			// If orchPath is already set (variant was found), skip the generic file entirely.
			continue
		}

		// Copy everything else (e.g. _shared/) as-is under workflowDir.
		// agents/ and commands/ directories are NOT created — OpenCode uses opencode.json.
		dstPath := filepath.Join(workflowDir, name)
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: workflow copy %q: %w", name, err)
		}
		managedPaths = append(managedPaths, dstPath)
	}

	// Build shared placeholder replacements: {SKILLS_PATH} and {SDD_MODEL_*}.
	// Uses buildWorkflowPlaceholderReplacements to avoid double-slash bugs and
	// to ensure {SDD_MODEL_*} markers are resolved from workflow role metadata.
	// OpenCode subagent resolver: maps role names to their OpenCode agent names.
	openCodeSubagentResolver := func(roleName string) string { return roleName }
	replacements := buildWorkflowPlaceholderReplacements(wf, workspaceRoot, r.def.SkillDir, resolveOpenCodeModel, r.modelOverrides, openCodeSubagentResolver)
	// Override {WORKFLOW_DIR} to point to the workspace-local workflow directory
	// instead of the shared skillsBase path.
	replacements["{WORKFLOW_DIR}"] = workflowDir

	// Capture registry content for catalog injection; apply shared replacements.
	if wf.Components.Registry != "" {
		content, err := captureRegistryContent(cachePath, wf.Components.Registry, replacements)
		if err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: capture registry: %w", err)
		}
		if content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// Resolve placeholders in skill .md files under skillsBase.
	if err := resolvePlaceholders(skillsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: resolve skill placeholders: %w", err)
	}
	// Resolve placeholders in workflow files under the workspace-local workflowDir (if it exists).
	if _, statErr := os.Stat(workflowDir); statErr == nil {
		if err := resolvePlaceholders(workflowDir, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: resolve workflow placeholders: %w", err)
		}
	}

	// Synthesize SDD agents into opencode.json from components.roles.
	if len(wf.Components.Roles) > 0 {
		if err := r.synthesizeAgents(wf, orchPath, workspaceRoot, skillsBase, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("opencode: synthesize agents: %w", err)
		}
		managedPaths = append(managedPaths, filepath.Join(workspaceRoot, "opencode.json"))
	}

	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// synthesizeAgents reads the workflow roles and synthesizes OpenCode agent entries
// into opencode.json. The existing opencode.json content (MCP config, permissions,
// schema, other sections) is preserved; only the "agent" section is updated.
//
// For subagent roles: creates agent entry with mode: "subagent", prompt pointing to
// the installed SKILL.md, and standard tool set.
// For orchestrator role: creates agent entry with mode: "all" and the full
// ORCHESTRATOR.md content as the prompt with placeholders resolved.
func (r *OpenCodeRenderer) synthesizeAgents(wf model.WorkflowManifest, orchPath string, workspaceRoot, skillsBase string, replacements map[string]string) error {
	opencodeJSON := filepath.Join(workspaceRoot, "opencode.json")

	// Load existing opencode.json or start fresh.
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(opencodeJSON); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Get or create the agent section.
	agentSection, _ := existing["agent"].(map[string]interface{})
	if agentSection == nil {
		agentSection = make(map[string]interface{})
	}

	for _, role := range wf.Components.Roles {
		switch role.Kind {
		case "subagent":
			entry := r.buildSubagentEntry(role, skillsBase)
			agentSection[role.Name] = entry

		case "orchestrator":
			entry, err := r.buildOrchestratorEntry(wf, role, orchPath, replacements)
			if err != nil {
				return fmt.Errorf("synthesizeAgents: orchestrator entry: %w", err)
			}
			agentSection[role.Name] = entry
		}
	}

	existing["agent"] = agentSection

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("synthesizeAgents: marshal: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return fmt.Errorf("synthesizeAgents: mkdir: %w", err)
	}
	return os.WriteFile(opencodeJSON, data, 0o644)
}

// buildSubagentEntry creates an opencode.json agent entry for a subagent role.
// The prompt instructs the agent to read its SKILL.md from the installed skills path.
func (r *OpenCodeRenderer) buildSubagentEntry(role model.WorkflowRole, skillsBase string) map[string]interface{} {
	skillPath := skillsBase + "/" + role.Skill + "/SKILL.md"
	prompt := fmt.Sprintf(
		"You are the %s sub-agent. Read the skill file at %s FIRST, then follow its instructions exactly.",
		role.Name, skillPath,
	)

	entry := map[string]interface{}{
		"description": buildRoleDescription(role.Name, "sub-agent"),
		"mode":        "subagent",
		"prompt":      prompt,
		"tools":       toolsCopy(openCodeAgentTools),
	}

	// Check TUI-selected model override first, then fall back to role.Model from workflow.yaml.
	modelValue := ""
	if r.modelOverrides != nil {
		if v, ok := r.modelOverrides[role.Name]; ok && v != "" && v != model.ModelInheritOption {
			modelValue = v
		}
	}
	if modelValue == "" {
		modelValue = role.Model
	}
	if modelValue != "" {
		entry["model"] = resolveOpenCodeModel(modelValue)
	}

	return entry
}

// buildOrchestratorEntry creates an opencode.json agent entry for the orchestrator role.
// The prompt is the full content of the ORCHESTRATOR.md file with all placeholders
// resolved ({SKILLS_PATH}, {SDD_MODEL_*}) using the provided replacements map.
func (r *OpenCodeRenderer) buildOrchestratorEntry(wf model.WorkflowManifest, role model.WorkflowRole, orchPath string, replacements map[string]string) (map[string]interface{}, error) {
	var prompt string
	if orchPath != "" {
		data, err := os.ReadFile(orchPath)
		if err != nil {
			return nil, fmt.Errorf("read orchestrator %q: %w", orchPath, err)
		}
		prompt = string(data)
		// Resolve {SKILLS_PATH} and {SDD_MODEL_*} in the orchestrator prompt content.
		for placeholder, value := range replacements {
			prompt = strings.ReplaceAll(prompt, placeholder, value)
		}
	}

	description := buildRoleDescription(role.Name, fmt.Sprintf("coordinates %s workflow via sub-agents", wf.Metadata.EffectiveDisplayName()))

	entry := map[string]interface{}{
		"description": description,
		"mode":        "all",
		"prompt":      prompt,
		"tools":       toolsCopy(openCodeAgentTools),
	}

	// Orchestrators generally do not specify a model (they inherit session model).
	// Only set it if explicitly defined in role metadata.
	if role.Model != "" {
		entry["model"] = resolveOpenCodeModel(role.Model)
	}

	return entry, nil
}

// buildRoleDescription generates a human-readable description for an agent role.
// Examples:
//
//	"sdd-explorer" + "sub-agent"  → "SDD Explore sub-agent"
//	"sdd-orchestrator" + "..."    → "SDD Orchestrator — coordinates sdd workflow via sub-agents"
func buildRoleDescription(roleName, suffix string) string {
	// Convert "sdd-explorer" → "SDD Explore" style label for known SDD roles.
	label := roleName
	switch roleName {
	case "sdd-explorer":
		label = "SDD Explore"
	case "sdd-planner":
		label = "SDD Plan"
	case "sdd-implementer":
		label = "SDD Implement"
	case "sdd-reviewer":
		label = "SDD Review"
	case "sdd-orchestrator":
		label = "SDD Orchestrator"
	}
	if suffix != "" {
		return label + " " + suffix
	}
	return label
}

// toolsCopy returns a shallow copy of the given tools map.
// This prevents multiple agent entries from sharing the same map reference.
func toolsCopy(tools map[string]bool) map[string]bool {
	out := make(map[string]bool, len(tools))
	for k, v := range tools {
		out[k] = v
	}
	return out
}

// ManagedConfigPaths returns the workspace-level config file paths that the OpenCode
// renderer owns and that the materializer should track for cleanup purposes.
// Uses the same config-derived path as RenderMCPs to prevent drift.
func (r *OpenCodeRenderer) ManagedConfigPaths(workspaceRoot string) []string {
	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	return []string{ResolveMCPOutputPath(workspaceRoot, mcpConfig)}
}

// RenderSettings writes MCP permission entries into the opencode.json "permission" section.
// Returns nil early if no settings block is configured on the agent definition.
func (r *OpenCodeRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	if r.agentDef.Settings == nil {
		return nil
	}

	mcpConfig := EffectiveMCPConfig(r.agentDef.MCP)
	opencodeJSON := ResolveMCPOutputPath(workspaceRoot, mcpConfig)

	// Load existing opencode.json (already written by RenderMCPs/synthesizeAgents).
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(opencodeJSON); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Get or create the "permission" section.
	permSection, _ := existing["permission"].(map[string]interface{})
	if permSection == nil {
		permSection = make(map[string]interface{})
	}

	// Add MCP permissions: "<name>_*": "<level>" for each MCP that declares a level.
	for _, mcp := range r.normalizedMCPs {
		if level, ok := mcp.Permissions["level"]; ok && level != "" {
			key := fmt.Sprintf("%s_*", mcp.Name)
			permSection[key] = level
		}
	}

	// Only write back if there are permission entries to record.
	if len(permSection) == 0 {
		return nil
	}

	existing["permission"] = permSection

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("opencode: marshal opencode.json for settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(opencodeJSON), 0o755); err != nil {
		return fmt.Errorf("opencode: mkdir for settings %q: %w", filepath.Dir(opencodeJSON), err)
	}
	return os.WriteFile(opencodeJSON, data, 0o644)
}

// Finalize is a no-op for OpenCode.
// The env key and placeholder format are now written correctly upfront by RenderMCPs
// via applyMCPEnvTransform, so post-write normalization is no longer needed.
func (r *OpenCodeRenderer) Finalize(workspaceRoot string) error {
	return nil
}

// transformMCPForOpenCode converts a canonical MCP server config (Claude/catalog format)
// into the OpenCode-native format required by opencode.json.
//
// Transformations applied:
//  1. command+args (string+array) → type:"local", command:[cmd, ...args] merged array
//  2. type:"http" → type:"remote"
//
// All other fields (url, headers, environment, env) are passed through unchanged.
// applyMCPEnvTransform handles env key renaming and placeholder style separately.
func transformMCPForOpenCode(src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}

	// Transform type:"http" → type:"remote".
	if t, ok := out["type"].(string); ok && t == "http" {
		out["type"] = "remote"
	}

	// Transform command+args → type:"local", command:[cmd, ...args].
	// Canonical format: command is a string, args is []interface{}.
	// OpenCode format:  type is "local", command is []interface{} (merged).
	if cmdStr, ok := out["command"].(string); ok {
		// Build the merged command array: [command, ...args].
		merged := []interface{}{cmdStr}
		if args, ok := out["args"].([]interface{}); ok {
			merged = append(merged, args...)
		}
		out["command"] = merged
		delete(out, "args")

		// Set type to "local" only if not already set.
		if _, hasType := out["type"]; !hasType {
			out["type"] = "local"
		}
	}

	return out
}
