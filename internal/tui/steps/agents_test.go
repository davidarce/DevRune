// SPDX-License-Identifier: MIT

package steps

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Tests for SelectAgents preselection logic (T020)
// ---------------------------------------------------------------------------

// buildPreSet is a helper that mirrors the internal preSet construction
// in SelectAgents, allowing direct unit tests of the preselection logic
// without invoking the interactive bubbletea form.
func buildPreSet(preselected []string) map[string]bool {
	preSet := make(map[string]bool, len(preselected))
	for _, p := range preselected {
		preSet[p] = true
	}
	return preSet
}

// TestSelectAgents_PreselectedMatchingAgentsAreMarked verifies that agents
// whose names appear in the preselected slice would be marked as selected
// (i.e. preSet[name] == true) when building the option list.
func TestSelectAgents_PreselectedMatchingAgentsAreMarked(t *testing.T) {
	preselected := []string{"claude", "opencode"}
	preSet := buildPreSet(preselected)

	for _, agent := range knownAgents {
		wantSelected := agent == "claude" || agent == "opencode"
		if preSet[agent] != wantSelected {
			t.Errorf("preSet[%q] = %v, want %v", agent, preSet[agent], wantSelected)
		}
	}
}

// TestSelectAgents_PreselectedValuesNotInKnownAgentsAreDropped verifies that
// values in preselected that are NOT in knownAgents are silently ignored
// (smart merge: preselect only what still exists).
func TestSelectAgents_PreselectedValuesNotInKnownAgentsAreDropped(t *testing.T) {
	// "legacy-agent" and "old-bot" no longer exist in knownAgents.
	preselected := []string{"claude", "legacy-agent", "old-bot"}
	preSet := buildPreSet(preselected)

	// "claude" is a known agent and should be preselected.
	if !preSet["claude"] {
		t.Error("expected preSet[claude] to be true")
	}

	// Non-existent agents result in a truthy preSet entry but will not match
	// any option in the form because knownAgents does not include them.
	// Verify knownAgents does not contain the stale entries.
	knownSet := make(map[string]bool, len(knownAgents))
	for _, a := range knownAgents {
		knownSet[a] = true
	}

	for _, stale := range []string{"legacy-agent", "old-bot"} {
		if knownSet[stale] {
			t.Errorf("stale agent %q should not be in knownAgents", stale)
		}
		// The combined effect: preSet[stale] is true but no option with that
		// value exists in the form, so it is silently dropped.
	}
}

// TestSelectAgents_NilPreselectedProducesNoSelections verifies that passing
// nil for preselected results in no agents being pre-checked.
func TestSelectAgents_NilPreselectedProducesNoSelections(t *testing.T) {
	preSet := buildPreSet(nil)

	for _, agent := range knownAgents {
		if preSet[agent] {
			t.Errorf("preSet[%q] = true with nil preselected, want false", agent)
		}
	}
}

// TestSelectAgents_AllKnownAgentsCanBePreselected verifies that all agents
// in knownAgents can simultaneously be marked as preselected.
func TestSelectAgents_AllKnownAgentsCanBePreselected(t *testing.T) {
	preSet := buildPreSet(knownAgents)

	for _, agent := range knownAgents {
		if !preSet[agent] {
			t.Errorf("preSet[%q] = false, want true when all agents are preselected", agent)
		}
	}
}

// TestKnownAgents_ContainsExpectedValues verifies the knownAgents list
// contains the expected AI agent names.
func TestKnownAgents_ContainsExpectedValues(t *testing.T) {
	expected := []string{"claude", "codex", "opencode", "copilot", "factory"}
	if len(knownAgents) != len(expected) {
		t.Fatalf("len(knownAgents) = %d, want %d", len(knownAgents), len(expected))
	}
	knownSet := make(map[string]bool, len(knownAgents))
	for _, a := range knownAgents {
		knownSet[a] = true
	}
	for _, e := range expected {
		if !knownSet[e] {
			t.Errorf("knownAgents does not contain %q", e)
		}
	}
}
