// SPDX-License-Identifier: MIT

package steps

import (
	"strings"
	"testing"
)

// TestSDDInfoContent verifies that sddInfoContent returns the correct description
// based on terminal width relative to narrowWidthThreshold (70).
func TestSDDInfoContent(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		wantSubstring string
		wantAbsent    string
	}{
		{
			name:          "wide terminal (w=80) returns full content with all four phases",
			width:         80,
			wantSubstring: "Explore",
		},
		{
			name:          "wide terminal (w=80) contains Plan phase",
			width:         80,
			wantSubstring: "Plan",
		},
		{
			name:          "wide terminal (w=80) contains Implement phase",
			width:         80,
			wantSubstring: "Implement",
		},
		{
			name:          "wide terminal (w=80) contains Review phase",
			width:         80,
			wantSubstring: "Review",
		},
		{
			name:          "narrow terminal (w=60) returns short content with '4 phases'",
			width:         60,
			wantSubstring: "4 phases",
		},
		{
			name:          "narrow terminal (w=60) short content does not contain numbered phase list",
			width:         60,
			wantAbsent:    "① Explore",
		},
		{
			name:          "at threshold (w=70) returns full content (strictly less-than check)",
			width:         70,
			wantSubstring: "Explore",
		},
		{
			name:          "one below threshold (w=69) returns short content",
			width:         69,
			wantSubstring: "4 phases",
		},
		{
			name:          "very wide terminal (w=200) returns full content",
			width:         200,
			wantSubstring: "Implement",
		},
		{
			name:          "minimum width (w=1) returns short content",
			width:         1,
			wantSubstring: "4 phases",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sddInfoContent(tc.width)

			if tc.wantSubstring != "" && !strings.Contains(got, tc.wantSubstring) {
				t.Errorf("sddInfoContent(%d) = %q; want it to contain %q", tc.width, got, tc.wantSubstring)
			}

			if tc.wantAbsent != "" && strings.Contains(got, tc.wantAbsent) {
				t.Errorf("sddInfoContent(%d) = %q; want it NOT to contain %q", tc.width, got, tc.wantAbsent)
			}
		})
	}
}

// TestSDDInfoContentDistinctBranches verifies that wide and narrow terminals return
// distinct content (not the same string).
func TestSDDInfoContentDistinctBranches(t *testing.T) {
	wide := sddInfoContent(80)
	narrow := sddInfoContent(60)

	if wide == narrow {
		t.Error("sddInfoContent(80) and sddInfoContent(60) returned identical strings; expected distinct content for wide vs narrow terminals")
	}
}

// TestSDDInfoContentFullHasFourPhases verifies the full content mentions all four
// SDD phase names in a single call.
func TestSDDInfoContentFullHasFourPhases(t *testing.T) {
	content := sddInfoContent(80)

	phases := []string{"Explore", "Plan", "Implement", "Review"}
	for _, phase := range phases {
		if !strings.Contains(content, phase) {
			t.Errorf("sddInfoContent(80): full content missing phase %q", phase)
		}
	}
}
