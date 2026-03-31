// SPDX-License-Identifier: MIT

package renderers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// codexAgentDef returns a default Codex agent definition for tests.
// Matches the values in agents/codex.yaml.
func codexAgentDef() model.AgentDefinition {
	return model.AgentDefinition{
		Name:             "codex",
		Type:             "codex",
		Workspace:        ".codex",
		SkillDir:         "../.agents/skills",
		RulesDir:         "rules",
		CatalogFile:      "AGENTS.md",
		DefaultRules: "individual",
		MCP: &model.MCPConfig{
			FilePath:    "config.toml",
			RootKey:     "mcp_servers",
			EnvKey:      "env",
			EnvVarStyle: "${VAR}",
		},
		Settings: &model.SettingsConfig{Permissions: []string{}},
	}
}

// TestCodexRenderer_Name verifies Name() returns "codex".
func TestCodexRenderer_Name(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	if r.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", r.Name(), "codex")
	}
}

// TestCodexRenderer_AgentType verifies AgentType() returns "codex".
func TestCodexRenderer_AgentType(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	if r.AgentType() != "codex" {
		t.Errorf("AgentType() = %q, want %q", r.AgentType(), "codex")
	}
}

// TestCodexRenderer_NeedsCopyMode verifies NeedsCopyMode() returns true.
func TestCodexRenderer_NeedsCopyMode(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	if !r.NeedsCopyMode() {
		t.Error("NeedsCopyMode() = false, want true")
	}
}

// TestCodexRenderer_RenderSkill_Full tests rendering a full canonical skill against
// the golden expected output for Codex (drops unsupported frontmatter fields).
func TestCodexRenderer_RenderSkill_Full(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	inputPath := goldenInputPath(t, "canonical-full.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "codex-full.md")
}

// TestCodexRenderer_RenderSkill_Minimal tests rendering a minimal canonical skill.
func TestCodexRenderer_RenderSkill_Minimal(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	inputPath := goldenInputPath(t, "canonical-minimal.md")
	destDir := t.TempDir()

	if err := r.RenderSkill(inputPath, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	outputPath := filepath.Join(destDir, "SKILL.md")
	compareWithGolden(t, outputPath, "codex-minimal.md")
}

// TestCodexRenderer_DropsUnsupportedFields verifies that fields not supported by
// Codex are dropped from frontmatter: allowed-tools, argument-hint,
// disable-model-invocation, tools-mode, mode.
func TestCodexRenderer_DropsUnsupportedFields(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())

	input := `---
name: drop-test
description: Test
allowed-tools:
  - Bash
argument-hint: "[topic]"
disable-model-invocation: false
tools-mode: auto
mode: subagent
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	if err := r.RenderSkill(srcDir, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	fm, _, _ := parse.ParseFrontmatter(data)

	dropped := []string{"allowed-tools", "argument-hint", "disable-model-invocation", "tools-mode", "mode"}
	for _, field := range dropped {
		if _, ok := fm[field]; ok {
			t.Errorf("field %q should have been dropped for Codex", field)
		}
	}
}

// TestCodexRenderer_KeepsAllowedFields verifies that fields supported by Codex are kept:
// name, description, model, reasoning-effort, temperature.
func TestCodexRenderer_KeepsAllowedFields(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())

	input := `---
name: keep-test
description: Test keeping fields
model: sonnet
reasoning-effort: low
temperature: 0.5
allowed-tools:
  - Bash
---
Body.
`
	srcDir := writeSkillFile(t, input)
	destDir := t.TempDir()

	if err := r.RenderSkill(srcDir, destDir); err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	fm, _, _ := parse.ParseFrontmatter(data)

	kept := []string{"name", "description", "model", "reasoning-effort", "temperature"}
	for _, field := range kept {
		if _, ok := fm[field]; !ok {
			t.Errorf("field %q should have been kept for Codex", field)
		}
	}
	// allowed-tools must be dropped.
	if _, ok := fm["allowed-tools"]; ok {
		t.Error("allowed-tools should have been dropped for Codex")
	}
}

// TestCodexRenderer_TransformFrontmatter verifies the transformFrontmatter method
// via the exported wrapper in export_test.go.
func TestCodexRenderer_TransformFrontmatter(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())

	input := map[string]interface{}{
		"name":                    "my-skill",
		"description":             "A skill",
		"allowed-tools":           []string{"Bash"},
		"argument-hint":           "[topic]",
		"disable-model-invocation": false,
		"tools-mode":              "auto",
		"mode":                    "subagent",
		"model":                   "sonnet",
		"temperature":             0.7,
	}

	got := renderers.CodexTransformFrontmatter(r, input)

	// Must be dropped.
	dropped := []string{"allowed-tools", "argument-hint", "disable-model-invocation", "tools-mode", "mode"}
	for _, field := range dropped {
		if _, ok := got[field]; ok {
			t.Errorf("transformFrontmatter: field %q should have been dropped", field)
		}
	}

	// Must be kept.
	kept := []string{"name", "description", "model", "temperature"}
	for _, field := range kept {
		if _, ok := got[field]; !ok {
			t.Errorf("transformFrontmatter: field %q should have been kept", field)
		}
	}
}

// TestCodexRenderer_RenderMCPs_TOML verifies that RenderMCPs produces valid TOML
// output with the correct [mcp_servers.<name>] table structure.
func TestCodexRenderer_RenderMCPs_TOML(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCodexRenderer(codexAgentDef())

	// Build a fake MCP cache entry.
	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "abc123")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: engram
command: npx
args:
  - "-y"
  - "@anthropic/engram-server"
env:
  ENGRAM_PROJECT: "${ENGRAM_PROJECT}"
`
	if err := os.WriteFile(filepath.Join(mcpDir, "engram.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}

	cache := &fakeCacheStore{dirs: map[string]string{"abc123": mcpDir}}
	mcps := []model.LockedMCP{{Name: "engram", Hash: "abc123"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	// config.toml lives at workspaceRoot/config.toml (MCP filePath is "config.toml").
	content, err := os.ReadFile(filepath.Join(workspaceRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	tomlStr := string(content)

	// Must be valid TOML.
	var parsed map[string]interface{}
	if err := toml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("config.toml is not valid TOML: %v\ncontent:\n%s", err, tomlStr)
	}

	// Root key must be "mcp_servers".
	mcpServers, ok := parsed["mcp_servers"]
	if !ok {
		t.Fatalf("config.toml should have 'mcp_servers' root key; content:\n%s", tomlStr)
	}

	// Must contain the "engram" server entry.
	serversMap, ok := mcpServers.(map[string]interface{})
	if !ok {
		t.Fatalf("mcp_servers should be a map; got %T", mcpServers)
	}
	if _, ok := serversMap["engram"]; !ok {
		t.Errorf("mcp_servers should contain 'engram'; content:\n%s", tomlStr)
	}

	// Env var placeholder must use ${VAR} format.
	if !strings.Contains(tomlStr, "${ENGRAM_PROJECT}") {
		t.Errorf("config.toml should contain ${ENGRAM_PROJECT} env var format; content:\n%s", tomlStr)
	}
}

// TestCodexRenderer_RenderMCPs_EmptyList verifies no file is written when MCPs list is empty.
func TestCodexRenderer_RenderMCPs_EmptyList(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCodexRenderer(codexAgentDef())

	cache := &fakeCacheStore{dirs: map[string]string{}}
	if err := r.RenderMCPs([]model.LockedMCP{}, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs with empty list: %v", err)
	}

	// config.toml must NOT be written when there are no MCPs.
	configPath := filepath.Join(workspaceRoot, "config.toml")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("config.toml should NOT be written when MCP list is empty, but it exists")
	}
}

// TestCodexRenderer_RenderMCPs_TOMLTableStructure verifies the TOML uses
// nested table notation [mcp_servers.<name>] (not inline tables).
func TestCodexRenderer_RenderMCPs_TOMLTableStructure(t *testing.T) {
	workspaceRoot := t.TempDir()
	r := renderers.NewCodexRenderer(codexAgentDef())

	cacheDir := t.TempDir()
	mcpDir := filepath.Join(cacheDir, "hash1")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mcpYAML := `name: context7
command: npx
args:
  - "-y"
  - "@upstash/context7-mcp"
`
	if err := os.WriteFile(filepath.Join(mcpDir, "context7.yaml"), []byte(mcpYAML), 0o644); err != nil {
		t.Fatalf("write mcp yaml: %v", err)
	}

	cache := &fakeCacheStore{dirs: map[string]string{"hash1": mcpDir}}
	mcps := []model.LockedMCP{{Name: "context7", Hash: "hash1"}}

	if err := r.RenderMCPs(mcps, cache, workspaceRoot); err != nil {
		t.Fatalf("RenderMCPs: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workspaceRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}

	// The TOML must be parseable.
	var parsed map[string]interface{}
	if err := toml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("config.toml parse error: %v\ncontent:\n%s", err, content)
	}

	// Must have mcp_servers.context7 nested.
	mcpServers, _ := parsed["mcp_servers"].(map[string]interface{})
	if _, ok := mcpServers["context7"]; !ok {
		t.Errorf("mcp_servers.context7 must exist; content:\n%s", content)
	}
}

// TestCodexRenderer_Finalize_EnvResolution verifies that ${VAR} placeholders
// in config.toml are resolved from environment variables during Finalize.
func TestCodexRenderer_Finalize_EnvResolution(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())

	workspaceRoot := t.TempDir()

	// Write a config.toml with a ${VAR} placeholder (same format as TOML output).
	tomlContent := "[mcp_servers.engram]\ncommand = \"npx\"\n\n[mcp_servers.engram.env]\nENGRAM_PROJECT = \"${CODEX_TEST_VAR}\"\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Set the env var.
	t.Setenv("CODEX_TEST_VAR", "my-project-resolved")

	if err := r.Finalize(workspaceRoot); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	result, err := os.ReadFile(filepath.Join(workspaceRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml after Finalize: %v", err)
	}
	resultStr := string(result)

	if !strings.Contains(resultStr, "my-project-resolved") {
		t.Errorf("env var not resolved; content:\n%s", resultStr)
	}
	if strings.Contains(resultStr, "${CODEX_TEST_VAR}") {
		t.Error("placeholder still present after Finalize resolution")
	}
}

// TestCodexRenderer_Finalize_NoFile verifies Finalize is a no-op when config.toml is absent.
func TestCodexRenderer_Finalize_NoFile(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	if err := r.Finalize(t.TempDir()); err != nil {
		t.Errorf("Finalize on empty dir: unexpected error: %v", err)
	}
}

// TestCodexRenderer_Finalize_KeepsUnresolvedPlaceholders verifies that unset env vars
// leave the placeholder unchanged (graceful degradation).
func TestCodexRenderer_Finalize_KeepsUnresolvedPlaceholders(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())

	workspaceRoot := t.TempDir()
	tomlContent := "[mcp_servers.test]\ncommand = \"${UNSET_CODEX_VAR_99999}\"\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	_ = os.Unsetenv("UNSET_CODEX_VAR_99999")

	if err := r.Finalize(workspaceRoot); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	result, err := os.ReadFile(filepath.Join(workspaceRoot, "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	if !strings.Contains(string(result), "${UNSET_CODEX_VAR_99999}") {
		t.Error("placeholder should be kept when env var is not set")
	}
}

// TestCodexRenderer_RenderSettings_Noop verifies that RenderSettings is a no-op
// for Codex v1 (returns nil without writing any files).
func TestCodexRenderer_RenderSettings_Noop(t *testing.T) {
	r := renderers.NewCodexRenderer(codexAgentDef())
	workspaceRoot := t.TempDir()

	if err := r.RenderSettings(workspaceRoot, nil, nil); err != nil {
		t.Errorf("RenderSettings: expected nil error, got: %v", err)
	}
}
