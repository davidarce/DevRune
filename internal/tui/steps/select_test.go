// SPDX-License-Identifier: MIT

package steps

import (
	"testing"

	"github.com/davidarce/devrune/internal/recommend"
)

// ── BuildSkillsShInput ────────────────────────────────────────────────────────

func TestBuildSkillsShInput_EmptyDetected(t *testing.T) {
	got := BuildSkillsShInput(nil)
	if got != nil {
		t.Errorf("BuildSkillsShInput(nil): want nil, got %+v", got)
	}

	got = BuildSkillsShInput([]recommend.DetectedTech{})
	if got != nil {
		t.Errorf("BuildSkillsShInput([]): want nil, got %+v", got)
	}
}

func TestBuildSkillsShInput_TechWithNoSkillsReturnedNil(t *testing.T) {
	detected := []recommend.DetectedTech{
		{Framework: "React", Skills: nil},
	}
	got := BuildSkillsShInput(detected)
	if got != nil {
		t.Errorf("BuildSkillsShInput with no skills: want nil, got %+v", got)
	}
}

func TestBuildSkillsShInput_SingleTechSingleSkill(t *testing.T) {
	detected := []recommend.DetectedTech{
		{
			Framework: "React",
			Skills: []recommend.SkillRef{
				{Path: "vercel-labs/agent-skills/vercel-react-best-practices", Description: "React best practices"},
			},
		},
	}

	got := BuildSkillsShInput(detected)
	if got == nil {
		t.Fatal("BuildSkillsShInput: want non-nil result, got nil")
	}

	if got.Source != "Skills.sh Curated" {
		t.Errorf("Source: want %q, got %q", "Skills.sh Curated", got.Source)
	}

	// Items: header "React" + skill "vercel-react-best-practices"
	if len(got.Skills) != 2 {
		t.Fatalf("Skills length: want 2, got %d: %v", len(got.Skills), got.Skills)
	}
	if got.Skills[0] != "React" {
		t.Errorf("Skills[0]: want %q (header), got %q", "React", got.Skills[0])
	}
	if got.Skills[1] != "vercel-react-best-practices" {
		t.Errorf("Skills[1]: want %q, got %q", "vercel-react-best-practices", got.Skills[1])
	}

	// Header map must mark the framework name as a header.
	if !got.SkillHeaders["React"] {
		t.Error("SkillHeaders[React]: want true (header label), got false")
	}
	if got.SkillHeaders["vercel-react-best-practices"] {
		t.Error("SkillHeaders[vercel-react-best-practices]: want false (selectable), got true")
	}

	// Description must be populated.
	if got.Descs["vercel-react-best-practices"] != "React best practices" {
		t.Errorf("Descs[vercel-react-best-practices]: want %q, got %q",
			"React best practices", got.Descs["vercel-react-best-practices"])
	}
}

func TestBuildSkillsShInput_MultipleTechs(t *testing.T) {
	detected := []recommend.DetectedTech{
		{
			Framework: "React",
			Skills: []recommend.SkillRef{
				{Path: "vercel-labs/agent-skills/vercel-react-best-practices", Description: "React"},
				{Path: "vercel-labs/agent-skills/vercel-composition-patterns", Description: "Composition"},
			},
		},
		{
			Framework: "Spring Boot",
			Skills: []recommend.SkillRef{
				{Path: "github/awesome-copilot/spring-boot-docs", Description: "Spring Boot docs"},
			},
		},
	}

	got := BuildSkillsShInput(detected)
	if got == nil {
		t.Fatal("BuildSkillsShInput: want non-nil result, got nil")
	}

	// Expected order: React (header), react-skill-1, react-skill-2, Spring Boot (header), spring-skill
	wantItems := []string{
		"React",
		"vercel-react-best-practices",
		"vercel-composition-patterns",
		"Spring Boot",
		"spring-boot-docs",
	}
	if len(got.Skills) != len(wantItems) {
		t.Fatalf("Skills length: want %d, got %d: %v", len(wantItems), len(got.Skills), got.Skills)
	}
	for i, want := range wantItems {
		if got.Skills[i] != want {
			t.Errorf("Skills[%d]: want %q, got %q", i, want, got.Skills[i])
		}
	}

	// Both tech names must be headers.
	if !got.SkillHeaders["React"] {
		t.Error("SkillHeaders[React]: want true")
	}
	if !got.SkillHeaders["Spring Boot"] {
		t.Error("SkillHeaders[Spring Boot]: want true")
	}
}

func TestBuildSkillsShInput_EmptyDescriptionNotAdded(t *testing.T) {
	detected := []recommend.DetectedTech{
		{
			Framework: "React",
			Skills: []recommend.SkillRef{
				{Path: "vercel-labs/agent-skills/vercel-react-best-practices", Description: ""},
			},
		},
	}

	got := BuildSkillsShInput(detected)
	if got == nil {
		t.Fatal("want non-nil result")
	}

	if _, ok := got.Descs["vercel-react-best-practices"]; ok {
		t.Error("Descs: want no entry for skill with empty description, but found one")
	}
}

func TestBuildSkillsShInput_TwoSegmentPath(t *testing.T) {
	detected := []recommend.DetectedTech{
		{
			Framework: "Misc",
			Skills: []recommend.SkillRef{
				{Path: "owner/repo", Description: "two-segment path"},
			},
		},
	}

	got := BuildSkillsShInput(detected)
	if got == nil {
		t.Fatal("want non-nil result")
	}

	// Short name of "owner/repo" is "repo".
	found := false
	for _, item := range got.Skills {
		if item == "repo" {
			found = true
		}
	}
	if !found {
		t.Errorf("want skill name %q in items %v", "repo", got.Skills)
	}
}
