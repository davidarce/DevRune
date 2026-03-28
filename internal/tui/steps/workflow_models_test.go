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

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil)
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

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil)
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

	result, err := RunWorkflowModelSelection(agents, selection, nil, nil)
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

	result, err := RunWorkflowModelSelection(agents, selection, nil, workflows)
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
