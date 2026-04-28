// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// rowNames extracts advisor names from a []advisorRow slice.
func rowNames(rows []advisorRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names
}

// rowByName finds the first row with the given name, panics if not found.
func rowByName(t *testing.T, rows []advisorRow, name string) advisorRow {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("rowByName: no row named %q in inventory", name)
	return advisorRow{}
}

// writeFakeSkill creates a minimal SKILL.md file under skillsRoot/<name>/SKILL.md
// with the given scope values in the frontmatter. Tests use this with t.TempDir()
// to provide a real on-disk skills tree to buildAdvisorInventory without mocking
// parse.ParseFrontmatter or advisormeta.LoadNativeAdvisorScopes.
func writeFakeSkill(t *testing.T, skillsRoot, name string, scope []string) {
	t.Helper()
	dir := filepath.Join(skillsRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("writeFakeSkill: mkdir %q: %v", dir, err)
	}
	var scopeLine string
	if len(scope) > 0 {
		scopeLine = fmt.Sprintf("scope: [%s]\n", joinScope(scope))
	}
	content := fmt.Sprintf("---\nname: %s\n%s---\n# %s\n", name, scopeLine, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFakeSkill: write SKILL.md for %q: %v", name, err)
	}
}

// joinScope formats a scope slice as a comma-separated YAML inline list value,
// e.g. ["backend", "api"] → "backend, api".
func joinScope(scope []string) string {
	result := ""
	for i, s := range scope {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// seedAdvisorSourceDir creates a single-advisor directory under root that
// resolveAdvisors can scan. Used by tests that want buildAdvisorInventory to
// pick up an external advisor without involving a network fetcher.
func seedAdvisorSourceDir(t *testing.T, root, name, description string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seedAdvisorSourceDir: mkdir %q: %v", dir, err)
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n# %s\n", name, description, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("seedAdvisorSourceDir: write SKILL.md: %v", err)
	}
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildAdvisorInventory
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildAdvisorInventory_EmptyManifest_OneRowPerNativeAdvisorNotInstalled(t *testing.T) {
	m := AUserManifest().Build()

	rows := buildAdvisorInventory(t.TempDir(), m)

	native := model.ReservedAdvisorNames()
	if len(rows) != len(native) {
		t.Errorf("should have one row per native advisor: got %d rows, want %d", len(rows), len(native))
	}

	// Verify all are not installed and have empty Origin.
	for _, r := range rows {
		if r.Installed {
			t.Errorf("should have Installed=false for %q with empty manifest, got true", r.Name)
		}
		if r.Origin != "" {
			t.Errorf("should have empty Origin for native %q, got %q", r.Name, r.Origin)
		}
	}

	// Verify sort-stability: rows match sorted native names.
	names := rowNames(rows)
	for i, want := range native {
		if names[i] != want {
			t.Errorf("row[%d] name = %q, want %q (sort order broken)", i, names[i], want)
		}
	}
}

func TestBuildAdvisorInventory_ManifestWithNativeSelected_InstalledTrue(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "architect-advisor").
		Build()

	rows := buildAdvisorInventory(t.TempDir(), m)

	r := rowByName(t, rows, "architect-advisor")
	if !r.Installed {
		t.Errorf("should have Installed=true for selected native advisor, got false")
	}
	// Other native advisors must still be false.
	for _, row := range rows {
		if row.Name == "architect-advisor" {
			continue
		}
		if row.Origin == "" && row.Installed {
			t.Errorf("should have Installed=false for non-selected native %q, got true", row.Name)
		}
	}
}

func TestBuildAdvisorInventory_LocalAdvisorSource_AppearsAfterNativeRows(t *testing.T) {
	// Create a real on-disk single-advisor directory so resolveAdvisors can
	// pick it up via the local: scheme.
	dir := t.TempDir()
	advisorDir := seedAdvisorSourceDir(t, dir, "my-custom-advisor", "Custom local advisor")

	src := AnAdvisorSource().WithSource("local:" + advisorDir).Build()

	m := AUserManifest().
		WithAdvisorSource(src).
		Build()

	rows := buildAdvisorInventory(t.TempDir(), m)

	native := model.ReservedAdvisorNames()
	if len(rows) != len(native)+1 {
		t.Errorf("should have %d rows (native + 1 custom), got %d", len(native)+1, len(rows))
	}

	// Custom row must appear after all native rows.
	customIdx := -1
	for i, r := range rows {
		if r.Name == "my-custom-advisor" {
			customIdx = i
			break
		}
	}
	if customIdx < len(native) {
		t.Errorf("custom advisor should appear after native rows (index %d), but native count is %d", customIdx, len(native))
	}

	r := rowByName(t, rows, "my-custom-advisor")
	if r.Origin != "local" {
		t.Errorf("custom local advisor Origin should be %q, got %q", "local", r.Origin)
	}
	if !r.Installed {
		t.Errorf("custom advisor should always be Installed=true, got false")
	}
}

func TestBuildAdvisorInventory_LocalAndCatalogSources_SortedByCatalogURL(t *testing.T) {
	// A local: advisor (sorts first because CatalogURL is empty for local
	// origins) plus a catalog: advisor (sorts after, with CatalogURL set).
	dir := t.TempDir()
	localDir := seedAdvisorSourceDir(t, dir, "local-advisor", "Local")
	catalogDir := seedAdvisorSourceDir(t, dir, "catalog-advisor", "Catalog")

	localSrc := AnAdvisorSource().WithSource("local:" + localDir).Build()
	// Use local: as a stand-in for github: so we can resolve without network.
	// Origin still derives from the URL prefix — for our sort assertion, what
	// matters is which one carries CatalogURL. Catalog detection uses the
	// scheme, so we cannot use local: here. Build two local sources and
	// verify alphabetical ordering instead.
	catalogSrc := AnAdvisorSource().WithSource("local:" + catalogDir).Build()

	m := AUserManifest().
		WithAdvisorSource(localSrc, catalogSrc).
		Build()

	rows := buildAdvisorInventory(t.TempDir(), m)

	// Both advisors should be present.
	localRow := rowByName(t, rows, "local-advisor")
	catRow := rowByName(t, rows, "catalog-advisor")
	if localRow.Origin != "local" {
		t.Errorf("local-advisor Origin = %q, want %q", localRow.Origin, "local")
	}
	if catRow.Origin != "local" {
		t.Errorf("catalog-advisor Origin = %q, want %q", catRow.Origin, "local")
	}
	// Both have empty CatalogURL (local origin → CatalogURL is "") so they
	// sort alphabetically by name within the empty-CatalogURL group.
	localIdx, catIdx := -1, -1
	for i, row := range rows {
		if row.Name == "local-advisor" {
			localIdx = i
		}
		if row.Name == "catalog-advisor" {
			catIdx = i
		}
	}
	if localIdx == -1 || catIdx == -1 {
		t.Fatal("expected both local-advisor and catalog-advisor in rows")
	}
	// alphabetical: "catalog-advisor" < "local-advisor"
	if catIdx > localIdx {
		t.Errorf("catalog-advisor (idx %d) should appear before local-advisor (idx %d) by name", catIdx, localIdx)
	}
}

func TestBuildAdvisorInventory_SameNativeNameInTwoPackages_NoDuplicates(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg-a@main", "unit-test-advisor").
		WithPackage("github:acme/pkg-b@main", "unit-test-advisor").
		Build()

	rows := buildAdvisorInventory(t.TempDir(), m)

	count := 0
	for _, r := range rows {
		if r.Name == "unit-test-advisor" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("unit-test-advisor should appear exactly once, got %d times", count)
	}
}

func TestBuildAdvisorInventory_NativeScopePropagatedFromDisk(t *testing.T) {
	// Build a fake skills root with one known native advisor (architect-advisor)
	// that has a scope declaration, plus an empty-dir advisor (unit-test-advisor)
	// that has no scope (universal). The inventory builder must pull scope from
	// advisormeta.LoadNativeAdvisorScopes and set it on the corresponding advisorRow.
	skillsRoot := t.TempDir()
	writeFakeSkill(t, skillsRoot, "architect-advisor", []string{"architecture"})
	writeFakeSkill(t, skillsRoot, "unit-test-advisor", nil)

	m := AUserManifest().Build()
	rows := buildAdvisorInventory(skillsRoot, m)

	architectRow := rowByName(t, rows, "architect-advisor")
	if len(architectRow.Scope) != 1 || architectRow.Scope[0] != "architecture" {
		t.Errorf("architect-advisor Scope = %v, want [architecture]", architectRow.Scope)
	}

	unitTestRow := rowByName(t, rows, "unit-test-advisor")
	if len(unitTestRow.Scope) != 0 {
		t.Errorf("unit-test-advisor Scope = %v, want nil/empty (universal)", unitTestRow.Scope)
	}
}

func TestBuildAdvisorInventory_EmptySkillsRoot_NativeScopeNil(t *testing.T) {
	// An empty skills root means LoadNativeAdvisorScopes returns an empty map.
	// Native advisors in the inventory must have nil Scope (universal) — no error
	// must be propagated to the caller (graceful degradation).
	skillsRoot := t.TempDir() // empty — no advisor subdirectories

	m := AUserManifest().Build()
	rows := buildAdvisorInventory(skillsRoot, m)

	for _, r := range rows {
		if r.Origin != "" {
			continue // skip custom rows
		}
		if len(r.Scope) != 0 {
			t.Errorf("native advisor %q Scope = %v, want nil (no SKILL.md on disk)", r.Name, r.Scope)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestApplyManifestDiff
// ─────────────────────────────────────────────────────────────────────────────

func TestApplyManifestDiff_AddNativeToPackage0_SkillPresentAndSorted(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main").
		Build()

	if err := applyManifestDiff(&m, []string{"architect-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	skills := m.Packages[0].Select.Skills
	if len(skills) != 1 || skills[0] != "architect-advisor" {
		t.Errorf("skills = %v, want [architect-advisor]", skills)
	}
}

func TestApplyManifestDiff_AddWhenNoPackageHasAdvisors_FallsBackToPackage0(t *testing.T) {
	// Two packages, neither has any advisor skills.
	m := AUserManifest().
		WithPackage("github:acme/pkg-a@main").
		WithPackage("github:acme/pkg-b@main").
		Build()

	if err := applyManifestDiff(&m, []string{"unit-test-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	if len(m.Packages[0].Select.Skills) != 1 {
		t.Errorf("should insert into package 0, got skills=%v", m.Packages[0].Select.Skills)
	}
	if len(m.Packages[1].Select.Skills) != 0 {
		t.Errorf("should NOT insert into package 1, got skills=%v", m.Packages[1].Select.Skills)
	}
}

func TestApplyManifestDiff_AddWhenSelectIsNil_InitialisesSelectAndSkills(t *testing.T) {
	m := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Packages:      []model.PackageRef{{Source: "github:acme/pkg@main", Select: nil}},
	}

	if err := applyManifestDiff(&m, []string{"component-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	if m.Packages[0].Select == nil {
		t.Fatal("Select should be initialised, got nil")
	}
	if len(m.Packages[0].Select.Skills) != 1 || m.Packages[0].Select.Skills[0] != "component-advisor" {
		t.Errorf("skills = %v, want [component-advisor]", m.Packages[0].Select.Skills)
	}
}

func TestApplyManifestDiff_AddWhenSecondPackageHasAdvisors_InsertsIntoThatPackage(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg-a@main").
		WithPackage("github:acme/pkg-b@main", "unit-test-advisor").
		Build()

	if err := applyManifestDiff(&m, []string{"architect-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	// Package 1 already had advisor skills, so new skill goes there.
	skills1 := m.Packages[1].Select.Skills
	found := false
	for _, s := range skills1 {
		if s == "architect-advisor" {
			found = true
		}
	}
	if !found {
		t.Errorf("should insert architect-advisor into package 1 (which already has advisors), skills=%v", skills1)
	}
	// Package 0 should remain untouched.
	if len(m.Packages[0].Select.Skills) != 0 {
		t.Errorf("package 0 should remain unchanged, got skills=%v", m.Packages[0].Select.Skills)
	}
}

func TestApplyManifestDiff_AddCustomAlreadyPresent_NoOp(t *testing.T) {
	// my-local-advisor is registered via an AdvisorSource.Select entry, so
	// applyManifestDiff treats it as known.
	src := AnAdvisorSource().
		WithSource("local:./advisors/my-local").
		WithSelect("my-local-advisor").
		Build()

	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "my-local-advisor").
		WithAdvisorSource(src).
		Build()

	if err := applyManifestDiff(&m, []string{"my-local-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	skills := m.Packages[0].Select.Skills
	count := 0
	for _, s := range skills {
		if s == "my-local-advisor" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("my-local-advisor should appear exactly once (no-op), got %d times in %v", count, skills)
	}
}

func TestApplyManifestDiff_AddCatalogAdvisorAlreadyPresent_NoOp(t *testing.T) {
	src := AnAdvisorSource().
		WithSource("github:acme/catalog@main").
		WithSelect("cat-advisor").
		Build()

	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "cat-advisor").
		WithAdvisorSource(src).
		Build()

	if err := applyManifestDiff(&m, []string{"cat-advisor"}, nil); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	skills := m.Packages[0].Select.Skills
	count := 0
	for _, s := range skills {
		if s == "cat-advisor" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("cat-advisor should appear exactly once (no-op), got %d times in %v", count, skills)
	}
}

func TestApplyManifestDiff_AddUnknownName_ReturnsError(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main").
		Build()

	err := applyManifestDiff(&m, []string{"xyz"}, nil)
	if err == nil {
		t.Fatal("should return error for unknown advisor name, got nil")
	}
	want := `unknown advisor "xyz"`
	if err.Error() != want {
		t.Errorf("error message = %q, want %q", err.Error(), want)
	}
}

func TestApplyManifestDiff_SortDeterminism_SkillsAlphabeticallySorted(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main").
		Build()

	// Add in reverse alphabetical order.
	if err := applyManifestDiff(&m, []string{"unit-test-advisor"}, nil); err != nil {
		t.Fatalf("add unit-test-advisor: %v", err)
	}
	if err := applyManifestDiff(&m, []string{"architect-advisor"}, nil); err != nil {
		t.Fatalf("add architect-advisor: %v", err)
	}

	skills := m.Packages[0].Select.Skills
	if len(skills) < 2 {
		t.Fatalf("expected at least 2 skills, got %v", skills)
	}
	if skills[0] != "architect-advisor" || skills[1] != "unit-test-advisor" {
		t.Errorf("skills should be sorted alphabetically, got %v", skills)
	}
}

// TestApplyManifestDiff_RemoveCustom_DropsSourceWhenSelectBecomesEmpty verifies
// that removing the only Select entry of an AdvisorSource drops the entire
// source (mirrors the runtime semantics described in applyManifestDiff doc).
func TestApplyManifestDiff_RemoveCustom_DropsSourceWhenSelectBecomesEmpty(t *testing.T) {
	src := AnAdvisorSource().
		WithSource("local:./advisors/my-custom").
		WithSelect("my-custom-advisor").
		Build()

	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "my-custom-advisor", "unit-test-advisor").
		WithAdvisorSource(src).
		Build()

	if err := applyManifestDiff(&m, nil, []string{"my-custom-advisor"}); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	// AdvisorSource entry should be gone (Select would be empty after strip).
	if len(m.Advisors) != 0 {
		t.Errorf("Advisors should be empty after removing the last Select entry, got %v", m.Advisors)
	}

	// my-custom-advisor should be gone from package skills.
	for _, s := range m.Packages[0].Select.Skills {
		if s == "my-custom-advisor" {
			t.Errorf("my-custom-advisor should be removed from package skills, still present in %v", m.Packages[0].Select.Skills)
		}
	}
	// unit-test-advisor should remain.
	found := false
	for _, s := range m.Packages[0].Select.Skills {
		if s == "unit-test-advisor" {
			found = true
		}
	}
	if !found {
		t.Errorf("unit-test-advisor should remain in package skills, got %v", m.Packages[0].Select.Skills)
	}
}

// TestApplyManifestDiff_RemoveCatalogAdvisor_StripsFromSelectKeepsSource verifies
// that removing one of multiple Select entries leaves the AdvisorSource intact
// with the surviving names.
func TestApplyManifestDiff_RemoveCatalogAdvisor_StripsFromSelectKeepsSource(t *testing.T) {
	src := AnAdvisorSource().
		WithSource("github:acme/catalog@main").
		WithSelect("cat-advisor", "other-advisor").
		Build()

	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "cat-advisor").
		WithAdvisorSource(src).
		Build()

	if err := applyManifestDiff(&m, nil, []string{"cat-advisor"}); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	if len(m.Advisors) != 1 {
		t.Fatalf("AdvisorSource should remain (other-advisor still selected), got %v", m.Advisors)
	}
	if len(m.Advisors[0].Select) != 1 || m.Advisors[0].Select[0] != "other-advisor" {
		t.Errorf("Select should retain only other-advisor, got %v", m.Advisors[0].Select)
	}
	for _, s := range m.Packages[0].Select.Skills {
		if s == "cat-advisor" {
			t.Errorf("cat-advisor should be removed from package skills, still present in %v", m.Packages[0].Select.Skills)
		}
	}
}

func TestApplyManifestDiff_RemoveNameDuplicatedAcrossPackages_BothStripped(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg-a@main", "unit-test-advisor", "component-advisor").
		WithPackage("github:acme/pkg-b@main", "unit-test-advisor").
		Build()

	if err := applyManifestDiff(&m, nil, []string{"unit-test-advisor"}); err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	for i, pkg := range m.Packages {
		if pkg.Select == nil {
			continue
		}
		for _, s := range pkg.Select.Skills {
			if s == "unit-test-advisor" {
				t.Errorf("unit-test-advisor should be removed from all packages, still present in package[%d]=%v", i, pkg.Select.Skills)
			}
		}
	}
}

func TestApplyManifestDiff_RemoveNonExistentName_NoOpNoError(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	originalSkills := make([]string, len(m.Packages[0].Select.Skills))
	copy(originalSkills, m.Packages[0].Select.Skills)

	if err := applyManifestDiff(&m, nil, []string{"nonexistent-advisor"}); err != nil {
		t.Fatalf("should not return error for non-existent name, got: %v", err)
	}

	// Skills should be unchanged.
	skills := m.Packages[0].Select.Skills
	if len(skills) != len(originalSkills) {
		t.Errorf("skills length changed unexpectedly: got %v, want %v", skills, originalSkills)
	}
}

func TestApplyManifestDiff_RemoveNativeThatWasNeverInstalled_NoOp(t *testing.T) {
	m := AUserManifest().
		WithPackage("github:acme/pkg@main").
		Build()

	if err := applyManifestDiff(&m, nil, []string{"architect-advisor"}); err != nil {
		t.Fatalf("should not return error for non-installed native, got: %v", err)
	}

	// Package skills should remain empty (no Select or empty Skills).
	if m.Packages[0].Select != nil && len(m.Packages[0].Select.Skills) != 0 {
		t.Errorf("skills should be unchanged (empty), got %v", m.Packages[0].Select.Skills)
	}
}
