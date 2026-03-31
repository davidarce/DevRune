// SPDX-License-Identifier: MIT

package materialize_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	settingsErr        error
	finalizeErr        error
	renderedSkills     []string
	settingsCalled     bool
	finalizeCalled     bool
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

	if err := m.Install(context.Background(), lock, agents, cfg, tmpDir, nil); err != nil {
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
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil)

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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(renderer.renderedSkills) != 1 {
		t.Errorf("RenderSkill called %d times, want 1", len(renderer.renderedSkills))
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
	err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "unknown-agent"}}, model.InstallConfig{}, t.TempDir(), nil)

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
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil)

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
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil)

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

	err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, t.TempDir(), nil)
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
	_ = m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil)

	if stateMgr.written.SchemaVersion != "devrune/state/v1" {
		t.Errorf("state.SchemaVersion = %q, want %q", stateMgr.written.SchemaVersion, "devrune/state/v1")
	}
}

// TestMaterializer_Install_CallsRenderSettings verifies that RenderSettings is
// called during Install.
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
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !renderer.settingsCalled {
		t.Error("RenderSettings should have been called during Install")
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "copilot"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "factory"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "opencode"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
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

// --- T023: Root catalog generation tests ---

// TestMaterializer_Install_WritesRootCatalog verifies that Install creates AGENTS.md
// at the project root containing a managed block with catalog content.
func TestMaterializer_Install_WritesRootCatalog(t *testing.T) {
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

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:root-catalog"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// AGENTS.md must exist at project root.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md should exist at project root: %v", err)
	}
	content := string(data)

	// Must contain managed block markers.
	if !strings.Contains(content, "# >>> devrune managed") {
		t.Errorf("AGENTS.md should contain begin marker; got:\n%s", content)
	}
	if !strings.Contains(content, "# <<< devrune managed") {
		t.Errorf("AGENTS.md should contain end marker; got:\n%s", content)
	}
	// Must contain catalog header.
	if !strings.Contains(content, "# Agent Catalog") {
		t.Errorf("AGENTS.md should contain '# Agent Catalog'; got:\n%s", content)
	}
}

// TestMaterializer_Install_CreatesCLAUDEmdSymlink verifies that CLAUDE.md is created
// at project root (as symlink or copy pointing to AGENTS.md).
func TestMaterializer_Install_CreatesCLAUDEmdSymlink(t *testing.T) {
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

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:symlink-test"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// CLAUDE.md must exist at project root.
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if _, err := os.Stat(claudePath); err != nil {
		t.Fatalf("CLAUDE.md should exist at project root: %v", err)
	}

	// CLAUDE.md content must match AGENTS.md content (it is a symlink or copy).
	agentsData, _ := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	claudeData, _ := os.ReadFile(claudePath)
	if string(agentsData) != string(claudeData) {
		t.Errorf("CLAUDE.md content should match AGENTS.md; AGENTS.md=%q CLAUDE.md=%q",
			string(agentsData), string(claudeData))
	}
}

// TestMaterializer_Install_RemovesOldWorkspaceCatalogs verifies that old workspace
// catalog files (.claude/CLAUDE.md, .opencode/AGENTS.md, etc.) are removed during install.
func TestMaterializer_Install_RemovesOldWorkspaceCatalogs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create old workspace catalog files to simulate a pre-migration install.
	oldPaths := []string{
		filepath.Join(tmpDir, ".claude", "CLAUDE.md"),
		filepath.Join(tmpDir, ".opencode", "AGENTS.md"),
		filepath.Join(tmpDir, ".factory", "AGENTS.md"),
		filepath.Join(tmpDir, ".github", "copilot-instructions.md"),
	}
	for _, p := range oldPaths {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("old catalog content"), 0o644)
	}

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

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:old-catalogs"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// All old workspace catalog files must be removed.
	for _, p := range oldPaths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("old workspace catalog %q should have been removed", p)
		}
	}
}

// TestMaterializer_Install_RootCatalogContainsSkillNames verifies that the root
// AGENTS.md includes skill names when skills are installed.
func TestMaterializer_Install_RootCatalogContainsSkillNames(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake skill package in the cache.
	pkgDir := t.TempDir()
	skillDir := filepath.Join(pkgDir, "skills", "unit-test-adviser")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: unit-test-adviser\ndescription: Domain unit test patterns\n---\nBody.\n"), 0o644)

	cache := newStubCache()
	pkgHash := "sha256:skill-catalog-test"
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
		ManifestHash:  "sha256:skill-catalog",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "unit-test-adviser", Path: "skills/unit-test-adviser", Description: "Domain unit test patterns"},
				},
			},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md should exist: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "unit-test-adviser") {
		t.Errorf("AGENTS.md should contain skill name 'unit-test-adviser'; got:\n%s", content)
	}
}

// --- T025: Uninstall / managed block removal tests ---

// TestMaterializer_Reinstall_PreservesUserContentInRootCatalog verifies that
// user content outside the managed block is preserved across reinstalls.
func TestMaterializer_Reinstall_PreservesUserContentInRootCatalog(t *testing.T) {
	tmpDir := t.TempDir()

	agentDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   filepath.Join(tmpDir, ".claude"),
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}
	cache := newStubCache()
	linker, _ := materialize.NewLinker("copy")

	// First install.
	renderer1 := newStubRenderer("claude", "claude", agentDef)
	stateMgr := &stubStateManager{}
	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{"claude": renderer1})
	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:reinstall-1"}
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Simulate user adding content before the managed block.
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	existing, _ := os.ReadFile(agentsPath)
	userContent := "# My Custom Notes\n\nKeep this content.\n\n"
	_ = os.WriteFile(agentsPath, append([]byte(userContent), existing...), 0o644)

	// Second install — same state manager (prev state from first install).
	renderer2 := newStubRenderer("claude", "claude", agentDef)
	stateMgr2 := &stubStateManager{readState: stateMgr.written}
	m2 := materialize.NewMaterializer(cache, linker, stateMgr2, map[string]materialize.AgentRenderer{"claude": renderer2})
	lock2 := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:reinstall-2"}
	if err := m2.Install(context.Background(), lock2, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	// User content before the managed block must still be present.
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md should still exist after reinstall: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# My Custom Notes") {
		t.Errorf("user content should be preserved after reinstall; got:\n%s", content)
	}
	if !strings.Contains(content, "# Agent Catalog") {
		t.Errorf("catalog content should be present after reinstall; got:\n%s", content)
	}
}

// TestMaterializer_Install_CLAUDEmd_NotRemovedIfUserOwned verifies that if CLAUDE.md
// already exists as a user-owned file (not a symlink to AGENTS.md), the install
// completes successfully (with a warning) and CLAUDE.md is not clobbered.
func TestMaterializer_Install_CLAUDEmd_NotRemovedIfUserOwned(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a user-owned CLAUDE.md that should NOT be overwritten.
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	userContent := "# My custom instructions\n\nDo not overwrite me.\n"
	_ = os.WriteFile(claudePath, []byte(userContent), 0o644)

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

	lock := model.Lockfile{SchemaVersion: "v1", ManifestHash: "sha256:user-owned-claude"}
	// Install should succeed even though CLAUDE.md is user-owned (warn only, non-fatal).
	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "claude"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install should succeed even with user-owned CLAUDE.md: %v", err)
	}

	// User-owned CLAUDE.md must NOT be overwritten.
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("CLAUDE.md should still exist: %v", err)
	}
	if string(data) != userContent {
		t.Errorf("user-owned CLAUDE.md should not be overwritten; got:\n%s", string(data))
	}

	// AGENTS.md must still exist at project root.
	if _, err := os.Stat(filepath.Join(tmpDir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md should still be created at project root: %v", err)
	}
}

// --- T023: Shared-directory skill deduplication integration tests ---

// TestMaterializer_Install_SkillDeduplication_SharedDirRenderedOnce verifies that
// when multiple agents share the same resolved skill directory (e.g. .agents/skills/),
// RenderSkill is called exactly ONCE per skill — not once per agent.
//
// Scenario: 3 agents (codex, factory, opencode) share .agents/skills/. 1 agent (claude)
// uses .claude/skills/ exclusively. A lockfile contains 1 skill package.
//
// Expected: RenderSkill invoked 1 time total for the .agents/skills/ group,
// and 1 time for .claude/skills/ — so 2 total invocations, not 4.
func TestMaterializer_Install_SkillDeduplication_SharedDirRenderedOnce(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the shared .agents/skills workspace root so path resolution works.
	agentsSkillsDir := filepath.Join(tmpDir, ".agents", "skills")
	if err := os.MkdirAll(agentsSkillsDir, 0o755); err != nil {
		t.Fatalf("mkdir .agents/skills: %v", err)
	}

	// Build agent definitions — all three share .agents/skills via the "../.agents/skills"
	// SkillDir pattern that resolves through filepath.Join(workspace, skillDir).
	codexWorkspace := filepath.Join(tmpDir, ".codex")
	factoryWorkspace := filepath.Join(tmpDir, ".factory")
	opencodeWorkspace := filepath.Join(tmpDir, ".opencode")
	claudeWorkspace := filepath.Join(tmpDir, ".claude")

	codexDef := model.AgentDefinition{
		Name:        "codex",
		Type:        "codex",
		Workspace:   codexWorkspace,
		SkillDir:    "../.agents/skills",
		CatalogFile: "AGENTS.md",
	}
	factoryDef := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   factoryWorkspace,
		SkillDir:    "../.agents/skills",
		CatalogFile: "AGENTS.md",
	}
	opencodeDef := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   opencodeWorkspace,
		SkillDir:    "../.agents/skills",
		CatalogFile: "AGENTS.md",
	}
	claudeDef := model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   claudeWorkspace,
		SkillDir:    "skills",
		CatalogFile: "CLAUDE.md",
	}

	codexRenderer := newStubRenderer("codex", "codex", codexDef)
	factoryRenderer := newStubRenderer("factory", "factory", factoryDef)
	opencodeRenderer := newStubRenderer("opencode", "opencode", opencodeDef)
	claudeRenderer := newStubRenderer("claude", "claude", claudeDef)

	// Create a fake skill package in the cache.
	pkgDir := t.TempDir()
	skillSubDir := filepath.Join(pkgDir, "skills", "unit-test-adviser")
	if err := os.MkdirAll(skillSubDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillSubDir, "SKILL.md"),
		[]byte("---\nname: unit-test-adviser\ndescription: Domain unit tests\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	cache := newStubCache()
	pkgHash := "sha256:dedup-test"
	cache.entries[pkgHash] = pkgDir

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")

	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{
		"codex":    codexRenderer,
		"factory":  factoryRenderer,
		"opencode": opencodeRenderer,
		"claude":   claudeRenderer,
	})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:dedup-manifest",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "unit-test-adviser", Path: "skills/unit-test-adviser"},
				},
			},
		},
	}

	agents := []model.AgentRef{
		{Name: "codex"},
		{Name: "factory"},
		{Name: "opencode"},
		{Name: "claude"},
	}

	if err := m.Install(context.Background(), lock, agents, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The .agents/skills group (codex, factory, opencode) must collectively render
	// the skill exactly ONCE. Count invocations across all three renderers.
	sharedRenderCount := len(codexRenderer.renderedSkills) +
		len(factoryRenderer.renderedSkills) +
		len(opencodeRenderer.renderedSkills)

	if sharedRenderCount != 1 {
		t.Errorf("skills in .agents/skills/ should be rendered exactly once across shared agents, got %d invocations (codex=%d, factory=%d, opencode=%d)",
			sharedRenderCount,
			len(codexRenderer.renderedSkills),
			len(factoryRenderer.renderedSkills),
			len(opencodeRenderer.renderedSkills))
	}

	// Claude has a unique skill directory — must render the skill independently.
	if len(claudeRenderer.renderedSkills) != 1 {
		t.Errorf("claude should render skills independently (unique dir .claude/skills/), got %d invocations",
			len(claudeRenderer.renderedSkills))
	}

	// The skill file must exist in .agents/skills/ (written by whichever renderer won).
	skillPath := filepath.Join(agentsSkillsDir, "unit-test-adviser", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("skill file should exist in .agents/skills/unit-test-adviser/SKILL.md: %v", err)
	}

	// The skill file must also exist in .claude/skills/.
	claudeSkillPath := filepath.Join(claudeWorkspace, "skills", "unit-test-adviser", "SKILL.md")
	if _, err := os.Stat(claudeSkillPath); err != nil {
		t.Errorf("skill file should exist in .claude/skills/unit-test-adviser/SKILL.md: %v", err)
	}
}

// TestMaterializer_Install_SkillDeduplication_TwoAgentsSameDir verifies the minimal
// case: two agents sharing the same skill directory render each skill only once.
func TestMaterializer_Install_SkillDeduplication_TwoAgentsSameDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Two agents with the same resolved skill dir: .agents/skills/.
	agent1Workspace := filepath.Join(tmpDir, ".agent1")
	agent2Workspace := filepath.Join(tmpDir, ".agent2")

	agent1Def := model.AgentDefinition{
		Name:        "agent1",
		Type:        "factory",
		Workspace:   agent1Workspace,
		SkillDir:    "../.agents/skills",
		CatalogFile: "AGENTS.md",
	}
	agent2Def := model.AgentDefinition{
		Name:        "agent2",
		Type:        "opencode",
		Workspace:   agent2Workspace,
		SkillDir:    "../.agents/skills",
		CatalogFile: "AGENTS.md",
	}

	renderer1 := newStubRenderer("agent1", "factory", agent1Def)
	renderer2 := newStubRenderer("agent2", "opencode", agent2Def)

	// Skill package with two skills.
	pkgDir := t.TempDir()
	for _, name := range []string{"skill-a", "skill-b"} {
		skillSubDir := filepath.Join(pkgDir, "skills", name)
		if err := os.MkdirAll(skillSubDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(skillSubDir, "SKILL.md"),
			[]byte("---\nname: "+name+"\ndescription: "+name+"\n---\nBody.\n"), 0o644); err != nil {
			t.Fatalf("write %s/SKILL.md: %v", name, err)
		}
	}

	cache := newStubCache()
	pkgHash := "sha256:two-agent-dedup"
	cache.entries[pkgHash] = pkgDir

	stateMgr := &stubStateManager{}
	linker, _ := materialize.NewLinker("copy")
	m := materialize.NewMaterializer(cache, linker, stateMgr, map[string]materialize.AgentRenderer{
		"agent1": renderer1,
		"agent2": renderer2,
	})

	lock := model.Lockfile{
		SchemaVersion: "v1",
		ManifestHash:  "sha256:two-agent-dedup-manifest",
		Packages: []model.LockedPackage{
			{
				Hash: pkgHash,
				Contents: []model.ContentItem{
					{Kind: model.KindSkill, Name: "skill-a", Path: "skills/skill-a"},
					{Kind: model.KindSkill, Name: "skill-b", Path: "skills/skill-b"},
				},
			},
		},
	}

	if err := m.Install(context.Background(), lock, []model.AgentRef{{Name: "agent1"}, {Name: "agent2"}}, model.InstallConfig{}, tmpDir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Total renders across both agents must be 2 (one per skill), not 4.
	totalRenders := len(renderer1.renderedSkills) + len(renderer2.renderedSkills)
	if totalRenders != 2 {
		t.Errorf("two skills in shared dir should be rendered 2 times total, got %d (agent1=%d, agent2=%d)",
			totalRenders, len(renderer1.renderedSkills), len(renderer2.renderedSkills))
	}
}
