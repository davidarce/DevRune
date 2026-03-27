package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildOpenCodeSourceJSON builds a minimal models.json payload for testing.
// provider is the provider key (e.g. "github-copilot").
// models maps model ID → tool_call bool.
func buildOpenCodeSourceJSON(t *testing.T, provider string, models map[string]bool) []byte {
	t.Helper()
	type entry struct {
		ToolCall bool `json:"tool_call"`
	}
	type providerObj struct {
		Models map[string]entry `json:"models"`
	}
	root := map[string]providerObj{}
	entries := make(map[string]entry, len(models))
	for id, tc := range models {
		entries[id] = entry{ToolCall: tc}
	}
	root[provider] = providerObj{Models: entries}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("buildOpenCodeSourceJSON: marshal: %v", err)
	}
	return data
}

// writeFile writes content to path, creating parent dirs as needed.
func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("writeFile: mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("writeFile: write %s: %v", path, err)
	}
}

// setMtime sets the mtime of path to the given time.
func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("setMtime: %v", err)
	}
}

// TestLoadOpenCodeModels_ValidSource tests normal operation with valid source JSON.
func TestLoadOpenCodeModels_ValidSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source", "models.json")
	cachePath := filepath.Join(dir, "cache", "opencode-copilot.json")

	// Build source with 3 models, 2 tool_call=true, 1 false.
	srcData := buildOpenCodeSourceJSON(t, openCodeProvider, map[string]bool{
		"gpt-4o":    true,
		"gpt-4o-mini": true,
		"ada":       false,
	})
	writeFile(t, srcPath, srcData)

	// Inject paths.
	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	got := LoadOpenCodeModels([]string{"fallback"})

	// Should return the 2 tool_call=true models, sorted.
	want := []string{"gpt-4o", "gpt-4o-mini"}
	if len(got) != len(want) {
		t.Fatalf("LoadOpenCodeModels() = %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
}

// TestLoadOpenCodeModels_MissingFile tests fallback when source file is absent.
func TestLoadOpenCodeModels_MissingFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "nonexistent.json")
	cachePath := filepath.Join(dir, "cache.json")

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	fallback := []string{"model-a", "model-b"}
	got := LoadOpenCodeModels(fallback)

	if len(got) != len(fallback) {
		t.Fatalf("LoadOpenCodeModels() = %v, want fallback %v", got, fallback)
	}
	for i, v := range fallback {
		if got[i] != v {
			t.Errorf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
}

// TestLoadOpenCodeModels_MalformedJSON tests fallback when source JSON is invalid.
func TestLoadOpenCodeModels_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	writeFile(t, srcPath, []byte("this is not json"))

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	fallback := []string{"fallback-model"}
	got := LoadOpenCodeModels(fallback)

	if len(got) != 1 || got[0] != "fallback-model" {
		t.Errorf("LoadOpenCodeModels() = %v, want fallback %v", got, fallback)
	}
}

// TestLoadOpenCodeModels_CacheHit tests that a fresh cache is returned without re-parsing source.
func TestLoadOpenCodeModels_CacheHit(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	// Write source with one model.
	srcData := buildOpenCodeSourceJSON(t, openCodeProvider, map[string]bool{
		"model-from-source": true,
	})
	writeFile(t, srcPath, srcData)

	// Write cache with a different model list, mtime AFTER source.
	cacheData, _ := json.Marshal([]string{"model-from-cache"})
	writeFile(t, cachePath, cacheData)

	// Set source mtime to past, cache mtime to now (cache is newer → cache hit).
	past := time.Now().Add(-10 * time.Minute)
	setMtime(t, srcPath, past)
	// cachePath is already newer (written after source mtime was set to past).

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	got := LoadOpenCodeModels(nil)

	// Should return cached list, not parsed source.
	if len(got) != 1 || got[0] != "model-from-cache" {
		t.Errorf("LoadOpenCodeModels() = %v, want [model-from-cache] (cache hit)", got)
	}
}

// TestLoadOpenCodeModels_NoGithubCopilotProvider tests that missing provider returns empty list (not fallback).
func TestLoadOpenCodeModels_NoGithubCopilotProvider(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	// Source has a different provider, not github-copilot.
	srcData := buildOpenCodeSourceJSON(t, "openai", map[string]bool{
		"gpt-4o": true,
	})
	writeFile(t, srcPath, srcData)

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	got := LoadOpenCodeModels([]string{"fallback"})

	// Provider not found → empty list (not fallback).
	if len(got) != 0 {
		t.Errorf("LoadOpenCodeModels() = %v, want empty list when provider absent", got)
	}
}

// TestLoadOpenCodeModels_ToolCallFalseFiltered tests that models with tool_call=false are excluded.
func TestLoadOpenCodeModels_ToolCallFalseFiltered(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	srcData := buildOpenCodeSourceJSON(t, openCodeProvider, map[string]bool{
		"capable-model":   true,
		"incapable-model": false,
	})
	writeFile(t, srcPath, srcData)

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	got := LoadOpenCodeModels(nil)

	if len(got) != 1 || got[0] != "capable-model" {
		t.Errorf("LoadOpenCodeModels() = %v, want [capable-model]", got)
	}
}

// TestLoadOpenCodeModels_CacheStaleness tests that a stale cache triggers re-parse of source.
func TestLoadOpenCodeModels_CacheStaleness(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	// Write cache first (stale).
	cacheData, _ := json.Marshal([]string{"stale-model"})
	writeFile(t, cachePath, cacheData)

	// Set cache mtime to the past.
	past := time.Now().Add(-10 * time.Minute)
	setMtime(t, cachePath, past)

	// Write source AFTER cache (source is newer → cache stale).
	srcData := buildOpenCodeSourceJSON(t, openCodeProvider, map[string]bool{
		"fresh-model": true,
	})
	writeFile(t, srcPath, srcData)

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	got := LoadOpenCodeModels(nil)

	// Should re-parse source, not use stale cache.
	if len(got) != 1 || got[0] != "fresh-model" {
		t.Errorf("LoadOpenCodeModels() = %v, want [fresh-model] (stale cache bypass)", got)
	}
}

// TestOpenCodeModelOptions_PrependsInheritSentinel tests that OpenCodeModelOptions
// prepends the inherit sentinel before the model list.
func TestOpenCodeModelOptions_PrependsInheritSentinel(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "models.json")
	cachePath := filepath.Join(dir, "cache.json")

	srcData := buildOpenCodeSourceJSON(t, openCodeProvider, map[string]bool{
		"alpha": true,
		"beta":  true,
	})
	writeFile(t, srcPath, srcData)

	openCodeModelsSource = srcPath
	openCodeModelsCache = cachePath
	defer func() {
		openCodeModelsSource = ""
		openCodeModelsCache = ""
	}()

	opts := OpenCodeModelOptions(nil)

	if len(opts) == 0 {
		t.Fatal("OpenCodeModelOptions() returned empty slice")
	}

	// First option must be the inherit sentinel.
	if opts[0].Value != SDDModelInheritOption {
		t.Errorf("opts[0].Value = %q, want inherit sentinel %q", opts[0].Value, SDDModelInheritOption)
	}
	if opts[0].Label != SDDModelInheritOption {
		t.Errorf("opts[0].Label = %q, want inherit sentinel %q", opts[0].Label, SDDModelInheritOption)
	}

	// Remaining options should be the models.
	if len(opts) < 3 {
		t.Fatalf("OpenCodeModelOptions() returned %d options, want at least 3", len(opts))
	}
}
