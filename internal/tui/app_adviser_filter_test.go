// SPDX-License-Identifier: MIT

package tui_test

import (
	"testing"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/recommend"
)

// TestAdviserFilterInScanPath verifies the adviser filtering contract used by the
// TUI scan path. Since tui.Run() requires a TTY, the integration contract is
// tested through the public recommend.FilterAdvisersByProfile() function with
// representative mock profiles.
func TestAdviserFilterInScanPath(t *testing.T) {
	allAdvisers := []string{
		"component-adviser",
		"web-accessibility-adviser",
		"frontend-test-adviser",
		"api-first-adviser",
		"architect-adviser",
		"integration-test-adviser",
		"unit-test-adviser",
	}

	t.Run("frontend project only shows frontend and universal advisers", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{"React"},
			Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
		}

		got := recommend.FilterAdvisersByProfile(profile, allAdvisers)
		gotSet := toSet(got)

		mustContain := []string{"component-adviser", "unit-test-adviser"}
		mustExclude := []string{"api-first-adviser"}

		for _, name := range mustContain {
			if !gotSet[name] {
				t.Errorf("expected %q in filtered result %v", name, got)
			}
		}
		for _, name := range mustExclude {
			if gotSet[name] {
				t.Errorf("did not expect %q in filtered result %v", name, got)
			}
		}
	})

	t.Run("backend project only shows backend and universal advisers", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{},
			Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
		}

		got := recommend.FilterAdvisersByProfile(profile, allAdvisers)
		gotSet := toSet(got)

		mustContain := []string{"api-first-adviser", "architect-adviser", "integration-test-adviser", "unit-test-adviser"}
		mustExclude := []string{"component-adviser", "web-accessibility-adviser", "frontend-test-adviser"}

		for _, name := range mustContain {
			if !gotSet[name] {
				t.Errorf("expected %q in filtered result %v", name, got)
			}
		}
		for _, name := range mustExclude {
			if gotSet[name] {
				t.Errorf("did not expect %q in filtered result %v", name, got)
			}
		}
	})

	t.Run("unknown project returns all advisers unchanged", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{},
			Languages:  []detect.LanguageInfo{{Name: "COBOL", Percentage: 100}},
		}

		got := recommend.FilterAdvisersByProfile(profile, allAdvisers)
		if len(got) != len(allAdvisers) {
			t.Errorf("expected all %d advisers returned for unknown project, got %d: %v", len(allAdvisers), len(got), got)
		}
	})
}

func toSet(names []string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[n] = true
	}
	return s
}
