package devrune_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/cache"
	"github.com/davidarce/devrune/internal/materialize"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/resolve"
	"github.com/davidarce/devrune/internal/state"
)

// TestE2E_ResolveAndInstall exercises the full DevRune pipeline:
//  1. Parse a devrune.yaml manifest
//  2. Resolve packages via LocalFetcher + FileCacheStore → write lockfile
//  3. Install via Materializer with ClaudeRenderer
//  4. Assert workspace artifacts are correct
//  5. Reinstall (idempotency check)
func TestE2E_ResolveAndInstall(t *testing.T) {
	// Create an isolated project root (all writes go here).
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	// -----------------------------------------------------------------------
	// Step 1: Create fixture package directory with skills and rules.
	//
	// The LocalFetcher adds a "local/" prefix to tar entries so that
	// extractTarGz's stripFirstComponent removes it cleanly, preserving
	// the actual directory structure (skills/, rules/, etc.).
	// -----------------------------------------------------------------------
	pkgDir := filepath.Join(projectRoot, "fixtures", "my-pkg")
	pkgContent := pkgDir
	if err := os.MkdirAll(pkgContent, 0o755); err != nil {
		t.Fatalf("mkdir pkg content: %v", err)
	}

	// Skill: git-commit
	gitCommitDir := filepath.Join(pkgContent, "skills", "git-commit")
	if err := os.MkdirAll(gitCommitDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	gitCommitSkill := `---
name: git:commit
description: "Automate git commits following Conventional Commits."
allowed-tools:
  - Bash(git:*)
model: sonnet
---

# git:commit Skill

Automate git commits following Conventional Commits specification.
`
	if err := os.WriteFile(filepath.Join(gitCommitDir, "SKILL.md"), []byte(gitCommitSkill), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Skill: architect-adviser
	architectDir := filepath.Join(pkgContent, "skills", "architect-adviser")
	if err := os.MkdirAll(architectDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	architectSkill := `---
name: architect-adviser
description: "Clean architecture patterns: hexagonal, DDD, ports and adapters."
allowed-tools:
  - Read
  - Grep
model: sonnet
---

# architect-adviser Skill

Provides clean architecture guidance.
`
	if err := os.WriteFile(filepath.Join(architectDir, "SKILL.md"), []byte(architectSkill), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Rule: clean-architecture
	ruleDir := filepath.Join(pkgContent, "rules", "architecture", "clean-architecture")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatalf("mkdir rule: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "clean-architecture-rules.md"), []byte("# Clean Architecture Rules\n"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 2: Create fixture workflow directory.
	// -----------------------------------------------------------------------
	wfDir := filepath.Join(projectRoot, "fixtures", "my-workflow")
	wfContent := wfDir
	if err := os.MkdirAll(wfContent, 0o755); err != nil {
		t.Fatalf("mkdir workflow content: %v", err)
	}

	// workflow.yaml
	workflowYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  description: "Spec-Driven Development workflow"
  version: "1.0.0"
components:
  skills:
    - sdd-explore
    - sdd-plan
  entrypoint: "ORCHESTRATOR.md"
`
	if err := os.WriteFile(filepath.Join(wfContent, "workflow.yaml"), []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	// ORCHESTRATOR.md
	if err := os.WriteFile(filepath.Join(wfContent, "ORCHESTRATOR.md"), []byte("# SDD Orchestrator\n\nInstructions here.\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	// sdd-explore skill
	sddExploreDir := filepath.Join(wfContent, "sdd-explore")
	if err := os.MkdirAll(sddExploreDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-explore: %v", err)
	}
	sddExploreSkill := `---
name: sdd-explore
description: "SDD explore phase: investigate and discover."
allowed-tools:
  - Read
  - Grep
  - Glob
model: sonnet
---

# sdd-explore

Explore and investigate the codebase.
`
	if err := os.WriteFile(filepath.Join(sddExploreDir, "SKILL.md"), []byte(sddExploreSkill), 0o644); err != nil {
		t.Fatalf("write sdd-explore SKILL.md: %v", err)
	}

	// sdd-plan skill
	sddPlanDir := filepath.Join(wfContent, "sdd-plan")
	if err := os.MkdirAll(sddPlanDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	sddPlanSkill := `---
name: sdd-plan
description: "SDD plan phase: create implementation plan."
allowed-tools:
  - Read
  - Write
model: sonnet
---

# sdd-plan

Create a detailed implementation plan.
`
	if err := os.WriteFile(filepath.Join(sddPlanDir, "SKILL.md"), []byte(sddPlanSkill), 0o644); err != nil {
		t.Fatalf("write sdd-plan SKILL.md: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 3: Write devrune.yaml manifest referencing local fixtures.
	// Use absolute paths because parseLocalRef stores the raw path value
	// and LocalFetcher resolves it relative to the process CWD, not baseDir.
	// -----------------------------------------------------------------------
	manifestYAML := "schemaVersion: devrune/v1\n" +
		"packages:\n" +
		"  - source: \"local:" + pkgDir + "\"\n" +
		"workflows:\n" +
		"  - \"local:" + wfDir + "\"\n" +
		"agents:\n" +
		"  - name: claude\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 4: Parse manifest.
	// -----------------------------------------------------------------------
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 5: Resolve — parse manifest → lockfile.
	// -----------------------------------------------------------------------
	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Verify lockfile.
	if lockfile.SchemaVersion != "devrune/lock/v1" {
		t.Errorf("lockfile.SchemaVersion = %q, want %q", lockfile.SchemaVersion, "devrune/lock/v1")
	}
	if !strings.HasPrefix(lockfile.ManifestHash, "sha256:") {
		t.Errorf("lockfile.ManifestHash = %q, missing sha256: prefix", lockfile.ManifestHash)
	}
	if len(lockfile.Packages) != 1 {
		t.Fatalf("lockfile.Packages count = %d, want 1", len(lockfile.Packages))
	}
	if len(lockfile.Workflows) != 1 {
		t.Fatalf("lockfile.Workflows count = %d, want 1", len(lockfile.Workflows))
	}
	if lockfile.Workflows[0].Name != "sdd" {
		t.Errorf("workflow name = %q, want %q", lockfile.Workflows[0].Name, "sdd")
	}

	// Verify package has skills and rules.
	pkg := lockfile.Packages[0]
	var skillNames []string
	var ruleNames []string
	for _, item := range pkg.Contents {
		switch item.Kind {
		case model.KindSkill:
			skillNames = append(skillNames, item.Name)
		case model.KindRule:
			ruleNames = append(ruleNames, item.Name)
		}
	}
	if len(skillNames) != 2 {
		t.Errorf("skills = %v, want 2 skills (git-commit, architect-adviser)", skillNames)
	}
	if len(ruleNames) != 1 {
		t.Errorf("rules = %v, want 1 rule", ruleNames)
	}

	// -----------------------------------------------------------------------
	// Step 6: Serialize lockfile.
	// -----------------------------------------------------------------------
	lockData, err := parse.SerializeLockfile(lockfile)
	if err != nil {
		t.Fatalf("serialize lockfile: %v", err)
	}
	lockPath := filepath.Join(projectRoot, "devrune.lock")
	if err := os.WriteFile(lockPath, lockData, 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	// Verify lockfile was written.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 7: Install — parse lockfile → materialize workspace.
	// -----------------------------------------------------------------------
	// Build an AgentDefinition with absolute workspace path.
	claudeDef := model.AgentDefinition{
		Name:         "claude",
		Type:         "claude",
		Workspace:    filepath.Join(projectRoot, ".claude"),
		SkillDir:     "skills",
		CommandDir:   "commands",
		RulesDir:     "rules",
		CatalogFile:  "CLAUDE.md",
		DefaultRules: "individual",
	}

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{claudeDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	linker, err := materialize.NewLinker("copy")
	if err != nil {
		t.Fatalf("linker: %v", err)
	}

	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	agents := []model.AgentRef{{Name: "claude"}}
	installCfg := model.InstallConfig{}

	if err := mat.Install(context.Background(), lockfile, agents, installCfg); err != nil {
		t.Fatalf("install: %v", err)
	}

	// -----------------------------------------------------------------------
	// Step 8: Verify workspace artifacts.
	// -----------------------------------------------------------------------
	claudeDir := filepath.Join(projectRoot, ".claude")

	// Skills directory must exist.
	skillsDir := filepath.Join(claudeDir, "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Errorf(".claude/skills/ not created: %v", err)
	}

	// git-commit skill must have SKILL.md.
	gitCommitInstalled := filepath.Join(skillsDir, "git-commit", "SKILL.md")
	if _, err := os.Stat(gitCommitInstalled); err != nil {
		t.Errorf("git-commit/SKILL.md not installed: %v", err)
	}

	// architect-adviser skill must have SKILL.md.
	architectInstalled := filepath.Join(skillsDir, "architect-adviser", "SKILL.md")
	if _, err := os.Stat(architectInstalled); err != nil {
		t.Errorf("architect-adviser/SKILL.md not installed: %v", err)
	}

	// Installed SKILL.md should have correct frontmatter (name field present).
	skillContent, err := os.ReadFile(gitCommitInstalled)
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillContent), "name: git:commit") {
		t.Errorf("installed SKILL.md missing name field, content:\n%s", skillContent)
	}
	// Claude strips non-Claude fields; model field should be dropped.
	// (Claude keeps model field — it's not in the drop list)

	// SDD workflow skills must be installed.
	sddSkillsDir := filepath.Join(skillsDir, "sdd")
	if _, err := os.Stat(sddSkillsDir); err != nil {
		t.Errorf("sdd workflow dir not created: %v", err)
	}

	sddExploreInstalled := filepath.Join(sddSkillsDir, "sdd-explore", "SKILL.md")
	if _, err := os.Stat(sddExploreInstalled); err != nil {
		t.Errorf("sdd-explore/SKILL.md not installed: %v", err)
	}

	// ORCHESTRATOR.md should be installed as-is.
	orchestratorInstalled := filepath.Join(sddSkillsDir, "ORCHESTRATOR.md")
	if _, err := os.Stat(orchestratorInstalled); err != nil {
		t.Errorf("ORCHESTRATOR.md not installed: %v", err)
	}

	// Catalog file must exist.
	catalogPath := filepath.Join(claudeDir, "CLAUDE.md")
	if _, err := os.Stat(catalogPath); err != nil {
		t.Errorf("CLAUDE.md not created: %v", err)
	}

	// State file must exist.
	stateFilePath := filepath.Join(projectRoot, ".devrune", "state.yaml")
	if _, err := os.Stat(stateFilePath); err != nil {
		t.Errorf(".devrune/state.yaml not created: %v", err)
	}

	// State must reference the manifest hash.
	savedState, err := stateMgr.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if savedState.LockHash != lockfile.ManifestHash {
		t.Errorf("state.LockHash = %q, want %q", savedState.LockHash, lockfile.ManifestHash)
	}
	if len(savedState.ActiveAgents) != 1 || savedState.ActiveAgents[0] != "claude" {
		t.Errorf("state.ActiveAgents = %v, want [claude]", savedState.ActiveAgents)
	}
	if len(savedState.ActiveWorkflows) != 1 || savedState.ActiveWorkflows[0] != "sdd" {
		t.Errorf("state.ActiveWorkflows = %v, want [sdd]", savedState.ActiveWorkflows)
	}

	// -----------------------------------------------------------------------
	// Step 9: Idempotency — reinstall should succeed with same result.
	// -----------------------------------------------------------------------
	if err := mat.Install(context.Background(), lockfile, agents, installCfg); err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	// Artifacts should still be present after reinstall.
	if _, err := os.Stat(gitCommitInstalled); err != nil {
		t.Errorf("git-commit/SKILL.md missing after reinstall: %v", err)
	}
	if _, err := os.Stat(catalogPath); err != nil {
		t.Errorf("CLAUDE.md missing after reinstall: %v", err)
	}

	// State should still be consistent.
	savedState2, err := stateMgr.Read()
	if err != nil {
		t.Fatalf("read state after reinstall: %v", err)
	}
	if savedState2.LockHash != lockfile.ManifestHash {
		t.Errorf("state.LockHash after reinstall = %q, want %q", savedState2.LockHash, lockfile.ManifestHash)
	}
}

// TestE2E_PackageOnly exercises the pipeline with a package only (no workflow).
func TestE2E_PackageOnly(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	// Create fixture package with a single skill.
	pkgDir := filepath.Join(projectRoot, "fixtures", "simple-pkg")
	skillDir := filepath.Join(pkgDir, "skills", "unit-test-adviser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: unit-test-adviser\ndescription: Unit test guidance.\n---\n# Unit Test Adviser\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	manifestYAML := "schemaVersion: devrune/v1\n" +
		"packages:\n" +
		"  - source: \"local:" + pkgDir + "\"\n" +
		"agents:\n" +
		"  - name: claude\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(lockfile.Packages) != 1 {
		t.Fatalf("packages count = %d, want 1", len(lockfile.Packages))
	}

	claudeDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(projectRoot, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{claudeDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}
	linker, _ := materialize.NewLinker("copy")
	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	if err := mat.Install(context.Background(), lockfile, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}); err != nil {
		t.Fatalf("install: %v", err)
	}

	installed := filepath.Join(projectRoot, ".claude", "skills", "unit-test-adviser", "SKILL.md")
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !strings.Contains(string(data), "unit-test-adviser") {
		t.Errorf("installed skill missing name, content:\n%s", data)
	}
}

// buildSddWorkflowFixture creates a minimal SDD workflow cache directory containing
// sdd-plan/SKILL.md, ORCHESTRATOR.md, _shared/shared.md, and REGISTRY.md.
// Returns the fixture directory path.
func buildSddWorkflowFixture(t *testing.T) string {
	t.Helper()
	wfDir := t.TempDir()

	workflowYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  description: "Spec-Driven Development"
  version: "1.0.0"
components:
  skills:
    - sdd-plan
  entrypoint: "ORCHESTRATOR.md"
  registry: "REGISTRY.md"
  roles:
    - name: sdd-planner
      kind: subagent
      skill: sdd-plan
      model: sonnet
    - name: sdd-orchestrator
      kind: orchestrator
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowFixture: write workflow.yaml: %v", err)
	}

	sddPlanDir := filepath.Join(wfDir, "sdd-plan")
	if err := os.MkdirAll(sddPlanDir, 0o755); err != nil {
		t.Fatalf("buildSddWorkflowFixture: mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sddPlanDir, "SKILL.md"),
		[]byte("---\nname: sdd-plan\ndescription: Create a plan\n---\n# sdd-plan\n"), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowFixture: write sdd-plan/SKILL.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wfDir, "ORCHESTRATOR.md"),
		[]byte("# SDD Orchestrator\n\nCoordinates SDD.\n"), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowFixture: write ORCHESTRATOR.md: %v", err)
	}

	sharedDir := filepath.Join(wfDir, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("buildSddWorkflowFixture: mkdir _shared: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "shared.md"),
		[]byte("# Shared\n"), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowFixture: write _shared/shared.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wfDir, "REGISTRY.md"),
		[]byte("# SDD Registry\n\n| sdd-plan | Plan |\n"), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowFixture: write REGISTRY.md: %v", err)
	}

	return wfDir
}

// TestE2E_FactoryParityLayout verifies that an SDD workflow installed for the Factory
// agent produces the correct file tree and does NOT create the legacy .factory/droids/
// path. This is the T018 Factory parity check.
func TestE2E_FactoryParityLayout(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	wfDir := buildSddWorkflowFixture(t)

	manifestYAML := "schemaVersion: devrune/v1\n" +
		"workflows:\n" +
		"  - \"local:" + wfDir + "\"\n" +
		"agents:\n" +
		"  - name: factory\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifestData, _ := os.ReadFile(manifestPath)
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	factoryDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   filepath.Join(projectRoot, ".factory"),
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{factoryDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	linker, _ := materialize.NewLinker("copy")
	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	if err := mat.Install(context.Background(), lockfile, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}); err != nil {
		t.Fatalf("install: %v", err)
	}

	factoryDir := filepath.Join(projectRoot, ".factory")

	// POSITIVE: orchestrator must be at .factory/skills/sdd-orchestrator/ORCHESTRATOR.md
	orchestratorPath := filepath.Join(factoryDir, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
	if _, err := os.Stat(orchestratorPath); err != nil {
		t.Errorf("POSITIVE: .factory/skills/sdd-orchestrator/ORCHESTRATOR.md must exist: %v", err)
	}

	// POSITIVE: skill installed flat under .factory/skills/
	sddPlanPath := filepath.Join(factoryDir, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(sddPlanPath); err != nil {
		t.Errorf("POSITIVE: .factory/skills/sdd-plan/SKILL.md must exist: %v", err)
	}

	// NEGATIVE: old legacy .factory/droids/ path must NOT exist
	droidsPath := filepath.Join(factoryDir, "droids", "sdd-orchestrator.md")
	if _, err := os.Stat(droidsPath); err == nil {
		t.Errorf("NEGATIVE: .factory/droids/sdd-orchestrator.md must NOT exist (legacy path)")
	}
}

// TestE2E_OpenCodeParityLayout verifies that an SDD workflow installed for OpenCode
// produces synthesized agent entries in opencode.json and does NOT create the legacy
// .opencode/agents/ path. This is the T018 OpenCode parity check.
func TestE2E_OpenCodeParityLayout(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	wfDir := buildSddWorkflowFixture(t)

	manifestYAML := "schemaVersion: devrune/v1\n" +
		"workflows:\n" +
		"  - \"local:" + wfDir + "\"\n" +
		"agents:\n" +
		"  - name: opencode\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifestData, _ := os.ReadFile(manifestPath)
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	opencodeDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   filepath.Join(projectRoot, ".opencode"),
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{opencodeDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	linker, _ := materialize.NewLinker("copy")
	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	if err := mat.Install(context.Background(), lockfile, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}); err != nil {
		t.Fatalf("install: %v", err)
	}

	opencodeDir := filepath.Join(projectRoot, ".opencode")

	// POSITIVE: opencode.json must exist and contain sdd-planner agent entry
	opencodeJSONPath := filepath.Join(opencodeDir, "opencode.json")
	jsonData, err := os.ReadFile(opencodeJSONPath)
	if err != nil {
		t.Fatalf("POSITIVE: opencode.json must exist: %v", err)
	}
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "sdd-planner") {
		t.Errorf("POSITIVE: opencode.json must contain sdd-planner agent entry, got:\n%s", jsonStr)
	}

	// POSITIVE: skill installed under .opencode/skills/
	sddPlanPath := filepath.Join(opencodeDir, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(sddPlanPath); err != nil {
		t.Errorf("POSITIVE: .opencode/skills/sdd-plan/SKILL.md must exist: %v", err)
	}

	// NEGATIVE: old legacy .opencode/agents/ path must NOT exist
	agentsLegacyPath := filepath.Join(opencodeDir, "agents", "sdd-plan")
	if _, err := os.Stat(agentsLegacyPath); err == nil {
		t.Errorf("NEGATIVE: .opencode/agents/sdd-plan/ must NOT exist (legacy path)")
	}
}

// buildE2EMCPFixtureDir creates a minimal MCP fixture directory with a mcp.yaml that
// uses an env var placeholder. The mcp.yaml uses ${VAR} style (canonical Claude format)
// which each renderer transforms to its own style during RenderMCPs.
// Returns the absolute path to the fixture directory.
func buildE2EMCPFixtureDir(t *testing.T) string {
	t.Helper()
	mcpDir := t.TempDir()
	mcpYAML := "command: npx\nargs:\n  - -y\n  - test-mcp-server\nenv:\n  TEST_API_KEY: \"${TEST_API_KEY}\"\n"
	if err := os.WriteFile(filepath.Join(mcpDir, "mcp.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("buildE2EMCPFixtureDir: write mcp.yaml: %v", err)
	}
	return mcpDir
}

// runMCPE2E is a shared helper that exercises the full DevRune pipeline for a single
// agent with MCPs: parse manifest → resolve → install → return the parsed MCP JSON file.
// agentDef is the AgentDefinition to use (includes MCP config).
// mcpOutputPath is the absolute path to the expected MCP config file.
func runMCPE2E(t *testing.T, projectRoot, cacheDir string, agentDef model.AgentDefinition, mcpOutputPath string) map[string]interface{} {
	t.Helper()

	mcpDir := buildE2EMCPFixtureDir(t)

	manifestYAML := "schemaVersion: devrune/v1\n" +
		"mcps:\n" +
		"  - source: \"local:" + mcpDir + "\"\n" +
		"agents:\n" +
		"  - name: " + agentDef.Name + "\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(lockfile.MCPs) != 1 {
		t.Fatalf("lockfile.MCPs count = %d, want 1", len(lockfile.MCPs))
	}

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{agentDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	linker, _ := materialize.NewLinker("copy")
	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	if err := mat.Install(context.Background(), lockfile, []model.AgentRef{{Name: agentDef.Name}}, model.InstallConfig{}); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Read and parse the MCP config file.
	data, err := os.ReadFile(mcpOutputPath)
	if err != nil {
		t.Fatalf("MCP config file not found at %q: %v", mcpOutputPath, err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse MCP JSON at %q: %v\ncontent: %s", mcpOutputPath, err, data)
	}

	return parsed
}

// TestE2E_FactoryMCPInstall exercises the full pipeline for a Factory agent with an
// MCP: parse manifest → resolve → install → assert mcp.json structure and format.
//
// Expected: <workspace>/mcp.json with "mcpServers" root key, env key "env", ${VAR} style.
func TestE2E_FactoryMCPInstall(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	factoryDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   filepath.Join(projectRoot, ".factory"),
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
		MCP: &model.MCPConfig{
			FilePath:    "mcp.json",
			RootKey:     "mcpServers",
			EnvKey:      "env",
			EnvVarStyle: "${VAR}",
		},
	}

	mcpOutputPath := filepath.Join(projectRoot, ".factory", "mcp.json")
	parsed := runMCPE2E(t, projectRoot, cacheDir, factoryDef, mcpOutputPath)

	// POSITIVE: root key must be "mcpServers" (Factory format).
	servers, ok := parsed["mcpServers"]
	if !ok {
		t.Fatalf("POSITIVE: Factory mcp.json must have 'mcpServers' root key; got keys: %v", keysOf(parsed))
	}
	serversMap, ok := servers.(map[string]interface{})
	if !ok {
		t.Fatalf("'mcpServers' must be an object")
	}

	// POSITIVE: must have exactly one MCP entry (named after the fixture dir basename).
	if len(serversMap) != 1 {
		t.Errorf("POSITIVE: Factory mcp.json should have 1 MCP server entry, got %d: %v", len(serversMap), keysOf(serversMap))
	}

	// POSITIVE: env key must be "env" and env var must use ${VAR} style.
	for mcpName, rawEntry := range serversMap {
		entry, ok := rawEntry.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q must be an object", mcpName)
			continue
		}
		envRaw, hasEnv := entry["env"]
		if !hasEnv {
			t.Errorf("POSITIVE: Factory MCP entry %q must have 'env' key; got keys: %v", mcpName, keysOf(entry))
			continue
		}
		envMap, ok := envRaw.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q 'env' must be an object", mcpName)
			continue
		}
		for varName, varVal := range envMap {
			s, _ := varVal.(string)
			if !strings.HasPrefix(s, "${") || !strings.HasSuffix(s, "}") {
				t.Errorf("POSITIVE: Factory env var %q must use ${VAR} style, got %q", varName, s)
			}
			// NEGATIVE: must NOT use OpenCode or Copilot format.
			if strings.HasPrefix(s, "{env:") {
				t.Errorf("NEGATIVE: Factory env var must not use OpenCode '{env:VAR}' format, got %q", s)
			}
		}

		// NEGATIVE: must NOT have "environment" key (OpenCode-specific).
		if _, hasEnvironment := entry["environment"]; hasEnvironment {
			t.Errorf("NEGATIVE: Factory mcp.json must not use 'environment' key (OpenCode-specific)")
		}
	}

	// NEGATIVE: must NOT have "servers" root key (Copilot format).
	if _, ok := parsed["servers"]; ok {
		t.Errorf("NEGATIVE: Factory mcp.json must not have 'servers' root key (Copilot format)")
	}
	// NEGATIVE: must NOT have "mcp" root key (OpenCode format).
	if _, ok := parsed["mcp"]; ok {
		t.Errorf("NEGATIVE: Factory mcp.json must not have 'mcp' root key (OpenCode format)")
	}
}

// TestE2E_OpenCodeMCPInstall exercises the full pipeline for an OpenCode agent with
// an MCP: parse manifest → resolve → install → assert opencode.json structure.
//
// Expected: <workspace>/opencode.json with "mcp" root key, env key "environment",
// {env:VAR} placeholder style.
func TestE2E_OpenCodeMCPInstall(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	opencodeDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   filepath.Join(projectRoot, ".opencode"),
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
		MCP: &model.MCPConfig{
			FilePath:    "opencode.json",
			RootKey:     "mcp",
			EnvKey:      "environment",
			EnvVarStyle: "{env:VAR}",
		},
	}

	mcpOutputPath := filepath.Join(projectRoot, ".opencode", "opencode.json")
	parsed := runMCPE2E(t, projectRoot, cacheDir, opencodeDef, mcpOutputPath)

	// POSITIVE: root key must be "mcp" (OpenCode format).
	mcpSection, ok := parsed["mcp"]
	if !ok {
		t.Fatalf("POSITIVE: OpenCode opencode.json must have 'mcp' root key; got keys: %v", keysOf(parsed))
	}
	mcpMap, ok := mcpSection.(map[string]interface{})
	if !ok {
		t.Fatalf("'mcp' must be an object")
	}

	// POSITIVE: must have exactly one MCP entry.
	if len(mcpMap) != 1 {
		t.Errorf("POSITIVE: OpenCode opencode.json should have 1 MCP server entry, got %d: %v", len(mcpMap), keysOf(mcpMap))
	}

	// POSITIVE: env key must be "environment" and env var must use {env:VAR} style.
	for mcpName, rawEntry := range mcpMap {
		entry, ok := rawEntry.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q must be an object", mcpName)
			continue
		}
		envRaw, hasEnvironment := entry["environment"]
		if !hasEnvironment {
			t.Errorf("POSITIVE: OpenCode MCP entry %q must have 'environment' key; got keys: %v", mcpName, keysOf(entry))
			continue
		}
		envMap, ok := envRaw.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q 'environment' must be an object", mcpName)
			continue
		}
		for varName, varVal := range envMap {
			s, _ := varVal.(string)
			if !strings.HasPrefix(s, "{env:") || !strings.HasSuffix(s, "}") {
				t.Errorf("POSITIVE: OpenCode env var %q must use {env:VAR} style, got %q", varName, s)
			}
			// NEGATIVE: must NOT use ${VAR} (Claude/Factory) or ${env:VAR} (Copilot) format.
			if strings.HasPrefix(s, "${") {
				t.Errorf("NEGATIVE: OpenCode env var must not use ${VAR} format, got %q", s)
			}
		}

		// NEGATIVE: must NOT have "env" key (Claude/Factory-specific).
		if _, hasEnv := entry["env"]; hasEnv {
			t.Errorf("NEGATIVE: OpenCode opencode.json must not use 'env' key (use 'environment')")
		}
	}

	// NEGATIVE: must NOT have "mcpServers" root key (Claude/Factory format).
	if _, ok := parsed["mcpServers"]; ok {
		t.Errorf("NEGATIVE: OpenCode opencode.json must not have 'mcpServers' root key")
	}
	// NEGATIVE: must NOT have "servers" root key (Copilot format).
	if _, ok := parsed["servers"]; ok {
		t.Errorf("NEGATIVE: OpenCode opencode.json must not have 'servers' root key (Copilot format)")
	}
}

// TestE2E_CopilotMCPInstall exercises the full pipeline for a Copilot agent with an
// MCP: parse manifest → resolve → install → assert .vscode/mcp.json structure.
//
// Expected: <projectRoot>/.vscode/mcp.json with "servers" root key, env key "env",
// ${env:VAR} placeholder style.
func TestE2E_CopilotMCPInstall(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	copilotDef := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(projectRoot, ".github"),
		SkillDir:    "skills",
		AgentDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
		MCP: &model.MCPConfig{
			FilePath:    "../.vscode/mcp.json",
			RootKey:     "servers",
			EnvKey:      "env",
			EnvVarStyle: "${env:VAR}",
		},
	}

	// Copilot writes to workspace/../.vscode/mcp.json = projectRoot/.vscode/mcp.json
	mcpOutputPath := filepath.Join(projectRoot, ".github", "../.vscode/mcp.json")
	parsed := runMCPE2E(t, projectRoot, cacheDir, copilotDef, mcpOutputPath)

	// POSITIVE: root key must be "servers" (Copilot/VS Code format).
	serversSection, ok := parsed["servers"]
	if !ok {
		t.Fatalf("POSITIVE: Copilot .vscode/mcp.json must have 'servers' root key; got keys: %v", keysOf(parsed))
	}
	serversMap, ok := serversSection.(map[string]interface{})
	if !ok {
		t.Fatalf("'servers' must be an object")
	}

	// POSITIVE: must have exactly one MCP entry.
	if len(serversMap) != 1 {
		t.Errorf("POSITIVE: Copilot .vscode/mcp.json should have 1 MCP server entry, got %d: %v", len(serversMap), keysOf(serversMap))
	}

	// POSITIVE: env key must be "env" and env var must use ${env:VAR} style.
	for mcpName, rawEntry := range serversMap {
		entry, ok := rawEntry.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q must be an object", mcpName)
			continue
		}
		envRaw, hasEnv := entry["env"]
		if !hasEnv {
			t.Errorf("POSITIVE: Copilot MCP entry %q must have 'env' key; got keys: %v", mcpName, keysOf(entry))
			continue
		}
		envMap, ok := envRaw.(map[string]interface{})
		if !ok {
			t.Errorf("MCP entry %q 'env' must be an object", mcpName)
			continue
		}
		for varName, varVal := range envMap {
			s, _ := varVal.(string)
			if !strings.HasPrefix(s, "${env:") || !strings.HasSuffix(s, "}") {
				t.Errorf("POSITIVE: Copilot env var %q must use ${env:VAR} style, got %q", varName, s)
			}
			// NEGATIVE: must NOT use plain ${VAR} (Claude/Factory) or {env:VAR} (OpenCode).
			if strings.HasPrefix(s, "{env:") {
				t.Errorf("NEGATIVE: Copilot env var must not use OpenCode '{env:VAR}' format, got %q", s)
			}
		}

		// NEGATIVE: must NOT have "environment" key (OpenCode-specific).
		if _, hasEnvironment := entry["environment"]; hasEnvironment {
			t.Errorf("NEGATIVE: Copilot .vscode/mcp.json must not use 'environment' key (OpenCode-specific)")
		}
	}

	// NEGATIVE: must NOT have "mcpServers" root key (Claude/Factory format).
	if _, ok := parsed["mcpServers"]; ok {
		t.Errorf("NEGATIVE: Copilot .vscode/mcp.json must not have 'mcpServers' root key")
	}
	// NEGATIVE: must NOT have "mcp" root key (OpenCode format).
	if _, ok := parsed["mcp"]; ok {
		t.Errorf("NEGATIVE: Copilot .vscode/mcp.json must not have 'mcp' root key (OpenCode format)")
	}
}

// keysOf returns the keys of a map[string]interface{} as a slice for error messages.
func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestE2E_CopilotParityLayout verifies that an SDD workflow installed for Copilot
// produces the correct file tree: skills under .github/skills/, orchestrator surfaced
// at .github/agents/sdd-orchestrator.agent.md, and no legacy skill .agent.md files.
// This is the T018 Copilot parity check.
func TestE2E_CopilotParityLayout(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := t.TempDir()

	wfDir := buildSddWorkflowFixture(t)

	manifestYAML := "schemaVersion: devrune/v1\n" +
		"workflows:\n" +
		"  - \"local:" + wfDir + "\"\n" +
		"agents:\n" +
		"  - name: copilot\n"
	manifestPath := filepath.Join(projectRoot, "devrune.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifestData, _ := os.ReadFile(manifestPath)
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(nil, nil, localFetcher)
	cacheStore := cache.NewFileCacheStore(cacheDir)
	resolver := resolve.NewResolver(multiFetcher, cacheStore, projectRoot)

	lockfile, err := resolver.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	copilotDef := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(projectRoot, ".github"),
		SkillDir:    "skills",
		AgentDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{copilotDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	linker, _ := materialize.NewLinker("copy")
	stateMgr := state.NewFileStateManager(projectRoot)
	mat := materialize.NewMaterializer(cacheStore, linker, stateMgr, renderers)

	if err := mat.Install(context.Background(), lockfile, []model.AgentRef{{Name: "copilot"}}, model.InstallConfig{}); err != nil {
		t.Fatalf("install: %v", err)
	}

	githubDir := filepath.Join(projectRoot, ".github")

	// POSITIVE: orchestrator surfaced as .github/agents/sdd-orchestrator.agent.md
	orchestratorAgentPath := filepath.Join(githubDir, "agents", "sdd-orchestrator.agent.md")
	if _, err := os.Stat(orchestratorAgentPath); err != nil {
		t.Errorf("POSITIVE: .github/agents/sdd-orchestrator.agent.md must exist: %v", err)
	}

	// POSITIVE: subagent role surfaced as native .agent.md (T018 — role name, not skill name)
	sddPlannerAgentPath := filepath.Join(githubDir, "agents", "sdd-planner.agent.md")
	if _, err := os.Stat(sddPlannerAgentPath); err != nil {
		t.Errorf("POSITIVE: .github/agents/sdd-planner.agent.md must exist (T018 sub-agent): %v", err)
	}

	// NEGATIVE: SDD subagent skill must NOT be copied to skills/ (T017 — embedded in .agent.md)
	sddPlanSkillPath := filepath.Join(githubDir, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(sddPlanSkillPath); err == nil {
		t.Errorf("NEGATIVE: .github/skills/sdd-plan/SKILL.md must NOT exist (T017: subagent skills are embedded, not copied to skills/)")
	}

	// NEGATIVE: skill-name .agent.md must NOT be created — only role-name .agent.md files exist
	sddPlanAgentPath := filepath.Join(githubDir, "agents", "sdd-plan.agent.md")
	if _, err := os.Stat(sddPlanAgentPath); err == nil {
		t.Errorf("NEGATIVE: .github/agents/sdd-plan.agent.md must NOT exist (use role name sdd-planner, not skill name)")
	}
}
