// SPDX-License-Identifier: MIT

package model

import (
	"testing"
)

// TestClaudeModelOptions verifies the options list for Claude agents.
func TestClaudeModelOptions(t *testing.T) {
	opts := ClaudeModelOptions()

	// Must start with the inherit sentinel.
	if len(opts) == 0 {
		t.Fatal("ClaudeModelOptions() returned empty slice")
	}
	if opts[0].Value != ModelInheritOption {
		t.Errorf("first option value = %q, want %q", opts[0].Value, ModelInheritOption)
	}
	if opts[0].Label != ModelInheritOption {
		t.Errorf("first option label = %q, want %q", opts[0].Label, ModelInheritOption)
	}

	// Must include exactly haiku, sonnet, opus (3 models) after the sentinel.
	wantModels := []string{"haiku", "sonnet", "opus"}
	if len(opts) != len(wantModels)+1 {
		t.Errorf("ClaudeModelOptions() returned %d options, want %d (1 sentinel + 3 models)", len(opts), len(wantModels)+1)
	}

	for i, want := range wantModels {
		got := opts[i+1]
		if got.Value != want {
			t.Errorf("opts[%d].Value = %q, want %q", i+1, got.Value, want)
		}
		if got.Label != want {
			t.Errorf("opts[%d].Label = %q, want %q", i+1, got.Label, want)
		}
	}
}

// TestModelRoutingAgents verifies the routing agents set contains claude and opencode.
func TestModelRoutingAgents(t *testing.T) {
	for _, agent := range []string{"claude", "opencode"} {
		if !ModelRoutingAgents[agent] {
			t.Errorf("ModelRoutingAgents[%q] = false, want true", agent)
		}
	}
	// Should not include other agents like factory or copilot.
	for _, agent := range []string{"factory", "copilot", "codex"} {
		if ModelRoutingAgents[agent] {
			t.Errorf("ModelRoutingAgents[%q] = true, want false", agent)
		}
	}
}

// TestModelInheritOption verifies the sentinel constant value.
func TestModelInheritOption(t *testing.T) {
	if ModelInheritOption == "" {
		t.Error("ModelInheritOption is empty, want non-empty sentinel string")
	}
}

// TestBackwardCompatAliases verifies the backward-compatible aliases point to the same values.
func TestBackwardCompatAliases(t *testing.T) {
	if SDDModelInheritOption != ModelInheritOption {
		t.Errorf("SDDModelInheritOption = %q, want same as ModelInheritOption = %q", SDDModelInheritOption, ModelInheritOption)
	}
	for k, v := range ModelRoutingAgents {
		if SDDModelRoutingAgents[k] != v {
			t.Errorf("SDDModelRoutingAgents[%q] = %v, want %v", k, SDDModelRoutingAgents[k], v)
		}
	}
}

// TestPlaceholderKeyFromRole tests the placeholder key derivation logic.
func TestPlaceholderKeyFromRole(t *testing.T) {
	tests := []struct {
		workflowName        string
		roleName            string
		explicitPlaceholder string
		want                string
	}{
		{"sdd", "sdd-explorer", "", "EXPLORER"},
		{"sdd", "sdd-planner", "", "PLANNER"},
		{"sdd", "sdd-implementer", "", "IMPLEMENTER"},
		{"sdd", "sdd-reviewer", "", "REVIEWER"},
		{"cicd", "reviewer", "", "REVIEWER"},
		{"cicd", "code-quality-checker", "", "CODE_QUALITY_CHECKER"},
		{"cicd", "code-quality-checker", "CHECKER", "CHECKER"},
		{"sdd", "sdd-explorer", "SCOUT", "SCOUT"},
		{"myflow", "myflow-step-one", "", "STEP_ONE"},
	}

	for _, tt := range tests {
		t.Run(tt.roleName, func(t *testing.T) {
			got := PlaceholderKeyFromRole(tt.workflowName, tt.roleName, tt.explicitPlaceholder)
			if got != tt.want {
				t.Errorf("PlaceholderKeyFromRole(%q, %q, %q) = %q, want %q",
					tt.workflowName, tt.roleName, tt.explicitPlaceholder, got, tt.want)
			}
		})
	}
}
