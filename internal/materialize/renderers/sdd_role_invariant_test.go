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
)

// roleInvariantPhrase is the canonical heading of the SDD orchestrator's
// "delegate-only" contract. The catalog places this block at the top of every
// ORCHESTRATOR.md variant (see devrune-starter-catalog#34). Every renderer
// must preserve the block when transforming content for its agent —
// placeholder substitution and inlining (OpenCode opencode.json, Copilot
// .agent.md) are the highest-risk transformations.
const roleInvariantPhrase = "Role Invariant — you orchestrate, you do not implement"

// roleInvariantMaxLineFromTop bounds where the phrase may appear in the
// rendered orchestrator surface. The block must sit near the top so the
// agent reads it before scanning past — burying it past this line is a
// regression even if the text is technically present.
const roleInvariantMaxLineFromTop = 30

// orchestratorBodyWithRoleInvariant returns ORCHESTRATOR content that opens
// with the canonical role invariant block, mirroring what the catalog ships.
// `title` lets each variant have a distinguishable H1 so test failures
// attribute correctly when the wrong file is selected by variant probing.
func orchestratorBodyWithRoleInvariant(title string) string {
	return "# " + title + "\n\n" +
		"## " + roleInvariantPhrase + "\n\n" +
		"Outside `.sdd/{change}/`, your only outputs are: sub-agent launches, AskUserQuestion, mkdir for .sdd/, and Bash(crit ...).\n\n" +
		"You do **not**: Edit/Write source files, run builds/tests/lints, run git commit/push, create branches/commits/PRs.\n\n" +
		"If your next planned action is on the \"do not\" list, you have lost the role — re-read this section and delegate.\n\n" +
		"## Body\n\nRest of orchestrator instructions.\n"
}

// assertRoleInvariantInRenderedSurface asserts the role invariant phrase
// appears in the rendered text and is positioned within the first
// roleInvariantMaxLineFromTop lines.
func assertRoleInvariantInRenderedSurface(t *testing.T, agent string, content string) {
	t.Helper()
	if !strings.Contains(content, roleInvariantPhrase) {
		t.Fatalf("%s: role invariant phrase %q not found in orchestrator surface; full content:\n%s",
			agent, roleInvariantPhrase, content)
	}
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, roleInvariantPhrase) {
			if i+1 > roleInvariantMaxLineFromTop {
				t.Errorf("%s: role invariant phrase appears at line %d (max %d); the block has been buried",
					agent, i+1, roleInvariantMaxLineFromTop)
			}
			return
		}
	}
}

// writeSDDOrchestratorCacheWithRoleInvariant seeds a temp cache dir with the
// minimum SDD layout required by every renderer (workflow.yaml, one phase
// SKILL.md stub, ORCHESTRATOR.md and an optional ORCHESTRATOR.<variant>.md).
// Both orchestrator files contain the canonical role invariant block; when a
// variant is present the agents that probe for one (claude / copilot /
// opencode) MUST install the variant content.
func writeSDDOrchestratorCacheWithRoleInvariant(t *testing.T, variantSuffix string) string {
	t.Helper()
	cache := t.TempDir()

	wfYAML := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
  workingDir: sdd-orchestrator
components:
  skills:
    - sdd-plan
  entrypoint: ORCHESTRATOR.md
  roles:
    - name: sdd-planner
      kind: subagent
      skill: sdd-plan
      model: sonnet
    - name: sdd-orchestrator
      kind: orchestrator
`
	if err := os.WriteFile(filepath.Join(cache, "workflow.yaml"), []byte(wfYAML), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	skillDir := filepath.Join(cache, "sdd-plan")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir sdd-plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: sdd-plan\ndescription: Plan\n---\nBody.\n"), 0o644); err != nil {
		t.Fatalf("write sdd-plan/SKILL.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cache, "ORCHESTRATOR.md"),
		[]byte(orchestratorBodyWithRoleInvariant("SDD Orchestrator")), 0o644); err != nil {
		t.Fatalf("write ORCHESTRATOR.md: %v", err)
	}

	if variantSuffix != "" {
		variantPath := filepath.Join(cache, "ORCHESTRATOR."+variantSuffix+".md")
		if err := os.WriteFile(variantPath,
			[]byte(orchestratorBodyWithRoleInvariant("SDD Orchestrator ("+variantSuffix+" variant)")),
			0o644); err != nil {
			t.Fatalf("write ORCHESTRATOR.%s.md: %v", variantSuffix, err)
		}
	}
	return cache
}

func sddTestWorkflowForRoleInvariant() model.WorkflowManifest {
	return model.WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   model.WorkflowMetadata{Name: "sdd", Version: "1.0.0", WorkingDir: "sdd-orchestrator"},
		Components: model.WorkflowComponents{
			Skills:     []string{"sdd-plan"},
			Entrypoint: "ORCHESTRATOR.md",
			Roles: []model.WorkflowRole{
				{Name: "sdd-planner", Kind: "subagent", Skill: "sdd-plan", Model: "sonnet"},
				{Name: "sdd-orchestrator", Kind: "orchestrator"},
			},
		},
	}
}

// TestSDDRoleInvariant_SurvivesRendering — for every supported agent,
// rendering an SDD workflow whose ORCHESTRATOR.md begins with the canonical
// "Role Invariant" block must produce an orchestrator prompt that still
// contains the block, near the top.
//
// Companion to davidarce/DevRune#58 / catalog #34. Guards against renderer
// changes (placeholder substitution, JSON inlining, .agent.md generation)
// that strip or bury the delegate-only contract — the failure mode that
// motivated the catalog audit.
func TestSDDRoleInvariant_SurvivesRendering(t *testing.T) {
	t.Run("claude", func(t *testing.T) {
		r := renderers.NewClaudeRenderer(claudeNativeAgentDef())
		cache := writeSDDOrchestratorCacheWithRoleInvariant(t, "claude")
		workspace := t.TempDir()
		if _, err := r.InstallWorkflow(sddTestWorkflowForRoleInvariant(), cache, workspace); err != nil {
			t.Fatalf("InstallWorkflow: %v", err)
		}
		path := filepath.Join(workspace, "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read installed orchestrator at %s: %v", path, err)
		}
		assertRoleInvariantInRenderedSurface(t, "claude", string(data))
	})

	t.Run("codex", func(t *testing.T) {
		projectRoot := t.TempDir()
		workspaceDir := filepath.Join(projectRoot, ".codex")
		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		def := model.AgentDefinition{
			Name:        "codex",
			Type:        "codex",
			Workspace:   workspaceDir,
			SkillDir:    "../.agents/skills",
			RulesDir:    "rules",
			CatalogFile: "AGENTS.md",
		}
		r := renderers.NewCodexRenderer(def)
		cache := writeSDDOrchestratorCacheWithRoleInvariant(t, "")
		if _, err := r.InstallWorkflow(sddTestWorkflowForRoleInvariant(), cache, workspaceDir); err != nil {
			t.Fatalf("InstallWorkflow: %v", err)
		}
		path := filepath.Join(projectRoot, ".agents", "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read installed orchestrator at %s: %v", path, err)
		}
		assertRoleInvariantInRenderedSurface(t, "codex", string(data))
	})

	t.Run("factory", func(t *testing.T) {
		projectRoot := t.TempDir()
		workspaceDir := filepath.Join(projectRoot, ".factory")
		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		def := model.AgentDefinition{
			Name:        "factory",
			Type:        "factory",
			Workspace:   workspaceDir,
			SkillDir:    "../.agents/skills",
			RulesDir:    "rules",
			CatalogFile: "AGENTS.md",
		}
		r := renderers.NewFactoryRenderer(def)
		cache := writeSDDOrchestratorCacheWithRoleInvariant(t, "")
		if _, err := r.InstallWorkflow(sddTestWorkflowForRoleInvariant(), cache, workspaceDir); err != nil {
			t.Fatalf("InstallWorkflow: %v", err)
		}
		path := filepath.Join(projectRoot, ".agents", "skills", "sdd-orchestrator", "ORCHESTRATOR.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read installed orchestrator at %s: %v", path, err)
		}
		assertRoleInvariantInRenderedSurface(t, "factory", string(data))
	})

	t.Run("copilot", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		def := model.AgentDefinition{
			Name:        "copilot",
			Type:        "copilot",
			Workspace:   workspaceRoot,
			SkillDir:    "skills",
			AgentDir:    "agents",
			RulesDir:    "rules",
			CatalogFile: "copilot-instructions.md",
		}
		r := renderers.NewCopilotRenderer(def)
		cache := writeSDDOrchestratorCacheWithRoleInvariant(t, "copilot")
		if _, err := r.InstallWorkflow(sddTestWorkflowForRoleInvariant(), cache, workspaceRoot); err != nil {
			t.Fatalf("InstallWorkflow: %v", err)
		}
		path := filepath.Join(workspaceRoot, "agents", "sdd-orchestrator.agent.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read installed orchestrator at %s: %v", path, err)
		}
		assertRoleInvariantInRenderedSurface(t, "copilot", string(data))
	})

	t.Run("opencode", func(t *testing.T) {
		projectRoot := t.TempDir()
		workspaceDir := filepath.Join(projectRoot, ".opencode")
		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		def := model.AgentDefinition{
			Name:        "opencode",
			Type:        "opencode",
			Workspace:   workspaceDir,
			SkillDir:    "../.agents/skills",
			CommandDir:  "commands",
			RulesDir:    "rules",
			CatalogFile: "AGENTS.md",
		}
		r := renderers.NewOpenCodeRenderer(def)
		cache := writeSDDOrchestratorCacheWithRoleInvariant(t, "opencode")
		if _, err := r.InstallWorkflow(sddTestWorkflowForRoleInvariant(), cache, workspaceDir); err != nil {
			t.Fatalf("InstallWorkflow: %v", err)
		}
		// OpenCode inlines ORCHESTRATOR content into opencode.json under
		// agent.sdd-orchestrator.prompt — placeholder substitution and JSON
		// escaping make this the highest-risk transformation for stripping.
		data, err := os.ReadFile(filepath.Join(workspaceDir, "opencode.json"))
		if err != nil {
			t.Fatalf("read opencode.json: %v", err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("parse opencode.json: %v", err)
		}
		agentSection, ok := cfg["agent"].(map[string]any)
		if !ok {
			t.Fatalf("opencode.json missing 'agent' section; content:\n%s", string(data))
		}
		orch, ok := agentSection["sdd-orchestrator"].(map[string]any)
		if !ok {
			t.Fatalf("agent section missing 'sdd-orchestrator'; keys: %v", mapKeys(agentSection))
		}
		prompt, _ := orch["prompt"].(string)
		if prompt == "" {
			t.Fatalf("sdd-orchestrator prompt is empty in opencode.json")
		}
		assertRoleInvariantInRenderedSurface(t, "opencode", prompt)
	})
}
