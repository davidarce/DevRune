// SPDX-License-Identifier: MIT

package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// openCodeModelsSource is the path override for the OpenCode models source file.
// When empty, the default path is computed from os.UserHomeDir().
// Tests can set this variable to inject a custom path; always defer-reset to "".
var openCodeModelsSource = ""

// openCodeModelsCache is the path override for the devrune models cache file.
// When empty, the default path is computed from os.UserHomeDir().
// Tests can set this variable to inject a custom path; always defer-reset to "".
var openCodeModelsCache = ""

// openCodeProvider is the provider key to look up in the models source JSON.
const openCodeProvider = "github-copilot"

// openCodeModelEntry represents a single model entry in the provider's models map.
type openCodeModelEntry struct {
	ToolCall bool `json:"tool_call"`
}

// LoadOpenCodeModels discovers the list of OpenCode models that support tool calling.
//
// It reads the OpenCode models cache at ~/.cache/opencode/models.json, filters the
// github-copilot provider models to those with tool_call=true, sorts them alphabetically,
// and writes a cache to ~/.cache/devrune/models/opencode-copilot.json.
//
// On cache hit (cache exists AND cache mtime >= source mtime), the cached list is returned
// without re-parsing the source.
//
// If the source file is not found, unreadable, or the JSON is malformed, fallback is
// returned unchanged.
func LoadOpenCodeModels(fallback []string) []string {
	sourcePath := openCodeModelsSource
	if sourcePath == "" {
		// OpenCode always stores its models cache at ~/.cache/opencode/models.json
		// regardless of the OS (it uses the XDG cache convention, not OS-native dirs).
		// We must NOT use os.UserCacheDir() here — on macOS that returns ~/Library/Caches
		// which is the wrong location.
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fallback
		}
		sourcePath = filepath.Join(homeDir, ".cache", "opencode", "models.json")
	}

	cachePath := openCodeModelsCache
	if cachePath == "" {
		// Mirror the XDG convention for our own cache as well.
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fallback
		}
		cachePath = filepath.Join(homeDir, ".cache", "devrune", "models", "opencode-copilot.json")
	}

	// Stat source file; return fallback if not found or unreadable.
	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return fallback
	}

	// Cache hit check: if cache file exists AND its mtime >= source mtime, return cache.
	if cacheStat, err := os.Stat(cachePath); err == nil {
		if !cacheStat.ModTime().Before(sourceStat.ModTime()) {
			if cached := readOpenCodeCacheFile(cachePath); cached != nil {
				return cached
			}
		}
	}

	// Cache miss or stale: parse source file.
	models, err := parseOpenCodeModels(sourcePath)
	if err != nil {
		return fallback
	}

	// Write cache (non-fatal on failure).
	writeOpenCodeModelsCache(cachePath, models)

	return models
}

// OpenCodeModelOptions wraps LoadOpenCodeModels and returns []ModelOption suitable for
// huh Select fields. The inherit sentinel is prepended as the first option.
// Each model's Label and Value are both the bare model ID.
func OpenCodeModelOptions(fallback []string) []ModelOption {
	models := LoadOpenCodeModels(fallback)
	opts := make([]ModelOption, 0, len(models)+1)
	opts = append(opts, ModelOption{
		Label: SDDModelInheritOption,
		Value: SDDModelInheritOption,
	})
	for _, id := range models {
		opts = append(opts, ModelOption{
			Label: id,
			Value: id,
		})
	}
	return opts
}

// readOpenCodeCacheFile reads the JSON array of strings from the cache file.
// Returns nil if the file cannot be read or parsed.
func readOpenCodeCacheFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var models []string
	if err := json.Unmarshal(data, &models); err != nil {
		return nil
	}
	return models
}

// parseOpenCodeModels reads the source JSON, finds the github-copilot provider,
// extracts model IDs where tool_call=true, and returns them sorted alphabetically.
func parseOpenCodeModels(sourcePath string) ([]string, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}

	// Parse as a map of provider name -> provider object. We only need the
	// models sub-map for the target provider, so use a two-level decode.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	providerRaw, ok := root[openCodeProvider]
	if !ok {
		// Provider not found — return empty but non-nil result.
		return []string{}, nil
	}

	// Decode the provider object; we only care about the "models" field.
	var provider struct {
		Models map[string]openCodeModelEntry `json:"models"`
	}
	if err := json.Unmarshal(providerRaw, &provider); err != nil {
		return nil, err
	}

	var ids []string
	for id, entry := range provider.Models {
		if entry.ToolCall {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// writeOpenCodeModelsCache serialises models as a JSON array and writes it to cachePath.
// The parent directory is created if it does not exist.
// Failures are logged to stderr but not returned — cache write is non-fatal.
func writeOpenCodeModelsCache(cachePath string, models []string) {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "devrune: failed to create models cache dir: %v\n", err)
		return
	}

	data, err := json.Marshal(models)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devrune: failed to marshal models cache: %v\n", err)
		return
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "devrune: failed to write models cache %s: %v\n", cachePath, err)
	}
}
