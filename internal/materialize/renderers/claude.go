// SPDX-License-Identifier: MIT

// Package renderers provides built-in AgentRenderer implementations for each
// supported agent type: Claude, OpenCode, Copilot, and Factory.
// All agent-specific frontmatter conversion, model name mapping, tools format
// conversion, and MCP config generation lives here — not in YAML config.
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

// claudeDropFields lists frontmatter fields that Claude does not use and should be stripped.
// Claude's format is the canonical format, so most fields pass through unchanged.
var claudeDropFields = []string{
	"mode",
	"reasoning-effort",
	"temperature",
	"tools-mode",
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
}

// NewClaudeRenderer constructs a ClaudeRenderer from the given agent definition.
func NewClaudeRenderer(agentDef model.AgentDefinition) *ClaudeRenderer {
	return &ClaudeRenderer{
		agentDef: agentDef,
		def: matypes.AgentPaths{
			Workspace:   agentDef.Workspace,
			SkillDir:    agentDef.SkillDir,
			CommandDir:  agentDef.CommandDir,
			RulesDir:    agentDef.RulesDir,
			CatalogFile: agentDef.CatalogFile,
		},
		registryContents:     make(map[string]string),
		mcpAgentInstructions: make(map[string]string),
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
	fm := map[string]interface{}{
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
	servers := make(map[string]interface{})
	for _, mcp := range normalized {
		if mcp.AgentInstructions != "" {
			r.mcpAgentInstructions[mcp.Name] = mcp.AgentInstructions
		}
		servers[mcp.Name] = mcp.ServerConfig
	}

	mcpJSON := map[string]interface{}{
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

// InstallWorkflow materializes a workflow into the Claude workspace.
// It walks the workflow directory, applying:
//   - RenderSkill() to subdirs listed in wf.Components.Skills
//   - Copies wf.Components.Entrypoint (e.g. ORCHESTRATOR.md) as-is to the skills dir
//   - Copies all other files/dirs as-is
//
// T021: Loads Registry content during installation and stores it for RenderCatalog.
// T018: Calls postProcessWorkflow after all entries are installed.
func (r *ClaudeRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath string, workspaceRoot string) (matypes.WorkflowInstallResult, error) {
	skillsSet := make(map[string]bool, len(wf.Components.Skills))
	for _, s := range wf.Components.Skills {
		skillsSet[s] = true
	}

	destBase := filepath.Join(workspaceRoot, r.def.SkillDir, wf.Metadata.Name)
	if err := os.MkdirAll(destBase, 0o755); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow mkdir %q: %w", destBase, err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: read workflow dir %q: %w", cachePath, err)
	}

	var skillDirs []string

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(cachePath, name)
		dstPath := filepath.Join(destBase, name)

		if name == "workflow.yaml" {
			continue // Skip the manifest file itself.
		}

		if skillsSet[name] {
			// Install skills at first level so the Skill tool can discover them.
			dstPath = filepath.Join(workspaceRoot, r.def.SkillDir, name)
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
			// Install entrypoint as-is.
			if err := copyEntry(srcPath, dstPath, entry); err != nil {
				return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow entrypoint %q: %w", name, err)
			}
			continue
		}

		if wf.Components.Registry != "" && name == wf.Components.Registry {
			// Registry is read into the catalog (CLAUDE.md), not copied to the workspace.
			continue
		}

		// Copy everything else as-is.
		if err := copyEntry(srcPath, dstPath, entry); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow copy %q: %w", name, err)
		}
	}

	// Build shared placeholder replacements: {SKILLS_PATH} and {SDD_MODEL_*}.
	// Hoisted outside the registry block so postProcessWorkflow can use it too.
	replacements := buildWorkflowPlaceholderReplacements(wf, r.def.Workspace, r.def.SkillDir+"/"+wf.Metadata.Name, nil, r.modelOverrides)

	// T021: Load registry content if declared in the workflow manifest.
	if wf.Components.Registry != "" {
		content, readErr := captureRegistryContent(cachePath, wf.Components.Registry, replacements)
		if readErr == nil && content != "" {
			r.registryContents[wf.Metadata.Name] = content
		}
	}

	// T018: Run post-processing after all entries are installed.
	if err := r.postProcessWorkflow(wf, destBase, replacements); err != nil {
		return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow post-process %q: %w", wf.Metadata.Name, err)
	}
	for _, sd := range skillDirs {
		if err := r.postProcessWorkflow(wf, sd, replacements); err != nil {
			return matypes.WorkflowInstallResult{}, fmt.Errorf("claude: workflow skill post-process %q: %w", sd, err)
		}
	}

	managedPaths := make([]string, 0, 1+len(skillDirs))
	managedPaths = append(managedPaths, destBase)
	managedPaths = append(managedPaths, skillDirs...)
	return matypes.WorkflowInstallResult{ManagedPaths: managedPaths}, nil
}

// postProcessWorkflow runs post-installation processing on a workflow's rendered files.
// T017: For SKILL.md files containing <!-- ADVISER_TABLE_PLACEHOLDER -->, replaces it
// with a markdown table of installed adviser skills. For ALL .md files (including
// ORCHESTRATOR.md), resolves shared placeholders ({SKILLS_PATH}, {SDD_MODEL_*}).
func (r *ClaudeRenderer) postProcessWorkflow(wf model.WorkflowManifest, destBase string, replacements map[string]string) error {
	// Build the adviser table from installed skills whose names contain "adviser".
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
				// No adviser skills installed — log warning but don't fail.
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

// buildAdviserTable creates a markdown table of installed adviser skills.
// Adviser skills are those whose Name contains "adviser".
func (r *ClaudeRenderer) buildAdviserTable() string {
	var advisers []model.ContentItem
	for _, skill := range r.installedSkills {
		if strings.Contains(skill.Name, "adviser") {
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

	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": permissions,
		},
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

// Finalize is a no-op for Claude.
func (r *ClaudeRenderer) Finalize(workspaceRoot string) error { return nil }

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
func readMCPDefinition(mcpDir string) (map[string]interface{}, error) {
	// If mcpDir is not an existing directory, treat it as a path stem and probe
	// <mcpDir>.yaml and <mcpDir>.yml directly (catalog-hosted single-file MCPs).
	if info, err := os.Stat(mcpDir); err != nil || !info.IsDir() {
		for _, ext := range []string{".yaml", ".yml"} {
			candidate := mcpDir + ext
			if data, err := os.ReadFile(candidate); err == nil {
				var def map[string]interface{}
				if parseErr := parseYAML(data, &def); parseErr != nil {
					return nil, parseErr
				}
				return def, nil
			}
		}
		return map[string]interface{}{}, nil
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
		var def map[string]interface{}
		if err := parseYAML(data, &def); err != nil {
			return nil, err
		}
		return def, nil
	}
	return map[string]interface{}{}, nil
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
