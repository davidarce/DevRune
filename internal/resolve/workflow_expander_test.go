// SPDX-License-Identifier: MIT

package resolve

import (
	"context"
	"errors"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// mockFetcherSimple is a minimal mock Fetcher for workflow expander tests.
type mockFetcherSimple struct {
	data map[string][]byte
	err  error
}

func (m *mockFetcherSimple) Fetch(_ context.Context, ref model.SourceRef) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := ref.CacheKey()
	if data, ok := m.data[key]; ok {
		return data, nil
	}
	return nil, errors.New("mock fetcher: no data for " + key)
}

func (m *mockFetcherSimple) Supports(scheme model.Scheme) bool {
	return true
}

// TestExpandWorkflows_ValidWorkflowSources verifies that valid workflow source refs pass validation.
func TestExpandWorkflows_ValidWorkflowSources(t *testing.T) {
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Packages:      []model.PackageRef{},
		Agents:        []model.AgentRef{{Name: "claude"}},
		Workflows: map[string]model.WorkflowEntry{
			"repo": {Source: "github:owner/repo@v1.0.0"},
		},
	}

	f := &mockFetcherSimple{data: map[string][]byte{}}
	result, err := ExpandWorkflows(context.Background(), manifest, f, "")
	if err != nil {
		t.Fatalf("ExpandWorkflows() error = %v", err)
	}

	// The manifest should be returned unchanged.
	if result.SchemaVersion != manifest.SchemaVersion {
		t.Errorf("SchemaVersion changed: got %q, want %q", result.SchemaVersion, manifest.SchemaVersion)
	}
	if len(result.Workflows) != 1 {
		t.Errorf("Workflows count: got %d, want 1", len(result.Workflows))
	}
	if result.Workflows["repo"].Source != manifest.Workflows["repo"].Source {
		t.Errorf("Workflow[repo].Source: got %q, want %q", result.Workflows["repo"].Source, manifest.Workflows["repo"].Source)
	}
}

// TestExpandWorkflows_NoWorkflows verifies that a manifest with no workflows passes through unchanged.
func TestExpandWorkflows_NoWorkflows(t *testing.T) {
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Packages: []model.PackageRef{
			{Source: "github:owner/repo@v1.0.0"},
		},
		Agents:    []model.AgentRef{{Name: "claude"}},
		Workflows: nil,
	}

	f := &mockFetcherSimple{}
	result, err := ExpandWorkflows(context.Background(), manifest, f, "")
	if err != nil {
		t.Fatalf("ExpandWorkflows() error = %v", err)
	}

	if len(result.Packages) != 1 {
		t.Errorf("Packages count: got %d, want 1", len(result.Packages))
	}
	if len(result.Workflows) != 0 {
		t.Errorf("Workflows count: got %d, want 0", len(result.Workflows))
	}
}


// TestExpandWorkflows_MultipleWorkflows verifies that multiple valid workflow sources all pass.
func TestExpandWorkflows_MultipleWorkflows(t *testing.T) {
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Workflows: map[string]model.WorkflowEntry{
			"repo-a": {Source: "github:owner/repo-a@v1.0.0"},
			"repo-b": {Source: "github:owner/repo-b@v2.0.0"},
		},
	}

	f := &mockFetcherSimple{data: map[string][]byte{}}
	result, err := ExpandWorkflows(context.Background(), manifest, f, "")
	if err != nil {
		t.Fatalf("ExpandWorkflows() error = %v", err)
	}

	if len(result.Workflows) != 2 {
		t.Errorf("Workflows count: got %d, want 2", len(result.Workflows))
	}
}

// TestExpandWorkflows_InvalidSourceRef verifies that an invalid source ref causes an error.
func TestExpandWorkflows_InvalidSourceRef(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
	}{
		{
			name:     "missing scheme",
			workflow: "owner/repo@v1.0.0",
		},
		{
			name:     "unknown scheme",
			workflow: "s3:bucket/path",
		},
		{
			name:     "empty workflow source",
			workflow: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := model.UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []model.AgentRef{{Name: "claude"}},
				Workflows: map[string]model.WorkflowEntry{
					"test": {Source: tt.workflow},
				},
			}

			f := &mockFetcherSimple{}
			_, err := ExpandWorkflows(context.Background(), manifest, f, "")
			if err == nil {
				t.Errorf("ExpandWorkflows(%q) expected error, got nil", tt.workflow)
			}
		})
	}
}

// TestExpandWorkflows_LocalSourceRef verifies that local source refs are accepted.
func TestExpandWorkflows_LocalSourceRef(t *testing.T) {
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Workflows: map[string]model.WorkflowEntry{
			"my-workflow": {Source: "local:./my-workflow"},
		},
	}

	f := &mockFetcherSimple{data: map[string][]byte{}}
	result, err := ExpandWorkflows(context.Background(), manifest, f, "/some/base/dir")
	if err != nil {
		t.Fatalf("ExpandWorkflows() error = %v", err)
	}

	if len(result.Workflows) != 1 || result.Workflows["my-workflow"].Source != "local:./my-workflow" {
		t.Errorf("Workflows = %v, want {my-workflow: {source: local:./my-workflow}}", result.Workflows)
	}
}

// TestExpandWorkflows_PackagesUnchanged verifies that package refs are not modified.
func TestExpandWorkflows_PackagesUnchanged(t *testing.T) {
	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages: []model.PackageRef{
			{Source: "github:owner/pkg@v1.0.0"},
		},
		Workflows: map[string]model.WorkflowEntry{
			"wf": {Source: "github:owner/wf@v1.0.0"},
		},
	}

	f := &mockFetcherSimple{data: map[string][]byte{}}
	result, err := ExpandWorkflows(context.Background(), manifest, f, "")
	if err != nil {
		t.Fatalf("ExpandWorkflows() error = %v", err)
	}

	if len(result.Packages) != 1 {
		t.Errorf("Packages count: got %d, want 1", len(result.Packages))
	}
	if result.Packages[0].Source != "github:owner/pkg@v1.0.0" {
		t.Errorf("Package source: got %q, want %q", result.Packages[0].Source, "github:owner/pkg@v1.0.0")
	}
}

// TestStripFirstPathComponent verifies the helper function.
func TestStripFirstPathComponent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard github path",
			input: "owner-repo-abc123/skills/git-commit/SKILL.md",
			want:  "skills/git-commit/SKILL.md",
		},
		{
			name:  "single component (root dir)",
			input: "owner-repo-abc123",
			want:  "",
		},
		{
			name:  "leading slash",
			input: "/owner-repo-abc123/skills/SKILL.md",
			want:  "skills/SKILL.md",
		},
		{
			name:  "workflow.yaml at root of subdir",
			input: "repo-sha/workflow.yaml",
			want:  "workflow.yaml",
		},
		{
			name:  "nested path",
			input: "prefix/a/b/c.md",
			want:  "a/b/c.md",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFirstPathComponent(tt.input)
			if got != tt.want {
				t.Errorf("stripFirstPathComponent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
