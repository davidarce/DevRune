// SPDX-License-Identifier: MIT

// Package renderers provides built-in AgentRenderer implementations for each
// supported agent type: Claude, OpenCode, Copilot, and Factory.
// All agent-specific frontmatter conversion, model name mapping, tools format
// conversion, and MCP config generation lives here — not in YAML config.
package renderers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// deprecateAdviserSuffixOnce gates the one-time deprecation log emitted when
// RegenerateAdvisorFiles encounters a legacy "-adviser" suffix item.
var deprecateAdviserSuffixOnce sync.Once

// claudeDropFields lists frontmatter fields that Claude does not use and should be stripped.
// Claude's format is the canonical format, so most fields pass through unchanged.
var claudeDropFields = []string{
	"mode",
	"reasoning-effort",
	"temperature",
	"tools-mode",
}

// claudeSubAgentTools defines the `tools:` frontmatter allowlist for synthesized
// `.claude/agents/*.md` subagent files. Per Anthropic docs
// (https://code.claude.com/docs/en/sub-agents — "By default, subagents inherit
// all tools from the main conversation, including MCP tools"), omitting `tools:`
// lets the subagent inherit the parent's full allowlist.
//
// PHASE sub-agents (`sdd-explore`, `sdd-plan`, `sdd-implement`, `sdd-review`) are
// intentionally ABSENT from this map: omitting the entry causes the renderer to
// skip emitting the `tools:` frontmatter field, which gives the sub-agent the
// parent's full allowlist. Each phase skill's `allowed-tools` already governs
// what the skill body invokes; duplicating that list here would drift as skills
// evolve without adding restriction value (phases legitimately need broad access).
//
// ADVISER/ADVISOR sub-agents ARE listed because Read/Grep/Glob is STRICTER than the
// parent's allowlist; the explicit `tools:` field enforces read-only behavior.
// Both -adviser (legacy) and -advisor (new) keys are kept during the transition
// period until T040 renames the installed skill directories.
var claudeSubAgentTools = map[string][]string{
	"architect-adviser":          {"Read", "Grep", "Glob"},
	"architect-advisor":          {"Read", "Grep", "Glob"},
	"api-first-adviser":          {"Read", "Grep", "Glob"},
	"api-first-advisor":          {"Read", "Grep", "Glob"},
	"unit-test-adviser":          {"Read", "Grep", "Glob"},
	"unit-test-advisor":          {"Read", "Grep", "Glob"},
	"integration-test-adviser":   {"Read", "Grep", "Glob"},
	"integration-test-advisor":   {"Read", "Grep", "Glob"},
	"component-adviser":          {"Read", "Grep", "Glob"},
	"component-advisor":          {"Read", "Grep", "Glob"},
	"frontend-test-adviser":      {"Read", "Grep", "Glob"},
	"frontend-test-advisor":      {"Read", "Grep", "Glob"},
	"web-accessibility-adviser":  {"Read", "Grep", "Glob"},
	"web-accessibility-advisor":  {"Read", "Grep", "Glob"},
}

// claudeSubAgentMCPServers defines the `mcpServers:` frontmatter list per
// Subagent Permissions Matrix (plan Section 2). Derived from what each
// SKILL.md body actually invokes:
//   - engram for ALL sub-agents and advisors (every phase saves via mem_save;
//     advisors persist guidance via mem_save per persistence-contract.md).
//   - atlassian ONLY for sdd-explore (its SKILL.md Step 2 fetches Jira ticket
//     details via mcp__atlassian__jira_get_issue). The sdd-plan, sdd-implement,
//     and sdd-review SKILL.md bodies do NOT reference Atlassian, so they do NOT
//     receive it.
var claudeSubAgentMCPServers = map[string][]string{
	"sdd-explore":                {"engram", "atlassian"},
	"sdd-plan":                   {"engram"},
	"sdd-implement":              {"engram"},
	"sdd-review":                 {"engram"},
	"architect-adviser":          {"engram"},
	"architect-advisor":          {"engram"},
	"api-first-adviser":          {"engram"},
	"api-first-advisor":          {"engram"},
	"unit-test-adviser":          {"engram"},
	"unit-test-advisor":          {"engram"},
	"integration-test-adviser":   {"engram"},
	"integration-test-advisor":   {"engram"},
	"component-adviser":          {"engram"},
	"component-advisor":          {"engram"},
	"frontend-test-adviser":      {"engram"},
	"frontend-test-advisor":      {"engram"},
	"web-accessibility-adviser":  {"engram"},
	"web-accessibility-advisor":  {"engram"},
}

// ClaudeRenderer materializes skills, commands, MCPs, and catalog entries for
// Claude Code. Claude's native frontmatter IS the canonical format, so most
// fields pass through unchanged. Only a small set of non-Claude fields are dropped.
type ClaudeRenderer struct {
	def                  matypes.AgentPaths
	agentDef             model.AgentDefinition
	installedSkills      []model.ContentItem
	registryContents     map[string]string // keyed by workflow name
	mcpAgentInstructions map[string]string // keyed by MCP name
	normalizedMCPs       []normalizedMCP   // MCP entries with permissions for settings injection
	modelOverrides       map[string]string // role-name → model-value from TUI SDD model selection
	workflowCachePaths   map[string]string // keyed by workflow name → cache directory path
}

// NewClaudeRenderer constructs a ClaudeRenderer from the given agent definition.
func NewClaudeRenderer(agentDef model.AgentDefinition) *ClaudeRenderer {
	return &ClaudeRenderer{
		agentDef: agentDef,
		def: matypes.AgentPaths{
			Workspace:   agentDef.Workspace,
			SkillDir:    agentDef.SkillDir,
			AgentDir:    agentDef.AgentDir,
			CommandDir:  agentDef.CommandDir,
			RulesDir:    agentDef.RulesDir,
			CatalogFile: agentDef.CatalogFile,
		},
		registryContents:     make(map[string]string),
		mcpAgentInstructions: make(map[string]string),
		workflowCachePaths:   make(map[string]string),
	}
}

// Name returns "claude".
func (r *ClaudeRenderer) Name() string { return r.agentDef.Name }

// AgentType returns "claude".
func (r *ClaudeRenderer) AgentType() string { return "claude" }

// Definition returns the agent definition.
func (r *ClaudeRenderer) Definition() model.AgentDefinition { return r.agentDef }

// WorkspacePaths returns the configured workspace paths.
func (r *ClaudeRenderer) WorkspacePaths() matypes.AgentPaths { return r.def }

// NeedsCopyMode returns true — Claude strips some fields from frontmatter.
func (r *ClaudeRenderer) NeedsCopyMode() bool { return true }

// SetInstalledSkills stores the installed skills for use during workflow post-processing.
// Called by the materializer before InstallWorkflow.
func (r *ClaudeRenderer) SetInstalledSkills(skills []model.ContentItem) {
	r.installedSkills = skills
}

// SetModelOverrides stores per-role model overrides selected in the TUI SDD model
// selection step. When set, these override the role.Model values from workflow.yaml
// when building {SDD_MODEL_*} placeholder replacements.
func (r *ClaudeRenderer) SetModelOverrides(overrides map[string]string) {
	r.modelOverrides = overrides
}

// RenderSkill reads the canonical SKILL.md from canonicalPath, strips non-Claude
// fields, and writes the result to {destDir}/SKILL.md.
//
// canonicalPath may be either a file (SKILL.md) or a directory containing SKILL.md.
func (r *ClaudeRenderer) RenderSkill(canonicalPath string, destDir string) error {
	skillFile, err := resolveSkillFile(canonicalPath)
	if err != nil {
		return fmt.Errorf("claude: resolve skill file %q: %w", canonicalPath, err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("claude: read %q: %w", skillFile, err)
	}

	fm, body, err := parse.ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("claude: parse frontmatter %q: %w", skillFile, err)
	}

	// Drop fields not supported by Claude.
	for _, field := range claudeDropFields {
		delete(fm, field)
	}

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("claude: serialize frontmatter %q: %w", skillFile, err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("claude: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return fmt.Errorf("claude: write %q: %w", dest, err)
	}
	return nil
}

// RenderCommand writes a minimal SKILL.md stub for a workflow command.
func (r *ClaudeRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	fm := map[string]any{
		"name":          cmd.Name,
		"description":   cmd.Action,
		"argument-hint": cmd.Argument,
	}
	if cmd.Argument == "" {
		delete(fm, "argument-hint")
	}

	body := fmt.Sprintf("# %s\n\n%s\n", cmd.Name, cmd.Action)
	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("claude: render command %q: %w", cmd.Name, err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("claude: mkdir %q: %w", destDir, err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return os.WriteFile(dest, out, 0o644)
}

// RenderMCPs writes a .mcp.json file at the project root (one level above workspace).
// Format: { "mcpServers": { "<name>": { "command": "...", "args": [...] } } }
func (r *ClaudeRenderer) RenderMCPs(mcps []model.LockedMCP, cacheStore matypes.CacheStore, workspaceRoot string) error {
	normalized, err := normalizeMCPDefinitions(mcps, cacheStore)
	if err != nil {
		return fmt.Errorf("claude: normalize MCPs: %w", err)
	}
	// Store normalized MCPs for RenderSettings permission injection.
	r.normalizedMCPs = normalized

	// Populate agentInstructions map and build server configs.
	servers := make(map[string]any)
	for _, mcp := range normalized {
		if mcp.AgentInstructions != "" {
			r.mcpAgentInstructions[mcp.Name] = mcp.AgentInstructions
		}
		servers[mcp.Name] = mcp.ServerConfig
	}

	mcpJSON := map[string]any{
		"mcpServers": servers,
	}

	data, err := json.MarshalIndent(mcpJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("claude: marshal MCP JSON: %w", err)
	}
	data = append(data, '\n')

	// .mcp.json lives at project root, one level above the workspace dir.
	// workspaceRoot is relative (e.g. ".claude"), so project root is ".".
	projectRoot := filepath.Dir(workspaceRoot)
	mcpPath := filepath.Join(projectRoot, ".mcp.json")
	return os.WriteFile(mcpPath, data, 0o644)
}

// RegistryContents returns the registry contents map (keyed by workflow name).
// Used by the materializer to pass to RenderRootCatalog.
func (r *ClaudeRenderer) RegistryContents() map[string]string {
	return r.registryContents
}

// MCPAgentInstructions returns the MCP agent instructions map (keyed by MCP name).
// Used by the materializer to pass to RenderRootCatalog.
func (r *ClaudeRenderer) MCPAgentInstructions() map[string]string {
	return r.mcpAgentInstructions
}

// InstallWorkflow materializes a workflow into the Claude workspace using the
// Claude-native layout (always on; no branching).
//
// The renderer now produces:
//   - .claude/skills/{skill}/SKILL.md for every workflow skill
//   - .claude/skills/{workingDir}/ORCHESTRATOR.md for the entrypoint (variant-probed:
//     ORCHESTRATOR.claude.md is preferred when present in the cache; suffix is stripped
//     at write time so the destination filename remains wf.Components.Entrypoint)
//   - .claude/agents/{role-name}.md for every subagent role in workflow.yaml
//   - .claude/agents/{advisor-name}.md for every installed `*-advisor` or `*-adviser` skill
//
// Hook script assets are copied as before; this logic is preserved verbatim.
// T021: Loads Registry content during installation and stores it for RenderCatalog.
func (r *ClaudeRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	// 1. Probe the Claude-native orchestrator variant.
	const variantEntrypointName = "ORCHESTRATOR.claude.md"
	variantOrchPath := ""
	if _, statErr := os.Stat(filepath.Join(cachePath, variantEntrypointName)); statErr == nil {
		variantOrchPath = filepath.Join(cachePath, variantEntrypointName)
	}

	// 1a. Probe the Claude-native registry variant. When REGISTRY.claude.md exists
	// alongside the generic REGISTRY.md, the Claude renderer uses the variant so that
	// CLAUDE.md receives Agent(subagent_type:...) launch instructions rather than the
	// generic Task()+Skill() pattern.
	variantRegistryFile := ""
	if wf.Components.Registry != "" {
		candidateName := strings.TrimSuffix(wf.Components.Registry, ".md") + ".claude.md"
		if _, statErr := os.Stat(filepath.Join(cachePath, candidateName)); statErr == nil {
			variantRegistryFile = candidateName
		}
	}

	// 2. Build sets used by the scan loop.
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	// 3. Determine destinations.
	destBase := filepath.Join(workspaceRoot, r.def.SkillDir, wf.Metadata.EffectiveWorkingDir())
	skillsBase := filepath.Join(workspaceRoot, r.def.SkillDir)
	agentDirName := r.def.AgentDir
	if agentDirName == "" {
		// Defensive default — claude.yaml now sets `agentDir: "agents"` permanently,
		// but fixtures and older test definitions may not.
		agentDirName = "agents"
	}
	agentsBase := filepath.Join(workspaceRoot, agentDirName)
	if err := os.MkdirAll(destBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow mkdir %q: %w", destBase, err)
	}
	if err := os.MkdirAll(agentsBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow mkdir agents %q: %w", agentsBase, err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: read workflow dir %q: %w", cachePath, err)
	}

	var skillDirs []string

	// 4. Scan loop — install skills, place orchestrator body via variant probe,
	//    copy remaining aux files (e.g. _shared/) into destBase.
	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(cachePath, name)
		dstPath := filepath.Join(destBase, name)

		if name == "workflow.yaml" {
			continue // Skip the manifest file itself.
		}

		// Skip all orchestrator variant files — none should be copied as loose files.
		// The Claude-native variant (if present) is placed via the entrypoint branch
		// below with the suffix stripped to wf.Components.Entrypoint.
		if orchestratorVariantNames[name] {
			continue
		}

		if skillsSet[name] {
			// Install skills at first level so the Skill tool can discover them.
			dstPath = filepath.Join(skillsBase, name)
			// Apply frontmatter transform.
			if err := r.RenderSkill(srcPath, dstPath); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow skill %q: %w", name, err)
			}
			if err := copySkillSubdirs(srcPath, dstPath); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: copy skill subdirs for %s: %w", name, err)
			}
			skillDirs = append(skillDirs, dstPath)
			continue
		}

		if name == wf.Components.Entrypoint {
			// Install the entrypoint, preferring the Claude-native variant when present.
			// Destination filename is always wf.Components.Entrypoint (suffix-stripped).
			effectiveSrc := srcPath
			if variantOrchPath != "" {
				effectiveSrc = variantOrchPath
			}
			if err := copySingleFile(effectiveSrc, dstPath, 0o644); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow entrypoint %q: %w", name, err)
			}
			continue
		}

		if wf.Components.Registry != "" && (name == wf.Components.Registry || name == variantRegistryFile) {
			// Registry (and its Claude-native variant) are read into the catalog (CLAUDE.md),
			// not copied to the workspace.
			continue
		}

		// Skip hook/plugin asset directories — only copied by the agent that declares them.
		if hookAssetDirNames[name] {
			continue
		}

		// Copy everything else; apply variant-suffix stripping for _shared/ so that
		// launch-templates.claude.md → launch-templates.md and files for other variants
		// are skipped entirely.
		if entry.IsDir() && name == "_shared" {
			if err := copyDirRecursiveStripVariant(srcPath, dstPath, "claude"); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow copy %q: %w", name, err)
			}
		} else if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow copy %q: %w", name, err)
		}
	}

	// Remove _shared/ files that the Claude-native orchestrator does not reference.
	// launch-templates.md is now installed from launch-templates.claude.md (variant suffix
	// stripped) and IS referenced by ORCHESTRATOR.claude.md — do NOT remove it.
	// adviser-templates.md is only needed by Codex/Factory via the generic ORCHESTRATOR.md.
	_ = os.Remove(filepath.Join(destBase, "_shared", "adviser-templates.md"))

	// 5. If the entrypoint was not present in the cache directory (cache lacks the
	//    generic ORCHESTRATOR.md) but a Claude variant IS present, install the variant
	//    under the generic filename. This preserves the suffix-stripping contract for
	//    variant-only catalogs.
	if variantOrchPath != "" {
		destEntrypoint := filepath.Join(destBase, wf.Components.Entrypoint)
		if _, statErr := os.Stat(destEntrypoint); os.IsNotExist(statErr) {
			if err := copySingleFile(variantOrchPath, destEntrypoint, 0o644); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: install variant orchestrator: %w", err)
			}
		}
	}

	// Store cache path so RenderSettings can locate hook JSON definitions.
	r.workflowCachePaths[wf.Metadata.Name] = cachePath

	// Copy hook script assets for Claude (preserved from previous InstallWorkflow body).
	// For each hook definition, read the JSON and copy any .sh files referenced
	// (string values ending in .sh) from the workflow cache to the agent workspace
	// with executable permissions (0o755).
	if wf.Components.Hooks != nil {
		if defs, ok := wf.Components.Hooks.Agents["claude"]; ok {
			for _, def := range defs {
				jsonPath := filepath.Join(cachePath, def.Definition)
				hookData, err := ReadAndValidateHookJSON(jsonPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠️  claude: invalid hook JSON %s: %v (skipping asset copy)\n", def.Definition, err)
					continue
				}
				if err := copyHookScriptAssets(hookData, cachePath, workspaceRoot, r.def.Workspace, ".sh", 0o755); err != nil {
					return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: copy hook assets for %s: %w", def.Definition, err)
				}
			}
		}
	}

	// Build shared placeholder replacements: {SKILLS_PATH}, {WORKFLOW_MODEL_*}, etc.
	// Passing nil for modelResolver keeps bare short names (e.g. "sonnet", "opus") in
	// the agent-file frontmatter — Claude Code expects short IDs, not the "anthropic/..."
	// form produced by resolveModel().
	replacements := buildWorkflowPlaceholderReplacements(wf, r.def.Workspace, r.def.SkillDir, nil, r.modelOverrides, nil)
	// Overwrite subagent placeholders with native role names (Claude-native uses
	// Agent(subagent_type: 'sdd-explorer', ...) rather than a generic type).
	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" {
			continue
		}
		key := model.PlaceholderKeyFromRole(wf.Metadata.Name, role.Name, role.Placeholder)
		replacements["{WORKFLOW_SUBAGENT_"+key+"}"] = role.Name
	}

	// 6. Synthesize phase subagent .md files — one per subagent role (skip sentinels).
	var agentFilePaths []string
	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" || role.Skill == "" || strings.Contains(role.Skill, "*") {
			continue
		}
		skillSrcDir := filepath.Join(cachePath, role.Skill)
		dstPath := filepath.Join(agentsBase, role.Name+".md")
		if err := r.generateSubAgentFile(skillSrcDir, dstPath, role, wf); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: generate sub-agent %q: %w", role.Name, err)
		}
		agentFilePaths = append(agentFilePaths, dstPath)
	}

	// 7. Synthesize advisor wrapper .md files — one per installed *-advisor or *-adviser skill.
	//    Both suffixes are checked for transition compat until T040 renames installed skill dirs.
	//    Skip if a file already exists (e.g. generated by the subagent role loop above).
	for _, skill := range r.installedSkills {
		if !strings.HasSuffix(skill.Name, "-advisor") && !strings.HasSuffix(skill.Name, "-adviser") {
			continue
		}
		dstPath := filepath.Join(agentsBase, skill.Name+".md")
		if _, statErr := os.Stat(dstPath); statErr == nil {
			continue
		}
		if err := r.generateAdvisorAgentFile(agentsBase, skill.Name); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: generate advisor %q: %w", skill.Name, err)
		}
		agentFilePaths = append(agentFilePaths, dstPath)
	}

	// T021: Load registry content if declared in the workflow manifest.
	// Prefer the Claude-native variant (REGISTRY.claude.md) when present so that
	// CLAUDE.md receives Agent(subagent_type:...) launch instructions rather than
	// the generic Task()+Skill() pattern.
	if wf.Components.Registry != "" {
		registryFile := wf.Components.Registry
		if variantRegistryFile != "" {
			registryFile = variantRegistryFile
		}
		content, readErr := captureRegistryContent(cachePath, registryFile, replacements)
		if readErr == nil && content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// 8. Post-process: resolve placeholders across destBase (workflow dir), skillDirs
	//    (individual skill SKILL.md files), and agentsBase (synthesized agent files).
	if err := r.postProcessWorkflow(destBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow post-process %q: %w", wf.Metadata.Name, err)
	}
	for _, sd := range skillDirs {
		if err := r.postProcessWorkflow(sd, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow skill post-process %q: %w", sd, err)
		}
	}
	if err := r.postProcessWorkflow(agentsBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow agents post-process: %w", err)
	}

	// 9. Strip any lines with unresolved model placeholders (phases where the user
	//    selected "inherit from session" leave {WORKFLOW_MODEL_*} unsubstituted).
	if err := removeModelPlaceholderLines(agentsBase); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: remove model placeholder lines (agents): %w", err)
	}

	// 10. Compose ManagedPaths. Include destBase, skillDirs, and each synthesized
	//     agent file individually. We intentionally DO NOT include agentsBase itself
	//     — the materializer calls os.RemoveAll on every managed path on reinstall,
	//     so listing agentsBase would wipe user-authored subagents in .claude/agents/.
	//     Listing individual files keeps cleanup scoped to the renderer's own output.
	managedPaths := make([]string, 0, 1+len(skillDirs)+len(agentFilePaths))
	managedPaths = append(managedPaths, destBase)
	managedPaths = append(managedPaths, skillDirs...)
	managedPaths = append(managedPaths, agentFilePaths...)
	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// generateSubAgentFile synthesizes a phase sub-agent file at `dstPath` by writing
// YAML frontmatter + a minimal body that references the preloaded skill.
//
// The `skills:` frontmatter points Claude Code to the installed skill in
// `.claude/skills/{role.Skill}/`, so the full skill body is NOT embedded here
// (the `<1 KB` file size goal per plan.md Risks §6). Tools are omitted for phase
// subagents — the absence of `tools:` causes Claude Code to fall back to the
// parent conversation's full allowlist. MCP servers and the model placeholder
// come from the Subagent Permissions Matrix.
//
// Model is emitted as the `{WORKFLOW_MODEL_<KEY>}` placeholder and resolved later
// by postProcessWorkflow via `replacements` (which is populated by
// buildWorkflowPlaceholderReplacements).
func (r *ClaudeRenderer) generateSubAgentFile(
	skillSrcDir string,
	dstPath string,
	role model.WorkflowRole,
	wf model.WorkflowManifest,
) error {
	// Verify the skill source exists (catch typos in workflow.yaml early).
	skillFile := filepath.Join(skillSrcDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		return fmt.Errorf("claude: sub-agent %q: skill file %q missing: %w", role.Name, skillFile, err)
	}

	description := skillDescriptionForRole(role.Skill)
	if description == "" {
		description = role.Skill + " sub-agent"
	}
	description += ". Invoked by sdd-orchestrator. Do not invoke directly."

	fm := map[string]any{
		"name":                     role.Name,
		"description":              description,
		"skills":                   []string{role.Skill},
		"permissionMode":           "default",
		"disable-model-invocation": false,
	}

	// Tools — phase roles are absent from claudeSubAgentTools by design; omit the
	// field entirely so the subagent inherits the parent's full tool allowlist.
	if tools, ok := claudeSubAgentTools[role.Skill]; ok && len(tools) > 0 {
		fm["tools"] = tools
	}

	// MCP servers — every phase subagent declares engram; sdd-explore adds atlassian.
	if servers, ok := claudeSubAgentMCPServers[role.Skill]; ok && len(servers) > 0 {
		fm["mcpServers"] = servers
	}

	// Model: emit the placeholder so postProcessWorkflow can substitute it using
	// the shared {WORKFLOW_MODEL_*} table (TUI override > role.Model > removed if
	// inherit-from-session). Never hard-code "sonnet"/"opus" here.
	placeholderKey := "{WORKFLOW_MODEL_" + model.PlaceholderKeyFromRole(wf.Metadata.Name, role.Name, role.Placeholder) + "}"
	fm["model"] = placeholderKey

	body := fmt.Sprintf(
		"You are the %s sub-agent. Your full instructions are preloaded from the `%s` skill above. Follow them and return the SDD Envelope.\n",
		role.Name, role.Skill,
	)

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("claude: serialize sub-agent frontmatter for %q: %w", role.Name, err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("claude: mkdir sub-agent: %w", err)
	}
	return os.WriteFile(dstPath, out, 0o644)
}

// generateAdvisorAgentFile synthesizes a lightweight advisor wrapper at
// `.claude/agents/{advisor-name}.md`. Unlike phase sub-agents, advisors DECLARE
// `tools: [Read, Grep, Glob]` — this list is STRICTER than the parent's allowlist,
// so emitting it enforces read-only behavior. The `skills:` field preloads the
// advisor skill body, so the agent file itself stays under 1 KB.
//
// Model resolves via the {WORKFLOW_MODEL_ADVISER} placeholder emitted by the
// sentinel `sdd-adviser` role in workflow.yaml (substituted by postProcessWorkflow
// after this helper returns).
func (r *ClaudeRenderer) generateAdvisorAgentFile(agentsBase string, advisorName string) error {
	description := skillDescriptionForRole(advisorName)
	if description == "" {
		description = advisorName + " specialist advisor"
	}
	description += ". Invoked by sdd-orchestrator during guidance loop. Do not invoke directly."

	fm := map[string]any{
		"name":                     advisorName,
		"description":              description,
		"skills":                   []string{advisorName},
		"permissionMode":           "default",
		"disable-model-invocation": false,
		"model":                    "{WORKFLOW_MODEL_ADVISER}",
	}

	tools := claudeSubAgentTools[advisorName]
	if len(tools) == 0 {
		tools = []string{"Read", "Grep", "Glob"}
	}
	fm["tools"] = tools

	if servers, ok := claudeSubAgentMCPServers[advisorName]; ok && len(servers) > 0 {
		fm["mcpServers"] = servers
	} else {
		fm["mcpServers"] = []string{"engram"}
	}

	body := "You are a specialist advisor. Your skill content is preloaded above. Follow its instructions: read the plan at `.sdd/{change-name}/plan.md`, analyse from your specialist domain perspective, and persist guidance via `mem_save`. Return the structured advice format (Strengths / Issues Found / Recommendations).\n"

	out, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		return fmt.Errorf("claude: serialize advisor frontmatter for %q: %w", advisorName, err)
	}

	dstPath := filepath.Join(agentsBase, advisorName+".md")
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("claude: mkdir advisor: %w", err)
	}
	return os.WriteFile(dstPath, out, 0o644)
}

// postProcessWorkflow runs post-installation processing on a workflow's rendered files.
// T017: For SKILL.md files containing <!-- ADVISER_TABLE_PLACEHOLDER -->, replaces it
// with a markdown table of installed advisor skills. For ALL .md files (including
// ORCHESTRATOR.md), resolves shared placeholders ({SKILLS_PATH}, {SDD_MODEL_*}).
func (r *ClaudeRenderer) postProcessWorkflow(destBase string, replacements map[string]string) error {
	// Build the advisor table from installed skills whose names contain "advisor" or "adviser".
	adviserTable := r.buildAdviserTable()

	// Walk all .md files under destBase and apply replacements.
	return filepath.WalkDir(destBase, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("postProcess: read %q: %w", path, err)
		}

		content := string(data)
		modified := false

		// Resolve all shared placeholders ({SKILLS_PATH}, {SDD_MODEL_*}).
		for placeholder, value := range replacements {
			if strings.Contains(content, placeholder) {
				content = strings.ReplaceAll(content, placeholder, value)
				modified = true
			}
		}

		// Replace <!-- ADVISER_TABLE_PLACEHOLDER --> only in SKILL.md files.
		if d.Name() == "SKILL.md" && strings.Contains(content, "<!-- ADVISER_TABLE_PLACEHOLDER -->") {
			if adviserTable == "" {
				// No advisor skills installed — log warning but don't fail.
				content = strings.ReplaceAll(content, "<!-- ADVISER_TABLE_PLACEHOLDER -->", "")
			} else {
				content = strings.ReplaceAll(content, "<!-- ADVISER_TABLE_PLACEHOLDER -->", adviserTable)
			}
			modified = true
		}

		if modified {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("postProcess: write %q: %w", path, err)
			}
		}
		return nil
	})
}

// buildAdviserTable creates a markdown table of installed advisor skills.
// Advisor skills are those whose Name contains "adviser" or "advisor".
func (r *ClaudeRenderer) buildAdviserTable() string {
	var advisers []model.ContentItem
	for _, skill := range r.installedSkills {
		if strings.Contains(skill.Name, "adviser") || strings.Contains(skill.Name, "advisor") {
			advisers = append(advisers, skill)
		}
	}
	if len(advisers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("| Skill | Invocation | Use When |\n")
	sb.WriteString("|-------|------------|----------|\n")
	for _, a := range advisers {
		_, _ = fmt.Fprintf(&sb, "| `%s` | `/%s` | %s |\n", a.Name, a.Name, a.Description)
	}
	return sb.String()
}

// RenderSettings generates .claude/settings.json with permissions derived from
// the agent definition and installed workflows.
// T022: If agentDef.Settings is nil, settings generation is skipped (returns nil).
func (r *ClaudeRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	if r.agentDef.Settings == nil {
		return nil
	}

	// Collect all permissions: base from agent definition + workflow-level permissions.
	seen := make(map[string]bool)
	var permissions []string

	for _, p := range r.agentDef.Settings.Permissions {
		if !seen[p] {
			seen[p] = true
			permissions = append(permissions, p)
		}
	}
	for _, wf := range workflows {
		for _, p := range wf.Components.Permissions {
			if !seen[p] {
				seen[p] = true
				permissions = append(permissions, p)
			}
		}
	}

	// Merge MCP permissions: for each MCP with level "allow", add "mcp__<name>__*".
	for _, mcp := range r.normalizedMCPs {
		if mcp.Permissions["level"] == "allow" {
			p := fmt.Sprintf("mcp__%s__*", mcp.Name)
			if !seen[p] {
				seen[p] = true
				permissions = append(permissions, p)
			}
		}
	}

	settings := map[string]any{
		"permissions": map[string]any{
			"allow": permissions,
		},
	}

	// T003: Deep-merge opaque hook JSON definitions for Claude into settings.
	// DevRune validates JSON syntax but does NOT interpret the content.
	// The JSON IS the native settings.json fragment.
	for _, wf := range workflows {
		if wf.Components.Hooks == nil {
			continue
		}
		defs, ok := wf.Components.Hooks.Agents["claude"]
		if !ok {
			continue
		}
		cachePath := r.workflowCachePaths[wf.Metadata.Name]
		if cachePath == "" {
			continue
		}
		for _, def := range defs {
			jsonPath := filepath.Join(cachePath, def.Definition)
			hookData, err := ReadAndValidateHookJSON(jsonPath)
			if err != nil {
				// Warn user and skip — invalid hook JSON is non-fatal.
				fmt.Fprintf(os.Stderr, "⚠️  claude: invalid hook JSON %s: %v (skipping)\n", def.Definition, err)
				continue
			}
			settings = deepMergeJSON(settings, hookData)
		}
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("claude: marshal settings.json: %w", err)
	}
	data = append(data, '\n')

	settingsPath := filepath.Join(workspaceRoot, "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("claude: mkdir for settings: %w", err)
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

// SettingsManagedPaths returns the settings file path that RenderSettings writes for Claude.
// Used by the materializer to track the file for cleanup on reinstall.
func (r *ClaudeRenderer) SettingsManagedPaths(workspaceRoot string) []string {
	if r.agentDef.Settings == nil {
		return nil
	}
	return []string{filepath.Join(workspaceRoot, "settings.json")}
}

// Finalize is a no-op for Claude.
func (r *ClaudeRenderer) Finalize(workspaceRoot string) error { return nil }

// RegenerateAdvisorFiles implements the materialize.AdvisorRenderer capability port.
// It writes or removes advisor agent wrapper files under {workspaceRoot}/{agentDir}/
// without running a full resolve cycle.
//
// Detection rule (applied uniformly across all renderers):
//
//	hasAdvisorSuffix := strings.HasSuffix(strings.ToLower(item.Name), "-advisor")
//	hasLegacySuffix  := strings.HasSuffix(strings.ToLower(item.Name), "-adviser") // compat shim
//	isAdvisor        := hasAdvisorSuffix || hasLegacySuffix || item.Custom
//
// Legacy "-adviser" names emit a one-per-process deprecation log via sync.Once.
// Non-advisor entries in installed are silently ignored.
//
// The method is stateless on the receiver — it does NOT assign installed to
// r.installedSkills or mutate any other receiver field. Two consecutive calls
// with different installed sets produce independent results with no state leak.
//
// postProcessWorkflow is intentionally NOT called here; callers that need
// placeholder resolution must invoke it separately.
func (r *ClaudeRenderer) RegenerateAdvisorFiles(
	workspaceRoot string,
	installed []model.ContentItem,
	removed []string,
	modelOverrides map[string]string,
) (matypes.AdvisorRenderResult, error) {
	agentDirName := r.def.AgentDir
	if agentDirName == "" {
		agentDirName = "agents"
	}
	agentsBase := filepath.Join(workspaceRoot, agentDirName)

	var result matypes.AdvisorRenderResult

	// Write advisor files for each installed item that passes the detection rule.
	for _, item := range installed {
		nameLower := strings.ToLower(item.Name)
		hasAdvisorSuffix := strings.HasSuffix(nameLower, "-advisor")
		hasLegacySuffix := strings.HasSuffix(nameLower, "-adviser")
		isAdvisor := hasAdvisorSuffix || hasLegacySuffix || item.Custom

		if !isAdvisor {
			continue
		}

		if hasLegacySuffix && !hasAdvisorSuffix {
			deprecateAdviserSuffixOnce.Do(func() {
				log.Printf("DEPRECATION: advisor name %q uses legacy '-adviser' suffix; rename to '-advisor' before next minor release", item.Name)
			})
		}

		if err := r.generateAdvisorAgentFile(agentsBase, item.Name); err != nil {
			return result, fmt.Errorf("claude: RegenerateAdvisorFiles: generate advisor %q: %w", item.Name, err)
		}
		result.Written = append(result.Written, filepath.Join(agentsBase, item.Name+".md"))
	}

	// Delete advisor files for removed skill names.
	// removed wins: names present in both installed and removed result in absent files
	// (the delete loop below runs after the write loop above, so removal always wins).
	for _, name := range removed {
		target := filepath.Join(agentsBase, name+".md")
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return result, fmt.Errorf("claude: RegenerateAdvisorFiles: remove advisor file %q: %w", target, err)
		}
		result.Deleted = append(result.Deleted, target)
	}

	return result, nil
}

// resolveSkillFile returns the path to SKILL.md given either a directory or file path.
func resolveSkillFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		candidate := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(candidate); err != nil {
			return "", fmt.Errorf("no SKILL.md in directory %q", path)
		}
		return candidate, nil
	}
	return path, nil
}

// readMCPDefinition reads the MCP server definition from a cached directory.
// The definition YAML file may be named mcp.yaml or have the MCP name.
func readMCPDefinition(mcpDir string) (map[string]any, error) {
	// If mcpDir is not an existing directory, treat it as a path stem and probe
	// <mcpDir>.yaml and <mcpDir>.yml directly (catalog-hosted single-file MCPs).
	if info, err := os.Stat(mcpDir); err != nil || !info.IsDir() {
		for _, ext := range []string{".yaml", ".yml"} {
			candidate := mcpDir + ext
			if data, err := os.ReadFile(candidate); err == nil {
				var def map[string]any
				if parseErr := parseYAML(data, &def); parseErr != nil {
					return nil, parseErr
				}
				return def, nil
			}
		}
		return map[string]any{}, nil
	}

	// mcpDir is a real directory — scan for a YAML definition file.
	// Try common names.
	candidates := []string{"mcp.yaml", "definition.yaml"}
	entries, err := os.ReadDir(mcpDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
			candidates = append([]string{e.Name()}, candidates...)
			break
		}
	}

	for _, name := range candidates {
		path := filepath.Join(mcpDir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var def map[string]any
		if err := parseYAML(data, &def); err != nil {
			return nil, err
		}
		return def, nil
	}
	return map[string]any{}, nil
}

// copyEntry copies a directory entry (file or dir) from src to dst.
func copyEntry(src, dst string, entry os.DirEntry) error {
	if entry.IsDir() {
		return copyDirRecursive(src, dst)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	return copySingleFile(src, dst, info.Mode())
}
