// SPDX-License-Identifier: MIT

package model

// SDDPhaseRoleNames is the ordered list of SDD phase role names used in workflow.yaml.
var SDDPhaseRoleNames = []string{"sdd-explorer", "sdd-planner", "sdd-implementer", "sdd-reviewer"}

// SDDModelRoutingAgents is the set of agent names that support per-phase model routing.
// Only agents in this set will show the SDD model selection step in the TUI.
var SDDModelRoutingAgents = map[string]bool{"claude": true, "opencode": true}

// SDDModelInheritOption is the sentinel value indicating that a phase should inherit
// the model from the session context rather than use a specific model override.
const SDDModelInheritOption = "No model (inherit from session)"

// SDDPhaseLabels maps role names to human-readable display labels for the TUI.
var SDDPhaseLabels = map[string]string{
	"sdd-explorer":    "Explore model",
	"sdd-planner":     "Plan model",
	"sdd-implementer": "Implement model",
	"sdd-reviewer":    "Review model",
}

// ModelOption is a label/value pair used to populate huh Select options in the TUI.
type ModelOption struct {
	Label string
	Value string
}

// claudeShortNames is the ordered list of short model names supported for Claude agents.
// Full IDs are resolved at install time by modelShortToFull in renderers/helpers.go.
var claudeShortNames = []string{"haiku", "sonnet", "opus"}

// ClaudeModelOptions returns the static model choices for Claude agents.
// The first option is the inherit sentinel (no override). Followed by haiku, sonnet, opus.
func ClaudeModelOptions() []ModelOption {
	opts := make([]ModelOption, 0, len(claudeShortNames)+1)
	opts = append(opts, ModelOption{
		Label: SDDModelInheritOption,
		Value: SDDModelInheritOption,
	})
	for _, name := range claudeShortNames {
		opts = append(opts, ModelOption{
			Label: name,
			Value: name,
		})
	}
	return opts
}
