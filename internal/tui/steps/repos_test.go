package steps

import (
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
