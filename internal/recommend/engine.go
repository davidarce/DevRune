// SPDX-License-Identifier: MIT

package recommend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/davidarce/devrune/internal/detect"
)

// Engine is the main recommendation engine. It invokes an AI agent CLI
// to produce catalog recommendations from a detected project profile.
type Engine struct {
	agentBinary string
	agentName   string
	cfg         RecommendConfig
}

// NewEngine creates a new Engine for the given agent binary and config.
// agentBinary is the full path to the agent executable.
// agentName is "claude" or "opencode" (controls which flags are used).
func NewEngine(agentBinary string, agentName string, cfg RecommendConfig) *Engine {
	if cfg.Models.Claude == "" || cfg.Models.OpenCode == "" {
		defaults := DefaultAgentModels()
		if cfg.Models.Claude == "" {
			cfg.Models.Claude = defaults.Claude
		}
		if cfg.Models.OpenCode == "" {
			cfg.Models.OpenCode = defaults.OpenCode
		}
	}
	return &Engine{
		agentBinary: agentBinary,
		agentName:   agentName,
		cfg:         cfg,
	}
}

// recommendTimeout is the default timeout for the AI agent invocation.
// T026: If the agent takes longer than this, the context is cancelled.
const recommendTimeout = 60 * time.Second

// Recommend invokes the AI agent with the project profile and catalog items,
// parses the structured JSON response, and filters results by the configured
// confidence threshold.
//
// T025: Returns an empty result immediately if catalog is empty.
// T026: Applies a 60-second timeout to the AI agent invocation.
// T027: Returns a descriptive error if the AI response cannot be parsed as JSON.
func (e *Engine) Recommend(ctx context.Context, profile detect.ProjectProfile, catalog []CatalogItem) (*RecommendResult, error) {
	// Skip AI invocation entirely when catalog is empty.
	if len(catalog) == 0 {
		return &RecommendResult{Recommendations: []Recommendation{}}, nil
	}

	// Check cache first.
	key := cacheKey(profile, catalog)
	if cached := loadCachedResult(key, DefaultCacheTTL); cached != nil {
		// Apply threshold filter to cached results.
		threshold := e.cfg.Threshold
		if threshold <= 0 {
			threshold = 0.7
		}
		filtered := cached.Recommendations[:0]
		for _, rec := range cached.Recommendations {
			if rec.Confidence >= threshold {
				filtered = append(filtered, rec)
			}
		}
		cached.Recommendations = filtered
		return cached, nil
	}

	// Apply a timeout so the AI call doesn't block indefinitely.
	ctx, cancel := context.WithTimeout(ctx, recommendTimeout)
	defer cancel()

	prompt := buildPrompt(profile, catalog)
	args := e.buildArgs(prompt)

	cmd := exec.CommandContext(ctx, e.agentBinary, args...) //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			return nil, fmt.Errorf("agent %q exited with code %d: %s", e.agentName, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("running agent %q: %w", e.agentName, err)
	}

	// Claude's --output-format json wraps the response in an envelope:
	// {"result":"...", "structured_output":{...}, "session_id":"...", ...}
	// The actual recommendation data is in structured_output when --json-schema is used.
	var result RecommendResult
	if e.agentName == "claude" {
		var envelope struct {
			Result           string          `json:"result"`
			StructuredOutput json.RawMessage `json:"structured_output"`
		}
		if err := json.Unmarshal(out, &envelope); err != nil {
			return nil, fmt.Errorf("parsing claude envelope: %w (output: %.200s)", err, string(out))
		}
		if len(envelope.StructuredOutput) > 0 && string(envelope.StructuredOutput) != "null" {
			if err := json.Unmarshal(envelope.StructuredOutput, &result); err != nil {
				return nil, fmt.Errorf("parsing structured_output: %w (output: %.200s)", err, string(envelope.StructuredOutput))
			}
		} else {
			// Fallback: try extracting JSON from the result text.
			resultText := strings.TrimSpace(envelope.Result)
			// Claude may wrap JSON in markdown code blocks.
			if idx := strings.Index(resultText, "```json"); idx >= 0 {
				resultText = resultText[idx+7:]
				if end := strings.Index(resultText, "```"); end >= 0 {
					resultText = resultText[:end]
				}
			} else if idx := strings.Index(resultText, "```"); idx >= 0 {
				resultText = resultText[idx+3:]
				if end := strings.Index(resultText, "```"); end >= 0 {
					resultText = resultText[:end]
				}
			}
			// Try to find JSON object in the text.
			resultText = strings.TrimSpace(resultText)
			if resultText == "" {
				// No text result either — try the raw output as last resort.
				resultText = strings.TrimSpace(string(out))
			}
			if err := json.Unmarshal([]byte(resultText), &result); err != nil {
				return nil, fmt.Errorf("parsing claude result as JSON: %w (output: %.300s)", err, envelope.Result)
			}
		}
	} else {
		if err := json.Unmarshal(out, &result); err != nil {
			return nil, fmt.Errorf("parsing agent response as JSON: %w (output: %.200s)", err, string(out))
		}
	}

	// Cache the full result (before threshold filter) so different thresholds
	// can reuse the same cache. Save with both keys (full profile + quick dir-based).
	saveCachedResult(key, &result)
	dir, _ := os.Getwd()
	quickKey := QuickCacheKey(dir, catalog)
	if quickKey != key {
		saveCachedResult(quickKey, &result)
	}

	// Filter recommendations below threshold.
	threshold := e.cfg.Threshold
	if threshold <= 0 {
		threshold = 0.7
	}
	filtered := result.Recommendations[:0]
	for _, rec := range result.Recommendations {
		if rec.Confidence >= threshold {
			filtered = append(filtered, rec)
		}
	}
	result.Recommendations = filtered

	return &result, nil
}

// buildArgs constructs the CLI argument slice for the agent, selecting
// the correct flags based on the agent name.
//
//   - claude: --model <model> -p <prompt> --output-format json --json-schema <schema> --bare
//   - opencode: -m <model> -p <prompt> --output-format json
func (e *Engine) buildArgs(prompt string) []string {
	switch e.agentName {
	case "claude":
		return []string{
			"--model", e.cfg.Models.Claude,
			"-p", prompt,
			"--output-format", "json",
			"--json-schema", jsonSchema(),
			"--system-prompt", systemPrompt(),
		}
	case "opencode":
		return []string{
			"-m", e.cfg.Models.OpenCode,
			"-p", prompt,
			"--output-format", "json",
		}
	default:
		// Unknown agent: pass prompt and output-format as a baseline.
		return []string{"-p", prompt, "--output-format", "json"}
	}
}

// isExitError checks whether err is an *exec.ExitError and writes to target.
// Uses errors.As so it correctly unwraps wrapped errors.
func isExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}
