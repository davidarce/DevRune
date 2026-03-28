// SPDX-License-Identifier: MIT

package model

import "strings"

// ModelRoutingAgents is the set of agent names that support per-role model routing.
// Only agents in this set will show the workflow model selection step in the TUI.
var ModelRoutingAgents = map[string]bool{"claude": true, "opencode": true}

// ModelInheritOption is the sentinel value indicating that a role should inherit
// the model from the session context rather than use a specific model override.
const ModelInheritOption = "No model (inherit from session)"

// Backward-compatible aliases so existing code that references the old names
// continues to compile during the migration period. These will be removed in
// a future release.
var SDDModelRoutingAgents = ModelRoutingAgents

const SDDModelInheritOption = ModelInheritOption

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
		Label: ModelInheritOption,
		Value: ModelInheritOption,
	})
	for _, name := range claudeShortNames {
		opts = append(opts, ModelOption{
			Label: name,
			Value: name,
		})
	}
	return opts
}

// PlaceholderKeyFromRole derives the placeholder key suffix for a workflow role.
//
// When explicitPlaceholder is non-empty, it is returned as-is (uppercased).
// Otherwise the key is auto-derived: strip the "<workflowName>-" prefix from
// roleName, uppercase, and replace hyphens with underscores.
//
// Examples:
//
//	PlaceholderKeyFromRole("sdd", "sdd-explorer", "")        → "EXPLORER"
//	PlaceholderKeyFromRole("cicd", "reviewer", "")            → "REVIEWER"
//	PlaceholderKeyFromRole("cicd", "code-quality-checker", "CHECKER") → "CHECKER"
func PlaceholderKeyFromRole(workflowName, roleName, explicitPlaceholder string) string {
	if explicitPlaceholder != "" {
		return strings.ToUpper(explicitPlaceholder)
	}
	// Strip workflow name prefix (e.g. "sdd-" from "sdd-explorer").
	stripped := strings.TrimPrefix(roleName, workflowName+"-")
	// Uppercase and replace hyphens with underscores.
	return strings.ToUpper(strings.ReplaceAll(stripped, "-", "_"))
}
