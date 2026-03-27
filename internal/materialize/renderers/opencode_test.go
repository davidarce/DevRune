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

// openCodeAgentDef returns a default OpenCode agent definition for tests.
// Matches the real agents/opencode.yaml configuration (skillDir: "skills", no commandDir, no agentDir).
func openCodeAgentDef() model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   ".opencode",
		SkillDir:    "skills",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
}

func TestOpenCodeRenderer_Name(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	if r.Name() != "opencode" {
		t.Errorf("Name() = %q, want %q", r.Name(), "opencode")
	}
}

func TestOpenCodeRenderer_AgentType(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	if r.AgentType() != "opencode" {
		t.Errorf("AgentType() = %q, want %q", r.AgentType(), "opencode")
	}
}

func TestOpenCodeRenderer_NeedsCopyMode(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	if !r.NeedsCopyMode() {
		t.Error("NeedsCopyMode() = false, want true")
	}
}

// TestOpenCodeRenderer_RenderSkill_Full tests rendering a full canonical skill.
func TestOpenCodeRenderer_RenderSkill_Full(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	inputPath := goldenInputPath(t, "canonical-full.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	// OpenCode writes SKILL.md inside the destDir (same layout as Claude/Factory).
	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "opencode-full.md")
}

// TestOpenCodeRenderer_RenderSkill_Minimal tests rendering a minimal skill.
func TestOpenCodeRenderer_RenderSkill_Minimal(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	inputPath := goldenInputPath(t, "canonical-minimal.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "opencode-minimal.md")
}

// TestOpenCodeRenderer_ModelResolution verifies short model name → full ID.
func TestOpenCodeRenderer_ModelResolution(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := `---
name: test-skill
description: Test
model: sonnet
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)

	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if fm["model"] != "github-copilot/claude-sonnet-4.6" {
		t.Errorf("model = %v, want %q", fm["model"], "github-copilot/claude-sonnet-4.6")
	}
}

// TestOpenCodeRenderer_ModelResolution_Opus verifies opus short name resolution.
func TestOpenCodeRenderer_ModelResolution_Opus(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	input := "---\nname: opus-skill\ndescription: Test\nmodel: opus\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if fm["model"] != "github-copilot/claude-opus-4.6" {
		t.Errorf("model = %v, want %q", fm["model"], "github-copilot/claude-opus-4.6")
	}
}

// TestOpenCodeRenderer_ModelResolution_UnknownBareName verifies that an unknown bare model
// name (no provider prefix) gets the github-copilot/ prefix prepended.
func TestOpenCodeRenderer_ModelResolution_UnknownBareName(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	input := "---\nname: custom-skill\ndescription: Test\nmodel: gpt-4o\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	// Unknown bare names get the github-copilot/ prefix so OpenCode can route them.
	if fm["model"] != "github-copilot/gpt-4o" {
		t.Errorf("model = %v, want %q", fm["model"], "github-copilot/gpt-4o")
	}
}

// TestOpenCodeRenderer_ModelResolution_AlreadyQualifiedPassthrough verifies that a model ID
// that already contains a provider prefix ("/") is returned unchanged.
func TestOpenCodeRenderer_ModelResolution_AlreadyQualifiedPassthrough(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	input := "---\nname: custom-skill\ndescription: Test\nmodel: anthropic/claude-3-5-sonnet-20241022\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if fm["model"] != "anthropic/claude-3-5-sonnet-20241022" {
		t.Errorf("model = %v, want %q", fm["model"], "anthropic/claude-3-5-sonnet-20241022")
	}
}

// TestOpenCodeRenderer_ToolsConversionToBoolMap verifies tools list → bool map.
func TestOpenCodeRenderer_ToolsConversionToBoolMap(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := `---
name: tools-skill
description: Test
tools:
  - Bash(git:*)
  - Read
  - Edit
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	toolsVal, ok := fm["tools"]
	if !ok {
		t.Fatal("tools field should be present")
	}

	toolsMap, ok := toolsVal.(map[string]interface{})
	if !ok {
		t.Fatalf("tools should be map[string]interface{}, got %T", toolsVal)
	}

	expectedKeys := []string{"bash", "read", "edit"}
	for _, key := range expectedKeys {
		if val, ok := toolsMap[key]; !ok || val != true {
			t.Errorf("tools[%q] should be true; map: %v", key, toolsMap)
		}
	}
}

// TestOpenCodeRenderer_ModeInjection verifies mode: subagent is always injected.
func TestOpenCodeRenderer_ModeInjection(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	// Even without mode in input, it should be injected.
	input := "---\nname: no-mode\ndescription: Test\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if fm["mode"] != "subagent" {
		t.Errorf("mode = %v, want %q", fm["mode"], "subagent")
	}
}

// TestOpenCodeRenderer_ModeOverwrite verifies that mode is always set to "subagent"
// even if the input had a different mode value.
func TestOpenCodeRenderer_ModeOverwrite(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := "---\nname: with-mode\ndescription: Test\nmode: agent\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if fm["mode"] != "subagent" {
		t.Errorf("mode = %v, want %q", fm["mode"], "subagent")
	}
}

// TestOpenCodeRenderer_ColonToHyphenInName verifies name transformation.
func TestOpenCodeRenderer_ColonToHyphenInName(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := "---\nname: git:commit\ndescription: Test\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)

	// Output file should be SKILL.md inside destDir (OpenCode uses SKILL.md backing tree).
	skillPath := filepath.Join(destDir, "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("expected SKILL.md but got error: %v", err)
	}

	data, _ := os.ReadFile(skillPath)
	fm, _, _ := parse.ParseFrontmatter(data)
	if fm["name"] != "git-commit" {
		t.Errorf("name in frontmatter = %v, want %q", fm["name"], "git-commit")
	}
}

// TestOpenCodeRenderer_DropsAllowedTools verifies allowed-tools is dropped.
func TestOpenCodeRenderer_DropsAllowedTools(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := `---
name: drop-test
description: Test
allowed-tools:
  - Bash
  - Read
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if _, ok := fm["allowed-tools"]; ok {
		t.Error("allowed-tools should be dropped for OpenCode")
	}
}

// TestOpenCodeRenderer_DropsArgumentHint verifies argument-hint is dropped.
func TestOpenCodeRenderer_DropsArgumentHint(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())

	input := "---\nname: hint-skill\ndescription: Test\nargument-hint: \"<topic>\"\n---\nBody.\n"
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	_ = r.RenderSkill(srcDir, destDir)
	data, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	if _, ok := fm["argument-hint"]; ok {
		t.Error("argument-hint should be dropped for OpenCode")
	}
}

// TestOpenCodeRenderer_RenderCatalog verifies catalog generation.
func TestOpenCodeRenderer_RenderCatalog(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "AGENTS.md")

	skills := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git:commit", Path: "skills/git-commit/"},
	}

	if err := r.RenderCatalog(skills, nil, nil, destPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	content := string(mustReadFile(t, destPath))

	if !strings.Contains(content, "# OpenCode Agent Catalog") {
		t.Error("catalog missing heading")
	}
	// OpenCode applies colonToHyphen in catalog.
	if !strings.Contains(content, "`git-commit`") {
		t.Errorf("catalog should contain git-commit (hyphen), not git:commit; content:\n%s", content)
	}
}

// TestOpenCodeRenderer_RenderCommand verifies command rendering.
func TestOpenCodeRenderer_RenderCommand(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	destDir := t.TempDir()

	cmd := model.WorkflowCommand{
		Name:   "git:push",
		Action: "Push changes to remote",
	}

	if err := r.RenderCommand(cmd, destDir); err != nil {
		t.Fatalf("RenderCommand: %v", err)
	}

	// Output should be git-push.md.
	data, err := os.ReadFile(filepath.Join(destDir, "git-push.md"))
	if err != nil {
		t.Fatalf("command output not found: %v", err)
	}

	fm, _, _ := parse.ParseFrontmatter(data)
	if fm["name"] != "git-push" {
		t.Errorf("name = %v, want %q", fm["name"], "git-push")
	}
	if fm["mode"] != "subagent" {
		t.Errorf("mode = %v, want %q", fm["mode"], "subagent")
	}
}

// TestOpenCodeRenderer_Finalize_NoFile verifies Finalize is a no-op when opencode.json
// does not exist.
func TestOpenCodeRenderer_Finalize_NoFile(t *testing.T) {
	r := renderers.NewOpenCodeRenderer(openCodeAgentDef())
	if err := r.Finalize(t.TempDir()); err != nil {
		t.Errorf("Finalize on empty dir: unexpected error: %v", err)
	}
}

// --- helpers ---

// writeSkillFile creates a temp dir containing SKILL.md with the given content.
func writeSkillFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeSkillFile: %v", err)
	}
	return dir
}

// --- T015: OpenCode parity regression tests ---

// buildSddWorkflowCache creates a workflow cache dir for the SDD workflow
// used in parity regression tests. It writes:
//   - workflow.yaml
//   - sdd-plan/SKILL.md
//   - _shared/ (empty directory)
//   - ORCHESTRATOR.md (with the given content)
//
// Returns the cache dir path.
func buildSddWorkflowCache(t *testing.T, orchContent string) string {
	t.Helper()
	wfCacheDir := t.TempDir()

	// workflow.yaml
	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  skills:
    - sdd-plan
  entrypoint: ORCHESTRATOR.md
  roles:
    - name: sdd-planner
      kind: subagent
      skill: sdd-plan
      model: sonnet
    - name: sdd-reviewer
      kind: subagent
      skill: sdd-plan
      model: opus
    - name: sdd-orchestrator
      kind: orchestrator
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowCache: write workflow.yaml: %v", err)
	}

	// sdd-plan/SKILL.md
	sddPlanDir := filepath.Join(wfCacheDir, "sdd-plan")
	if err := os.MkdirAll(sddPlanDir, 0o755); err != nil {
		t.Fatalf("buildSddWorkflowCache: mkdir sdd-plan: %v", err)
	}
	skillContent := "---\nname: sdd-plan\ndescription: Plan the software design\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(sddPlanDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowCache: write sdd-plan/SKILL.md: %v", err)
	}

	// _shared/ directory
	sharedDir := filepath.Join(wfCacheDir, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("buildSddWorkflowCache: mkdir _shared: %v", err)
	}

	// ORCHESTRATOR.md
	if err := os.WriteFile(filepath.Join(wfCacheDir, "ORCHESTRATOR.md"), []byte(orchContent), 0o644); err != nil {
		t.Fatalf("buildSddWorkflowCache: write ORCHESTRATOR.md: %v", err)
	}

	return wfCacheDir
}

// sddParityManifest returns the SDD workflow manifest used in T015 parity tests.
func sddParityManifest() model.WorkflowManifest {
	return model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-planner", Kind: "subagent", Skill: "sdd-plan", Model: "sonnet"},
				{Name: "sdd-reviewer", Kind: "subagent", Skill: "sdd-plan", Model: "opus"},
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}
}

// TestOpenCodeRenderer_InstallWorkflow_SkillsUnderSkillsDir verifies that workflow
// skills are installed under {workspaceRoot}/skills/, _shared/ is also copied there,
// and the old buggy agents/ path is never created.
func TestOpenCodeRenderer_InstallWorkflow_SkillsUnderSkillsDir(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	wfCacheDir := buildSddWorkflowCache(t, "# SDD Orchestrator\n\nCoordinates SDD.\n")
	wf := sddParityManifest()

	result, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// POSITIVE: skill file installed under skills/
	skillMD := filepath.Join(workspaceRoot, "skills", "sdd-plan", "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		t.Errorf("expected %s to exist: %v", skillMD, err)
	}

	// POSITIVE: _shared/ directory installed under skills/
	sharedDir := filepath.Join(workspaceRoot, "skills", "_shared")
	info, err := os.Stat(sharedDir)
	if err != nil {
		t.Errorf("expected %s to exist: %v", sharedDir, err)
	} else if !info.IsDir() {
		t.Errorf("expected %s to be a directory", sharedDir)
	}

	// POSITIVE: opencode.json synthesized (from roles)
	opencodeJSON := filepath.Join(workspaceRoot, "opencode.json")
	if _, err := os.Stat(opencodeJSON); err != nil {
		t.Errorf("expected opencode.json to exist: %v", err)
	}

	// POSITIVE: ManagedPaths is non-empty
	if len(result.ManagedPaths) == 0 {
		t.Error("ManagedPaths should be non-empty after install")
	}

	// NEGATIVE: agents/ directory must NOT exist
	agentsDir := filepath.Join(workspaceRoot, "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		t.Errorf("agents/ directory should NOT exist, but it does: %s", agentsDir)
	}

	// NEGATIVE: REGISTRY.md must NOT be copied loose under skills/
	registryFile := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryFile); err == nil {
		t.Errorf("REGISTRY.md should NOT be copied loose to skills/, but it exists: %s", registryFile)
	}
}

// TestOpenCodeRenderer_InstallWorkflow_AgentEntriesInOpencodeJSON verifies that
// opencode.json contains an "agent" section with entries for every declared role,
// with correct mode values.
func TestOpenCodeRenderer_InstallWorkflow_AgentEntriesInOpencodeJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	orchContent := "# SDD Orchestrator\n\nCoordinates SDD.\n"
	wfCacheDir := buildSddWorkflowCache(t, orchContent)
	wf := sddParityManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workspaceRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}

	agentSection, ok := cfg["agent"].(map[string]interface{})
	if !ok {
		t.Fatalf("opencode.json missing 'agent' section; content:\n%s", string(data))
	}

	// sdd-planner: mode must be "subagent" and prompt must reference the role name.
	planner, ok := agentSection["sdd-planner"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent section missing 'sdd-planner'; keys: %v", mapKeys(agentSection))
	}
	if planner["mode"] != "subagent" {
		t.Errorf("sdd-planner mode = %v, want %q", planner["mode"], "subagent")
	}
	if prompt, _ := planner["prompt"].(string); !strings.Contains(prompt, "sdd-planner") {
		t.Errorf("sdd-planner prompt should contain 'sdd-planner'; got: %q", prompt)
	}

	// sdd-reviewer: must exist with mode "subagent".
	reviewer, ok := agentSection["sdd-reviewer"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent section missing 'sdd-reviewer'; keys: %v", mapKeys(agentSection))
	}
	if reviewer["mode"] != "subagent" {
		t.Errorf("sdd-reviewer mode = %v, want %q", reviewer["mode"], "subagent")
	}

	// sdd-orchestrator: mode must be "all".
	orch, ok := agentSection["sdd-orchestrator"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent section missing 'sdd-orchestrator'; keys: %v", mapKeys(agentSection))
	}
	if orch["mode"] != "all" {
		t.Errorf("sdd-orchestrator mode = %v, want %q", orch["mode"], "all")
	}
}

// TestOpenCodeRenderer_InstallWorkflow_ModelResolvedInAgentEntry verifies that
// a role with Model: "sonnet" resolves to the full anthropic model ID in opencode.json.
func TestOpenCodeRenderer_InstallWorkflow_ModelResolvedInAgentEntry(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	wfCacheDir := buildSddWorkflowCache(t, "# SDD Orchestrator\n\nCoordinates SDD.\n")
	wf := sddParityManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workspaceRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}

	agentSection := cfg["agent"].(map[string]interface{})
	planner := agentSection["sdd-planner"].(map[string]interface{})

	const wantModel = "github-copilot/claude-sonnet-4.6"
	if planner["model"] != wantModel {
		t.Errorf("sdd-planner model = %v, want %q", planner["model"], wantModel)
	}
}

// TestOpenCodeRenderer_InstallWorkflow_NoAgentsDirCreated verifies that the old
// buggy agents/ path is never created during workflow installation.
func TestOpenCodeRenderer_InstallWorkflow_NoAgentsDirCreated(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	wfCacheDir := buildSddWorkflowCache(t, "# SDD Orchestrator\n\nCoordinates SDD.\n")
	wf := sddParityManifest()

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	agentsDir := filepath.Join(workspaceRoot, "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		t.Errorf("agents/ directory should NOT exist after install, but it does: %s", agentsDir)
	}
}

// TestOpenCodeRenderer_InstallWorkflow_RegistryInjectedIntoCatalog verifies that
// a workflow Registry file is NOT copied loose. After the post-review fix, registry
// content is also NOT injected verbatim into the catalog — a minimal orchestrator
// pointer is emitted instead.
func TestOpenCodeRenderer_InstallWorkflow_RegistryInjectedIntoCatalog(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	// Build a cache dir with a REGISTRY.md file.
	wfCacheDir := t.TempDir()
	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  skills:
    - sdd-plan
  registry: REGISTRY.md
  commands:
    - name: sdd-explore
      action: Explore
`
	if err := os.WriteFile(filepath.Join(wfCacheDir, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}
	sddPlanDir := filepath.Join(wfCacheDir, "sdd-plan")
	if err := os.MkdirAll(sddPlanDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sddPlanDir, "SKILL.md"), []byte("---\nname: sdd-plan\ndescription: Plan\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}
	registryContent := "## SDD Skills\n\n- sdd-explore\n"
	if err := os.WriteFile(filepath.Join(wfCacheDir, "REGISTRY.md"), []byte(registryContent), 0o644); err != nil {
		t.Fatalf("write REGISTRY.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: model.WorkflowComponents{
			Skills:   []string{"sdd-plan"},
			Registry: "REGISTRY.md",
			Commands: []model.WorkflowCommand{{Name: "sdd-explore", Action: "Explore"}},
		},
	}

	if _, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Render catalog and verify registry content appears in it.
	catalogPath := filepath.Join(workspaceRoot, "AGENTS.md")
	workflows := []model.WorkflowManifest{wf}
	if err := r.RenderCatalog(nil, nil, workflows, catalogPath); err != nil {
		t.Fatalf("RenderCatalog: %v", err)
	}

	catalog := string(mustReadFile(t, catalogPath))
	// The workflow name ("sdd") must appear in the catalog as the workflow section heading.
	// Registry content is intentionally NOT injected verbatim — a minimal section is emitted instead.
	if !strings.Contains(catalog, "sdd") {
		t.Errorf("catalog should contain workflow name 'sdd'; content:\n%s", catalog)
	}

	// NEGATIVE: REGISTRY.md must NOT exist in the workspace.
	registryInWorkspace := filepath.Join(workspaceRoot, "skills", "REGISTRY.md")
	if _, err := os.Stat(registryInWorkspace); err == nil {
		t.Errorf("REGISTRY.md should NOT be copied to workspace, but it exists: %s", registryInWorkspace)
	}
}

// TestOpenCodeRenderer_InstallWorkflow_ManagedPathsNonEmpty verifies that
// WorkflowInstallResult.ManagedPaths is non-empty after a successful install.
func TestOpenCodeRenderer_InstallWorkflow_ManagedPathsNonEmpty(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CommandDir:  "commands",
		RulesDir:    "rules",
		CatalogFile: "AGENTS.md",
	}
	r := renderers.NewOpenCodeRenderer(def)

	wfCacheDir := buildSddWorkflowCache(t, "# SDD Orchestrator\n\nCoordinates SDD.\n")
	wf := sddParityManifest()

	result, err := r.InstallWorkflow(wf, wfCacheDir, workspaceRoot)
	if err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	if len(result.ManagedPaths) == 0 {
		t.Error("ManagedPaths should be non-empty after install with skills and roles")
	}
}

// TestOpenCodeRenderer_RenderMCPs_EnvVarFormat verifies that env var values in the rendered
// opencode.json use OpenCode format ({env:VAR_NAME}) instead of Claude format (${VAR_NAME}).
func TestOpenCodeRenderer_RenderMCPs_EnvVarFormat(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP: &model.MCPConfig{
			FilePath:    "opencode.json",
			RootKey:     "mcp",
			EnvKey:      "environment",
			EnvVarStyle: "{env:VAR}",
		},
	}
	r := renderers.NewOpenCodeRenderer(def)

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

	// Read the rendered opencode.json.
	content, err := os.ReadFile(filepath.Join(workspaceRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	mcpContent := string(content)

	// Must use OpenCode env var format.
	if !strings.Contains(mcpContent, "{env:EXA_API_KEY}") {
		t.Errorf("opencode.json should contain OpenCode env format {env:EXA_API_KEY}; content:\n%s", mcpContent)
	}
	// Must NOT contain the raw Claude format as the only occurrence.
	if strings.Contains(mcpContent, "${EXA_API_KEY}") {
		t.Errorf("opencode.json should not contain raw Claude format ${EXA_API_KEY}; content:\n%s", mcpContent)
	}

	// Root key must be "mcp" (OpenCode convention), not "mcpServers" (Claude default).
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}
	if _, ok := parsed["mcp"]; !ok {
		t.Errorf("opencode.json should have 'mcp' root key; got keys: %v", mapKeys(parsed))
	}
	if _, ok := parsed["mcpServers"]; ok {
		t.Errorf("opencode.json should NOT have 'mcpServers' root key (Claude default); content:\n%s", mcpContent)
	}

	// Env key must be "environment" (OpenCode convention).
	mcpSection, _ := parsed["mcp"].(map[string]interface{})
	exaEntry, _ := mcpSection["exa"].(map[string]interface{})
	if _, ok := exaEntry["environment"]; !ok {
		t.Errorf("exa server entry should have 'environment' env key; keys: %v", mapKeys(exaEntry))
	}
	if _, ok := exaEntry["env"]; ok {
		t.Errorf("exa server entry should NOT have 'env' key (should be 'environment'); content:\n%s", mcpContent)
	}
}

// TestOpenCodeRenderer_ManagedConfigPaths verifies that ManagedConfigPaths returns the
// config-driven opencode.json path (not a hardcoded path).
func TestOpenCodeRenderer_ManagedConfigPaths(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP: &model.MCPConfig{
			FilePath:    "opencode.json",
			RootKey:     "mcp",
			EnvKey:      "environment",
			EnvVarStyle: "{env:VAR}",
		},
	}
	r := renderers.NewOpenCodeRenderer(def)

	paths := r.ManagedConfigPaths(workspaceRoot)

	if len(paths) != 1 {
		t.Fatalf("ManagedConfigPaths() returned %d paths, want 1", len(paths))
	}

	wantPath := filepath.Join(workspaceRoot, "opencode.json")
	if paths[0] != wantPath {
		t.Errorf("ManagedConfigPaths()[0] = %q, want %q", paths[0], wantPath)
	}
}

// mapKeys returns the keys of a map[string]interface{} as a slice, for use in
// diagnostic error messages.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// transformMCPForOpenCode (tested via RenderMCPs)
// ---------------------------------------------------------------------------

// newOpenCodeRendererForMCPTest builds a minimal OpenCodeRenderer for MCP rendering tests.
func newOpenCodeRendererForMCPTest(workspaceRoot string) *renderers.OpenCodeRenderer {
	def := model.AgentDefinition{
		Name:        "opencode",
		Type:        "opencode",
		Workspace:   workspaceRoot,
		SkillDir:    "skills",
		CatalogFile: "AGENTS.md",
		MCP: &model.MCPConfig{
			FilePath:    "opencode.json",
			RootKey:     "mcp",
			EnvKey:      "environment",
			EnvVarStyle: "{env:VAR}",
		},
	}
	return renderers.NewOpenCodeRenderer(def)
}

// renderMCPsFromYAML is a helper that writes mcpYAML to a temp file, calls RenderMCPs,
// and returns the parsed mcp section of the resulting opencode.json.
func renderMCPsFromYAML(t *testing.T, workspaceRoot, mcpName, mcpYAML string) map[string]interface{} {
	t.Helper()
	cacheDir := t.TempDir()
	mcpFile := filepath.Join(cacheDir, mcpName+".yaml")
	if err := os.WriteFile(mcpFile, []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}

	cache := &fakeCacheStore{dirs: map[string]string{"hash1": cacheDir}}
	mcps := []model.LockedMCP{{Name: mcpName, Hash: "hash1", Dir: mcpName}}

	r := newOpenCodeRendererForMCPTest(workspaceRoot)
	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workspaceRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}
	mcpSection, _ := parsed["mcp"].(map[string]interface{})
	return mcpSection
}

// TestOpenCodeRenderer_RenderMCPs_LocalServer verifies that a catalog MCP with
// command+args is transformed to OpenCode "local" type with a merged command array.
func TestOpenCodeRenderer_RenderMCPs_LocalServer(t *testing.T) {
	workspaceRoot := t.TempDir()
	mcpSection := renderMCPsFromYAML(t, workspaceRoot, "atlassian", `name: atlassian
command: npx
args:
  - "-y"
  - "mcp-remote"
  - "https://mcp.atlassian.com/v1/sse"
`)

	entry, ok := mcpSection["atlassian"].(map[string]interface{})
	if !ok {
		t.Fatalf("atlassian entry missing or not a map; mcp section: %v", mcpSection)
	}

	// Must have type "local".
	if got := entry["type"]; got != "local" {
		t.Errorf("atlassian type = %q, want %q", got, "local")
	}

	// command must be a []interface{} (merged array).
	cmdArr, ok := entry["command"].([]interface{})
	if !ok {
		t.Fatalf("atlassian command should be an array, got %T: %v", entry["command"], entry["command"])
	}
	want := []interface{}{"npx", "-y", "mcp-remote", "https://mcp.atlassian.com/v1/sse"}
	if len(cmdArr) != len(want) {
		t.Errorf("atlassian command = %v, want %v", cmdArr, want)
	} else {
		for i, v := range want {
			if cmdArr[i] != v {
				t.Errorf("atlassian command[%d] = %q, want %q", i, cmdArr[i], v)
			}
		}
	}

	// "args" key must not exist — it was merged into command.
	if _, hasArgs := entry["args"]; hasArgs {
		t.Errorf("atlassian entry should not have 'args' key after transformation")
	}
}

// TestOpenCodeRenderer_RenderMCPs_RemoteServer verifies that a catalog MCP with
// type:"http" is transformed to type:"remote" for OpenCode.
func TestOpenCodeRenderer_RenderMCPs_RemoteServer(t *testing.T) {
	workspaceRoot := t.TempDir()
	mcpSection := renderMCPsFromYAML(t, workspaceRoot, "context7", `name: context7
type: http
url: https://mcp.context7.com/mcp
headers:
  CONTEXT7_API_KEY: "${CONTEXT7_API_KEY}"
`)

	entry, ok := mcpSection["context7"].(map[string]interface{})
	if !ok {
		t.Fatalf("context7 entry missing or not a map; mcp section: %v", mcpSection)
	}

	// type "http" must be converted to "remote".
	if got := entry["type"]; got != "remote" {
		t.Errorf("context7 type = %q, want %q", got, "remote")
	}

	// url must be preserved.
	if got := entry["url"]; got != "https://mcp.context7.com/mcp" {
		t.Errorf("context7 url = %q, want %q", got, "https://mcp.context7.com/mcp")
	}

	// Must not have command or args.
	if _, ok := entry["command"]; ok {
		t.Errorf("context7 entry should not have 'command' key")
	}
}

// TestOpenCodeRenderer_RenderMCPs_LocalServerWithEnv verifies that a local MCP with
// env vars gets command array transformation AND env key/format transformation.
func TestOpenCodeRenderer_RenderMCPs_LocalServerWithEnv(t *testing.T) {
	workspaceRoot := t.TempDir()
	mcpSection := renderMCPsFromYAML(t, workspaceRoot, "exa", `name: exa
command: npx
args:
  - "-y"
  - "exa-mcp-server"
env:
  EXA_API_KEY: "${EXA_API_KEY}"
`)

	entry, ok := mcpSection["exa"].(map[string]interface{})
	if !ok {
		t.Fatalf("exa entry missing or not a map; mcp section: %v", mcpSection)
	}

	// type must be "local".
	if got := entry["type"]; got != "local" {
		t.Errorf("exa type = %q, want %q", got, "local")
	}

	// command must be merged array.
	cmdArr, ok := entry["command"].([]interface{})
	if !ok {
		t.Fatalf("exa command should be an array, got %T: %v", entry["command"], entry["command"])
	}
	if len(cmdArr) < 2 || cmdArr[0] != "npx" {
		t.Errorf("exa command[0] = %v, want npx", cmdArr[0])
	}

	// env key must be "environment" and value must use OpenCode placeholder format.
	envMap, ok := entry["environment"].(map[string]interface{})
	if !ok {
		t.Fatalf("exa entry should have 'environment' key; keys: %v", mapKeys(entry))
	}
	if got := envMap["EXA_API_KEY"]; got != "{env:EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q", got, "{env:EXA_API_KEY}")
	}
	// Must not have "env" key.
	if _, ok := entry["env"]; ok {
		t.Errorf("exa entry should not have 'env' key after transformation")
	}
}
