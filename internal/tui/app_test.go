// SPDX-License-Identifier: MIT

package tui

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/steps"
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
	if result == nil || result.WorkflowModels == nil {
		t.Fatal("expected non-nil manifest with WorkflowModels when devrune.yaml exists")
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

// toolNames extracts tool names from a slice of ToolDef for assertion helpers.
func toolNames(tools []model.ToolDef) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

// containsName returns true if the name is present in the slice.
func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// selectionWith builds a SelectionResult with a single repo entry.
func selectionWith(selectedTools, selectedMCPs, selectedWorkflows []string) steps.SelectionResult {
	return steps.SelectionResult{
		Repos: []steps.RepoSelectionResult{
			{
				Source:            "test/repo",
				SelectedTools:     selectedTools,
				SelectedMCPs:      selectedMCPs,
				SelectedWorkflows: selectedWorkflows,
			},
		},
	}
}

// TestFilterToolsBySelection covers the filtering logic with table-driven tests.
func TestFilterToolsBySelection(t *testing.T) {
	// Fixture tools used across test cases.
	toolNoDeps := model.ToolDef{Name: "nodeps", Command: "brew install nodeps"}
	toolMCPDep := model.ToolDef{
		Name:      "mcp-tool",
		Command:   "brew install mcp-tool",
		DependsOn: &model.ToolDeps{MCP: "engram"},
	}
	toolWorkflowDep := model.ToolDef{
		Name:      "wf-tool",
		Command:   "brew install wf-tool",
		DependsOn: &model.ToolDeps{Workflow: "sdd"},
	}
	toolBothDeps := model.ToolDef{
		Name:      "both-tool",
		Command:   "brew install both-tool",
		DependsOn: &model.ToolDeps{MCP: "engram", Workflow: "sdd"},
	}

	tests := []struct {
		name         string
		tools        []model.ToolDef
		selection    steps.SelectionResult
		wantNames    []string
		wantExcluded []string
	}{
		{
			name:      "no_deps_tool_selected",
			tools:     []model.ToolDef{toolNoDeps},
			selection: selectionWith([]string{"nodeps"}, nil, nil),
			wantNames: []string{"nodeps"},
		},
		{
			name:  "mcp_dep_matches_selected_mcp",
			tools: []model.ToolDef{toolMCPDep},
			selection: selectionWith(
				[]string{"mcp-tool"},
				[]string{"engram"},
				nil,
			),
			wantNames: []string{"mcp-tool"},
		},
		{
			name:  "workflow_dep_matches_selected_workflow",
			tools: []model.ToolDef{toolWorkflowDep},
			selection: selectionWith(
				[]string{"wf-tool"},
				nil,
				[]string{"sdd"},
			),
			wantNames: []string{"wf-tool"},
		},
		{
			name: "both_conditions_or_logic_mcp_match",
			// Tool has both MCP and workflow dep; only MCP is selected → still included (OR).
			tools: []model.ToolDef{toolBothDeps},
			selection: selectionWith(
				[]string{"both-tool"},
				[]string{"engram"},
				nil,
			),
			wantNames: []string{"both-tool"},
		},
		{
			name: "both_conditions_or_logic_workflow_match",
			// Tool has both MCP and workflow dep; only workflow is selected → still included (OR).
			tools: []model.ToolDef{toolBothDeps},
			selection: selectionWith(
				[]string{"both-tool"},
				nil,
				[]string{"sdd"},
			),
			wantNames: []string{"both-tool"},
		},
		{
			name: "dep_not_met_excluded",
			// Tool requires MCP "engram" but user selected MCP "other" → excluded.
			tools: []model.ToolDef{toolMCPDep},
			selection: selectionWith(
				[]string{"mcp-tool"},
				[]string{"other"},
				nil,
			),
			wantNames:    []string{},
			wantExcluded: []string{"mcp-tool"},
		},
		{
			name: "user_deselected_tool_excluded",
			// Tool has no deps but user did not select it in the Tools category.
			tools:        []model.ToolDef{toolNoDeps},
			selection:    selectionWith(nil, nil, nil),
			wantNames:    []string{},
			wantExcluded: []string{"nodeps"},
		},
		{
			name:      "empty_tools_list",
			tools:     nil,
			selection: selectionWith([]string{"anything"}, nil, nil),
			wantNames: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filterToolsBySelection(tc.tools, tc.selection)

			for _, want := range tc.wantNames {
				names := toolNames(got)
				if !containsName(names, want) {
					t.Errorf("expected %q to be included, got: %v", want, names)
				}
			}

			for _, excluded := range tc.wantExcluded {
				names := toolNames(got)
				if containsName(names, excluded) {
					t.Errorf("expected %q to be excluded, got: %v", excluded, names)
				}
			}

			if len(tc.wantNames) == 0 && len(got) != 0 {
				t.Errorf("expected empty result, got: %v", toolNames(got))
			}
		})
	}
}
