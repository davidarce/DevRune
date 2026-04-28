// SPDX-License-Identifier: MIT

package recommend

import (
	"testing"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test-only builder — mirrors cli.AnAdvisorDef() but lives here to avoid
// the circular import cli → recommend → cli.
// ─────────────────────────────────────────────────────────────────────────────

type advisorDefBuilder struct {
	def model.AdvisorDef
}

func anAdvisorDef() *advisorDefBuilder {
	return &advisorDefBuilder{
		def: model.AdvisorDef{
			Name:        "test-advisor",
			Description: "A test advisor",
		},
	}
}

func (b *advisorDefBuilder) named(name string) *advisorDefBuilder {
	b.def.Name = name
	return b
}

func (b *advisorDefBuilder) withScope(scope ...string) *advisorDefBuilder {
	if len(scope) == 0 {
		b.def.Scope = nil
		return b
	}
	b.def.Scope = scope
	return b
}

func (b *advisorDefBuilder) build() model.AdvisorDef {
	return b.def
}

// ─────────────────────────────────────────────────────────────────────────────
// TestScopesIntersect — 2-axis table: a × b → expected bool
// ─────────────────────────────────────────────────────────────────────────────

func TestScopesIntersect(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "nil nil — both empty, no intersection",
			a:    nil,
			b:    nil,
			want: false,
		},
		{
			name: "empty empty — both empty, no intersection",
			a:    []string{},
			b:    []string{},
			want: false,
		},
		{
			name: "non-empty nil — b is nil, no intersection",
			a:    []string{"x"},
			b:    nil,
			want: false,
		},
		{
			name: "nil non-empty — a is nil, no intersection",
			a:    nil,
			b:    []string{"x"},
			want: false,
		},
		{
			name: "single no match — disjoint singletons",
			a:    []string{"x"},
			b:    []string{"y"},
			want: false,
		},
		{
			name: "single match — identical singletons",
			a:    []string{"x"},
			b:    []string{"x"},
			want: true,
		},
		{
			name: "multi partial overlap — one shared element",
			a:    []string{"x", "y"},
			b:    []string{"y", "z"},
			want: true,
		},
		{
			name: "multi full overlap — identical slices",
			a:    []string{"x", "y"},
			b:    []string{"x", "y"},
			want: true,
		},
		{
			name: "multi disjoint — no shared element",
			a:    []string{"x", "y"},
			b:    []string{"z", "w"},
			want: false,
		},
		{
			name: "duplicates in a — set semantics, still intersects",
			a:    []string{"x", "x"},
			b:    []string{"x"},
			want: true,
		},
		{
			name: "duplicates in b — set semantics, still intersects",
			a:    []string{"x"},
			b:    []string{"x", "x"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScopesIntersect(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("ScopesIntersect(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectProjectScope — covers frontend, backend, and nil/empty profiles.
// CONTRACT: returns []string, callers MUST NOT assume len <= 1.
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectProjectScope(t *testing.T) {
	tests := []struct {
		name    string
		profile *detect.ProjectProfile
		want    []string
	}{
		{
			name: "React frontend framework detected — scope is [frontend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"React"},
				Languages:  []detect.LanguageInfo{{Name: "JavaScript", Percentage: 90}},
			},
			want: []string{model.AdvisorScopeFrontend},
		},
		{
			name: "Vue.js frontend framework detected — scope is [frontend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"Vue.js"},
				Languages:  []detect.LanguageInfo{{Name: "JavaScript", Percentage: 85}},
			},
			want: []string{model.AdvisorScopeFrontend},
		},
		{
			name: "Next.js frontend framework detected — scope is [frontend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"Next.js"},
				Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 95}},
			},
			want: []string{model.AdvisorScopeFrontend},
		},
		{
			name: "Go backend language — scope is [backend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
			},
			want: []string{model.AdvisorScopeBackend},
		},
		{
			name: "Java backend language — scope is [backend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Java", Percentage: 80}},
			},
			want: []string{model.AdvisorScopeBackend},
		},
		{
			name: "Kotlin backend language — scope is [backend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "Kotlin", Percentage: 75}},
			},
			want: []string{model.AdvisorScopeBackend},
		},
		{
			name: "React + Go mixed — frontend check wins first, scope is [frontend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{"React"},
				Languages: []detect.LanguageInfo{
					{Name: "Go", Percentage: 60},
					{Name: "JavaScript", Percentage: 40},
				},
			},
			want: []string{model.AdvisorScopeFrontend},
		},
		{
			name: "nil profile — unknown project, scope is nil (not empty slice)",
			profile: nil,
			want:    nil,
		},
		{
			name: "empty profile no frameworks no languages — scope is nil",
			profile: &detect.ProjectProfile{},
			want:    nil,
		},
		{
			name: "unknown language no frontend frameworks — scope is nil",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages:  []detect.LanguageInfo{{Name: "COBOL", Percentage: 100}},
			},
			want: nil,
		},
		{
			name: "multiple languages backend primary wins — scope is [backend]",
			profile: &detect.ProjectProfile{
				Frameworks: []string{},
				Languages: []detect.LanguageInfo{
					{Name: "Go", Percentage: 70},
					{Name: "Shell", Percentage: 30},
				},
			},
			want: []string{model.AdvisorScopeBackend},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectProjectScope(tt.profile)

			// Explicit nil vs empty-slice check: contract requires nil, never [].
			if tt.want == nil {
				if got != nil {
					t.Errorf("DetectProjectScope() = %v (len=%d), want nil — callers treat nil as 'unknown/include all'", got, len(got))
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("DetectProjectScope() = %v (len=%d), want %v (len=%d)", got, len(got), tt.want, len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("DetectProjectScope()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFilterAdvisersByProfile_NilAdvisorsReturnsNil — CONTRACT pin.
// FilterAdvisersByProfile(profile, nil) MUST return nil, never an empty slice.
// ─────────────────────────────────────────────────────────────────────────────

func TestFilterAdvisersByProfile_NilAdvisorsReturnsNil(t *testing.T) {
	frontendProfile := &detect.ProjectProfile{
		Frameworks: []string{"React"},
		Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
	}

	got := FilterAdvisersByProfile(frontendProfile, nil)
	if got != nil {
		t.Fatalf("FilterAdvisersByProfile(profile, nil) = %v (len=%d), want nil — CONTRACT: nil-in → nil-out, never an empty slice", got, len(got))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFilterAdvisersByProfile_Matrix — 2-axis matrix: advisor.scope × project.scope
// ─────────────────────────────────────────────────────────────────────────────

func TestFilterAdvisersByProfile_Matrix(t *testing.T) {
	// Helper profiles for each scenario.
	frontendProfile := &detect.ProjectProfile{
		Frameworks: []string{"React"},
		Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
	}
	backendProfile := &detect.ProjectProfile{
		Frameworks: []string{},
		Languages:  []detect.LanguageInfo{{Name: "Go", Percentage: 95}},
	}
	multiBackendProfile := &detect.ProjectProfile{
		Frameworks: []string{},
		Languages:  []detect.LanguageInfo{{Name: "Java", Percentage: 70}, {Name: "Go", Percentage: 30}},
	}
	unknownProfile := &detect.ProjectProfile{
		Frameworks: []string{},
		Languages:  []detect.LanguageInfo{{Name: "COBOL", Percentage: 100}},
	}

	tests := []struct {
		name            string
		advisorScope    []string // scope of the single advisor under test
		profile         *detect.ProjectProfile
		wantIncluded    bool
		note            string
	}{
		// Nil project scope (unknown) — return input unchanged regardless of advisor scope.
		{
			name:         "universal advisor nil project scope — included (nil project → pass-through)",
			advisorScope: nil,
			profile:      nil,
			wantIncluded: true,
			note:         "nil profile → DetectProjectScope returns nil → return advisors unchanged",
		},
		{
			name:         "scoped advisor nil project scope — included (nil project → pass-through)",
			advisorScope: []string{model.AdvisorScopeFrontend},
			profile:      nil,
			wantIncluded: true,
			note:         "nil project means unknown → no filtering applied, all advisors returned",
		},
		{
			name:         "universal advisor unknown project scope — included (unknown project → pass-through)",
			advisorScope: nil,
			profile:      unknownProfile,
			wantIncluded: true,
			note:         "unknown project (COBOL) → DetectProjectScope returns nil → no filtering",
		},

		// Universal advisor (empty scope) — always included when project scope is known.
		{
			name:         "universal advisor frontend project — included",
			advisorScope: nil,
			profile:      frontendProfile,
			wantIncluded: true,
		},
		{
			name:         "universal advisor backend project — included",
			advisorScope: nil,
			profile:      backendProfile,
			wantIncluded: true,
		},

		// Single-scope advisor × single project scope.
		{
			name:         "frontend advisor frontend project — included (match)",
			advisorScope: []string{model.AdvisorScopeFrontend},
			profile:      frontendProfile,
			wantIncluded: true,
		},
		{
			name:         "frontend advisor backend project — excluded (no match)",
			advisorScope: []string{model.AdvisorScopeFrontend},
			profile:      backendProfile,
			wantIncluded: false,
		},
		{
			name:         "backend advisor backend project — included (match)",
			advisorScope: []string{model.AdvisorScopeBackend},
			profile:      backendProfile,
			wantIncluded: true,
		},
		{
			name:         "backend advisor frontend project — excluded (no match)",
			advisorScope: []string{model.AdvisorScopeBackend},
			profile:      frontendProfile,
			wantIncluded: false,
		},

		// Multi-scope advisor × single project scope.
		{
			name:         "frontend+testing advisor frontend project — included (frontend matches)",
			advisorScope: []string{model.AdvisorScopeFrontend, model.AdvisorScopeTesting},
			profile:      frontendProfile,
			wantIncluded: true,
		},
		{
			name:         "frontend+testing advisor backend project — excluded (no match)",
			advisorScope: []string{model.AdvisorScopeFrontend, model.AdvisorScopeTesting},
			profile:      backendProfile,
			wantIncluded: false,
		},
		{
			name:         "frontend+testing advisor testing project not detected — excluded when only backend detected",
			advisorScope: []string{model.AdvisorScopeFrontend, model.AdvisorScopeTesting},
			profile:      backendProfile,
			wantIncluded: false,
			note:         "DetectProjectScope returns [backend] for Go — testing is not a detected scope tag",
		},

		// Multi-scope advisor × multi project scope (future-compatibility case).
		{
			name:         "backend+testing advisor frontend+backend multi project — included (backend matches)",
			advisorScope: []string{model.AdvisorScopeBackend, model.AdvisorScopeTesting},
			profile:      multiBackendProfile,
			wantIncluded: true,
			note:         "Java primary → [backend]; backend intersects advisor.scope",
		},
		{
			name:         "frontend+architecture advisor backend project — excluded (disjoint)",
			advisorScope: []string{model.AdvisorScopeFrontend, model.AdvisorScopeArchitecture},
			profile:      backendProfile,
			wantIncluded: false,
			note:         "Go project → [backend]; advisor has [frontend,architecture] — no intersection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			advisor := anAdvisorDef().named("subject-advisor").withScope(tt.advisorScope...).build()
			advisors := []model.AdvisorDef{advisor}

			got := FilterAdvisersByProfile(tt.profile, advisors)

			if tt.wantIncluded {
				if len(got) != 1 {
					t.Errorf("FilterAdvisersByProfile() = %v (len=%d), want advisor included; scope=%v profile=%v",
						got, len(got), tt.advisorScope, tt.profile)
				}
			} else {
				if len(got) != 0 {
					t.Errorf("FilterAdvisersByProfile() = %v (len=%d), want advisor excluded; scope=%v profile=%v",
						got, len(got), tt.advisorScope, tt.profile)
				}
			}
		})
	}
}

// TestFilterAdvisersByProfile_EmptyAdvisors verifies that an empty (non-nil) slice
// returns an empty (non-nil) slice — distinct from the nil-in/nil-out contract.
func TestFilterAdvisersByProfile_EmptyAdvisors(t *testing.T) {
	frontendProfile := &detect.ProjectProfile{
		Frameworks: []string{"React"},
	}
	got := FilterAdvisersByProfile(frontendProfile, []model.AdvisorDef{})
	if got == nil {
		t.Fatal("FilterAdvisersByProfile(profile, []model.AdvisorDef{}) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("FilterAdvisersByProfile(profile, []) = %v (len=%d), want empty", got, len(got))
	}
}

// TestFilterAdvisersByProfile_MultipleAdvisors verifies correct subset selection
// when the input contains a mix of universal, matching, and non-matching advisors.
func TestFilterAdvisersByProfile_MultipleAdvisors(t *testing.T) {
	frontendProfile := &detect.ProjectProfile{
		Frameworks: []string{"React"},
		Languages:  []detect.LanguageInfo{{Name: "TypeScript", Percentage: 90}},
	}

	universal := anAdvisorDef().named("unit-test-advisor").withScope().build()
	frontendOnly := anAdvisorDef().named("component-advisor").withScope(model.AdvisorScopeFrontend).build()
	backendOnly := anAdvisorDef().named("architect-advisor").withScope(model.AdvisorScopeBackend).build()
	frontendAndTesting := anAdvisorDef().named("frontend-test-advisor").withScope(model.AdvisorScopeFrontend, model.AdvisorScopeTesting).build()

	advisors := []model.AdvisorDef{universal, frontendOnly, backendOnly, frontendAndTesting}
	got := FilterAdvisersByProfile(frontendProfile, advisors)

	gotNames := make(map[string]bool, len(got))
	for _, a := range got {
		gotNames[a.Name] = true
	}

	wantIncluded := []string{"unit-test-advisor", "component-advisor", "frontend-test-advisor"}
	wantExcluded := []string{"architect-advisor"}

	for _, name := range wantIncluded {
		if !gotNames[name] {
			t.Errorf("FilterAdvisersByProfile() missing %q; got names: %v", name, gotNames)
		}
	}
	for _, name := range wantExcluded {
		if gotNames[name] {
			t.Errorf("FilterAdvisersByProfile() should not contain %q; got names: %v", name, gotNames)
		}
	}
}
