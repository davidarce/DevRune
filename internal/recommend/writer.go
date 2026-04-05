// SPDX-License-Identifier: MIT

package recommend

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/model"
)

// WriteRecommendedYAML builds a RecommendedManifest from the recommendation
// result and project profile, serialises it to YAML, and writes it to path.
func WriteRecommendedYAML(path string, result *RecommendResult, profile detect.ProjectProfile) error {
	// Build language name list.
	languages := make([]string, 0, len(profile.Languages))
	for _, l := range profile.Languages {
		languages = append(languages, l.Name)
	}

	// Convert recommendations to model items.
	items := make([]model.RecommendedItem, 0, len(result.Recommendations))
	for _, r := range result.Recommendations {
		items = append(items, model.RecommendedItem{
			Name:       r.Name,
			Kind:       r.Kind,
			Source:     r.Source,
			Confidence: r.Confidence,
			Reason:     r.Reason,
		})
	}

	manifest := model.RecommendedManifest{
		SchemaVersion:   "devrune/recommend/v1",
		Recommendations: items,
		Profile: model.RecommendedProfile{
			Languages:  languages,
			Frameworks: profile.Frameworks,
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
