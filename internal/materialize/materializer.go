// SPDX-License-Identifier: MIT

package materialize

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/state"
)

// RulesMode constants for rules materialization.
const (
	RulesModeConcat     = "concat"
	RulesModeIndividual = "individual"
	RulesModeBoth       = "both"
)

// Materializer orchestrates the 12-step installation pipeline.
// It reads from the cache, delegates agent-specific rendering to AgentRenderer
// implementations, and writes the final workspace state.
//
// Step sequence:
//  1. Acquire advisory lock
//  2. Clean managed paths from previous install
//  3. Create workspace directories
//  4. RenderSkill() for each locked skill
//  5. Materialize rules per agent's configured mode
//  6. RenderMCPs() for all locked MCPs
//  7. InstallWorkflow() for each locked workflow
//  8. RenderCatalog() with all installed skills + workflows
//  9. Update .gitignore for each workflow
//  10. Finalize()
//  11. Write state
//  12. Release advisory lock
type Materializer struct {
	cache     CacheStore
	linker    Linker
	stateMgr  state.StateManager
	renderers map[string]AgentRenderer // keyed by agent type (e.g. "claude")
}

// NewMaterializer constructs a Materializer with the given dependencies.
// renderers maps agent type strings (e.g. "claude") to their AgentRenderer implementations.
func NewMaterializer(
	cache CacheStore,
	linker Linker,
	stateMgr state.StateManager,
	renderers map[string]AgentRenderer,
) *Materializer {
	return &Materializer{
		cache:     cache,
		linker:    linker,
		stateMgr:  stateMgr,
		renderers: renderers,
	}
}

// Install applies a lockfile to the workspace for all requested agents.
// projectRoot is the absolute path to the project root (where devrune.yaml lives).
// It must be provided so that .gitignore and .mcp.json are written to the correct
// directory regardless of the process working directory.
// workflowModels is the optional per-agent role model override map from devrune.yaml (may be nil).
func (m *Materializer) Install(
	ctx context.Context,
	lock model.Lockfile,
	agents []model.AgentRef,
	installCfg model.InstallConfig,
	projectRoot string,
	workflowModels map[string]map[string]string,
) error {
	// Step 1: Acquire advisory lock.
	if err := m.stateMgr.AcquireLock(); err != nil {
		return fmt.Errorf("materializer: acquire lock: %w", err)
	}
	defer m.stateMgr.ReleaseLock() //nolint:errcheck

	// Read existing state so we know which paths to clean.
	prevState, err := m.stateMgr.Read()
	if err != nil {
		return fmt.Errorf("materializer: read state: %w", err)
	}

	// Step 2: Clean managed paths from previous install.
	for _, p := range prevState.ManagedPaths {
		if removeErr := os.RemoveAll(p); removeErr != nil {
			// Non-fatal: log and continue.
			_, _ = fmt.Fprintf(os.Stderr, "materializer: warning: remove %q: %v\n", p, removeErr)
		}
	}

	var managedPaths []string
	var activeAgents []string

	for _, agentRef := range agents {
		renderer, ok := m.renderers[agentRef.Name]
		if !ok {
			return fmt.Errorf("materializer: no renderer for agent %q", agentRef.Name)
		}

		def := renderer.Definition()
		agentWorkspace := def.Workspace

		// Step 3: Create workspace directories.
		dirs := []string{
			filepath.Join(agentWorkspace, def.SkillDir),
		}
		if def.CommandDir != "" {
			dirs = append(dirs, filepath.Join(agentWorkspace, def.CommandDir))
		}
		if def.RulesDir != "" {
			dirs = append(dirs, filepath.Join(agentWorkspace, def.RulesDir))
		}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("materializer: mkdir %q: %w", d, err)
			}
		}

		// Collect all skills and rules for catalog generation.
		var installedSkills []model.ContentItem
		var installedRules []model.ContentItem

		// Step 4 + 5: RenderSkill() and materialize rules for each locked package.
		for _, pkg := range lock.Packages {
			// Verify cache has the package.
			if !m.cache.Has(pkg.Hash) {
				return fmt.Errorf("materializer: package %q not in cache (run 'devrune resolve')", pkg.Hash)
			}
			pkgDir, ok := m.cache.Get(pkg.Hash)
			if !ok {
				return fmt.Errorf("materializer: get cached package %q: not found", pkg.Hash)
			}

			// Step 4: Skills.
			for _, item := range pkg.Contents {
				if item.Kind != model.KindSkill {
					continue
				}
				srcPath := filepath.Join(pkgDir, item.Path)
				destDir := filepath.Join(agentWorkspace, def.SkillDir, item.Name)

				if err := renderer.RenderSkill(srcPath, destDir); err != nil {
					return fmt.Errorf("materializer: render skill %q for agent %q: %w",
						item.Name, agentRef.Name, err)
				}
				// Copy extra files and subdirectories (gotchas.md, references/, etc.)
				// that live alongside SKILL.md. RenderSkill only handles SKILL.md itself.
				if err := renderers.CopySkillExtras(srcPath, destDir); err != nil {
					return fmt.Errorf("materializer: copy skill extras %q for agent %q: %w",
						item.Name, agentRef.Name, err)
				}
				managedPaths = append(managedPaths, destDir)
				installedSkills = append(installedSkills, item)
			}

			// Step 5: Rules.
			rulesMode := installCfg.RulesMode[agentRef.Name]
			if rulesMode == "" {
				rulesMode = def.DefaultRules
			}
			if rulesMode == "" {
				rulesMode = RulesModeIndividual
			}
			for _, item := range pkg.Contents {
				if item.Kind != model.KindRule {
					continue
				}
				srcPath := filepath.Join(pkgDir, item.Path)
				if err := m.materializeRule(item, srcPath, agentWorkspace, def.RulesDir, rulesMode); err != nil {
					return fmt.Errorf("materializer: materialize rule %q for agent %q: %w",
						item.Name, agentRef.Name, err)
				}
				installedRules = append(installedRules, item)
			}
		}

		// Step 6: RenderMCPs().
		if len(lock.MCPs) > 0 {
			if err := renderer.RenderMCPs(lock.MCPs, m.cache, agentWorkspace); err != nil {
				return fmt.Errorf("materializer: render MCPs for agent %q: %w", agentRef.Name, err)
			}
			// Track renderer-managed config surfaces written during MCP rendering
			// (e.g. .factory/mcp.json, .opencode/opencode.json). Renderers that own a
			// config file declare it via the optional ManagedConfigPaths interface so
			// cleanup can remove stale config on reinstall without guessing.
			if cp, ok := renderer.(interface {
				ManagedConfigPaths(workspaceRoot string) []string
			}); ok {
				managedPaths = append(managedPaths, cp.ManagedConfigPaths(agentWorkspace)...)
			}
		}

		// T020: Propagate installed skills to the renderer before workflow installation
		// so that workflow post-processing (adviser table injection) works correctly.
		if setter, ok := renderer.(interface{ SetInstalledSkills([]model.ContentItem) }); ok {
			setter.SetInstalledSkills(installedSkills)
		}

		// Inject per-agent workflow model overrides into the renderer.
		// Uses interface-based injection so renderers that don't support this are silently skipped.
		if setter, ok := renderer.(interface{ SetModelOverrides(map[string]string) }); ok {
			if workflowModels != nil {
				if agentModels, exists := workflowModels[agentRef.Name]; exists {
					setter.SetModelOverrides(agentModels)
				}
			}
		}

		// Collect installed workflow manifests for catalog generation.
		var installedWorkflows []model.WorkflowManifest

		// Step 7: InstallWorkflow() for each locked workflow.
		for _, wf := range lock.Workflows {
			if !m.cache.Has(wf.Hash) {
				return fmt.Errorf("materializer: workflow %q not in cache (run 'devrune resolve')", wf.Name)
			}
			cacheDir, ok := m.cache.Get(wf.Hash)
			if !ok {
				return fmt.Errorf("materializer: get cached workflow %q: not found", wf.Name)
			}

			// wf.Dir is the relative path within the cached archive where workflow.yaml
			// lives (e.g. "workflows/sdd" for a catalog archive, "" for a standalone
			// workflow archive). Join to get the actual workflow root directory.
			wfDir := cacheDir
			if wf.Dir != "" {
				wfDir = filepath.Join(cacheDir, wf.Dir)
			}

			// Parse the workflow manifest from the cached directory.
			wfManifest, err := loadWorkflowManifest(wfDir)
			if err != nil {
				return fmt.Errorf("materializer: load workflow manifest %q: %w", wf.Name, err)
			}

			wfResult, err := renderer.InstallWorkflow(wfManifest, wfDir, agentWorkspace)
			if err != nil {
				return fmt.Errorf("materializer: install workflow %q for agent %q: %w",
					wf.Name, agentRef.Name, err)
			}
			// Use renderer-reported managed paths exclusively. All built-in renderers
			// (Claude, Factory, OpenCode, Copilot) return the actual installed paths
			// via WorkflowInstallResult.ManagedPaths. Custom renderers that have not
			// yet implemented the contract will simply add no workflow paths here,
			// which is safe — they opt in by returning ManagedPaths.
			managedPaths = append(managedPaths, wfResult.ManagedPaths...)
			installedWorkflows = append(installedWorkflows, wfManifest)

			// Step 9: Update .gitignore for this workflow.
			if err := addGitignoreEntry(wfManifest.Metadata.Name, projectRoot); err != nil {
				// Non-fatal: gitignore update failure should not block installation.
				_, _ = fmt.Fprintf(os.Stderr, "materializer: warning: gitignore update for %q: %v\n",
					wfManifest.Metadata.Name, err)
			}
		}

		// Step 8: RenderCatalog().
		catalogPath := filepath.Join(agentWorkspace, def.CatalogFile)
		if err := renderer.RenderCatalog(installedSkills, installedRules, installedWorkflows, catalogPath); err != nil {
			return fmt.Errorf("materializer: render catalog for agent %q: %w", agentRef.Name, err)
		}
		managedPaths = append(managedPaths, catalogPath)

		// Step 8.5: RenderSettings() — generate agent settings file (e.g. .claude/settings.json).
		if err := renderer.RenderSettings(agentWorkspace, installedSkills, installedWorkflows); err != nil {
			return fmt.Errorf("materializer: render settings for agent %q: %w", agentRef.Name, err)
		}

		// Step 10: Finalize().
		if err := renderer.Finalize(agentWorkspace); err != nil {
			return fmt.Errorf("materializer: finalize agent %q: %w", agentRef.Name, err)
		}

		activeAgents = append(activeAgents, agentRef.Name)
	}

	// Step 6.5: Ensure project-root .mcp.json exists for any MCP-enabled install.
	// This is agent-agnostic: Claude's RenderMCPs also writes .mcp.json, but for
	// non-Claude agents the materializer owns the shared project-root file.
	// We run this once after all per-agent RenderMCPs calls complete.
	if len(lock.MCPs) > 0 {
		if err := m.ensureRootMCPJSON(lock.MCPs, projectRoot); err != nil {
			return fmt.Errorf("materializer: ensure root .mcp.json: %w", err)
		}
	}

	// Step 11: Write state.
	var activeWorkflows []string
	for _, wf := range lock.Workflows {
		activeWorkflows = append(activeWorkflows, wf.Name)
	}
	newState := state.State{
		SchemaVersion:   "devrune/state/v1",
		LockHash:        lock.ManifestHash,
		ManagedPaths:    managedPaths,
		ActiveAgents:    activeAgents,
		ActiveWorkflows: activeWorkflows,
	}
	if err := m.stateMgr.Write(newState); err != nil {
		return fmt.Errorf("materializer: write state: %w", err)
	}

	// Step 12.5: Ensure .gitignore protects generated workspace directories and
	// MCP config files (which may contain resolved secrets for Factory).
	hasMCPs := len(lock.MCPs) > 0
	if err := ensureGitignore(agents, m.renderers, hasMCPs, projectRoot); err != nil {
		// Non-fatal: warn but don't fail the install.
		_, _ = fmt.Fprintf(os.Stderr, "materializer: warning: update .gitignore: %v\n", err)
	}

	// Step 13: Release lock (handled by defer).
	return nil
}

// materializeRule copies or concatenates a rule into the agent's rules directory.
func (m *Materializer) materializeRule(
	item model.ContentItem,
	srcPath, agentWorkspace, rulesDir, mode string,
) error {
	if rulesDir == "" {
		return nil
	}
	destBase := filepath.Join(agentWorkspace, rulesDir)

	switch mode {
	case RulesModeIndividual, RulesModeBoth:
		// Flatten: use the base filename as destination, e.g. "a11y-rules.md"
		// srcPath points to the individual .md file.
		baseName := filepath.Base(srcPath)
		dest := filepath.Join(destBase, baseName)
		if err := copyFile(srcPath, dest, 0o644); err != nil {
			return fmt.Errorf("materialize rule %q: %w", item.Name, err)
		}
	}

	// Concat and both modes: append the rule file to a combined rules.md.
	if mode == RulesModeConcat || mode == RulesModeBoth {
		combinedPath := filepath.Join(destBase, "rules.md")
		if err := appendRuleToConcat(srcPath, combinedPath, item.Name); err != nil {
			return fmt.Errorf("materialize rule %q (concat): %w", item.Name, err)
		}
	}
	return nil
}

// appendRuleToConcat reads the rule file at srcPath and appends its contents
// to the combined file at combinedPath, prefixed with a section header.
func appendRuleToConcat(srcPath, combinedPath, ruleName string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read rule file %q: %w", srcPath, err)
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "\n\n<!-- rule: %s -->\n", ruleName)
	sb.Write(data)
	sb.WriteString("\n")

	f, err := os.OpenFile(combinedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open combined file %q: %w", combinedPath, err)
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(sb.String())
	return err
}

// loadWorkflowManifest reads and parses a workflow.yaml from the workflow directory.
func loadWorkflowManifest(wfDir string) (model.WorkflowManifest, error) {
	manifestPath := filepath.Join(wfDir, "workflow.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return model.WorkflowManifest{}, fmt.Errorf("read workflow.yaml: %w", err)
	}
	var wf model.WorkflowManifest
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return model.WorkflowManifest{}, fmt.Errorf("parse workflow.yaml: %w", err)
	}
	return wf, nil
}

// addGitignoreEntry appends ".{workflowName}/" to the project's .gitignore if not already present.
func addGitignoreEntry(workflowName, projectRoot string) error {
	entry := fmt.Sprintf(".%s/", workflowName)
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// Check if already present.
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == entry {
				return nil // Already present.
			}
		}
	}

	// Append the entry.
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintf(f, "%s\n", entry)
	return err
}

// ensureRootMCPJSON creates or merges the project-root .mcp.json file for any
// agent install where MCPs are configured. This makes root MCP config creation
// agent-agnostic: Claude's RenderMCPs also writes .mcp.json, but non-Claude
// agents (Factory, OpenCode, Copilot) rely on the materializer to ensure the
// shared file exists.
//
// Behavior:
//   - If .mcp.json does not exist, it is created from sanitized MCP data.
//   - If .mcp.json already exists, new MCP entries are merged in; existing
//     entries are preserved so unrelated content is never clobbered.
//
// Sanitized MCP entries include only runtime/transport keys:
// command, args, env, environment, type, url, headers.
// The agentInstructions and name metadata fields are stripped.
func (m *Materializer) ensureRootMCPJSON(mcps []model.LockedMCP, projectRoot string) error {
	// allowedMCPKeys lists the runtime/transport keys that may appear in .mcp.json.
	// Metadata keys (agentInstructions, name) must not be serialized.
	allowedMCPKeys := map[string]bool{
		"command":     true,
		"args":        true,
		"env":         true,
		"environment": true,
		"type":        true,
		"url":         true,
		"headers":     true,
	}

	// Build sanitized server map from all MCPs.
	newServers := make(map[string]interface{})
	for _, mcp := range mcps {
		if !m.cache.Has(mcp.Hash) {
			return fmt.Errorf("ensureRootMCPJSON: MCP %q not in cache", mcp.Name)
		}
		cacheDir, ok := m.cache.Get(mcp.Hash)
		if !ok {
			return fmt.Errorf("ensureRootMCPJSON: get MCP %q: not in cache", mcp.Name)
		}

		// Resolve the effective path for the MCP definition (same logic as normalizeMCPDefinitions).
		mcpDefDir := renderers.ResolveMCPDefDir(cacheDir, mcp.Dir)

		rawDef, err := renderers.ReadMCPDefinitionFromDir(mcpDefDir)
		if err != nil {
			return fmt.Errorf("ensureRootMCPJSON: read MCP definition %q: %w", mcp.Name, err)
		}

		// Strip metadata; keep only allowed runtime/transport keys.
		sanitized := make(map[string]interface{})
		for k, v := range rawDef {
			if allowedMCPKeys[k] {
				sanitized[k] = v
			}
		}
		newServers[mcp.Name] = sanitized
	}

	mcpPath := filepath.Join(projectRoot, ".mcp.json")

	// Read existing .mcp.json if it exists — merge: preserve existing entries.
	existingServers := make(map[string]interface{})
	if data, err := os.ReadFile(mcpPath); err == nil {
		var existing map[string]interface{}
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			if servers, ok := existing["mcpServers"]; ok {
				if sm, ok := servers.(map[string]interface{}); ok {
					existingServers = sm
				}
			}
		}
	}

	// Merge: new entries win over empty existing entries (e.g. {} placeholders);
	// non-empty existing entries are preserved so unrelated user config is never clobbered.
	merged := make(map[string]interface{})
	for k, v := range existingServers {
		merged[k] = v
	}
	for k, v := range newServers {
		existing, exists := merged[k]
		if !exists {
			merged[k] = v
			continue
		}
		// Overwrite only if the existing entry is an empty map (i.e. a stale {} placeholder).
		if em, ok := existing.(map[string]interface{}); ok && len(em) == 0 {
			merged[k] = v
		}
	}

	out := map[string]interface{}{
		"mcpServers": merged,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("ensureRootMCPJSON: marshal: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(mcpPath, data, 0o644)
}

const (
	gitignoreBeginMarker = "# >>> devrune managed — do not edit"
	gitignoreEndMarker   = "# <<< devrune managed"
)

// ensureGitignore adds or updates a managed block in .gitignore to protect
// generated workspace directories and MCP config files from being committed.
// The block is delimited by markers so it can be idempotently replaced on
// subsequent installs without touching user-authored entries.
func ensureGitignore(agents []model.AgentRef, renderers map[string]AgentRenderer, hasMCPs bool, projectRoot string) error {
	// Collect entries that must be ignored.
	entries := []string{".devrune/"}
	for _, a := range agents {
		r, ok := renderers[a.Name]
		if !ok {
			continue
		}
		def := r.Definition()
		ws := def.Workspace // may be absolute (tests) or relative (production)
		// Compute path relative to projectRoot so the gitignore entry is always
		// a short workspace-relative pattern (e.g. ".claude/", ".github/").
		if filepath.IsAbs(ws) {
			rel, err := filepath.Rel(projectRoot, ws)
			if err == nil && !strings.HasPrefix(rel, "..") {
				ws = rel
			}
		}
		entries = append(entries, ws+"/")
	}
	if hasMCPs {
		entries = append(entries, ".mcp.json")
	}

	// Build the managed block.
	var block strings.Builder
	block.WriteString(gitignoreBeginMarker + "\n")
	for _, e := range entries {
		block.WriteString(e + "\n")
	}
	block.WriteString(gitignoreEndMarker + "\n")

	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// Read existing content (may not exist yet — that's fine).
	existing, _ := os.ReadFile(gitignorePath)
	content := string(existing)

	// Replace an existing managed block, or append a new one.
	beginIdx := strings.Index(content, gitignoreBeginMarker)
	endIdx := strings.Index(content, gitignoreEndMarker)

	if beginIdx >= 0 && endIdx >= 0 {
		// Found existing block — replace it in-place.
		endIdx += len(gitignoreEndMarker)
		// Consume the trailing newline if present.
		if endIdx < len(content) && content[endIdx] == '\n' {
			endIdx++
		}
		content = content[:beginIdx] + block.String() + content[endIdx:]
	} else {
		// No existing block — append.
		// Ensure there's a blank line separator before our block.
		if len(content) > 0 && !strings.HasSuffix(content, "\n\n") {
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n"
		}
		content += block.String()
	}

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}
