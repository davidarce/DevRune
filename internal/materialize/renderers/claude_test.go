// SPDX-License-Identifier: MIT

package renderers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// claudeAgentDef returns a default Claude agent definition for tests.
func claudeAgentDef() model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "claude",
		Type:        "claude",
		Workspace:   ".claude",
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "CLAUDE.md",
	}
}

func TestClaudeRenderer_Name(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	if r.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", r.Name(), "claude")
	}
}

func TestClaudeRenderer_AgentType(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	if r.AgentType() != "claude" {
		t.Errorf("AgentType() = %q, want %q", r.AgentType(), "claude")
	}
}

func TestClaudeRenderer_NeedsCopyMode(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	if !r.NeedsCopyMode() {
		t.Error("NeedsCopyMode() = false, want true")
	}
}

func TestClaudeRenderer_Definition(t *testing.T) {
	def := claudeAgentDef()
	r := renderers.NewClaudeRenderer(def)
	got := r.Definition()
	if got.Name != def.Name || got.Type != def.Type || got.Workspace != def.Workspace {
		t.Errorf("Definition() mismatch: got %+v, want %+v", got, def)
	}
}

// TestClaudeRenderer_RenderSkill_Full tests that a canonical skill with all fields
// is correctly rendered: non-Claude fields are dropped.
func TestClaudeRenderer_RenderSkill_Full(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	inputPath := goldenInputPath(t, "canonical-full.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "claude-full.md")
}

// TestClaudeRenderer_RenderSkill_Minimal tests rendering a skill with only name + description.
func TestClaudeRenderer_RenderSkill_Minimal(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	inputPath := goldenInputPath(t, "canonical-minimal.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "claude-minimal.md")
}

// TestClaudeRenderer_RenderSkill_DropsNonClaudeFields verifies the specific fields
// that Claude drops.
func TestClaudeRenderer_RenderSkill_DropsNonClaudeFields(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Write a skill with all drop-candidate fields.
	input := `---
name: test-skill
description: Test description
mode: subagent
reasoning-effort: low
temperature: 0.5
tools-mode: auto
---
Body here.
`
	srcDir := t.TempDir()
	skillFile := filepath.Join(srcDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(input), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	destDir := t.TempDir()
	if err := r.RenderSkill(srcDir, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	fm, _, err := parse.ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("parse output frontmatter: %v", err)
	}

	droppedFields := []string{"mode", "reasoning-effort", "temperature", "tools-mode"}
	for _, field := range droppedFields {
		if _, ok := fm[field]; ok {
			t.Errorf("field %q should have been dropped but is present in output", field)
		}
	}

	// name and description must be preserved.
	if fm["name"] != "test-skill" {
		t.Errorf("name = %v, want %q", fm["name"], "test-skill")
	}
	if fm["description"] != "Test description" {
		t.Errorf("description = %v, want %q", fm["description"], "Test description")
	}
}

// TestClaudeRenderer_RenderSkill_PreservesAllowedTools verifies allowed-tools passes through.
func TestClaudeRenderer_RenderSkill_PreservesAllowedTools(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	input := `---
name: tools-skill
description: Has tools
allowed-tools:
  - Bash(git:*)
  - Read
  - Edit
---
Body.
`
	srcDir := t.TempDir()
	skillFile := filepath.Join(srcDir, "SKILL.md")
	_ = os.WriteFile(skillFile, []byte(input), 0o644)

	destDir := t.TempDir()
	_ = r.RenderSkill(srcDir, destDir)

	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if _, ok := fm["allowed-tools"]; !ok {
		t.Error("allowed-tools should be preserved for Claude but was dropped")
	}
}

// TestClaudeRenderer_RenderSkill_DirectoryInput verifies that canonicalPath can be
// either a SKILL.md file or a directory containing SKILL.md.
func TestClaudeRenderer_RenderSkill_DirectoryInput(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	srcDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("---\nname: dir-skill\ndescription: From dir\n---\nBody.\n"), 0o644)

	destDir := t.TempDir()
	if err := r.RenderSkill(srcDir, destDir); err != nil {
		t.Fatalf("RenderSkill with directory input: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not created: %v", err)
	}
}

// TestClaudeRenderer_RenderSkill_FileInput verifies that canonicalPath as a file works.
func TestClaudeRenderer_RenderSkill_FileInput(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	srcDir := t.TempDir()
	skillFile := filepath.Join(srcDir, "SKILL.md")
	_ = os.WriteFile(skillFile, []byte("---\nname: file-skill\ndescription: From file\n---\nBody.\n"), 0o644)

	destDir := t.TempDir()
	if err := r.RenderSkill(skillFile, destDir); err != nil {
		t.Fatalf("RenderSkill with file input: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not created: %v", err)
	}
}

// TestClaudeRenderer_RenderSkill_NonexistentInput verifies error handling.
func TestClaudeRenderer_RenderSkill_NonexistentInput(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	err := r.RenderSkill("/nonexistent/path", t.TempDir())
	if err == nil {
		t.Fatal("expected error for nonexistent path but got none")
	}
}

// --- T028: RenderSettings ---

// TestClaudeRenderer_RenderSettings_WithPermissions verifies settings.json is created
// with the correct permissions structure when Settings is configured.
func TestClaudeRenderer_RenderSettings_WithPermissions(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Bash(git:*)", "Read", "Edit"},
	}
	r := renderers.NewClaudeRenderer(def)
	workspaceDir := t.TempDir()

	if err := r.RenderSettings(workspaceDir, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	settingsPath := filepath.Join(workspaceDir, "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	data := mustReadFile(t, settingsPath)
	content := string(data)

	if !strings.Contains(content, `"allow"`) {
		t.Errorf("settings.json missing 'allow' key; content:\n%s", content)
	}
	if !strings.Contains(content, `"Bash(git:*)"`) {
		t.Errorf("settings.json missing Bash(git:*) permission; content:\n%s", content)
	}
	if !strings.Contains(content, `"Read"`) {
		t.Errorf("settings.json missing Read permission; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_NilSettings verifies that no file is created
// when Settings is nil.
func TestClaudeRenderer_RenderSettings_NilSettings(t *testing.T) {
	def := claudeAgentDef()
	// Settings is nil by default.
	r := renderers.NewClaudeRenderer(def)
	workspaceDir := t.TempDir()

	if err := r.RenderSettings(workspaceDir, nil, nil); err != nil {
		t.Fatalf("RenderSettings with nil settings: %v", err)
	}

	settingsPath := filepath.Join(workspaceDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("settings.json should NOT be created when Settings is nil, but it was created")
	}
}

// TestClaudeRenderer_RenderSettings_WorkflowPermissionsMerged verifies that
// permissions from workflow manifests are merged with base agent permissions.
func TestClaudeRenderer_RenderSettings_WorkflowPermissionsMerged(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Bash(git:*)"},
	}
	r := renderers.NewClaudeRenderer(def)
	workspaceDir := t.TempDir()

	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{Name: "sdd"},
			Components: model.WorkflowComponents{
				Permissions: []string{"Bash(mvn:*)", "Read"},
			},
		},
	}

	if err := r.RenderSettings(workspaceDir, nil, workflows); err != nil {
		t.Fatalf("RenderSettings with workflow permissions: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// Both base and workflow permissions must appear.
	if !strings.Contains(content, `"Bash(git:*)"`) {
		t.Errorf("settings.json missing base permission; content:\n%s", content)
	}
	if !strings.Contains(content, `"Bash(mvn:*)"`) {
		t.Errorf("settings.json missing workflow permission Bash(mvn:*); content:\n%s", content)
	}
	if !strings.Contains(content, `"Read"`) {
		t.Errorf("settings.json missing workflow permission Read; content:\n%s", content)
	}
}

// --- T020: RenderSettings with MCP permissions ---

// TestClaudeRenderer_RenderSettings_MCPAllowPermission verifies that an MCP
// with permissions.level="allow" produces "mcp__<name>__*" in permissions.allow[].
func TestClaudeRenderer_RenderSettings_MCPAllowPermission(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{Permissions: []string{}}
	r := renderers.NewClaudeRenderer(def)

	// Seed normalizedMCPs via RenderMCPs with a fake YAML cache.
	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-atlassian")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: atlassian
command: npx
args: ["-y", "mcp-remote"]
permissions:
  level: allow
`
	if err := os.WriteFile(filepath.Join(mcpDir, "atlassian.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}
	cache := &fakeCacheStore{dirs: map[string]string{"hash-atlassian": mcpDir}}
	mcps := []model.LockedMCP{{Name: "atlassian", Hash: "hash-atlassian"}}
	workspaceDir := t.TempDir()
	if err := r.RenderMCPs(mcps, cache, workspaceDir); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	if !strings.Contains(content, `"mcp__atlassian__*"`) {
		t.Errorf("settings.json missing mcp__atlassian__* permission; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_MCPNoPermissions verifies that an MCP
// without a permissions block does not add extra entries.
func TestClaudeRenderer_RenderSettings_MCPNoPermissions(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{Permissions: []string{}}
	r := renderers.NewClaudeRenderer(def)

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-noperms")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: noperms
command: node
args: ["server.js"]
`
	if err := os.WriteFile(filepath.Join(mcpDir, "noperms.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}
	cache := &fakeCacheStore{dirs: map[string]string{"hash-noperms": mcpDir}}
	mcps := []model.LockedMCP{{Name: "noperms", Hash: "hash-noperms"}}
	workspaceDir := t.TempDir()
	if err := r.RenderMCPs(mcps, cache, workspaceDir); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	if strings.Contains(content, "mcp__noperms__*") {
		t.Errorf("settings.json must not contain mcp__noperms__* (no permissions declared); content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_MCPDeduplication verifies that MCP permissions
// are deduplicated if the same pattern would be added twice.
func TestClaudeRenderer_RenderSettings_MCPDeduplication(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{Permissions: []string{"mcp__atlassian__*"}}
	r := renderers.NewClaudeRenderer(def)

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-atlassian2")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: atlassian
command: npx
args: ["-y", "mcp-remote"]
permissions:
  level: allow
`
	if err := os.WriteFile(filepath.Join(mcpDir, "atlassian.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}
	cache := &fakeCacheStore{dirs: map[string]string{"hash-atlassian2": mcpDir}}
	mcps := []model.LockedMCP{{Name: "atlassian", Hash: "hash-atlassian2"}}
	workspaceDir := t.TempDir()
	if err := r.RenderMCPs(mcps, cache, workspaceDir); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, nil); err != nil {
		t.Fatalf("RenderSettings dedup: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	count := strings.Count(content, `"mcp__atlassian__*"`)
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of mcp__atlassian__*, got %d; content:\n%s", count, content)
	}
}

// TestClaudeRenderer_RenderSettings_DeduplicatesPermissions verifies that
// duplicate permissions are not included in settings.json.
func TestClaudeRenderer_RenderSettings_DeduplicatesPermissions(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Bash(git:*)", "Read"},
	}
	r := renderers.NewClaudeRenderer(def)
	workspaceDir := t.TempDir()

	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{Name: "sdd"},
			Components: model.WorkflowComponents{
				// "Read" is a duplicate — should only appear once.
				Permissions: []string{"Read", "Edit"},
			},
		},
	}

	if err := r.RenderSettings(workspaceDir, nil, workflows); err != nil {
		t.Fatalf("RenderSettings dedup: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// Count occurrences of "Read".
	count := strings.Count(content, `"Read"`)
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of \"Read\" in settings.json, got %d; content:\n%s", count, content)
	}
}

// --- T029: Workflow post-processing ---

// TestClaudeRenderer_InstallWorkflow_ReplacesAdvisorTablePlaceholder verifies that
// <!-- ADVISOR_TABLE_PLACEHOLDER --> in a SKILL.md is replaced with the advisor skills table.
func TestClaudeRenderer_InstallWorkflow_ReplacesAdvisorTablePlaceholder(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Set up installed skills containing an advisor-type skill.
	advisors := []model.ContentItem{
		{
			Kind:        model.KindSkill,
			Name:        "unit-test-advisor",
			Path:        "skills/unit-test-advisor/",
			Description: "Unit test patterns and structure",
		},
	}
	r.SetInstalledSkills(advisors)

	// Create a temporary workflow directory with a SKILL.md containing the placeholder.
	wfCacheDir := t.TempDir()

	skillDir := filepath.Join(wfCacheDir, "sdd-explore")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	skillContent := "---\nname: sdd-explore\ndescription: Explore and investigate\n---\n<!-- ADVISOR_TABLE_PLACEHOLDER -->\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Write a minimal workflow.yaml.
	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  skills:
    - sdd-explore
  commands:
    - name: sdd-explore
      action: Explore and investigate
      argument: "<topic>"
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:   []string{"sdd-explore"},
			Commands: []model.WorkflowCommand{{Name: "sdd-explore", Action: "Explore and investigate", Argument: "<topic>"}},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Verify the placeholder was replaced.
	destSkillMD := filepath.Join(workspaceDir, "skills", "sdd-explore", "SKILL.md")
	data := mustReadFile(t, destSkillMD)
	content := string(data)

	if strings.Contains(content, "<!-- ADVISOR_TABLE_PLACEHOLDER -->") {
		t.Errorf("placeholder was not replaced; SKILL.md content:\n%s", content)
	}
	if !strings.Contains(content, "unit-test-advisor") {
		t.Errorf("advisor table not injected; SKILL.md content:\n%s", content)
	}
}

// TestClaudeRenderer_InstallWorkflow_ReplacesPlaceholders verifies that {SKILLS_PATH}
// and {WORKFLOW_DIR} in an ORCHESTRATOR.md file are replaced with actual paths.
func TestClaudeRenderer_InstallWorkflow_ReplacesSkillsPath(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Create a temporary workflow directory with an ORCHESTRATOR.md containing placeholders.
	wfCacheDir := t.TempDir()

	orchestratorContent := "# Orchestrator\n\nSkills: {SKILLS_PATH}\nWorkflow: {WORKFLOW_DIR}\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "ORCHESTRATOR.md"), []byte(orchestratorContent), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	// Write a minimal workflow.yaml.
	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  entrypoint: ORCHESTRATOR.md
  commands:
    - name: sdd-explore
      action: Explore and investigate
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Commands:   []model.WorkflowCommand{{Name: "sdd-explore", Action: "Explore and investigate"}},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Verify {SKILLS_PATH} was replaced in ORCHESTRATOR.md.
	destOrchestrator := filepath.Join(workspaceDir, "skills", "sdd", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrchestrator)
	content := string(data)

	if strings.Contains(content, "{SKILLS_PATH}") {
		t.Errorf("{SKILLS_PATH} was not replaced; ORCHESTRATOR.md content:\n%s", content)
	}
	if strings.Contains(content, "{WORKFLOW_DIR}") {
		t.Errorf("{WORKFLOW_DIR} was not replaced; ORCHESTRATOR.md content:\n%s", content)
	}
	// SKILLS_PATH is the base skills directory; WORKFLOW_DIR includes the workingDir (defaults to name "sdd").
	if !strings.Contains(content, "skills/sdd") {
		t.Errorf("replaced workflow dir does not reference workflow workingDir; content:\n%s", content)
	}
}

// TestClaudeRenderer_InstallWorkflow_ResolvesSddModelPlaceholders verifies that
// {SDD_MODEL_EXPLORE}, {SDD_MODEL_PLAN}, {SDD_MODEL_IMPLEMENT}, and {SDD_MODEL_REVIEW}
// placeholders in workflow .md files are replaced with the resolved model IDs from
// workflow role metadata.
func TestClaudeRenderer_InstallWorkflow_ResolvesSddModelPlaceholders(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	wfCacheDir := t.TempDir()

	// ORCHESTRATOR.md contains all four {SDD_MODEL_*} placeholders.
	orchestratorContent := `# Orchestrator

Explore model: {SDD_MODEL_EXPLORE}
Plan model: {SDD_MODEL_PLAN}
Implement model: {SDD_MODEL_IMPLEMENT}
Review model: {SDD_MODEL_REVIEW}
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "ORCHESTRATOR.md"), []byte(orchestratorContent), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-explorer", Kind: "subagent", Models: map[string]string{"claude": "sonnet"}},
				{Name: "sdd-planner", Kind: "subagent", Models: map[string]string{"claude": "opus"}},
				{Name: "sdd-implementer", Kind: "subagent", Models: map[string]string{"claude": "sonnet"}},
				{Name: "sdd-reviewer", Kind: "subagent", Models: map[string]string{"claude": "haiku"}},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destOrchestrator := filepath.Join(workspaceDir, "skills", "sdd", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrchestrator)
	content := string(data)

	// No unresolved placeholders should remain.
	for _, placeholder := range []string{"{SDD_MODEL_EXPLORE}", "{SDD_MODEL_PLAN}", "{SDD_MODEL_IMPLEMENT}", "{SDD_MODEL_REVIEW}"} {
		if strings.Contains(content, placeholder) {
			t.Errorf("%s was not replaced; ORCHESTRATOR.md content:\n%s", placeholder, content)
		}
	}

	// Claude uses short model names (the Agent tool understands "sonnet", "opus", "haiku").
	expectations := map[string]string{
		"Explore model":   "sonnet",
		"Plan model":      "opus",
		"Implement model": "sonnet",
		"Review model":    "haiku",
	}
	for label, wantModel := range expectations {
		if !strings.Contains(content, wantModel) {
			t.Errorf("%s: expected resolved model %q in content:\n%s", label, wantModel, content)
		}
	}
}

// TestClaudeRenderer_InstallWorkflow_RegistryNoDoubleSlash verifies that {SKILLS_PATH}
// in REGISTRY.md is resolved without double slashes and without spurious subdirectories.
// The real catalog uses {SKILLS_PATH}/ORCHESTRATOR.md (ORCHESTRATOR.md sits directly
// under the workflow dir, not under sdd-orchestrator/).
func TestClaudeRenderer_InstallWorkflow_RegistryNoDoubleSlash(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	wfCacheDir := t.TempDir()

	// REGISTRY.md uses {WORKFLOW_DIR}/ORCHESTRATOR.md — matching the real catalog template.
	registryContent := "Full orchestrator instructions: {WORKFLOW_DIR}/ORCHESTRATOR.md\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(registryContent), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{Registry: "REGISTRY.md"},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Registry content is stored in the renderer for later use by RenderRootCatalog.
	contents := r.RegistryContents()
	content, ok := contents[wf.Metadata.Name]
	if !ok {
		t.Fatal("expected registry content for workflow 'sdd' but not found")
	}

	if strings.Contains(content, "//") {
		t.Errorf("double slash found in registry content; got:\n%s", content)
	}
	if strings.Contains(content, "{WORKFLOW_DIR}") {
		t.Errorf("{WORKFLOW_DIR} was not resolved; got:\n%s", content)
	}
	// Path must resolve to skills/sdd/ORCHESTRATOR.md — workingDir defaults to name "sdd".
	wantSuffix := "skills/sdd/ORCHESTRATOR.md"
	if !strings.Contains(content, wantSuffix) {
		t.Errorf("expected path containing %q; got:\n%s", wantSuffix, content)
	}
}

// TestClaudeRenderer_InstallWorkflow_RegistryClaudeVariantPreferred verifies that
// when REGISTRY.claude.md exists alongside REGISTRY.md, the Claude renderer uses the
// variant so that CLAUDE.md receives Agent(subagent_type:...) launch instructions
// instead of the generic Task()+Skill() pattern.
func TestClaudeRenderer_InstallWorkflow_RegistryClaudeVariantPreferred(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	wfCacheDir := t.TempDir()

	// Generic REGISTRY.md — contains Task()+Skill() language (wrong for Claude-native).
	genericRegistry := "## SDD — How to Start\n\n2. Read `_shared/launch-templates.md` — Task() templates\n4. Launch sub-agents via `Task()` tool\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(genericRegistry), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}

	// Claude-native REGISTRY.claude.md — contains Agent(subagent_type:...) language.
	claudeRegistry := "## SDD — How to Start\n\n2. Create artifact directory\n3. Launch sub-agents via `Agent(subagent_type: 'sdd-{phase}')` — skills preloaded via frontmatter\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.claude.md"), []byte(claudeRegistry), 0o644); err != nil {
		t.Fatalf("write REGISTRY.claude.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{Registry: "REGISTRY.md"},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	contents := r.RegistryContents()
	content, ok := contents[wf.Metadata.Name]
	if !ok {
		t.Fatal("expected registry content for workflow 'sdd' but not found")
	}

	// Must use the Claude-native variant — Agent() language must be present.
	if !strings.Contains(content, "Agent(subagent_type:") {
		t.Errorf("registry content does not use Claude-native Agent() language; got:\n%s", content)
	}
	// Must NOT contain Task()+Skill() language from the generic REGISTRY.md.
	if strings.Contains(content, "Task()") {
		t.Errorf("registry content contains generic Task() language; Claude-native variant should win; got:\n%s", content)
	}
	if strings.Contains(content, "launch-templates.md") {
		t.Errorf("registry content references launch-templates.md; Claude-native variant should not; got:\n%s", content)
	}
}

// TestClaudeRenderer_InstallWorkflow_RegistryClaudeVariantNotCopiedToWorkspace verifies
// that REGISTRY.claude.md is never copied as a loose file into the workspace.
func TestClaudeRenderer_InstallWorkflow_RegistryClaudeVariantNotCopiedToWorkspace(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	wfCacheDir := t.TempDir()

	genericRegistry := "Generic registry content.\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(genericRegistry), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}
	claudeRegistry := "Claude-native registry content.\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.claude.md"), []byte(claudeRegistry), 0o644); err != nil {
		t.Fatalf("write REGISTRY.claude.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{Registry: "REGISTRY.md"},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Neither REGISTRY.md nor REGISTRY.claude.md should be copied as a loose file.
	destBase := filepath.Join(workspaceDir, "skills", "sdd")
	if _, err := os.Stat(filepath.Join(destBase, "REGISTRY.md")); err == nil {
		t.Error("REGISTRY.md was copied to workspace; it must be read-only for catalog injection")
	}
	if _, err := os.Stat(filepath.Join(destBase, "REGISTRY.claude.md")); err == nil {
		t.Error("REGISTRY.claude.md was copied to workspace; it must be suppressed like the generic registry")
	}
}

// TestClaudeRenderer_InstallWorkflow_RegistryFallsBackWhenVariantMissing verifies
// that when REGISTRY.claude.md is absent, the generic REGISTRY.md content is used.
func TestClaudeRenderer_InstallWorkflow_RegistryFallsBackWhenVariantMissing(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	wfCacheDir := t.TempDir()

	genericRegistry := "Generic registry content with Task() language.\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(genericRegistry), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{Registry: "REGISTRY.md"},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	contents := r.RegistryContents()
	content, ok := contents[wf.Metadata.Name]
	if !ok {
		t.Fatal("expected registry content for workflow 'sdd' but not found")
	}

	// Falls back to generic REGISTRY.md content.
	if !strings.Contains(content, "Generic registry content") {
		t.Errorf("registry content does not contain generic fallback text; got:\n%s", content)
	}
}

// TestClaudeRenderer_InstallWorkflow_NoAdvisorSkills verifies that when no advisor
// skills are installed, the placeholder is removed (replaced with empty string).
func TestClaudeRenderer_InstallWorkflow_NoAdvisorSkills(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// No advisor skills set — installedSkills is empty.
	r.SetInstalledSkills([]model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/", Description: "Commit changes"},
	})

	wfCacheDir := t.TempDir()

	skillDir := filepath.Join(wfCacheDir, "sdd-explore")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	skillContent := "---\nname: sdd-explore\ndescription: Explore\n---\n<!-- ADVISOR_TABLE_PLACEHOLDER -->\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  skills:\n    - sdd-explore\n  commands:\n    - name: sdd-explore\n      action: Explore\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:   []string{"sdd-explore"},
			Commands: []model.WorkflowCommand{{Name: "sdd-explore", Action: "Explore"}},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destSkillMD := filepath.Join(workspaceDir, "skills", "sdd-explore", "SKILL.md")
	data := mustReadFile(t, destSkillMD)
	content := string(data)

	// Placeholder should be gone.
	if strings.Contains(content, "<!-- ADVISOR_TABLE_PLACEHOLDER -->") {
		t.Errorf("placeholder should be removed when no advisor skills installed; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderCommand verifies command rendering.
func TestClaudeRenderer_RenderCommand(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()

	cmd := model.WorkflowCommand{
		Name:     "sdd-explore",
		Action:   "Explore and investigate a topic",
		Argument: "<topic>",
	}

	if err := r.RenderCommand(cmd, destDir); err != nil {
		t.Fatalf("RenderCommand: %v", err)
	}

	data := mustReadFile(t, filepath.Join(destDir, "SKILL.md"))
	fm, _, err := parse.ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}

	if fm["name"] != "sdd-explore" {
		t.Errorf("name = %v, want %q", fm["name"], "sdd-explore")
	}
	if _, ok := fm["argument-hint"]; !ok {
		t.Error("argument-hint should be present when Argument is non-empty")
	}
}

// TestClaudeRenderer_RenderCommand_NoArgument verifies argument-hint is omitted
// when the command has no argument.
func TestClaudeRenderer_RenderCommand_NoArgument(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()

	cmd := model.WorkflowCommand{Name: "no-arg", Action: "Do something"}

	if err := r.RenderCommand(cmd, destDir); err != nil {
		t.Fatalf("RenderCommand: %v", err)
	}

	data := mustReadFile(t, filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)
	if _, ok := fm["argument-hint"]; ok {
		t.Error("argument-hint should be absent when Argument is empty")
	}
}

// TestClaudeRenderer_Finalize verifies Finalize is a no-op.
func TestClaudeRenderer_Finalize(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	if err := r.Finalize(t.TempDir()); err != nil {
		t.Errorf("Finalize: unexpected error: %v", err)
	}
}

// --- MCP agentInstructions injection ---

// TestClaudeRenderer_RenderMCPs_ExtractsAgentInstructions verifies that
// agentInstructions from an MCP YAML are extracted and injected into the catalog,
// and stripped from the .mcp.json output.
func TestClaudeRenderer_RenderMCPs_ExtractsAgentInstructions(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Create a fake cache dir with an MCP YAML containing agentInstructions.
	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "abc123")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mcpYAML := `name: engram
command: npx
args:
  - "-y"
  - "@engramhq/engram-mcp"
env:
  ENGRAM_API_KEY: "${ENGRAM_API_KEY}"
agentInstructions: |
  ## Memory

  You have access to Engram persistent memory.
  Save proactively after significant work.
`
	if err := os.WriteFile(filepath.Join(mcpDir, "engram.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}

	cache := &fakeCacheStore{dirs: map[string]string{"abc123": mcpDir}}
	mcps := []model.LockedMCP{{Name: "engram", Hash: "abc123"}}

	// Create a temp workspace dir for .mcp.json output.
	workspaceDir := t.TempDir()
	workspaceRoot := filepath.Join(workspaceDir, ".claude")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	// Verify .mcp.json does NOT contain agentInstructions or name.
	mcpJSON := mustReadFile(t, filepath.Join(workspaceDir, ".mcp.json"))
	mcpContent := string(mcpJSON)
	if strings.Contains(mcpContent, "agentInstructions") {
		t.Errorf(".mcp.json should not contain agentInstructions; content:\n%s", mcpContent)
	}
	if strings.Contains(mcpContent, `"name"`) {
		t.Errorf(".mcp.json should not contain name field; content:\n%s", mcpContent)
	}

	// Verify MCP instructions are stored in the renderer for later use by RenderRootCatalog.
	mcpInstructions := r.MCPAgentInstructions()
	if len(mcpInstructions) == 0 {
		t.Fatal("MCPAgentInstructions should not be empty after RenderMCPs")
	}
	found := false
	for _, instructions := range mcpInstructions {
		if strings.Contains(instructions, "Engram persistent memory") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MCPAgentInstructions should contain injected MCP instructions; got:\n%v", mcpInstructions)
	}
}

// fakeCacheStore is a test double for matypes.CacheStore.
type fakeCacheStore struct {
	dirs map[string]string
}

func (f *fakeCacheStore) Has(hash string) bool {
	_, ok := f.dirs[hash]
	return ok
}

func (f *fakeCacheStore) Get(hash string) (string, bool) {
	dir, ok := f.dirs[hash]
	return dir, ok
}

func (f *fakeCacheStore) Store(hash string, data []byte) (string, error) {
	return "", nil
}

// --- helpers ---

// goldenInputPath returns the absolute path to a golden input file.
func goldenInputPath(t *testing.T, filename string) string {
	t.Helper()
	// From renderers package, testdata is in the parent materialize package.
	path := filepath.Join("..", "testdata", "golden", "input", filename)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("golden input %q not found: %v", path, err)
	}
	return path
}

// goldenExpectedPath returns the absolute path to a golden expected file.
func goldenExpectedPath(t *testing.T, filename string) string {
	t.Helper()
	return filepath.Join("..", "testdata", "golden", "expected", filename)
}

// compareWithGolden reads the actual output file and the golden expected file,
// then compares them byte-for-byte.
func compareWithGolden(t *testing.T, actualPath, expectedFilename string) {
	t.Helper()
	actual, err := os.ReadFile(actualPath)
	if err != nil {
		t.Fatalf("read actual %q: %v", actualPath, err)
	}
	expectedPath := goldenExpectedPath(t, expectedFilename)
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read expected %q: %v", expectedPath, err)
	}
	if string(actual) != string(expected) {
		t.Errorf("output mismatch for %s:\nwant:\n%s\ngot:\n%s",
			expectedFilename, string(expected), string(actual))
	}
}

// mustReadFile reads a file and fails the test if it cannot.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return data
}

// --- T023: RenderSettings deep-merges hook JSON ---

// TestClaudeRenderer_RenderSettings_HookJSONMerged verifies that RenderSettings
// deep-merges hook JSON from a workflow's hook definitions into settings.json
// alongside the existing permissions block.
func TestClaudeRenderer_RenderSettings_HookJSONMerged(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Bash(git:*)"},
	}
	r := renderers.NewClaudeRenderer(def)

	// Create a minimal workflow cache directory with a hook JSON file.
	cacheDir := t.TempDir()
	hooksDir := filepath.Join(cacheDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks dir: %v", err)
	}
	hookJSON := `{
  "hooks": {
    "PreCompact": [
      {
        "matcher": ".*",
        "hooks": [{"type": "command", "command": "echo precompact"}]
      }
    ]
  }
}`
	hookJSONPath := filepath.Join(hooksDir, "claude-precompact.json")
	if err := os.WriteFile(hookJSONPath, []byte(hookJSON), 0o644); err != nil {
		t.Fatalf("write hook JSON: %v", err)
	}

	// Write a minimal workflow.yaml (required by InstallWorkflow directory walk).
	wfYAML := "apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  commands:\n    - name: sdd-explore\n      action: Explore\n"
	if err := os.WriteFile(filepath.Join(cacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Commands: []model.WorkflowCommand{{Name: "sdd-explore", Action: "Explore"}},
			Hooks: &model.WorkflowHooksConfig{
				Agents: map[string][]model.WorkflowHookDef{
					"claude": {{Definition: "hooks/claude-precompact.json"}},
				},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, cacheDir, cacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, []model.WorkflowManifest{wf}); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// permissions must still be present
	if !strings.Contains(content, `"Bash(git:*)"`) {
		t.Errorf("settings.json missing base permission; content:\n%s", content)
	}
	// hook must have been deep-merged in
	if !strings.Contains(content, "PreCompact") {
		t.Errorf("settings.json missing hooks.PreCompact after merge; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_InvalidHookJSONSkipped verifies that an invalid
// hook JSON file produces a warning and is skipped — settings.json is still valid
// and contains the permissions block without any hooks key.
func TestClaudeRenderer_RenderSettings_InvalidHookJSONSkipped(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Read"},
	}
	r := renderers.NewClaudeRenderer(def)

	cacheDir := t.TempDir()
	hooksDir := filepath.Join(cacheDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write malformed JSON.
	if err := os.WriteFile(filepath.Join(hooksDir, "bad.json"), []byte(`{ "hooks": [`), 0o644); err != nil {
		t.Fatalf("write bad JSON: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  commands:\n    - name: run\n      action: Run\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Commands: []model.WorkflowCommand{{Name: "run", Action: "Run"}},
			Hooks: &model.WorkflowHooksConfig{
				Agents: map[string][]model.WorkflowHookDef{
					"claude": {{Definition: "hooks/bad.json"}},
				},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, cacheDir, cacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// RenderSettings must succeed despite the invalid hook JSON.
	if err := r.RenderSettings(workspaceDir, nil, []model.WorkflowManifest{wf}); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// settings.json must be valid and contain permissions
	if !strings.Contains(content, `"Read"`) {
		t.Errorf("settings.json missing Read permission; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_NoHooksForClaudeAgent verifies that a workflow
// with no hooks for the "claude" agent produces settings.json without a hooks key.
func TestClaudeRenderer_RenderSettings_NoHooksForClaudeAgent(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Edit"},
	}
	r := renderers.NewClaudeRenderer(def)

	cacheDir := t.TempDir()
	hooksDir := filepath.Join(cacheDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Hook only for opencode — claude agent has no hooks.
	hookJSON := `{"hooks": {"Compact": []}}`
	if err := os.WriteFile(filepath.Join(hooksDir, "opencode.json"), []byte(hookJSON), 0o644); err != nil {
		t.Fatalf("write hook JSON: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  commands:\n    - name: run\n      action: Run\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Commands: []model.WorkflowCommand{{Name: "run", Action: "Run"}},
			Hooks: &model.WorkflowHooksConfig{
				Agents: map[string][]model.WorkflowHookDef{
					"opencode": {{Definition: "hooks/opencode.json"}},
					// no "claude" entry
				},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, cacheDir, cacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, []model.WorkflowManifest{wf}); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// hooks key must NOT appear (no claude hooks)
	if strings.Contains(content, `"hooks"`) {
		t.Errorf("settings.json must not contain hooks key when no claude hooks defined; content:\n%s", content)
	}
	if !strings.Contains(content, `"Edit"`) {
		t.Errorf("settings.json missing Edit permission; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderSettings_MultipleHookFilesForClaudeAgentMerged verifies
// that multiple hook JSON files for the same agent are merged sequentially into
// settings.json.
func TestClaudeRenderer_RenderSettings_MultipleHookFilesForClaudeAgentMerged(t *testing.T) {
	def := claudeAgentDef()
	def.Settings = &model.SettingsConfig{
		Permissions: []string{"Read"},
	}
	r := renderers.NewClaudeRenderer(def)

	cacheDir := t.TempDir()
	hooksDir := filepath.Join(cacheDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	hook1JSON := `{"hooks": {"PreCompact": [{"matcher": ".*", "hooks": [{"type": "command", "command": "pre.sh"}]}]}}`
	hook2JSON := `{"hooks": {"UserPromptSubmit": [{"matcher": ".*", "hooks": [{"type": "command", "command": "prompt.sh"}]}]}}`

	if err := os.WriteFile(filepath.Join(hooksDir, "precompact.json"), []byte(hook1JSON), 0o644); err != nil {
		t.Fatalf("write hook1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "userprompt.json"), []byte(hook2JSON), 0o644); err != nil {
		t.Fatalf("write hook2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  commands:\n    - name: run\n      action: Run\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Commands: []model.WorkflowCommand{{Name: "run", Action: "Run"}},
			Hooks: &model.WorkflowHooksConfig{
				Agents: map[string][]model.WorkflowHookDef{
					"claude": {
						{Definition: "hooks/precompact.json"},
						{Definition: "hooks/userprompt.json"},
					},
				},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, cacheDir, cacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if err := r.RenderSettings(workspaceDir, nil, []model.WorkflowManifest{wf}); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	data := mustReadFile(t, filepath.Join(workspaceDir, "settings.json"))
	content := string(data)

	// Both hook events must appear in the merged output.
	if !strings.Contains(content, "PreCompact") {
		t.Errorf("settings.json missing PreCompact after sequential merge; content:\n%s", content)
	}
	if !strings.Contains(content, "UserPromptSubmit") {
		t.Errorf("settings.json missing UserPromptSubmit after sequential merge; content:\n%s", content)
	}
}

// --- T007: Claude-native renderer variant and subagent coverage ---

// sddTestWorkflow returns the canonical SDD workflow manifest used by the T007
// test suite: four phase subagents + orchestrator + advisor sentinel, matching
// the real `devrune-starter-catalog/workflows/sdd/workflow.yaml`.
func sddTestWorkflow() model.WorkflowManifest {
	return model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-explore", "sdd-plan", "sdd-implement", "sdd-review"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-explorer", Kind: "subagent", Skill: "sdd-explore", Models: map[string]string{"claude": "sonnet", "opencode": "sonnet", "copilot": "Claude Sonnet 4.6"}},
				{Name: "sdd-planner", Kind: "subagent", Skill: "sdd-plan", Models: map[string]string{"claude": "opus", "opencode": "opus", "copilot": "Claude Opus 4.6"}},
				{Name: "sdd-implementer", Kind: "subagent", Skill: "sdd-implement", Models: map[string]string{"claude": "sonnet", "opencode": "sonnet", "copilot": "Claude Sonnet 4.6"}},
				{Name: "sdd-reviewer", Kind: "subagent", Skill: "sdd-review", Models: map[string]string{"claude": "opus", "opencode": "opus", "copilot": "Claude Opus 4.6"}},
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
				{Name: "sdd-advisor", Kind: "subagent", Skill: "*-advisor", Models: map[string]string{"claude": "opus", "opencode": "opus", "copilot": "Claude Opus 4.6"}},
			},
		},
	}
}

// claudeNativeAgentDef returns a Claude agent definition with AgentDir set,
// matching the real claude.yaml used in installs.
func claudeNativeAgentDef() model.AgentDefinition {
	d := claudeAgentDef()
	d.AgentDir = "agents"
	return d
}

// writeClaudeSDDCache seeds a temp cache dir with ORCHESTRATOR.md, the four
// phase SKILL.md files, and workflow.yaml. When variantBody != "", also writes
// ORCHESTRATOR.claude.md with that body.
func writeClaudeSDDCache(t *testing.T, variantBody string) string {
	t.Helper()
	cache := t.TempDir()

	for _, skill := range []string{"sdd-explore", "sdd-plan", "sdd-implement", "sdd-review"} {
		skillDir := filepath.Join(cache, skill)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", skill, err)
		}
		body := "---\nname: " + skill + "\ndescription: " + skill + " skill\n---\nBody of " + skill + ".\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s/SKILL.md: %v", skill, err)
		}
	}
	if err := os.WriteFile(filepath.Join(cache, "ORCHESTRATOR.md"), []byte("# Generic Orchestrator\nGeneric body.\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}
	if variantBody != "" {
		if err := os.WriteFile(filepath.Join(cache, "ORCHESTRATOR.claude.md"), []byte(variantBody), 0o644); err != nil {
			t.Fatalf("write ORCHESTRATOR.claude.md: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(cache, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}
	return cache
}

// TestClaudeRenderer_InstallWorkflow_VariantProbe verifies that
// ORCHESTRATOR.claude.md is preferred over ORCHESTRATOR.md when both exist,
// and that the destination filename is always ORCHESTRATOR.md (suffix stripped).
func TestClaudeRenderer_InstallWorkflow_VariantProbe(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())

	variantMarker := "# Claude-native Orchestrator\nVARIANT_BODY_SENTINEL\n"
	cache := writeClaudeSDDCache(t, variantMarker)
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destOrch := filepath.Join(workspace, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrch)
	if !strings.Contains(string(data), "VARIANT_BODY_SENTINEL") {
		t.Errorf("installed ORCHESTRATOR.md does not contain Claude variant body; got:\n%s", string(data))
	}
	if strings.Contains(string(data), "Generic body.") {
		t.Errorf("installed ORCHESTRATOR.md unexpectedly contains generic body; variant should win. content:\n%s", string(data))
	}

	// No ORCHESTRATOR.claude.md should be copied into the workspace.
	if _, err := os.Stat(filepath.Join(workspace, "skills", "sdd-orchestrator", "ORCHESTRATOR.claude.md")); err == nil {
		t.Error("ORCHESTRATOR.claude.md was copied into workspace; suffix must be stripped")
	}
}

// TestClaudeRenderer_InstallWorkflow_FallsBackWhenVariantMissing verifies that
// when ORCHESTRATOR.claude.md is absent, the generic ORCHESTRATOR.md body is
// installed (no error).
func TestClaudeRenderer_InstallWorkflow_FallsBackWhenVariantMissing(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "") // no variant
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destOrch := filepath.Join(workspace, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrch)
	if !strings.Contains(string(data), "Generic body.") {
		t.Errorf("installed ORCHESTRATOR.md missing generic body; got:\n%s", string(data))
	}
}

// readAgentFrontmatter reads a generated .claude/agents/{name}.md file and
// returns its parsed frontmatter map.
func readAgentFrontmatter(t *testing.T, workspace, agentName string) map[string]any {
	t.Helper()
	path := filepath.Join(workspace, "agents", agentName+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s.md: %v", agentName, err)
	}
	fm, _, err := parse.ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("parse frontmatter for %s.md: %v", agentName, err)
	}
	return fm
}

// TestClaudeRenderer_InstallWorkflow_PhaseSubagents_OmitToolsField asserts each
// phase subagent file has NO `tools:` key in its parsed frontmatter (omitted so
// the subagent inherits the parent's full tool allowlist per Anthropic docs).
func TestClaudeRenderer_InstallWorkflow_PhaseSubagents_OmitToolsField(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	phaseAgents := []string{"sdd-explorer", "sdd-planner", "sdd-implementer", "sdd-reviewer"}
	for _, name := range phaseAgents {
		t.Run(name, func(t *testing.T) {
			fm := readAgentFrontmatter(t, workspace, name)
			if _, ok := fm["tools"]; ok {
				t.Errorf("%s.md: tools field must be OMITTED for phase subagents (to inherit parent allowlist); got: %v", name, fm["tools"])
			}
			// Sanity: skills and permissionMode must be present.
			if _, ok := fm["skills"]; !ok {
				t.Errorf("%s.md: expected 'skills' frontmatter key", name)
			}
			if pm, _ := fm["permissionMode"].(string); pm != "default" {
				t.Errorf("%s.md: permissionMode = %q, want %q", name, pm, "default")
			}
		})
	}
}

// TestClaudeRenderer_InstallWorkflow_AdvisorSubagents_DeclareReadOnlyTools
// asserts that advisor subagent files have explicit `tools: [Read, Grep, Glob]`.
func TestClaudeRenderer_InstallWorkflow_AdvisorSubagents_DeclareReadOnlyTools(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	r.SetInstalledSkills([]model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Clean architecture"},
		{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit tests"},
	})

	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	wantTools := map[string]bool{"Read": true, "Grep": true, "Glob": true}
	for _, name := range []string{"architect-advisor", "unit-test-advisor"} {
		t.Run(name, func(t *testing.T) {
			fm := readAgentFrontmatter(t, workspace, name)
			toolsVal, ok := fm["tools"]
			if !ok {
				t.Fatalf("%s.md: tools field is missing — advisors must declare read-only tools", name)
			}
			toolsList, ok := toolsVal.([]any)
			if !ok {
				t.Fatalf("%s.md: tools is %T, want []any", name, toolsVal)
			}
			got := make(map[string]bool)
			for _, v := range toolsList {
				if s, ok := v.(string); ok {
					got[s] = true
				}
			}
			if len(got) != len(wantTools) {
				t.Errorf("%s.md: tools = %v, want %v", name, got, wantTools)
			}
			for tool := range wantTools {
				if !got[tool] {
					t.Errorf("%s.md: missing tool %q; got %v", name, tool, got)
				}
			}
		})
	}
}

// TestClaudeRenderer_InstallWorkflow_MCPServers verifies that sdd-explorer.md
// has mcpServers: [engram, atlassian] and all other subagents have [engram] only.
func TestClaudeRenderer_InstallWorkflow_MCPServers(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	r.SetInstalledSkills([]model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "arch"},
	})
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	extractServers := func(fm map[string]any) map[string]bool {
		result := make(map[string]bool)
		raw, ok := fm["mcpServers"]
		if !ok {
			return result
		}
		if list, ok := raw.([]any); ok {
			for _, v := range list {
				if s, ok := v.(string); ok {
					result[s] = true
				}
			}
		}
		return result
	}

	cases := []struct {
		agent string
		want  map[string]bool
	}{
		{"sdd-explorer", map[string]bool{"engram": true, "atlassian": true}},
		{"sdd-planner", map[string]bool{"engram": true}},
		{"sdd-implementer", map[string]bool{"engram": true}},
		{"sdd-reviewer", map[string]bool{"engram": true}},
		{"architect-advisor", map[string]bool{"engram": true}},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			got := extractServers(readAgentFrontmatter(t, workspace, tc.agent))
			if len(got) != len(tc.want) {
				t.Errorf("%s.md: mcpServers = %v, want %v", tc.agent, got, tc.want)
			}
			for server := range tc.want {
				if !got[server] {
					t.Errorf("%s.md: missing mcp server %q; got %v", tc.agent, server, got)
				}
			}
			// Stricter checks: explorer must have atlassian; non-explorer must NOT.
			if tc.agent == "sdd-explorer" && !got["atlassian"] {
				t.Errorf("sdd-explorer.md: atlassian mcp server is required")
			}
			if tc.agent != "sdd-explorer" && got["atlassian"] {
				t.Errorf("%s.md: atlassian must NOT be present; got %v", tc.agent, got)
			}
		})
	}
}

// TestClaudeRenderer_InstallWorkflow_ModelPlaceholders verifies that each phase
// subagent file has `model:` resolved to the value from the workflow role —
// NOT left as a raw `{WORKFLOW_MODEL_*}` placeholder.
func TestClaudeRenderer_InstallWorkflow_ModelPlaceholders(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	cases := []struct {
		agent string
		want  string
	}{
		{"sdd-explorer", "sonnet"},
		{"sdd-planner", "opus"},
		{"sdd-implementer", "sonnet"},
		{"sdd-reviewer", "opus"},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			fm := readAgentFrontmatter(t, workspace, tc.agent)
			got, _ := fm["model"].(string)
			if got != tc.want {
				t.Errorf("%s.md: model = %q, want %q (placeholder must be resolved from role.Model)", tc.agent, got, tc.want)
			}
			if strings.Contains(got, "{WORKFLOW_MODEL_") {
				t.Errorf("%s.md: unresolved placeholder in model field: %q", tc.agent, got)
			}
		})
	}
}

// TestClaudeRenderer_InstallWorkflow_ModelOverrideFromTUI verifies that a TUI
// override via SetModelOverrides wins over the role.Model default from workflow.yaml.
func TestClaudeRenderer_InstallWorkflow_ModelOverrideFromTUI(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	// Override sdd-planner (default "opus") with "opus" and sdd-explorer (default
	// "sonnet") with "opus" — verify the override value lands in the agent file.
	r.SetModelOverrides(map[string]string{
		"sdd-explorer": "opus",
	})
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	fm := readAgentFrontmatter(t, workspace, "sdd-explorer")
	got, _ := fm["model"].(string)
	if got != "opus" {
		t.Errorf("sdd-explorer.md: model = %q, want %q (TUI override must win over role.Model=sonnet)", got, "opus")
	}

	// Sanity: sdd-planner (no override) should still get its workflow.yaml default "opus".
	plannerFM := readAgentFrontmatter(t, workspace, "sdd-planner")
	plannerModel, _ := plannerFM["model"].(string)
	if plannerModel != "opus" {
		t.Errorf("sdd-planner.md: model = %q, want %q (no override, should use role.Model)", plannerModel, "opus")
	}
}

// TestClaudeRenderer_InstallWorkflow_FivePhrasesInOrchestrator asserts the
// generated installed ORCHESTRATOR.md (from ORCHESTRATOR.claude.md source)
// contains all 5 critical phrases — locking in the verbatim-draft contract.
func TestClaudeRenderer_InstallWorkflow_FivePhrasesInOrchestrator(t *testing.T) {
	// Source body simulates the Claude-native ORCHESTRATOR.claude.md with all
	// five critical phrases present. In the real catalog, this content is
	// authored at devrune-starter-catalog/workflows/sdd/ORCHESTRATOR.claude.md (T009).
	variantBody := `# Claude-native SDD Orchestrator

## Post-Phase Protocol (MANDATORY)

Runs after every sub-agent.

5. **Guidance loop** (plan phase only): re-enter plan with crit feedback.

## Crit Plan Review Protocol

Auto-launched after plan returns ok when the crit tool is available.

Crit confirmation guard: if crit succeeds, require state.yaml crit_completed = true.

Gotchas:
- Crit timeout ≠ approval — absence of .crit.json means the review was NEVER completed.
`
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, variantBody)
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destOrch := filepath.Join(workspace, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrch)
	body := string(data)

	phrases := []string{
		"Post-Phase Protocol",
		"Guidance loop",
		"Crit Plan Review Protocol",
		"Crit confirmation guard",
		"Crit timeout",
	}
	for _, phrase := range phrases {
		if !strings.Contains(body, phrase) {
			t.Errorf("installed ORCHESTRATOR.md missing critical phrase %q; content:\n%s", phrase, body)
		}
	}
}

// TestClaudeRenderer_InstallWorkflow_ManagedPaths_PreserveUserAgents is the
// critical regression test: on reinstall, user-authored files in
// .claude/agents/ must NOT be removed. ManagedPaths must list each synthesized
// file individually, NOT the agents/ directory itself.
func TestClaudeRenderer_InstallWorkflow_ManagedPaths_PreserveUserAgents(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	result, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	agentsBase := filepath.Join(workspace, "agents")

	// No managed path may equal the agents/ directory itself — if it did, the
	// materializer's os.RemoveAll would wipe user-authored subagents on reinstall.
	for _, p := range result.ManagedPaths {
		if filepath.Clean(p) == filepath.Clean(agentsBase) {
			t.Errorf("ManagedPaths contains .claude/agents/ directory (%q); must list each synthesized agent file individually to preserve user-authored agents on reinstall", p)
		}
	}

	// Each synthesized phase agent file MUST be individually listed in ManagedPaths.
	want := []string{
		filepath.Join(agentsBase, "sdd-explorer.md"),
		filepath.Join(agentsBase, "sdd-planner.md"),
		filepath.Join(agentsBase, "sdd-implementer.md"),
		filepath.Join(agentsBase, "sdd-reviewer.md"),
	}
	managed := make(map[string]bool)
	for _, p := range result.ManagedPaths {
		managed[filepath.Clean(p)] = true
	}
	for _, w := range want {
		if !managed[filepath.Clean(w)] {
			t.Errorf("ManagedPaths missing synthesized agent file %q; got: %v", w, result.ManagedPaths)
		}
	}
}

// TestClaudeRenderer_InstallWorkflow_SubagentFileSizeUnder1KB verifies that
// each synthesized subagent file is under 1 KB — the body must be minimal
// (one-paragraph instruction) and NOT embed the full SKILL.md content.
func TestClaudeRenderer_InstallWorkflow_SubagentFileSizeUnder1KB(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	r.SetInstalledSkills([]model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "arch"},
	})
	cache := writeClaudeSDDCache(t, "")
	workspace := t.TempDir()

	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	const maxBytes = 1024
	agents := []string{"sdd-explorer", "sdd-planner", "sdd-implementer", "sdd-reviewer", "architect-advisor"}
	for _, name := range agents {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(workspace, "agents", name+".md")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat %s.md: %v", name, err)
			}
			if info.Size() >= maxBytes {
				t.Errorf("%s.md: size = %d bytes, want < %d (body must be minimal — do NOT embed SKILL.md content)", name, info.Size(), maxBytes)
			}
		})
	}
}

// --- T009: RegenerateAdvisorFiles ---

// advisorAgentDef returns a ClaudeRenderer backed by a minimal AgentDefinition
// with AgentDir set, ready for RegenerateAdvisorFiles tests.
func advisorAgentDef() model.AgentDefinition {
	d := claudeAgentDef()
	d.AgentDir = "agents"
	return d
}

// agentFilePath returns the expected path for an advisor agent file under workspaceRoot.
func agentFilePath(workspaceRoot, name string) string {
	return filepath.Join(workspaceRoot, "agents", name+".md")
}

// assertAgentFileExists asserts that the agent file for name exists and starts with "---".
func assertAgentFileExists(t *testing.T, workspaceRoot, name string) {
	t.Helper()
	path := agentFilePath(workspaceRoot, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("agent file %s.md not found: %v", name, err)
	}
	if !strings.HasPrefix(string(data), "---") {
		t.Errorf("agent file %s.md missing YAML frontmatter (must start with ---); content:\n%s", name, string(data))
	}
}

// assertAgentFileAbsent asserts that the agent file for name does NOT exist.
func assertAgentFileAbsent(t *testing.T, workspaceRoot, name string) {
	t.Helper()
	path := agentFilePath(workspaceRoot, name)
	if _, err := os.Stat(path); err == nil {
		t.Errorf("agent file %s.md should not exist but was found at %s", name, path)
	}
}

// TestClaudeRenderer_RegenerateAdvisorFiles_InstallTwo verifies that installing
// two advisor items from empty creates two .claude/agents/*.md files with valid
// YAML frontmatter.
func TestClaudeRenderer_RegenerateAdvisorFiles_InstallTwo(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Architecture"},
		{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit tests"},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 2 {
		t.Errorf("Written = %v, want 2 entries", result.Written)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty", result.Deleted)
	}

	assertAgentFileExists(t, workspaceRoot, "architect-advisor")
	assertAgentFileExists(t, workspaceRoot, "unit-test-advisor")
}

// TestClaudeRenderer_RegenerateAdvisorFiles_RemoveOne verifies that removing one
// advisor deletes its file while leaving the other untouched.
func TestClaudeRenderer_RegenerateAdvisorFiles_RemoveOne(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	// First install both.
	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor"},
		{Kind: model.KindSkill, Name: "unit-test-advisor"},
	}
	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("setup install: %v", err)
	}

	// Now remove only "architect-advisor".
	result, err := r.RegenerateAdvisorFiles(workspaceRoot, nil, []string{"architect-advisor"}, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles remove: %v", err)
	}

	if len(result.Deleted) != 1 {
		t.Errorf("Deleted = %v, want 1 entry", result.Deleted)
	}

	assertAgentFileAbsent(t, workspaceRoot, "architect-advisor")
	assertAgentFileExists(t, workspaceRoot, "unit-test-advisor")
}

// TestClaudeRenderer_RegenerateAdvisorFiles_RemoveNonExistent verifies that
// removing a name that has no corresponding file produces no error.
func TestClaudeRenderer_RegenerateAdvisorFiles_RemoveNonExistent(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	_, err := r.RegenerateAdvisorFiles(workspaceRoot, nil, []string{"ghost-advisor"}, nil)
	if err != nil {
		t.Errorf("expected no error for non-existent removal, got: %v", err)
	}
}

// TestClaudeRenderer_RegenerateAdvisorFiles_NonAdvisorNamesIgnored verifies that
// non-advisor names in installed (e.g. "git-commit", "sdd-plan") are silently
// ignored — no files created, no error.
func TestClaudeRenderer_RegenerateAdvisorFiles_NonAdvisorNamesIgnored(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit"},
		{Kind: model.KindSkill, Name: "sdd-plan"},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 0 {
		t.Errorf("Written = %v, want empty (non-advisor names must be ignored)", result.Written)
	}

	assertAgentFileAbsent(t, workspaceRoot, "git-commit")
	assertAgentFileAbsent(t, workspaceRoot, "sdd-plan")
}

// TestClaudeRenderer_RegenerateAdvisorFiles_IdempotencyByteIdentical verifies that
// calling RegenerateAdvisorFiles twice with the same input produces byte-identical
// output files (idempotency / Risk 1 mitigation).
func TestClaudeRenderer_RegenerateAdvisorFiles_IdempotencyByteIdentical(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Description: "Clean architecture"},
	}

	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("first call: %v", err)
	}
	firstBytes, err := os.ReadFile(agentFilePath(workspaceRoot, "architect-advisor"))
	if err != nil {
		t.Fatalf("read after first call: %v", err)
	}

	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("second call: %v", err)
	}
	secondBytes, err := os.ReadFile(agentFilePath(workspaceRoot, "architect-advisor"))
	if err != nil {
		t.Fatalf("read after second call: %v", err)
	}

	if string(firstBytes) != string(secondBytes) {
		t.Errorf("byte-identical re-install failed:\nfirst:\n%s\nsecond:\n%s", string(firstBytes), string(secondBytes))
	}
}

// TestClaudeRenderer_RegenerateAdvisorFiles_CustomFlaggedItem verifies that a
// ContentItem with Custom:true produces an advisor agent file regardless of name suffix.
func TestClaudeRenderer_RegenerateAdvisorFiles_CustomFlaggedItem(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "my-custom-tool", Custom: true},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 1 {
		t.Errorf("Written = %v, want 1 entry for Custom:true item", result.Written)
	}
	assertAgentFileExists(t, workspaceRoot, "my-custom-tool")
}

// TestClaudeRenderer_RegenerateAdvisorFiles_NameInBothInstalledAndRemoved verifies
// the contract when a name appears in both installed and removed: the implementation
// writes the file first (installed phase) then removes it (removed phase), so the
// final state is that the file does NOT exist (removed wins over installed).
func TestClaudeRenderer_RegenerateAdvisorFiles_NameInBothInstalledAndRemoved(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	name := "architect-advisor"
	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: name},
	}
	removed := []string{name}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, removed, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	// The implementation writes installed first, then deletes removed —
	// so the file ends up deleted (removed phase runs after installed phase).
	if len(result.Written) != 1 {
		t.Errorf("Written = %v, want 1 (file was written during installed phase)", result.Written)
	}
	if len(result.Deleted) != 1 {
		t.Errorf("Deleted = %v, want 1 (file was removed during removed phase)", result.Deleted)
	}
	// Final state: file is absent because removal runs after install.
	assertAgentFileAbsent(t, workspaceRoot, name)
}

// TestClaudeRenderer_RegenerateAdvisorFiles_EmptyInputs verifies that nil installed
// and nil removed return an empty AdvisorRenderResult without touching the filesystem.
func TestClaudeRenderer_RegenerateAdvisorFiles_EmptyInputs(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if result.Written != nil {
		t.Errorf("Written = %v, want nil for empty inputs", result.Written)
	}
	if result.Deleted != nil {
		t.Errorf("Deleted = %v, want nil for empty inputs", result.Deleted)
	}

	// Agents directory must not have been created.
	agentsDir := filepath.Join(workspaceRoot, "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		entries, _ := os.ReadDir(agentsDir)
		if len(entries) > 0 {
			t.Errorf("agents/ directory was created with files on empty inputs: %v", entries)
		}
	}
}

// TestClaudeRenderer_RegenerateAdvisorFiles_StatelessReceiver verifies that two
// consecutive calls with different inputs on the same renderer instance produce
// independent results with no state leakage between calls.
func TestClaudeRenderer_RegenerateAdvisorFiles_StatelessReceiver(t *testing.T) {
	r := renderers.NewClaudeRenderer(advisorAgentDef())
	workspaceRoot := t.TempDir()

	// First call: install advisor-a.
	firstInstalled := []model.ContentItem{
		{Kind: model.KindSkill, Name: "advisor-a-advisor"},
	}
	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, firstInstalled, nil, nil); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call: install advisor-b only (no mention of advisor-a).
	secondInstalled := []model.ContentItem{
		{Kind: model.KindSkill, Name: "advisor-b-advisor"},
	}
	result, err := r.RegenerateAdvisorFiles(workspaceRoot, secondInstalled, nil, nil)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Second call should only write advisor-b — no state from first call leaks in.
	if len(result.Written) != 1 {
		t.Errorf("second call Written = %v, want 1 (only advisor-b-advisor)", result.Written)
	}
	if !strings.Contains(result.Written[0], "advisor-b-advisor") {
		t.Errorf("second call Written[0] = %q, expected to contain advisor-b-advisor", result.Written[0])
	}

	// advisor-a was written by the first call and should still exist (not cleaned up by second call).
	assertAgentFileExists(t, workspaceRoot, "advisor-a-advisor")
	// advisor-b was written by the second call.
	assertAgentFileExists(t, workspaceRoot, "advisor-b-advisor")
}

// TestClaudeRenderer_InstallWorkflow_NoSharedClaudeMdFiles verifies that after
// install, no .claude.md files exist under the installed _shared/ directory —
// the Round 3 elimination decision removed all shared Claude variants.
func TestClaudeRenderer_InstallWorkflow_NoSharedClaudeMdFiles(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "")

	// Seed _shared/ with generic files; ensure no .claude.md sibling exists in the
	// source so the renderer has nothing to copy over anyway.
	sharedDir := filepath.Join(cache, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir _shared: %v", err)
	}
	for _, name := range []string{"launch-templates.md", "advisor-templates.md", "envelope-contract.md"} {
		if err := os.WriteFile(filepath.Join(sharedDir, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	workspace := t.TempDir()
	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Walk the installed _shared directory and assert no *.claude.md files exist.
	installedShared := filepath.Join(workspace, "skills", "sdd-orchestrator", "_shared")
	walkErr := filepath.Walk(installedShared, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".claude.md") {
			t.Errorf("unexpected .claude.md file under installed _shared/: %s (Round 3 elimination)", path)
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		t.Fatalf("walk installed _shared: %v", walkErr)
	}
}

// TestClaudeRenderer_InstallWorkflow_SharedVariantSuffixStripping verifies that
// _shared/launch-templates.claude.md is installed as _shared/launch-templates.md,
// while copilot and opencode variant files are skipped entirely.
func TestClaudeRenderer_InstallWorkflow_SharedVariantSuffixStripping(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
	cache := writeClaudeSDDCache(t, "")

	// Seed _shared/ with variant-suffixed files plus a generic file.
	sharedDir := filepath.Join(cache, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir _shared: %v", err)
	}
	sharedFiles := map[string]string{
		"launch-templates.claude.md":   "# claude launch-templates\n",
		"launch-templates.copilot.md":  "# copilot launch-templates\n",
		"launch-templates.opencode.md": "# opencode launch-templates\n",
		"envelope-contract.md":         "# envelope\n",
	}
	for name, content := range sharedFiles {
		if err := os.WriteFile(filepath.Join(sharedDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	workspace := t.TempDir()
	if _, err := r.InstallWorkflow(sddTestWorkflow(), cache, cache, workspace); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	installedShared := filepath.Join(workspace, "skills", "sdd-orchestrator", "_shared")

	// launch-templates.md must exist (from launch-templates.claude.md).
	ltPath := filepath.Join(installedShared, "launch-templates.md")
	data, err := os.ReadFile(ltPath)
	if err != nil {
		t.Fatalf("launch-templates.md not installed: %v", err)
	}
	if string(data) != "# claude launch-templates\n" {
		t.Errorf("launch-templates.md content = %q, want claude variant content", string(data))
	}

	// envelope-contract.md must exist (generic file, no variant suffix).
	if _, err := os.Stat(filepath.Join(installedShared, "envelope-contract.md")); err != nil {
		t.Errorf("envelope-contract.md should be present: %v", err)
	}

	// No variant-suffixed files must exist.
	for _, absent := range []string{
		"launch-templates.claude.md",
		"launch-templates.copilot.md",
		"launch-templates.opencode.md",
	} {
		path := filepath.Join(installedShared, absent)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("variant file %q must not be installed in _shared/", absent)
		}
	}
}

// TestClaudeRenderer_InstallWorkflow_ExternalCatalogSkill verifies that a skill
// listed in workflow.yaml's components.skills but living at the catalog top
// level (catalogRoot/skills/<name>/) — not under the workflow directory — is
// resolved correctly and installed in the workspace.
//
// Scenario mirrors the real catalog layout: SDD's PRD gate references
// write-a-prd, which is a top-level reusable skill at catalog/skills/write-a-prd/,
// not at catalog/workflows/sdd/write-a-prd/. The renderer must reach across
// to catalogRoot to resolve it.
func TestClaudeRenderer_InstallWorkflow_ExternalCatalogSkill(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	catalogRoot := t.TempDir()
	wfCacheDir := filepath.Join(catalogRoot, "workflows", "sdd")
	if err := os.MkdirAll(wfCacheDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	// Internal skill (lives under workflow dir): sdd-explore.
	internalSkillDir := filepath.Join(wfCacheDir, "sdd-explore")
	if err := os.MkdirAll(internalSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir internal skill: %v", err)
	}
	internalContent := "---\nname: sdd-explore\ndescription: Explore phase\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(internalSkillDir, "SKILL.md"), []byte(internalContent), 0o644); err != nil {
		t.Fatalf("write internal SKILL.md: %v", err)
	}

	// External skill (lives at catalog top level): write-a-prd.
	externalSkillDir := filepath.Join(catalogRoot, "skills", "write-a-prd")
	if err := os.MkdirAll(externalSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir external skill: %v", err)
	}
	externalContent := "---\nname: write-a-prd\ndescription: Generate a PRD\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(externalSkillDir, "SKILL.md"), []byte(externalContent), 0o644); err != nil {
		t.Fatalf("write external SKILL.md: %v", err)
	}

	// Workflow.yaml lists BOTH skills in components.skills. The internal one
	// resolves via the existing scan; the external one resolves via the new
	// external pass.
	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  skills:
    - sdd-explore
    - write-a-prd
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills: []string{"sdd-explore", "write-a-prd"},
		},
	}

	workspaceDir := t.TempDir()
	result, err := r.InstallWorkflow(wf, wfCacheDir, catalogRoot, workspaceDir)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Both skills must land at the top-level workspace skills dir.
	internalDest := filepath.Join(workspaceDir, "skills", "sdd-explore", "SKILL.md")
	if _, err := os.Stat(internalDest); err != nil {
		t.Errorf("internal skill not installed at %q: %v", internalDest, err)
	}
	externalDest := filepath.Join(workspaceDir, "skills", "write-a-prd", "SKILL.md")
	if _, err := os.Stat(externalDest); err != nil {
		t.Errorf("external skill not installed at %q: %v", externalDest, err)
	}

	// External skill should appear in ManagedPaths so uninstall can remove it.
	foundExternal := false
	for _, p := range result.ManagedPaths {
		if p == filepath.Join(workspaceDir, "skills", "write-a-prd") {
			foundExternal = true
			break
		}
	}
	if !foundExternal {
		t.Errorf("external skill dir missing from ManagedPaths; got: %v", result.ManagedPaths)
	}
}

// TestClaudeRenderer_InstallWorkflow_ExternalSkillMissingErrors verifies that
// listing a skill in components.skills that is neither workflow-internal nor a
// catalog top-level skill produces a clear error.
func TestClaudeRenderer_InstallWorkflow_ExternalSkillMissingErrors(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	catalogRoot := t.TempDir()
	wfCacheDir := filepath.Join(catalogRoot, "workflows", "sdd")
	if err := os.MkdirAll(wfCacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: sdd\n  version: 1.0.0\ncomponents:\n  skills:\n    - nonexistent\n"), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{Skills: []string{"nonexistent"}},
	}

	workspaceDir := t.TempDir()
	_, err := r.InstallWorkflow(wf, wfCacheDir, catalogRoot, workspaceDir)
	if err == nil {
		t.Fatal("expected error for missing skill, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should name the missing skill, got: %v", err)
	}
}
