// SPDX-License-Identifier: MIT

package steps

import (
	"reflect"
	"testing"
)

func TestDeduplicateSources(t *testing.T) {
	tests := []struct {
		name       string
		predefined []string
		custom     []string
		want       []string
	}{
		{
			name:       "no duplicates",
			predefined: []string{"a"},
			custom:     []string{"b"},
			want:       []string{"a", "b"},
		},
		{
			name:       "duplicate removed",
			predefined: []string{"a"},
			custom:     []string{"a", "b"},
			want:       []string{"a", "b"},
		},
		{
			name:       "empty predefined",
			predefined: []string{},
			custom:     []string{"a"},
			want:       []string{"a"},
		},
		{
			name:       "empty custom",
			predefined: []string{"a"},
			custom:     []string{},
			want:       []string{"a"},
		},
		{
			name:       "both empty",
			predefined: []string{},
			custom:     []string{},
			want:       nil,
		},
		{
			name:       "order preserved predefined first",
			predefined: []string{"a", "b"},
			custom:     []string{"c", "a"},
			want:       []string{"a", "b", "c"},
		},
		{
			name:       "nil predefined",
			predefined: nil,
			custom:     []string{"a"},
			want:       []string{"a"},
		},
		{
			name:       "nil custom",
			predefined: []string{"a"},
			custom:     nil,
			want:       []string{"a"},
		},
		{
			name:       "both nil",
			predefined: nil,
			custom:     nil,
			want:       nil,
		},
		{
			name:       "all custom are duplicates",
			predefined: []string{"a", "b"},
			custom:     []string{"b", "a"},
			want:       []string{"a", "b"},
		},
		{
			name:       "duplicate within custom only",
			predefined: []string{"a"},
			custom:     []string{"b", "b"},
			want:       []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateSources(tt.predefined, tt.custom)

			if len(got) != len(tt.want) {
				t.Fatalf("deduplicateSources(%v, %v) = %v (len %d), want %v (len %d)",
					tt.predefined, tt.custom, got, len(got), tt.want, len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("deduplicateSources(%v, %v)[%d] = %q, want %q",
						tt.predefined, tt.custom, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeKnownSources(t *testing.T) {
	tests := []struct {
		name  string
		base  []knownSource
		extra []knownSource
		want  []knownSource
	}{
		{
			name:  "nil extra returns base unchanged",
			base:  []knownSource{{label: "A", value: "a"}},
			extra: nil,
			want:  []knownSource{{label: "A", value: "a"}},
		},
		{
			name:  "empty extra returns base unchanged",
			base:  []knownSource{{label: "A", value: "a"}},
			extra: []knownSource{},
			want:  []knownSource{{label: "A", value: "a"}},
		},
		{
			name:  "non-overlapping extra appended",
			base:  []knownSource{{label: "A", value: "a"}},
			extra: []knownSource{{label: "B", value: "b"}},
			want:  []knownSource{{label: "A", value: "a"}, {label: "B", value: "b"}},
		},
		{
			name:  "duplicate value in extra is skipped",
			base:  []knownSource{{label: "Starter Catalog", value: "github:owner/repo"}},
			extra: []knownSource{{label: "github:owner/repo", value: "github:owner/repo"}},
			want:  []knownSource{{label: "Starter Catalog", value: "github:owner/repo"}},
		},
		{
			name: "multiple extra with one duplicate",
			base: []knownSource{
				{label: "Starter", value: "github:owner/starter"},
			},
			extra: []knownSource{
				{label: "github:owner/starter", value: "github:owner/starter"},
				{label: "github:myorg/custom", value: "github:myorg/custom"},
			},
			want: []knownSource{
				{label: "Starter", value: "github:owner/starter"},
				{label: "github:myorg/custom", value: "github:myorg/custom"},
			},
		},
		{
			name:  "empty base with extra",
			base:  []knownSource{},
			extra: []knownSource{{label: "X", value: "x"}},
			want:  []knownSource{{label: "X", value: "x"}},
		},
		{
			name: "base label is preserved when duplicate found (not overwritten by extra label)",
			base: []knownSource{
				{label: "DevRune Starter Catalog", value: "github:davidarce/devrune-starter-catalog"},
			},
			extra: []knownSource{
				// Same value, different label — base label must be kept.
				{label: "github:davidarce/devrune-starter-catalog", value: "github:davidarce/devrune-starter-catalog"},
			},
			want: []knownSource{
				{label: "DevRune Starter Catalog", value: "github:davidarce/devrune-starter-catalog"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeKnownSources(tt.base, tt.extra)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeKnownSources() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEnterRepositoriesExtraSourcesConvertedToKnownSource(t *testing.T) {
	// Test that mergeKnownSources correctly handles the conversion that
	// EnterRepositories performs internally: extra ref strings become
	// knownSource entries with label == value == ref string.
	extra := []knownSource{
		{label: "github:myorg/custom-catalog@v2", value: "github:myorg/custom-catalog@v2"},
	}
	base := []knownSource{
		{label: "DevRune Starter Catalog", value: "github:davidarce/devrune-starter-catalog"},
	}
	got := mergeKnownSources(base, extra)

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	if got[0].value != "github:davidarce/devrune-starter-catalog" {
		t.Errorf("got[0].value = %q, want %q", got[0].value, "github:davidarce/devrune-starter-catalog")
	}
	if got[1].value != "github:myorg/custom-catalog@v2" {
		t.Errorf("got[1].value = %q, want %q", got[1].value, "github:myorg/custom-catalog@v2")
	}
	if got[1].label != "github:myorg/custom-catalog@v2" {
		t.Errorf("got[1].label = %q, want source ref as label %q", got[1].label, "github:myorg/custom-catalog@v2")
	}
}
