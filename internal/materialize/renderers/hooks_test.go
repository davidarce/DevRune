// SPDX-License-Identifier: MIT

package renderers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
)

// TestDeepMergeJSON_MergesNestedMaps verifies that deepMergeJSON merges nested
// maps recursively — e.g. hooks.PreCompact merged alongside existing permissions.
func TestDeepMergeJSON_MergesNestedMaps(t *testing.T) {
	dst := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []interface{}{"Bash(git:*)"},
		},
	}
	src := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreCompact": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "sdd-pre-compact.sh",
						},
					},
				},
			},
		},
	}

	result := renderers.DeepMergeJSON(dst, src)

	// permissions must still be present (not overwritten)
	perms, ok := result["permissions"].(map[string]interface{})
	if !ok {
		t.Fatalf("permissions key missing or not a map after merge")
	}
	allow, ok := perms["allow"].([]interface{})
	if !ok || len(allow) == 0 {
		t.Errorf("permissions.allow missing or empty after merge")
	}

	// hooks must have been merged in
	if _, ok := result["hooks"]; !ok {
		t.Error("hooks key missing after merge")
	}
}

// TestDeepMergeJSON_OverwritesScalarValues verifies that scalar values in src
// overwrite corresponding values in dst.
func TestDeepMergeJSON_OverwritesScalarValues(t *testing.T) {
	dst := map[string]interface{}{
		"version": "1",
		"debug":   false,
	}
	src := map[string]interface{}{
		"version": "2",
		"debug":   true,
	}

	result := renderers.DeepMergeJSON(dst, src)

	if result["version"] != "2" {
		t.Errorf("version = %v, want %q", result["version"], "2")
	}
	if result["debug"] != true {
		t.Errorf("debug = %v, want true", result["debug"])
	}
}

// TestReadAndValidateHookJSON_ValidFile verifies that a valid JSON file is read
// and returned as a parsed map.
func TestReadAndValidateHookJSON_ValidFile(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{
  "hooks": {
    "PreCompact": [
      {
        "matcher": ".*",
        "hooks": [{"type": "command", "command": "sdd-pre-compact.sh"}]
      }
    ]
  }
}`
	jsonPath := filepath.Join(dir, "claude-precompact.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	result, err := renderers.ReadAndValidateHookJSON(jsonPath)
	if err != nil {
		t.Fatalf("ReadAndValidateHookJSON: unexpected error: %v", err)
	}

	if _, ok := result["hooks"]; !ok {
		t.Error("parsed result missing 'hooks' key")
	}
}

// TestReadAndValidateHookJSON_InvalidJSON verifies that malformed JSON returns an error.
func TestReadAndValidateHookJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(jsonPath, []byte(`{ "hooks": [`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := renderers.ReadAndValidateHookJSON(jsonPath)
	if err == nil {
		t.Error("ReadAndValidateHookJSON: expected error for malformed JSON, got nil")
	}
}

// TestReadAndValidateHookJSON_NonexistentFile verifies that a missing file returns an error.
func TestReadAndValidateHookJSON_NonexistentFile(t *testing.T) {
	_, err := renderers.ReadAndValidateHookJSON("/nonexistent/path/hook.json")
	if err == nil {
		t.Error("ReadAndValidateHookJSON: expected error for non-existent file, got nil")
	}
}
