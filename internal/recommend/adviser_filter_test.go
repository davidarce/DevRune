// SPDX-License-Identifier: MIT

package recommend

import (
	"testing"

	"github.com/davidarce/devrune/internal/detect"
)

func TestDetectProjectTier(t *testing.T) {
	tests := []struct {
		name    string
		profile *detect.ProjectProfile
		want    AdviserTier
	}{
		{
			name: "frontend project with React framework",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"React"},
				Languages:  []detect.LanguageInfo{{Name: "JavaScript", Percentage: 90}},
			},
			want: AdviserTierFrontend,
		},
		{
			name: "frontend project with Vue.js framework",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"Vue.js"},
				Languages:  []detect.LanguageInfo{{Name: "JavaScript", Percentage: 85}},
			},
			want: AdviserTierFrontend,
		},
		{
			name: "frontend project with Next.js framework",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"Next.js"},
				Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 95}},
			},
			want: AdviserTierFrontend,
		},
		{
			name: "backend project with Go as primary language",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
			},
			want: AdviserTierBackend,
		},
		{
			name: "backend project with Java as primary language",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Java", Percentage: 80}},
			},
			want: AdviserTierBackend,
		},
		{
			name: "backend project with Kotlin as primary language",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Kotlin", Percentage: 75}},
			},
			want: AdviserTierBackend,
		},
		{
			name: "mixed project React + Go — frontend check wins first",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"React"},
				Languages: []detect.LanguageInfo{
					{Name: "Go", Percentage: 60},
					{Name: "JavaScript", Percentage: 40},
				},
			},
			want: AdviserTierFrontend,
		},
		{
			name:    "nil profile",
			profile: nil,
			want:    "",
		},
		{
			name:    "empty profile no frameworks no languages",
			profile: &detect.ProjectProfile{},
			want:    "",
		},
		{
			name: "unknown language no frontend frameworks",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "COBOL", Percentage: 100}},
			},
			want: "",
		},
		{
			name: "multiple languages backend primary wins",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages: []detect.LanguageInfo{
					{Name: "Go", Percentage: 70},
					{Name: "Shell", Percentage: 30},
				},
			},
			want: AdviserTierBackend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectProjectTier(tt.profile)
			if got != tt.want {
				t.Errorf("detectProjectTier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterAdvisersByProfile(t *testing.T) {
	frontendProfile := &detect.ProjectProfile{
		Frameworks: []string{"React"},
		Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
	}
	backendProfile := &detect.ProjectProfile{
		Frameworks: []string{},
		Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
	}

	tests := []struct {
		name         string
		profile      *detect.ProjectProfile
		advisers     []string
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "frontend profile includes frontend and universal advisers",
			profile:      frontendProfile,
			advisers:     []string{"component-adviser", "api-first-adviser", "unit-test-adviser"},
			wantContains: []string{"component-adviser", "unit-test-adviser"},
			wantExcludes: []string{"api-first-adviser"},
		},
		{
			name:         "frontend profile excludes all backend advisers",
			profile:      frontendProfile,
			advisers:     []string{"architect-adviser", "integration-test-adviser", "frontend-test-adviser", "web-accessibility-adviser"},
			wantContains: []string{"frontend-test-adviser", "web-accessibility-adviser"},
			wantExcludes: []string{"architect-adviser", "integration-test-adviser"},
		},
		{
			name:         "backend profile includes backend and universal advisers",
			profile:      backendProfile,
			advisers:     []string{"architect-adviser", "frontend-test-adviser", "unit-test-adviser"},
			wantContains: []string{"architect-adviser", "unit-test-adviser"},
			wantExcludes: []string{"frontend-test-adviser"},
		},
		{
			name:         "backend profile excludes all frontend advisers",
			profile:      backendProfile,
			advisers:     []string{"component-adviser", "web-accessibility-adviser", "api-first-adviser", "integration-test-adviser"},
			wantContains: []string{"api-first-adviser", "integration-test-adviser"},
			wantExcludes: []string{"component-adviser", "web-accessibility-adviser"},
		},
		{
			name: "unknown profile returns all advisers unchanged",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "COBOL", Percentage: 100}},
			},
			advisers:     []string{"component-adviser", "api-first-adviser", "unit-test-adviser"},
			wantContains: []string{"component-adviser", "api-first-adviser", "unit-test-adviser"},
			wantExcludes: []string{},
		},
		{
			name:         "nil profile returns all advisers unchanged",
			profile:      nil,
			advisers:     []string{"component-adviser", "api-first-adviser", "unit-test-adviser"},
			wantContains: []string{"component-adviser", "api-first-adviser", "unit-test-adviser"},
			wantExcludes: []string{},
		},
		{
			name:         "unknown adviser name always included as universal default",
			profile:      frontendProfile,
			advisers:     []string{"component-adviser", "some-unknown-adviser", "api-first-adviser"},
			wantContains: []string{"component-adviser", "some-unknown-adviser"},
			wantExcludes: []string{"api-first-adviser"},
		},
		{
			name:         "empty adviser list returns empty slice",
			profile:      frontendProfile,
			advisers:     []string{},
			wantContains: []string{},
			wantExcludes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterAdvisersByProfile(tt.profile, tt.advisers)

			gotSet := make(map[string]bool, len(got))
			for _, name := range got {
				gotSet[name] = true
			}

			for _, want := range tt.wantContains {
				if !gotSet[want] {
					t.Errorf("FilterAdvisersByProfile() missing %q in result %v", want, got)
				}
			}

			for _, excluded := range tt.wantExcludes {
				if gotSet[excluded] {
					t.Errorf("FilterAdvisersByProfile() should not contain %q in result %v", excluded, got)
				}
			}
		})
	}
}
