package renderers_test

import (
	"path/filepath"
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
