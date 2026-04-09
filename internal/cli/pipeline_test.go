// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ── expandSkillsShPackages ────────────────────────────────────────────────────

func TestExpandSkillsShPackages_EmptyInput(t *testing.T) {
	got := expandSkillsShPackages(nil)
	if len(got) != 0 {
		t.Errorf("expandSkillsShPackages(nil): want empty, got %d entries", len(got))
	}

	got = expandSkillsShPackages([]model.PackageRef{})
	if len(got) != 0 {
		t.Errorf("expandSkillsShPackages([]): want empty, got %d entries", len(got))
	}
}

func TestExpandSkillsShPackages_NonSentinelPassedThrough(t *testing.T) {
	packages := []model.PackageRef{
		{Source: "github:owner/repo"},
		{Source: "gitlab:other/pkg"},
	}

	got := expandSkillsShPackages(packages)
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Source != "github:owner/repo" {
		t.Errorf("got[0].Source: want %q, got %q", "github:owner/repo", got[0].Source)
	}
	if got[1].Source != "gitlab:other/pkg" {
		t.Errorf("got[1].Source: want %q, got %q", "gitlab:other/pkg", got[1].Source)
	}
}

func TestExpandSkillsShPackages_SentinelWithNoSelect(t *testing.T) {
	packages := []model.PackageRef{
		{Source: skillsShCuratedSource, Select: nil},
	}

	got := expandSkillsShPackages(packages)
	// Sentinel with no select should be dropped entirely.
	if len(got) != 0 {
		t.Errorf("sentinel with nil select: want 0 entries, got %d", len(got))
	}
}

func TestExpandSkillsShPackages_SentinelWithEmptySkills(t *testing.T) {
	packages := []model.PackageRef{
		{Source: skillsShCuratedSource, Select: &model.SelectFilter{Skills: []string{}}},
	}

	got := expandSkillsShPackages(packages)
	if len(got) != 0 {
		t.Errorf("sentinel with empty skills: want 0 entries, got %d", len(got))
	}
}

func TestExpandSkillsShPackages_KnownSkillExpanded(t *testing.T) {
	// "vercel-react-best-practices" is the short name of
	// "vercel-labs/agent-skills/vercel-react-best-practices" in the registry.
	packages := []model.PackageRef{
		{
			Source: skillsShCuratedSource,
			Select: &model.SelectFilter{
				Skills: []string{"vercel-react-best-practices"},
			},
		},
	}

	got := expandSkillsShPackages(packages)
	if len(got) != 1 {
		t.Fatalf("want 1 expanded PackageRef, got %d: %v", len(got), got)
	}

	// The expanded entry must reference the correct GitHub repo.
	if got[0].Source != "github:vercel-labs/agent-skills" {
		t.Errorf("Source: want %q, got %q", "github:vercel-labs/agent-skills", got[0].Source)
	}

	// The skill name must be preserved in the Select filter.
	if got[0].Select == nil {
		t.Fatal("Select: want non-nil, got nil")
	}
	if len(got[0].Select.Skills) != 1 || got[0].Select.Skills[0] != "vercel-react-best-practices" {
		t.Errorf("Select.Skills: want [vercel-react-best-practices], got %v", got[0].Select.Skills)
	}
}

func TestExpandSkillsShPackages_UnknownSkillSkipped(t *testing.T) {
	packages := []model.PackageRef{
		{
			Source: skillsShCuratedSource,
			Select: &model.SelectFilter{
				Skills: []string{"this-skill-does-not-exist-in-registry"},
			},
		},
	}

	got := expandSkillsShPackages(packages)
	if len(got) != 0 {
		t.Errorf("unknown skill: want 0 entries, got %d", len(got))
	}
}

func TestExpandSkillsShPackages_MultipleSkillsExpanded(t *testing.T) {
	// Two known skills from the React entry in SkillsRegistry.
	packages := []model.PackageRef{
		{
			Source: skillsShCuratedSource,
			Select: &model.SelectFilter{
				Skills: []string{
					"vercel-react-best-practices",
					"vercel-composition-patterns",
				},
			},
		},
	}

	got := expandSkillsShPackages(packages)
	if len(got) != 2 {
		t.Fatalf("want 2 expanded PackageRefs, got %d", len(got))
	}

	sources := map[string]bool{}
	for _, pkg := range got {
		sources[pkg.Source] = true
	}
	if !sources["github:vercel-labs/agent-skills"] {
		t.Error("want github:vercel-labs/agent-skills in expanded sources")
	}
}

func TestExpandSkillsShPackages_MixedSentinelAndNormal(t *testing.T) {
	packages := []model.PackageRef{
		{Source: "github:some/other-pkg"},
		{
			Source: skillsShCuratedSource,
			Select: &model.SelectFilter{
				Skills: []string{"vercel-react-best-practices"},
			},
		},
		{Source: "github:yet/another-pkg"},
	}

	got := expandSkillsShPackages(packages)
	// 2 normal packages + 1 expanded sentinel skill = 3 total
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d: %v", len(got), got)
	}
	if got[0].Source != "github:some/other-pkg" {
		t.Errorf("got[0].Source: want %q, got %q", "github:some/other-pkg", got[0].Source)
	}
	// got[1] is the expanded sentinel
	if got[1].Source != "github:vercel-labs/agent-skills" {
		t.Errorf("got[1].Source: want %q, got %q", "github:vercel-labs/agent-skills", got[1].Source)
	}
	if got[2].Source != "github:yet/another-pkg" {
		t.Errorf("got[2].Source: want %q, got %q", "github:yet/another-pkg", got[2].Source)
	}
}
