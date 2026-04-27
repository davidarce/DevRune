// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// advisorsBeginMarker is the opening HTML comment that bounds the managed
// advisor table inside SDD skill instruction files.
const advisorsBeginMarker = "<!-- devrune advisors:begin -->"

// advisorsEndMarker is the closing HTML comment that bounds the managed
// advisor table inside SDD skill instruction files.
const advisorsEndMarker = "<!-- devrune advisors:end -->"

// sddSkillFiles is the fixed set of SDD skill instruction files that may
// contain a managed advisor table block. The paths are relative to wd.
var sddSkillFiles = []string{
	filepath.Join(".claude", "skills", "sdd-plan", "SKILL.md"),
	filepath.Join(".claude", "skills", "sdd-review", "SKILL.md"),
	filepath.Join(".claude", "skills", "sdd-orchestrator", "ORCHESTRATOR.md"),
}

// SyncCatalogDocsResult carries the outcome of a SyncCatalogDocs call.
type SyncCatalogDocsResult struct {
	// WrittenRootFiles contains the absolute paths of root catalog files
	// (CLAUDE.md, AGENTS.md) that were written or updated.
	WrittenRootFiles []string

	// WrittenSDDFiles contains the absolute paths of SDD skill instruction
	// files whose managed advisor block was updated.
	WrittenSDDFiles []string

	// SkippedSDDFiles contains the absolute paths of SDD skill instruction
	// files that were skipped because they were missing, absent, or lacked
	// the devrune-advisors managed markers.
	SkippedSDDFiles []string

	// Warnings contains user-facing notices collected during the sync (e.g.
	// SDD skill files missing the advisor markers). These are propagated up
	// to AdvisorsSyncResult.Warnings so the TUI can display them after the
	// altscreen form has exited — never logged to stderr/stdout while the
	// TUI is active.
	Warnings []string
}

// SyncCatalogDocs re-renders the advisor-visible sections of the root catalog
// files (CLAUDE.md, AGENTS.md) and the advisor tables inside known SDD skill
// instruction files.
//
// It is called from SyncAdvisors AFTER all renderer calls succeed and BEFORE
// state.yaml is written. On error, state is not mutated.
//
// installedAdvisors is the canonical list — native + custom + catalog —
// AFTER the in-memory manifest diff has been applied.
//
// The function is idempotent: calling it twice with identical inputs produces
// byte-identical files on disk.
func SyncCatalogDocs(
	wd string,
	manifest model.UserManifest,
	installedAdvisors []model.ContentItem,
) (SyncCatalogDocsResult, error) {
	var result SyncCatalogDocsResult

	// Step 1: Re-render CLAUDE.md and AGENTS.md managed blocks.
	if err := syncRootCatalogFiles(wd, manifest, installedAdvisors, &result); err != nil {
		return result, fmt.Errorf("SyncCatalogDocs: root catalog sync: %w", err)
	}

	// Step 2: Splice advisor tables into SDD skill instruction files.
	if err := syncSDDSkillFiles(wd, installedAdvisors, &result); err != nil {
		return result, fmt.Errorf("SyncCatalogDocs: SDD skill sync: %w", err)
	}

	return result, nil
}

// syncRootCatalogFiles re-renders CLAUDE.md and AGENTS.md using
// renderers.RenderRootCatalog with the current set of all installed skills
// (read from the lockfile) merged with installedAdvisors.
func syncRootCatalogFiles(
	wd string,
	manifest model.UserManifest,
	installedAdvisors []model.ContentItem,
	result *SyncCatalogDocsResult,
) error {
	// Build the merged skills list from the lockfile + installedAdvisors.
	allSkills, allRules, err := readInstalledSkillsAndRules(wd, manifest)
	if err != nil {
		// Non-fatal for rules: if the lockfile is absent (first run before devrune install),
		// just use installedAdvisors alone. Skills is the critical set.
		allSkills = nil
		allRules = nil
	}

	// Merge installedAdvisors into allSkills (dedup by Name).
	allSkills = mergeSkills(allSkills, installedAdvisors)

	// Call RenderRootCatalog. For the narrow SyncAdvisors context we pass:
	//   - skills = allSkills (deduplicated existing + advisors)
	//   - rules  = allRules (from lockfile, may be empty)
	//   - workflows, mcpInstructions, registryContents = empty (preserved via
	//     the surrounding unmanaged content; only the managed block is replaced)
	catalog, err := renderers.RenderRootCatalog(allSkills, allRules, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("syncRootCatalogFiles: RenderRootCatalog: %w", err)
	}

	// Write CLAUDE.md managed block.
	claudePath := filepath.Join(wd, "CLAUDE.md")
	if err := renderers.WriteManagedBlock(claudePath, renderers.CatalogBeginMarker, renderers.CatalogEndMarker, catalog); err != nil {
		return fmt.Errorf("syncRootCatalogFiles: write CLAUDE.md: %w", err)
	}
	result.WrittenRootFiles = append(result.WrittenRootFiles, claudePath)

	// Write AGENTS.md managed block (same catalog content for non-Claude agents).
	agentsPath := filepath.Join(wd, "AGENTS.md")
	if err := renderers.WriteManagedBlock(agentsPath, renderers.CatalogBeginMarker, renderers.CatalogEndMarker, catalog); err != nil {
		return fmt.Errorf("syncRootCatalogFiles: write AGENTS.md: %w", err)
	}
	result.WrittenRootFiles = append(result.WrittenRootFiles, agentsPath)

	return nil
}

// syncSDDSkillFiles splices the advisor table between the managed marker pair
// (<!-- devrune advisors:begin --> / <!-- devrune advisors:end -->) in each
// of the known SDD skill instruction files.
//
// Files that do not exist or lack the markers are recorded in SkippedSDDFiles
// with a warning log. The function never returns an error for missing/unmarked
// files — only genuine IO or write failures are surfaced.
func syncSDDSkillFiles(
	wd string,
	installedAdvisors []model.ContentItem,
	result *SyncCatalogDocsResult,
) error {
	table := buildAdvisorTable(installedAdvisors)

	for _, rel := range sddSkillFiles {
		abs := filepath.Join(wd, rel)

		data, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				result.SkippedSDDFiles = append(result.SkippedSDDFiles, abs)
				continue
			}
			return fmt.Errorf("syncSDDSkillFiles: read %q: %w", abs, err)
		}

		content := string(data)
		beginIdx := strings.Index(content, advisorsBeginMarker)
		endIdx := strings.Index(content, advisorsEndMarker)

		if beginIdx < 0 || endIdx <= beginIdx {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("SDD skill %q missing devrune-advisors markers (run 'devrune install' to upgrade)", rel))
			result.SkippedSDDFiles = append(result.SkippedSDDFiles, abs)
			continue
		}

		// Splice the new advisor table between the markers.
		// Preserve content before begin marker and after end marker.
		before := content[:beginIdx+len(advisorsBeginMarker)]
		after := content[endIdx:] // includes the end marker

		// Build replacement: begin marker + newline + table + end marker section.
		var newContent strings.Builder
		newContent.WriteString(before)
		newContent.WriteString("\n")
		if table != "" {
			newContent.WriteString(table)
		}
		newContent.WriteString(after)

		updated := newContent.String()

		// Only write if the content actually changed (idempotency: avoid unnecessary
		// disk writes and mtime changes). Unchanged files are NOT added to
		// WrittenSDDFiles — they were verified but not written.
		if updated == content {
			continue
		}

		if err := atomicWriteFile(abs, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("syncSDDSkillFiles: write %q: %w", abs, err)
		}
		result.WrittenSDDFiles = append(result.WrittenSDDFiles, abs)
	}

	return nil
}

// buildAdvisorTable renders a markdown table of all installedAdvisors, sorted
// by name. The table shape matches the Skills table emitted by RenderRootCatalog
// so both surfaces read identically.
//
// Returns an empty string when installedAdvisors is empty.
func buildAdvisorTable(installedAdvisors []model.ContentItem) string {
	if len(installedAdvisors) == 0 {
		return ""
	}

	// Sort by name for deterministic output.
	sorted := make([]model.ContentItem, len(installedAdvisors))
	copy(sorted, installedAdvisors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var sb strings.Builder
	sb.WriteString("| Skill | Invocation | Use When |\n")
	sb.WriteString("|-------|------------|----------|\n")
	for _, a := range sorted {
		_, _ = fmt.Fprintf(&sb, "| `%s` | `/%s` | %s |\n", a.Name, a.Name, a.Description)
	}
	return sb.String()
}

// readInstalledSkillsAndRules reads the devrune.lock file from wd and
// returns the installed ContentItems separated into skills and rules.
// If the lockfile is absent or unreadable the function returns empty slices
// and a nil error — callers treat missing lockfile as "nothing installed yet".
func readInstalledSkillsAndRules(wd string, _ model.UserManifest) (skills []model.ContentItem, rules []model.ContentItem, err error) {
	lockPath := filepath.Join(wd, "devrune.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("readInstalledSkillsAndRules: read %q: %w", lockPath, err)
	}

	lock, err := parse.ParseLockfile(data)
	if err != nil {
		return nil, nil, fmt.Errorf("readInstalledSkillsAndRules: parse lockfile: %w", err)
	}

	seenSkill := make(map[string]bool)
	seenRule := make(map[string]bool)

	for _, pkg := range lock.Packages {
		for _, item := range pkg.Contents {
			switch item.Kind {
			case model.KindSkill:
				if !seenSkill[item.Name] {
					seenSkill[item.Name] = true
					skills = append(skills, item)
				}
			case model.KindRule:
				if !seenRule[item.Name] {
					seenRule[item.Name] = true
					rules = append(rules, item)
				}
			}
		}
	}

	return skills, rules, nil
}

// mergeSkills returns a deduplicated union of base and additions.
// Items in additions whose Name already appears in base are skipped.
// The relative order of base is preserved; additions are appended after.
func mergeSkills(base []model.ContentItem, additions []model.ContentItem) []model.ContentItem {
	if len(additions) == 0 {
		return base
	}

	seen := make(map[string]bool, len(base))
	for _, item := range base {
		seen[item.Name] = true
	}

	merged := make([]model.ContentItem, len(base), len(base)+len(additions))
	copy(merged, base)

	for _, item := range additions {
		if !seen[item.Name] {
			seen[item.Name] = true
			merged = append(merged, item)
		}
	}

	return merged
}

// atomicWriteFile writes data to path atomically by writing to a temporary file
// in the same directory and then renaming it. This ensures the file is never
// left in a partially-written state on POSIX systems.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".devrune-sync-*")
	if err != nil {
		return fmt.Errorf("atomicWriteFile: create temp: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure the temp file is cleaned up on failure.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomicWriteFile: write temp %q: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomicWriteFile: close temp %q: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("atomicWriteFile: chmod temp %q: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomicWriteFile: rename %q → %q: %w", tmpName, path, err)
	}

	committed = true
	return nil
}
