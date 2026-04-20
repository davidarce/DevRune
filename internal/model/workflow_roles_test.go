// SPDX-License-Identifier: MIT

package model

import (
	"math"
	"strings"
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

// TestModelRoutingAgents verifies the routing agents set contains claude, opencode, and copilot.
func TestModelRoutingAgents(t *testing.T) {
	for _, agent := range []string{"claude", "opencode", "copilot"} {
		if !ModelRoutingAgents[agent] {
			t.Errorf("ModelRoutingAgents[%q] = false, want true", agent)
		}
	}
	// Should not include other agents like factory or codex.
	for _, agent := range []string{"factory", "codex"} {
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

// TestCopilotModelOptions verifies the options list for Copilot agents.
// It checks count, sentinel, ordering, bare IDs (no "anthropic/" prefix),
// provider labels, and plan-availability strings.
func TestCopilotModelOptions(t *testing.T) {
	opts := CopilotModelOptions()

	// Must return exactly 11 options: sentinel + 10 models.
	const wantTotal = 11
	if len(opts) != wantTotal {
		t.Fatalf("CopilotModelOptions() returned %d options, want %d (1 sentinel + 10 models)", len(opts), wantTotal)
	}

	// First option must be the inherit sentinel.
	if opts[0].Value != ModelInheritOption {
		t.Errorf("opts[0].Value = %q, want %q (sentinel)", opts[0].Value, ModelInheritOption)
	}
	if opts[0].Label != ModelInheritOption {
		t.Errorf("opts[0].Label = %q, want %q (sentinel)", opts[0].Label, ModelInheritOption)
	}

	// Expected display-name order (provider-grouped: Anthropic → Google → OpenAI → xAI).
	// Values are VS Code display names, NOT API slugs.
	wantValues := []string{
		"Claude Haiku 4.5",  // Anthropic, pos 1
		"Claude Sonnet 4.6", // Anthropic, pos 2
		"Claude Opus 4.7",   // Anthropic, pos 3
		"Gemini 2.5 Pro",          // Google,    pos 4
		"Gemini 3.1 Pro (Preview)", // Google,    pos 5
		"Gemini 3 Flash (Preview)", // Google,    pos 6
		"GPT-5 mini",              // OpenAI,    pos 7
		"GPT-5.4",                 // OpenAI,    pos 8
		"GPT-5.3-Codex",           // OpenAI,    pos 9
		"Grok Code Fast 1",  // xAI,       pos 10
	}
	for i, want := range wantValues {
		got := opts[i+1]
		if got.Value != want {
			t.Errorf("opts[%d].Value = %q, want %q", i+1, got.Value, want)
		}
		// Values must be display names — no "anthropic/" prefix or slug format.
		if strings.HasPrefix(got.Value, "anthropic/") {
			t.Errorf("opts[%d].Value = %q contains 'anthropic/' prefix; want display name", i+1, got.Value)
		}
		// Label must differ from Value (label includes provider and plan tier).
		if got.Label == got.Value {
			t.Errorf("opts[%d].Label == Value (%q); want label to include provider and plan info", i+1, got.Value)
		}
	}

	// Provider-prefix checks: label must contain the expected provider string.
	providerChecks := []struct {
		idx      int
		modelID  string
		provider string
	}{
		{1, "Claude Haiku 4.5", "Anthropic"},
		{2, "Claude Sonnet 4.6", "Anthropic"},
		{3, "Claude Opus 4.7", "Anthropic"},
		{4, "Gemini 2.5 Pro", "Google"},
		{5, "Gemini 3.1 Pro (Preview)", "Google"},
		{6, "Gemini 3 Flash (Preview)", "Google"},
		{7, "GPT-5 mini", "OpenAI"},
		{8, "GPT-5.4", "OpenAI"},
		{9, "GPT-5.3-Codex", "OpenAI"},
		{10, "Grok Code Fast 1", "xAI"},
	}
	for _, c := range providerChecks {
		label := opts[c.idx].Label
		if !strings.Contains(label, c.provider) {
			t.Errorf("opts[%d] (%s) label = %q; want it to contain provider %q", c.idx, c.modelID, label, c.provider)
		}
	}

	// Plan-availability string checks.
	availChecks := []struct {
		idx     int
		modelID string
		substr  string
	}{
		{1, "claude-haiku-4.5", "Free"},
		{3, "claude-opus-4.7", "Pro+"},
		{5, "gemini-3.1-pro", "Preview"},
		{6, "gemini-3-flash", "Preview"},
	}
	for _, c := range availChecks {
		label := opts[c.idx].Label
		if !strings.Contains(label, c.substr) {
			t.Errorf("opts[%d] (%s) label = %q; want it to contain availability substring %q", c.idx, c.modelID, label, c.substr)
		}
	}

	// Provider-ordering: positions 1-3 Anthropic, 4-6 Google, 7-9 OpenAI, 10 xAI.
	anthropicIDs := []string{"Claude Haiku 4.5", "Claude Sonnet 4.6", "Claude Opus 4.7"}
	for i, want := range anthropicIDs {
		if opts[i+1].Value != want {
			t.Errorf("Anthropic position %d: got %q, want %q", i+1, opts[i+1].Value, want)
		}
	}
	googleIDs := []string{"Gemini 2.5 Pro", "Gemini 3.1 Pro (Preview)", "Gemini 3 Flash (Preview)"}
	for i, want := range googleIDs {
		if opts[i+4].Value != want {
			t.Errorf("Google position %d: got %q, want %q", i+4, opts[i+4].Value, want)
		}
	}
	openaiIDs := []string{"GPT-5 mini", "GPT-5.4", "GPT-5.3-Codex"}
	for i, want := range openaiIDs {
		if opts[i+7].Value != want {
			t.Errorf("OpenAI position %d: got %q, want %q", i+7, opts[i+7].Value, want)
		}
	}
	if opts[10].Value != "Grok Code Fast 1" {
		t.Errorf("xAI position 10: got %q, want %q", opts[10].Value, "Grok Code Fast 1")
	}
}

// TestModelRoutingAgents_IncludesCopilot verifies that "copilot" is in the ModelRoutingAgents map.
func TestModelRoutingAgents_IncludesCopilot(t *testing.T) {
	if !ModelRoutingAgents["copilot"] {
		t.Errorf("ModelRoutingAgents[\"copilot\"] = false, want true; copilot model routing was not enabled")
	}
}

// TestCopilotModelOptionsUpTo verifies tier-based filtering of Copilot model options.
func TestCopilotModelOptionsUpTo(t *testing.T) {
	// MaxFloat64 → same result as CopilotModelOptions() — sentinel + all 10 models.
	optsAll := CopilotModelOptionsUpTo(math.MaxFloat64)
	const wantTotal = 11
	if len(optsAll) != wantTotal {
		t.Errorf("CopilotModelOptionsUpTo(MaxFloat64) returned %d options, want %d (sentinel + 10 models)", len(optsAll), wantTotal)
	}
	if optsAll[0].Value != ModelInheritOption {
		t.Errorf("CopilotModelOptionsUpTo(MaxFloat64): first option = %q, want sentinel %q", optsAll[0].Value, ModelInheritOption)
	}

	// Tier 1.0 → sentinel + Claude Haiku 4.5 + GPT-5 mini (3 total).
	opts1 := CopilotModelOptionsUpTo(1.0)
	if len(opts1) != 3 {
		t.Errorf("CopilotModelOptionsUpTo(1.0) returned %d options, want 3 (sentinel + Haiku + GPT-5 mini)", len(opts1))
	}
	if opts1[0].Value != ModelInheritOption {
		t.Errorf("CopilotModelOptionsUpTo(1.0): first option = %q, want sentinel %q", opts1[0].Value, ModelInheritOption)
	}
	wantTier1Values := []string{"Claude Haiku 4.5", "GPT-5 mini"}
	for i, want := range wantTier1Values {
		got := opts1[i+1]
		if got.Value != want {
			t.Errorf("CopilotModelOptionsUpTo(1.0) opts[%d].Value = %q, want %q", i+1, got.Value, want)
		}
	}

	// Tier 2.0 → sentinel + all except Claude Opus 4.7 (tier 3.0) = 10 options.
	opts2 := CopilotModelOptionsUpTo(2.0)
	if len(opts2) != 10 {
		t.Errorf("CopilotModelOptionsUpTo(2.0) returned %d options, want 10 (sentinel + 9 tier-1/2 models)", len(opts2))
	}
	if opts2[0].Value != ModelInheritOption {
		t.Errorf("CopilotModelOptionsUpTo(2.0): first option = %q, want sentinel %q", opts2[0].Value, ModelInheritOption)
	}
	// Opus 4.7 must NOT appear.
	for _, opt := range opts2 {
		if opt.Value == "Claude Opus 4.7" {
			t.Errorf("CopilotModelOptionsUpTo(2.0) unexpectedly contains Claude Opus 4.7 (tier 3.0)")
		}
	}

	// Tier 3.0 → sentinel + all 10 models = 11 options (same as MaxFloat64).
	opts3 := CopilotModelOptionsUpTo(3.0)
	if len(opts3) != wantTotal {
		t.Errorf("CopilotModelOptionsUpTo(3.0) returned %d options, want %d (sentinel + 10 models)", len(opts3), wantTotal)
	}
	if opts3[0].Value != ModelInheritOption {
		t.Errorf("CopilotModelOptionsUpTo(3.0): first option = %q, want sentinel %q", opts3[0].Value, ModelInheritOption)
	}

	// Tier 0.5 → sentinel only (no models with tier <= 0.5).
	opts05 := CopilotModelOptionsUpTo(0.5)
	if len(opts05) != 1 {
		t.Errorf("CopilotModelOptionsUpTo(0.5) returned %d options, want 1 (sentinel only)", len(opts05))
	}
	if opts05[0].Value != ModelInheritOption {
		t.Errorf("CopilotModelOptionsUpTo(0.5): first option = %q, want sentinel %q", opts05[0].Value, ModelInheritOption)
	}
}

// TestCopilotTierForModel verifies the tier lookup by display name.
func TestCopilotTierForModel(t *testing.T) {
	tests := []struct {
		displayName string
		wantTier    float64
	}{
		{"Claude Haiku 4.5", 1.0},
		{"Claude Sonnet 4.6", 2.0},
		{"Claude Opus 4.7", 3.0},
		{"GPT-5 mini", 1.0},
		{"", math.MaxFloat64},
		{ModelInheritOption, math.MaxFloat64},
		{"unknown-model", math.MaxFloat64},
	}

	for _, tt := range tests {
		t.Run(tt.displayName, func(t *testing.T) {
			got := CopilotTierForModel(tt.displayName)
			if got != tt.wantTier {
				t.Errorf("CopilotTierForModel(%q) = %v, want %v", tt.displayName, got, tt.wantTier)
			}
		})
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
