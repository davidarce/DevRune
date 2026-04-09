// SPDX-License-Identifier: MIT

package recommend

import (
	"testing"

	"github.com/davidarce/devrune/internal/detect"
)

// ── T009: StaticCatalog.FetchByFrameworks and FetchByProfile ─────────────────

func TestStaticCatalog_FetchByFrameworks(t *testing.T) {
	catalog := StaticCatalog{}

	t.Run("single known framework returns its skills", func(t *testing.T) {
		got, err := catalog.FetchByFrameworks([]string{"React"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 DetectedTech, got %d", len(got))
		}
		if got[0].Framework != "React" {
			t.Errorf("want Framework=React, got %q", got[0].Framework)
		}
		if len(got[0].Skills) == 0 {
			t.Error("want at least one skill for React, got none")
		}
	})

	t.Run("multiple known frameworks returns skills for both", func(t *testing.T) {
		got, err := catalog.FetchByFrameworks([]string{"React", "Spring Boot"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 DetectedTech entries, got %d", len(got))
		}
		frameworks := make(map[string]bool)
		for _, dt := range got {
			frameworks[dt.Framework] = true
		}
		if !frameworks["React"] {
			t.Error("want React in result, not found")
		}
		if !frameworks["Spring Boot"] {
			t.Error("want Spring Boot in result, not found")
		}
	})

	t.Run("unknown framework returns empty slice with no error", func(t *testing.T) {
		got, err := catalog.FetchByFrameworks([]string{"UnknownFramework"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty slice, got %d entries", len(got))
		}
	})

	t.Run("empty frameworks slice returns empty slice with no error", func(t *testing.T) {
		got, err := catalog.FetchByFrameworks([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty slice, got %d entries", len(got))
		}
	})

	t.Run("mix of known and unknown frameworks returns only known", func(t *testing.T) {
		got, err := catalog.FetchByFrameworks([]string{"React", "UnknownFramework"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 DetectedTech, got %d", len(got))
		}
		if got[0].Framework != "React" {
			t.Errorf("want Framework=React, got %q", got[0].Framework)
		}
	})

	t.Run("results preserve registry order for stable TUI display", func(t *testing.T) {
		// Vue.js appears before Angular in SkillsRegistry; verify ordering holds.
		got, err := catalog.FetchByFrameworks([]string{"Angular", "Vue.js"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 DetectedTech entries, got %d", len(got))
		}
		if got[0].Framework != "Vue.js" {
			t.Errorf("want Vue.js first (registry order), got %q first", got[0].Framework)
		}
		if got[1].Framework != "Angular" {
			t.Errorf("want Angular second (registry order), got %q second", got[1].Framework)
		}
	})
}

func TestStaticCatalog_FetchByProfile(t *testing.T) {
	catalog := StaticCatalog{}

	t.Run("nil profile returns nil with no error", func(t *testing.T) {
		got, err := catalog.FetchByProfile(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("empty profile returns nil with no error", func(t *testing.T) {
		got, err := catalog.FetchByProfile(&detect.ProjectProfile{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("profile with React framework returns React skills", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{"React"},
		}
		got, err := catalog.FetchByProfile(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 DetectedTech, got %d", len(got))
		}
		if got[0].Framework != "React" {
			t.Errorf("want Framework=React, got %q", got[0].Framework)
		}
	})

	t.Run("profile with Java language returns Java skills", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Languages: []detect.LanguageInfo{
				{Name: "Java", Files: 10, Lines: 500, Percentage: 80},
			},
		}
		got, err := catalog.FetchByProfile(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 DetectedTech, got %d", len(got))
		}
		if got[0].Framework != "Java" {
			t.Errorf("want Framework=Java, got %q", got[0].Framework)
		}
	})

	t.Run("profile with React framework and Java language returns both", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{"React"},
			Languages: []detect.LanguageInfo{
				{Name: "Java", Files: 5, Lines: 200, Percentage: 30},
			},
		}
		got, err := catalog.FetchByProfile(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 DetectedTech entries, got %d", len(got))
		}
		frameworks := make(map[string]bool)
		for _, dt := range got {
			frameworks[dt.Framework] = true
		}
		if !frameworks["React"] {
			t.Error("want React in result, not found")
		}
		if !frameworks["Java"] {
			t.Error("want Java in result, not found")
		}
	})

	t.Run("profile with overlapping framework and language entry deduplicates", func(t *testing.T) {
		// "Java" appears as both a language entry AND can appear as a framework
		// if someone passes it in Frameworks. This test ensures no duplicate.
		profile := &detect.ProjectProfile{
			Frameworks: []string{"Java"},
			Languages: []detect.LanguageInfo{
				{Name: "Java", Files: 10, Lines: 500, Percentage: 90},
			},
		}
		got, err := catalog.FetchByProfile(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Java must appear at most once even though it matches both criteria.
		count := 0
		for _, dt := range got {
			if dt.Framework == "Java" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("want Java to appear exactly once (deduplication), got %d times", count)
		}
	})

	t.Run("results preserve registry order for stable TUI display", func(t *testing.T) {
		// In SkillsRegistry: React comes before Java.
		// Profile has both, so React must appear first.
		profile := &detect.ProjectProfile{
			Frameworks: []string{"React"},
			Languages: []detect.LanguageInfo{
				{Name: "Java", Files: 5, Lines: 200, Percentage: 20},
			},
		}
		got, err := catalog.FetchByProfile(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) < 2 {
			t.Fatalf("want at least 2 entries, got %d", len(got))
		}
		if got[0].Framework != "React" {
			t.Errorf("want React first (registry order), got %q", got[0].Framework)
		}
		if got[1].Framework != "Java" {
			t.Errorf("want Java second (registry order), got %q", got[1].Framework)
		}
	})
}

// ── T010: BuildSkillsCatalogItems ─────────────────────────────────────────────

func TestBuildSkillsCatalogItems(t *testing.T) {
	t.Run("empty slice returns nil", func(t *testing.T) {
		got := BuildSkillsCatalogItems(nil)
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("empty detected tech returns nil", func(t *testing.T) {
		got := BuildSkillsCatalogItems([]DetectedTech{})
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("single tech with one skill produces one CatalogItem", func(t *testing.T) {
		detected := []DetectedTech{
			{
				Framework: "React",
				Skills: []SkillRef{
					{
						Path:        "vercel-labs/agent-skills/vercel-react-best-practices",
						Description: "React best practices from Vercel",
					},
				},
			},
		}
		got := BuildSkillsCatalogItems(detected)
		if len(got) != 1 {
			t.Fatalf("want 1 CatalogItem, got %d", len(got))
		}
		item := got[0]
		if item.Name != "vercel-react-best-practices" {
			t.Errorf("Name: want %q, got %q", "vercel-react-best-practices", item.Name)
		}
		if item.Kind != "skill" {
			t.Errorf("Kind: want %q, got %q", "skill", item.Kind)
		}
		if item.Source != "vercel-labs/agent-skills/vercel-react-best-practices" {
			t.Errorf("Source: want full path, got %q", item.Source)
		}
		if item.Description != "React best practices from Vercel" {
			t.Errorf("Description: want %q, got %q", "React best practices from Vercel", item.Description)
		}
	})

	t.Run("multiple techs produce items in order", func(t *testing.T) {
		detected := []DetectedTech{
			{
				Framework: "React",
				Skills: []SkillRef{
					{Path: "vercel-labs/agent-skills/vercel-react-best-practices", Description: "React best practices"},
					{Path: "vercel-labs/agent-skills/vercel-composition-patterns", Description: "Composition patterns"},
				},
			},
			{
				Framework: "Java",
				Skills: []SkillRef{
					{Path: "github/awesome-copilot/java-docs", Description: "Java docs"},
				},
			},
		}
		got := BuildSkillsCatalogItems(detected)
		if len(got) != 3 {
			t.Fatalf("want 3 CatalogItems, got %d", len(got))
		}
		// First two from React, last from Java.
		if got[0].Name != "vercel-react-best-practices" {
			t.Errorf("item[0].Name: want vercel-react-best-practices, got %q", got[0].Name)
		}
		if got[1].Name != "vercel-composition-patterns" {
			t.Errorf("item[1].Name: want vercel-composition-patterns, got %q", got[1].Name)
		}
		if got[2].Name != "java-docs" {
			t.Errorf("item[2].Name: want java-docs, got %q", got[2].Name)
		}
	})

	t.Run("two-segment path uses repo name as skill name", func(t *testing.T) {
		detected := []DetectedTech{
			{
				Framework: "Misc",
				Skills: []SkillRef{
					{Path: "owner/repo", Description: "no skill segment"},
				},
			},
		}
		got := BuildSkillsCatalogItems(detected)
		if len(got) != 1 {
			t.Fatalf("want 1 CatalogItem, got %d", len(got))
		}
		if got[0].Name != "repo" {
			t.Errorf("Name: want %q, got %q", "repo", got[0].Name)
		}
		if got[0].Source != "owner/repo" {
			t.Errorf("Source: want full path %q, got %q", "owner/repo", got[0].Source)
		}
	})

	t.Run("all items have Kind=skill", func(t *testing.T) {
		detected := []DetectedTech{
			{
				Framework: "React",
				Skills: []SkillRef{
					{Path: "a/b/c", Description: "desc1"},
					{Path: "d/e/f", Description: "desc2"},
				},
			},
		}
		got := BuildSkillsCatalogItems(detected)
		for i, item := range got {
			if item.Kind != "skill" {
				t.Errorf("item[%d].Kind: want %q, got %q", i, "skill", item.Kind)
			}
		}
	})

	t.Run("Description is propagated correctly", func(t *testing.T) {
		desc := "A very specific description"
		detected := []DetectedTech{
			{
				Framework: "SomeTech",
				Skills: []SkillRef{
					{Path: "x/y/z", Description: desc},
				},
			},
		}
		got := BuildSkillsCatalogItems(detected)
		if len(got) != 1 {
			t.Fatalf("want 1 item, got %d", len(got))
		}
		if got[0].Description != desc {
			t.Errorf("Description: want %q, got %q", desc, got[0].Description)
		}
	})
}
