// SPDX-License-Identifier: MIT

package recommend

import (
	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/model"
)

// frontendFrameworks is the set of framework names (from detect.inferFrameworks)
// that indicate a frontend-dominant project.
var frontendFrameworks = map[string]bool{
	"React": true, "Vue.js": true, "Angular": true,
	"Svelte": true, "Next.js": true, "Nuxt.js": true,
	"Astro": true, "Remix": true,
}

// backendLanguages is the set of primary language names that indicate
// a backend-dominant project when no frontend framework is detected.
var backendLanguages = map[string]bool{
	"Go": true, "Java": true, "Python": true,
	"Rust": true, "Kotlin": true, "Ruby": true, "PHP": true,
}

// DetectProjectScope returns the set of scope tags inferred from the project profile.
// Today the function returns a 0- or 1-element slice, but callers MUST NOT assume
// len(scope) <= 1 — the []string type is intentional for forward-compatibility with
// multi-tag profiles (e.g. a Go service with a React frontend → ["frontend","backend"]).
//
//   - Returns []string{"frontend"} when a frontend framework is detected.
//   - Returns []string{"backend"} when the dominant language is backend-only.
//   - Returns nil when the profile is nil or the project type cannot be determined.
//     nil means "unknown" — callers that receive nil MUST treat it as "include all advisors".
func DetectProjectScope(profile *detect.ProjectProfile) []string {
	if profile == nil {
		return nil
	}

	// Check frameworks first: if any frontend framework is detected, classify as frontend.
	for _, fw := range profile.Frameworks {
		if frontendFrameworks[fw] {
			return []string{model.AdvisorScopeFrontend}
		}
	}

	// Find primary language (highest percentage).
	var primaryLang string
	var maxPct float64
	for _, lang := range profile.Languages {
		if lang.Percentage > maxPct {
			maxPct = lang.Percentage
			primaryLang = lang.Name
		}
	}

	if backendLanguages[primaryLang] {
		return []string{model.AdvisorScopeBackend}
	}

	return nil
}

// ScopesIntersect reports whether slices a and b share at least one element.
// Both slices are treated as sets — duplicate elements within a or b do not
// affect the result. Empty slices always return false.
func ScopesIntersect(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// Build a lookup set from b (typically the shorter of the two in practice).
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	for _, v := range a {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

// FilterAdvisersByProfile returns the subset of advisors relevant for profile.
//
// Inclusion rule: an advisor is included when
//
//	len(advisor.Scope) == 0 (universal — applies to every project)
//	OR intersect(advisor.Scope, projectScope) != ∅
//
// Edge cases:
//   - nil/empty project scope (unknown project type) → return advisors unchanged.
//   - nil advisors → return nil (CONTRACT: nil-in/nil-out; never an empty slice).
func FilterAdvisersByProfile(profile *detect.ProjectProfile, advisors []model.AdvisorDef) []model.AdvisorDef {
	if advisors == nil {
		return nil
	}

	projectScope := DetectProjectScope(profile)
	if len(projectScope) == 0 {
		return advisors
	}

	out := make([]model.AdvisorDef, 0, len(advisors))
	for _, a := range advisors {
		if len(a.Scope) == 0 || ScopesIntersect(a.Scope, projectScope) {
			out = append(out, a)
		}
	}
	return out
}
