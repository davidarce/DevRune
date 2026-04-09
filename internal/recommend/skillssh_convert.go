// SPDX-License-Identifier: MIT

package recommend

import (
	"strings"

	"github.com/davidarce/devrune/internal/model"
)

// SkillRefToPackageRef converts a skills.sh SkillRef into a model.PackageRef
// that the existing resolve pipeline can fetch.
//
// The skills.sh path format is "owner/repo/skill-name". This maps to:
//   - Source: "github:owner/repo"
//   - Select: model.SelectFilter{Skills: []string{"skill-name"}}
//
// Edge case: if the path has only two segments ("owner/repo"), the returned
// PackageRef has no Select filter (fetches the entire repo).
//
// Paths with more than three segments treat the third and subsequent segments
// joined with "/" as the skill name (handles nested skill paths gracefully).
func SkillRefToPackageRef(ref SkillRef) model.PackageRef {
	parts := strings.SplitN(ref.Path, "/", 3)

	if len(parts) < 2 {
		// Malformed path — return as-is with no select filter.
		return model.PackageRef{
			Source: "github:" + ref.Path,
		}
	}

	source := "github:" + parts[0] + "/" + parts[1]

	if len(parts) == 2 || parts[2] == "" {
		// No skill name segment — fetch entire repo.
		return model.PackageRef{
			Source: source,
		}
	}

	// Third segment (and beyond, already joined by SplitN) is the skill name.
	skillName := parts[2]
	return model.PackageRef{
		Source: source,
		Select: &model.SelectFilter{
			Skills: []string{skillName},
		},
	}
}
