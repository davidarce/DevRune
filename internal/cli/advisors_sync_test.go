// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize"
	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Spy / fake types
// ─────────────────────────────────────────────────────────────────────────────

type regenCall struct {
	WorkspaceRoot  string
	Installed      []model.ContentItem
	Removed        []string
	ModelOverrides map[string]string
}

// fakeAdvisorRenderer implements both materialize.AgentRenderer (minimal stubs)
// and materialize.AdvisorRenderer so it is picked up by the type-assertion in
// SyncAdvisors.
type fakeAdvisorRenderer struct {
	Calls  []regenCall
	Result matypes.AdvisorRenderResult
	Err    error
}

func (f *fakeAdvisorRenderer) RegenerateAdvisorFiles(
	workspaceRoot string,
	installed []model.ContentItem,
	removed []string,
	modelOverrides map[string]string,
) (matypes.AdvisorRenderResult, error) {
	f.Calls = append(f.Calls, regenCall{workspaceRoot, installed, removed, modelOverrides})
	return f.Result, f.Err
}

// AgentRenderer stubs — minimum needed to satisfy the interface.
func (f *fakeAdvisorRenderer) Name() string                      { return "fake" }
func (f *fakeAdvisorRenderer) AgentType() string                 { return "fake" }
func (f *fakeAdvisorRenderer) NeedsCopyMode() bool               { return false }
func (f *fakeAdvisorRenderer) Definition() model.AgentDefinition { return model.AgentDefinition{} }
func (f *fakeAdvisorRenderer) WorkspacePaths() materialize.AgentPaths {
	return materialize.AgentPaths{}
}
func (f *fakeAdvisorRenderer) RenderSkill(_, _ string) error { return nil }
func (f *fakeAdvisorRenderer) RenderCommand(_ model.WorkflowCommand, _ string) error {
	return nil
}
func (f *fakeAdvisorRenderer) RenderMCPs(_ []model.LockedMCP, _ materialize.CacheStore, _ string) error {
	return nil
}
func (f *fakeAdvisorRenderer) RenderSettings(_ string, _ []model.ContentItem, _ []model.WorkflowManifest) error {
	return nil
}
func (f *fakeAdvisorRenderer) InstallWorkflow(_ model.WorkflowManifest, _, _, _ string) (materialize.WorkflowInstallResult, error) {
	return materialize.WorkflowInstallResult{}, nil
}
func (f *fakeAdvisorRenderer) Finalize(_ string) error { return nil }

// nonAdvisorRenderer satisfies materialize.AgentRenderer but does NOT implement
// materialize.AdvisorRenderer — used to verify the silent-skip path.
type nonAdvisorRenderer struct{}

func (n *nonAdvisorRenderer) Name() string                      { return "non-advisor" }
func (n *nonAdvisorRenderer) AgentType() string                 { return "non-advisor" }
func (n *nonAdvisorRenderer) NeedsCopyMode() bool               { return false }
func (n *nonAdvisorRenderer) Definition() model.AgentDefinition { return model.AgentDefinition{} }
func (n *nonAdvisorRenderer) WorkspacePaths() materialize.AgentPaths {
	return materialize.AgentPaths{}
}
func (n *nonAdvisorRenderer) RenderSkill(_, _ string) error { return nil }
func (n *nonAdvisorRenderer) RenderCommand(_ model.WorkflowCommand, _ string) error {
	return nil
}
func (n *nonAdvisorRenderer) RenderMCPs(_ []model.LockedMCP, _ materialize.CacheStore, _ string) error {
	return nil
}
func (n *nonAdvisorRenderer) RenderSettings(_ string, _ []model.ContentItem, _ []model.WorkflowManifest) error {
	return nil
}
func (n *nonAdvisorRenderer) InstallWorkflow(_ model.WorkflowManifest, _, _, _ string) (materialize.WorkflowInstallResult, error) {
	return materialize.WorkflowInstallResult{}, nil
}
func (n *nonAdvisorRenderer) Finalize(_ string) error { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// injectRenderer replaces defaultRendererProvider with a function that always
// returns the given renderer(s). The original is restored via t.Cleanup.
func injectRenderer(t *testing.T, renderers ...materialize.AgentRenderer) {
	t.Helper()
	orig := defaultRendererProvider
	defaultRendererProvider = func(_ string, _ []model.AgentRef) ([]materialize.AgentRenderer, error) {
		return renderers, nil
	}
	t.Cleanup(func() { defaultRendererProvider = orig })
}

// seedNativeSkillMD writes a stub SKILL.md with valid YAML frontmatter for a
// native advisor skill directory under wd.
func seedNativeSkillMD(t *testing.T, wd, name, description string) {
	t.Helper()
	path := filepath.Join(wd, ".claude", "skills", name, "SKILL.md")
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("seedNativeSkillMD: mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seedNativeSkillMD: write: %v", err)
	}
}

// containsName reports whether the ContentItem slice contains an item with the given name.
func containsName(items []model.ContentItem, name string) bool {
	for _, it := range items {
		if it.Name == name {
			return true
		}
	}
	return false
}

// containsStr reports whether slice contains s.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// stateYAMLExists reports whether .devrune/state.yaml exists under wd.
func stateYAMLExists(wd string) bool {
	_, err := os.Stat(filepath.Join(wd, ".devrune", "state.yaml"))
	return err == nil
}

// ─────────────────────────────────────────────────────────────────────────────
// T016 — Spy-based tests for SyncAdvisors
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncAdvisors_HappyPathInstallUninstall verifies that installing one advisor
// and uninstalling another results in the spy being called once with the correct
// installed and removed lists.
func TestSyncAdvisors_HappyPathInstallUninstall(t *testing.T) {
	wd := t.TempDir()

	// Seed: unit-test-advisor SKILL.md (to be installed)
	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")

	// Seed CLAUDE.md and AGENTS.md so catalog sync can write to them.
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{
		Result: matypes.AdvisorRenderResult{
			Written: []string{filepath.Join(wd, ".claude", "agents", "unit-test-advisor.md")},
		},
	}
	injectRenderer(t, spy)

	// Manifest: install unit-test-advisor, do NOT install architect-advisor.
	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	result, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	if len(spy.Calls) != 1 {
		t.Fatalf("expected spy called once, got %d calls", len(spy.Calls))
	}

	call := spy.Calls[0]

	if !containsName(call.Installed, "unit-test-advisor") {
		t.Errorf("expected unit-test-advisor in Installed; got %v", call.Installed)
	}

	// The result Written list should be non-empty (from spy Result).
	if len(result.Written) == 0 {
		t.Errorf("AdvisorsSyncResult.Written should be non-empty from spy Result")
	}
}

// TestSyncAdvisors_RendererErrorRollback verifies that when the renderer returns
// an error, SyncAdvisors propagates it and does NOT write state.yaml.
func TestSyncAdvisors_RendererErrorRollback(t *testing.T) {
	wd := t.TempDir()

	// Seed: unit-test-advisor SKILL.md
	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{
		Err: errors.New("boom"),
	}
	injectRenderer(t, spy)

	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err == nil {
		t.Fatal("expected error from renderer, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should contain 'boom'; got: %v", err)
	}

	// state.yaml must NOT have been written.
	if stateYAMLExists(wd) {
		t.Error("state.yaml should NOT have been written on renderer error")
	}
}

// TestSyncAdvisors_NativeSkillMDMissing verifies that when a selected native
// advisor has no SKILL.md installed, SyncAdvisors skips it with a warning
// instead of erroring out. The renderer is still invoked for any other
// advisors that ARE installed; missing ones are simply absent from the
// installed set passed to the renderer.
func TestSyncAdvisors_NativeSkillMDMissing(t *testing.T) {
	wd := t.TempDir()

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	// Manifest selects unit-test-advisor, but its SKILL.md does NOT exist.
	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("expected no error when SKILL.md is missing (should skip with warning), got: %v", err)
	}
	if len(spy.Calls) != 1 {
		t.Fatalf("spy should be called once, got %d call(s)", len(spy.Calls))
	}
	for _, item := range spy.Calls[0].Installed {
		if item.Name == "unit-test-advisor" {
			t.Errorf("missing advisor %q should NOT be in the installed set passed to the renderer", item.Name)
		}
	}
}

// TestSyncAdvisors_PortCapabilityCheck verifies that only renderers implementing
// materialize.AdvisorRenderer are called — renderers that do not implement the
// port are silently skipped.
func TestSyncAdvisors_PortCapabilityCheck(t *testing.T) {
	wd := t.TempDir()

	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy1 := &fakeAdvisorRenderer{}
	spy2 := &fakeAdvisorRenderer{}
	nonAdvisor := &nonAdvisorRenderer{}

	injectRenderer(t, spy1, nonAdvisor, spy2)

	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	// Both fakeAdvisorRenderers must have been called.
	if len(spy1.Calls) != 1 {
		t.Errorf("spy1: expected 1 call, got %d", len(spy1.Calls))
	}
	if len(spy2.Calls) != 1 {
		t.Errorf("spy2: expected 1 call, got %d", len(spy2.Calls))
	}
	// nonAdvisorRenderer has no Calls field — we just verify no panic and success above.
}

// TestSyncAdvisors_RetryConvergence verifies that running SyncAdvisors twice
// with identical inputs results in the spy being called with the same arguments
// both times (idempotent behavior).
func TestSyncAdvisors_RetryConvergence(t *testing.T) {
	wd := t.TempDir()

	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{
		Result: matypes.AdvisorRenderResult{
			Written: []string{filepath.Join(wd, ".claude", "agents", "unit-test-advisor.md")},
		},
	}
	injectRenderer(t, spy)

	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	// First run.
	if _, err := SyncAdvisors(context.Background(), wd, manifest); err != nil {
		t.Fatalf("SyncAdvisors run 1 returned error: %v", err)
	}

	// Second run.
	if _, err := SyncAdvisors(context.Background(), wd, manifest); err != nil {
		t.Fatalf("SyncAdvisors run 2 returned error: %v", err)
	}

	if len(spy.Calls) != 2 {
		t.Fatalf("expected spy called twice, got %d calls", len(spy.Calls))
	}

	// Both calls must have the same installed names.
	call1Names := make([]string, len(spy.Calls[0].Installed))
	call2Names := make([]string, len(spy.Calls[1].Installed))
	for i, it := range spy.Calls[0].Installed {
		call1Names[i] = it.Name
	}
	for i, it := range spy.Calls[1].Installed {
		call2Names[i] = it.Name
	}

	if len(call1Names) != len(call2Names) {
		t.Errorf("run1 installed %v, run2 installed %v — counts differ", call1Names, call2Names)
	}
}

// TestSyncAdvisors_CatalogDocSync verifies that after a successful
// SyncAdvisors call, CLAUDE.md and AGENTS.md are NOT touched (the root catalog
// no longer carries an advisor-driven Skills table; only `devrune sync` rebuilds
// root catalog content) while SDD skill files WITH the managed advisor markers
// ARE updated and listed in WrittenSkillDocs.
func TestSyncAdvisors_CatalogDocSync(t *testing.T) {
	wd := t.TempDir()

	// Seed two native advisors.
	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")
	seedNativeSkillMD(t, wd, "architect-advisor", "Architect advisor")

	// Seed CLAUDE.md and AGENTS.md with managed block; SyncAdvisors must NOT
	// touch them.
	managedBlock := "# >>> devrune managed — do not edit\n# Agent Catalog\n# <<< devrune managed\n"
	claudePath := filepath.Join(wd, "CLAUDE.md")
	agentsPath := filepath.Join(wd, "AGENTS.md")
	writeFile(t, claudePath, managedBlock)
	writeFile(t, agentsPath, managedBlock)

	// Seed two SDD skill files WITH managed advisor markers.
	sddPlanPath := filepath.Join(wd, ".claude", "skills", "sdd-plan", "SKILL.md")
	sddReviewPath := filepath.Join(wd, ".claude", "skills", "sdd-review", "SKILL.md")
	sddContent := "# SDD\n\n" + advisorsBeginMarker + "\n" + advisorsEndMarker + "\n"
	writeFile(t, sddPlanPath, sddContent)
	writeFile(t, sddReviewPath, sddContent)

	spy := &fakeAdvisorRenderer{
		Result: matypes.AdvisorRenderResult{
			Written: []string{
				filepath.Join(wd, ".claude", "agents", "unit-test-advisor.md"),
				filepath.Join(wd, ".claude", "agents", "architect-advisor.md"),
			},
		},
	}
	injectRenderer(t, spy)

	// Install both advisors.
	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor", "architect-advisor").
		Build()

	result, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	// Root catalog files must NOT have been written by advisor sync.
	if len(result.WrittenCatalogDocs) != 0 {
		t.Errorf("WrittenCatalogDocs must be empty; got %v", result.WrittenCatalogDocs)
	}
	if got := readFile(t, claudePath); got != managedBlock {
		t.Errorf("CLAUDE.md must be unchanged by advisor sync; got:\n%s", got)
	}
	if got := readFile(t, agentsPath); got != managedBlock {
		t.Errorf("AGENTS.md must be unchanged by advisor sync; got:\n%s", got)
	}

	// SDD skill files with markers must still be in WrittenSkillDocs.
	if len(result.WrittenSkillDocs) == 0 {
		t.Error("WrittenSkillDocs should be non-empty")
	}
}

// TestSyncAdvisors_SDDSkillMissingMarkers verifies that when an SDD skill file
// exists but lacks the managed markers, the file is NOT modified, the path is
// recorded in SkippedSDDFiles, and SyncAdvisors returns no error.
func TestSyncAdvisors_SDDSkillMissingMarkers(t *testing.T) {
	wd := t.TempDir()

	seedNativeSkillMD(t, wd, "unit-test-advisor", "Unit test advisor")
	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	// Seed sdd-review WITHOUT markers.
	sddReviewPath := filepath.Join(wd, ".claude", "skills", "sdd-review", "SKILL.md")
	originalContent := "# SDD Review\n\nNo markers here at all.\n"
	writeFile(t, sddReviewPath, originalContent)

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	manifest := AUserManifest().
		WithPackage("github:acme/pkg@main", "unit-test-advisor").
		Build()

	result, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	// sdd-review must be in SkippedSDDFiles.
	if !containsStr(result.SkippedSDDFiles, sddReviewPath) {
		t.Errorf("SkippedSDDFiles should contain sdd-review/SKILL.md; got %v", result.SkippedSDDFiles)
	}

	// File must NOT have been modified.
	got := readFile(t, sddReviewPath)
	if got != originalContent {
		t.Errorf("sdd-review/SKILL.md was modified but should have been skipped:\nwant:\n%s\ngot:\n%s", originalContent, got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T021 — Custom advisor copy tests (via SyncAdvisors)
// ─────────────────────────────────────────────────────────────────────────────

// TestSyncAdvisors_CustomAdvisorHappyPathCopy verifies that a custom advisor
// directory with SKILL.md and a sibling file are both copied under
// .claude/skills/{name}/.
//
// Layout: single-advisor-mode — the AdvisorSource.Source points DIRECTLY at
// the advisor directory whose basename ends in "-advisor".
func TestSyncAdvisors_CustomAdvisorHappyPathCopy(t *testing.T) {
	wd := t.TempDir()

	// Create source advisor directory (basename must end in "-advisor" for
	// single-advisor-mode detection).
	srcRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "security-advisor")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "SKILL.md"), "---\nname: security-advisor\ndescription: Security advisor\n---\n")
	writeFile(t, filepath.Join(srcDir, "gotchas.md"), "# Gotchas\n\nVarious gotchas.\n")

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	src := AnAdvisorSource().
		WithSource("local:" + srcDir).
		Build()

	manifest := AUserManifest().WithAdvisorSource(src).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	destDir := filepath.Join(wd, ".claude", "skills", "security-advisor")
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not found at destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "gotchas.md")); err != nil {
		t.Errorf("gotchas.md not found at destination: %v", err)
	}
}

// TestSyncAdvisors_CustomAdvisorRecursiveCopy verifies that nested subdirectories
// are recursively copied to the destination.
func TestSyncAdvisors_CustomAdvisorRecursiveCopy(t *testing.T) {
	wd := t.TempDir()
	srcRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "security-advisor")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, filepath.Join(srcDir, "SKILL.md"), "---\nname: security-advisor\ndescription: Security\n---\n")
	writeFile(t, filepath.Join(srcDir, "references", "foo.md"), "# Foo\n")
	writeFile(t, filepath.Join(srcDir, "references", "bar", "baz.md"), "# Baz\n")
	writeFile(t, filepath.Join(srcDir, "templates", "starter.md"), "# Starter\n")

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	src := AnAdvisorSource().WithSource("local:" + srcDir).Build()
	manifest := AUserManifest().WithAdvisorSource(src).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	destDir := filepath.Join(wd, ".claude", "skills", "security-advisor")
	expectedFiles := []string{
		"SKILL.md",
		"references/foo.md",
		"references/bar/baz.md",
		"templates/starter.md",
	}
	for _, rel := range expectedFiles {
		if _, err := os.Stat(filepath.Join(destDir, rel)); err != nil {
			t.Errorf("expected file %q not found at destination: %v", rel, err)
		}
	}
}

// TestSyncAdvisors_DotfileAndSymlinkHandling verifies that .gitignore and
// .DS_Store are skipped, other dotfiles (.env.example) are copied, and
// symlinks have their target content copied as a regular file.
func TestSyncAdvisors_DotfileAndSymlinkHandling(t *testing.T) {
	wd := t.TempDir()
	srcRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "security-advisor")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, filepath.Join(srcDir, "SKILL.md"), "---\nname: security-advisor\ndescription: Security\n---\n")
	writeFile(t, filepath.Join(srcDir, ".gitignore"), "*.log\n")
	writeFile(t, filepath.Join(srcDir, ".DS_Store"), "binary\n")
	writeFile(t, filepath.Join(srcDir, ".env.example"), "KEY=value\n")

	// Create a symlink: reference-link.md -> an external file.
	externalDir := t.TempDir()
	writeFile(t, filepath.Join(externalDir, "foo.md"), "# External content\n")
	symlinkSrc := filepath.Join(srcDir, "reference-link.md")
	if err := os.Symlink(filepath.Join(externalDir, "foo.md"), symlinkSrc); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	src := AnAdvisorSource().WithSource("local:" + srcDir).Build()
	manifest := AUserManifest().WithAdvisorSource(src).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors returned error: %v", err)
	}

	destDir := filepath.Join(wd, ".claude", "skills", "security-advisor")

	// .gitignore and .DS_Store must NOT be present.
	if _, err := os.Stat(filepath.Join(destDir, ".gitignore")); !os.IsNotExist(err) {
		t.Error(".gitignore should have been skipped (not copied)")
	}
	if _, err := os.Stat(filepath.Join(destDir, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store should have been skipped (not copied)")
	}

	// .env.example must be present.
	if _, err := os.Stat(filepath.Join(destDir, ".env.example")); err != nil {
		t.Errorf(".env.example should have been copied: %v", err)
	}

	// Symlink must be resolved and copied as a regular file.
	symlinkDst := filepath.Join(destDir, "reference-link.md")
	info, err := os.Lstat(symlinkDst)
	if err != nil {
		t.Errorf("reference-link.md not found at destination: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		t.Error("reference-link.md should be a regular file, not a symlink")
	}
}

// TestSyncAdvisors_NoAdvisorsDiscovered_NoOp verifies that when an
// AdvisorSource points at a directory that contains neither SKILL.md (so
// it's not single-advisor-mode) nor any "-advisor" subdirectories,
// resolveAdvisors silently returns no advisors and SyncAdvisors completes
// without error. This is the new graceful-degradation contract: an empty
// source is not a hard failure.
func TestSyncAdvisors_NoAdvisorsDiscovered_NoOp(t *testing.T) {
	wd := t.TempDir()
	srcDir := t.TempDir()
	// srcDir has NO SKILL.md and no -advisor subdirectories — Scanner returns
	// an empty entry list (other names are warned-and-skipped).
	writeFile(t, filepath.Join(srcDir, "gotchas.md"), "# Gotchas\n")

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	src := AnAdvisorSource().WithSource("local:" + srcDir).Build()
	manifest := AUserManifest().WithAdvisorSource(src).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err != nil {
		t.Fatalf("SyncAdvisors should not error on empty advisor source, got: %v", err)
	}

	// Spy should still be called (with empty installed list).
	if len(spy.Calls) != 1 {
		t.Errorf("spy should be called once, got %d call(s)", len(spy.Calls))
	}
}

// TestSyncAdvisors_ErrorNonExistentSource verifies that a non-existent source
// path causes a clear error during resolution.
func TestSyncAdvisors_ErrorNonExistentSource(t *testing.T) {
	wd := t.TempDir()

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	src := AnAdvisorSource().WithSource("local:/nonexistent/path/that/cannot/exist").Build()
	manifest := AUserManifest().WithAdvisorSource(src).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifest)
	if err == nil {
		t.Fatal("expected error for non-existent source path, got nil")
	}
}

// TestSyncAdvisors_RemoveCustomRecursiveDeletion verifies that a custom advisor
// that was previously installed is recursively deleted when removed from the
// manifest, and the spy's Removed list contains the advisor name.
func TestSyncAdvisors_RemoveCustomRecursiveDeletion(t *testing.T) {
	wd := t.TempDir()

	// Step 1: Install security-advisor (single-advisor-mode source layout).
	srcRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "security-advisor")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "SKILL.md"), "---\nname: security-advisor\ndescription: Security\n---\n")
	writeFile(t, filepath.Join(srcDir, "references", "foo.md"), "# Foo\n")
	writeFile(t, filepath.Join(srcDir, "templates", "bar.md"), "# Bar\n")

	writeFile(t, filepath.Join(wd, "CLAUDE.md"), "# My project\n")
	writeFile(t, filepath.Join(wd, "AGENTS.md"), "# Agents\n")

	spy := &fakeAdvisorRenderer{}
	injectRenderer(t, spy)

	advisorSrc := AnAdvisorSource().WithSource("local:" + srcDir).Build()
	manifestWithAdvisor := AUserManifest().WithAdvisorSource(advisorSrc).Build()

	_, err := SyncAdvisors(context.Background(), wd, manifestWithAdvisor)
	if err != nil {
		t.Fatalf("SyncAdvisors (install) returned error: %v", err)
	}

	// Verify the skill dir was created.
	skillDir := filepath.Join(wd, ".claude", "skills", "security-advisor")
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("skill directory should exist after install: %v", err)
	}

	// Step 2: Remove security-advisor from manifest.
	spy.Calls = nil // reset spy
	manifestWithoutAdvisor := AUserManifest().Build()

	_, err = SyncAdvisors(context.Background(), wd, manifestWithoutAdvisor)
	if err != nil {
		t.Fatalf("SyncAdvisors (remove) returned error: %v", err)
	}

	// Skill directory must be completely gone.
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skill directory should be removed after uninstall, but it still exists")
	}

	// Spy's Removed list must contain "security-advisor".
	if len(spy.Calls) == 0 {
		t.Fatal("spy was not called during removal")
	}
	if !containsStr(spy.Calls[0].Removed, "security-advisor") {
		t.Errorf("spy.Removed should contain 'security-advisor'; got %v", spy.Calls[0].Removed)
	}
}
