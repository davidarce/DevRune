// SPDX-License-Identifier: MIT

package recommend

import "github.com/davidarce/devrune/internal/detect"

// AdviserTier classifies an adviser by its applicable project type.
type AdviserTier string

const (
	AdviserTierFrontend  AdviserTier = "frontend"
	AdviserTierBackend   AdviserTier = "backend"
	AdviserTierUniversal AdviserTier = "universal"
)

// AdviserClassifications maps known adviser skill names to their tier.
// Names not in this map default to AdviserTierUniversal (always shown).
var AdviserClassifications = map[string]AdviserTier{
	"component-adviser":         AdviserTierFrontend,
	"web-accessibility-adviser": AdviserTierFrontend,
	"frontend-test-adviser":     AdviserTierFrontend,
	"api-first-adviser":         AdviserTierBackend,
	"architect-adviser":         AdviserTierBackend,
	"integration-test-adviser":  AdviserTierBackend,
	"unit-test-adviser":         AdviserTierUniversal,
}

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

// detectProjectTier returns the dominant tier for the profile.
// Returns "" when the tier cannot be determined (mixed/unknown → show all).
func detectProjectTier(profile *detect.ProjectProfile) AdviserTier {
	if profile == nil {
		return ""
	}

	// Check frameworks first: if any frontend framework is detected, classify as frontend.
	for _, fw := range profile.Frameworks {
		if frontendFrameworks[fw] {
			return AdviserTierFrontend
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
		return AdviserTierBackend
	}

	return ""
}

// FilterAdvisersByProfile returns the subset of adviserNames relevant for profile.
//   - Frontend tier: frontend + universal advisers only
//   - Backend tier: backend + universal advisers only
//   - Unknown/mixed tier: returns all adviserNames unchanged
//   - nil profile: returns all adviserNames unchanged
func FilterAdvisersByProfile(profile *detect.ProjectProfile, adviserNames []string) []string {
	tier := detectProjectTier(profile)
	if tier == "" {
		return adviserNames
	}

	result := make([]string, 0, len(adviserNames))
	for _, name := range adviserNames {
		classification, known := AdviserClassifications[name]
		if !known || classification == AdviserTierUniversal || classification == tier {
			result = append(result, name)
		}
	}
	return result
}
