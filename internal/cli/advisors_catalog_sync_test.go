// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// anAdvisorItem builds a minimal model.ContentItem suitable for use as an
// installed advisor in SyncCatalogDocs tests.
func anAdvisorItem(name, description string) model.ContentItem {
	return model.ContentItem{
		Kind:        model.KindSkill,
		Name:        name,
		Path:        "skills/" + name + "/",
		Description: description,
	}
}

// writeFile creates intermediate directories and writes data to path.
func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("writeFile: mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writeFile: write %q: %v", path, err)
	}
}

// readFile reads a file and returns its content as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile: %q: %v", path, err)
	}
	return string(data)
}

// containsPath reports whether absPath appears in the paths slice.
func containsPath(paths []string, absPath string) bool {
	for _, p := range paths {
		if p == absPath {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// T1 — Root files are NOT touched
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_DoesNotTouchRootFiles verifies that SyncCatalogDocs no
// longer rewrites CLAUDE.md / AGENTS.md (the Skills table that motivated this
// sync was removed from the root catalog renderer; only `devrune sync` rebuilds
// root catalog content now).
func TestSyncCatalogDocs_DoesNotTouchRootFiles(t *testing.T) {
	wd := t.TempDir()

	advisors := []model.ContentItem{
		anAdvisorItem("alpha", "Advisor alpha"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	if len(result.WrittenRootFiles) != 0 {
		t.Errorf("WrittenRootFiles must be empty; got %v", result.WrittenRootFiles)
	}

	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(wd, name)); err == nil {
			t.Errorf("%s should not have been created by SyncCatalogDocs", name)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T2 — SDD skill file happy path
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_SDDSkillHappyPath verifies that when an SDD skill
// instruction file contains the devrune advisors markers, SyncCatalogDocs
// replaces the managed block with a sorted table of all installed advisors.
func TestSyncCatalogDocs_SDDSkillHappyPath(t *testing.T) {
	wd := t.TempDir()

	// Pick one of the known SDD skill files to seed.
	relPath := filepath.Join(".claude", "skills", "sdd-plan", "SKILL.md")
	absPath := filepath.Join(wd, relPath)

	placeholder := advisorsBeginMarker + "\n" +
		"<!-- no advisors installed -->\n" +
		advisorsEndMarker

	sddContent := "# SDD Plan\n\nSome instructions.\n\n" + placeholder + "\n\nMore content.\n"
	writeFile(t, absPath, sddContent)

	advisors := []model.ContentItem{
		anAdvisorItem("charlie", "Advisor charlie"),
		anAdvisorItem("alice", "Advisor alice"),
		anAdvisorItem("bob", "Advisor bob"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	if !containsPath(result.WrittenSDDFiles, absPath) {
		t.Errorf("WrittenSDDFiles does not contain %s; got %v", relPath, result.WrittenSDDFiles)
	}

	content := readFile(t, absPath)

	// Table must contain all 3 advisors.
	for _, name := range []string{"alice", "bob", "charlie"} {
		if !strings.Contains(content, name) {
			t.Errorf("SDD skill file missing advisor %q; content:\n%s", name, content)
		}
	}

	// Table must be sorted: alice before bob before charlie.
	aliceIdx := strings.Index(content, "alice")
	bobIdx := strings.Index(content, "bob")
	charlieIdx := strings.Index(content, "charlie")

	if aliceIdx >= bobIdx || bobIdx >= charlieIdx {
		t.Errorf("advisors not sorted alphabetically: alice@%d bob@%d charlie@%d", aliceIdx, bobIdx, charlieIdx)
	}

	// Surrounding unmanaged content must be preserved.
	if !strings.Contains(content, "# SDD Plan") {
		t.Errorf("SDD skill file: header content lost; content:\n%s", content)
	}
	if !strings.Contains(content, "More content.") {
		t.Errorf("SDD skill file: trailing content lost; content:\n%s", content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T3 — SDD skill file missing markers
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_SDDSkillMissingMarkers verifies that when an SDD skill
// file exists but lacks the devrune-advisors marker pair, the file is NOT
// modified and the path is recorded in SkippedSDDFiles.
func TestSyncCatalogDocs_SDDSkillMissingMarkers(t *testing.T) {
	wd := t.TempDir()

	relPath := filepath.Join(".claude", "skills", "sdd-plan", "SKILL.md")
	absPath := filepath.Join(wd, relPath)

	originalContent := "# SDD Plan\n\nThis file has no managed markers at all.\n"
	writeFile(t, absPath, originalContent)

	advisors := []model.ContentItem{
		anAdvisorItem("any-advisor", "Any advisor"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	if !containsPath(result.SkippedSDDFiles, absPath) {
		t.Errorf("SkippedSDDFiles does not contain %s; got %v", relPath, result.SkippedSDDFiles)
	}

	// The file must be byte-identical to the original.
	gotContent := readFile(t, absPath)
	if gotContent != originalContent {
		t.Errorf("file was modified but should have been skipped:\nwant:\n%s\ngot:\n%s", originalContent, gotContent)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T4 — SDD skill file missing entirely
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_SDDSkillMissingFile verifies that a missing SDD skill
// instruction file is silently skipped with its path recorded in SkippedSDDFiles
// and no error is returned.
func TestSyncCatalogDocs_SDDSkillMissingFile(t *testing.T) {
	wd := t.TempDir()

	// Deliberately do NOT create sdd-review/SKILL.md.
	expectedSkipped := filepath.Join(wd, ".claude", "skills", "sdd-review", "SKILL.md")

	advisors := []model.ContentItem{
		anAdvisorItem("some-advisor", "Some advisor"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	if !containsPath(result.SkippedSDDFiles, expectedSkipped) {
		t.Errorf("SkippedSDDFiles should contain %q; got %v", expectedSkipped, result.SkippedSDDFiles)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T5 — Idempotency (SDD skill file)
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_Idempotency verifies that calling SyncCatalogDocs twice
// with identical inputs produces the same bytes on disk for the SDD skill file.
func TestSyncCatalogDocs_Idempotency(t *testing.T) {
	wd := t.TempDir()

	relPath := filepath.Join(".claude", "skills", "sdd-plan", "SKILL.md")
	absPath := filepath.Join(wd, relPath)

	sddContent := "# SDD Plan\n\n" +
		advisorsBeginMarker + "\n" +
		advisorsEndMarker + "\n"
	writeFile(t, absPath, sddContent)

	advisors := []model.ContentItem{
		anAdvisorItem("delta", "Delta advisor"),
		anAdvisorItem("echo", "Echo advisor"),
	}

	manifest := AUserManifest().Build()

	if _, err := SyncCatalogDocs(wd, manifest, advisors); err != nil {
		t.Fatalf("SyncCatalogDocs (run 1) returned error: %v", err)
	}
	sddAfterRun1 := readFile(t, absPath)

	if _, err := SyncCatalogDocs(wd, manifest, advisors); err != nil {
		t.Fatalf("SyncCatalogDocs (run 2) returned error: %v", err)
	}
	if got := readFile(t, absPath); got != sddAfterRun1 {
		t.Errorf("sdd-plan/SKILL.md not idempotent:\nrun1:\n%s\nrun2:\n%s", sddAfterRun1, got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T6 — Removed advisor
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_RemovedAdvisor verifies that when the SDD skill file
// previously listed 3 advisors in the managed block, running SyncCatalogDocs
// with only 2 advisors installed results in a block that lists exactly those 2.
func TestSyncCatalogDocs_RemovedAdvisor(t *testing.T) {
	wd := t.TempDir()

	relPath := filepath.Join(".claude", "skills", "sdd-plan", "SKILL.md")
	absPath := filepath.Join(wd, relPath)

	// Seed with 3 advisors in the managed block.
	oldBlock := advisorsBeginMarker + "\n" +
		"| Skill | Invocation | Use When |\n" +
		"|-------|------------|----------|\n" +
		"| `advisor-a` | `/advisor-a` | A |\n" +
		"| `advisor-b` | `/advisor-b` | B |\n" +
		"| `advisor-c` | `/advisor-c` | C |\n" +
		advisorsEndMarker

	writeFile(t, absPath, "# Plan\n\n"+oldBlock+"\n")

	// Only 2 advisors remain after removal of advisor-c.
	advisors := []model.ContentItem{
		anAdvisorItem("advisor-a", "Advisor A"),
		anAdvisorItem("advisor-b", "Advisor B"),
	}

	manifest := AUserManifest().Build()
	_, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	content := readFile(t, absPath)

	// advisor-c must be gone.
	if strings.Contains(content, "advisor-c") {
		t.Errorf("advisor-c should have been removed from the managed block; content:\n%s", content)
	}

	// advisor-a and advisor-b must still be present.
	for _, name := range []string{"advisor-a", "advisor-b"} {
		if !strings.Contains(content, name) {
			t.Errorf("advisor %q should still be in the managed block; content:\n%s", name, content)
		}
	}
}
