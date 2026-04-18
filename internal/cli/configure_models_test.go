// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// Tests for updateManifestWorkflowModels
// ---------------------------------------------------------------------------

func TestUpdateManifestWorkflowModels_EmptyManifest_NoOp(t *testing.T) {
	manifest := model.UserManifest{}
	// Should not panic and manifest stays empty.
	updateManifestWorkflowModels(&manifest, map[string]map[string]string{
		"claude": {"sdd-explorer": "claude-sonnet-4-5"},
	})
	if len(manifest.Workflows) != 0 {
		t.Errorf("expected empty workflows, got %d entries", len(manifest.Workflows))
	}
}

func TestUpdateManifestWorkflowModels_SingleWorkflow_UpdatesRoles(t *testing.T) {
	manifest := model.UserManifest{
		Workflows: map[string]model.WorkflowEntry{
			"sdd": {},
		},
	}
	newModels := map[string]map[string]string{
		"claude": {"sdd-explorer": "claude-sonnet-4-5", "sdd-planner": "claude-opus-4-5"},
	}
	updateManifestWorkflowModels(&manifest, newModels)

	entry := manifest.Workflows["sdd"]
	if entry.Roles == nil {
		t.Fatal("expected roles to be set, got nil")
	}
	if entry.Roles["claude"]["sdd-explorer"] != "claude-sonnet-4-5" {
		t.Errorf("sdd-explorer = %q, want %q", entry.Roles["claude"]["sdd-explorer"], "claude-sonnet-4-5")
	}
	if entry.Roles["claude"]["sdd-planner"] != "claude-opus-4-5" {
		t.Errorf("sdd-planner = %q, want %q", entry.Roles["claude"]["sdd-planner"], "claude-opus-4-5")
	}
}

func TestUpdateManifestWorkflowModels_NilNewModels_ClearsRoles(t *testing.T) {
	manifest := model.UserManifest{
		Workflows: map[string]model.WorkflowEntry{
			"sdd": {
				Roles: map[string]map[string]string{
					"claude": {"sdd-explorer": "claude-sonnet-4-5"},
				},
			},
		},
	}
	updateManifestWorkflowModels(&manifest, nil)

	entry := manifest.Workflows["sdd"]
	if entry.Roles != nil {
		t.Errorf("expected nil roles after clearing, got %v", entry.Roles)
	}
}

func TestUpdateManifestWorkflowModels_MultipleWorkflows_AllUpdated(t *testing.T) {
	manifest := model.UserManifest{
		Workflows: map[string]model.WorkflowEntry{
			"sdd":  {},
			"cicd": {},
		},
	}
	newModels := map[string]map[string]string{
		"claude": {"sdd-explorer": "claude-sonnet-4-5"},
	}
	updateManifestWorkflowModels(&manifest, newModels)

	for name, entry := range manifest.Workflows {
		if entry.Roles == nil {
			t.Errorf("workflow %q: expected roles to be set, got nil", name)
			continue
		}
		if entry.Roles["claude"]["sdd-explorer"] != "claude-sonnet-4-5" {
			t.Errorf("workflow %q: sdd-explorer = %q, want %q",
				name, entry.Roles["claude"]["sdd-explorer"], "claude-sonnet-4-5")
		}
	}
}
