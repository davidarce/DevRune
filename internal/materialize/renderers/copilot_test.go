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

	_ = r.RenderSkill(srcDir, "")

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
	_ = r.RenderSkill(srcDir, "")

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

	toolsList, ok := toolsVal.([]any)
	if !ok {
		t.Fatalf("tools should be []any, got %T", toolsVal)
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
	_ = r.RenderSkill(srcDir, "")

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
	_ = r.RenderSkill(srcDir, "")

	data, _ := os.ReadFile(filepath.Join(tmp, ".github", "agents", "dedup-test.agent.md"))
	fm, _, _ := parse.ParseFrontmatter(data)

	toolsVal := fm["tools"]
	toolsList, _ := toolsVal.([]any)

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
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
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

	// POSITIVE: _shared directory under skills/sdd-orchestrator/_shared
	// (matches the paths the orchestrator .agent.md references, e.g.
	// .github/skills/sdd-orchestrator/_shared/launch-templates.md)
	sharedDest := filepath.Join(workspaceRoot, "skills", "sdd-orchestrator", "_shared")
	if info, err := os.Stat(sharedDest); err != nil || !info.IsDir() {
		t.Errorf("expected %s to be a directory: err=%v", sharedDest, err)
	}

	// NEGATIVE: _shared must NOT be installed under agents/sdd-orchestrator/
	// (agents/ is flat: only .agent.md files, no subdirectories)
	sharedInAgents := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator", "_shared")
	if _, err := os.Stat(sharedInAgents); err == nil {
		t.Error("_shared must NOT exist under agents/sdd-orchestrator/ for Copilot — it belongs in skills/sdd-orchestrator/")
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
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
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
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Skills:   []string{"sdd-explore"},
			Registry: "REGISTRY.md",
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Verify registry content is captured (for later use by RenderRootCatalog).
	contents := r.RegistryContents()
	// Registry content is captured but not verbatim-injected (Copilot emits minimal pointer).
	// The workflow name "sdd" must exist as a key.
	if _, ok := contents[wf.Metadata.Name]; !ok {
		t.Errorf("RegistryContents should contain captured content for workflow 'sdd'; got keys: %v", func() []string {
			var keys []string
			for k := range contents {
				keys = append(keys, k)
			}
			return keys
		}())
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

// TestCopilotRenderer_ModelResolution verifies that model IDs in skill frontmatter
// are passed through unchanged — Copilot .agent.md requires bare IDs like "sonnet"
// or "claude-sonnet-4.6", NOT the "anthropic/..." format that resolveModel() produces.
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

	// Copilot uses identity resolution — model IDs pass through unchanged (no "anthropic/..." expansion).
	if fm["model"] != "sonnet" {
		t.Errorf("model = %q, want %q (identity passthrough for Copilot)", fm["model"], "sonnet")
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
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
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
	var parsed map[string]any
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

// mapCopilotKeys returns the keys of a map[string]any as a slice, for diagnostic messages.
func mapCopilotKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// --- T022: RenderSettings with MCP permissions ---

// copilotDefWithSettings returns a Copilot agent definition with Settings
// configured using an absolute workspace path, suitable for RenderSettings tests.
func copilotDefWithSettings(workspaceRoot string) model.AgentDefinition {
	return model.AgentDefinition{
		Name:        "copilot",
		Type:        "copilot",
		Workspace:   filepath.Join(workspaceRoot, ".github"),
		SkillDir:    "agents",
		RulesDir:    "rules",
		CatalogFile: "copilot-instructions.md",
		Settings:    &model.SettingsConfig{Permissions: []string{}},
	}
}

// TestCopilotRenderer_RenderSettings_MCPAllowPermission verifies that an MCP
// with permissions.level="allow" is added to the autoApprove array.
func TestCopilotRenderer_RenderSettings_MCPAllowPermission(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceRoot := filepath.Join(projectRoot, ".github")
	def := copilotDefWithSettings(projectRoot)
	r := renderers.NewCopilotRenderer(def)

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-atlassian-cp")
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
	cache := &fakeCacheStore{dirs: map[string]string{"hash-atlassian-cp": mcpDir}}
	mcps := []model.LockedMCP{{Name: "atlassian", Hash: "hash-atlassian-cp"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceRoot, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	// Settings must be written at project root (one level up from .github).
	settingsPath := filepath.Join(projectRoot, ".vscode", "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read .vscode/settings.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	const autoApproveKey = "github.copilot.chat.tools.autoApprove"
	raw, ok := parsed[autoApproveKey]
	if !ok {
		t.Fatalf("settings.json missing %q; content:\n%s", autoApproveKey, string(content))
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is not an array, got %T", autoApproveKey, raw)
	}
	found := false
	for _, v := range arr {
		if v == "mcp__atlassian__*" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("autoApprove must contain mcp__atlassian__*; got %v", arr)
	}
}

// TestCopilotRenderer_RenderSettings_MCPDenyNoEntry verifies that an MCP
// with permissions.level="deny" does NOT add any entry (Copilot only supports allow).
func TestCopilotRenderer_RenderSettings_MCPDenyNoEntry(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceRoot := filepath.Join(projectRoot, ".github")
	def := copilotDefWithSettings(projectRoot)
	r := renderers.NewCopilotRenderer(def)

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-deny-cp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: risky
command: node
args: ["server.js"]
permissions:
  level: deny
`
	if err := os.WriteFile(filepath.Join(mcpDir, "risky.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}
	cache := &fakeCacheStore{dirs: map[string]string{"hash-deny-cp": mcpDir}}
	mcps := []model.LockedMCP{{Name: "risky", Hash: "hash-deny-cp"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceRoot, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	// Settings must be written at project root (one level up from .github).
	settingsPath := filepath.Join(projectRoot, ".vscode", "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read .vscode/settings.json: %v", err)
	}

	if strings.Contains(string(content), "mcp__risky__*") {
		t.Errorf("autoApprove must NOT contain mcp__risky__* for deny-level MCP; content:\n%s", string(content))
	}
}

// TestCopilotRenderer_InstallWorkflow_OrchestratorVariant_UsedWhenPresent verifies that
// when ORCHESTRATOR.copilot.md is present in the cache, the installed
// .github/agents/sdd-orchestrator.agent.md contains the variant content (not the generic).
func TestCopilotRenderer_InstallWorkflow_OrchestratorVariant_UsedWhenPresent(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Write ORCHESTRATOR.md (generic content).
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# Generic Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}
	// Write ORCHESTRATOR.copilot.md (variant content).
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.copilot.md"), []byte("# Copilot Variant Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.copilot.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Read the generated .github/agents/sdd-orchestrator.agent.md.
	agentMDPath := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
	content, err := os.ReadFile(agentMDPath)
	if err != nil {
		t.Fatalf("read sdd-orchestrator.agent.md: %v", err)
	}
	contentStr := string(content)

	// Variant content must be present.
	if !strings.Contains(contentStr, "Copilot Variant") {
		t.Errorf("expected variant content in sdd-orchestrator.agent.md; got:\n%s", contentStr)
	}
	// Generic-only content must NOT be present.
	if strings.Contains(contentStr, "Generic") {
		t.Errorf("generic content must NOT appear in sdd-orchestrator.agent.md when variant is present; got:\n%s", contentStr)
	}
}

// TestCopilotRenderer_InstallWorkflow_OrchestratorVariant_FallsBackToGeneric verifies that
// when ORCHESTRATOR.copilot.md is absent, the installed .github/agents/sdd-orchestrator.agent.md
// contains the generic ORCHESTRATOR.md content.
func TestCopilotRenderer_InstallWorkflow_OrchestratorVariant_FallsBackToGeneric(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Write ONLY ORCHESTRATOR.md (generic content) — no variant file.
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# Generic Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Read the generated .github/agents/sdd-orchestrator.agent.md.
	agentMDPath := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
	content, err := os.ReadFile(agentMDPath)
	if err != nil {
		t.Fatalf("read sdd-orchestrator.agent.md: %v", err)
	}

	// Generic content must be present (fallback path used).
	if !strings.Contains(string(content), "Generic") {
		t.Errorf("expected generic content in sdd-orchestrator.agent.md (fallback); got:\n%s", string(content))
	}
}

// TestCopilotRenderer_InstallWorkflow_ForeignVariantNotCopied verifies that
// ORCHESTRATOR.opencode.md (a foreign variant) is never copied as a loose file
// into the Copilot workspace when both variant files are present in the cache.
// Only ORCHESTRATOR.copilot.md must be used (embedded in .agent.md);
// neither variant file should appear as a loose file on disk.
func TestCopilotRenderer_InstallWorkflow_ForeignVariantNotCopied(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Write generic + both variant files.
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# Generic\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.opencode.md"), []byte("# OpenCode variant\n"), 0o644); err != nil {
		t.Fatalf("write opencode variant: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.copilot.md"), []byte("# Copilot variant\n"), 0o644); err != nil {
		t.Fatalf("write copilot variant: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Walk the entire workspace root and assert neither variant appears as a loose file.
	forbidden := []string{"ORCHESTRATOR.opencode.md", "ORCHESTRATOR.copilot.md"}
	_ = filepath.WalkDir(workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		for _, f := range forbidden {
			if base == f {
				t.Errorf("forbidden file found as loose file: %s", path)
			}
		}
		return nil
	})
}

// TestCopilotRenderer_RenderSettings_ExistingContentPreserved verifies that
// existing .vscode/settings.json content is preserved when merging.
func TestCopilotRenderer_RenderSettings_ExistingContentPreserved(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceRoot := filepath.Join(projectRoot, ".github")
	def := copilotDefWithSettings(projectRoot)
	r := renderers.NewCopilotRenderer(def)

	// Pre-write a .vscode/settings.json at the project root (one level up from .github).
	vscodeDir := filepath.Join(projectRoot, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		t.Fatalf("mkdir .vscode: %v", err)
	}
	existing := `{"editor.tabSize": 2, "editor.formatOnSave": true}` + "\n"
	if err := os.WriteFile(filepath.Join(vscodeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing settings.json: %v", err)
	}

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash-ctx7-cp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: context7
command: npx
args: ["-y", "context7"]
permissions:
  level: allow
`
	if err := os.WriteFile(filepath.Join(mcpDir, "context7.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}
	cache := &fakeCacheStore{dirs: map[string]string{"hash-ctx7-cp": mcpDir}}
	mcps := []model.LockedMCP{{Name: "context7", Hash: "hash-ctx7-cp"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	if err := r.RenderSettings(workspaceRoot, nil, nil); err != nil {
		t.Fatalf("RenderSettings: %v", err)
	}

	// Settings are at the project root, not inside .github.
	content, err := os.ReadFile(filepath.Join(projectRoot, ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	contentStr := string(content)

	// Existing settings must be preserved.
	if !strings.Contains(contentStr, `"editor.tabSize"`) {
		t.Errorf("existing editor.tabSize setting must be preserved; content:\n%s", contentStr)
	}
	// New MCP entry must be present.
	if !strings.Contains(contentStr, "mcp__context7__*") {
		t.Errorf("settings.json must contain mcp__context7__*; content:\n%s", contentStr)
	}
}

// ---------------------------------------------------------------------------
// T011–T016: transformBodyForCopilot unit tests
// ---------------------------------------------------------------------------

// TestTransformBodyForCopilot_MCPNormalization verifies that Claude Code MCP shorthand
// calls are replaced with mcp__engram__* equivalents, with no false positives.
func TestTransformBodyForCopilot_MCPNormalization(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    string
		wantNotContains string
	}{
		{
			name:            "mem_save",
			input:           "call mem_save(title: \"foo\")",
			wantContains:    "mcp__engram__mem_save(title: \"foo\")",
			wantNotContains: "",
		},
		{
			name:            "mem_search",
			input:           "use mem_search(query: \"bar\")",
			wantContains:    "mcp__engram__mem_search(query: \"bar\")",
			wantNotContains: "",
		},
		{
			name:            "mem_context",
			input:           "run mem_context()",
			wantContains:    "mcp__engram__mem_context()",
			wantNotContains: "",
		},
		{
			name:            "mem_session_summary",
			input:           "call mem_session_summary(content: \"x\")",
			wantContains:    "mcp__engram__mem_session_summary(content: \"x\")",
			wantNotContains: "",
		},
		{
			name:            "mem_get_observation",
			input:           "mem_get_observation(id: 42)",
			wantContains:    "mcp__engram__mem_get_observation(id: 42)",
			wantNotContains: "",
		},
		{
			name:            "mem_timeline",
			input:           "mem_timeline(observation_id: 1)",
			wantContains:    "mcp__engram__mem_timeline(observation_id: 1)",
			wantNotContains: "",
		},
		{
			name:            "mem_save_prompt",
			input:           "mem_save_prompt(content: \"prompt\")",
			wantContains:    "mcp__engram__mem_save_prompt(content: \"prompt\")",
			wantNotContains: "",
		},
		{
			name:            "mem_stats",
			input:           "mem_stats()",
			wantContains:    "mcp__engram__mem_stats()",
			wantNotContains: "",
		},
		{
			name:            "mem_update",
			input:           "mem_update(id: 1, title: \"new\")",
			wantContains:    "mcp__engram__mem_update(id: 1, title: \"new\")",
			wantNotContains: "",
		},
		{
			name:            "mem_delete",
			input:           "mem_delete(id: 5)",
			wantContains:    "mcp__engram__mem_delete(id: 5)",
			wantNotContains: "",
		},
		{
			name:            "mem_suggest_topic_key",
			input:           "mem_suggest_topic_key(title: \"x\")",
			wantContains:    "mcp__engram__mem_suggest_topic_key(title: \"x\")",
			wantNotContains: "",
		},
		{
			name:            "mem_capture_passive",
			input:           "mem_capture_passive(content: \"y\")",
			wantContains:    "mcp__engram__mem_capture_passive(content: \"y\")",
			wantNotContains: "",
		},
		{
			name:            "mem_session_start",
			input:           "mem_session_start(id: \"s1\")",
			wantContains:    "mcp__engram__mem_session_start(id: \"s1\")",
			wantNotContains: "",
		},
		{
			name:            "mem_session_end",
			input:           "mem_session_end(id: \"s1\")",
			wantContains:    "mcp__engram__mem_session_end(id: \"s1\")",
			wantNotContains: "",
		},
		{
			name:            "no false positive on prose",
			input:           "remember to save your work",
			wantContains:    "remember to save your work",
			wantNotContains: "mcp__engram__",
		},
		{
			name:            "already prefixed passes through idempotently",
			input:           "mcp__engram__mem_save(title: \"x\")",
			wantContains:    "mcp__engram__mem_save(title: \"x\")",
			wantNotContains: "mcp__engram__mcp__engram__",
		},
		{
			name:            "mem_save_prompt not matched by mem_save key",
			input:           "mem_save_prompt(content: \"p\")",
			wantContains:    "mcp__engram__mem_save_prompt(content: \"p\")",
			wantNotContains: "mcp__engram__mem_save(",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderers.TransformBodyForCopilot(tt.input)
			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("output missing %q;\noutput: %q", tt.wantContains, got)
			}
			if tt.wantNotContains != "" && strings.Contains(got, tt.wantNotContains) {
				t.Errorf("output must NOT contain %q;\noutput: %q", tt.wantNotContains, got)
			}
		})
	}
}

// TestTransformBodyForCopilot_ShebangConversion verifies that Dynamic Context
// shebang lines (- **Label**: !`cmd`) are converted to a Pre-flight Commands section.
func TestTransformBodyForCopilot_ShebangConversion(t *testing.T) {
	t.Run("single shebang", func(t *testing.T) {
		input := "# My Skill\n\n- **Status**: !`git status --short`\n\nOther content.\n"
		got := renderers.TransformBodyForCopilot(input)
		if !strings.Contains(got, "Pre-flight Commands") {
			t.Errorf("expected 'Pre-flight Commands' section; output:\n%s", got)
		}
		if !strings.Contains(got, "git status --short") {
			t.Errorf("expected command in output; output:\n%s", got)
		}
		if strings.Contains(got, "!`git status") {
			t.Errorf("shebang syntax must be removed from output; output:\n%s", got)
		}
	})

	t.Run("multiple shebangs grouped", func(t *testing.T) {
		input := "# Skill\n\n- **Diff**: !`git diff HEAD`\n- **Status**: !`git status`\n\nContent.\n"
		got := renderers.TransformBodyForCopilot(input)
		if !strings.Contains(got, "Pre-flight Commands") {
			t.Errorf("expected 'Pre-flight Commands' section; output:\n%s", got)
		}
		if !strings.Contains(got, "git diff HEAD") {
			t.Errorf("expected first command in output; output:\n%s", got)
		}
		if !strings.Contains(got, "git status") {
			t.Errorf("expected second command in output; output:\n%s", got)
		}
	})

	t.Run("no shebang passes through unchanged", func(t *testing.T) {
		input := "# Skill\n\nJust normal markdown.\n"
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "Pre-flight") {
			t.Errorf("no shebang — output must not contain 'Pre-flight'; output:\n%s", got)
		}
		if got != input {
			t.Errorf("no shebang — output should be identical to input;\nwant: %q\ngot:  %q", input, got)
		}
	})

	t.Run("exclamation in prose not treated as shebang", func(t *testing.T) {
		input := "# Skill\n\nThis is important! Do not skip it.\n"
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "Pre-flight") {
			t.Errorf("prose exclamation must not trigger shebang conversion; output:\n%s", got)
		}
	})
}

// TestTransformBodyForCopilot_SkillTaskStripping verifies that Skill() references
// are replaced with @agent-name prose and Task() blocks are stripped.
func TestTransformBodyForCopilot_SkillTaskStripping(t *testing.T) {
	t.Run("inline Skill double-quoted name", func(t *testing.T) {
		input := `Call Skill("sdd-explore") to start.`
		got := renderers.TransformBodyForCopilot(input)
		if !strings.Contains(got, "@sdd-explore agent") {
			t.Errorf("expected '@sdd-explore agent'; output: %q", got)
		}
		if strings.Contains(got, "Skill(") {
			t.Errorf("Skill() must be removed; output: %q", got)
		}
	})

	t.Run("Skill with skill: keyword", func(t *testing.T) {
		input := `Use Skill(skill: "sdd-plan") here.`
		got := renderers.TransformBodyForCopilot(input)
		if !strings.Contains(got, "@sdd-plan agent") {
			t.Errorf("expected '@sdd-plan agent'; output: %q", got)
		}
	})

	t.Run("git-commit skill", func(t *testing.T) {
		input := `invoke Skill("git-commit") when done`
		got := renderers.TransformBodyForCopilot(input)
		if !strings.Contains(got, "@git-commit agent") {
			t.Errorf("expected '@git-commit agent'; output: %q", got)
		}
	})

	t.Run("multi-line Task block stripped", func(t *testing.T) {
		input := "Before task.\nTask(\n  description: \"explore\",\n  prompt: \"Do it\"\n)\nAfter task.\n"
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "Task(") {
			t.Errorf("Task( block must be removed; output: %q", got)
		}
		if !strings.Contains(got, "Invoke the appropriate sub-agent") {
			t.Errorf("expected replacement prose; output: %q", got)
		}
		if !strings.Contains(got, "After task.") {
			t.Errorf("content after task block must be preserved; output: %q", got)
		}
	})

	t.Run("prose word Skill not transformed", func(t *testing.T) {
		// English word "skill" in prose (capitalized but not a function call) should not be transformed.
		input := "The Skill is important for development."
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "@") {
			t.Errorf("prose 'Skill' must not be transformed; output: %q", got)
		}
	})
}

// TestTransformBodyForCopilot_AskUserQuestion verifies that AskUserQuestion references
// are replaced with Copilot-native prose equivalents.
func TestTransformBodyForCopilot_AskUserQuestion(t *testing.T) {
	t.Run("backtick-wrapped replaced", func(t *testing.T) {
		input := "Use `AskUserQuestion` to present choices."
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "AskUserQuestion") {
			t.Errorf("`AskUserQuestion` must be removed; output: %q", got)
		}
		if !strings.Contains(got, "ask the user directly") {
			t.Errorf("expected 'ask the user directly' in output; got: %q", got)
		}
	})

	t.Run("bare replaced", func(t *testing.T) {
		input := "Call AskUserQuestion with the options."
		got := renderers.TransformBodyForCopilot(input)
		if strings.Contains(got, "AskUserQuestion") {
			t.Errorf("bare AskUserQuestion must be removed; output: %q", got)
		}
		if !strings.Contains(got, "ask the user directly") {
			t.Errorf("expected replacement prose; output: %q", got)
		}
	})

	t.Run("no false positive on unrelated text", func(t *testing.T) {
		input := "Ask the user what they want."
		got := renderers.TransformBodyForCopilot(input)
		if got != input {
			t.Errorf("unrelated prose must not be modified; got: %q", got)
		}
	})
}

// TestTransformBodyForCopilot_ToolNameReplacement verifies that backtick-wrapped Claude Code
// tool names are replaced with Copilot-friendly prose equivalents.
func TestTransformBodyForCopilot_ToolNameReplacement(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   string
		wantNotContain string
	}{
		{"Write replaced", "Use `Write` to create the file.", "the file write tool", "`Write`"},
		{"Edit replaced", "Call `Edit` to modify it.", "the file edit tool", "`Edit`"},
		{"Read replaced", "Use `Read` to inspect the file.", "the file read tool", "`Read`"},
		{"Glob replaced", "Run `Glob` to find files.", "the file search tool", "`Glob`"},
		{"Grep replaced", "Use `Grep` for searching.", "the content search tool", "`Grep`"},
		{"Bash replaced", "Run `Bash` commands.", "the terminal tool", "`Bash`"},
		{"Bash with args replaced", "Use `Bash(git:*)` for git.", "the terminal tool", "`Bash(git:*)`"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderers.TransformBodyForCopilot(tc.input)
			if tc.wantNotContain != "" && strings.Contains(got, tc.wantNotContain) {
				t.Errorf("output must not contain %q; got: %q", tc.wantNotContain, got)
			}
			if !strings.Contains(got, tc.wantContains) {
				t.Errorf("expected %q in output; got: %q", tc.wantContains, got)
			}
		})
	}

	t.Run("prose Write not transformed", func(t *testing.T) {
		input := "Write the implementation in a clean way."
		got := renderers.TransformBodyForCopilot(input)
		if got != input {
			t.Errorf("prose 'Write' (no backticks) must not be transformed; got: %q", got)
		}
	})

	t.Run("prose Read not transformed", func(t *testing.T) {
		input := "Read the documentation carefully."
		got := renderers.TransformBodyForCopilot(input)
		if got != input {
			t.Errorf("prose 'Read' (no backticks) must not be transformed; got: %q", got)
		}
	})
}

// TestTransformBodyForCopilot_Integration verifies that a realistic SKILL.md body
// containing all five pattern types (shebangs, MCP calls, AskUserQuestion, Skill/Task
// blocks, tool names) produces coherent Copilot output with no Claude Code artifacts.
func TestTransformBodyForCopilot_Integration(t *testing.T) {
	input := `# SDD Review Skill

- **Git Status**: ` + "!`git status --short`" + `
- **Branch**: ` + "!`git branch --show-current`" + `

## Instructions

Call mem_save(title: "review", content: "done") when finished.
Use mem_search(query: "plan") to find related observations.

If blocked, use ` + "`AskUserQuestion`" + ` to clarify requirements.

Start by calling Skill("sdd-explore") to discover the codebase.
Then call Skill(skill: "sdd-plan") to plan the implementation.

Use ` + "`Read`" + ` to inspect files and ` + "`Edit`" + ` to make changes.
Run ` + "`Bash`" + ` to execute commands.

Task(
  description: "explore",
  prompt: "Do the work"
)

After completion, call mem_session_summary(content: "done").
`

	got := renderers.TransformBodyForCopilot(input)

	// No Claude Code MCP shorthands without mcp__engram__ prefix.
	if strings.Contains(got, "mem_save(") && !strings.Contains(got, "mcp__engram__mem_save(") {
		t.Errorf("bare mem_save( must not appear without mcp__engram__ prefix; output:\n%s", got)
	}
	if strings.Contains(got, "mem_search(") && !strings.Contains(got, "mcp__engram__mem_search(") {
		t.Errorf("bare mem_search( must not appear without mcp__engram__ prefix; output:\n%s", got)
	}
	if strings.Contains(got, "mem_session_summary(") && !strings.Contains(got, "mcp__engram__mem_session_summary(") {
		t.Errorf("bare mem_session_summary( must not appear; output:\n%s", got)
	}

	// No AskUserQuestion.
	if strings.Contains(got, "AskUserQuestion") {
		t.Errorf("AskUserQuestion must be replaced; output:\n%s", got)
	}

	// No Skill() call syntax.
	if strings.Contains(got, "Skill(") {
		t.Errorf("Skill() must be replaced; output:\n%s", got)
	}

	// No Task() block.
	if strings.Contains(got, "Task(") {
		t.Errorf("Task() block must be replaced; output:\n%s", got)
	}

	// Shebang lines converted to Pre-flight Commands section.
	if !strings.Contains(got, "Pre-flight Commands") {
		t.Errorf("expected Pre-flight Commands section; output:\n%s", got)
	}
	if strings.Contains(got, "!`git") {
		t.Errorf("shebang !` must be replaced; output:\n%s", got)
	}

	// No backtick-wrapped tool names.
	if strings.Contains(got, "`Read`") || strings.Contains(got, "`Edit`") || strings.Contains(got, "`Bash`") {
		t.Errorf("backtick-wrapped tool names must be replaced; output:\n%s", got)
	}

	// Agent references in output.
	if !strings.Contains(got, "@sdd-explore") {
		t.Errorf("expected @sdd-explore agent reference; output:\n%s", got)
	}
	if !strings.Contains(got, "@sdd-plan") {
		t.Errorf("expected @sdd-plan agent reference; output:\n%s", got)
	}
}

// TestCopilotRenderer_SubAgentToolSets verifies that the four SDD sub-agent roles
// receive the correct tool sets in their generated .agent.md files.
//
// Expected tool sets (from copilotSubAgentTools):
//   - sdd-explore:   ["read", "search", "edit", "execute"]
//   - sdd-plan:      ["read", "search", "edit", "execute"]
//   - sdd-implement: ["read", "search", "edit", "execute"]
//   - sdd-review:    ["read", "search", "execute"]
func TestCopilotRenderer_SubAgentToolSets(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Create SKILL.md for each sub-agent role.
	subAgentSkills := []string{"sdd-explore", "sdd-plan", "sdd-implement", "sdd-review"}
	for _, skill := range subAgentSkills {
		skillDir := filepath.Join(cachePath, skill)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", skill, err)
		}
		content := "---\nname: " + skill + "\ndescription: " + skill + " sub-agent\n---\nBody content for " + skill + ".\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s/SKILL.md: %v", skill, err)
		}
	}

	// Write ORCHESTRATOR.md.
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# SDD Orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
				{Name: "sdd-explorer", Kind: "subagent", Skill: "sdd-explore"},
				{Name: "sdd-planner", Kind: "subagent", Skill: "sdd-plan"},
				{Name: "sdd-implementer", Kind: "subagent", Skill: "sdd-implement"},
				{Name: "sdd-reviewer", Kind: "subagent", Skill: "sdd-review"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	// Helper: read tools from a generated .agent.md frontmatter.
	readTools := func(agentName string) map[string]bool {
		t.Helper()
		agentPath := filepath.Join(workspaceRoot, "agents", agentName+".agent.md")
		data, err := os.ReadFile(agentPath)
		if err != nil {
			t.Fatalf("read %s.agent.md: %v", agentName, err)
		}
		fm, _, err := parse.ParseFrontmatter(data)
		if err != nil {
			t.Fatalf("parse frontmatter for %s.agent.md: %v", agentName, err)
		}
		toolsVal, ok := fm["tools"]
		if !ok {
			t.Fatalf("%s.agent.md: 'tools' key missing from frontmatter", agentName)
		}
		toolsList, ok := toolsVal.([]any)
		if !ok {
			t.Fatalf("%s.agent.md: 'tools' is %T, want []any", agentName, toolsVal)
		}
		result := make(map[string]bool)
		for _, v := range toolsList {
			if s, ok := v.(string); ok {
				result[s] = true
			}
		}
		return result
	}

	// sdd-explorer: expects read, search, edit, execute
	explorerTools := readTools("sdd-explorer")
	for _, want := range []string{"read", "search", "edit", "execute"} {
		if !explorerTools[want] {
			t.Errorf("sdd-explorer: expected tool %q; got tools: %v", want, explorerTools)
		}
	}

	// sdd-planner: expects read, search, edit, execute
	plannerTools := readTools("sdd-planner")
	for _, want := range []string{"read", "search", "edit", "execute"} {
		if !plannerTools[want] {
			t.Errorf("sdd-planner: expected tool %q; got tools: %v", want, plannerTools)
		}
	}

	// sdd-implementer: expects read, search, edit, execute
	implementerTools := readTools("sdd-implementer")
	for _, want := range []string{"read", "search", "edit", "execute"} {
		if !implementerTools[want] {
			t.Errorf("sdd-implementer: expected tool %q; got tools: %v", want, implementerTools)
		}
	}
	if implementerTools["search"] && implementerTools["edit"] && !implementerTools["read"] {
		t.Error("sdd-implementer: expected 'read' tool to be present")
	}

	// sdd-reviewer: expects read, search, execute — NOT edit
	reviewerTools := readTools("sdd-reviewer")
	for _, want := range []string{"read", "search", "execute"} {
		if !reviewerTools[want] {
			t.Errorf("sdd-reviewer: expected tool %q; got tools: %v", want, reviewerTools)
		}
	}
	if reviewerTools["edit"] {
		t.Errorf("sdd-reviewer: must NOT have 'edit' tool; got tools: %v", reviewerTools)
	}
}

// TestCopilotRenderer_OrchestratorBodyTransform_MCPNormalized verifies that when
// ORCHESTRATOR.copilot.md contains mem_save() calls, the installed .agent.md
// has them normalized to mcp__engram__mem_save() format.
func TestCopilotRenderer_OrchestratorBodyTransform_MCPNormalized(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// ORCHESTRATOR.copilot.md with mem_save( and AskUserQuestion references.
	copilotVariantContent := `# Copilot Orchestrator

Call mem_save(title: "state", content: "active") after each phase.
Use mem_search(query: "sdd") to find prior context.
When blocked, use ` + "`AskUserQuestion`" + ` to clarify.
`
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.md"), []byte("# Generic\n"), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, "ORCHESTRATOR.copilot.md"), []byte(copilotVariantContent), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.copilot.md: %v", err)
	}

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}

	if _, err := r.InstallWorkflow(wf, cachePath, workspaceRoot); err != nil {
		t.Fatalf("InstallWorkflow: %v", err)
	}

	agentMDPath := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
	content, err := os.ReadFile(agentMDPath)
	if err != nil {
		t.Fatalf("read sdd-orchestrator.agent.md: %v", err)
	}
	contentStr := string(content)

	// mem_save( must be normalized.
	if strings.Contains(contentStr, "mem_save(") && !strings.Contains(contentStr, "mcp__engram__mem_save(") {
		t.Errorf("mem_save( must be normalized to mcp__engram__mem_save(; got:\n%s", contentStr)
	}
	// mem_search( must be normalized.
	if strings.Contains(contentStr, "mem_search(") && !strings.Contains(contentStr, "mcp__engram__mem_search(") {
		t.Errorf("mem_search( must be normalized to mcp__engram__mem_search(; got:\n%s", contentStr)
	}
	// AskUserQuestion must be replaced.
	if strings.Contains(contentStr, "AskUserQuestion") {
		t.Errorf("AskUserQuestion must be replaced in orchestrator body; got:\n%s", contentStr)
	}
	// Variant content must still be present (not the generic content).
	if !strings.Contains(contentStr, "Copilot Orchestrator") {
		t.Errorf("copilot variant content must be present; got:\n%s", contentStr)
	}
}

// ---------------------------------------------------------------------------
// T011: CopilotRenderer.RegenerateAdvisorFiles unit tests
// ---------------------------------------------------------------------------

// copilotAdvisorDef returns an AgentDefinition for advisor file generation tests.
// Copilot writes agent files to {workspaceRoot}/agents/{name}.agent.md.
func copilotAdvisorDef(workspaceRoot string) model.AgentDefinition {
	return model.AgentDefinition{
		Name:      "copilot",
		Type:      "copilot",
		Workspace: workspaceRoot,
		SkillDir:  "skills",
		AgentDir:  "agents",
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_InstallTwo verifies that installing
// two advisors creates two .agent.md files with valid frontmatter.
func TestCopilotRenderer_RegenerateAdvisorFiles_InstallTwo(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Architecture advice"},
		{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit test advice"},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 2 {
		t.Errorf("Written = %d paths, want 2; paths: %v", len(result.Written), result.Written)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %d paths, want 0; paths: %v", len(result.Deleted), result.Deleted)
	}

	agentsDir := filepath.Join(workspaceRoot, "agents")
	for _, name := range []string{"architect-advisor", "unit-test-advisor"} {
		agentPath := filepath.Join(agentsDir, name+".agent.md")
		data, err := os.ReadFile(agentPath)
		if err != nil {
			t.Fatalf("read %s.agent.md: %v", name, err)
		}
		fm, _, err := parse.ParseFrontmatter(data)
		if err != nil {
			t.Fatalf("parse frontmatter of %s.agent.md: %v", name, err)
		}
		if fm["name"] != name {
			t.Errorf("%s.agent.md: frontmatter name = %v, want %q", name, fm["name"], name)
		}
		if _, ok := fm["description"]; !ok {
			t.Errorf("%s.agent.md: missing 'description' in frontmatter", name)
		}
		if _, ok := fm["tools"]; !ok {
			t.Errorf("%s.agent.md: missing 'tools' in frontmatter", name)
		}
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_RemoveOne verifies that removing one
// advisor deletes its .agent.md while the other file remains untouched.
func TestCopilotRenderer_RegenerateAdvisorFiles_RemoveOne(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	// Install both advisors first.
	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Architecture advice"},
		{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit test advice"},
	}
	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("first RegenerateAdvisorFiles: %v", err)
	}

	// Remove architect-advisor, keep unit-test-advisor.
	result, err := r.RegenerateAdvisorFiles(workspaceRoot,
		[]model.ContentItem{{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit test advice"}},
		[]string{"architect-advisor"},
		nil,
	)
	if err != nil {
		t.Fatalf("second RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Deleted) != 1 {
		t.Errorf("Deleted = %d paths, want 1; paths: %v", len(result.Deleted), result.Deleted)
	}

	agentsDir := filepath.Join(workspaceRoot, "agents")

	// architect-advisor.agent.md must be gone.
	removedPath := filepath.Join(agentsDir, "architect-advisor.agent.md")
	if _, err := os.Stat(removedPath); err == nil {
		t.Errorf("architect-advisor.agent.md should have been deleted but still exists")
	}

	// unit-test-advisor.agent.md must still exist.
	keptPath := filepath.Join(agentsDir, "unit-test-advisor.agent.md")
	if _, err := os.Stat(keptPath); err != nil {
		t.Errorf("unit-test-advisor.agent.md should still exist: %v", err)
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_RemoveNonExistent verifies that
// removing a name that has no corresponding file produces no error.
func TestCopilotRenderer_RegenerateAdvisorFiles_RemoveNonExistent(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, nil, []string{"nonexistent-advisor"}, nil)
	if err != nil {
		t.Errorf("RegenerateAdvisorFiles with nonexistent name: expected no error, got: %v", err)
	}
	// Deleted still lists the path (Copilot removes what was there; non-existent is a no-op).
	_ = result
}

// TestCopilotRenderer_RegenerateAdvisorFiles_NonAdvisorNamesIgnored verifies that
// items without "-advisor" or "-adviser" suffix and Custom=false are silently ignored.
func TestCopilotRenderer_RegenerateAdvisorFiles_NonAdvisorNamesIgnored(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/", Description: "Git commits"},
		{Kind: model.KindSkill, Name: "sdd-plan", Path: "skills/sdd-plan/", Description: "Plan phase"},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 0 {
		t.Errorf("Written = %d paths, want 0 (non-advisor names must be ignored); paths: %v", len(result.Written), result.Written)
	}

	agentsDir := filepath.Join(workspaceRoot, "agents")
	for _, name := range []string{"git-commit", "sdd-plan"} {
		agentPath := filepath.Join(agentsDir, name+".agent.md")
		if _, err := os.Stat(agentPath); err == nil {
			t.Errorf("%s.agent.md must NOT be created for non-advisor skill", name)
		}
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_LegacySuffixCompat verifies that a
// ContentItem with legacy "-adviser" suffix still produces an .agent.md file.
func TestCopilotRenderer_RegenerateAdvisorFiles_LegacySuffixCompat(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "security-adviser", Path: "skills/security-adviser/", Description: "Security advice"},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 1 {
		t.Errorf("Written = %d paths, want 1; paths: %v", len(result.Written), result.Written)
	}

	// File must use the original name (security-adviser) with .agent.md extension.
	agentPath := filepath.Join(workspaceRoot, "agents", "security-adviser.agent.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Errorf("security-adviser.agent.md must exist (legacy suffix compat): %v", err)
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_Idempotency verifies that two
// consecutive calls with identical input produce byte-identical output files.
func TestCopilotRenderer_RegenerateAdvisorFiles_Idempotency(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Architecture advice"},
	}

	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("first call: %v", err)
	}
	agentPath := filepath.Join(workspaceRoot, "agents", "architect-advisor.agent.md")
	first, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}

	if _, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil); err != nil {
		t.Fatalf("second call: %v", err)
	}
	second, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("idempotency violated: byte contents differ between first and second call\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_CustomFlaggedItem verifies that a
// ContentItem with Custom=true produces a valid .agent.md output file.
func TestCopilotRenderer_RegenerateAdvisorFiles_CustomFlaggedItem(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: "security-advisor", Path: "skills/security-advisor/", Description: "Security advice", Custom: true},
	}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	if len(result.Written) != 1 {
		t.Errorf("Written = %d paths, want 1 for Custom=true item; paths: %v", len(result.Written), result.Written)
	}

	agentPath := filepath.Join(workspaceRoot, "agents", "security-advisor.agent.md")
	data, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read security-advisor.agent.md: %v", err)
	}
	fm, _, err := parse.ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if fm["name"] != "security-advisor" {
		t.Errorf("frontmatter name = %v, want %q", fm["name"], "security-advisor")
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_InstalledVsRemovedConflict verifies
// the contract when a name appears in both installed and removed.
// The implementation processes installed first (writes the file) then removed
// (deletes the file), so removed wins — the file is absent after the call.
// Both Written and Deleted record the path (reflecting the sequence of operations).
func TestCopilotRenderer_RegenerateAdvisorFiles_InstalledVsRemovedConflict(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	const advisorName = "architect-advisor"

	installed := []model.ContentItem{
		{Kind: model.KindSkill, Name: advisorName, Path: "skills/architect-advisor/", Description: "Architecture advice"},
	}
	removed := []string{advisorName}

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, installed, removed, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles: %v", err)
	}

	// The file was written (in the install pass) so Written must record it.
	if len(result.Written) != 1 {
		t.Errorf("Written = %d, want 1 (install pass ran); paths: %v", len(result.Written), result.Written)
	}
	// The file was then deleted (in the remove pass) so Deleted must record it.
	if len(result.Deleted) != 1 {
		t.Errorf("Deleted = %d, want 1 (remove pass ran after install); paths: %v", len(result.Deleted), result.Deleted)
	}

	// Net outcome: removed wins — file is absent on disk.
	agentPath := filepath.Join(workspaceRoot, "agents", advisorName+".agent.md")
	if _, err := os.Stat(agentPath); err == nil {
		t.Errorf("removed wins: %s.agent.md must NOT exist when name is in both installed and removed (remove runs after install)", advisorName)
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_EmptyInputs verifies that calling
// with empty installed and removed slices returns an empty result and creates no files.
func TestCopilotRenderer_RegenerateAdvisorFiles_EmptyInputs(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	result, err := r.RegenerateAdvisorFiles(workspaceRoot, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegenerateAdvisorFiles with empty inputs: %v", err)
	}

	if len(result.Written) != 0 {
		t.Errorf("Written = %d, want 0 for empty inputs; paths: %v", len(result.Written), result.Written)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %d, want 0 for empty inputs; paths: %v", len(result.Deleted), result.Deleted)
	}

	// agents directory must not have been created or must be empty.
	agentsDir := filepath.Join(workspaceRoot, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil && len(entries) != 0 {
		t.Errorf("agents dir must be empty for empty inputs; found: %v", entries)
	}
}

// TestCopilotRenderer_RegenerateAdvisorFiles_StatelessReceiver verifies that
// successive calls with different inputs on the same renderer instance do not
// leak state — each call is independent.
func TestCopilotRenderer_RegenerateAdvisorFiles_StatelessReceiver(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCopilotRenderer(copilotAdvisorDef(workspaceRoot))

	// First call: install architect-advisor only.
	first := []model.ContentItem{
		{Kind: model.KindSkill, Name: "architect-advisor", Path: "skills/architect-advisor/", Description: "Architecture"},
	}
	result1, err := r.RegenerateAdvisorFiles(workspaceRoot, first, nil, nil)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(result1.Written) != 1 {
		t.Errorf("first call: Written = %d, want 1; paths: %v", len(result1.Written), result1.Written)
	}

	// Second call: install unit-test-advisor only (different set).
	second := []model.ContentItem{
		{Kind: model.KindSkill, Name: "unit-test-advisor", Path: "skills/unit-test-advisor/", Description: "Unit tests"},
	}
	result2, err := r.RegenerateAdvisorFiles(workspaceRoot, second, nil, nil)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(result2.Written) != 1 {
		t.Errorf("second call: Written = %d, want 1 (should only reflect second input, not accumulate first); paths: %v", len(result2.Written), result2.Written)
	}
	if strings.Contains(result2.Written[0], "architect-advisor") {
		t.Errorf("second call must not return architect-advisor in Written (state leaked from first call); got: %v", result2.Written)
	}
	if !strings.Contains(result2.Written[0], "unit-test-advisor") {
		t.Errorf("second call Written[0] must contain unit-test-advisor; got: %v", result2.Written[0])
	}
}

// TestCopilotRenderer_InstallWorkflow_SharedVariantSuffixStripping verifies that
// _shared/launch-templates.copilot.md is installed as _shared/launch-templates.md,
// while claude and opencode variant files are skipped entirely.
func TestCopilotRenderer_InstallWorkflow_SharedVariantSuffixStripping(t *testing.T) {
	workspaceRoot := t.TempDir()
	def := copilotParityDef(workspaceRoot)
	r := renderers.NewCopilotRenderer(def)

	cachePath := t.TempDir()

	// Minimal workflow cache: one skill, ORCHESTRATOR.md, and _shared/ with variant files.
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

	sharedDir := filepath.Join(cachePath, "_shared")
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

	wf := model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
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

	installedShared := filepath.Join(workspaceRoot, "skills", "sdd-orchestrator", "_shared")

	// launch-templates.md must exist with copilot content (from launch-templates.copilot.md).
	ltPath := filepath.Join(installedShared, "launch-templates.md")
	data, err := os.ReadFile(ltPath)
	if err != nil {
		t.Fatalf("launch-templates.md not installed: %v", err)
	}
	if string(data) != "# copilot launch-templates\n" {
		t.Errorf("launch-templates.md content = %q, want copilot variant content", string(data))
	}

	// envelope-contract.md must exist (generic file).
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
