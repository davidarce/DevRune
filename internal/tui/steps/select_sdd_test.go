// SPDX-License-Identifier: MIT

package steps

import (
	"slices"
	"testing"
)

// ── filterOutStrings ──────────────────────────────────────────────────────────

func TestFilterOutStrings_SDDPartitioning(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		exclude []string
		want    []string
	}{
		{
			name:    "removes exact match",
			items:   []string{"sdd", "react-patterns"},
			exclude: []string{"sdd"},
			want:    []string{"react-patterns"},
		},
		{
			name:    "removes sdd-orchestrator",
			items:   []string{"sdd-orchestrator", "react-patterns"},
			exclude: []string{"sdd-orchestrator"},
			want:    []string{"react-patterns"},
		},
		{
			name:    "case-sensitive: SDD Workflow kept if exclude uses lowercase",
			items:   []string{"SDD Workflow", "react-patterns"},
			exclude: []string{"sdd workflow"},
			want:    []string{"SDD Workflow", "react-patterns"},
		},
		{
			name:    "case-sensitive: SDD Workflow removed when casing matches",
			items:   []string{"SDD Workflow", "react-patterns"},
			exclude: []string{"SDD Workflow"},
			want:    []string{"react-patterns"},
		},
		{
			name:    "empty input produces empty result",
			items:   nil,
			exclude: []string{"sdd"},
			want:    []string{},
		},
		{
			name:    "empty exclude returns original slice",
			items:   []string{"sdd", "react-patterns"},
			exclude: nil,
			want:    []string{"sdd", "react-patterns"},
		},
		{
			name:    "react-patterns kept when only sdd excluded",
			items:   []string{"sdd", "react-patterns"},
			exclude: []string{"sdd"},
			want:    []string{"react-patterns"},
		},
		{
			name:    "both empty produces empty result",
			items:   nil,
			exclude: nil,
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filterOutStrings(tc.items, tc.exclude)
			// Nil and empty-slice are treated as equivalent here.
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("filterOutStrings(%v, %v) = %v; want %v",
					tc.items, tc.exclude, got, tc.want)
			}
		})
	}
}

// ── NewSelectModel: auto-selected workflows not shown in TUI ──────────────────

func TestAutoSelectedWorkflowsNotInTUI(t *testing.T) {
	input := ScannedRepoInput{
		Source:                "test-catalog",
		Workflows:             []string{"sdd-orchestrator", "react-patterns"},
		AutoSelectedWorkflows: []string{"sdd-orchestrator"},
	}

	m := NewSelectModel([]ScannedRepoInput{input})

	wfCat := m.repos[0].Categories[3] // index 3 = Workflows

	for _, item := range wfCat.Items {
		if item == "sdd-orchestrator" {
			t.Error("Workflows category Items should NOT contain 'sdd-orchestrator' (auto-selected)")
		}
	}

	found := false
	for _, item := range wfCat.Items {
		if item == "react-patterns" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Workflows category Items SHOULD contain 'react-patterns'")
	}
}

func TestAutoSelectedWorkflowsStoredInRepo(t *testing.T) {
	input := ScannedRepoInput{
		Source:                "test-catalog",
		Workflows:             []string{"sdd-orchestrator", "react-patterns"},
		AutoSelectedWorkflows: []string{"sdd-orchestrator"},
	}

	m := NewSelectModel([]ScannedRepoInput{input})

	repo := m.repos[0]
	if len(repo.autoSelected) != 1 || repo.autoSelected[0] != "sdd-orchestrator" {
		t.Errorf("autoSelected: want [sdd-orchestrator], got %v", repo.autoSelected)
	}
}

// ── Result: auto-selected workflows injected into SelectedWorkflows ───────────

func TestAutoSelectedWorkflowsInResult(t *testing.T) {
	input := ScannedRepoInput{
		Source:                "test-catalog",
		Workflows:             []string{"sdd-orchestrator", "react-patterns"},
		AutoSelectedWorkflows: []string{"sdd-orchestrator"},
	}

	m := NewSelectModel([]ScannedRepoInput{input})

	// Select "react-patterns" manually so it also appears in the result.
	wfCat := &m.repos[0].Categories[3]
	wfCat.IsOn = true
	for _, item := range wfCat.Items {
		wfCat.Selected[item] = true
	}

	result := m.Result()

	if len(result.Repos) == 0 {
		t.Fatal("Result has no repos")
	}

	selected := result.Repos[0].SelectedWorkflows

	containsSDD := slices.Contains(selected, "sdd-orchestrator")
	if !containsSDD {
		t.Errorf("SelectedWorkflows should contain 'sdd-orchestrator'; got %v", selected)
	}

	containsReact := slices.Contains(selected, "react-patterns")
	if !containsReact {
		t.Errorf("SelectedWorkflows should contain 'react-patterns'; got %v", selected)
	}
}

func TestAutoSelectedWorkflowsInResultEvenWhenWFCategoryOff(t *testing.T) {
	// Even if the Workflows category toggle is OFF (user deselected all),
	// auto-selected workflows must still appear in the result.
	input := ScannedRepoInput{
		Source:                "test-catalog",
		Workflows:             []string{"sdd-orchestrator", "react-patterns"},
		AutoSelectedWorkflows: []string{"sdd-orchestrator"},
	}

	m := NewSelectModel([]ScannedRepoInput{input})

	// Explicitly leave category IsOn = false (default from buildCategoryWithDescs).
	wfCat := &m.repos[0].Categories[3]
	wfCat.IsOn = false

	result := m.Result()
	selected := result.Repos[0].SelectedWorkflows

	containsSDD := slices.Contains(selected, "sdd-orchestrator")
	if !containsSDD {
		t.Errorf("SelectedWorkflows must contain auto-selected 'sdd-orchestrator' even when category is off; got %v", selected)
	}
}

func TestNoAutoSelectedWorkflowsProducesEmptyAutoSlice(t *testing.T) {
	input := ScannedRepoInput{
		Source:    "test-catalog",
		Workflows: []string{"react-patterns"},
		// AutoSelectedWorkflows is nil
	}

	m := NewSelectModel([]ScannedRepoInput{input})

	if len(m.repos[0].autoSelected) != 0 {
		t.Errorf("autoSelected: want empty, got %v", m.repos[0].autoSelected)
	}
}
