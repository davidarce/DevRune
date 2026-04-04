// SPDX-License-Identifier: MIT

package steps

import (
	"testing"
)

// TestResponsiveDescription verifies that testableResponsiveDescription returns the
// correct description string based on terminal width relative to narrowWidthThreshold (70).
func TestResponsiveDescription(t *testing.T) {
	full := "Select one or more AI agents. Use space to toggle, enter to confirm."
	short := "Space to toggle, enter to confirm."

	tests := []struct {
		name  string
		width int
		want  string
	}{
		{
			name:  "wide terminal (w=80) returns full description",
			width: 80,
			want:  full,
		},
		{
			name:  "narrow terminal (w=60) returns short description",
			width: 60,
			want:  short,
		},
		{
			name:  "at threshold (w=70) returns full description (strictly less-than)",
			width: 70,
			want:  full,
		},
		{
			name:  "one below threshold (w=69) returns short description",
			width: 69,
			want:  short,
		},
		{
			name:  "very wide terminal (w=200) returns full description",
			width: 200,
			want:  full,
		},
		{
			name:  "minimum narrow (w=1) returns short description",
			width: 1,
			want:  short,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := testableResponsiveDescription(full, short, tc.width)
			if got != tc.want {
				t.Errorf("testableResponsiveDescription(full, short, %d) = %q; want %q", tc.width, got, tc.want)
			}
		})
	}
}

// TestDynamicHeight verifies that testableDynamicHeight correctly caps the requested
// height to the available space and enforces the minimum clamp.
func TestDynamicHeight(t *testing.T) {
	tests := []struct {
		name      string
		requested int
		termH     int
		want      int
	}{
		{
			name:      "tall terminal (h=40), requested=7: available=30, returns requested",
			requested: 7,
			termH:     40,
			want:      7,
		},
		{
			name:      "medium terminal (h=24), requested=7: available=14, returns requested",
			requested: 7,
			termH:     24,
			want:      7,
		},
		{
			name:      "short terminal (h=18), requested=7: available=8, returns requested",
			requested: 7,
			termH:     18,
			want:      7,
		},
		{
			name:      "very short terminal (h=15), requested=7: available=5, returns 5",
			requested: 7,
			termH:     15,
			want:      5,
		},
		{
			name:      "minimum clamp (h=12): available=2, clamps to minSelectHeight=3",
			requested: 7,
			termH:     12,
			want:      minSelectHeight,
		},
		{
			name:      "requested less than available: returns requested",
			requested: 3,
			termH:     40,
			want:      3,
		},
		{
			name:      "requested equals available: returns requested",
			requested: 30,
			termH:     40,
			want:      30,
		},
		{
			name:      "requested greater than available: returns available",
			requested: 35,
			termH:     40,
			want:      30, // 40 - headerOverhead(10) = 30
		},
		{
			name:      "zero-height terminal: clamps to minSelectHeight",
			requested: 7,
			termH:     0,
			want:      minSelectHeight,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := testableDynamicHeight(tc.requested, tc.termH)
			if got != tc.want {
				t.Errorf("testableDynamicHeight(%d, %d) = %d; want %d", tc.requested, tc.termH, got, tc.want)
			}
		})
	}
}

// TestResponsiveStepBanner verifies the width+height decision matrix for wizard step banners.
// The banner output itself contains ANSI escape sequences, so tests check for empty string
// (no banner) vs. non-empty (some banner), and distinguish compact vs. minimal by checking
// whether the 65-char separator is absent (compact) or present (minimal).
func TestResponsiveStepBanner(t *testing.T) {
	// The minimal banner contains a long separator that wraps on narrow terminals.
	const minimalBannerMarker = "─────────────────────────────────────────────────────────────────"

	tests := []struct {
		name        string
		w           int
		h           int
		wantEmpty   bool
		wantMinimal bool // true = minimal banner (contains separator); false = compact (no separator)
	}{
		{
			name:      "very short terminal (h=20): no banner",
			w:         80,
			h:         20,
			wantEmpty: true,
		},
		{
			name:      "h=24 (just below h<25 cutoff): no banner",
			w:         80,
			h:         24,
			wantEmpty: true,
		},
		{
			name:        "medium height, narrow width (h=30, w=60): compact banner",
			w:           60,
			h:           30,
			wantEmpty:   false,
			wantMinimal: false,
		},
		{
			name:        "medium height, wide width (h=30, w=80): compact banner (h<35 triggers compact)",
			w:           80,
			h:           30,
			wantEmpty:   false,
			wantMinimal: false,
		},
		{
			name:        "tall terminal, narrow width (h=40, w=60): compact banner (width < 70)",
			w:           60,
			h:           40,
			wantEmpty:   false,
			wantMinimal: false,
		},
		{
			name:        "tall terminal, wide width (h=40, w=80): minimal banner",
			w:           80,
			h:           40,
			wantEmpty:   false,
			wantMinimal: true,
		},
		{
			name:        "tall terminal, exactly at width threshold (h=40, w=70): minimal banner",
			w:           70,
			h:           40,
			wantEmpty:   false,
			wantMinimal: true,
		},
		{
			name:        "tall terminal, one below width threshold (h=40, w=69): compact banner",
			w:           69,
			h:           40,
			wantEmpty:   false,
			wantMinimal: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := testableResponsiveStepBanner(tc.w, tc.h)

			if tc.wantEmpty {
				if got != "" {
					t.Errorf("testableResponsiveStepBanner(%d, %d) = %q; want empty string", tc.w, tc.h, got)
				}
				return
			}

			if got == "" {
				t.Errorf("testableResponsiveStepBanner(%d, %d) = empty; want non-empty banner", tc.w, tc.h)
				return
			}

			// Use a string contains check to distinguish minimal banner (has long separator) from compact
			containsSeparator := len(got) > 0 && stringContains(got, minimalBannerMarker)

			if tc.wantMinimal && !containsSeparator {
				t.Errorf("testableResponsiveStepBanner(%d, %d): want minimal banner (with separator) but got compact banner", tc.w, tc.h)
			}
			if !tc.wantMinimal && containsSeparator {
				t.Errorf("testableResponsiveStepBanner(%d, %d): want compact banner (no separator) but got minimal banner", tc.w, tc.h)
			}
		})
	}
}

// stringContains reports whether substr is contained in s.
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

// searchSubstring is a simple substring search without importing strings package
// (to keep test file self-contained for clarity).
func searchSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
