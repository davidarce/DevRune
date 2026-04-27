// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidarce/devrune/internal/advisorcatalog"
	"github.com/davidarce/devrune/internal/advisormeta"
	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// catalog test helpers
// ─────────────────────────────────────────────────────────────────────────────

// seedCatalogAdvisor creates a catalog advisor directory with SKILL.md and an
// optional references/ subdirectory under catalogRoot.
func seedCatalogAdvisor(t *testing.T, catalogRoot, name, description string, withReferences bool) {
	t.Helper()
	dir := filepath.Join(catalogRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seedCatalogAdvisor: mkdir %q: %v", dir, err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("seedCatalogAdvisor: write SKILL.md: %v", err)
	}
	if withReferences {
		refDir := filepath.Join(dir, "references")
		if err := os.MkdirAll(refDir, 0o755); err != nil {
			t.Fatalf("seedCatalogAdvisor: mkdir references: %v", err)
		}
		if err := os.WriteFile(filepath.Join(refDir, "guide.md"), []byte("# Guide\n"), 0o644); err != nil {
			t.Fatalf("seedCatalogAdvisor: write guide.md: %v", err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// fetchCatalogSource tests
// ─────────────────────────────────────────────────────────────────────────────

// TestFetchCatalogSource_Local_ExistingDir verifies that fetchCatalogSource
// with a local: URL resolves the directory and returns its absolute path.
func TestFetchCatalogSource_Local_ExistingDir(t *testing.T) {
	wd := t.TempDir()
	catalogDir := filepath.Join(wd, "my-catalog")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	src := model.CatalogSource{URL: "local:" + catalogDir}
	got, err := fetchCatalogSource(context.Background(), wd, src)
	if err != nil {
		t.Fatalf("fetchCatalogSource returned error: %v", err)
	}
	if got != catalogDir {
		t.Errorf("fetchCatalogSource returned %q, want %q", got, catalogDir)
	}
}

// TestFetchCatalogSource_Local_NonExistent_ReturnsError verifies that a
// non-existent local: path returns an error.
func TestFetchCatalogSource_Local_NonExistent_ReturnsError(t *testing.T) {
	wd := t.TempDir()
	src := model.CatalogSource{URL: "local:/nonexistent/path/that/does/not/exist"}

	_, err := fetchCatalogSource(context.Background(), wd, src)
	if err == nil {
		t.Fatal("fetchCatalogSource expected error for non-existent path, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// filterAvailableEntries tests
// ─────────────────────────────────────────────────────────────────────────────

// TestFilterAvailableEntries_CollisionWithNative verifies that entries whose
// names match a native advisor are filtered out.
func TestFilterAvailableEntries_CollisionWithNative(t *testing.T) {
	// Use a known native advisor name.
	nativeAdvisorName := "unit-test-advisor"

	entries := []advisorcatalog.CatalogEntry{
		{Name: nativeAdvisorName, Description: "Collides with native"},
		{Name: "custom-new-advisor", Description: "New and custom"},
	}

	manifest := AUserManifest().Build()
	filtered := filterAvailableEntries(entries, &manifest)

	if len(filtered) != 1 {
		t.Fatalf("filterAvailableEntries returned %d entries, want 1; got: %v", len(filtered), filtered)
	}
	if filtered[0].Name != "custom-new-advisor" {
		t.Errorf("filtered[0].Name = %q, want 'custom-new-advisor'", filtered[0].Name)
	}
}

// TestFilterAvailableEntries_CollisionWithRegistered verifies that entries
// already registered in manifest.CustomAdvisors are filtered out.
func TestFilterAvailableEntries_CollisionWithRegistered(t *testing.T) {
	existingDef := AnAdvisorDef().Named("already-registered-advisor").Build()

	entries := []advisorcatalog.CatalogEntry{
		{Name: "already-registered-advisor", Description: "Already registered"},
		{Name: "fresh-advisor", Description: "Not yet registered"},
	}

	manifest := AUserManifest().WithCustom(existingDef).Build()
	filtered := filterAvailableEntries(entries, &manifest)

	if len(filtered) != 1 {
		t.Fatalf("filterAvailableEntries returned %d entries, want 1; got: %v", len(filtered), filtered)
	}
	if filtered[0].Name != "fresh-advisor" {
		t.Errorf("filtered[0].Name = %q, want 'fresh-advisor'", filtered[0].Name)
	}
}

// TestFilterAvailableEntries_AllAllowed verifies that entries with unique names
// (no collisions) all pass through.
func TestFilterAvailableEntries_AllAllowed(t *testing.T) {
	entries := []advisorcatalog.CatalogEntry{
		{Name: "zz-custom-advisor", Description: "Custom A"},
		{Name: "yy-custom-advisor", Description: "Custom B"},
	}

	manifest := AUserManifest().Build()
	filtered := filterAvailableEntries(entries, &manifest)

	if len(filtered) != 2 {
		t.Errorf("filterAvailableEntries returned %d entries, want 2", len(filtered))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// advisorBelongsToCatalog tests
// ─────────────────────────────────────────────────────────────────────────────

// TestAdvisorBelongsToCatalog_ExactURLMatch verifies that an advisor with
// SkillSource == catalog URL is attributed to that catalog.
func TestAdvisorBelongsToCatalog_ExactURLMatch(t *testing.T) {
	wd := t.TempDir()
	cat := ACatalogSource().WithURL("github:acme/catalog@main").Build()
	def := AnAdvisorDef().Named("my-advisor").WithSkillSource("github:acme/catalog@main").Build()
	all := []model.CatalogSource{cat}

	if !advisorBelongsToCatalog(def, cat, all, wd) {
		t.Error("expected advisor to belong to catalog (exact URL match)")
	}
}

// TestAdvisorBelongsToCatalog_PathPrefixMatch verifies that an advisor whose
// SkillSource is a path inside the catalog's cache directory is attributed to that catalog.
func TestAdvisorBelongsToCatalog_PathPrefixMatch(t *testing.T) {
	wd := t.TempDir()
	cat := ACatalogSource().WithURL("github:acme/catalog@main").Build()

	// Compute the cache key for this catalog URL and build a path inside it.
	cacheKey := advisorcatalog.CacheKey(cat)
	cacheDir := filepath.Join(wd, ".devrune", "advisor-catalogs", cacheKey)
	skillSrc := filepath.Join(cacheDir, "my-advisor")

	def := AnAdvisorDef().Named("my-advisor").WithSkillSource(skillSrc).Build()
	all := []model.CatalogSource{cat}

	if !advisorBelongsToCatalog(def, cat, all, wd) {
		t.Errorf("expected advisor to belong to catalog via path-prefix match; cacheDir=%q skillSrc=%q", cacheDir, skillSrc)
	}
}

// TestAdvisorBelongsToCatalog_SingleCatalog_LocalOrigin_NoHeuristic verifies
// that a local-origin advisor is NOT attributed to a github: catalog even when
// only one catalog is registered (the old single-catalog heuristic is removed).
func TestAdvisorBelongsToCatalog_SingleCatalog_LocalOrigin_NoHeuristic(t *testing.T) {
	wd := t.TempDir()
	cat := ACatalogSource().WithURL("github:acme/catalog@main").Build()
	// SkillSource is a local path, not the catalog URL and not under the cache dir.
	def := AnAdvisorDef().Named("my-advisor").WithSkillSource(filepath.Join(wd, "local", "my-advisor")).Build()
	all := []model.CatalogSource{cat}

	if advisorBelongsToCatalog(def, cat, all, wd) {
		t.Error("local-origin advisor should NOT be attributed to a github: catalog")
	}
}

// TestAdvisorBelongsToCatalog_MultipleCatalogs_NoMatch verifies that when
// multiple catalogs are registered and the SkillSource doesn't match, the
// advisor is NOT attributed to the catalog.
func TestAdvisorBelongsToCatalog_MultipleCatalogs_NoMatch(t *testing.T) {
	wd := t.TempDir()
	cat1 := ACatalogSource().WithURL("github:acme/catalog@main").Build()
	cat2 := ACatalogSource().WithURL("github:other/catalog@main").Build()
	def := AnAdvisorDef().Named("my-advisor").WithSkillSource("github:other/catalog@main").Build()
	all := []model.CatalogSource{cat1, cat2}

	// def.SkillSource matches cat2, not cat1.
	if advisorBelongsToCatalog(def, cat1, all, wd) {
		t.Error("advisor should NOT belong to cat1 (SkillSource matches cat2 instead)")
	}
	if !advisorBelongsToCatalog(def, cat2, all, wd) {
		t.Error("advisor SHOULD belong to cat2 (exact SkillSource match)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// fileSHA256 tests
// ─────────────────────────────────────────────────────────────────────────────

// TestFileSHA256_SameContentSameHash verifies that the same content produces
// the same hash.
func TestFileSHA256_SameContentSameHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	content := []byte("# Hello\n\nSame content.\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h1 := fileSHA256(path)
	h2 := fileSHA256(path)
	if h1 == "" {
		t.Fatal("fileSHA256 returned empty string for existing file")
	}
	if h1 != h2 {
		t.Errorf("fileSHA256 is not deterministic: h1=%q h2=%q", h1, h2)
	}
}

// TestFileSHA256_DifferentContentDifferentHash verifies that different content
// produces different hashes.
func TestFileSHA256_DifferentContentDifferentHash(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")
	fileB := filepath.Join(dir, "b.md")
	if err := os.WriteFile(fileA, []byte("content A"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("content B"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	hA := fileSHA256(fileA)
	hB := fileSHA256(fileB)
	if hA == hB {
		t.Errorf("fileSHA256 produced same hash %q for different files", hA)
	}
}

// TestFileSHA256_MissingFile_ReturnsEmpty verifies that a missing file
// returns an empty string (not a panic or error).
func TestFileSHA256_MissingFile_ReturnsEmpty(t *testing.T) {
	h := fileSHA256("/nonexistent/file/that/does/not/exist.md")
	if h != "" {
		t.Errorf("fileSHA256 returned %q for non-existent file, want empty string", h)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSingleAdvisorDir tests
// ─────────────────────────────────────────────────────────────────────────────

// TestIsSingleAdvisorDir_ValidAdvisorDir verifies that a directory ending in
// "-advisor" that contains SKILL.md is detected as a single-advisor directory.
func TestIsSingleAdvisorDir_ValidAdvisorDir(t *testing.T) {
	dir := t.TempDir()
	advisorDir := filepath.Join(dir, "security-advisor")
	if err := os.MkdirAll(advisorDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(advisorDir, "SKILL.md"), []byte("---\nname: security-advisor\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !isSingleAdvisorDir(advisorDir) {
		t.Error("isSingleAdvisorDir should return true for directory ending in -advisor with SKILL.md")
	}
}

// TestIsSingleAdvisorDir_NoBadSuffix verifies that a directory not ending in
// "-advisor" returns false.
func TestIsSingleAdvisorDir_NoBadSuffix(t *testing.T) {
	dir := t.TempDir()
	if !strings.HasSuffix(dir, "-advisor") {
		// dir is a tmpdir that doesn't end in -advisor — should return false.
		if isSingleAdvisorDir(dir) {
			t.Error("isSingleAdvisorDir should return false for directory not ending in -advisor")
		}
	}
}

// TestIsSingleAdvisorDir_MissingSkillMD verifies that a directory ending in
// "-advisor" but without SKILL.md returns false.
func TestIsSingleAdvisorDir_MissingSkillMD(t *testing.T) {
	dir := t.TempDir()
	advisorDir := filepath.Join(dir, "security-advisor")
	if err := os.MkdirAll(advisorDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No SKILL.md created.

	if isSingleAdvisorDir(advisorDir) {
		t.Error("isSingleAdvisorDir should return false when SKILL.md is missing")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// runRefreshCatalogsFlow integration tests
// ─────────────────────────────────────────────────────────────────────────────

// TestRunRefreshCatalogsFlow_NoCatalogsRegistered verifies that calling
// runRefreshCatalogsFlow with an empty AdvisorCatalogs returns an error.
func TestRunRefreshCatalogsFlow_NoCatalogsRegistered(t *testing.T) {
	wd := t.TempDir()
	manifest := AUserManifest().Build() // no catalogs

	_, err := runRefreshCatalogsFlow(context.Background(), wd, &manifest)
	if err == nil {
		t.Fatal("runRefreshCatalogsFlow expected error when no catalogs are registered, got nil")
	}
	if !strings.Contains(err.Error(), "no advisor catalogs registered") {
		t.Errorf("error should mention 'no advisor catalogs registered'; got: %v", err)
	}
}

// TestRunRefreshCatalogsFlow_LocalCatalog_ContentUpdate verifies that after
// a SKILL.md is modified in the source catalog, calling runRefreshCatalogsFlow
// updates the installed copy and sets LastFetched.
func TestRunRefreshCatalogsFlow_LocalCatalog_ContentUpdate(t *testing.T) {
	wd := t.TempDir()

	// Create catalog root with one advisor.
	catalogRoot := t.TempDir()
	seedCatalogAdvisor(t, catalogRoot, "refresh-advisor", "Initial description", true)

	// Install the advisor manually (simulating a previous import).
	advisorDst := filepath.Join(wd, ".claude", "skills", "refresh-advisor")
	if err := os.MkdirAll(advisorDst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initialSKILLContent := "---\nname: refresh-advisor\ndescription: Initial description\n---\n\n# Initial\n"
	if err := os.WriteFile(filepath.Join(advisorDst, "SKILL.md"), []byte(initialSKILLContent), 0o644); err != nil {
		t.Fatalf("WriteFile (installed SKILL.md): %v", err)
	}

	catalogURL := "local:" + catalogRoot
	cat := ACatalogSource().WithURL(catalogURL).Build()
	def := AnAdvisorDef().
		Named("refresh-advisor").
		WithSkillSource(catalogURL).
		WithOrigin(model.AdvisorOriginCatalog).
		WithDescription("Initial description").
		Build()

	manifest := AUserManifest().
		WithCatalog(cat).
		WithCustom(def).
		Build()

	// Now modify the source SKILL.md to have different content.
	updatedContent := "---\nname: refresh-advisor\ndescription: Updated description\n---\n\n# Updated\n"
	srcSKILLPath := filepath.Join(catalogRoot, "refresh-advisor", "SKILL.md")
	if err := os.WriteFile(srcSKILLPath, []byte(updatedContent), 0o644); err != nil {
		t.Fatalf("WriteFile (updated src SKILL.md): %v", err)
	}

	before := time.Now().UTC().Truncate(time.Second)
	_, err := runRefreshCatalogsFlow(context.Background(), wd, &manifest)
	if err != nil {
		t.Fatalf("runRefreshCatalogsFlow returned error: %v", err)
	}

	// Installed SKILL.md must contain the updated content.
	installedData, readErr := os.ReadFile(filepath.Join(advisorDst, "SKILL.md"))
	if readErr != nil {
		t.Fatalf("ReadFile (installed SKILL.md): %v", readErr)
	}
	if !strings.Contains(string(installedData), "Updated description") {
		t.Errorf("installed SKILL.md should contain updated content; got:\n%s", string(installedData))
	}

	// LastFetched must have been updated on the catalog entry.
	if manifest.AdvisorCatalogs[0].LastFetched == "" {
		t.Error("catalog LastFetched should be set after refresh")
	}
	// Parse the timestamp to verify it's a valid RFC3339 timestamp at or after our before marker.
	ts, parseErr := time.Parse(time.RFC3339, manifest.AdvisorCatalogs[0].LastFetched)
	if parseErr != nil {
		t.Errorf("LastFetched %q is not a valid RFC3339 timestamp: %v", manifest.AdvisorCatalogs[0].LastFetched, parseErr)
	} else if ts.UTC().Before(before) {
		t.Errorf("LastFetched %q (UTC: %v) should be >= before marker %v", manifest.AdvisorCatalogs[0].LastFetched, ts.UTC(), before)
	}
}

// TestRunRefreshCatalogsFlow_LocalCatalog_NoChange_SkipsRecopy verifies that
// when source SKILL.md is identical to the installed copy, the file is not
// re-copied (content stays the same).
func TestRunRefreshCatalogsFlow_LocalCatalog_NoChange_SkipsRecopy(t *testing.T) {
	wd := t.TempDir()
	catalogRoot := t.TempDir()

	skillContent := "---\nname: stable-advisor\ndescription: Stable\n---\n\n# Stable\n"
	seedCatalogAdvisor(t, catalogRoot, "stable-advisor", "Stable", false)

	// Install with same content.
	advisorDst := filepath.Join(wd, ".claude", "skills", "stable-advisor")
	if err := os.MkdirAll(advisorDst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(advisorDst, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Overwrite source to match exactly.
	if err := os.WriteFile(filepath.Join(catalogRoot, "stable-advisor", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}

	catalogURL := "local:" + catalogRoot
	cat := ACatalogSource().WithURL(catalogURL).Build()
	def := AnAdvisorDef().Named("stable-advisor").WithSkillSource(catalogURL).WithOrigin(model.AdvisorOriginCatalog).Build()
	manifest := AUserManifest().WithCatalog(cat).WithCustom(def).Build()

	_, err := runRefreshCatalogsFlow(context.Background(), wd, &manifest)
	if err != nil {
		t.Fatalf("runRefreshCatalogsFlow returned error: %v", err)
	}

	// Installed file content should be unchanged.
	installedData, readErr := os.ReadFile(filepath.Join(advisorDst, "SKILL.md"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(installedData) != skillContent {
		t.Errorf("installed SKILL.md was re-copied but content was the same (no drift):\ngot:\n%s", string(installedData))
	}
}

// TestRunRefreshCatalogsFlow_FetchError_ReturnsAggregatedError verifies that
// when a local: source path does not exist, the refresh flow aggregates the
// error and returns it after processing all catalogs (fail-aggregating semantics).
func TestRunRefreshCatalogsFlow_FetchError_ReturnsAggregatedError(t *testing.T) {
	wd := t.TempDir()

	cat := ACatalogSource().WithURL("local:/nonexistent/catalog/path").Build()
	manifest := AUserManifest().WithCatalog(cat).Build()

	// runRefreshCatalogsFlow aggregates per-catalog errors and returns them.
	_, err := runRefreshCatalogsFlow(context.Background(), wd, &manifest)
	if err == nil {
		t.Fatal("runRefreshCatalogsFlow should return an error when a catalog fetch fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch") {
		t.Errorf("error should mention 'failed to fetch'; got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Import from catalog (via inner helpers) — non-interactive path
// ─────────────────────────────────────────────────────────────────────────────

// TestImportFromLocalCatalog_HappyPath verifies the full import flow via the
// inner helpers (fetchCatalogSource + DirScanner.Scan + filterAvailableEntries
// + copyAdvisorDir + manifest mutation), bypassing the interactive TUI.
//
// This is the integration test for "Import from local catalog (non-interactive)"
// described in the task spec.
func TestImportFromLocalCatalog_HappyPath(t *testing.T) {
	wd := t.TempDir()

	// Seed CLAUDE.md and AGENTS.md (required by SyncAdvisors).
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	// Create fake catalog with 2 advisors, each with SKILL.md and references/.
	catalogRoot := t.TempDir()
	seedCatalogAdvisor(t, catalogRoot, "alpha-catalog-advisor", "Alpha advisor from catalog", true)
	seedCatalogAdvisor(t, catalogRoot, "beta-catalog-advisor", "Beta advisor from catalog", true)

	catalogURL := "local:" + catalogRoot
	cat := model.CatalogSource{URL: catalogURL}

	// Step 1: fetch the catalog root.
	rootDir, err := fetchCatalogSource(context.Background(), wd, cat)
	if err != nil {
		t.Fatalf("fetchCatalogSource returned error: %v", err)
	}

	// Step 2: scan the catalog.
	entries, err := advisorcatalog.DirScanner{}.Scan(rootDir)
	if err != nil {
		t.Fatalf("DirScanner.Scan returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Scan returned %d entries, want 2", len(entries))
	}

	// Step 3: filter (no collisions expected).
	manifest := AUserManifest().Build()
	filtered := filterAvailableEntries(entries, &manifest)
	if len(filtered) != 2 {
		t.Fatalf("filterAvailableEntries returned %d entries, want 2", len(filtered))
	}

	// Step 4: copy advisors and register in manifest.
	// Mimic what runAddAdvisorFlow does after selection.
	manifest.AdvisorCatalogs = append(manifest.AdvisorCatalogs, cat)
	for _, entry := range filtered {
		dst := filepath.Join(wd, ".claude", "skills", entry.Name)
		if _, copyErr := copyAdvisorDir(entry.DirPath, dst); copyErr != nil {
			t.Fatalf("copyAdvisorDir(%q): %v", entry.Name, copyErr)
		}
		manifest.CustomAdvisors = append(manifest.CustomAdvisors, model.AdvisorDef{
			Name:        entry.Name,
			Description: entry.Description,
			SkillSource: catalogURL,
			Scope:       append([]string(nil), entry.Scope...),
			Origin:      model.AdvisorOriginCatalog,
		})
	}

	// Assert manifest has 2 customAdvisors with Origin=catalog and 1 advisorCatalog.
	if len(manifest.CustomAdvisors) != 2 {
		t.Errorf("manifest.CustomAdvisors length = %d, want 2", len(manifest.CustomAdvisors))
	}
	if len(manifest.AdvisorCatalogs) != 1 {
		t.Errorf("manifest.AdvisorCatalogs length = %d, want 1", len(manifest.AdvisorCatalogs))
	}
	for _, def := range manifest.CustomAdvisors {
		if def.Origin != model.AdvisorOriginCatalog {
			t.Errorf("advisor %q: Origin = %q, want %q", def.Name, def.Origin, model.AdvisorOriginCatalog)
		}
	}

	// Assert .claude/skills/{name}/SKILL.md exists for both advisors.
	for _, name := range []string{"alpha-catalog-advisor", "beta-catalog-advisor"} {
		skillMDPath := filepath.Join(wd, ".claude", "skills", name, "SKILL.md")
		if _, err := os.Stat(skillMDPath); err != nil {
			t.Errorf("SKILL.md not found for advisor %q: %v", name, err)
		}
		// Assert references/ was also copied.
		refPath := filepath.Join(wd, ".claude", "skills", name, "references", "guide.md")
		if _, err := os.Stat(refPath); err != nil {
			t.Errorf("references/guide.md not found for advisor %q: %v", name, err)
		}
	}
}

// TestImportFromLocalCatalog_RendererSpyCalled verifies that SyncAdvisors
// (which follows import) calls the renderer with both advisors in Installed.
//
// SyncAdvisors copies advisor dirs using def.SkillSource as a filesystem path,
// so we set SkillSource to the actual advisor directory (not the catalog URL).
func TestImportFromLocalCatalog_RendererSpyCalled(t *testing.T) {
	wd := t.TempDir()

	// Seed root docs.
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	// Create catalog with 2 advisors.
	catalogRoot := t.TempDir()
	seedCatalogAdvisor(t, catalogRoot, "spy-alpha-advisor", "Alpha", false)
	seedCatalogAdvisor(t, catalogRoot, "spy-beta-advisor", "Beta", false)

	// For catalog-origin advisors, SkillSource must be a real directory path
	// that SyncAdvisors can copy from (it uses SkillSource as an fs path).
	alphaDir := filepath.Join(catalogRoot, "spy-alpha-advisor")
	betaDir := filepath.Join(catalogRoot, "spy-beta-advisor")

	manifest := AUserManifest().
		WithCatalog(ACatalogSource().WithURL("local:" + catalogRoot).Build()).
		WithCustom(
			AnAdvisorDef().Named("spy-alpha-advisor").WithSkillSource(alphaDir).WithOrigin(model.AdvisorOriginCatalog).WithDescription("Alpha").Build(),
			AnAdvisorDef().Named("spy-beta-advisor").WithSkillSource(betaDir).WithOrigin(model.AdvisorOriginCatalog).WithDescription("Beta").Build(),
		).
		Build()

	// Inject spy renderer.
	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	if len(spy.Calls) != 1 {
		t.Fatalf("renderer spy called %d times, want 1", len(spy.Calls))
	}

	// Both advisors must appear in Installed.
	installedNames := make(map[string]bool)
	for _, item := range spy.Calls[0].Installed {
		installedNames[item.Name] = true
	}
	if !installedNames["spy-alpha-advisor"] {
		t.Error("spy-alpha-advisor not found in Installed")
	}
	if !installedNames["spy-beta-advisor"] {
		t.Error("spy-beta-advisor not found in Installed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// singleAdvisorEntry scope propagation tests (T017)
// ─────────────────────────────────────────────────────────────────────────────

// writeSingleAdvisorSkill creates <dir>/<name>/SKILL.md with the given raw
// frontmatter block (everything between "---\n" and "---\n"). The body is
// set to "# <name>\n". Returns the advisor directory path.
func writeSingleAdvisorSkill(t *testing.T, dir, name, rawFrontmatter string) string {
	t.Helper()
	advisorDir := filepath.Join(dir, name)
	if err := os.MkdirAll(advisorDir, 0o755); err != nil {
		t.Fatalf("writeSingleAdvisorSkill: mkdir %q: %v", advisorDir, err)
	}
	content := "---\nname: " + name + "\n" + rawFrontmatter + "---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(advisorDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeSingleAdvisorSkill: write SKILL.md: %v", err)
	}
	return advisorDir
}

// TestSingleAdvisorEntry_ParsesScopeList verifies that a valid scope list is
// parsed and normalized into the CatalogEntry.Scope field.
func TestSingleAdvisorEntry_ParsesScopeList(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [backend, api]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error: %v", err)
	}
	want := []string{"backend", "api"}
	if len(entry.Scope) != len(want) {
		t.Fatalf("entry.Scope = %v, want %v", entry.Scope, want)
	}
	for i, v := range want {
		if entry.Scope[i] != v {
			t.Errorf("entry.Scope[%d] = %q, want %q", i, entry.Scope[i], v)
		}
	}
}

// TestSingleAdvisorEntry_TrimsScopeWhitespace verifies that whitespace is
// trimmed from scope values before normalization (CONTRACT pin).
func TestSingleAdvisorEntry_TrimsScopeWhitespace(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [\"  backend  \", api]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error: %v", err)
	}
	want := []string{"backend", "api"}
	if len(entry.Scope) != len(want) {
		t.Fatalf("entry.Scope = %v, want %v", entry.Scope, want)
	}
	for i, v := range want {
		if entry.Scope[i] != v {
			t.Errorf("entry.Scope[%d] = %q, want %q", i, entry.Scope[i], v)
		}
	}
}

// TestSingleAdvisorEntry_DropsUnknownScopeSoftly verifies that unknown
// vocabulary values are silently dropped (CONTRACT pin — soft fallback).
func TestSingleAdvisorEntry_DropsUnknownScopeSoftly(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [backend, foo]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error (want nil for unknown vocab): %v", err)
	}
	want := []string{"backend"}
	if len(entry.Scope) != len(want) || entry.Scope[0] != want[0] {
		t.Errorf("entry.Scope = %v, want %v (unknown 'foo' should be silently dropped)", entry.Scope, want)
	}
}

// TestSingleAdvisorEntry_AllUnknownBecomesUniversal verifies that when all
// scope values are unknown, the advisor is treated as universal (nil Scope)
// (CONTRACT pin — soft fallback, all-unknown → nil).
func TestSingleAdvisorEntry_AllUnknownBecomesUniversal(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [foo, bar]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error (want nil for all-unknown vocab): %v", err)
	}
	if entry.Scope != nil {
		t.Errorf("entry.Scope = %v, want nil (all-unknown should fall back to universal)", entry.Scope)
	}
}

// TestSingleAdvisorEntry_DedupesScope verifies that duplicate scope values are
// deduplicated, keeping the first occurrence.
func TestSingleAdvisorEntry_DedupesScope(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [backend, backend]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error: %v", err)
	}
	if len(entry.Scope) != 1 || entry.Scope[0] != "backend" {
		t.Errorf("entry.Scope = %v, want [backend] (duplicates should be deduped)", entry.Scope)
	}
}

// TestSingleAdvisorEntry_DropsEmptyScopeElementSoftly verifies that an empty
// string element in scope is silently dropped (post-trim empty → soft drop).
func TestSingleAdvisorEntry_DropsEmptyScopeElementSoftly(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [\"\", api]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error (want nil for empty element): %v", err)
	}
	if len(entry.Scope) != 1 || entry.Scope[0] != "api" {
		t.Errorf("entry.Scope = %v, want [api] (empty element should be silently dropped)", entry.Scope)
	}
}

// TestSingleAdvisorEntry_DropsWhitespaceOnlyElementSoftly verifies that a
// whitespace-only element in scope is silently dropped.
func TestSingleAdvisorEntry_DropsWhitespaceOnlyElementSoftly(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [\"   \", api]\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error (want nil for whitespace-only element): %v", err)
	}
	if len(entry.Scope) != 1 || entry.Scope[0] != "api" {
		t.Errorf("entry.Scope = %v, want [api] (whitespace-only element should be silently dropped)", entry.Scope)
	}
}

// TestSingleAdvisorEntry_RejectsScalarScope verifies that a scalar scope value
// (not a list) is a hard YAML-shape error wrapping ErrFrontmatterNotList.
func TestSingleAdvisorEntry_RejectsScalarScope(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: backend\n")

	_, err := singleAdvisorEntry(advisorDir)
	if err == nil {
		t.Fatal("singleAdvisorEntry expected error for scalar scope, got nil")
	}
	if !errors.Is(err, advisormeta.ErrFrontmatterNotList) {
		t.Errorf("error should wrap ErrFrontmatterNotList; got: %v", err)
	}
}

// TestSingleAdvisorEntry_RejectsNullScope verifies that a null scope value is
// a hard YAML-shape error wrapping ErrFrontmatterNullValue.
func TestSingleAdvisorEntry_RejectsNullScope(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: null\n")

	_, err := singleAdvisorEntry(advisorDir)
	if err == nil {
		t.Fatal("singleAdvisorEntry expected error for null scope, got nil")
	}
	if !errors.Is(err, advisormeta.ErrFrontmatterNullValue) {
		t.Errorf("error should wrap ErrFrontmatterNullValue; got: %v", err)
	}
}

// TestSingleAdvisorEntry_RejectsIntElement verifies that an integer element in
// the scope list is a hard YAML-shape error wrapping ErrFrontmatterNotString,
// and that the error message contains positional index info (e.g. "[1]").
func TestSingleAdvisorEntry_RejectsIntElement(t *testing.T) {
	dir := t.TempDir()
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "scope: [backend, 42]\n")

	_, err := singleAdvisorEntry(advisorDir)
	if err == nil {
		t.Fatal("singleAdvisorEntry expected error for int element in scope, got nil")
	}
	if !errors.Is(err, advisormeta.ErrFrontmatterNotString) {
		t.Errorf("error should wrap ErrFrontmatterNotString; got: %v", err)
	}
	if !strings.Contains(err.Error(), "[1]") {
		t.Errorf("error should contain positional index [1]; got: %v", err)
	}
}

// TestSingleAdvisorEntry_AcceptsMissingScope verifies that an advisor without
// a scope key is accepted and gets nil Scope (= universal).
func TestSingleAdvisorEntry_AcceptsMissingScope(t *testing.T) {
	dir := t.TempDir()
	// No scope frontmatter at all.
	advisorDir := writeSingleAdvisorSkill(t, dir, "demo-advisor", "description: No scope advisor\n")

	entry, err := singleAdvisorEntry(advisorDir)
	if err != nil {
		t.Fatalf("singleAdvisorEntry returned error for missing scope: %v", err)
	}
	if entry.Scope != nil {
		t.Errorf("entry.Scope = %v, want nil (missing scope = universal)", entry.Scope)
	}
}

// TestImportFromLocalCatalog_CollisionWithNative_NoFileSystemChanges verifies
// that when all catalog advisors collide with native advisors, the manifest
// is not mutated and no files are written.
func TestImportFromLocalCatalog_CollisionWithNative_NoFileSystemChanges(t *testing.T) {
	wd := t.TempDir()

	// Use a known native advisor name to trigger a collision.
	nativeName := "unit-test-advisor"
	catalogRoot := t.TempDir()
	seedCatalogAdvisor(t, catalogRoot, nativeName, "Collides with native", false)

	catalogURL := "local:" + catalogRoot
	cat := model.CatalogSource{URL: catalogURL}

	// Fetch and scan.
	rootDir, err := fetchCatalogSource(context.Background(), wd, cat)
	if err != nil {
		t.Fatalf("fetchCatalogSource: %v", err)
	}
	entries, err := advisorcatalog.DirScanner{}.Scan(rootDir)
	if err != nil {
		t.Fatalf("DirScanner.Scan: %v", err)
	}

	manifest := AUserManifest().Build()
	filtered := filterAvailableEntries(entries, &manifest)

	// All entries should be filtered out due to collision.
	if len(filtered) != 0 {
		t.Errorf("expected all entries filtered (collision), got %d entries: %v", len(filtered), filtered)
	}

	// Verify no skills directory was created.
	skillDir := filepath.Join(wd, ".claude", "skills", nativeName)
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skills directory for %q should NOT exist after collision detection", nativeName)
	}

	// Verify manifest was not mutated.
	if len(manifest.CustomAdvisors) != 0 {
		t.Errorf("manifest.CustomAdvisors should be empty after collision; got %d entries", len(manifest.CustomAdvisors))
	}
}
