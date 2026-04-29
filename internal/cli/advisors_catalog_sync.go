// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/davidarce/devrune/internal/model"
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
	// WrittenRootFiles is retained for backward compatibility with downstream
	// consumers (e.g. SyncAdvisors result aggregation) but is always nil:
	// SyncCatalogDocs no longer rewrites CLAUDE.md / AGENTS.md, since the root
	// catalog no longer carries an advisor-driven Skills table that needs
	// per-advisor sync.
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

// SyncCatalogDocs splices the advisor table into known SDD skill instruction
// files (sdd-plan, sdd-review, sdd-orchestrator).
//
// It is called from SyncAdvisors AFTER all renderer calls succeed and BEFORE
// state.yaml is written. On error, state is not mutated.
//
// installedAdvisors is the canonical list — native + custom + catalog —
// AFTER the in-memory manifest diff has been applied.
//
// The function is idempotent: calling it twice with identical inputs produces
// byte-identical files on disk.
//
// The root catalog files (CLAUDE.md, AGENTS.md) are NOT touched: the
// agent-discoverable Skills table was removed from RenderRootCatalog, so
// there is nothing for an advisor add/remove to refresh in the managed block.
// Run `devrune sync` to rebuild root catalog content (workflows + MCPs).
func SyncCatalogDocs(
	wd string,
	manifest model.UserManifest,
	installedAdvisors []model.ContentItem,
) (SyncCatalogDocsResult, error) {
	_ = manifest // retained in signature for backward compatibility with callers
	var result SyncCatalogDocsResult

	if err := syncSDDSkillFiles(wd, installedAdvisors, &result); err != nil {
		return result, fmt.Errorf("SyncCatalogDocs: SDD skill sync: %w", err)
	}

	return result, nil
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
// by name. The table shape is the canonical advisor table shape used inside
// SDD skill instruction files.
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
