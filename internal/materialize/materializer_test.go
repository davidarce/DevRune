package materialize_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/materialize"
	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/state"
)

// --- Test doubles ---

// stubCacheStore is a simple in-memory CacheStore for testing.
type stubCacheStore struct {
	entries map[string]string // hash → dir path
}

func newStubCache() *stubCacheStore {
	return &stubCacheStore{entries: make(map[string]string)}
}

func (c *stubCacheStore) Has(hash string) bool {
	_, ok := c.entries[hash]
	return ok
}

func (c *stubCacheStore) Get(hash string) (string, bool) {
	dir, ok := c.entries[hash]
	return dir, ok
}

func (c *stubCacheStore) Store(key string, data []byte) (string, error) {
	return "", nil
}

// stubStateManager is a minimal in-memory StateManager for testing.
type stubStateManager struct {
	written      state.State
	readState    state.State
	acquireErr   error
	releaseErr   error
	writeErr     error
	readErr      error
	lockAcquired bool
	lockReleased bool
}

func (m *stubStateManager) Read() (state.State, error) {
	return m.readState, m.readErr
}

func (m *stubStateManager) Write(s state.State) error {
	m.written = s
	return m.writeErr
}

func (m *stubStateManager) ManagedPaths() ([]string, error) {
	return m.readState.ManagedPaths, nil
}

func (m *stubStateManager) AcquireLock() error {
	m.lockAcquired = true
	return m.acquireErr
}

func (m *stubStateManager) ReleaseLock() error {
	m.lockReleased = true
	return m.releaseErr
}

// stubRenderer is a minimal AgentRenderer that records calls.
type stubRenderer struct {
	name               string
	agentType          string
	def                model.AgentDefinition
	skillErr           error
	catalogErr         error
	settingsErr        error
	finalizeErr        error
	renderedSkills     []string
	catalogCalled      bool
	settingsCalled     bool
	finalizeCalled     bool
	catalogSkills      []model.ContentItem
	catalogRules       []model.ContentItem
	catalogWorkflows   []model.WorkflowManifest
	settingsSkills     []model.ContentItem
	settingsWorkflows  []model.WorkflowManifest
	setInstalledSkills []model.ContentItem
	// workflowManagedPaths controls what InstallWorkflow returns in ManagedPaths.
	workflowManagedPaths []string
}

func newStubRenderer(name, agentType string, def model.AgentDefinition) *stubRenderer {
	return &stubRenderer{
		name:      name,
		agentType: agentType,
		def:       def,
	}
}

func (r *stubRenderer) Name() string                      { return r.name }
func (r *stubRenderer) AgentType() string                 { return r.agentType }
func (r *stubRenderer) Definition() model.AgentDefinition { return r.def }
func (r *stubRenderer) NeedsCopyMode() bool               { return false }
func (r *stubRenderer) WorkspacePaths() matypes.AgentPaths {
	return matypes.AgentPaths{
		Workspace:   r.def.Workspace,
		SkillDir:    r.def.SkillDir,
		CatalogFile: r.def.CatalogFile,
	}
}

func (r *stubRenderer) RenderSkill(canonicalPath, destDir string) error {
	r.renderedSkills = append(r.renderedSkills, canonicalPath)
	if r.skillErr != nil {
		return r.skillErr
	}
	// Create the SKILL.md to simulate a real render.
	_ = os.MkdirAll(destDir, 0o755)
	return os.WriteFile(filepath.Join(destDir, "SKILL.md"), []byte("# stub"), 0o644)
}

func (r *stubRenderer) RenderCommand(cmd model.WorkflowCommand, destDir string) error {
	return nil
}

func (r *stubRenderer) RenderMCPs(mcps []model.LockedMCP, cache matypes.CacheStore, workspaceRoot string) error {
	return nil
}

func (r *stubRenderer) RenderCatalog(skills []model.ContentItem, rules []model.ContentItem, workflows []model.WorkflowManifest, destPath string) error {
	r.catalogCalled = true
	r.catalogSkills = skills
	r.catalogRules = rules
	r.catalogWorkflows = workflows
	if r.catalogErr != nil {
		return r.catalogErr
	}
	_ = os.MkdirAll(filepath.Dir(destPath), 0o755)
	return os.WriteFile(destPath, []byte("# catalog\n"), 0o644)
}

func (r *stubRenderer) RenderSettings(workspaceRoot string, skills []model.ContentItem, workflows []model.WorkflowManifest) error {
	r.settingsCalled = true
	r.settingsSkills = skills
	r.settingsWorkflows = workflows
	return r.settingsErr
}

func (r *stubRenderer) SetInstalledSkills(skills []model.ContentItem) {
	r.setInstalledSkills = skills
}

func (r *stubRenderer) InstallWorkflow(wf model.WorkflowManifest, cachePath, workspaceRoot string) (materialize.WorkflowInstallResult, error) {
	return materialize.WorkflowInstallResult{ManagedPaths: r.workflowManagedPaths}, nil
}

func (r *stubRenderer) Finalize(workspaceRoot string) error {
	r.finalizeCalled = true
	return r.finalizeErr
}

// --- Tests ---

// TestMaterializer_Install_EmptyLockfile verifies that Install succeeds and writes
// state even when the lockfile has no packages.
func TestMaterializer_Install_EmptyLockfile(t *testing.T) {
	tmpDir := t.TempDir()

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CommandDir:  "commands",
		CatalogFile: "CLAUDE.md",
	}

	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(
		cache,
		linker,
		stateMgr,
		map[string]materialize.AgentRenderer{"claude": renderer},
	)

	lock := model.Lockfile{
		SchemaVersion: "devrune/lock/v1",
		ManifestHash:  "sha256:abc123",
	}
	agents := []model.AgentRef{{Name: "claude"}}
	cfg := model.InstallConfig{}

	if err := m.Install(context.Background(), lock, agents, cfg, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// State should have been written.
	if stateMgr.written.LockHash != "sha256:abc123" {
		t.Errorf("state.LockHash = %q, want %q", stateMgr.written.LockHash, "sha256:abc123")
	}
	if len(stateMgr.written.ActiveAgents) != 1 || stateMgr.written.ActiveAgents[0] != "claude" {
		t.Errorf("state.ActiveAgents = %v, want [claude]", stateMgr.written.ActiveAgents)
	}
}

// TestMaterializer_Install_CreatesWorkspaceDirs verifies that workspace directories are created.
func TestMaterializer_Install_CreatesWorkspaceDirs(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".claude")

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   workspace,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "CLAUDE.md",
	}

	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:xxx"}
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir)

	// skills dir should exist.
	skillsDir := filepath.Join(workspace, "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Errorf("skills dir not created: %v", err)
	}

	// commands dir should exist.
	commandsDir := filepath.Join(workspace, "commands")
	if _, err := os.Stat(commandsDir); err != nil {
		t.Errorf("commands dir not created: %v", err)
	}

	// rules dir should exist.
	rulesDir := filepath.Join(workspace, "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Errorf("rules dir not created: %v", err)
	}
}

// TestMaterializer_Install_RendersSkills verifies that RenderSkill is called for each skill.
func TestMaterializer_Install_RendersSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake package in the cache.
	pkgDir := t.TempDir()
	skillDir := filepath.Join(pkgDir, "skills", "git-commit")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: git:commit\ndescription: D\n---\nB.\n"), 0o644)

	cache := newStubCache()
	pkgHash := "sha256:pkghash"
	cache.entries[pkgHash] = pkgDir

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:manifest",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit"},
				},
			},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(renderer.renderedSkills) != 1 {
		t.Errorf("RenderSkill called %d times, want 1", len(renderer.renderedSkills))
	}
	if !renderer.catalogCalled {
		t.Error("RenderCatalog should have been called")
	}
	if !renderer.finalizeCalled {
		t.Error("Finalize should have been called")
	}
}

// TestMaterializer_Install_UnknownAgent verifies error when no renderer for agent.
func TestMaterializer_Install_UnknownAgent(t *testing.T) {
	cache := newStubCache()
	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:x"}
	err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "unknown-agent"}}, model.InstallConfig{}, t.TempDir())

	if err == nil {
		t.Fatal("expected error for unknown agent but got none")
	}
}

// TestMaterializer_Install_LockAcquired verifies advisory lock lifecycle.
func TestMaterializer_Install_LockAcquired(t *testing.T) {
	tmpDir := t.TempDir()
	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:x"}
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir)

	if !stateMgr.lockAcquired {
		t.Error("AcquireLock should have been called")
	}
	if !stateMgr.lockReleased {
		t.Error("ReleaseLock should have been called (via defer)")
	}
}

// TestMaterializer_Install_CleansPreviousManagedPaths verifies stale paths are removed.
func TestMaterializer_Install_CleansPreviousManagedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "stale" file that was previously managed.
	staleFile := filepath.Join(tmpDir, "old-skill.md")
	_ = os.WriteFile(staleFile, []byte("stale"), 0o644)

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{
		readState: state.State{
			ManagedPaths: []string{staleFile},
		},
	}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:x"}
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir)

	// The stale file should be removed.
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale file %q should have been removed", staleFile)
	}
}

// TestMaterializer_Install_PackageNotInCache verifies error when cached package is missing.
func TestMaterializer_Install_PackageNotInCache(t *testing.T) {
	tmpDir := t.TempDir()
	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache() // empty cache

	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:x",
		Packages: []model.LockedPackage{
			{Hash: "sha256:notincache", Contents: []model.ContentItem{
				{Kind: model.KindSkill, Name: "skill", Path: "skills/skill"},
			}},
		},
	}

	err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, t.TempDir())
	if err == nil {
		t.Fatal("expected error when package not in cache")
	}
}

// TestMaterializer_Install_WritesSchemaVersion verifies state schema version.
func TestMaterializer_Install_WritesSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:abc"}
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir)

	if stateMgr.written.SchemaVersion != "devrune/state/v1" {
		t.Errorf("state.SchemaVersion = %q, want %q", stateMgr.written.SchemaVersion, "devrune/state/v1")
	}
}

// TestMaterializer_Install_CallsRenderSettings verifies that RenderSettings is
// called during Install after RenderCatalog.
func TestMaterializer_Install_CallsRenderSettings(t *testing.T) {
	tmpDir := t.TempDir()

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}

	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:settings"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !renderer.settingsCalled {
		t.Error("RenderSettings should have been called during Install")
	}
}

// TestMaterializer_Install_PassesRulesToRenderCatalog verifies that rules from
// locked packages are collected and forwarded to RenderCatalog.
func TestMaterializer_Install_PassesRulesToRenderCatalog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake package with a rule file in the cache.
	pkgDir := t.TempDir()
	ruleDir := filepath.Join(pkgDir, "rules", "architecture")
	_ = os.MkdirAll(ruleDir, 0o755)
	_ = os.WriteFile(filepath.Join(ruleDir, "clean-architecture-rules.md"), []byte("# Clean Architecture\n"), 0o644)

	cache := newStubCache()
	pkgHash := "sha256:rulepkg"
	cache.entries[pkgHash] = pkgDir

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:manifest-rules",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindRule, Name: "architecture/clean-architecture-rules", Path: "rules/architecture/clean-architecture-rules.md"},
				},
			},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !renderer.catalogCalled {
		t.Fatal("RenderCatalog should have been called")
	}
	if len(renderer.catalogRules) != 1 {
		t.Errorf("RenderCatalog received %d rules, want 1", len(renderer.catalogRules))
	}
	if renderer.catalogRules[0].Name != "architecture/clean-architecture-rules" {
		t.Errorf("catalogRules[0].Name = %q, want %q", renderer.catalogRules[0].Name, "architecture/clean-architecture-rules")
	}
}

// TestMaterializer_Install_SetsInstalledSkillsBeforeWorkflow verifies that the
// materializer calls SetInstalledSkills on the renderer before InstallWorkflow,
// so that workflow post-processing (adviser table injection) can use them.
func TestMaterializer_Install_SetsInstalledSkillsBeforeWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake skill package in the cache.
	skillPkgDir := t.TempDir()
	skillDir := filepath.Join(skillPkgDir, "skills", "architect-adviser")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: architect-adviser\ndescription: Architecture advice\n---\nBody.\n"), 0o644)

	// Create a fake workflow in the cache.
	wfDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(`
schema_version: devrune/workflow/v1
metadata:
  name: sdd
  description: SDD workflow
components:
  skills: []
`), 0o644)

	cache := newStubCache()
	skillHash := "sha256:skillpkg"
	wfHash := "sha256:wfpkg"
	cache.entries[skillHash] = skillPkgDir
	cache.entries[wfHash] = wfDir

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:manifest-wf",
		Packages: []model.LockedPackage{
			{
				Hash: skillHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "architect-adviser", Path: "skills/architect-adviser"},
				},
			},
		},
		Workflows: []model.LockedWorkflow{
			{Name: "sdd", Hash: wfHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// SetInstalledSkills should have been called with the skills before InstallWorkflow.
	if len(renderer.setInstalledSkills) != 1 {
		t.Errorf("SetInstalledSkills received %d skills, want 1", len(renderer.setInstalledSkills))
	}
	if len(renderer.setInstalledSkills) > 0 && renderer.setInstalledSkills[0].Name != "architect-adviser" {
		t.Errorf("setInstalledSkills[0].Name = %q, want %q",
			renderer.setInstalledSkills[0].Name, "architect-adviser")
	}
}

// TestMaterializer_Install_WorkflowManagedPathsPersistedToState verifies that
// the paths returned in WorkflowInstallResult.ManagedPaths are saved into state.
// This is the T017 contract: renderer-reported workflow paths drive cleanup, not
// guessed workspace + skillDir + workflowName paths.
func TestMaterializer_Install_WorkflowManagedPathsPersistedToState(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake workflow cache dir with a minimal workflow.yaml.
	wfDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(`
apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  description: SDD workflow
components:
  skills: []
`), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	cache := newStubCache()
	wfHash := "sha256:wf-managed"
	cache.entries[wfHash] = wfDir

	agentDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "claude", // use claude type so stub is valid — we override InstallWorkflow
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("factory", "claude", agentDef)

	// Simulate the renderer reporting two managed paths from workflow installation.
	managedByWorkflow := []string{
		filepath.Join(tmpDir, ".factory", "skills", "sdd-plan", "SKILL.md"),
		filepath.Join(tmpDir, ".factory", "skills", "sdd-orchestrator", "ORCHESTRATOR.md"),
	}
	renderer.workflowManagedPaths = managedByWorkflow

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"factory": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:manifest-wf-paths",
		Workflows:     []model.LockedWorkflow{{Name: "sdd", Hash: wfHash}},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Both workflow-managed paths must appear in state.ManagedPaths.
	for _, want := range managedByWorkflow {
		found := false
		for _, got := range stateMgr.written.ManagedPaths {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected managed path %q to be persisted to state, got: %v",
				want, stateMgr.written.ManagedPaths)
		}
	}
}

// TestMaterializer_Install_StaleWorkflowPathsRemovedOnReinstall verifies that when a
// reinstall occurs the previous managed paths (from the old state) are removed before
// the new set is written. This proves that renderer-reported managed paths drive
// cleanup, not hardcoded old layout guesses.
func TestMaterializer_Install_StaleWorkflowPathsRemovedOnReinstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Create stale files that simulate the old wrong layout (e.g. .opencode/agents/).
	staleAgentsDir := filepath.Join(tmpDir, ".opencode", "agents", "sdd-plan")
	if err := os.MkdirAll(staleAgentsDir, 0o755); err != nil {
		t.Fatalf("mkdir stale agents dir: %v", err)
	}
	staleFile := filepath.Join(staleAgentsDir, "sdd-plan.md")
	if err := os.WriteFile(staleFile, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	agentDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".opencode"),
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
	}
	renderer := newStubRenderer("opencode", "claude", agentDef)

	// State from a prior install that tracked the stale path.
	stateMgr := &stubStateManager{
		readState: state.State{
			ManagedPaths: []string{staleFile},
		},
	}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"opencode": renderer})

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:reinstall"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The stale .opencode/agents/ file must be removed.
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale file %q should have been removed on reinstall", staleFile)
	}
}

// --- T017: Managed config path alignment and ensureRootMCPJSON format tests ---

// buildMCPFixtureDir creates a minimal MCP cache directory containing a mcp.yaml
// suitable for rendering by any renderer. Returns the directory path.
func buildMCPFixtureDir(t *testing.T, withEnvVar bool) string {
	t.Helper()
	mcpDir := t.TempDir()
	var yaml string
	if withEnvVar {
		yaml = "command: npx\nargs:\n  - -y\n  - exa-mcp-server\nenv:\n  EXA_API_KEY: \"${EXA_API_KEY}\"\n"
	} else {
		yaml = "command: npx\nargs:\n  - -y\n  - simple-mcp-server\n"
	}
	if err := os.WriteFile(filepath.Join(mcpDir, "mcp.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("buildMCPFixtureDir: write mcp.yaml: %v", err)
	}
	return mcpDir
}

// TestMaterializer_ManagedConfigPaths_FactoryAlignedWithRenderMCPs verifies that
// when MCPs are rendered for Factory, the ManagedConfigPaths-derived path (mcp.json)
// is what the materializer tracks in state and what was actually written.
// This ensures config-driven path tracking cannot drift from render output.
func TestMaterializer_ManagedConfigPaths_FactoryAlignedWithRenderMCPs(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".factory")

	mcpDef := &model.MCPConfig{
		FilePath:    "mcp.json",
		RootKey:     "mcpServers",
		EnvKey:      "env",
		EnvVarStyle: "${VAR}",
	}
	factoryDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspace,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP:         mcpDef,
	}

	// Build MCP cache entry.
	mcpDir := buildMCPFixtureDir(t, true)
	cache := newStubCache()
	mcpHash := "sha256:factory-mcp"
	cache.entries[mcpHash] = mcpDir

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{factoryDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, renderers)

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:factory-mcp-test",
		MCPs: []model.LockedMCP{
			{Name: "exa", Hash: mcpHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The config-derived mcp path must appear in state ManagedPaths.
	wantMCPPath := filepath.Join(workspace, "mcp.json")
	found := false
	for _, p := range stateMgr.written.ManagedPaths {
		if p == wantMCPPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("state.ManagedPaths should contain Factory config-derived mcp path %q; got: %v",
			wantMCPPath, stateMgr.written.ManagedPaths)
	}

	// The file must also actually exist at that path.
	if _, err := os.Stat(wantMCPPath); err != nil {
		t.Errorf("Factory mcp.json file must exist at config-derived path %q: %v", wantMCPPath, err)
	}
}

// TestMaterializer_ManagedConfigPaths_OpenCodeAlignedWithRenderMCPs verifies that
// when MCPs are rendered for OpenCode, the ManagedConfigPaths-derived path
// (opencode.json) is tracked in state and matches the actual written file.
func TestMaterializer_ManagedConfigPaths_OpenCodeAlignedWithRenderMCPs(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".opencode")

	mcpDef := &model.MCPConfig{
		FilePath:    "opencode.json",
		RootKey:     "mcp",
		EnvKey:      "environment",
		EnvVarStyle: "{env:VAR}",
	}
	opencodeDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspace,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP:         mcpDef,
	}

	mcpDir := buildMCPFixtureDir(t, false)
	cache := newStubCache()
	mcpHash := "sha256:opencode-mcp"
	cache.entries[mcpHash] = mcpDir

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{opencodeDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, renderers)

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:opencode-mcp-test",
		MCPs: []model.LockedMCP{
			{Name: "simple", Hash: mcpHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The config-derived opencode.json path must appear in state.
	wantMCPPath := filepath.Join(workspace, "opencode.json")
	found := false
	for _, p := range stateMgr.written.ManagedPaths {
		if p == wantMCPPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("state.ManagedPaths should contain OpenCode config-derived opencode.json path %q; got: %v",
			wantMCPPath, stateMgr.written.ManagedPaths)
	}

	// The file must also actually exist.
	if _, err := os.Stat(wantMCPPath); err != nil {
		t.Errorf("OpenCode opencode.json must exist at config-derived path %q: %v", wantMCPPath, err)
	}
}

// TestMaterializer_ManagedConfigPaths_CopilotAlignedWithRenderMCPs verifies that
// when MCPs are rendered for Copilot, the ManagedConfigPaths-derived path
// (.vscode/mcp.json) is tracked in state and matches the actual written file.
func TestMaterializer_ManagedConfigPaths_CopilotAlignedWithRenderMCPs(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".github")

	mcpDef := &model.MCPConfig{
		FilePath:    "../.vscode/mcp.json",
		RootKey:     "servers",
		EnvKey:      "env",
		EnvVarStyle: "${env:VAR}",
	}
	copilotDef := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   workspace,
		SkillDir:    "skills",
		AgentDir:    "agents",
		CatalogFile: "copilot-instructions.md",
		MCP:         mcpDef,
	}

	mcpDir := buildMCPFixtureDir(t, false)
	cache := newStubCache()
	mcpHash := "sha256:copilot-mcp"
	cache.entries[mcpHash] = mcpDir

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{copilotDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, renderers)

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:copilot-mcp-test",
		MCPs: []model.LockedMCP{
			{Name: "simple", Hash: mcpHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "copilot"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Copilot derives its path relative to workspace: workspace + "/../.vscode/mcp.json"
	// which resolves to tmpDir + "/.vscode/mcp.json".
	wantMCPPath := filepath.Join(workspace, "../.vscode/mcp.json")
	found := false
	for _, p := range stateMgr.written.ManagedPaths {
		if p == wantMCPPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("state.ManagedPaths should contain Copilot config-derived .vscode/mcp.json path %q; got: %v",
			wantMCPPath, stateMgr.written.ManagedPaths)
	}

	// The file must also actually exist (materializer creates parent dirs).
	if _, err := os.Stat(wantMCPPath); err != nil {
		t.Errorf("Copilot .vscode/mcp.json must exist at config-derived path %q: %v", wantMCPPath, err)
	}
}

// TestMaterializer_EnsureRootMCPJSON_AlwaysClaudeFormat verifies that when Install
// is called with MCPs for a non-Claude agent (Factory in this case), the materializer
// still creates a project-root .mcp.json using Claude format (mcpServers key).
// This proves ensureRootMCPJSON() is agent-agnostic and always emits Claude format.
func TestMaterializer_EnsureRootMCPJSON_AlwaysClaudeFormat(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".factory")

	mcpDef := &model.MCPConfig{
		FilePath:    "mcp.json",
		RootKey:     "mcpServers",
		EnvKey:      "env",
		EnvVarStyle: "${VAR}",
	}
	factoryDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspace,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP:         mcpDef,
	}

	mcpDir := buildMCPFixtureDir(t, false)
	cache := newStubCache()
	mcpHash := "sha256:root-mcp-format-test"
	cache.entries[mcpHash] = mcpDir

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{factoryDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, renderers)

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:root-mcp-format",
		MCPs: []model.LockedMCP{
			{Name: "simple", Hash: mcpHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The materializer must have created .mcp.json at project root (tmpDir).
	rootMCPPath := filepath.Join(tmpDir, ".mcp.json")
	data, err := os.ReadFile(rootMCPPath)
	if err != nil {
		t.Fatalf("ensureRootMCPJSON should create %q: %v", rootMCPPath, err)
	}

	// Must be Claude format: root key must be "mcpServers" regardless of agent type.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	if _, ok := parsed["mcpServers"]; !ok {
		t.Errorf("ensureRootMCPJSON must use Claude format 'mcpServers' root key; got keys: %v", parsed)
	}
	// Must NOT use non-Claude keys like "servers" (Copilot) or "mcp" (OpenCode).
	if _, ok := parsed["servers"]; ok {
		t.Errorf("ensureRootMCPJSON must not use Copilot 'servers' key; content: %s", data)
	}
	if _, ok := parsed["mcp"]; ok {
		t.Errorf("ensureRootMCPJSON must not use OpenCode 'mcp' key; content: %s", data)
	}
}

// TestMaterializer_EnsureRootMCPJSON_OpenCodeAgentStillClaudeFormat verifies that
// when MCPs are rendered for OpenCode (which uses "mcp" rootKey), the project-root
// .mcp.json still uses "mcpServers" key (Claude format), not OpenCode format.
func TestMaterializer_EnsureRootMCPJSON_OpenCodeAgentStillClaudeFormat(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, ".opencode")

	mcpDef := &model.MCPConfig{
		FilePath:    "opencode.json",
		RootKey:     "mcp", // OpenCode uses "mcp" root key — but root .mcp.json must still use mcpServers
		EnvKey:      "environment",
		EnvVarStyle: "{env:VAR}",
	}
	opencodeDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspace,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP:         mcpDef,
	}

	mcpDir := buildMCPFixtureDir(t, false)
	cache := newStubCache()
	mcpHash := "sha256:root-opencode-mcp"
	cache.entries[mcpHash] = mcpDir

	renderers, err := materialize.NewRendererRegistry([]model.AgentDefinition{opencodeDef})
	if err != nil {
		t.Fatalf("renderer registry: %v", err)
	}

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, renderers)

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:root-opencode-format",
		MCPs: []model.LockedMCP{
			{Name: "simple", Hash: mcpHash},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Root .mcp.json must use Claude format regardless of OpenCode's "mcp" root key.
	rootMCPPath := filepath.Join(tmpDir, ".mcp.json")
	data, err := os.ReadFile(rootMCPPath)
	if err != nil {
		t.Fatalf("ensureRootMCPJSON should create %q even for OpenCode agent: %v", rootMCPPath, err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	if _, ok := parsed["mcpServers"]; !ok {
		t.Errorf("ensureRootMCPJSON must use 'mcpServers' even for OpenCode agent (rootKey=%q); got keys: %v",
			mcpDef.RootKey, parsed)
	}
}

// TestMaterializer_Install_RenderSettingsReceivesSkillsAndWorkflows verifies
// that RenderSettings is called with the collected skills and installed workflows.
func TestMaterializer_Install_RenderSettingsReceivesSkillsAndWorkflows(t *testing.T) {
	tmpDir := t.TempDir()

	// Skill package in cache.
	skillPkgDir := t.TempDir()
	skillDir := filepath.Join(skillPkgDir, "skills", "git-commit")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: git:commit\ndescription: Commit helper\n---\nBody.\n"), 0o644)

	cache := newStubCache()
	pkgHash := "sha256:skillpkg2"
	cache.entries[pkgHash] = skillPkgDir

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	renderer := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:manifest-settings",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit"},
				},
			},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !renderer.settingsCalled {
		t.Fatal("RenderSettings should have been called")
	}
	if len(renderer.settingsSkills) != 1 {
		t.Errorf("RenderSettings received %d skills, want 1", len(renderer.settingsSkills))
	}
	if len(renderer.settingsSkills) > 0 && renderer.settingsSkills[0].Name != "git-commit" {
		t.Errorf("settingsSkills[0].Name = %q, want %q", renderer.settingsSkills[0].Name, "git-commit")
	}
}
