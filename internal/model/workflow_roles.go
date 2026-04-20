// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"math"
	"strings"
)

// ModelRoutingAgents is the set of agent names that support per-role model routing.
// Only agents in this set will show the workflow model selection step in the TUI.
var ModelRoutingAgents = map[string]bool{"claude": true, "opencode": true, "copilot": true}

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

// CopilotModelEntry pairs a VS Code display name with its provider and Copilot plan availability.
// DisplayName is the exact string VS Code Copilot recognises in .agent.md `model:` frontmatter
// (e.g. "Claude Sonnet 4.6"). API slugs (e.g. "claude-sonnet-4.6") are NOT accepted by VS Code.
type CopilotModelEntry struct {
	DisplayName  string
	Provider     string
	Availability string
	Tier         float64 // capability/cost tier used for sub-agent filtering
}

// copilotModelEntries is the ordered canonical list of Copilot model choices for the TUI.
// Source: docs.github.com/en/copilot/reference/ai-models/supported-models (April 2026)
// DisplayName values are written verbatim to .agent.md frontmatter — VS Code requires the
// display-name format (e.g. "Claude Sonnet 4.6"), NOT API slugs or "anthropic/..." prefixes.
// IMPORTANT: Non-Anthropic display names are best-effort — verify via VS Code Copilot model picker.
//
// Ordering: inherit sentinel first (added by CopilotModelOptions), then providers alphabetically
// (Anthropic → Google → OpenAI → xAI), then within each provider by capability tier
// (haiku < sonnet < opus; mini < standard < codex).
var copilotModelEntries = []CopilotModelEntry{
	// Anthropic
	{DisplayName: "Claude Haiku 4.5", Provider: "Anthropic", Availability: "Free · Pro · Pro+ · Business · Enterprise", Tier: 1.0},
	{DisplayName: "Claude Sonnet 4.6", Provider: "Anthropic", Availability: "Pro · Pro+ · Business · Enterprise", Tier: 2.0},
	{DisplayName: "Claude Opus 4.7", Provider: "Anthropic", Availability: "Pro+ · Business · Enterprise", Tier: 3.0},
	// Google
	{DisplayName: "Gemini 2.5 Pro", Provider: "Google", Availability: "Pro and above", Tier: 2.0},
	{DisplayName: "Gemini 3.1 Pro (Preview)", Provider: "Google", Availability: "Pro and above", Tier: 2.0},
	{DisplayName: "Gemini 3 Flash (Preview)", Provider: "Google", Availability: "Pro and above", Tier: 2.0},
	// OpenAI
	{DisplayName: "GPT-5 mini", Provider: "OpenAI", Availability: "Free · All plans", Tier: 1.0},
	{DisplayName: "GPT-5.4", Provider: "OpenAI", Availability: "Pro and above", Tier: 2.0},
	{DisplayName: "GPT-5.3-Codex", Provider: "OpenAI", Availability: "Pro and above", Tier: 2.0},
	// xAI
	{DisplayName: "Grok Code Fast 1", Provider: "xAI", Availability: "Pro and above", Tier: 2.0},
}

// CopilotTierForModel returns the Tier for the given DisplayName.
// Returns math.MaxFloat64 for empty, sentinel, or unknown values.
// Used to derive the maxTier for sub-agent filtering from a saved orchestrator model value.
func CopilotTierForModel(displayName string) float64 {
	if displayName == "" || displayName == ModelInheritOption {
		return math.MaxFloat64
	}
	for _, e := range copilotModelEntries {
		if e.DisplayName == displayName {
			return e.Tier
		}
	}
	return math.MaxFloat64
}

// CopilotModelOptionsUpTo returns model choices filtered to Tier <= maxTier.
// The inherit sentinel is always included as the first option regardless of maxTier.
// Pass math.MaxFloat64 to include all models (no filtering).
func CopilotModelOptionsUpTo(maxTier float64) []ModelOption {
	opts := make([]ModelOption, 0, len(copilotModelEntries)+1)
	opts = append(opts, ModelOption{Label: ModelInheritOption, Value: ModelInheritOption})
	for _, e := range copilotModelEntries {
		if e.Tier <= maxTier {
			opts = append(opts, ModelOption{
				Label: fmt.Sprintf("%s · %s  (%s)", e.Provider, e.DisplayName, e.Availability),
				Value: e.DisplayName,
			})
		}
	}
	return opts
}

// CopilotModelOptions returns all Copilot model choices (no tier filtering).
// Backward-compatible wrapper for CopilotModelOptionsUpTo(math.MaxFloat64).
func CopilotModelOptions() []ModelOption { return CopilotModelOptionsUpTo(math.MaxFloat64) }

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
