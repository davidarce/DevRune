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
	if opts[0].Value != SDDModelInheritOption {
		t.Errorf("first option value = %q, want %q", opts[0].Value, SDDModelInheritOption)
	}
	if opts[0].Label != SDDModelInheritOption {
		t.Errorf("first option label = %q, want %q", opts[0].Label, SDDModelInheritOption)
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

// TestSDDPhaseRoleNames verifies the phase role names slice is correctly ordered.
func TestSDDPhaseRoleNames(t *testing.T) {
	want := []string{"sdd-explorer", "sdd-planner", "sdd-implementer", "sdd-reviewer"}
	if len(SDDPhaseRoleNames) != len(want) {
		t.Fatalf("SDDPhaseRoleNames len = %d, want %d", len(SDDPhaseRoleNames), len(want))
	}
	for i, name := range want {
		if SDDPhaseRoleNames[i] != name {
			t.Errorf("SDDPhaseRoleNames[%d] = %q, want %q", i, SDDPhaseRoleNames[i], name)
		}
	}
}

// TestSDDModelRoutingAgents verifies the routing agents set contains claude and opencode.
func TestSDDModelRoutingAgents(t *testing.T) {
	for _, agent := range []string{"claude", "opencode"} {
		if !SDDModelRoutingAgents[agent] {
			t.Errorf("SDDModelRoutingAgents[%q] = false, want true", agent)
		}
	}
	// Should not include other agents like factory or copilot.
	for _, agent := range []string{"factory", "copilot", "codex"} {
		if SDDModelRoutingAgents[agent] {
			t.Errorf("SDDModelRoutingAgents[%q] = true, want false", agent)
		}
	}
}

// TestSDDPhaseLabels verifies the phase label map covers all phase role names.
func TestSDDPhaseLabels(t *testing.T) {
	wantLabels := map[string]string{
		"sdd-explorer":    "Explore model",
		"sdd-planner":     "Plan model",
		"sdd-implementer": "Implement model",
		"sdd-reviewer":    "Review model",
	}
	for roleName, wantLabel := range wantLabels {
		got, ok := SDDPhaseLabels[roleName]
		if !ok {
			t.Errorf("SDDPhaseLabels[%q] not found", roleName)
			continue
		}
		if got != wantLabel {
			t.Errorf("SDDPhaseLabels[%q] = %q, want %q", roleName, got, wantLabel)
		}
	}
}

// TestSDDModelInheritOption verifies the sentinel constant value.
func TestSDDModelInheritOption(t *testing.T) {
	if SDDModelInheritOption == "" {
		t.Error("SDDModelInheritOption is empty, want non-empty sentinel string")
	}
}
