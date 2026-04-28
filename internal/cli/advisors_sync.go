// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidarce/devrune/internal/materialize"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/state"
)

// AdvisorsSyncResult carries the outcome of a SyncAdvisors call.
type AdvisorsSyncResult struct {
	// Written contains the absolute paths of agent files that were created or updated.
	Written []string

	// Deleted contains the absolute paths of agent files that were removed.
	Deleted []string

	// WrittenCatalogDocs contains the absolute paths of root catalog files
	// (CLAUDE.md, AGENTS.md) that were written or updated.
	WrittenCatalogDocs []string

	// WrittenSkillDocs contains the absolute paths of SDD skill instruction
	// files whose managed advisor block was updated.
	WrittenSkillDocs []string

	// SkippedSDDFiles contains the absolute paths of SDD skill instruction
	// files that were skipped because they were missing or lacked the markers.
	SkippedSDDFiles []string

	// Warnings contains user-facing notices collected during sync (e.g. native
	// advisors selected in the manifest but missing on disk, SDD skill files
	// missing the managed-block markers). These are surfaced in the TUI after
	// the sync completes — they MUST NOT be written to stderr/stdout while the
	// TUI altscreen is active, or they corrupt the rendering.
	Warnings []string
}

// defaultRendererProvider is overridable in tests via package-level assignment.
// It builds the AgentRenderer slice from the configured agents in the manifest.
// Production code uses materialize.LoadDefaultRegistry to build all known renderers,
// then filters to those requested in manifest.Agents.
var defaultRendererProvider func(wd string, agents []model.AgentRef) ([]materialize.AgentRenderer, error) = buildDefaultRenderers

// buildDefaultRenderers loads the default renderer registry and returns the renderers
// matching the agents specified in the manifest.
func buildDefaultRenderers(_ string, agents []model.AgentRef) ([]materialize.AgentRenderer, error) {
	registry, err := materialize.LoadDefaultRegistry()
	if err != nil {
		return nil, fmt.Errorf("SyncAdvisors: load renderer registry: %w", err)
	}

	result := make([]materialize.AgentRenderer, 0, len(agents))
	for _, agentRef := range agents {
		r, ok := registry[agentRef.Name]
		if !ok {
			// Skip unknown agent types silently — they may not implement AdvisorRenderer.
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

// SyncAdvisors orchestrates advisor file management for the workspace:
//  1. Reads prior state (for allowlist-based managed-path tracking).
//  2. Resolves manifest.Advisors[] via resolveAdvisors and copies each
//     advisor's directory under .claude/skills/.
//  3. Builds native ContentItems from installed SKILL.md frontmatter.
//  4. Combines native + custom into the installed list.
//  5. Computes the removed list via allowlist.
//  6. Calls RegenerateAdvisorFiles on each AdvisorRenderer.
//  7. Deletes skill directories for removed non-native advisors.
//  8. Syncs catalog docs (CLAUDE.md, AGENTS.md, SDD skill files).
//  9. Writes state.
//
// State is NOT written if any renderer call or catalog-doc sync fails (transactional ordering).
func SyncAdvisors(ctx context.Context, wd string, manifest model.UserManifest) (AdvisorsSyncResult, error) {
	var result AdvisorsSyncResult

	// ── Step 1: Read existing state ──────────────────────────────────────────
	stateMgr := state.NewFileStateManager(wd)
	prevState, err := stateMgr.Read()
	if err != nil {
		return result, fmt.Errorf("SyncAdvisors: read state: %w", err)
	}

	// ── Step 1a: Resolve external advisors via the new Advisors[] schema ─────
	// Each AdvisorSource is fetched + scanned, then its (possibly filtered)
	// advisors are copied into .claude/skills/<name>/.
	resolved, err := resolveAdvisors(ctx, wd, manifest)
	if err != nil {
		return result, fmt.Errorf("SyncAdvisors: resolve advisors: %w", err)
	}

	var customItems []model.ContentItem
	for _, r := range resolved {
		destDir := filepath.Join(wd, ".claude", "skills", r.Def.Name)
		if _, err := copyAdvisorDir(r.DirPath, destDir); err != nil {
			return result, fmt.Errorf("SyncAdvisors: copy advisor %q: %w", r.Def.Name, err)
		}

		customItems = append(customItems, model.ContentItem{
			Kind:        model.KindSkill,
			Name:        r.Def.Name,
			Description: r.Def.Description,
			Custom:      true,
		})
	}

	// ── Step 2: Build native ContentItems from installed SKILL.md frontmatter ─
	// Build lookup: is this native name selected in any package?
	selectedSkills := make(map[string]bool)
	for _, pkg := range manifest.Packages {
		if pkg.Select == nil {
			continue
		}
		for _, s := range pkg.Select.Skills {
			selectedSkills[s] = true
		}
	}

	var nativeItems []model.ContentItem
	for _, name := range model.ReservedAdvisorNames() {
		if !selectedSkills[name] {
			// Not installed — skip.
			continue
		}

		skillMDPath := filepath.Join(wd, ".claude", "skills", name, "SKILL.md")
		data, err := os.ReadFile(skillMDPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Manifest claims this native advisor is selected, but the
				// SKILL.md file is missing on disk. Collect as a warning and
				// skip — the sync of installed advisors continues. The user
				// recovers by running `devrune sync` to install the skill.
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("native advisor %q skipped — SKILL.md missing (run 'devrune sync' to install)", name))
				continue
			}
			return result, fmt.Errorf("SyncAdvisors: read SKILL.md for %q: %w", name, err)
		}

		fm, _, fmErr := parse.ParseFrontmatter(data)
		if fmErr != nil {
			return result, fmt.Errorf("SyncAdvisors: parse frontmatter for %q: %w", name, fmErr)
		}

		description := ""
		if desc, ok := fm["description"]; ok {
			if s, ok := desc.(string); ok {
				description = s
			}
		}

		nativeItems = append(nativeItems, model.ContentItem{
			Kind:        model.KindSkill,
			Name:        name,
			Description: description,
			Custom:      false,
		})
	}

	// ── Step 3: Combine native + custom into the installed list ──────────────
	installed := make([]model.ContentItem, 0, len(nativeItems)+len(customItems))
	installed = append(installed, nativeItems...)
	installed = append(installed, customItems...)

	// ── Step 4: Build removed list via allowlist ──────────────────────────────
	// allowlist = ReservedAdvisorNames ∪ prior.skill-dirs ∪ current.resolved-names
	nativeSet := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		nativeSet[n] = true
	}

	allowlist := make(map[string]bool)
	for _, n := range model.ReservedAdvisorNames() {
		allowlist[n] = true
	}
	// Collect prior custom advisor names from prevState managed paths.
	// ManagedPaths may contain skill dirs like ".claude/skills/<name>".
	skillsBase := filepath.Join(wd, ".claude", "skills")
	for _, p := range prevState.ManagedPaths {
		// Check if the path is directly under skills base.
		rel, relErr := filepath.Rel(skillsBase, p)
		if relErr == nil && rel != "." && !filepath.IsAbs(rel) {
			// Only top-level skill dirs (no sub-paths).
			if filepath.Dir(rel) == "." {
				allowlist[rel] = true
			}
		}
	}
	// Current resolved external advisor names.
	for _, r := range resolved {
		allowlist[r.Def.Name] = true
	}

	// Determine the installed name set for removal computation.
	installedNames := make(map[string]bool, len(installed))
	for _, item := range installed {
		installedNames[item.Name] = true
	}

	// Removed = names in allowlist that are NOT in installed.
	var removed []string
	for name := range allowlist {
		if !installedNames[name] {
			removed = append(removed, name)
		}
	}

	// Determine model overrides (none in this context; callers can extend later).
	var modelOverrides map[string]string

	// ── Step 5: Call RegenerateAdvisorFiles on each AdvisorRenderer ──────────
	agentRenderers, err := defaultRendererProvider(wd, manifest.Agents)
	if err != nil {
		return result, fmt.Errorf("SyncAdvisors: build renderers: %w", err)
	}

	for _, r := range agentRenderers {
		ar, ok := r.(materialize.AdvisorRenderer)
		if !ok {
			// Renderer does not implement AdvisorRenderer — skip silently.
			continue
		}

		// Pass the renderer's workspace directory (e.g. /abs/path/.claude), not the
		// git repo root. RegenerateAdvisorFiles computes agentsBase relative to this path.
		workspace := filepath.Join(wd, r.WorkspacePaths().Workspace)
		renderResult, renderErr := ar.RegenerateAdvisorFiles(workspace, installed, removed, modelOverrides)
		if renderErr != nil {
			// Fail-fast: do NOT write state.
			return result, fmt.Errorf("SyncAdvisors: renderer %T: %w", r, renderErr)
		}

		result.Written = append(result.Written, renderResult.Written...)
		result.Deleted = append(result.Deleted, renderResult.Deleted...)
	}

	// ── Step 6: Delete skill directories for removed non-native advisors ─────
	// Only run after all renderer calls succeed (transactional ordering).
	//
	// A name is deletable if it was previously tracked as a custom advisor in
	// state (prevStateCustomSet) OR appears in the current resolved set under
	// a name override. A name that is ONLY in nativeSet (never installed as
	// custom) is left alone — its skill dir is owned by the resolver
	// (devrune sync), not by SyncAdvisors.
	prevStateCustomSet := make(map[string]bool)
	for _, p := range prevState.ManagedPaths {
		rel, relErr := filepath.Rel(skillsBase, p)
		if relErr == nil && rel != "." && !filepath.IsAbs(rel) && filepath.Dir(rel) == "." {
			prevStateCustomSet[rel] = true
		}
	}
	currentCustomNames := make(map[string]bool, len(resolved))
	for _, r := range resolved {
		currentCustomNames[r.Def.Name] = true
	}

	for _, name := range removed {
		wasCustom := prevStateCustomSet[name] || currentCustomNames[name]
		if nativeSet[name] && !wasCustom {
			// Purely native advisor (never installed as custom): skill dir is owned
			// by the resolver, not by SyncAdvisors — leave it alone.
			continue
		}
		// Remove the custom/catalog advisor skill directory.
		skillDir := filepath.Join(wd, ".claude", "skills", name)
		if err := os.RemoveAll(skillDir); err != nil {
			return result, fmt.Errorf("SyncAdvisors: remove skill dir %q: %w", skillDir, err)
		}
	}

	// ── Step 7: SyncCatalogDocs ──────────────────────────────────────────────
	catalogResult, catalogErr := SyncCatalogDocs(wd, manifest, installed)
	result.Warnings = append(result.Warnings, catalogResult.Warnings...)
	if catalogErr != nil {
		// Treat as abort: state is NOT written.
		return result, fmt.Errorf("SyncAdvisors: sync catalog docs: %w", catalogErr)
	}
	result.WrittenCatalogDocs = catalogResult.WrittenRootFiles
	result.WrittenSkillDocs = catalogResult.WrittenSDDFiles
	result.SkippedSDDFiles = catalogResult.SkippedSDDFiles

	// ── Step 8: Write state LAST (transactional: only after all above succeed) ─
	// Build the new managed paths using the allowlist merge rule:
	// Start from prevState.ManagedPaths, remove paths for advisor names that
	// are no longer in the allowlist, add paths for newly installed advisors.
	//
	// Strategy: rebuild ManagedPaths as:
	//   - All prevState paths that are NOT skill dirs under the allowlist (preserve unrelated paths).
	//   - Plus skill dirs for all currently installed advisors.
	//   - Plus catalogDocPaths written by SyncCatalogDocs (WrittenSDDFiles only —
	//     CLAUDE.md/AGENTS.md are not advisor-owned paths).
	var nextManagedPaths []string

	// Preserve unrelated paths from prev state.
	for _, p := range prevState.ManagedPaths {
		rel, relErr := filepath.Rel(skillsBase, p)
		if relErr != nil || filepath.IsAbs(rel) || filepath.Dir(rel) != "." {
			// Not a top-level skill dir — preserve.
			nextManagedPaths = append(nextManagedPaths, p)
			continue
		}
		// It IS a top-level skill dir: only preserve if name is in current allowlist AND installed.
		if installedNames[rel] {
			nextManagedPaths = append(nextManagedPaths, p)
		}
		// Removed skill dir paths are dropped (they were removed from disk above).
	}

	// Add skill dirs for newly installed advisors (idempotent — dedup below).
	existingPaths := make(map[string]bool, len(nextManagedPaths))
	for _, p := range nextManagedPaths {
		existingPaths[p] = true
	}
	for _, item := range installed {
		skillDir := filepath.Join(wd, ".claude", "skills", item.Name)
		if !existingPaths[skillDir] {
			existingPaths[skillDir] = true
			nextManagedPaths = append(nextManagedPaths, skillDir)
		}
	}

	// Add agent files written by renderers (Contract #6: result.Written paths must
	// be tracked so the next SyncAdvisors call can identify and manage them).
	// result.Deleted paths are intentionally excluded — they no longer exist on disk.
	deletedSet := make(map[string]bool, len(result.Deleted))
	for _, p := range result.Deleted {
		deletedSet[p] = true
	}
	for _, p := range result.Written {
		if !existingPaths[p] && !deletedSet[p] {
			existingPaths[p] = true
			nextManagedPaths = append(nextManagedPaths, p)
		}
	}

	// Add WrittenSDDFiles to managed paths (the SDD skill files modified by SyncCatalogDocs).
	for _, p := range catalogResult.WrittenSDDFiles {
		if !existingPaths[p] {
			existingPaths[p] = true
			nextManagedPaths = append(nextManagedPaths, p)
		}
	}

	nextState := state.State{
		SchemaVersion:   prevState.SchemaVersion,
		LockHash:        prevState.LockHash,
		ActiveAgents:    prevState.ActiveAgents,
		ActiveWorkflows: prevState.ActiveWorkflows,
		ManagedPaths:    nextManagedPaths,
	}

	if err := stateMgr.Write(nextState); err != nil {
		return result, fmt.Errorf("SyncAdvisors: write state: %w", err)
	}

	// ── Step 9: Return ────────────────────────────────────────────────────────
	return result, nil
}
