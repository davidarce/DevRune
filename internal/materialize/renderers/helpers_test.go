// SPDX-License-Identifier: MIT

package renderers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
)

// TestTransformEnvVarValues_Copilot verifies ${VAR} → ${env:VAR} for Copilot.
func TestTransformEnvVarValues_Copilot(t *testing.T) {
	input := map[string]interface{}{
		"command": "npx",
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
			"STATIC_VAL":  "not-a-placeholder",
		},
	}

	result := renderers.TransformEnvVarValues(input, "copilot")

	envMap, ok := result["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'env' is not map[string]interface{}, got %T", result["env"])
	}

	if envMap["EXA_API_KEY"] != "${env:EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q", envMap["EXA_API_KEY"], "${env:EXA_API_KEY}")
	}
	// Non-placeholder values must pass through unchanged.
	if envMap["STATIC_VAL"] != "not-a-placeholder" {
		t.Errorf("STATIC_VAL = %q, want %q", envMap["STATIC_VAL"], "not-a-placeholder")
	}
	// Other keys (command) must be preserved.
	if result["command"] != "npx" {
		t.Errorf("command = %v, want %q", result["command"], "npx")
	}
}

// TestTransformEnvVarValues_OpenCode verifies ${VAR} → {env:VAR} for OpenCode.
func TestTransformEnvVarValues_OpenCode(t *testing.T) {
	input := map[string]interface{}{
		"command": "npx",
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
		},
	}

	result := renderers.TransformEnvVarValues(input, "opencode")

	envMap, ok := result["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'env' is not map[string]interface{}, got %T", result["env"])
	}

	if envMap["EXA_API_KEY"] != "{env:EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q", envMap["EXA_API_KEY"], "{env:EXA_API_KEY}")
	}
}

// TestTransformEnvVarValues_EnvironmentKey verifies the "environment" key is also transformed.
func TestTransformEnvVarValues_EnvironmentKey(t *testing.T) {
	input := map[string]interface{}{
		"environment": map[string]interface{}{
			"MY_TOKEN": "${MY_TOKEN}",
		},
	}

	result := renderers.TransformEnvVarValues(input, "copilot")

	envMap, ok := result["environment"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'environment' is not map[string]interface{}, got %T", result["environment"])
	}

	if envMap["MY_TOKEN"] != "${env:MY_TOKEN}" {
		t.Errorf("MY_TOKEN = %q, want %q", envMap["MY_TOKEN"], "${env:MY_TOKEN}")
	}
}

// TestTransformEnvVarValues_NonPlaceholderUnchanged verifies that values that do not
// match the ${VAR_NAME} pattern are returned unchanged for all formats.
func TestTransformEnvVarValues_NonPlaceholderUnchanged(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"RAW":     "already-resolved-value",
			"EMPTY":   "",
			"PARTIAL": "${INCOMPLETE",
		},
	}

	for _, format := range []string{"copilot", "opencode"} {
		result := renderers.TransformEnvVarValues(input, format)
		envMap := result["env"].(map[string]interface{})

		if envMap["RAW"] != "already-resolved-value" {
			t.Errorf("[%s] RAW = %q, want %q", format, envMap["RAW"], "already-resolved-value")
		}
		if envMap["EMPTY"] != "" {
			t.Errorf("[%s] EMPTY = %q, want empty string", format, envMap["EMPTY"])
		}
		if envMap["PARTIAL"] != "${INCOMPLETE" {
			t.Errorf("[%s] PARTIAL = %q, want %q", format, envMap["PARTIAL"], "${INCOMPLETE")
		}
	}
}

// TestTransformEnvVarValues_DoesNotMutateOriginal verifies the input map is not modified.
func TestTransformEnvVarValues_DoesNotMutateOriginal(t *testing.T) {
	originalEnv := map[string]interface{}{
		"API_KEY": "${API_KEY}",
	}
	input := map[string]interface{}{
		"env": originalEnv,
	}

	_ = renderers.TransformEnvVarValues(input, "copilot")

	// Original env map must still hold the original value.
	if originalEnv["API_KEY"] != "${API_KEY}" {
		t.Errorf("original env was mutated: API_KEY = %q", originalEnv["API_KEY"])
	}
}

// TestTransformEnvVarValues_BothEnvKeys verifies that if a server config has both
// "env" and "environment" keys, both are transformed.
func TestTransformEnvVarValues_BothEnvKeys(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"VAR_A": "${VAR_A}",
		},
		"environment": map[string]interface{}{
			"VAR_B": "${VAR_B}",
		},
	}

	result := renderers.TransformEnvVarValues(input, "opencode")

	envMap := result["env"].(map[string]interface{})
	if envMap["VAR_A"] != "{env:VAR_A}" {
		t.Errorf("env.VAR_A = %q, want %q", envMap["VAR_A"], "{env:VAR_A}")
	}

	envMap2 := result["environment"].(map[string]interface{})
	if envMap2["VAR_B"] != "{env:VAR_B}" {
		t.Errorf("environment.VAR_B = %q, want %q", envMap2["VAR_B"], "{env:VAR_B}")
	}
}

// TestTransformEnvVarValues_NoEnvKey verifies a server config with no env key is returned
// as-is (shallow copy with all other fields intact).
func TestTransformEnvVarValues_NoEnvKey(t *testing.T) {
	input := map[string]interface{}{
		"command": "node",
		"args":    []interface{}{"server.js"},
		"type":    "local",
	}

	result := renderers.TransformEnvVarValues(input, "copilot")

	if result["command"] != "node" {
		t.Errorf("command = %v, want %q", result["command"], "node")
	}
	if result["type"] != "local" {
		t.Errorf("type = %v, want %q", result["type"], "local")
	}
}

// ---------------------------------------------------------------------------
// Tests for EffectiveMCPConfig
// ---------------------------------------------------------------------------

// TestEffectiveMCPConfig_NilReturnsDefaults verifies that a nil input returns
// the Claude-compatible defaults (backward-compatible behavior).
func TestEffectiveMCPConfig_NilReturnsDefaults(t *testing.T) {
	got := renderers.EffectiveMCPConfig(nil)

	if got.FilePath != "../.mcp.json" {
		t.Errorf("FilePath = %q, want %q", got.FilePath, "../.mcp.json")
	}
	if got.RootKey != "mcpServers" {
		t.Errorf("RootKey = %q, want %q", got.RootKey, "mcpServers")
	}
	if got.EnvKey != "env" {
		t.Errorf("EnvKey = %q, want %q", got.EnvKey, "env")
	}
	if got.EnvVarStyle != "${VAR}" {
		t.Errorf("EnvVarStyle = %q, want %q", got.EnvVarStyle, "${VAR}")
	}
}

// TestEffectiveMCPConfig_PartialFillsGaps verifies that only empty fields are
// filled with defaults while non-empty fields are preserved.
func TestEffectiveMCPConfig_PartialFillsGaps(t *testing.T) {
	input := &model.MCPConfig{
		FilePath: "mcp.json", // explicitly set
		// RootKey, EnvKey, EnvVarStyle are empty — should be filled with defaults
	}

	got := renderers.EffectiveMCPConfig(input)

	if got.FilePath != "mcp.json" {
		t.Errorf("FilePath = %q, want %q", got.FilePath, "mcp.json")
	}
	if got.RootKey != "mcpServers" {
		t.Errorf("RootKey = %q, want %q (default should fill gap)", got.RootKey, "mcpServers")
	}
	if got.EnvKey != "env" {
		t.Errorf("EnvKey = %q, want %q (default should fill gap)", got.EnvKey, "env")
	}
	if got.EnvVarStyle != "${VAR}" {
		t.Errorf("EnvVarStyle = %q, want %q (default should fill gap)", got.EnvVarStyle, "${VAR}")
	}
}

// TestEffectiveMCPConfig_FullInputPassesThrough verifies that a fully populated
// MCPConfig is returned unchanged (no defaults override existing values).
func TestEffectiveMCPConfig_FullInputPassesThrough(t *testing.T) {
	input := &model.MCPConfig{
		FilePath:    "../.vscode/mcp.json",
		RootKey:     "servers",
		EnvKey:      "env",
		EnvVarStyle: "${env:VAR}",
	}

	got := renderers.EffectiveMCPConfig(input)

	if got.FilePath != "../.vscode/mcp.json" {
		t.Errorf("FilePath = %q, want %q", got.FilePath, "../.vscode/mcp.json")
	}
	if got.RootKey != "servers" {
		t.Errorf("RootKey = %q, want %q", got.RootKey, "servers")
	}
	if got.EnvKey != "env" {
		t.Errorf("EnvKey = %q, want %q", got.EnvKey, "env")
	}
	if got.EnvVarStyle != "${env:VAR}" {
		t.Errorf("EnvVarStyle = %q, want %q", got.EnvVarStyle, "${env:VAR}")
	}
}

// TestEffectiveMCPConfig_TableDriven runs a matrix of input configurations to verify
// the merging logic is consistent.
func TestEffectiveMCPConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		input       *model.MCPConfig
		wantPath    string
		wantRootKey string
		wantEnvKey  string
		wantStyle   string
	}{
		{
			name:        "nil input → all defaults",
			input:       nil,
			wantPath:    "../.mcp.json",
			wantRootKey: "mcpServers",
			wantEnvKey:  "env",
			wantStyle:   "${VAR}",
		},
		{
			name:        "opencode config → custom values preserved",
			input:       &model.MCPConfig{FilePath: "opencode.json", RootKey: "mcp", EnvKey: "environment", EnvVarStyle: "{env:VAR}"},
			wantPath:    "opencode.json",
			wantRootKey: "mcp",
			wantEnvKey:  "environment",
			wantStyle:   "{env:VAR}",
		},
		{
			name:        "copilot config → custom values preserved",
			input:       &model.MCPConfig{FilePath: "../.vscode/mcp.json", RootKey: "servers", EnvKey: "env", EnvVarStyle: "${env:VAR}"},
			wantPath:    "../.vscode/mcp.json",
			wantRootKey: "servers",
			wantEnvKey:  "env",
			wantStyle:   "${env:VAR}",
		},
		{
			name:        "only RootKey set → other fields default",
			input:       &model.MCPConfig{RootKey: "customServers"},
			wantPath:    "../.mcp.json",
			wantRootKey: "customServers",
			wantEnvKey:  "env",
			wantStyle:   "${VAR}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderers.EffectiveMCPConfig(tt.input)
			if got.FilePath != tt.wantPath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantPath)
			}
			if got.RootKey != tt.wantRootKey {
				t.Errorf("RootKey = %q, want %q", got.RootKey, tt.wantRootKey)
			}
			if got.EnvKey != tt.wantEnvKey {
				t.Errorf("EnvKey = %q, want %q", got.EnvKey, tt.wantEnvKey)
			}
			if got.EnvVarStyle != tt.wantStyle {
				t.Errorf("EnvVarStyle = %q, want %q", got.EnvVarStyle, tt.wantStyle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for ResolveMCPOutputPath
// ---------------------------------------------------------------------------

// TestResolveMCPOutputPath_TableDriven verifies path resolution for various
// workspace roots and relative MCPConfig.FilePath values.
func TestResolveMCPOutputPath_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		workspaceRoot string
		mcpConfig     model.MCPConfig
		wantPath      string
	}{
		{
			name:          "claude: workspace .claude + ../.mcp.json",
			workspaceRoot: "/project/.claude",
			mcpConfig:     model.MCPConfig{FilePath: "../.mcp.json"},
			wantPath:      filepath.Join("/project/.claude", "../.mcp.json"),
		},
		{
			name:          "factory: workspace .factory + mcp.json",
			workspaceRoot: "/project/.factory",
			mcpConfig:     model.MCPConfig{FilePath: "mcp.json"},
			wantPath:      filepath.Join("/project/.factory", "mcp.json"),
		},
		{
			name:          "opencode: workspace .opencode + opencode.json",
			workspaceRoot: "/project/.opencode",
			mcpConfig:     model.MCPConfig{FilePath: "opencode.json"},
			wantPath:      filepath.Join("/project/.opencode", "opencode.json"),
		},
		{
			name:          "copilot: workspace .github + ../.vscode/mcp.json",
			workspaceRoot: "/project/.github",
			mcpConfig:     model.MCPConfig{FilePath: "../.vscode/mcp.json"},
			wantPath:      filepath.Join("/project/.github", "../.vscode/mcp.json"),
		},
		{
			name:          "absolute file path is joined to workspace root",
			workspaceRoot: "/workspace",
			mcpConfig:     model.MCPConfig{FilePath: "subdir/mcp.json"},
			wantPath:      filepath.Join("/workspace", "subdir/mcp.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderers.ResolveMCPOutputPath(tt.workspaceRoot, tt.mcpConfig)
			if got != tt.wantPath {
				t.Errorf("ResolveMCPOutputPath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for ApplyMCPEnvTransform
// ---------------------------------------------------------------------------

// TestApplyMCPEnvTransform_CopilotStyle verifies ${EXA_API_KEY} → ${env:EXA_API_KEY}.
func TestApplyMCPEnvTransform_CopilotStyle(t *testing.T) {
	input := map[string]interface{}{
		"command": "npx",
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	envMap, ok := result["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'env' is not map[string]interface{}, got %T", result["env"])
	}
	if envMap["EXA_API_KEY"] != "${env:EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q", envMap["EXA_API_KEY"], "${env:EXA_API_KEY}")
	}
	if result["command"] != "npx" {
		t.Errorf("command = %v, want %q", result["command"], "npx")
	}
}

// TestApplyMCPEnvTransform_OpenCodeStyle verifies ${EXA_API_KEY} → {env:EXA_API_KEY}.
func TestApplyMCPEnvTransform_OpenCodeStyle(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "environment", EnvVarStyle: "{env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	// Source key "env" should be renamed to "environment".
	if _, ok := result["env"]; ok {
		t.Error("source key 'env' should have been renamed to 'environment'")
	}
	envMap, ok := result["environment"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'environment' is not map[string]interface{}, got %T", result["environment"])
	}
	if envMap["EXA_API_KEY"] != "{env:EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q", envMap["EXA_API_KEY"], "{env:EXA_API_KEY}")
	}
}

// TestApplyMCPEnvTransform_ClaudeDefaultStyle verifies ${EXA_API_KEY} stays unchanged
// when EnvVarStyle is "${VAR}" (identity transform).
func TestApplyMCPEnvTransform_ClaudeDefaultStyle(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	envMap, ok := result["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'env' is not map[string]interface{}, got %T", result["env"])
	}
	if envMap["EXA_API_KEY"] != "${EXA_API_KEY}" {
		t.Errorf("EXA_API_KEY = %q, want %q (should be unchanged)", envMap["EXA_API_KEY"], "${EXA_API_KEY}")
	}
}

// TestApplyMCPEnvTransform_EnvKeyRename verifies source "env" is renamed when
// mcpConfig.EnvKey = "environment".
func TestApplyMCPEnvTransform_EnvKeyRename(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"MY_TOKEN": "${MY_TOKEN}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "environment", EnvVarStyle: "{env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	if _, ok := result["env"]; ok {
		t.Error("original 'env' key should have been removed after rename")
	}
	if _, ok := result["environment"]; !ok {
		t.Error("renamed 'environment' key should be present")
	}
}

// TestApplyMCPEnvTransform_EnvKeyPreserved verifies source "env" stays as "env" when
// mcpConfig.EnvKey = "env".
func TestApplyMCPEnvTransform_EnvKeyPreserved(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"MY_TOKEN": "${MY_TOKEN}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	if _, ok := result["env"]; !ok {
		t.Error("'env' key should remain when EnvKey is 'env'")
	}
}

// TestApplyMCPEnvTransform_NonPlaceholderUnchanged verifies non-placeholder values
// are not modified by the transform.
func TestApplyMCPEnvTransform_NonPlaceholderUnchanged(t *testing.T) {
	input := map[string]interface{}{
		"env": map[string]interface{}{
			"STATIC_VAL": "already-resolved",
			"EMPTY":      "",
			"PARTIAL":    "${INCOMPLETE",
			"HAS_SPACE":  "${VAR NAME}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	envMap, ok := result["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("result 'env' is not map[string]interface{}")
	}
	if envMap["STATIC_VAL"] != "already-resolved" {
		t.Errorf("STATIC_VAL = %q, want %q", envMap["STATIC_VAL"], "already-resolved")
	}
	if envMap["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want empty string", envMap["EMPTY"])
	}
	if envMap["PARTIAL"] != "${INCOMPLETE" {
		t.Errorf("PARTIAL = %q, want %q", envMap["PARTIAL"], "${INCOMPLETE")
	}
	if envMap["HAS_SPACE"] != "${VAR NAME}" {
		t.Errorf("HAS_SPACE = %q, want %q", envMap["HAS_SPACE"], "${VAR NAME}")
	}
}

// TestApplyMCPEnvTransform_DoesNotMutateOriginal verifies the original map is not
// modified (immutability guarantee).
func TestApplyMCPEnvTransform_DoesNotMutateOriginal(t *testing.T) {
	originalEnv := map[string]interface{}{
		"API_KEY": "${API_KEY}",
	}
	input := map[string]interface{}{
		"env": originalEnv,
	}
	cfg := model.MCPConfig{EnvKey: "environment", EnvVarStyle: "{env:VAR}"}

	_ = renderers.ApplyMCPEnvTransform(input, cfg)

	// Original env map must still hold the original value.
	if originalEnv["API_KEY"] != "${API_KEY}" {
		t.Errorf("original env was mutated: API_KEY = %q", originalEnv["API_KEY"])
	}
	// Original input map must still have 'env' key.
	if _, ok := input["env"]; !ok {
		t.Error("original input map 'env' key was removed (mutation detected)")
	}
}

// TestApplyMCPEnvTransform_NoEnvKey verifies a server config with no env key is returned
// as-is (all fields preserved, no transform applied).
func TestApplyMCPEnvTransform_NoEnvKey(t *testing.T) {
	input := map[string]interface{}{
		"command": "node",
		"args":    []interface{}{"server.js"},
		"type":    "local",
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	if result["command"] != "node" {
		t.Errorf("command = %v, want %q", result["command"], "node")
	}
	if result["type"] != "local" {
		t.Errorf("type = %v, want %q", result["type"], "local")
	}
	if _, ok := result["env"]; ok {
		t.Error("no env key should be present when source had none")
	}
}

// TestApplyMCPEnvTransform_HeadersOpenCodeStyle verifies that ${REF_API_KEY} in
// the "headers" field is transformed to {env:REF_API_KEY} for OpenCode style.
// This covers HTTP-type MCPs (ref, context7) that pass API keys via headers.
func TestApplyMCPEnvTransform_HeadersOpenCodeStyle(t *testing.T) {
	input := map[string]interface{}{
		"type": "remote",
		"url":  "https://api.ref.tools/mcp",
		"headers": map[string]interface{}{
			"x-ref-api-key": "${REF_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "{env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	headers, ok := result["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers should be a map, got %T", result["headers"])
	}
	got := headers["x-ref-api-key"]
	if got != "{env:REF_API_KEY}" {
		t.Errorf("x-ref-api-key = %v, want {env:REF_API_KEY}", got)
	}
}

// TestApplyMCPEnvTransform_HeadersCopilotStyle verifies that ${CONTEXT7_API_KEY} in
// the "headers" field is transformed to ${env:CONTEXT7_API_KEY} for Copilot style.
func TestApplyMCPEnvTransform_HeadersCopilotStyle(t *testing.T) {
	input := map[string]interface{}{
		"type": "remote",
		"url":  "https://mcp.context7.com/mcp",
		"headers": map[string]interface{}{
			"CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${env:VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	headers, ok := result["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers should be a map, got %T", result["headers"])
	}
	got := headers["CONTEXT7_API_KEY"]
	if got != "${env:CONTEXT7_API_KEY}" {
		t.Errorf("CONTEXT7_API_KEY = %v, want ${env:CONTEXT7_API_KEY}", got)
	}
}

// TestApplyMCPEnvTransform_HeadersClaudeStyle verifies that ${REF_API_KEY} in headers
// passes through unchanged when envVarStyle is the Claude default ${VAR}.
func TestApplyMCPEnvTransform_HeadersClaudeStyle(t *testing.T) {
	input := map[string]interface{}{
		"type": "remote",
		"url":  "https://api.ref.tools/mcp",
		"headers": map[string]interface{}{
			"x-ref-api-key": "${REF_API_KEY}",
		},
	}
	cfg := model.MCPConfig{EnvKey: "env", EnvVarStyle: "${VAR}"}

	result := renderers.ApplyMCPEnvTransform(input, cfg)

	headers, ok := result["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("headers should be a map, got %T", result["headers"])
	}
	got := headers["x-ref-api-key"]
	if got != "${REF_API_KEY}" {
		t.Errorf("x-ref-api-key = %v, want ${REF_API_KEY}", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for BuildWorkflowPlaceholderReplacements (model override behaviour)
// ---------------------------------------------------------------------------

// makeWFWithRoles builds a minimal WorkflowManifest with the given roles for testing.
func makeWFWithRoles(roles []model.WorkflowRole) model.WorkflowManifest {
	var wf model.WorkflowManifest
	wf.Components.Roles = roles
	return wf
}

// TestBuildWorkflowPlaceholderReplacements_OverrideTakesPrecedence verifies that a
// value in modelOverrides replaces the role.Model from workflow.yaml.
func TestBuildWorkflowPlaceholderReplacements_OverrideTakesPrecedence(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-explorer", Kind: "subagent", Model: "haiku"},
	})
	overrides := map[string]string{
		"sdd-explorer": "sonnet",
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, overrides)

	got := result["{SDD_MODEL_EXPLORE}"]
	if got != "sonnet" {
		t.Errorf("{SDD_MODEL_EXPLORE} = %q, want %q (override should win)", got, "sonnet")
	}
}

// TestBuildWorkflowPlaceholderReplacements_InheritSentinelFallsBackToRoleModel verifies
// that the SDDModelInheritOption sentinel in overrides is treated as "no override",
// causing fallback to role.Model.
func TestBuildWorkflowPlaceholderReplacements_InheritSentinelFallsBackToRoleModel(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-planner", Kind: "subagent", Model: "opus"},
	})
	overrides := map[string]string{
		"sdd-planner": model.SDDModelInheritOption, // sentinel = no override
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, overrides)

	got := result["{SDD_MODEL_PLAN}"]
	if got != "opus" {
		t.Errorf("{SDD_MODEL_PLAN} = %q, want %q (inherit sentinel should fall back to role.Model)", got, "opus")
	}
}

// TestBuildWorkflowPlaceholderReplacements_EmptyOverrideMapIsBackwardCompatible verifies
// that an empty (non-nil) override map behaves the same as nil — role.Model is used.
func TestBuildWorkflowPlaceholderReplacements_EmptyOverrideMapIsBackwardCompatible(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-implementer", Kind: "subagent", Model: "sonnet"},
	})
	emptyOverrides := map[string]string{}

	resultWithEmpty := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, emptyOverrides)
	resultWithNil := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	if resultWithEmpty["{SDD_MODEL_IMPLEMENT}"] != resultWithNil["{SDD_MODEL_IMPLEMENT}"] {
		t.Errorf("empty override map produced %q, nil produced %q — should be equal",
			resultWithEmpty["{SDD_MODEL_IMPLEMENT}"], resultWithNil["{SDD_MODEL_IMPLEMENT}"])
	}
}

// TestBuildWorkflowPlaceholderReplacements_NilOverridesIsBackwardCompatible verifies that
// nil modelOverrides does not change any existing placeholder behaviour.
func TestBuildWorkflowPlaceholderReplacements_NilOverridesIsBackwardCompatible(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-reviewer", Kind: "subagent", Model: "haiku"},
	})

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	got := result["{SDD_MODEL_REVIEW}"]
	if got != "haiku" {
		t.Errorf("{SDD_MODEL_REVIEW} = %q, want %q", got, "haiku")
	}
}

// TestBuildWorkflowPlaceholderReplacements_OverrideWithResolveModelsTrue verifies that
// when a modelResolver is provided, a short-name override is resolved to the full model ID.
func TestBuildWorkflowPlaceholderReplacements_OverrideWithResolveModelsTrue(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-explorer", Kind: "subagent", Model: "haiku"},
	})
	overrides := map[string]string{
		"sdd-explorer": "sonnet",
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", renderers.ResolveModel, overrides)

	got := result["{SDD_MODEL_EXPLORE}"]
	wantPrefix := "anthropic/claude-sonnet" // full ID starts with this
	if len(got) == 0 {
		t.Fatal("{SDD_MODEL_EXPLORE} is empty, want resolved full model ID")
	}
	if got == "sonnet" {
		t.Errorf("{SDD_MODEL_EXPLORE} = %q (short name), want resolved full ID starting with %q", got, wantPrefix)
	}
}

// TestBuildWorkflowPlaceholderReplacements_OverrideWithResolveModelsFalse verifies that
// when modelResolver is nil, the raw override value is used as-is without resolution.
func TestBuildWorkflowPlaceholderReplacements_OverrideWithResolveModelsFalse(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-planner", Kind: "subagent", Model: "opus"},
	})
	overrides := map[string]string{
		"sdd-planner": "sonnet",
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, overrides)

	got := result["{SDD_MODEL_PLAN}"]
	if got != "sonnet" {
		t.Errorf("{SDD_MODEL_PLAN} = %q, want %q (raw short name without resolution)", got, "sonnet")
	}
}

// ---------------------------------------------------------------------------
// Tests for resolveOpenCodeModel
// ---------------------------------------------------------------------------

// TestResolveOpenCodeModel_ShortNames verifies that known short names resolve to
// github-copilot/{bareModelID} format.
func TestResolveOpenCodeModel_ShortNames(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sonnet", "github-copilot/claude-sonnet-4.6"},
		{"opus", "github-copilot/claude-opus-4.6"},
		{"haiku", "github-copilot/claude-haiku-4.5"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := renderers.ResolveOpenCodeModel(tt.input)
			if got != tt.want {
				t.Errorf("ResolveOpenCodeModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestResolveOpenCodeModel_AlreadyQualifiedPassesThrough verifies that a model ID
// that already contains a provider prefix ("/") is returned unchanged.
func TestResolveOpenCodeModel_AlreadyQualifiedPassesThrough(t *testing.T) {
	input := "github-copilot/claude-sonnet-4.6"
	got := renderers.ResolveOpenCodeModel(input)
	if got != input {
		t.Errorf("ResolveOpenCodeModel(%q) = %q, want %q (already qualified, should pass through)", input, got, input)
	}
}

// TestResolveOpenCodeModel_UnknownBareNameGetsPrefixed verifies that an unknown
// bare model name (no "/" prefix) gets the github-copilot/ prefix prepended.
func TestResolveOpenCodeModel_UnknownBareNameGetsPrefixed(t *testing.T) {
	input := "gpt-4o"
	want := "github-copilot/gpt-4o"
	got := renderers.ResolveOpenCodeModel(input)
	if got != want {
		t.Errorf("ResolveOpenCodeModel(%q) = %q, want %q", input, got, want)
	}
}

// TestBuildWorkflowPlaceholderReplacements_OpenCodeResolver verifies that using
// resolveOpenCodeModel produces github-copilot/... format placeholders.
func TestBuildWorkflowPlaceholderReplacements_OpenCodeResolver(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-explorer", Kind: "subagent", Model: "sonnet"},
	})

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", renderers.ResolveOpenCodeModel, nil)

	got := result["{SDD_MODEL_EXPLORE}"]
	const want = "github-copilot/claude-sonnet-4.6"
	if got != want {
		t.Errorf("{SDD_MODEL_EXPLORE} = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Tests for buildWorkflowPathReplacements
// ---------------------------------------------------------------------------

// TestBuildWorkflowPathReplacements_OnlySkillsPath verifies that the function
// returns only a {SKILLS_PATH} entry and no {SDD_MODEL_*} entries.
func TestBuildWorkflowPathReplacements_OnlySkillsPath(t *testing.T) {
	result := renderers.BuildWorkflowPathReplacements("/ws", "skills")

	if len(result) != 1 {
		t.Errorf("expected exactly 1 replacement, got %d: %v", len(result), result)
	}
	got, ok := result["{SKILLS_PATH}"]
	if !ok {
		t.Fatal("{SKILLS_PATH} key missing from result")
	}
	if got != "/ws/skills" {
		t.Errorf("{SKILLS_PATH} = %q, want %q", got, "/ws/skills")
	}
	// Verify no model placeholders are present.
	for key := range result {
		if key != "{SKILLS_PATH}" {
			t.Errorf("unexpected key %q in result — only {SKILLS_PATH} should be present", key)
		}
	}
}

// TestBuildWorkflowPathReplacements_EmptySkillDir uses workspaceDir directly when skillDir is empty.
func TestBuildWorkflowPathReplacements_EmptySkillDir(t *testing.T) {
	result := renderers.BuildWorkflowPathReplacements("/ws", "")
	got := result["{SKILLS_PATH}"]
	if got != "/ws" {
		t.Errorf("{SKILLS_PATH} = %q, want %q (empty skillDir should yield workspaceDir only)", got, "/ws")
	}
}

// TestBuildWorkflowPathReplacements_TrailingSlashStripped verifies trailing slashes are
// stripped from both workspaceDir and skillDir before joining.
func TestBuildWorkflowPathReplacements_TrailingSlashStripped(t *testing.T) {
	result := renderers.BuildWorkflowPathReplacements("/ws/", "skills/")
	got := result["{SKILLS_PATH}"]
	const want = "/ws/skills"
	if got != want {
		t.Errorf("{SKILLS_PATH} = %q, want %q (trailing slashes should be stripped)", got, want)
	}
}

// TestBuildWorkflowPathReplacements_NoModelKeysEvenWithRoles verifies that unlike
// buildWorkflowPlaceholderReplacements, this function never adds {SDD_MODEL_*} keys
// regardless of the workflow manifest roles provided.
func TestBuildWorkflowPathReplacements_NoModelKeysEvenWithRoles(t *testing.T) {
	// The function doesn't take a WorkflowManifest — this test confirms the design
	// by checking that no model keys appear when we use the path-only function.
	result := renderers.BuildWorkflowPathReplacements("/project", "agents")
	for key := range result {
		if key != "{SKILLS_PATH}" {
			t.Errorf("unexpected key %q — buildWorkflowPathReplacements must only produce {SKILLS_PATH}", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for removeModelPlaceholderLines
// ---------------------------------------------------------------------------

// TestRemoveModelPlaceholderLines_RemovesModelLines verifies that lines containing
// {SDD_MODEL_*} are removed from .md files while leaving other content intact.
func TestRemoveModelPlaceholderLines_RemovesModelLines(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "ORCHESTRATOR.md")

	content := "# Orchestrator\n\nTask(\n  description: 'explore',\n  subagent_type: 'general',\n  model: '{SDD_MODEL_EXPLORE}',\n  prompt: 'Do the thing.'\n)\n"
	if err := os.WriteFile(mdFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	result := string(got)

	if contains(result, "{SDD_MODEL_EXPLORE}") {
		t.Error("result still contains {SDD_MODEL_EXPLORE} placeholder — should have been removed")
	}
	if contains(result, "model:") {
		t.Error("result still contains 'model:' line — the whole line should have been removed")
	}
	// Non-model content should be preserved.
	if !contains(result, "description: 'explore'") {
		t.Error("description line was incorrectly removed")
	}
	if !contains(result, "subagent_type: 'general'") {
		t.Error("subagent_type line was incorrectly removed")
	}
}

// TestRemoveModelPlaceholderLines_AllFourPlaceholders verifies all four SDD model
// placeholders (EXPLORE, PLAN, IMPLEMENT, REVIEW) are removed.
func TestRemoveModelPlaceholderLines_AllFourPlaceholders(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "ORCHESTRATOR.md")

	content := "  model: '{SDD_MODEL_EXPLORE}',\n  model: '{SDD_MODEL_PLAN}',\n  model: '{SDD_MODEL_IMPLEMENT}',\n  model: '{SDD_MODEL_REVIEW}',\n  other: 'kept',\n"
	if err := os.WriteFile(mdFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, _ := os.ReadFile(mdFile)
	result := string(got)

	for _, placeholder := range []string{"{SDD_MODEL_EXPLORE}", "{SDD_MODEL_PLAN}", "{SDD_MODEL_IMPLEMENT}", "{SDD_MODEL_REVIEW}"} {
		if contains(result, placeholder) {
			t.Errorf("placeholder %q was not removed", placeholder)
		}
	}
	if !contains(result, "other: 'kept'") {
		t.Error("non-model line 'other: kept' was incorrectly removed")
	}
}

// TestRemoveModelPlaceholderLines_NoOpWhenNoPlaceholders verifies the function
// does not modify files that contain no {SDD_MODEL_*} patterns.
func TestRemoveModelPlaceholderLines_NoOpWhenNoPlaceholders(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "clean.md")
	original := "# No model placeholders here\n\nJust text.\n"
	if err := os.WriteFile(mdFile, []byte(original), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, _ := os.ReadFile(mdFile)
	if string(got) != original {
		t.Errorf("file was modified when no placeholders present:\ngot:  %q\nwant: %q", string(got), original)
	}
}

// TestRemoveModelPlaceholderLines_SkipsNonMdFiles verifies that non-.md files are
// not modified even if they contain {SDD_MODEL_*} strings.
func TestRemoveModelPlaceholderLines_SkipsNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	txtFile := filepath.Join(dir, "config.txt")
	original := "model: '{SDD_MODEL_EXPLORE}'\n"
	if err := os.WriteFile(txtFile, []byte(original), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, _ := os.ReadFile(txtFile)
	if string(got) != original {
		t.Errorf("non-.md file was modified (should be skipped): got %q, want %q", string(got), original)
	}
}

// TestRemoveModelPlaceholderLines_WalksSubdirectories verifies the function
// recursively processes .md files in subdirectories.
func TestRemoveModelPlaceholderLines_WalksSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sdd-orchestrator")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mdFile := filepath.Join(subDir, "ORCHESTRATOR.md")
	if err := os.WriteFile(mdFile, []byte("  model: '{SDD_MODEL_PLAN}',\n  prompt: 'x',\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, _ := os.ReadFile(mdFile)
	result := string(got)
	if contains(result, "{SDD_MODEL_PLAN}") {
		t.Error("placeholder in subdirectory file was not removed")
	}
	if !contains(result, "prompt: 'x'") {
		t.Error("non-placeholder line was incorrectly removed")
	}
}

// contains is a helper to check substring presence in test output strings.
func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
