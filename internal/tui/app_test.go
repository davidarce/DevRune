// SPDX-License-Identifier: MIT

package tui

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
)

// TestLoadExistingManifest_NoFile verifies loadExistingManifest returns nil
// when no devrune.yaml exists in the current directory.
func TestLoadExistingManifest_NoFile(t *testing.T) {
	// Change to a temp dir that has no devrune.yaml.
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	result := loadExistingManifest()
	if result != nil {
		t.Errorf("expected nil when no devrune.yaml exists, got %+v", result)
	}
}

// TestLoadExistingManifest_WithWorkflowModels verifies loadExistingManifest returns
// a populated manifest with WorkflowModels when devrune.yaml contains the workflowModels field.
func TestLoadExistingManifest_WithWorkflowModels(t *testing.T) {
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// Write a devrune.yaml with workflowModels.
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		WorkflowModels: map[string]map[string]string{
			"claude": {
				"sdd-explorer": "sonnet",
				"sdd-planner":  "opus",
			},
		},
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile("devrune.yaml", data, 0o644); err != nil {
		t.Fatalf("write devrune.yaml: %v", err)
	}

	result := loadExistingManifest()
	if result == nil {
		t.Fatal("expected non-nil manifest when devrune.yaml exists")
	}
	if result.WorkflowModels == nil {
		t.Fatal("expected WorkflowModels to be populated")
	}
	if got := result.WorkflowModels["claude"]["sdd-explorer"]; got != "sonnet" {
		t.Errorf("WorkflowModels[claude][sdd-explorer]: got %q, want %q", got, "sonnet")
	}
	if got := result.WorkflowModels["claude"]["sdd-planner"]; got != "opus" {
		t.Errorf("WorkflowModels[claude][sdd-planner]: got %q, want %q", got, "opus")
	}
}

// TestLoadExistingManifest_InvalidYAML verifies loadExistingManifest returns nil
// when devrune.yaml contains invalid YAML.
func TestLoadExistingManifest_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	if err := os.WriteFile("devrune.yaml", []byte("{ invalid yaml: ["), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result := loadExistingManifest()
	if result != nil {
		t.Errorf("expected nil for invalid YAML, got %+v", result)
	}
}

// TestManifestRoundTrip_WithWorkflowModels verifies that a manifest with WorkflowModels
// survives a YAML marshal/unmarshal round-trip without data loss.
func TestManifestRoundTrip_WithWorkflowModels(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "devrune.yaml")

	original := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}, {Name: "opencode"}},
		WorkflowModels: map[string]map[string]string{
			"claude": {
				"sdd-explorer":    "haiku",
				"sdd-planner":     "sonnet",
				"sdd-implementer": "opus",
				"sdd-reviewer":    "sonnet",
			},
			"opencode": {
				"sdd-explorer": "claude-sonnet-4.5",
			},
		},
	}

	// Serialize to YAML.
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(yamlPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Deserialize back.
	readData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded model.UserManifest
	if err := yaml.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify WorkflowModels are preserved.
	if loaded.WorkflowModels == nil {
		t.Fatal("WorkflowModels is nil after round-trip")
	}
	for agentName, roles := range original.WorkflowModels {
		loadedRoles, ok := loaded.WorkflowModels[agentName]
		if !ok {
			t.Errorf("agent %q missing from loaded WorkflowModels", agentName)
			continue
		}
		for roleName, modelVal := range roles {
			if got := loadedRoles[roleName]; got != modelVal {
				t.Errorf("WorkflowModels[%s][%s]: got %q, want %q", agentName, roleName, got, modelVal)
			}
		}
	}
}
