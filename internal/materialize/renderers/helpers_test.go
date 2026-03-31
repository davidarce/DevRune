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
// T019: Tests for SanitizeMCPDefinition permission extraction
// ---------------------------------------------------------------------------

// TestSanitizeMCPDefinition_WithPermissions verifies that a permissions block
// is extracted into the returned permissions map and NOT included in serverConfig.
func TestSanitizeMCPDefinition_WithPermissions(t *testing.T) {
	def := map[string]interface{}{
		"command": "npx",
		"args":    []interface{}{"-y", "@modelcontextprotocol/server-exa"},
		"env": map[string]interface{}{
			"EXA_API_KEY": "${EXA_API_KEY}",
		},
		"permissions": map[string]interface{}{
			"level": "allow",
		},
	}

	serverConfig, _, permissions := renderers.SanitizeMCPDefinition(def)

	// permissions map must contain "level": "allow"
	if permissions["level"] != "allow" {
		t.Errorf("permissions[\"level\"] = %q, want %q", permissions["level"], "allow")
	}

	// "permissions" must NOT appear in serverConfig
	if _, ok := serverConfig["permissions"]; ok {
		t.Error("serverConfig must not contain \"permissions\" key")
	}

	// runtime fields must be preserved in serverConfig
	if serverConfig["command"] != "npx" {
		t.Errorf("serverConfig[\"command\"] = %v, want %q", serverConfig["command"], "npx")
	}
}

// TestSanitizeMCPDefinition_WithoutPermissions verifies that when no permissions
// block is present, an empty (non-nil) map is returned.
func TestSanitizeMCPDefinition_WithoutPermissions(t *testing.T) {
	def := map[string]interface{}{
		"command": "node",
		"args":    []interface{}{"server.js"},
	}

	_, _, permissions := renderers.SanitizeMCPDefinition(def)

	if permissions == nil {
		t.Error("permissions map must not be nil when no permissions block is present")
	}
	if len(permissions) != 0 {
		t.Errorf("permissions map must be empty when no permissions block is present, got %v", permissions)
	}
}

// TestSanitizeMCPDefinition_PermissionsNotInServerConfig verifies the "permissions"
// key is stripped from serverConfig even when it is the only extra field.
func TestSanitizeMCPDefinition_PermissionsNotInServerConfig(t *testing.T) {
	def := map[string]interface{}{
		"command": "npx",
		"permissions": map[string]interface{}{
			"level": "deny",
		},
	}

	serverConfig, _, permissions := renderers.SanitizeMCPDefinition(def)

	if _, ok := serverConfig["permissions"]; ok {
		t.Error("serverConfig must not contain \"permissions\" key after sanitization")
	}
	if permissions["level"] != "deny" {
		t.Errorf("permissions[\"level\"] = %q, want %q", permissions["level"], "deny")
	}
}

// TestSanitizeMCPDefinition_AllPermissionLevels verifies that all three semantic
// levels (allow, ask, deny) are correctly extracted.
func TestSanitizeMCPDefinition_AllPermissionLevels(t *testing.T) {
	for _, level := range []string{"allow", "ask", "deny"} {
		t.Run("level="+level, func(t *testing.T) {
			def := map[string]interface{}{
				"command": "npx",
				"permissions": map[string]interface{}{
					"level": level,
				},
			}
			_, _, permissions := renderers.SanitizeMCPDefinition(def)
			if permissions["level"] != level {
				t.Errorf("permissions[\"level\"] = %q, want %q", permissions["level"], level)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for BuildWorkflowPlaceholderReplacements (model override behaviour)
// ---------------------------------------------------------------------------

// makeWFWithRoles builds a minimal WorkflowManifest named "sdd" with the given roles for testing.
func makeWFWithRoles(roles []model.WorkflowRole) model.WorkflowManifest {
	var wf model.WorkflowManifest
	wf.Metadata.Name = "sdd"
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
		"sdd-planner": model.ModelInheritOption, // sentinel = no override
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

// ---------------------------------------------------------------------------
// Tests for dynamic {WORKFLOW_MODEL_*} placeholders
// ---------------------------------------------------------------------------

// TestBuildWorkflowPlaceholderReplacements_SDDEmitsBothFormats verifies that SDD
// workflows emit both {WORKFLOW_MODEL_*} and legacy {SDD_MODEL_*} placeholders.
func TestBuildWorkflowPlaceholderReplacements_SDDEmitsBothFormats(t *testing.T) {
	wf := makeWFWithRoles([]model.WorkflowRole{
		{Name: "sdd-explorer", Kind: "subagent", Model: "sonnet"},
	})

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	// New format.
	if got := result["{WORKFLOW_MODEL_EXPLORER}"]; got != "sonnet" {
		t.Errorf("{WORKFLOW_MODEL_EXPLORER} = %q, want %q", got, "sonnet")
	}
	// Legacy format.
	if got := result["{SDD_MODEL_EXPLORE}"]; got != "sonnet" {
		t.Errorf("{SDD_MODEL_EXPLORE} = %q, want %q", got, "sonnet")
	}
}

// TestBuildWorkflowPlaceholderReplacements_NonSDDWorkflow verifies that non-SDD
// workflows emit only {WORKFLOW_MODEL_*} and no {SDD_MODEL_*} placeholders.
func TestBuildWorkflowPlaceholderReplacements_NonSDDWorkflow(t *testing.T) {
	var wf model.WorkflowManifest
	wf.Metadata.Name = "code-review"
	wf.Components.Roles = []model.WorkflowRole{
		{Name: "reviewer", Kind: "subagent", Model: "opus"},
		{Name: "code-review-checker", Kind: "subagent", Model: "sonnet"},
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	if got := result["{WORKFLOW_MODEL_REVIEWER}"]; got != "opus" {
		t.Errorf("{WORKFLOW_MODEL_REVIEWER} = %q, want %q", got, "opus")
	}
	if got := result["{WORKFLOW_MODEL_CHECKER}"]; got != "sonnet" {
		t.Errorf("{WORKFLOW_MODEL_CHECKER} = %q, want %q", got, "sonnet")
	}
	// No SDD legacy aliases.
	for key := range result {
		if strings.HasPrefix(key, "{SDD_MODEL_") {
			t.Errorf("unexpected SDD legacy placeholder %q in non-SDD workflow", key)
		}
	}
}

// TestBuildWorkflowPlaceholderReplacements_ExplicitPlaceholder verifies that a role
// with an explicit Placeholder field uses it instead of auto-deriving.
func TestBuildWorkflowPlaceholderReplacements_ExplicitPlaceholder(t *testing.T) {
	var wf model.WorkflowManifest
	wf.Metadata.Name = "cicd"
	wf.Components.Roles = []model.WorkflowRole{
		{Name: "code-quality-checker", Kind: "subagent", Model: "haiku", Placeholder: "CHECKER"},
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	if got := result["{WORKFLOW_MODEL_CHECKER}"]; got != "haiku" {
		t.Errorf("{WORKFLOW_MODEL_CHECKER} = %q, want %q", got, "haiku")
	}
	// The auto-derived key should NOT be present.
	if _, ok := result["{WORKFLOW_MODEL_CODE_QUALITY_CHECKER}"]; ok {
		t.Error("auto-derived key should not be present when explicit Placeholder is set")
	}
}

// TestBuildWorkflowPlaceholderReplacements_SkipsOrchestrator verifies that orchestrator
// roles are not included in the placeholder map.
func TestBuildWorkflowPlaceholderReplacements_SkipsOrchestrator(t *testing.T) {
	var wf model.WorkflowManifest
	wf.Metadata.Name = "myflow"
	wf.Components.Roles = []model.WorkflowRole{
		{Name: "myflow-orchestrator", Kind: "orchestrator", Model: "opus"},
		{Name: "myflow-worker", Kind: "subagent", Model: "sonnet"},
	}

	result := renderers.BuildWorkflowPlaceholderReplacements(wf, "/ws", "skills", nil, nil)

	if _, ok := result["{WORKFLOW_MODEL_ORCHESTRATOR}"]; ok {
		t.Error("orchestrator role should not produce a placeholder")
	}
	if got := result["{WORKFLOW_MODEL_WORKER}"]; got != "sonnet" {
		t.Errorf("{WORKFLOW_MODEL_WORKER} = %q, want %q", got, "sonnet")
	}
}

// TestRemoveModelPlaceholderLines_RemovesWorkflowModelLines verifies that lines
// containing {WORKFLOW_MODEL_*} are removed from .md files.
func TestRemoveModelPlaceholderLines_RemovesWorkflowModelLines(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "ORCHESTRATOR.md")

	content := "# Orchestrator\n\n  model: '{WORKFLOW_MODEL_EXPLORER}',\n  other: 'kept',\n"
	if err := os.WriteFile(mdFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := renderers.RemoveModelPlaceholderLines(dir); err != nil {
		t.Fatalf("RemoveModelPlaceholderLines() error: %v", err)
	}

	got, _ := os.ReadFile(mdFile)
	result := string(got)

	if contains(result, "{WORKFLOW_MODEL_EXPLORER}") {
		t.Error("result still contains {WORKFLOW_MODEL_EXPLORER} — should have been removed")
	}
	if !contains(result, "other: 'kept'") {
		t.Error("non-model line was incorrectly removed")
	}
}

// ---------------------------------------------------------------------------
// T005: WriteManagedBlock and RemoveManagedBlock tests
// ---------------------------------------------------------------------------

const testBeginMarker = "# >>> test managed"
const testEndMarker = "# <<< test managed"

// TestWriteManagedBlock_FileDoesNotExist verifies that WriteManagedBlock creates
// the file with only the managed block when the file does not exist.
func TestWriteManagedBlock_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")

	err := renderers.WriteManagedBlock(path, testBeginMarker, testEndMarker, "hello\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, testBeginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, testEndMarker) {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "hello") {
		t.Error("missing managed content")
	}
}

// TestWriteManagedBlock_FileExistsWithoutMarkers verifies that WriteManagedBlock
// appends the managed block and preserves existing content.
func TestWriteManagedBlock_FileExistsWithoutMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	_ = os.WriteFile(path, []byte("# User content\n"), 0o644)

	err := renderers.WriteManagedBlock(path, testBeginMarker, testEndMarker, "managed\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# User content") {
		t.Error("existing content was lost")
	}
	if !strings.Contains(content, testBeginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, "managed") {
		t.Error("missing managed content")
	}
}

// TestWriteManagedBlock_ReplacesExistingBlock verifies that WriteManagedBlock
// replaces only the managed block and preserves surrounding content.
func TestWriteManagedBlock_ReplacesExistingBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	initial := "# Before\n" + testBeginMarker + "\nold content\n" + testEndMarker + "\n# After\n"
	_ = os.WriteFile(path, []byte(initial), 0o644)

	err := renderers.WriteManagedBlock(path, testBeginMarker, testEndMarker, "new content\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "old content") {
		t.Error("old managed content was not replaced")
	}
	if !strings.Contains(content, "new content") {
		t.Error("new managed content is missing")
	}
	if !strings.Contains(content, "# Before") {
		t.Error("content before managed block was lost")
	}
	if !strings.Contains(content, "# After") {
		t.Error("content after managed block was lost")
	}
}

// TestWriteManagedBlock_EmptyContent verifies that WriteManagedBlock writes
// markers with no content between them when content is empty.
func TestWriteManagedBlock_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")

	err := renderers.WriteManagedBlock(path, testBeginMarker, testEndMarker, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, testBeginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, testEndMarker) {
		t.Error("missing end marker")
	}
}

// TestRemoveManagedBlock_FileDoesNotExist verifies that RemoveManagedBlock
// returns nil when the file does not exist.
func TestRemoveManagedBlock_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.md")
	if err := renderers.RemoveManagedBlock(path, testBeginMarker, testEndMarker); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestRemoveManagedBlock_RemovesBlockPreservesOtherContent verifies that
// RemoveManagedBlock removes only the managed block and leaves other content intact.
func TestRemoveManagedBlock_RemovesBlockPreservesOtherContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	initial := "# User content\n" + testBeginMarker + "\nmanaged\n" + testEndMarker + "\n# More user content\n"
	_ = os.WriteFile(path, []byte(initial), 0o644)

	if err := renderers.RemoveManagedBlock(path, testBeginMarker, testEndMarker); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, testBeginMarker) {
		t.Error("begin marker still present after removal")
	}
	if strings.Contains(content, "managed") {
		t.Error("managed content still present after removal")
	}
	if !strings.Contains(content, "# User content") {
		t.Error("user content was removed unexpectedly")
	}
	if !strings.Contains(content, "# More user content") {
		t.Error("trailing user content was removed unexpectedly")
	}
}

// TestRemoveManagedBlock_DeletesFileWhenEmpty verifies that RemoveManagedBlock
// deletes the file entirely when only the managed block remains.
func TestRemoveManagedBlock_DeletesFileWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	initial := testBeginMarker + "\nmanaged\n" + testEndMarker + "\n"
	_ = os.WriteFile(path, []byte(initial), 0o644)

	if err := renderers.RemoveManagedBlock(path, testBeginMarker, testEndMarker); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been deleted but still exists")
	}
}

// TestRemoveManagedBlock_NoMarkersReturnsNil verifies that RemoveManagedBlock
// returns nil and leaves the file unchanged when markers are not present.
func TestRemoveManagedBlock_NoMarkersReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	_ = os.WriteFile(path, []byte("# User content\n"), 0o644)

	if err := renderers.RemoveManagedBlock(path, testBeginMarker, testEndMarker); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "# User content\n" {
		t.Error("file was modified even though markers were not found")
	}
}

// ---------------------------------------------------------------------------
// T006: CreateSymlinkOrCopy and RemoveSymlinkOrCopy tests
// ---------------------------------------------------------------------------

// TestCreateSymlinkOrCopy_CreatesSymlink verifies that a symlink is created
// at linkPath pointing to target when linkPath does not exist.
func TestCreateSymlinkOrCopy_CreatesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("content"), 0o644)

	if err := renderers.CreateSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// linkPath must exist and be readable.
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("cannot read linkPath after creation: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("linkPath content = %q, want %q", string(data), "content")
	}
}

// TestCreateSymlinkOrCopy_NoopWhenCorrectSymlinkExists verifies that
// CreateSymlinkOrCopy is a no-op when the correct symlink already exists.
func TestCreateSymlinkOrCopy_NoopWhenCorrectSymlinkExists(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("content"), 0o644)

	// Create symlink first.
	if err := renderers.CreateSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	// Second call must succeed (no-op).
	if err := renderers.CreateSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("second call (no-op) failed: %v", err)
	}
}

// TestCreateSymlinkOrCopy_ErrorOnRegularFile verifies that CreateSymlinkOrCopy
// returns an error when linkPath already exists as a regular file.
func TestCreateSymlinkOrCopy_ErrorOnRegularFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("agents"), 0o644)
	_ = os.WriteFile(linkPath, []byte("user content"), 0o644) // pre-existing regular file

	err := renderers.CreateSymlinkOrCopy(target, linkPath)
	if err == nil {
		t.Fatal("expected error when linkPath is a regular file, got nil")
	}
}

// TestRemoveSymlinkOrCopy_RemovesSymlink verifies that RemoveSymlinkOrCopy
// removes a symlink pointing to the expected target.
func TestRemoveSymlinkOrCopy_RemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("content"), 0o644)
	if err := renderers.CreateSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := renderers.RemoveSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("linkPath still exists after removal")
	}
}

// TestRemoveSymlinkOrCopy_NoopWhenNotExist verifies that RemoveSymlinkOrCopy
// returns nil when linkPath does not exist.
func TestRemoveSymlinkOrCopy_NoopWhenNotExist(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("content"), 0o644)

	if err := renderers.RemoveSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestRemoveSymlinkOrCopy_LeavesUserOwnedFile verifies that RemoveSymlinkOrCopy
// does not remove a regular file whose content differs from the target.
func TestRemoveSymlinkOrCopy_LeavesUserOwnedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	linkPath := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("agents content"), 0o644)
	_ = os.WriteFile(linkPath, []byte("user owned — different content"), 0o644)

	if err := renderers.RemoveSymlinkOrCopy(target, linkPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(linkPath); err != nil {
		t.Error("user-owned file was incorrectly removed")
	}
}
