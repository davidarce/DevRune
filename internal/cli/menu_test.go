// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/huh/v2"
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
	// Use only agents that are NOT in ModelRoutingAgents (factory, codex).
	// Note: copilot is a routing agent as of the copilot-model-routing feature.
	yaml := `schemaVersion: devrune/v1
agents:
  - name: factory
  - name: codex
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

// ---------------------------------------------------------------------------
// Tests for buildMenuOptions (T024)
// ---------------------------------------------------------------------------

// indexOfAction returns the index of the first option with the given value, or -1.
func indexOfAction(opts []huh.Option[menuAction], action menuAction) int {
	for i, o := range opts {
		if o.Value == action {
			return i
		}
	}
	return -1
}

// TestMenuActionManageAdvisorsConstant verifies the constant has the expected value.
func TestMenuActionManageAdvisorsConstant(t *testing.T) {
	if string(menuActionManageAdvisors) != "manage-advisors" {
		t.Errorf("menuActionManageAdvisors = %q, want %q", string(menuActionManageAdvisors), "manage-advisors")
	}
}

// TestBuildMenuOptions_WithModelRoutingAgents_OrderingIsCorrect verifies that when
// hasRouting is true the "Configure role models" option appears directly before
// "Manage SDD advisors".
func TestBuildMenuOptions_WithModelRoutingAgents_OrderingIsCorrect(t *testing.T) {
	opts := buildMenuOptions(true)

	idxConfigure := indexOfAction(opts, menuActionConfigureModels)
	idxAdvisors := indexOfAction(opts, menuActionManageAdvisors)

	if idxConfigure == -1 {
		t.Fatal("option 'Configure role models' not found when hasRouting=true")
	}
	if idxAdvisors == -1 {
		t.Fatal("option 'Manage SDD advisors' not found when hasRouting=true")
	}
	if idxConfigure >= idxAdvisors {
		t.Errorf("'Configure role models' (index %d) must come before 'Manage SDD advisors' (index %d)",
			idxConfigure, idxAdvisors)
	}
	// Verify they are adjacent (directly followed by advisors).
	if idxAdvisors != idxConfigure+1 {
		t.Errorf("'Manage SDD advisors' (index %d) must be directly after 'Configure role models' (index %d), gap = %d",
			idxAdvisors, idxConfigure, idxAdvisors-idxConfigure)
	}
}

// TestBuildMenuOptions_WithoutModelRoutingAgents_OrderingIsCorrect verifies that when
// hasRouting is false "Sync project" is directly followed by "Manage SDD advisors"
// and "Configure role models" is absent.
func TestBuildMenuOptions_WithoutModelRoutingAgents_OrderingIsCorrect(t *testing.T) {
	opts := buildMenuOptions(false)

	idxSync := indexOfAction(opts, menuActionSync)
	idxAdvisors := indexOfAction(opts, menuActionManageAdvisors)
	idxConfigure := indexOfAction(opts, menuActionConfigureModels)

	if idxSync == -1 {
		t.Fatal("option 'Sync project' not found when hasRouting=false")
	}
	if idxAdvisors == -1 {
		t.Fatal("option 'Manage SDD advisors' not found when hasRouting=false")
	}
	if idxConfigure != -1 {
		t.Errorf("option 'Configure role models' found at index %d but should be absent when hasRouting=false", idxConfigure)
	}
	// Verify "Sync project" is directly followed by "Manage SDD advisors".
	if idxAdvisors != idxSync+1 {
		t.Errorf("'Manage SDD advisors' (index %d) must be directly after 'Sync project' (index %d) when hasRouting=false",
			idxAdvisors, idxSync)
	}
}

// TestBuildMenuOptions_ManageAdvisorsAlwaysBeforeStatus verifies "Manage SDD advisors"
// always appears before "Status" regardless of the hasRouting flag.
func TestBuildMenuOptions_ManageAdvisorsAlwaysBeforeStatus(t *testing.T) {
	for _, hasRouting := range []bool{true, false} {
		opts := buildMenuOptions(hasRouting)

		idxAdvisors := indexOfAction(opts, menuActionManageAdvisors)
		idxStatus := indexOfAction(opts, menuActionStatus)

		if idxAdvisors == -1 {
			t.Errorf("hasRouting=%v: 'Manage SDD advisors' not found", hasRouting)
			continue
		}
		if idxStatus == -1 {
			t.Errorf("hasRouting=%v: 'Status' not found", hasRouting)
			continue
		}
		if idxAdvisors >= idxStatus {
			t.Errorf("hasRouting=%v: 'Manage SDD advisors' (index %d) must come before 'Status' (index %d)",
				hasRouting, idxAdvisors, idxStatus)
		}
	}
}

// TestBuildMenuOptions_OptionLabels verifies the human-readable key for "Manage SDD advisors"
// option matches the expected label.
func TestBuildMenuOptions_OptionLabels(t *testing.T) {
	opts := buildMenuOptions(false)
	for _, o := range opts {
		if o.Value == menuActionManageAdvisors {
			if o.Key != "Manage SDD advisors" {
				t.Errorf("label for menuActionManageAdvisors = %q, want %q", o.Key, "Manage SDD advisors")
			}
			return
		}
	}
	t.Error("menuActionManageAdvisors option not found in buildMenuOptions(false)")
}

// TestBuildMenuOptions_WithRouting_ConfigureModelsLabel verifies the label for the
// "Configure role models" option when hasRouting=true.
func TestBuildMenuOptions_WithRouting_ConfigureModelsLabel(t *testing.T) {
	opts := buildMenuOptions(true)
	for _, o := range opts {
		if o.Value == menuActionConfigureModels {
			if o.Key != "Configure role models" {
				t.Errorf("label for menuActionConfigureModels = %q, want %q", o.Key, "Configure role models")
			}
			return
		}
	}
	t.Error("menuActionConfigureModels option not found in buildMenuOptions(true)")
}

// TestBuildMenuOptions_ManageAdvisorsDispatcherAction verifies that the menu option
// for managing advisors maps to menuActionManageAdvisors (i.e., the dispatcher
// switch case will match correctly).
func TestBuildMenuOptions_ManageAdvisorsDispatcherAction(t *testing.T) {
	for _, hasRouting := range []bool{true, false} {
		opts := buildMenuOptions(hasRouting)
		found := false
		for _, o := range opts {
			if o.Key == "Manage SDD advisors" {
				if o.Value != menuActionManageAdvisors {
					t.Errorf("hasRouting=%v: option 'Manage SDD advisors' has value %q, want %q",
						hasRouting, o.Value, menuActionManageAdvisors)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("hasRouting=%v: option 'Manage SDD advisors' not found", hasRouting)
		}
	}
}
