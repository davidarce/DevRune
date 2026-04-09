// SPDX-License-Identifier: MIT

package recommend

import (
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

func TestSkillRefToPackageRef(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantSrc  string
		wantSkls []string // nil means no Select filter expected
	}{
		{
			name:     "standard three-segment path",
			path:     "owner/repo/skill-name",
			wantSrc:  "github:owner/repo",
			wantSkls: []string{"skill-name"},
		},
		{
			name:     "vercel react best practices",
			path:     "vercel-labs/agent-skills/vercel-react-best-practices",
			wantSrc:  "github:vercel-labs/agent-skills",
			wantSkls: []string{"vercel-react-best-practices"},
		},
		{
			name:     "two-segment path — no skill name",
			path:     "owner/repo",
			wantSrc:  "github:owner/repo",
			wantSkls: nil,
		},
		{
			name:     "path with nested segments beyond three",
			path:     "org/repo/nested/path",
			wantSrc:  "github:org/repo",
			wantSkls: []string{"nested/path"},
		},
		{
			name:     "empty third segment treated as two-segment",
			path:     "owner/repo/",
			wantSrc:  "github:owner/repo",
			wantSkls: nil,
		},
		{
			name:     "single segment — malformed",
			path:     "onlyone",
			wantSrc:  "github:onlyone",
			wantSkls: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := SkillRef{Path: tc.path, Description: "test"}
			got := SkillRefToPackageRef(ref)

			if got.Source != tc.wantSrc {
				t.Errorf("Source: got %q, want %q", got.Source, tc.wantSrc)
			}

			if tc.wantSkls == nil {
				if got.Select != nil && len(got.Select.Skills) > 0 {
					t.Errorf("Select.Skills: expected none, got %v", got.Select.Skills)
				}
				return
			}

			if got.Select == nil {
				t.Fatalf("Select: expected non-nil with skills %v, got nil", tc.wantSkls)
			}

			if len(got.Select.Skills) != len(tc.wantSkls) {
				t.Fatalf("Select.Skills length: got %d, want %d", len(got.Select.Skills), len(tc.wantSkls))
			}

			for i, s := range tc.wantSkls {
				if got.Select.Skills[i] != s {
					t.Errorf("Select.Skills[%d]: got %q, want %q", i, got.Select.Skills[i], s)
				}
			}
		})
	}
}

func TestSkillRefToPackageRef_ReturnsCorrectType(t *testing.T) {
	ref := SkillRef{Path: "owner/repo/skill", Description: "desc"}
	got := SkillRefToPackageRef(ref)

	// Verify the return type is model.PackageRef (compile-time check).
	_ = model.PackageRef(got)
}
