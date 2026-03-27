package steps

import (
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// Tests for formatPhaseLabel
// ---------------------------------------------------------------------------

func TestFormatPhaseLabel_KnownRoles(t *testing.T) {
	tests := []struct {
		roleName    string
		agentPrefix string
		want        string
	}{
		{"sdd-explorer", "", "Explore model"},
		{"sdd-planner", "", "Plan model"},
		{"sdd-implementer", "", "Implement model"},
		{"sdd-reviewer", "", "Review model"},
	}

	for _, tt := range tests {
		t.Run(tt.roleName, func(t *testing.T) {
			got := formatPhaseLabel(tt.roleName, tt.agentPrefix)
			if got != tt.want {
				t.Errorf("formatPhaseLabel(%q, %q) = %q, want %q", tt.roleName, tt.agentPrefix, got, tt.want)
			}
		})
	}
}

func TestFormatPhaseLabel_WithAgentPrefix(t *testing.T) {
	got := formatPhaseLabel("sdd-explorer", "claude")
	want := "[claude] Explore model"
	if got != want {
		t.Errorf("formatPhaseLabel(%q, %q) = %q, want %q", "sdd-explorer", "claude", got, want)
	}
}

func TestFormatPhaseLabel_UnknownRole_NoPrefix(t *testing.T) {
	// Unknown roles: strip "sdd-" prefix, capitalise first letter, append " model".
	tests := []struct {
		roleName string
		want     string
	}{
		{"sdd-custom", "Custom model"},
		{"other-role", "Other-role model"}, // no "sdd-" prefix to strip
		{"sdd-", " model"},                 // empty after stripping
	}

	for _, tt := range tests {
		t.Run(tt.roleName, func(t *testing.T) {
			got := formatPhaseLabel(tt.roleName, "")
			if got != tt.want {
				t.Errorf("formatPhaseLabel(%q, %q) = %q, want %q", tt.roleName, "", got, tt.want)
			}
		})
	}
}

func TestFormatPhaseLabel_UnknownRole_WithPrefix(t *testing.T) {
	got := formatPhaseLabel("sdd-custom", "opencode")
	want := "[opencode] Custom model"
	if got != want {
		t.Errorf("formatPhaseLabel(%q, %q) = %q, want %q", "sdd-custom", "opencode", got, want)
	}
}

// ---------------------------------------------------------------------------
// Tests for RunSDDModelSelection — skip conditions
// ---------------------------------------------------------------------------

// TestRunSDDModelSelection_ReturnsNilWhenNoAgentSupportsModelRouting verifies
// that the step is skipped when none of the selected agents are in
// model.SDDModelRoutingAgents.
func TestRunSDDModelSelection_ReturnsNilWhenNoAgentSupportsModelRouting(t *testing.T) {
	agents := []string{"copilot", "factory"} // neither supports model routing
	selection := SelectionResult{
		Repos: []RepoSelectionResult{
			{
				Source:            "github.com/org/repo",
				SelectedWorkflows: []string{"sdd"},
			},
		},
	}

	result, err := RunSDDModelSelection(agents, selection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when no agents support model routing, got %v", result)
	}
}

// TestRunSDDModelSelection_ReturnsNilWhenNoWorkflowsSelected verifies that the
// step is skipped when no workflows are selected in any repo.
func TestRunSDDModelSelection_ReturnsNilWhenNoWorkflowsSelected(t *testing.T) {
	agents := []string{"claude", "opencode"} // both support model routing
	selection := SelectionResult{
		Repos: []RepoSelectionResult{
			{
				Source:            "github.com/org/repo",
				SelectedWorkflows: []string{}, // no workflows selected
			},
		},
	}

	result, err := RunSDDModelSelection(agents, selection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when no workflows selected, got %v", result)
	}
}

// TestRunSDDModelSelection_ReturnsNilWhenNoRepos verifies the step is skipped
// with an empty repos list.
func TestRunSDDModelSelection_ReturnsNilWhenNoRepos(t *testing.T) {
	agents := []string{"claude"}
	selection := SelectionResult{
		Repos: []RepoSelectionResult{}, // empty
	}

	result, err := RunSDDModelSelection(agents, selection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when repos is empty, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// Tests for AgentModelConfig building logic
// ---------------------------------------------------------------------------

// TestAgentModelConfig_ClaudeOptions verifies that the AgentModelConfig built for
// "claude" uses ClaudeModelOptions (inherit sentinel + haiku/sonnet/opus).
func TestAgentModelConfig_ClaudeOptions(t *testing.T) {
	opts := model.ClaudeModelOptions()

	if len(opts) < 2 {
		t.Fatalf("ClaudeModelOptions() returned %d options, want at least 2 (inherit + models)", len(opts))
	}

	// First option must be the inherit sentinel.
	if opts[0].Value != model.SDDModelInheritOption {
		t.Errorf("opts[0].Value = %q, want inherit sentinel %q", opts[0].Value, model.SDDModelInheritOption)
	}

	// There should be exactly 4 options: inherit + haiku + sonnet + opus.
	if len(opts) != 4 {
		t.Errorf("len(ClaudeModelOptions()) = %d, want 4 (inherit + haiku + sonnet + opus)", len(opts))
	}
}

// TestAgentModelConfig_OpenCodeOptionsWithFallback verifies that OpenCodeModelOptions
// always prepends the inherit sentinel as the first option.
func TestAgentModelConfig_OpenCodeOptionsWithFallback(t *testing.T) {
	fallback := []string{"gpt-4o", "claude-sonnet-4.5"}
	opts := model.OpenCodeModelOptions(fallback)

	if len(opts) == 0 {
		t.Fatal("OpenCodeModelOptions() returned empty slice")
	}

	// First option must always be the inherit sentinel regardless of source.
	if opts[0].Value != model.SDDModelInheritOption {
		t.Errorf("opts[0].Value = %q, want inherit sentinel %q", opts[0].Value, model.SDDModelInheritOption)
	}

	// Must have at least the sentinel + at least one model option.
	if len(opts) < 2 {
		t.Errorf("len(OpenCodeModelOptions(fallback)) = %d, want >= 2 (sentinel + at least one model)", len(opts))
	}
}

// TestAgentModelConfig_SDDModelRoutingAgents verifies that the routing agents map
// contains the expected entries.
func TestAgentModelConfig_SDDModelRoutingAgents(t *testing.T) {
	if !model.SDDModelRoutingAgents["claude"] {
		t.Error("claude should be in SDDModelRoutingAgents")
	}
	if !model.SDDModelRoutingAgents["opencode"] {
		t.Error("opencode should be in SDDModelRoutingAgents")
	}
	if model.SDDModelRoutingAgents["factory"] {
		t.Error("factory should NOT be in SDDModelRoutingAgents")
	}
	if model.SDDModelRoutingAgents["copilot"] {
		t.Error("copilot should NOT be in SDDModelRoutingAgents")
	}
}
