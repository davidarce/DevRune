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

// copilotAgentDef returns a default Copilot agent definition for tests.
// Matches the real agents/copilot.yaml configuration (skillDir: "skills", agentDir: "agents").
func copilotAgentDef() model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   ".github",
		SkillDir:    "skills",
		AgentDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
}

func TestCopilotRenderer_Name(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	if r.Name() != "copilot" {
		t.Errorf("Name() = %q, want %q", r.Name(), "copilot")
	}
}

func TestCopilotRenderer_AgentType(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	if r.AgentType() != "copilot" {
		t.Errorf("AgentType() = %q, want %q", r.AgentType(), "copilot")
	}
}

func TestCopilotRenderer_NeedsCopyMode(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	if !r.NeedsCopyMode() {
		t.Error("NeedsCopyMode() = false, want true")
	}
}

// TestCopilotRenderer_RenderSkill_Full tests rendering a full canonical skill.
// Copilot writes to r.def.Workspace/r.def.SkillDir/{name}.agent.md using the
// workspace path baked into the renderer, so we use a real temp workspace.
func TestCopilotRenderer_RenderSkill_Full(t *testing.T) {
	// Copilot writes to {workspace}/{skillDir}/{name}.agent.md where
	// workspace is the value from agent definition. We need to use
	// the current working directory as the base, so patch with relative paths.
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(tmp, ".github"),
		SkillDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
	r := renderers.NewCopilotRenderer(def)
	inputPath := goldenInputPath(t, "canonical-full.md")

	if err := r.RenderSkill(inputPath, ""); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	// Output is at {workspace}/agents/git-commit.agent.md.
	outputPath := filepath.Join(tmp, ".github", "agents", "git-commit.agent.md")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output %q: %v", outputPath, err)
	}

	// Compare with golden.
	expectedPath := goldenExpectedPath(t, "copilot-full.md")
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}

	if string(content) != string(expected) {
		t.Errorf("output mismatch:\nwant:\n%s\ngot:\n%s", string(expected), string(content))
	}
}

// TestCopilotRenderer_RenderSkill_Minimal tests rendering a minimal skill.
func TestCopilotRenderer_RenderSkill_Minimal(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(tmp, ".github"),
		SkillDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
	r := renderers.NewCopilotRenderer(def)
	inputPath := goldenInputPath(t, "canonical-minimal.md")

	if err := r.RenderSkill(inputPath, ""); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(tmp, ".github", "agents", "my-skill.agent.md")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	expectedPath := goldenExpectedPath(t, "copilot-minimal.md")
	expected, _ := os.ReadFile(expectedPath)

	if string(content) != string(expected) {
		t.Errorf("output mismatch:\nwant:\n%s\ngot:\n%s", string(expected), string(content))
	}
}

// TestCopilotRenderer_OutputIsAgentMdFormat verifies output uses .agent.md extension.
func TestCopilotRenderer_OutputIsAgentMdFormat(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(tmp, ".github"),
		SkillDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
	r := renderers.NewCopilotRenderer(def)

	input := "---\nname: test-agent\ndescription: Test\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)

	if err := r.RenderSkill(srcDir, ""); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(tmp, ".github", "agents", "test-agent.agent.md")
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf(".agent.md file not created at expected path: %v", err)
	}
}

// TestCopilotRenderer_ColonToHyphenInName verifies name colon→hyphen replacement.
func TestCopilotRenderer_ColonToHyphenInName(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "agents",
	}
	r := renderers.NewCopilotRenderer(def)

	input := "---\nname: git:commit\ndescription: Test\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)

	r.RenderSkill(srcDir, "")

	// File should be git-commit.agent.md.
	outputPath := filepath.Join(tmp, ".github", "agents", "git-commit.agent.md")
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("expected git-commit.agent.md: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	fm, _, _ := parse.ParseFrontmatter(data)
	if fm["name"] != "git-commit" {
		t.Errorf("name in frontmatter = %v, want %q", fm["name"], "git-commit")
	}
}

// TestCopilotRenderer_ToolsConversion verifies allowed-tools → Copilot aliases.
func TestCopilotRenderer_ToolsConversion(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "agents",
	}
	r := renderers.NewCopilotRenderer(def)

	input := `---
name: tool-test
description: Test
allowed-tools:
  - Bash(git:*)
  - Read
  - Edit
---
Body.
`
	srcDir := writeSkillFile(t, input)
	r.RenderSkill(srcDir, "")

	data, _ := os.ReadFile(filepath.Join(tmp, ".github", "agents", "tool-test.agent.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	// allowed-tools should be gone, tools should contain aliases.
	if _, ok := fm["allowed-tools"]; ok {
		t.Error("allowed-tools should be removed")
	}

	toolsVal, ok := fm["tools"]
	if !ok {
		t.Fatal("tools should be present after conversion")
	}

	toolsList, ok := toolsVal.([]interface{})
	if !ok {
		t.Fatalf("tools should be []interface{}, got %T", toolsVal)
	}

	toolsSet := make(map[string]bool)
	for _, v := range toolsList {
		if s, ok := v.(string); ok {
			toolsSet[s] = true
		}
	}

	// Bash → execute, Read → read, Edit → edit.
	if !toolsSet["execute"] {
		t.Error("expected 'execute' alias for Bash")
	}
	if !toolsSet["read"] {
		t.Error("expected 'read' alias for Read")
	}
	if !toolsSet["edit"] {
		t.Error("expected 'edit' alias for Edit")
	}
}

// TestCopilotRenderer_DropsNonCopilotFields verifies field dropping.
func TestCopilotRenderer_DropsNonCopilotFields(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "agents",
	}
	r := renderers.NewCopilotRenderer(def)

	input := `---
name: drop-test
description: Test
argument-hint: "[topic]"
disable-model-invocation: false
mode: subagent
reasoning-effort: low
temperature: 0.7
tools-mode: auto
---
Body.
`
	srcDir := writeSkillFile(t, input)
	r.RenderSkill(srcDir, "")

	data, _ := os.ReadFile(filepath.Join(tmp, ".github", "agents", "drop-test.agent.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	dropped := []string{"argument-hint", "disable-model-invocation", "mode", "reasoning-effort", "temperature", "tools-mode"}
	for _, field := range dropped {
		if _, ok := fm[field]; ok {
			t.Errorf("field %q should have been dropped", field)
		}
	}
}

// TestCopilotRenderer_ToolsDeduplicated verifies that duplicate aliases are removed.
func TestCopilotRenderer_ToolsDeduplicated(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "agents",
	}
	r := renderers.NewCopilotRenderer(def)

	// Both Bash and Execute map to "execute" alias.
	input := `---
name: dedup-test
description: Test
allowed-tools:
  - Bash
  - execute
---
Body.
`
	srcDir := writeSkillFile(t, input)
	r.RenderSkill(srcDir, "")

	data, _ := os.ReadFile(filepath.Join(tmp, ".github", "agents", "dedup-test.agent.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	toolsVal, _ := fm["tools"]
	toolsList, _ := toolsVal.([]interface{})

	count := 0
	for _, v := range toolsList {
		if s, _ := v.(string); s == "execute" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("execute alias appears %d times; expected deduplication to 1", count)
	}
}

// TestCopilotRenderer_RenderCatalog verifies the copilot catalog format.
func TestCopilotRenderer_RenderCatalog(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "copilot-instructions.md")

	skills := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git:commit", Path: "skills/git-commit/"},
	}

	if err := r.RenderCatalog(skills, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	content := string(mustReadFile(t, destPath))
	if !strings.Contains(content, "# GitHub Copilot Custom Agent Instructions") {
		t.Error("catalog missing Copilot heading")
	}
	// Copilot applies colonToHyphen in catalog.
	if !strings.Contains(content, "`git-commit`") {
		t.Errorf("catalog should contain git-commit (hyphen); content:\n%s", content)
	}
}

// TestCopilotRenderer_RenderCatalog_Empty verifies empty catalog generation.
func TestCopilotRenderer_RenderCatalog_Empty(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "copilot-instructions.md")

	if err := r.RenderCatalog(nil, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog empty: %v", err)
	}

	content := string(mustReadFile(t, destPath))
	if !strings.Contains(content, "GitHub Copilot") {
		t.Error("empty catalog missing heading")
	}
}

// TestCopilotRenderer_Finalize_NoOp verifies Finalize is a no-op.
func TestCopilotRenderer_Finalize(t *testing.T) {
	r := renderers.NewCopilotRenderer(copilotAgentDef())
	if err := r.Finalize(t.TempDir()); err != nil {
		t.Errorf("Finalize: unexpected error: %v", err)
	}
}

// --- T016: Copilot parity regression tests ---

// copilotParityDef returns an agent definition matching the real .github Copilot layout:
// skills under .github/skills/, agents (native .agent.md) under .github/agents/.
func copilotParityDef(workspaceRoot string) model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		AgentDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
	}
}

// TestCopilotRenderer_InstallWorkflow_SkillsUnderSkillsDir verifies that workflow skills
// land under {workspaceRoot}/skills/{skill-name}/SKILL.md, _shared under skills/, and the
// orchestrator entrypoint is surfaced as {workspaceRoot}/agents/sdd-orchestrator.agent.md.
func TestCopilotRenderer_InstallWorkflow_SkillsUnderSkillsDir(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	// Build a workflow cache dir mimicking a real sdd workflow.
	cachePath := t.TempDir()

	// sdd-plan/SKILL.md
	skillDir := filepath.Join(cachePath, "sdd-plan")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: sdd-plan\ndescription: Plan phase\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}

	// _shared/ directory
	sharedDir := filepath.Join(cachePath, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir _shared: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "common.md"), []byte("# Shared\n"), 0o644); err != nil {
		t.Fatalf("write _shared/common.md: %v", err)
	}

	// ORCHESTRATOR.md (entrypoint)
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# SDD Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	result, err := r.InstallWorkflow(wf, cachePath, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// POSITIVE: skill backing tree
	skillMD := filepath.Join(workspaceRoot, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		t.Errorf("expected %s to exist: %v", skillMD, err)
	}

	// POSITIVE: _shared directory under skills/
	sharedDest := filepath.Join(workspaceRoot, "skills", "_shared")
	if info, err := os.Stat(sharedDest); err != nil || !info.IsDir() {
		t.Errorf("expected %s to be a directory: err=%v", sharedDest, err)
	}

	// POSITIVE: orchestrator surfaced as native .agent.md in agents/
	orchAgent := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
	if _, err := os.Stat(orchAgent); err != nil {
		t.Errorf("expected %s to exist: %v", orchAgent, err)
	}

	// POSITIVE: ManagedPaths is non-empty
	if len(result.ManagedPaths) == 0 {
		t.Error("WorkflowInstallResult.ManagedPaths should be non-empty")
	}

	// NEGATIVE: ordinary skills must NOT be surfaced as agents
	skillAgent := filepath.Join(workspaceRoot, "agents", "sdd-plan.agent.md")
	if _, err := os.Stat(skillAgent); err == nil {
		t.Error("sdd-plan.agent.md should NOT exist in agents/ (ordinary skills are not native agents)")
	}

	// NEGATIVE: registry should not be copied as a loose file
	registryInAgents := filepath.Join(workspaceRoot, "agents", "REGISTRY.md")
	if _, err := os.Stat(registryInAgents); err == nil {
		t.Error("REGISTRY.md should NOT exist in agents/")
	}
	registryInSkills := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryInSkills); err == nil {
		t.Error("REGISTRY.md should NOT exist in skills/")
	}
}

// TestCopilotRenderer_InstallWorkflow_OrchestratorOnlyInAgentsDir verifies that only
// the orchestrator role is surfaced as a native .agent.md, not ordinary workflow skills.
func TestCopilotRenderer_InstallWorkflow_OrchestratorOnlyInAgentsDir(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	skillDir := filepath.Join(cachePath, "sdd-plan")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: sdd-plan\ndescription: Plan\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# SDD Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Orchestrator must be in agents/.
	orchAgent := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
	if _, err := os.Stat(orchAgent); err != nil {
		t.Errorf("sdd-orchestrator.agent.md should exist in agents/: %v", err)
	}

	// Ordinary skill must NOT be in agents/.
	skillAgent := filepath.Join(workspaceRoot, "agents", "sdd-plan.agent.md")
	if _, err := os.Stat(skillAgent); err == nil {
		t.Error("sdd-plan.agent.md should NOT exist in agents/ (only orchestrator is surfaced)")
	}
}

// TestCopilotRenderer_InstallWorkflow_RegistryInjectedIntoCatalog verifies that
// registry content is captured (for potential other use) but NOT injected verbatim
// into the catalog — instead a minimal orchestrator pointer is emitted.
// Also verifies no REGISTRY.md file is written anywhere in the workspace.
func TestCopilotRenderer_InstallWorkflow_RegistryInjectedIntoCatalog(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Minimal skill so the workflow is valid.
	skillDir := filepath.Join(cachePath, "sdd-explore")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-explore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: sdd-explore\ndescription: Explore\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-explore/SKILL.md: %v", err)
	}

	// REGISTRY.md with known content.
	registryContent := "## SDD Skills\n\n- sdd-explore\n"
	if err := os.WriteFile(filepath.Join(cachePath, "REGISTRY.md"), []byte(registryContent), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:   []string{"sdd-explore"},
			Registry: "REGISTRY.md",
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Render catalog to verify registry content is injected.
	catalogPath := filepath.Join(workspaceRoot, "copilot-instructions.md")
	workflows := []model.WorkflowManifest{wf}
	if err := r.RenderCatalog(nil, nil, workflows, catalogPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	catalog := string(catalogData)

	// The workflow name ("sdd") must appear in the catalog as the workflow section heading.
	// Registry text is intentionally NOT injected verbatim — the catalog now emits a
	// minimal section (name + description + orchestrator pointer) instead of the full REGISTRY.md.
	if !strings.Contains(catalog, "sdd") {
		t.Errorf("catalog should contain workflow name 'sdd'; content:\n%s", catalog)
	}

	// No loose REGISTRY.md should exist anywhere in workspace.
	registryInSkills := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryInSkills); err == nil {
		t.Error("REGISTRY.md should NOT exist as a loose file under skills/")
	}
	registryInAgents := filepath.Join(workspaceRoot, "agents", "REGISTRY.md")
	if _, err := os.Stat(registryInAgents); err == nil {
		t.Error("REGISTRY.md should NOT exist as a loose file under agents/")
	}
	registryAtRoot := filepath.Join(workspaceRoot, "REGISTRY.md")
	if _, err := os.Stat(registryAtRoot); err == nil {
		t.Error("REGISTRY.md should NOT exist at workspace root")
	}
}

// TestCopilotRenderer_RenderSkill_ToSkillsDir_UsesSKILLmd verifies that when RenderSkill
// is called with a non-empty destDir (workflow skill mode), the output is SKILL.md, not
// {name}.agent.md — backing skills are stored as SKILL.md, not surfaced as native agents.
func TestCopilotRenderer_RenderSkill_ToSkillsDir_UsesSKILLmd(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "skills",
		AgentDir:  "agents",
	}
	r := renderers.NewCopilotRenderer(def)

	input := "---\nname: sdd-plan\ndescription: Plan phase\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	if err := r.RenderSkill(srcDir, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	// Should be SKILL.md, not sdd-plan.agent.md.
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md when destDir is provided: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "sdd-plan.agent.md")); err == nil {
		t.Error("sdd-plan.agent.md should NOT exist when destDir is provided (should be SKILL.md)")
	}
}

// TestCopilotRenderer_ModelResolution verifies that a short model alias in the skill
// frontmatter (e.g. "sonnet") is resolved to the full Copilot-compatible model ID.
func TestCopilotRenderer_ModelResolution(t *testing.T) {
	tmp := t.TempDir()
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: filepath.Join(tmp, ".github"),
		SkillDir:  "agents", // standalone mode
	}
	r := renderers.NewCopilotRenderer(def)

	input := "---\nname: model-test\ndescription: Test\nmodel: sonnet\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)

	if err := r.RenderSkill(srcDir, ""); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".github", "agents", "model-test.agent.md"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	fm, _, err := parse.ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}

	// "sonnet" should resolve to the full model ID.
	if fm["model"] == "sonnet" {
		t.Errorf("model short alias %q was not resolved; want full model ID", "sonnet")
	}
	if fm["model"] == nil || fm["model"] == "" {
		t.Error("model field should be present and non-empty after resolution")
	}
}

// TestCopilotRenderer_InstallWorkflow_ManagedPathsNonEmpty verifies that InstallWorkflow
// returns a WorkflowInstallResult with at least one managed path.
func TestCopilotRenderer_InstallWorkflow_ManagedPathsNonEmpty(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	skillDir := filepath.Join(cachePath, "sdd-plan")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: sdd-plan\ndescription: Plan\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# SDD Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	result, err := r.InstallWorkflow(wf, cachePath, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if len(result.ManagedPaths) == 0 {
		t.Error("WorkflowInstallResult.ManagedPaths should be non-empty after installing a workflow")
	}
}

// TestCopilotRenderer_RenderMCPs_EnvVarFormat verifies that env var values in the rendered
// .vscode/mcp.json use Copilot format (${env:VAR_NAME}) instead of Claude format (${VAR_NAME}).
func TestCopilotRenderer_RenderMCPs_EnvVarFormat(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceRoot := filepath.Join(projectRoot, ".github")
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: workspaceRoot,
		SkillDir:  "skills",
		AgentDir:  "agents",
		MCP: &model.MCPConfig{
			FilePath:    "../.vscode/mcp.json",
			RootKey:     "servers",
			EnvKey:      "env",
			EnvVarStyle: "${env:VAR}",
		},
	}
	r := renderers.NewCopilotRenderer(def)

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

	// Read the rendered .vscode/mcp.json.
	vscodeMCPPath := filepath.Join(projectRoot, ".vscode", "mcp.json")
	content, err := os.ReadFile(vscodeMCPPath)
	if err != nil {
		t.Fatalf("read .vscode/mcp.json: %v", err)
	}
	mcpContent := string(content)

	// Must use Copilot env var format.
	if !strings.Contains(mcpContent, "${env:EXA_API_KEY}") {
		t.Errorf(".vscode/mcp.json should contain Copilot env format ${env:EXA_API_KEY}; content:\n%s", mcpContent)
	}
	// Must NOT use Claude env var format (raw ${EXA_API_KEY} without "env:").
	// Note: ${env:EXA_API_KEY} contains ${EXA_API_KEY} as a substring, so check for
	// the raw format that does NOT include "env:".
	if strings.Contains(mcpContent, "${EXA_API_KEY}") {
		t.Errorf(".vscode/mcp.json should not contain raw Claude format ${EXA_API_KEY}; content:\n%s", mcpContent)
	}

	// Root key must be "servers" (Copilot/VS Code convention), not "mcpServers" (Claude default).
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse .vscode/mcp.json: %v", err)
	}
	if _, ok := parsed["servers"]; !ok {
		t.Errorf(".vscode/mcp.json should have 'servers' root key; got keys: %v", mapCopilotKeys(parsed))
	}
	if _, ok := parsed["mcpServers"]; ok {
		t.Errorf(".vscode/mcp.json should NOT have 'mcpServers' root key (Claude default); content:\n%s", mcpContent)
	}
}

// TestCopilotRenderer_ManagedConfigPaths verifies that ManagedConfigPaths returns the
// config-driven .vscode/mcp.json path relative to workspaceRoot.
func TestCopilotRenderer_ManagedConfigPaths(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceRoot := filepath.Join(projectRoot, ".github")
	def := model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: workspaceRoot,
		SkillDir:  "skills",
		AgentDir:  "agents",
		MCP: &model.MCPConfig{
			FilePath:    "../.vscode/mcp.json",
			RootKey:     "servers",
			EnvKey:      "env",
			EnvVarStyle: "${env:VAR}",
		},
	}
	r := renderers.NewCopilotRenderer(def)

	paths := r.ManagedConfigPaths(workspaceRoot)

	if len(paths) != 1 {
		t.Fatalf("ManagedConfigPaths() returned %d paths, want 1", len(paths))
	}

	wantPath := filepath.Join(projectRoot, ".vscode", "mcp.json")
	if paths[0] != wantPath {
		t.Errorf("ManagedConfigPaths()[0] = %q, want %q", paths[0], wantPath)
	}
}

// mapCopilotKeys returns the keys of a map[string]interface{} as a slice, for diagnostic messages.
func mapCopilotKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
