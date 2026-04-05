// SPDX-License-Identifier: MIT

package model

// RecommendedManifest represents the devrune.recommended.yaml file produced by the recommend command.
type RecommendedManifest struct {
	SchemaVersion   string             `yaml:"schemaVersion"`   // "devrune/recommend/v1"
	Recommendations []RecommendedItem  `yaml:"recommendations"` // AI-scored catalog items
	Profile         RecommendedProfile `yaml:"profile"`         // summary of detected tech
	GeneratedAt     string             `yaml:"generatedAt"`     // ISO 8601
}

// RecommendedItem is a single recommended catalog item with a confidence score.
type RecommendedItem struct {
	Name       string  `yaml:"name"`
	Kind       string  `yaml:"kind"`
	Source     string  `yaml:"source"`
	Confidence float64 `yaml:"confidence"`
	Reason     string  `yaml:"reason"`
}

// RecommendedProfile summarises the detected tech stack that drove the recommendations.
type RecommendedProfile struct {
	Languages  []string `yaml:"languages"`
	Frameworks []string `yaml:"frameworks"`
}
