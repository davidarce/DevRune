// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMenuActionConstants verifies all menuAction constants have the expected string values.
func TestMenuActionConstants(t *testing.T) {
	tests := []struct {
		name   string
		action menuAction
		want   string
	}{
		{"init", menuActionInit, "init"},
		{"sync", menuActionSync, "sync"},
		{"status", menuActionStatus, "status"},
		{"configure-models", menuActionConfigureModels, "configure-models"},
		{"upgrade", menuActionUpgrade, "upgrade"},
		{"uninstall", menuActionUninstall, "uninstall"},
		{"quit", menuActionQuit, "quit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.action) != tt.want {
				t.Errorf("menuAction %s = %q, want %q", tt.name, string(tt.action), tt.want)
			}
		})
	}
}

// TestMenuActionCount verifies there are exactly 7 menu actions defined.
func TestMenuActionCount(t *testing.T) {
	actions := []menuAction{
		menuActionInit,
		menuActionSync,
		menuActionStatus,
		menuActionConfigureModels,
		menuActionUpgrade,
		menuActionUninstall,
		menuActionQuit,
	}

	const wantCount = 7
	if len(actions) != wantCount {
		t.Errorf("menu action count = %d, want %d", len(actions), wantCount)
	}
}

// TestMenuActionUniqueness verifies that all menuAction constants are unique.
func TestMenuActionUniqueness(t *testing.T) {
	seen := make(map[menuAction]bool)
	actions := []menuAction{
		menuActionInit,
		menuActionSync,
		menuActionStatus,
		menuActionConfigureModels,
		menuActionUpgrade,
		menuActionUninstall,
		menuActionQuit,
	}

	for _, a := range actions {
		if seen[a] {
			t.Errorf("duplicate menuAction value: %q", a)
		}
		seen[a] = true
	}
}

// TestMenuActionType verifies menuAction is a string-based type.
func TestMenuActionType(t *testing.T) {
	var a menuAction = "custom"
	if string(a) != "custom" {
		t.Errorf("menuAction string conversion failed: got %q, want %q", string(a), "custom")
	}
}

// ---------------------------------------------------------------------------
// Tests for hasModelRoutingAgents
// ---------------------------------------------------------------------------

func TestHasModelRoutingAgents_ReturnsTrueForClaudeAgent(t *testing.T) {
	dir := t.TempDir()
	yaml := `schemaVersion: devrune/v1
agents:
  - name: claude
`
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasModelRoutingAgents(dir) {
		t.Error("expected true for manifest with claude agent")
	}
}

func TestHasModelRoutingAgents_ReturnsTrueForOpenCodeAgent(t *testing.T) {
	dir := t.TempDir()
	yaml := `schemaVersion: devrune/v1
agents:
  - name: opencode
`
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasModelRoutingAgents(dir) {
		t.Error("expected true for manifest with opencode agent")
	}
}

func TestHasModelRoutingAgents_ReturnsFalseForNonRoutingAgents(t *testing.T) {
	dir := t.TempDir()
	yaml := `schemaVersion: devrune/v1
agents:
  - name: copilot
  - name: factory
`
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasModelRoutingAgents(dir) {
		t.Error("expected false for manifest with no model-routing agents")
	}
}

func TestHasModelRoutingAgents_ReturnsFalseWhenNoManifest(t *testing.T) {
	dir := t.TempDir()
	if hasModelRoutingAgents(dir) {
		t.Error("expected false when devrune.yaml does not exist")
	}
}

func TestHasModelRoutingAgents_ReturnsFalseForInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(":::invalid yaml:::"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasModelRoutingAgents(dir) {
		t.Error("expected false for invalid YAML manifest")
	}
}
