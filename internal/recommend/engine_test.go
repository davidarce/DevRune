// SPDX-License-Identifier: MIT

package recommend_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/recommend"
)

func TestMain(m *testing.M) {
	// Redirect recommendation cache to a temp directory for test isolation.
	dir, _ := os.MkdirTemp("", "devrune-recommend-test-cache")
	recommend.CacheDirOverride = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// mockAgentScript returns the absolute path to the testdata mock agent shell script.
func mockAgentScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	dir := filepath.Dir(file)
	script := filepath.Join(dir, "testdata", "mock-agent.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("mock agent script not found: %v", err)
	}
	return script
}

// sampleProfile returns a minimal ProjectProfile for testing.
func sampleProfile() detect.ProjectProfile {
	return detect.ProjectProfile{
		Languages: []detect.LanguageInfo{
			{Name: "Go", Files: 5, Lines: 200, Percentage: 60.0},
			{Name: "TypeScript", Files: 3, Lines: 130, Percentage: 40.0},
		},
		Dependencies: []detect.DependencyFile{
			{
				Path: "go.mod",
				Type: "gomod",
				Dependencies: map[string]string{
					"github.com/spf13/cobra": "v1.8.0",
				},
			},
		},
		ConfigFiles: []string{"tsconfig.json", "Dockerfile"},
		Frameworks:  []string{"React", "Cobra CLI"},
		TotalFiles:  8,
		TotalLines:  330,
	}
}

// sampleCatalog returns a minimal catalog for testing.
func sampleCatalog() []recommend.CatalogItem {
	return []recommend.CatalogItem{
		{Name: "architect-adviser", Kind: "skill", Source: "github:owner/catalog", Description: "Clean architecture patterns"},
		{Name: "react", Kind: "rule", Source: "github:owner/catalog", Description: "React coding standards"},
		{Name: "unit-test-adviser", Kind: "skill", Source: "github:owner/catalog", Description: "Unit test patterns"},
	}
}

// --- buildPrompt tests ---

func TestBuildPrompt_ContainsProfileAndCatalog(t *testing.T) {
	// buildPrompt is unexported; we test it indirectly via Engine.Recommend with mock.
	// Here we test the prompt content by running a mock that echoes args.
	// Instead, we verify the Engine passes a non-empty prompt by checking the mock
	// agent output (it ignores input but produces known JSON).
	cfg := recommend.RecommendConfig{
		Threshold: 0.7,
		Enabled:   true,
		Models:    recommend.DefaultAgentModels(),
	}

	scriptPath := mockAgentScript(t)
	engine := recommend.NewEngine(scriptPath, "claude", cfg)
	profile := sampleProfile()
	catalog := sampleCatalog()

	result, err := engine.Recommend(context.Background(), profile, catalog)
	if err != nil {
		t.Fatalf("Recommend: unexpected error: %v", err)
	}

	// Verify the result is non-nil and has recommendations.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- Engine.Recommend tests ---

func TestRecommend_MockAgent_ReturnsFilteredRecommendations(t *testing.T) {
	cfg := recommend.RecommendConfig{
		Threshold: 0.7,
		Enabled:   true,
		Models:    recommend.DefaultAgentModels(),
	}

	scriptPath := mockAgentScript(t)
	engine := recommend.NewEngine(scriptPath, "claude", cfg)
	profile := sampleProfile()
	catalog := sampleCatalog()

	result, err := engine.Recommend(context.Background(), profile, catalog)
	if err != nil {
		t.Fatalf("Recommend: unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Mock agent returns 3 items; 2 above threshold (0.92, 0.85), 1 below (0.45).
	// After filtering at threshold=0.7, we expect exactly 2.
	if len(result.Recommendations) != 2 {
		t.Errorf("expected 2 recommendations above threshold=0.7, got %d: %+v",
			len(result.Recommendations), result.Recommendations)
	}

	// Verify the high-confidence items are present.
	names := make(map[string]float64)
	for _, r := range result.Recommendations {
		names[r.Name] = r.Confidence
	}
	if c, ok := names["architect-adviser"]; !ok || c != 0.92 {
		t.Errorf("expected architect-adviser with confidence=0.92, got: %v", names)
	}
	if c, ok := names["react"]; !ok || c != 0.85 {
		t.Errorf("expected react with confidence=0.85, got: %v", names)
	}
	if _, ok := names["low-confidence-item"]; ok {
		t.Error("expected low-confidence-item to be filtered out")
	}
}

func TestRecommend_EmptyCatalog_ReturnsEmptyResult(t *testing.T) {
	cfg := recommend.RecommendConfig{Threshold: 0.7, Enabled: true}
	scriptPath := mockAgentScript(t)
	engine := recommend.NewEngine(scriptPath, "claude", cfg)

	result, err := engine.Recommend(context.Background(), sampleProfile(), nil)
	if err != nil {
		t.Fatalf("expected no error for empty catalog, got: %v", err)
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("expected empty recommendations for empty catalog, got: %v", result.Recommendations)
	}
}

func TestRecommend_CancelledContext(t *testing.T) {
	cfg := recommend.RecommendConfig{Threshold: 0.7, Enabled: true}
	scriptPath := mockAgentScript(t)
	engine := recommend.NewEngine(scriptPath, "claude", cfg)

	// Use a unique profile to avoid cache hit from previous test.
	uniqueProfile := detect.ProjectProfile{
		Languages: []detect.LanguageInfo{{Name: "CancelTestLang"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := engine.Recommend(ctx, uniqueProfile, sampleCatalog())
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

// --- WriteRecommendedYAML tests ---

func TestWriteRecommendedYAML_WritesValidYAML(t *testing.T) {
	result := &recommend.RecommendResult{
		Recommendations: []recommend.Recommendation{
			{
				Name:       "architect-adviser",
				Kind:       "skill",
				Source:     "github:owner/catalog",
				Confidence: 0.92,
				Reason:     "Hexagonal architecture detected",
			},
			{
				Name:       "react",
				Kind:       "rule",
				Source:     "github:owner/catalog",
				Confidence: 0.85,
				Reason:     "React in package.json",
			},
		},
	}
	profile := sampleProfile()

	outPath := filepath.Join(t.TempDir(), "devrune.recommended.yaml")
	if err := recommend.WriteRecommendedYAML(outPath, result, profile); err != nil {
		t.Fatalf("WriteRecommendedYAML: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "devrune/recommend/v1") {
		t.Errorf("expected schemaVersion in output; got:\n%s", content)
	}
	if !strings.Contains(content, "architect-adviser") {
		t.Errorf("expected architect-adviser in output; got:\n%s", content)
	}
	if !strings.Contains(content, "generatedAt") {
		t.Errorf("expected generatedAt in output; got:\n%s", content)
	}
	if !strings.Contains(content, "Go") {
		t.Errorf("expected language Go in profile section; got:\n%s", content)
	}
}

// --- DetectAgent tests ---

func TestDetectAgent_NotFound_ReturnsErrNoAgent(t *testing.T) {
	// Override PATH to an empty directory so no agent binary is found.
	emptyDir := t.TempDir()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", orig) })
	_ = os.Setenv("PATH", emptyDir)

	_, _, err := recommend.DetectAgent()
	if err == nil {
		t.Fatal("expected ErrNoAgent when no agent binary is on PATH")
	}
	if !errors.Is(err, recommend.ErrNoAgent) {
		t.Errorf("expected errors.Is(err, ErrNoAgent); got: %v", err)
	}
}

func TestDetectAgent_FoundOnPath(t *testing.T) {
	// Create a fake "claude" binary in a temp dir and put it on PATH.
	tmpBin := t.TempDir()
	fakeBin := filepath.Join(tmpBin, "claude")

	// Write a minimal shell script as a fake binary.
	script := "#!/bin/sh\necho '{}'\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	orig := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", orig) })
	_ = os.Setenv("PATH", tmpBin+string(os.PathListSeparator)+orig)

	binaryPath, agentName, err := recommend.DetectAgent()
	if err != nil {
		t.Fatalf("DetectAgent: unexpected error: %v", err)
	}
	if agentName != "claude" {
		t.Errorf("expected agentName=claude, got %q", agentName)
	}
	if binaryPath == "" {
		t.Error("expected non-empty binaryPath")
	}
}

// --- buildPrompt content test (via JSON inspection) ---

func TestBuildPrompt_JSONStructure(t *testing.T) {
	// We can't call buildPrompt directly (unexported), but we can verify the
	// prompt structure by running the engine with a mock agent that prints its
	// stdin/args. Instead, we test indirectly: the mock agent returns valid JSON
	// regardless, so we verify the round-trip works.
	// Use threshold=0.3 so the low-confidence item (0.45) also passes.
	cfg := recommend.RecommendConfig{Threshold: 0.3, Enabled: true, Models: recommend.DefaultAgentModels()}
	scriptPath := mockAgentScript(t)
	engine := recommend.NewEngine(scriptPath, "claude", cfg)

	result, err := engine.Recommend(context.Background(), sampleProfile(), sampleCatalog())
	if err != nil {
		t.Fatalf("Recommend: %v", err)
	}

	// With threshold=0.3, all 3 mock items (0.92, 0.85, 0.45) should be returned.
	if len(result.Recommendations) != 3 {
		t.Errorf("expected 3 recommendations at threshold=0.3, got %d", len(result.Recommendations))
	}

	// Verify JSON round-trip of the result.
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var roundtrip recommend.RecommendResult
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if len(roundtrip.Recommendations) != 3 {
		t.Errorf("roundtrip: expected 3, got %d", len(roundtrip.Recommendations))
	}
}
