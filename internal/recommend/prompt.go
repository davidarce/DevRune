// SPDX-License-Identifier: MIT

package recommend

import (
	"encoding/json"
	"fmt"

	"github.com/davidarce/devrune/internal/detect"
)

// systemPrompt returns the system prompt used when invoking Claude with --system-prompt.
// This replaces Claude's default system prompt for faster, focused responses.
// It instructs the model to ONLY produce the structured JSON output, nothing else.
func systemPrompt() string {
	return `You are a catalog recommender. Given a project tech profile and a catalog of items, return ONLY the structured JSON output matching the provided schema. Do not explain, summarize, or add commentary. Just fill the structured output with recommendations. Be selective: only recommend items clearly relevant to the detected technologies. Assign confidence 0.0-1.0 and a 1-sentence reason per item.`
}

// promptPayload is the JSON structure sent to the AI in the user message.
type promptPayload struct {
	ProjectProfile profileSummary `json:"project_profile"`
	CatalogItems   []CatalogItem  `json:"catalog_items"`
}

// profileSummary is a JSON-friendly summary of the detected project profile.
type profileSummary struct {
	Languages    []string          `json:"languages"`
	Frameworks   []string          `json:"frameworks"`
	Dependencies []depSummary      `json:"dependencies"`
	ConfigFiles  []string          `json:"config_files"`
	TotalFiles   int               `json:"total_files"`
	TotalLines   int               `json:"total_lines"`
}

type depSummary struct {
	File         string            `json:"file"`
	Type         string            `json:"type"`
	Dependencies map[string]string `json:"dependencies"`
}

// buildPrompt constructs the combined prompt string sent to the AI agent.
// The system prompt and user payload are separated by a newline, as both
// claude and opencode accept them as a single -p argument.
func buildPrompt(profile detect.ProjectProfile, catalog []CatalogItem) string {
	summary := profileSummary{
		Frameworks:  profile.Frameworks,
		ConfigFiles: profile.ConfigFiles,
		TotalFiles:  profile.TotalFiles,
		TotalLines:  profile.TotalLines,
	}
	for _, lang := range profile.Languages {
		summary.Languages = append(summary.Languages, lang.Name)
	}
	for _, dep := range profile.Dependencies {
		summary.Dependencies = append(summary.Dependencies, depSummary{
			File:         dep.Path,
			Type:         dep.Type,
			Dependencies: dep.Dependencies,
		})
	}

	payload := promptPayload{
		ProjectProfile: summary,
		CatalogItems:   catalog,
	}
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")

	return fmt.Sprintf("Recommend relevant catalog items for this project. Return ONLY the structured output.\n\n%s", string(payloadJSON))
}
