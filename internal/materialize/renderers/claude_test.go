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

// TestClaudeRenderer_RenderCatalog_WithSkills verifies catalog format.
func TestClaudeRenderer_RenderCatalog_WithSkills(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	skills := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git:commit", Path: "skills/git-commit/"},
		{Kind: model.KindSkill, Name: "unit-test", Path: "skills/unit-test/"},
	}

	if err := r.RenderCatalog(skills, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	catalog := string(content)

	if !strings.Contains(catalog, "# Claude Code Agent Catalog") {
		t.Error("catalog missing heading")
	}
	if !strings.Contains(catalog, "auto-generated") {
		t.Error("catalog missing auto-generated note")
	}
	if !strings.Contains(catalog, "`git:commit`") {
		t.Errorf("catalog missing git:commit skill; content:\n%s", catalog)
	}
	if !strings.Contains(catalog, "`unit-test`") {
		t.Errorf("catalog missing unit-test skill; content:\n%s", catalog)
	}
}

// TestClaudeRenderer_RenderCatalog_Empty verifies catalog is generated even with no items.
func TestClaudeRenderer_RenderCatalog_Empty(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	if err := r.RenderCatalog(nil, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog empty: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if !strings.Contains(string(content), "# Claude Code Agent Catalog") {
		t.Error("empty catalog missing heading")
	}
}

// TestClaudeRenderer_RenderCatalog_WithWorkflows verifies workflow section format.
func TestClaudeRenderer_RenderCatalog_WithWorkflows(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{
				Name:        "sdd",
				Description: "Software Design Document workflow",
			},
			Components: model.WorkflowComponents{
				Commands: []model.WorkflowCommand{
					{Name: "sdd-explore", Action: "Explore and investigate", Argument: "<topic>"},
				},
			},
		},
	}

	if err := r.RenderCatalog(nil, nil, workflows, destPath); err != nil {
		t.Fatalf("RenderCatalog with workflows: %v", err)
	}

	content := string(mustReadFile(t, destPath))
	if !strings.Contains(content, "## Workflows") {
		t.Error("catalog missing Workflows section")
	}
	if !strings.Contains(content, "sdd") {
		t.Error("catalog missing workflow name")
	}
	// Verify 2-column format (no Argument column).
	if !strings.Contains(content, "| Command | Action |") {
		t.Errorf("catalog missing 2-column Command/Action header; content:\n%s", content)
	}
	if strings.Contains(content, "| Command | Action | Argument |") {
		t.Errorf("catalog should NOT have 3-column format; content:\n%s", content)
	}
	// Verify argument is merged into the command.
	if !strings.Contains(content, "`/sdd-explore <topic>`") {
		t.Errorf("catalog missing merged command+argument; content:\n%s", content)
	}
}

// --- T027: RenderCatalog enriched skills table and rules table ---

// TestClaudeRenderer_RenderCatalog_SkillsTableFormat verifies the skills table
// uses the canonical | Skill | Invocation | Use When | format and that the
// description populates the "Use When" column.
func TestClaudeRenderer_RenderCatalog_SkillsTableFormat(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	skills := []model.ContentItem{
		{
			Kind:        model.KindSkill,
			Name:        "unit-test-adviser",
			Path:        "skills/unit-test-adviser/",
			Description: "Domain unit test patterns and structure",
		},
	}

	if err := r.RenderCatalog(skills, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	content := string(mustReadFile(t, destPath))

	// Verify table header format.
	if !strings.Contains(content, "| Skill | Invocation | Use When |") {
		t.Errorf("catalog missing expected table header; content:\n%s", content)
	}
	if !strings.Contains(content, "|-------|------------|----------|") {
		t.Errorf("catalog missing expected table separator; content:\n%s", content)
	}
	// Verify description appears in "Use When" column.
	if !strings.Contains(content, "| `unit-test-adviser` | `/unit-test-adviser` | Domain unit test patterns and structure |") {
		t.Errorf("catalog missing expected skill row; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderCatalog_WithRules verifies the Project Rules table is rendered
// using the DisplayName field when available.
func TestClaudeRenderer_RenderCatalog_WithRules(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	rules := []model.ContentItem{
		{
			Kind:        model.KindRule,
			Name:        "architecture/clean-architecture-rules",
			Path:        "rules/architecture/clean-architecture/",
			Description: "Hexagonal architecture and DDD patterns",
			RuleMeta: &model.RuleMeta{
				Scope:       "architecture",
				Technology:  "any",
				AppliesTo:   "architect-adviser",
				Description: "Hexagonal architecture, DDD patterns, ports and adapters",
				DisplayName: "clean-architecture",
			},
		},
	}

	if err := r.RenderCatalog(nil, rules, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog with rules: %v", err)
	}

	content := string(mustReadFile(t, destPath))

	if !strings.Contains(content, "## Project Rules") {
		t.Error("catalog missing Project Rules section")
	}
	if !strings.Contains(content, "| Rule | Scope | Technology | Applies To | Description |") {
		t.Errorf("catalog missing Project Rules table header; content:\n%s", content)
	}
	// DisplayName should be used, not the path-based Name.
	if !strings.Contains(content, "`clean-architecture`") {
		t.Errorf("catalog should show DisplayName 'clean-architecture'; content:\n%s", content)
	}
	if strings.Contains(content, "`architecture/clean-architecture-rules`") {
		t.Errorf("catalog should NOT show path-based name when DisplayName is set; content:\n%s", content)
	}
	if !strings.Contains(content, "architecture") {
		t.Errorf("catalog missing rule scope; content:\n%s", content)
	}
	if !strings.Contains(content, "architect-adviser") {
		t.Errorf("catalog missing applies-to; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderCatalog_WithRules_FallbackToName verifies that when
// DisplayName is empty, the rule's ContentItem.Name is used as fallback.
func TestClaudeRenderer_RenderCatalog_WithRules_FallbackToName(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	rules := []model.ContentItem{
		{
			Kind: model.KindRule,
			Name: "my-custom-rule",
			Path: "rules/my-custom-rule/",
			RuleMeta: &model.RuleMeta{
				Scope:     "architecture",
				AppliesTo: "architect-adviser",
				// DisplayName intentionally empty — should fall back to Name.
			},
		},
	}

	if err := r.RenderCatalog(nil, rules, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog with rules: %v", err)
	}

	content := string(mustReadFile(t, destPath))

	if !strings.Contains(content, "`my-custom-rule`") {
		t.Errorf("catalog should fall back to ContentItem.Name when DisplayName is empty; content:\n%s", content)
	}
}

// TestClaudeRenderer_RenderCatalog_DecisionRulesFromWorkflows verifies that
// DecisionRules from workflow manifests appear in the catalog.
func TestClaudeRenderer_RenderCatalog_DecisionRulesFromWorkflows(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "CLAUDE.md")

	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{Name: "dev-workflow"},
			Components: model.WorkflowComponents{
				Commands: []model.WorkflowCommand{
					{Name: "dev-build", Action: "Build the project"},
				},
				DecisionRules: []model.DecisionRule{
					{Scenario: `"commit", "create commit"`, Resolution: "Use `git:commit`"},
					{Scenario: `"PR", "pull request"`, Resolution: "Use `git:pull-request`"},
				},
			},
		},
	}

	if err := r.RenderCatalog(nil, nil, workflows, destPath); err != nil {
		t.Fatalf("RenderCatalog with decision rules: %v", err)
	}

	content := string(mustReadFile(t, destPath))

	if !strings.Contains(content, "## Decision Rules") {
		t.Error("catalog missing Decision Rules section")
	}
	if !strings.Contains(content, "git:commit") {
		t.Errorf("catalog missing decision rule resolution; content:\n%s", content)
	}
	if !strings.Contains(content, "git:pull-request") {
		t.Errorf("catalog missing second decision rule; content:\n%s", content)
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

// TestClaudeRenderer_InstallWorkflow_ReplacesAdviserTablePlaceholder verifies that
// <!-- ADVISER_TABLE_PLACEHOLDER --> in a SKILL.md is replaced with the adviser skills table.
func TestClaudeRenderer_InstallWorkflow_ReplacesAdviserTablePlaceholder(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Set up installed skills containing an adviser-type skill.
	advisers := []model.ContentItem{
		{
			Kind:        model.KindSkill,
			Name:        "unit-test-adviser",
			Path:        "skills/unit-test-adviser/",
			Description: "Unit test patterns and structure",
		},
	}
	r.SetInstalledSkills(advisers)

	// Create a temporary workflow directory with a SKILL.md containing the placeholder.
	wfCacheDir := t.TempDir()

	skillDir := filepath.Join(wfCacheDir, "sdd-explore")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	skillContent := "---\nname: sdd-explore\ndescription: Explore and investigate\n---\n<!-- ADVISER_TABLE_PLACEHOLDER -->\n"
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
	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Verify the placeholder was replaced.
	destSkillMD := filepath.Join(workspaceDir, "skills", "sdd", "sdd-explore", "SKILL.md")
	data := mustReadFile(t, destSkillMD)
	content := string(data)

	if strings.Contains(content, "<!-- ADVISER_TABLE_PLACEHOLDER -->") {
		t.Errorf("placeholder was not replaced; SKILL.md content:\n%s", content)
	}
	if !strings.Contains(content, "unit-test-adviser") {
		t.Errorf("adviser table not injected; SKILL.md content:\n%s", content)
	}
}

// TestClaudeRenderer_InstallWorkflow_ReplacesSkillsPath verifies that {SKILLS_PATH}
// in an ORCHESTRATOR.md file is replaced with the actual workspace-relative path.
func TestClaudeRenderer_InstallWorkflow_ReplacesSkillsPath(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// Create a temporary workflow directory with an ORCHESTRATOR.md containing {SKILLS_PATH}.
	wfCacheDir := t.TempDir()

	orchestratorContent := "# Orchestrator\n\nSkills path: {SKILLS_PATH}\n"
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
	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Verify {SKILLS_PATH} was replaced in ORCHESTRATOR.md.
	destOrchestrator := filepath.Join(workspaceDir, "skills", "sdd", "ORCHESTRATOR.md")
	data := mustReadFile(t, destOrchestrator)
	content := string(data)

	if strings.Contains(content, "{SKILLS_PATH}") {
		t.Errorf("{SKILLS_PATH} was not replaced; ORCHESTRATOR.md content:\n%s", content)
	}
	// The replacement should contain the workflow name in the path.
	if !strings.Contains(content, "sdd") {
		t.Errorf("replaced skills path does not reference workflow name; content:\n%s", content)
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
				{Name: "sdd-explorer", Kind: "subagent", Model: "sonnet"},
				{Name: "sdd-planner", Kind: "subagent", Model: "opus"},
				{Name: "sdd-implementer", Kind: "subagent", Model: "sonnet"},
				{Name: "sdd-reviewer", Kind: "subagent", Model: "haiku"},
			},
		},
	}

	workspaceDir := t.TempDir()
	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceDir); err != nil {
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

	// REGISTRY.md uses {SKILLS_PATH}/ORCHESTRATOR.md — matching the real catalog template.
	registryContent := "Full orchestrator instructions: {SKILLS_PATH}/ORCHESTRATOR.md\n"
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
	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Render catalog so registry content is flushed.
	catalogPath := filepath.Join(workspaceDir, "CLAUDE.md")
	if err := r.RenderCatalog(nil, nil, []model.WorkflowManifest{wf}, catalogPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	data := mustReadFile(t, catalogPath)
	content := string(data)

	if strings.Contains(content, "//") {
		t.Errorf("double slash found in CLAUDE.md; got:\n%s", content)
	}
	if strings.Contains(content, "{SKILLS_PATH}") {
		t.Errorf("{SKILLS_PATH} was not resolved; got:\n%s", content)
	}
	// Path must resolve to skills/sdd/ORCHESTRATOR.md — no spurious sdd-orchestrator/ subdir.
	wantSuffix := "skills/sdd/ORCHESTRATOR.md"
	if !strings.Contains(content, wantSuffix) {
		t.Errorf("expected path containing %q; got:\n%s", wantSuffix, content)
	}
}

// TestClaudeRenderer_InstallWorkflow_NoAdviserSkills verifies that when no adviser
// skills are installed, the placeholder is removed (replaced with empty string).
func TestClaudeRenderer_InstallWorkflow_NoAdviserSkills(t *testing.T) {
	r := renderers.NewClaudeRenderer(claudeAgentDef())

	// No adviser skills set — installedSkills is empty.
	r.SetInstalledSkills([]model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/", Description: "Commit changes"},
	})

	wfCacheDir := t.TempDir()

	skillDir := filepath.Join(wfCacheDir, "sdd-explore")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	skillContent := "---\nname: sdd-explore\ndescription: Explore\n---\n<!-- ADVISER_TABLE_PLACEHOLDER -->\n"
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
	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceDir); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	destSkillMD := filepath.Join(workspaceDir, "skills", "sdd", "sdd-explore", "SKILL.md")
	data := mustReadFile(t, destSkillMD)
	content := string(data)

	// Placeholder should be gone.
	if strings.Contains(content, "<!-- ADVISER_TABLE_PLACEHOLDER -->") {
		t.Errorf("placeholder should be removed when no adviser skills installed; content:\n%s", content)
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

	// Now render catalog and verify the instructions appear.
	destPath := filepath.Join(workspaceDir, "CLAUDE.md")
	if err := r.RenderCatalog(nil, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	catalogContent := string(mustReadFile(t, destPath))
	if !strings.Contains(catalogContent, "Engram persistent memory") {
		t.Errorf("catalog should contain injected MCP instructions; content:\n%s", catalogContent)
	}
	if !strings.Contains(catalogContent, "## Engram") {
		t.Errorf("catalog should contain MCP section header; content:\n%s", catalogContent)
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
