// SPDX-License-Identifier: MIT

package recommend

// CatalogItem describes a single catalog entry passed to the AI for matching.
type CatalogItem struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`        // "skill" | "rule" | "mcp" | "workflow"
	Source      string `json:"source"`      // source ref string
	Description string `json:"description"` // human-readable description
}

// Recommendation is a single AI recommendation for a catalog item.
type Recommendation struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reason     string  `json:"reason"`     // why this item was recommended
}

// RecommendResult holds the full AI response.
type RecommendResult struct {
	Recommendations []Recommendation `json:"recommendations"`
}

// RecommendConfig holds configuration for the recommend engine.
type RecommendConfig struct {
	Threshold float64     // confidence threshold for pre-selection (default 0.7)
	Enabled   bool        // feature toggle (default true)
	Models    AgentModels // model to use per AI agent CLI
}

// AgentModels specifies which model each agent CLI should use.
// Defaults: Claude → "haiku", OpenCode → "openai/gpt-5.4-mini".
type AgentModels struct {
	Claude   string // passed as --model flag to claude CLI
	OpenCode string // passed as -m flag to opencode CLI
}

// DefaultAgentModels returns the default model configuration for each agent.
func DefaultAgentModels() AgentModels {
	return AgentModels{Claude: "haiku", OpenCode: "openai/gpt-5.4-mini"}
}
