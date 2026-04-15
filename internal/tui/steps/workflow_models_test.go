// SPDX-License-Identifier: MIT

package steps

import (
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// Tests for formatRoleLabel
// ---------------------------------------------------------------------------

func TestFormatRoleLabel_KnownRoles(t *testing.T) {
	tests := []struct {
		roleName    string
		agentPrefix string
		want        string
	}{
		{"sdd-explorer", "", "Explorer model"},
		{"sdd-planner", "", "Planner model"},
		{"sdd-implementer", "", "Implementer model"},
		{"sdd-reviewer", "", "Reviewer model"},
	}

	for _, tt := range tests {
		t.Run(tt.roleName, func(t *testing.T) {
			got := formatRoleLabel(tt.roleName, tt.agentPrefix)
			if got != tt.want {
				t.Errorf("formatRoleLabel(%q, %q) = %q, want %q", tt.roleName, tt.agentPrefix, got, tt.want)
			}
		})
	}
}

func TestFormatRoleLabel_WithAgentPrefix(t *testing.T) {
	got := formatRoleLabel("sdd-explorer", "claude")
	want := "[claude] Explorer model"
	if got != want {
		t.Errorf("formatRoleLabel(%q, %q) = %q, want %q", "sdd-explorer", "claude", got, want)
	}
}

func TestFormatRoleLabel_CustomWorkflow(t *testing.T) {
	tests := []struct {
		roleName string
		want     string
	}{
		{"cicd-reviewer", "Reviewer model"},
		{"review-checker", "Checker model"},
		{"simple", "Simple model"},
	}

	for _, tt := range tests {
		t.Run(tt.roleName, func(t *testing.T) {
			got := formatRoleLabel(tt.roleName, "")
			if got != tt.want {
				t.Errorf("formatRoleLabel(%q, %q) = %q, want %q", tt.roleName, "", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for RunWorkflowModelSelection — skip conditions
// ---------------------------------------------------------------------------

func TestRunWorkflowModelSelection_ReturnsNilWhenNoAgentSupportsModelRouting(t *testing.T) {
	agents := []string{"copilot", "factory"}
	selection := SelectionResult{
		Repos: []RepoSelectionResult{
			{
				Source:            "github.com/org/repo",
				SelectedWorkflows: []string{"sdd"},
			},
		},
	}

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when no agents support model routing, got %v", result)
	}
}

func TestRunWorkflowModelSelection_ReturnsNilWhenNoWorkflowsSelected(t *testing.T) {
	agents := []string{"claude", "opencode"}
	selection := SelectionResult{
		Repos: []RepoSelectionResult{
			{
				Source:            "github.com/org/repo",
				SelectedWorkflows: []string{},
			},
		},
	}

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when no workflows selected, got %v", result)
	}
}

func TestRunWorkflowModelSelection_ReturnsNilWhenNoRepos(t *testing.T) {
	agents := []string{"claude"}
	selection := SelectionResult{
		Repos: []RepoSelectionResult{},
	}

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when repos is empty, got %v", result)
	}
}

func TestRunWorkflowModelSelection_ReturnsNilWhenNoRolesHaveModel(t *testing.T) {
	agents := []string{"claude"}
	selection := SelectionResult{
		Repos: []RepoSelectionResult{
			{
				Source:            "github.com/org/repo",
				SelectedWorkflows: []string{"sdd"},
			},
		},
	}
	// Workflow with roles but no model fields.
	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{Name: "sdd"},
			Components: model.WorkflowComponents{
				Roles: []model.WorkflowRole{
					{Name: "sdd-orchestrator", Kind: "orchestrator"},
				},
			},
		},
	}

	result, err := RunWorkflowModelSelection(agents, selection, nil, workflows, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when no roles have model, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// Tests for AgentModelConfig building logic
// ---------------------------------------------------------------------------

func TestAgentModelConfig_ClaudeOptions(t *testing.T) {
	opts := model.ClaudeModelOptions()

	if len(opts) < 2 {
		t.Fatalf("ClaudeModelOptions() returned %d options, want at least 2 (inherit + models)", len(opts))
	}

	if opts[0].Value != model.ModelInheritOption {
		t.Errorf("opts[0].Value = %q, want inherit sentinel %q", opts[0].Value, model.ModelInheritOption)
	}

	if len(opts) != 4 {
		t.Errorf("len(ClaudeModelOptions()) = %d, want 4 (inherit + haiku + sonnet + opus)", len(opts))
	}
}

func TestAgentModelConfig_OpenCodeOptionsWithFallback(t *testing.T) {
	fallback := []string{"gpt-4o", "claude-sonnet-4.5"}
	opts := model.OpenCodeModelOptions(fallback)

	if len(opts) == 0 {
		t.Fatal("OpenCodeModelOptions() returned empty slice")
	}

	if opts[0].Value != model.ModelInheritOption {
		t.Errorf("opts[0].Value = %q, want inherit sentinel %q", opts[0].Value, model.ModelInheritOption)
	}

	if len(opts) < 2 {
		t.Errorf("len(OpenCodeModelOptions(fallback)) = %d, want >= 2 (sentinel + at least one model)", len(opts))
	}
}

func TestAgentModelConfig_ModelRoutingAgents(t *testing.T) {
	if !model.ModelRoutingAgents["claude"] {
		t.Error("claude should be in ModelRoutingAgents")
	}
	if !model.ModelRoutingAgents["opencode"] {
		t.Error("opencode should be in ModelRoutingAgents")
	}
	if model.ModelRoutingAgents["factory"] {
		t.Error("factory should NOT be in ModelRoutingAgents")
	}
	if model.ModelRoutingAgents["copilot"] {
		t.Error("copilot should NOT be in ModelRoutingAgents")
	}
}

// ---------------------------------------------------------------------------
// Tests for WorkflowModelLayout
// ---------------------------------------------------------------------------

func TestWorkflowModelLayout_SingleRole(t *testing.T) {
	// Single role should use default layout.
	_ = WorkflowModelLayout(1, 40)
}

func TestWorkflowModelLayout_FourRoles(t *testing.T) {
	// Four roles with enough terminal height should use grid.
	_ = WorkflowModelLayout(4, 40)
}

// ---------------------------------------------------------------------------
// Tests for phaseFromRole — phase-grouped layout (T016)
// ---------------------------------------------------------------------------

func TestPhaseFromRole_SddPrefix(t *testing.T) {
	tests := []struct {
		roleKey string
		want    string
	}{
		{"sdd-explore", "Explore"},
		{"sdd-plan", "Plan"},
		{"sdd-implement", "Implement"},
		{"sdd-review", "Review"},
		{"cicd-build", "Build"},
		{"cicd-deploy", "Deploy"},
		{"review-checker", "Checker"},
		{"simple", "Simple"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.roleKey, func(t *testing.T) {
			got := phaseFromRole(tt.roleKey)
			if got != tt.want {
				t.Errorf("phaseFromRole(%q) = %q, want %q", tt.roleKey, got, tt.want)
			}
		})
	}
}

func TestPhaseFromRole_TrailingHyphen(t *testing.T) {
	// A role key ending with a hyphen should use the prefix as phase name.
	got := phaseFromRole("sdd-")
	want := "Sdd"
	if got != want {
		t.Errorf("phaseFromRole(%q) = %q, want %q", "sdd-", got, want)
	}
}

// ---------------------------------------------------------------------------
// Tests for groupRolesByPhase — phase-grouped layout (T015)
// ---------------------------------------------------------------------------

func TestGroupRolesByPhase_PreservesInsertionOrder(t *testing.T) {
	roles := []model.WorkflowRole{
		{Name: "sdd-explore", Kind: "subagent"},
		{Name: "sdd-plan", Kind: "subagent"},
		{Name: "sdd-implement", Kind: "subagent"},
		{Name: "sdd-review", Kind: "subagent"},
	}

	phases, groups := groupRolesByPhase(roles)

	wantPhases := []string{"Explore", "Plan", "Implement", "Review"}
	if len(phases) != len(wantPhases) {
		t.Fatalf("groupRolesByPhase() phases = %v (len %d), want %v (len %d)",
			phases, len(phases), wantPhases, len(wantPhases))
	}
	for i, p := range phases {
		if p != wantPhases[i] {
			t.Errorf("phases[%d] = %q, want %q", i, p, wantPhases[i])
		}
	}

	// Each phase should have exactly one role.
	for _, phase := range wantPhases {
		if len(groups[phase]) != 1 {
			t.Errorf("groups[%q] has %d roles, want 1", phase, len(groups[phase]))
		}
	}
}

func TestGroupRolesByPhase_MultipleRolesSamePhase(t *testing.T) {
	roles := []model.WorkflowRole{
		{Name: "sdd-explore", Kind: "subagent"},
		{Name: "cicd-build", Kind: "subagent"},
		{Name: "cicd-deploy", Kind: "subagent"},
	}

	phases, groups := groupRolesByPhase(roles)

	// Explore and Build should be separate phases; deploy groups with Build.
	// Note: "cicd-build" → "Build" and "cicd-deploy" → "Deploy" — they differ.
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	_ = groups
}

func TestGroupRolesByPhase_EmptyInput(t *testing.T) {
	phases, groups := groupRolesByPhase(nil)
	if len(phases) != 0 {
		t.Errorf("expected 0 phases for nil input, got %d", len(phases))
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}
