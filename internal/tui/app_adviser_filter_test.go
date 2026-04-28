// SPDX-License-Identifier: MIT

package tui_test

import (
	"testing"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/recommend"
)

// advisorFixture is a minimal local builder for model.AdvisorDef used in this
// test package. It mirrors the pattern in internal/recommend/advisor_filter_test.go
// to avoid a circular import (tui_test → cli → ...).
func advisorFixture(name string, scope ...string) model.AdvisorDef {
	def := model.AdvisorDef{
		Name:        name,
		Description: "test fixture",
	}
	if len(scope) > 0 {
		def.Scope = scope
	}
	return def
}

// TestAdviserFilterInScanPath verifies the adviser filtering contract used by the
// TUI scan path. Since tui.Run() requires a TTY, the integration contract is
// tested through the public recommend.FilterAdvisersByProfile() function with
// representative mock profiles.
//
// Scope assignments reflect the canonical advisor vocabulary:
//   - component-advisor:          frontend
//   - web-accessibility-advisor:  frontend, accessibility
//   - frontend-test-advisor:      frontend, testing
//   - api-first-advisor:          backend, api
//   - architect-advisor:          backend, architecture
//   - integration-test-advisor:   backend, testing
//   - unit-test-advisor:          (nil) universal — applies to every project
func TestAdviserFilterInScanPath(t *testing.T) {
	allAdvisers := []model.AdvisorDef{
		advisorFixture("component-advisor", model.AdvisorScopeFrontend),
		advisorFixture("web-accessibility-advisor", model.AdvisorScopeFrontend, model.AdvisorScopeAccessibility),
		advisorFixture("frontend-test-advisor", model.AdvisorScopeFrontend, model.AdvisorScopeTesting),
		advisorFixture("api-first-advisor", model.AdvisorScopeBackend, model.AdvisorScopeAPI),
		advisorFixture("architect-advisor", model.AdvisorScopeBackend, model.AdvisorScopeArchitecture),
		advisorFixture("integration-test-advisor", model.AdvisorScopeBackend, model.AdvisorScopeTesting),
		advisorFixture("unit-test-advisor"), // universal — empty scope
	}

	t.Run("frontend project only shows frontend and universal advisers", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{"React"},
			Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
		}

		got := recommend.FilterAdvisersByProfile(profile, allAdvisers)
		gotSet := toAdvisorNameSet(got)

		mustContain := []string{"component-advisor", "unit-test-advisor"}
		mustExclude := []string{"api-first-advisor"}

		for _, name := range mustContain {
			if !gotSet[name] {
				t.Errorf("expected %q in filtered result %v", name, advisorNames(got))
			}
		}
		for _, name := range mustExclude {
			if gotSet[name] {
				t.Errorf("did not expect %q in filtered result %v", name, advisorNames(got))
			}
		}
	})

	t.Run("backend project only shows backend and universal advisers", func(t *testing.T) {
		profile := &detect.ProjectProfile{
			Frameworks: []string{},
			Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
		}

		got := recommend.FilterAdvisersByProfile(profile, allAdvisers)
		gotSet := toAdvisorNameSet(got)

		mustContain := []string{"api-first-advisor", "architect-advisor", "integration-test-advisor", "unit-test-advisor"}
		mustExclude := []string{"component-advisor", "web-accessibility-advisor", "frontend-test-advisor"}

		for _, name := range mustContain {
			if !gotSet[name] {
				t.Errorf("expected %q in filtered result %v", name, advisorNames(got))
			}
		}
		for _, name := range mustExclude {
			if gotSet[name] {
				t.Errorf("did not expect %q in filtered result %v", name, advisorNames(got))
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
			t.Errorf("expected all %d advisers returned for unknown project, got %d: %v", len(allAdvisers), len(got), advisorNames(got))
		}
	})
}

// toAdvisorNameSet converts a []model.AdvisorDef slice to a name→bool lookup set.
func toAdvisorNameSet(defs []model.AdvisorDef) map[string]bool {
	s := make(map[string]bool, len(defs))
	for _, d := range defs {
		s[d.Name] = true
	}
	return s
}

// advisorNames extracts advisor names for readable error output.
func advisorNames(defs []model.AdvisorDef) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}
