// SPDX-License-Identifier: MIT

package renderers_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// factoryAgentDef returns a default Factory agent definition for tests.
// Includes the MCP config matching agents/factory.yaml so tests reflect
// the config-driven renderer behavior (mcp.json at workspaceRoot, mcpServers root key).
func factoryAgentDef() model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   ".factory",
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
}

func TestFactoryRenderer_Name(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	if r.Name() != "factory" {
		t.Errorf("Name() = %q, want %q", r.Name(), "factory")
	}
}

func TestFactoryRenderer_AgentType(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	if r.AgentType() != "factory" {
		t.Errorf("AgentType() = %q, want %q", r.AgentType(), "factory")
	}
}

func TestFactoryRenderer_NeedsCopyMode(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	if !r.NeedsCopyMode() {
		t.Error("NeedsCopyMode() = false, want true")
	}
}

// TestFactoryRenderer_RenderSkill_Full tests rendering a full canonical skill.
func TestFactoryRenderer_RenderSkill_Full(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	inputPath := goldenInputPath(t, "canonical-full.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "factory-full.md")
}

// TestFactoryRenderer_RenderSkill_Minimal tests rendering a minimal skill.
func TestFactoryRenderer_RenderSkill_Minimal(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	inputPath := goldenInputPath(t, "canonical-minimal.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "factory-minimal.md")
}

// TestFactoryRenderer_ModelResolution verifies short model name → full ID.
func TestFactoryRenderer_ModelResolution(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	tests := []struct {
		modelIn  string
		modelOut string
	}{
		{"sonnet", "anthropic/claude-sonnet-4-20250514"},
		{"opus", "anthropic/claude-opus-4-20250514"},
		{"haiku", "anthropic/claude-haiku-4-5-20250929"},
		{"gpt-4o", "gpt-4o"}, // unknown passes through
	}

	for _, tt := range tests {
		t.Run("model="+tt.modelIn, func(t *testing.T) {
			input := "---\nname: m\ndescription: T\nmodel: " + tt.modelIn + "\n---\nB.\n"
			srcDir := writeSkillFile(t, input)
			destDir := t.TempDir()

			_ = r.RenderSkill(srcDir, destDir)
			data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
			fm, _, _ := parse.ParseFrontmatter(data)

			if fm["model"] != tt.modelOut {
				t.Errorf("model = %v, want %q", fm["model"], tt.modelOut)
			}
		})
	}
}

// TestFactoryRenderer_ReasoningEffortCamelCase verifies reasoning-effort → reasoningEffort.
func TestFactoryRenderer_ReasoningEffortCamelCase(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	input := "---\nname: reasoning-skill\ndescription: Test\nreasoning-effort: low\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	// reasoning-effort should be dropped (it's in factoryDropFields).
	if _, ok := fm["reasoning-effort"]; ok {
		t.Error("reasoning-effort should be dropped for Factory")
	}

	// reasoningEffort (camelCase) should be present.
	if _, ok := fm["reasoningEffort"]; !ok {
		t.Error("reasoningEffort (camelCase) should be present after rename")
	}
}

// TestFactoryRenderer_UserInvocableNotInjectedForRegularSkills verifies that
// user-invocable is NOT injected for regular skills (only for workflow skills).
func TestFactoryRenderer_UserInvocableNotInjectedForRegularSkills(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	input := "---\nname: regular-skill\ndescription: Test\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if _, ok := fm["user-invocable"]; ok {
		t.Error("user-invocable should NOT be injected for regular (non-workflow) skills")
	}
}

// TestFactoryRenderer_DropsFields verifies non-factory fields are dropped.
func TestFactoryRenderer_DropsFields(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	input := `---
name: drop-test
description: Test
argument-hint: "[topic]"
disable-model-invocation: false
reasoning-effort: low
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	dropped := []string{"argument-hint", "disable-model-invocation", "reasoning-effort"}
	for _, field := range dropped {
		if _, ok := fm[field]; ok {
			t.Errorf("field %q should have been dropped", field)
		}
	}
}

// TestFactoryRenderer_RenderCommand verifies command rendering.
func TestFactoryRenderer_RenderCommand(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	destDir := t.TempDir()

	cmd := model.WorkflowCommand{
		Name:   "sdd-review",
		Action: "Review implementation",
	}

	if err := r.RenderCommand(cmd, destDir); err != nil {
		t.Fatalf("RenderCommand: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read command output: %v", err)
	}

	fm, _, _ := parse.ParseFrontmatter(data)
	if fm["name"] != "sdd-review" {
		t.Errorf("name = %v, want %q", fm["name"], "sdd-review")
	}
}

// TestFactoryRenderer_Finalize_NoFile verifies Finalize is a no-op when mcp.json absent.
func TestFactoryRenderer_Finalize_NoFile(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())
	if err := r.Finalize(t.TempDir()); err != nil {
		t.Errorf("Finalize on empty dir: unexpected error: %v", err)
	}
}

// TestFactoryRenderer_Finalize_ResolvesEnvPlaceholders verifies ${VAR} substitution.
// mcp.json lives at the workspace root (.factory/mcp.json), not in a config/ subdir.
func TestFactoryRenderer_Finalize_ResolvesEnvPlaceholders(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	workspaceRoot := t.TempDir()

	// Write mcp.json at the workspace root (the correct .factory/mcp.json location).
	mcpContent := `{"mcpServers":{"test":{"command":"${TEST_CMD_VAR}"}}}`
	_ = os.WriteFile(filepath.Join(workspaceRoot, "mcp.json"), []byte(mcpContent), 0o644)

	// Set the env var.
	t.Setenv("TEST_CMD_VAR", "npx-resolved")

	if err := r.Finalize(workspaceRoot); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(workspaceRoot, "mcp.json"))
	if !strings.Contains(string(result), "npx-resolved") {
		t.Errorf("env var not resolved; content: %s", string(result))
	}
	if strings.Contains(string(result), "${TEST_CMD_VAR}") {
		t.Error("placeholder still present after resolution")
	}
}

// TestFactoryRenderer_Finalize_KeepsUnresolvedPlaceholders verifies that unset env vars
// leave the placeholder unchanged.
// mcp.json lives at the workspace root (.factory/mcp.json), not in a config/ subdir.
func TestFactoryRenderer_Finalize_KeepsUnresolvedPlaceholders(t *testing.T) {
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	workspaceRoot := t.TempDir()

	mcpContent := `{"mcpServers":{"test":{"command":"${UNSET_VAR_12345}"}}}`
	_ = os.WriteFile(filepath.Join(workspaceRoot, "mcp.json"), []byte(mcpContent), 0o644)

	// Ensure the var is unset.
	_ = os.Unsetenv("UNSET_VAR_12345")

	if err := r.Finalize(workspaceRoot); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(workspaceRoot, "mcp.json"))
	if !strings.Contains(string(result), "${UNSET_VAR_12345}") {
		t.Error("placeholder should be kept when env var is not set")
	}
}

// --- T014: Factory parity regression tests ---

// setupFactoryWorkflowFixture creates a workflow cache directory with the standard
// sdd fixture: sdd-plan/SKILL.md, ORCHESTRATOR.md, _shared/shared.md, REGISTRY.md.
// It returns the cache dir path.
func setupFactoryWorkflowFixture(t *testing.T) string {
	t.Helper()
	wfCacheDir := t.TempDir()

	// sdd-plan skill directory.
	sddPlanDir := filepath.Join(wfCacheDir, "sdd-plan")
	if err := os.MkdirAll(sddPlanDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	skillContent := "---\nname: sdd-plan\ndescription: Create implementation plan\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(sddPlanDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}

	// Orchestrator entrypoint.
	orchContent := "# SDD Orchestrator\n\nOrchestrate the SDD workflow.\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "ORCHESTRATOR.md"), []byte(orchContent), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	// _shared directory with a file.
	sharedDir := filepath.Join(wfCacheDir, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir _shared: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "shared.md"), []byte("# Shared\n"), 0o644); err != nil {
		t.Fatalf("write _shared/shared.md: %v", err)
	}

	// REGISTRY.md (should be captured, not copied loose).
	registryContent := "# SDD Skills Registry\n\n| sdd-explore |\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(registryContent), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}

	return wfCacheDir
}

// sddWorkflowManifest returns the canonical sdd WorkflowManifest used in T014 tests.
func sddWorkflowManifest() model.WorkflowManifest {
	return model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Registry:   "REGISTRY.md",
		},
	}
}

// TestFactoryRenderer_InstallWorkflow_SkillsInCorrectLocation verifies that workflow
// skills are placed at .factory/skills/<skill-name>/SKILL.md (flat, not nested under
// the workflow name), and the orchestrator is at .factory/skills/sdd-orchestrator/ORCHESTRATOR.md.
func TestFactoryRenderer_InstallWorkflow_SkillsInCorrectLocation(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspaceRoot,
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
	r := renderers.NewFactoryRenderer(def)

	wfCacheDir := setupFactoryWorkflowFixture(t)
	wf := sddWorkflowManifest()

	result, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Positive: skill installed at flat location (not under workflow name dir).
	sddPlanSkill := filepath.Join(workspaceRoot, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(sddPlanSkill); err != nil {
		t.Errorf("expected sdd-plan/SKILL.md at flat location but got: %v", err)
	}

	// Positive: orchestrator installed with <wf-name>-orchestrator naming convention.
	orchestratorFile := filepath.Join(workspaceRoot, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
	if _, err := os.Stat(orchestratorFile); err != nil {
		t.Errorf("expected sdd-orchestrator/ORCHESTRATOR.md but got: %v", err)
	}

	// Positive: _shared directory copied under skillsBase.
	sharedDir := filepath.Join(workspaceRoot, "skills", "_shared")
	if _, err := os.Stat(sharedDir); err != nil {
		t.Errorf("expected _shared/ directory under skills but got: %v", err)
	}

	// Positive: ManagedPaths is non-empty.
	if len(result.ManagedPaths) == 0 {
		t.Error("ManagedPaths should be non-empty after installing a workflow")
	}

	// Negative: old buggy path — droids directory must NOT exist.
	droidsOrchestratorFile := filepath.Join(workspaceRoot, "droids", "sdd-orchestrator.md")
	if _, err := os.Stat(droidsOrchestratorFile); !os.IsNotExist(err) {
		t.Errorf("droids/sdd-orchestrator.md must NOT exist (old buggy path), but found it")
	}

	// Negative: REGISTRY.md must NOT be copied as a loose file.
	registryLoose := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryLoose); !os.IsNotExist(err) {
		t.Errorf("skills/REGISTRY.md must NOT exist as a loose file, but found it")
	}
}

// TestFactoryRenderer_InstallWorkflow_ManagedPathsNonEmpty verifies that
// InstallWorkflow returns a non-empty ManagedPaths slice.
func TestFactoryRenderer_InstallWorkflow_ManagedPathsNonEmpty(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspaceRoot,
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
	r := renderers.NewFactoryRenderer(def)

	wfCacheDir := setupFactoryWorkflowFixture(t)
	wf := sddWorkflowManifest()

	result, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if len(result.ManagedPaths) == 0 {
		t.Error("ManagedPaths should be non-empty after installing a workflow with skills")
	}
}

// TestFactoryRenderer_InstallWorkflow_NoDroidsDir verifies that InstallWorkflow does
// NOT create a droids/ directory (old layout that must not exist).
func TestFactoryRenderer_InstallWorkflow_NoDroidsDir(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspaceRoot,
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
	r := renderers.NewFactoryRenderer(def)

	wfCacheDir := setupFactoryWorkflowFixture(t)
	wf := sddWorkflowManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	droidsDir := filepath.Join(workspaceRoot, "droids")
	if _, err := os.Stat(droidsDir); !os.IsNotExist(err) {
		t.Errorf("droids/ directory must NOT exist after InstallWorkflow, but found it")
	}
}

// TestFactoryRenderer_InstallWorkflow_RegistryInjectedIntoCatalog verifies that
// REGISTRY.md content is captured and injected into the catalog by RenderCatalog,
// and that no loose REGISTRY.md file exists in the workspace.
func TestFactoryRenderer_InstallWorkflow_RegistryInjectedIntoCatalog(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspaceRoot,
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
	r := renderers.NewFactoryRenderer(def)

	wfCacheDir := setupFactoryWorkflowFixture(t)
	wf := sddWorkflowManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Registry content is captured in the renderer for later use by RenderRootCatalog.
	contents := r.RegistryContents()
	registryContent, ok := contents[wf.Metadata.Name]
	if !ok {
		t.Fatalf("RegistryContents should contain captured content for workflow 'sdd'; got keys: %v", func() []string {
			var keys []string
			for k := range contents {
				keys = append(keys, k)
			}
			return keys
		}())
	}

	// Registry text should be in the captured content.
	if !strings.Contains(registryContent, "# SDD Skills Registry") {
		t.Errorf("registry content missing heading; content:\n%s", registryContent)
	}
	if !strings.Contains(registryContent, "| sdd-explore |") {
		t.Errorf("registry content missing table row; content:\n%s", registryContent)
	}

	// REGISTRY.md must NOT exist as a loose file anywhere under skills/.
	registryLoose := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryLoose); !os.IsNotExist(err) {
		t.Errorf("REGISTRY.md must NOT exist as a loose file under skills/, but found it")
	}
}

// TestFactoryRenderer_InstallWorkflow_SkillsAreFlat verifies that workflow skills
// are installed under skills/<skillname>/SKILL.md (flat layout) and NOT under
// skills/sdd/<skillname>/SKILL.md (that would be the Claude nested layout).
func TestFactoryRenderer_InstallWorkflow_SkillsAreFlat(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "factory",
		Type:        "factory",
		Workspace:   workspaceRoot,
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
	r := renderers.NewFactoryRenderer(def)

	wfCacheDir := setupFactoryWorkflowFixture(t)
	wf := sddWorkflowManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Flat location must exist.
	flatSkillPath := filepath.Join(workspaceRoot, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(flatSkillPath); err != nil {
		t.Errorf("skills/sdd-plan/SKILL.md should exist (flat layout) but got: %v", err)
	}

	// Nested location (Claude-style) must NOT exist.
	nestedSkillDir := filepath.Join(workspaceRoot, "skills", "sdd")
	if _, err := os.Stat(nestedSkillDir); !os.IsNotExist(err) {
		t.Errorf("skills/sdd/ directory must NOT exist (that would be the Claude nested layout), but found it")
	}
}

// TestFactoryRenderer_RenderMCPs_RootKeyAndEnvVarFormat verifies that RenderMCPs writes
// mcp.json with the "mcpServers" root key and ${VAR} env var format (Factory convention).
func TestFactoryRenderer_RenderMCPs_RootKeyAndEnvVarFormat(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	// Create a fake MCP cache entry with an env var in Claude format.
	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "abc123")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: exa
command: npx
args:
  - "-y"
  - "@modelcontextprotocol/server-exa"
env:
  EXA_API_KEY: "${EXA_API_KEY}"
`
	if err := os.WriteFile(filepath.Join(mcpDir, "exa.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}

	cache := &fakeCacheStore{dirs: map[string]string{"abc123": mcpDir}}
	mcps := []model.LockedMCP{{Name: "exa", Hash: "abc123"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	// mcp.json lives at workspaceRoot/mcp.json (config-driven, not in a subdir).
	content, err := os.ReadFile(filepath.Join(workspaceRoot, "mcp.json"))
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	mcpContent := string(content)

	// Root key must be "mcpServers" (Factory/Claude convention).
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse mcp.json: %v", err)
	}
	if _, ok := parsed["mcpServers"]; !ok {
		t.Errorf("mcp.json should have 'mcpServers' root key; content:\n%s", mcpContent)
	}

	// Env var placeholder must use Factory format (${VAR_NAME}).
	if !strings.Contains(mcpContent, "${EXA_API_KEY}") {
		t.Errorf("mcp.json should contain Factory env format ${EXA_API_KEY}; content:\n%s", mcpContent)
	}
}

// TestFactoryRenderer_ManagedConfigPaths verifies that ManagedConfigPaths returns the
// config-driven mcp.json path at workspaceRoot.
func TestFactoryRenderer_ManagedConfigPaths(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewFactoryRenderer(factoryAgentDef())

	paths := r.ManagedConfigPaths(workspaceRoot)

	if len(paths) != 1 {
		t.Fatalf("ManagedConfigPaths() returned %d paths, want 1", len(paths))
	}

	wantPath := filepath.Join(workspaceRoot, "mcp.json")
	if paths[0] != wantPath {
		t.Errorf("ManagedConfigPaths()[0] = %q, want %q", paths[0], wantPath)
	}
}
