// SPDX-License-Identifier: MIT

package advisorcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// mkAdvisorDir creates <root>/<name>/SKILL.md with the given frontmatter fields.
// scope is the list of scope tags (e.g. []string{"frontend", "testing"}).
// An empty or nil scope slice means the advisor is universal — no scope line is
// written, matching the "omitted scope = universal" contract.
func mkAdvisorDir(t *testing.T, root, name, description string, scope []string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkAdvisorDir: mkdir %q: %v", dir, err)
	}
	frontmatter := "---\nname: " + name + "\n"
	if description != "" {
		frontmatter += "description: " + description + "\n"
	}
	if len(scope) > 0 {
		frontmatter += "scope: [" + strings.Join(scope, ", ") + "]\n"
	}
	frontmatter += "---\n\n# " + name + "\n"
	skillMD := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte(frontmatter), 0o644); err != nil {
		t.Fatalf("mkAdvisorDir: write SKILL.md: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DirScanner tests
// ─────────────────────────────────────────────────────────────────────────────

// TestDirScanner_FlatLayout_ThreeAdvisors verifies that scanning a root with
// 3 valid *-advisor subdirectories returns 3 entries sorted by name.
func TestDirScanner_FlatLayout_ThreeAdvisors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkAdvisorDir(t, root, "charlie-advisor", "Charlie description", []string{"backend"})
	mkAdvisorDir(t, root, "alpha-advisor", "Alpha description", []string{"frontend"})
	mkAdvisorDir(t, root, "bravo-advisor", "Bravo description", nil)

	scanner := DirScanner{}
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Scan returned %d entries, want 3", len(entries))
	}

	// Must be sorted by name.
	wantNames := []string{"alpha-advisor", "bravo-advisor", "charlie-advisor"}
	for i, want := range wantNames {
		if entries[i].Name != want {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, want)
		}
	}

	// Verify descriptions and paths.
	if entries[0].Description != "Alpha description" {
		t.Errorf("entries[0].Description = %q, want %q", entries[0].Description, "Alpha description")
	}
	wantScope := []string{"frontend"}
	if len(entries[0].Scope) != len(wantScope) || (len(wantScope) > 0 && entries[0].Scope[0] != wantScope[0]) {
		t.Errorf("entries[0].Scope = %v, want %v", entries[0].Scope, wantScope)
	}
	if entries[0].SKILLPath != filepath.Join(root, "alpha-advisor", "SKILL.md") {
		t.Errorf("entries[0].SKILLPath = %q, unexpected", entries[0].SKILLPath)
	}
	if entries[0].DirPath != filepath.Join(root, "alpha-advisor") {
		t.Errorf("entries[0].DirPath = %q, unexpected", entries[0].DirPath)
	}
}

// TestDirScanner_MixedEntries verifies the filtering rules:
// - valid *-advisor dirs with SKILL.md → included
// - subdir without -advisor suffix → skipped with warning log
// - valid-advisor dir without SKILL.md → skipped with warning log
// - loose file at root → silently ignored
func TestDirScanner_MixedEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Two valid advisors.
	mkAdvisorDir(t, root, "security-advisor", "Security checks", []string{"backend"})
	mkAdvisorDir(t, root, "testing-advisor", "Test patterns", nil)

	// Subdir without -advisor suffix — should be skipped with warning.
	badNameDir := filepath.Join(root, "utilities")
	if err := os.MkdirAll(badNameDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Add a SKILL.md inside — should still be skipped because name doesn't end in -advisor.
	if err := os.WriteFile(filepath.Join(badNameDir, "SKILL.md"), []byte("---\nname: utilities\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Valid -advisor dir without SKILL.md — should be skipped with warning.
	noSkillDir := filepath.Join(root, "no-skill-advisor")
	if err := os.MkdirAll(noSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Loose file at root — silently ignored.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Catalog\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	scanner := DirScanner{}
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Scan returned %d entries, want 2; entries: %v", len(entries), entries)
	}

	// Both valid advisors should be in the result.
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["security-advisor"] {
		t.Error("expected security-advisor in results")
	}
	if !names["testing-advisor"] {
		t.Error("expected testing-advisor in results")
	}
	// Invalid dirs should NOT be in results.
	if names["utilities"] {
		t.Error("utilities (no -advisor suffix) should have been skipped")
	}
	if names["no-skill-advisor"] {
		t.Error("no-skill-advisor (missing SKILL.md) should have been skipped")
	}
}

// TestDirScanner_NestedLayout_NotScanned verifies that nested layouts
// (e.g., backend/security-advisor/SKILL.md) are NOT scanned in v1 — only
// one level of directory depth is traversed.
func TestDirScanner_NestedLayout_NotScanned(t *testing.T) {
	t.Skip("nested layouts are v2 — only flat one-level scanning is supported in v1")
}

// TestDirScanner_FrontmatterMissingDescription_EntryStillReturned verifies that
// an advisor with a SKILL.md that lacks a description field is still returned as
// an entry (with empty Description), not skipped.
func TestDirScanner_FrontmatterMissingDescription_EntryStillReturned(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create advisor with no description in frontmatter.
	advisorDir := filepath.Join(root, "minimal-advisor")
	if err := os.MkdirAll(advisorDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Frontmatter only has name, no description.
	skillMD := "---\nname: minimal-advisor\n---\n\n# Minimal\n"
	if err := os.WriteFile(filepath.Join(advisorDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	scanner := DirScanner{}
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Scan returned %d entries, want 1", len(entries))
	}
	if entries[0].Name != "minimal-advisor" {
		t.Errorf("entries[0].Name = %q, want 'minimal-advisor'", entries[0].Name)
	}
	if entries[0].Description != "" {
		t.Errorf("entries[0].Description = %q, want empty string when frontmatter lacks description", entries[0].Description)
	}
}

// TestDirScanner_EmptyRoot_ReturnsEmpty verifies that scanning an empty
// directory returns an empty (non-nil) slice without error.
func TestDirScanner_EmptyRoot_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	scanner := DirScanner{}
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned unexpected error for empty root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Scan returned %d entries for empty root, want 0", len(entries))
	}
}

// TestDirScanner_NonExistentRoot_ReturnsError verifies that scanning a
// non-existent root returns an error.
func TestDirScanner_NonExistentRoot_ReturnsError(t *testing.T) {
	t.Parallel()

	scanner := DirScanner{}
	_, err := scanner.Scan("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("Scan expected error for non-existent root, got nil")
	}
}

// TestDirScanner_Sorted_AlphaOrder verifies that entries returned by Scan
// are always sorted alphabetically regardless of filesystem order.
func TestDirScanner_Sorted_AlphaOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Create in reverse alphabetical order.
	mkAdvisorDir(t, root, "zz-advisor", "Z", nil)
	mkAdvisorDir(t, root, "aa-advisor", "A", nil)
	mkAdvisorDir(t, root, "mm-advisor", "M", nil)

	scanner := DirScanner{}
	entries, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Scan returned %d entries, want 3", len(entries))
	}

	if entries[0].Name != "aa-advisor" {
		t.Errorf("entries[0] = %q, want 'aa-advisor'", entries[0].Name)
	}
	if entries[1].Name != "mm-advisor" {
		t.Errorf("entries[1] = %q, want 'mm-advisor'", entries[1].Name)
	}
	if entries[2].Name != "zz-advisor" {
		t.Errorf("entries[2] = %q, want 'zz-advisor'", entries[2].Name)
	}
}
