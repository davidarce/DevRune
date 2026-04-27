// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
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
// T1 — CLAUDE.md + AGENTS.md round-trip
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_RootFilesRoundTrip verifies that running SyncCatalogDocs
// on a tmpdir that already has CLAUDE.md and AGENTS.md with a managed block
// (listing 3 skills) replaces the managed block to reflect 2 installed advisors
// while leaving unmanaged content above and below the block byte-for-byte.
func TestSyncCatalogDocs_RootFilesRoundTrip(t *testing.T) {
	wd := t.TempDir()

	// Build a file that has unmanaged content above and below the managed block.
	prefix := "# My project notes\n\nThis is not managed.\n\n"
	suffix := "\n## Other section\n\nMore unmanaged content.\n"

	managed := renderers.CatalogBeginMarker + "\n" +
		"# Agent Catalog\n\n" +
		"## Skills\n\n" +
		"| Skill | Invocation | Use When |\n" +
		"|-------|------------|----------|\n" +
		"| `alpha` | `/alpha` | Skill A |\n" +
		"| `beta` | `/beta` | Skill B |\n" +
		"| `gamma` | `/gamma` | Skill C |\n" +
		renderers.CatalogEndMarker + "\n"

	fullContent := prefix + managed + suffix

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), fullContent)
	writeFile(t, filepath.Join(wd, "AGENTS.md"), fullContent)

	advisors := []model.ContentItem{
		anAdvisorItem("alpha", "Advisor alpha"),
		anAdvisorItem("beta", "Advisor beta"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	// Both root files must be listed in WrittenRootFiles.
	claudePath := filepath.Join(wd, "CLAUDE.md")
	agentsPath := filepath.Join(wd, "AGENTS.md")

	if !containsPath(result.WrittenRootFiles, claudePath) {
		t.Errorf("WrittenRootFiles does not contain CLAUDE.md; got %v", result.WrittenRootFiles)
	}
	if !containsPath(result.WrittenRootFiles, agentsPath) {
		t.Errorf("WrittenRootFiles does not contain AGENTS.md; got %v", result.WrittenRootFiles)
	}

	for _, path := range []string{claudePath, agentsPath} {
		content := readFile(t, path)

		// Unmanaged prefix must be preserved.
		if !strings.HasPrefix(content, prefix) {
			t.Errorf("%s: unmanaged prefix not preserved; got:\n%s", filepath.Base(path), content)
		}

		// Unmanaged suffix must be preserved.
		if !strings.HasSuffix(content, suffix) {
			t.Errorf("%s: unmanaged suffix not preserved; got:\n%s", filepath.Base(path), content)
		}

		// The managed block must now list only 2 advisors (alpha, beta), not 3.
		if strings.Contains(content, "gamma") {
			t.Errorf("%s: gamma advisor should have been removed from the managed block", filepath.Base(path))
		}
		if !strings.Contains(content, "alpha") {
			t.Errorf("%s: alpha advisor missing from the managed block", filepath.Base(path))
		}
		if !strings.Contains(content, "beta") {
			t.Errorf("%s: beta advisor missing from the managed block", filepath.Base(path))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T2 — CLAUDE.md missing file
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_CLAUDEMDMissing verifies that when CLAUDE.md does not
// exist, SyncCatalogDocs creates it and places the managed block inside.
func TestSyncCatalogDocs_CLAUDEMDMissing(t *testing.T) {
	wd := t.TempDir()

	// CLAUDE.md intentionally absent; AGENTS.md also absent.

	advisors := []model.ContentItem{
		anAdvisorItem("my-advisor", "Does something useful"),
	}

	manifest := AUserManifest().Build()
	result, err := SyncCatalogDocs(wd, manifest, advisors)
	if err != nil {
		t.Fatalf("SyncCatalogDocs returned error: %v", err)
	}

	claudePath := filepath.Join(wd, "CLAUDE.md")
	if !containsPath(result.WrittenRootFiles, claudePath) {
		t.Errorf("WrittenRootFiles does not contain CLAUDE.md; got %v", result.WrittenRootFiles)
	}

	content := readFile(t, claudePath)

	if !strings.Contains(content, renderers.CatalogBeginMarker) {
		t.Errorf("CLAUDE.md missing CatalogBeginMarker; content:\n%s", content)
	}
	if !strings.Contains(content, renderers.CatalogEndMarker) {
		t.Errorf("CLAUDE.md missing CatalogEndMarker; content:\n%s", content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T3 — SDD skill file happy path
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
// T4 — SDD skill file missing markers
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
// T5 — SDD skill file missing entirely
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
// T6 — Idempotency
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncCatalogDocs_Idempotency verifies that calling SyncCatalogDocs twice
// with identical inputs produces the same bytes on disk.
func TestSyncCatalogDocs_Idempotency(t *testing.T) {
	wd := t.TempDir()

	// Seed the SDD skill file with markers so it participates in both runs.
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

	// First run.
	if _, err := SyncCatalogDocs(wd, manifest, advisors); err != nil {
		t.Fatalf("SyncCatalogDocs (run 1) returned error: %v", err)
	}

	// Capture disk state after first run.
	claudeAfterRun1 := readFile(t, filepath.Join(wd, "CLAUDE.md"))
	agentsAfterRun1 := readFile(t, filepath.Join(wd, "AGENTS.md"))
	sddAfterRun1 := readFile(t, absPath)

	// Second run.
	if _, err := SyncCatalogDocs(wd, manifest, advisors); err != nil {
		t.Fatalf("SyncCatalogDocs (run 2) returned error: %v", err)
	}

	// Files must be byte-identical.
	if got := readFile(t, filepath.Join(wd, "CLAUDE.md")); got != claudeAfterRun1 {
		t.Errorf("CLAUDE.md not idempotent:\nrun1:\n%s\nrun2:\n%s", claudeAfterRun1, got)
	}
	if got := readFile(t, filepath.Join(wd, "AGENTS.md")); got != agentsAfterRun1 {
		t.Errorf("AGENTS.md not idempotent:\nrun1:\n%s\nrun2:\n%s", agentsAfterRun1, got)
	}
	if got := readFile(t, absPath); got != sddAfterRun1 {
		t.Errorf("sdd-plan/SKILL.md not idempotent:\nrun1:\n%s\nrun2:\n%s", sddAfterRun1, got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T7 — Removed advisor
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
