// SPDX-License-Identifier: MIT

package steps_test

import (
	"testing"

	"github.com/davidarce/devrune/internal/recommend"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// makePrevSelection builds a SelectionResult for testing mergeRecommendationsIntoSelection.
func makePrevSelection(skills, rules []string) steps.SelectionResult {
	return steps.SelectionResult{
		Repos: []steps.RepoSelectionResult{
			{
				Source:         "github:owner/catalog",
				SelectedSkills: skills,
				SelectedRules:  rules,
			},
		},
	}
}

// TestMerge_AddsRecommendedItems verifies that recommended items above threshold
// are added to the existing selection.
func TestMerge_AddsRecommendedItems(t *testing.T) {
	prev := makePrevSelection([]string{"git-commit"}, nil)

	recs := []recommend.Recommendation{
		{Name: "architect-adviser", Kind: "skill", Confidence: 0.92},
		{Name: "unit-test-adviser", Kind: "skill", Confidence: 0.85},
	}

	merged := steps.MergeRecommendationsIntoSelection(prev, recs, 0.7)

	skills := merged.Repos[0].SelectedSkills
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills (1 user + 2 recommended), got %d: %v", len(skills), skills)
	}

	has := map[string]bool{}
	for _, s := range skills {
		has[s] = true
	}
	for _, expected := range []string{"git-commit", "architect-adviser", "unit-test-adviser"} {
		if !has[expected] {
			t.Errorf("expected %q in merged skills", expected)
		}
	}
}

// TestMerge_BelowThresholdNotAdded verifies items below threshold are NOT added.
func TestMerge_BelowThresholdNotAdded(t *testing.T) {
	prev := makePrevSelection(nil, nil)

	recs := []recommend.Recommendation{
		{Name: "architect-adviser", Kind: "skill", Confidence: 0.45},
	}

	merged := steps.MergeRecommendationsIntoSelection(prev, recs, 0.7)

	if len(merged.Repos[0].SelectedSkills) != 0 {
		t.Errorf("expected no skills added (below threshold), got %v", merged.Repos[0].SelectedSkills)
	}
}

// TestMerge_NoDuplicates verifies already-selected items are not duplicated.
func TestMerge_NoDuplicates(t *testing.T) {
	prev := makePrevSelection([]string{"architect-adviser"}, nil)

	recs := []recommend.Recommendation{
		{Name: "architect-adviser", Kind: "skill", Confidence: 0.92},
	}

	merged := steps.MergeRecommendationsIntoSelection(prev, recs, 0.7)

	if len(merged.Repos[0].SelectedSkills) != 1 {
		t.Errorf("expected 1 skill (no duplicate), got %d: %v", len(merged.Repos[0].SelectedSkills), merged.Repos[0].SelectedSkills)
	}
}

// TestMerge_PreservesOriginalSelection verifies user's original selection is not modified.
func TestMerge_PreservesOriginalSelection(t *testing.T) {
	prev := makePrevSelection([]string{"git-commit"}, []string{"clean-architecture"})

	recs := []recommend.Recommendation{
		{Name: "architect-adviser", Kind: "skill", Confidence: 0.92},
	}

	merged := steps.MergeRecommendationsIntoSelection(prev, recs, 0.7)

	// Original should be unchanged.
	if len(prev.Repos[0].SelectedSkills) != 1 || prev.Repos[0].SelectedSkills[0] != "git-commit" {
		t.Errorf("original selection was modified: %v", prev.Repos[0].SelectedSkills)
	}

	// Merged should have both.
	if len(merged.Repos[0].SelectedSkills) != 2 {
		t.Errorf("expected 2 skills in merged, got %v", merged.Repos[0].SelectedSkills)
	}
	if len(merged.Repos[0].SelectedRules) != 1 {
		t.Errorf("expected 1 rule preserved, got %v", merged.Repos[0].SelectedRules)
	}
}

// TestMerge_EmptyRecommendations returns selection unchanged.
func TestMerge_EmptyRecommendations(t *testing.T) {
	prev := makePrevSelection([]string{"git-commit"}, nil)

	merged := steps.MergeRecommendationsIntoSelection(prev, nil, 0.7)

	if len(merged.Repos[0].SelectedSkills) != 1 {
		t.Errorf("expected original 1 skill preserved, got %v", merged.Repos[0].SelectedSkills)
	}
}

// TestMerge_DefaultThreshold uses 0.7 when threshold is 0.
func TestMerge_DefaultThreshold(t *testing.T) {
	prev := makePrevSelection(nil, nil)

	recs := []recommend.Recommendation{
		{Name: "above", Kind: "skill", Confidence: 0.92},
		{Name: "below", Kind: "skill", Confidence: 0.5},
	}

	merged := steps.MergeRecommendationsIntoSelection(prev, recs, 0)

	skills := merged.Repos[0].SelectedSkills
	if len(skills) != 1 || skills[0] != "above" {
		t.Errorf("expected only 'above' with default threshold 0.7, got %v", skills)
	}
}
