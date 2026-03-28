// SPDX-License-Identifier: MIT

package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestUserManifest_Validate tests the Validate method on UserManifest.
func TestUserManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest UserManifest
		wantErr  bool
		errMsg   string // partial expected error message substring
	}{
		{
			name: "valid minimal manifest",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
				Packages: []PackageRef{
					{Source: "github:owner/repo@v1.0.0"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid manifest with workflows field",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
				Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
				Workflows: []string{
					"github:owner/workflows@v1.0.0//sdd",
					"local:./my-workflow",
				},
			},
			wantErr: false,
		},
		{
			name: "valid manifest with multiple agents and MCPs",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}, {Name: "opencode"}},
				Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
				MCPs:          []MCPRef{{Source: "github:owner/mcps@v1.0.0//github.yaml"}},
			},
			wantErr: false,
		},
		{
			name: "valid manifest with install config",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
				Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
				Install: InstallConfig{
					LinkMode:  "symlink",
					RulesMode: map[string]string{"claude": "both"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid manifest with no packages (just agents)",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
			},
			wantErr: false,
		},
		{
			name: "missing schemaVersion",
			manifest: UserManifest{
				Agents:   []AgentRef{{Name: "claude"}},
				Packages: []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
			},
			wantErr: true,
			errMsg:  "schemaVersion is required",
		},
		{
			name: "empty agents list",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{},
				Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
			},
			wantErr: true,
			errMsg:  "at least one agent",
		},
		{
			name: "nil agents list",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
			},
			wantErr: true,
			errMsg:  "at least one agent",
		},
		{
			name: "duplicate package sources",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
				Packages: []PackageRef{
					{Source: "github:owner/repo@v1.0.0"},
					{Source: "github:owner/repo@v1.0.0"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate package source",
		},
		{
			name: "package with empty source",
			manifest: UserManifest{
				SchemaVersion: "devrune/v1",
				Agents:        []AgentRef{{Name: "claude"}},
				Packages: []PackageRef{
					{Source: "github:owner/repo@v1.0.0"},
					{Source: ""},
				},
			},
			wantErr: true,
			errMsg:  "source must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestLockfile_Validate tests the Validate method on Lockfile.
func TestLockfile_Validate(t *testing.T) {
	validGitHubSource := SourceRef{
		Scheme: SchemeGitHub,
		Owner:  "owner",
		Repo:   "repo",
		Ref:    "v1.0.0",
	}

	tests := []struct {
		name     string
		lockfile Lockfile
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid minimal lockfile",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Packages: []LockedPackage{
					{
						Source:   validGitHubSource,
						Hash:     "sha256:def456",
						Contents: []ContentItem{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid lockfile with MCPs and workflows",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Packages:      []LockedPackage{},
				MCPs: []LockedMCP{
					{
						Source: validGitHubSource,
						Hash:   "sha256:mcp001",
						Name:   "github",
					},
				},
				Workflows: []LockedWorkflow{
					{
						Source: validGitHubSource,
						Hash:   "sha256:wf001",
						Name:   "sdd",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing schemaVersion",
			lockfile: Lockfile{
				ManifestHash: "sha256:abc123",
				Packages:     []LockedPackage{},
			},
			wantErr: true,
			errMsg:  "schemaVersion is required",
		},
		{
			name: "missing manifestHash",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				Packages:      []LockedPackage{},
			},
			wantErr: true,
			errMsg:  "manifestHash is required",
		},
		{
			name: "package with invalid source (empty scheme)",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Packages: []LockedPackage{
					{
						Source: SourceRef{Owner: "owner", Repo: "repo"},
						Hash:   "sha256:pkg001",
					},
				},
			},
			wantErr: true,
			errMsg:  "package[0]",
		},
		{
			name: "package with missing hash",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Packages: []LockedPackage{
					{
						Source: validGitHubSource,
						Hash:   "",
					},
				},
			},
			wantErr: true,
			errMsg:  "hash is required",
		},
		{
			name: "MCP with missing hash",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				MCPs: []LockedMCP{
					{
						Source: validGitHubSource,
						Hash:   "",
						Name:   "github",
					},
				},
			},
			wantErr: true,
			errMsg:  "hash is required",
		},
		{
			name: "MCP with missing name",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				MCPs: []LockedMCP{
					{
						Source: validGitHubSource,
						Hash:   "sha256:mcp001",
						Name:   "",
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "workflow with missing hash",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Workflows: []LockedWorkflow{
					{
						Source: validGitHubSource,
						Hash:   "",
						Name:   "sdd",
					},
				},
			},
			wantErr: true,
			errMsg:  "hash is required",
		},
		{
			name: "workflow with missing name",
			lockfile: Lockfile{
				SchemaVersion: "devrune/lock/v1",
				ManifestHash:  "sha256:abc123",
				Workflows: []LockedWorkflow{
					{
						Source: validGitHubSource,
						Hash:   "sha256:wf001",
						Name:   "",
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.lockfile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestLockfile_ManifestHashMatches tests the ManifestHashMatches method.
func TestLockfile_ManifestHashMatches(t *testing.T) {
	manifestContent := []byte(`schemaVersion: devrune/v1
agents:
  - name: claude
`)
	sum := sha256.Sum256(manifestContent)
	correctHash := fmt.Sprintf("sha256:%x", sum)

	tests := []struct {
		name          string
		lockfile      Lockfile
		manifestBytes []byte
		want          bool
	}{
		{
			name:          "hash matches",
			lockfile:      Lockfile{ManifestHash: correctHash},
			manifestBytes: manifestContent,
			want:          true,
		},
		{
			name:          "hash does not match (different content)",
			lockfile:      Lockfile{ManifestHash: correctHash},
			manifestBytes: []byte("different content"),
			want:          false,
		},
		{
			name:          "hash does not match (wrong format)",
			lockfile:      Lockfile{ManifestHash: "sha256:wronghash"},
			manifestBytes: manifestContent,
			want:          false,
		},
		{
			name:          "empty manifest bytes",
			lockfile:      Lockfile{ManifestHash: correctHash},
			manifestBytes: []byte{},
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.lockfile.ManifestHashMatches(tt.manifestBytes)
			if got != tt.want {
				t.Errorf("ManifestHashMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUserManifest_WorkflowModels_Serialization tests that WorkflowModels serializes to YAML correctly.
func TestUserManifest_WorkflowModels_Serialization(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		WorkflowModels: map[string]map[string]string{
			"claude": {
				"sdd-explorer": "sonnet",
				"sdd-planner":  "opus",
			},
		},
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if !strings.Contains(yamlStr, "workflowModels:") {
		t.Errorf("serialized YAML does not contain 'workflowModels:' key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "claude:") {
		t.Errorf("serialized YAML does not contain 'claude:' agent key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "sdd-explorer:") {
		t.Errorf("serialized YAML does not contain 'sdd-explorer:' role key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "sonnet") {
		t.Errorf("serialized YAML does not contain model value 'sonnet':\n%s", yamlStr)
	}
}

// TestUserManifest_WorkflowModels_OmitWhenNil tests that nil WorkflowModels omits the key entirely.
func TestUserManifest_WorkflowModels_OmitWhenNil(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		WorkflowModels: nil,
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if strings.Contains(yamlStr, "workflowModels") {
		t.Errorf("serialized YAML contains 'workflowModels' for nil value (omitempty violation):\n%s", yamlStr)
	}
}

// TestUserManifest_WorkflowModels_RoundTrip tests that marshal → unmarshal preserves WorkflowModels.
func TestUserManifest_WorkflowModels_RoundTrip(t *testing.T) {
	original := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}, {Name: "opencode"}},
		WorkflowModels: map[string]map[string]string{
			"claude": {
				"sdd-explorer":    "sonnet",
				"sdd-planner":     "opus",
				"sdd-implementer": "haiku",
				"sdd-reviewer":    "sonnet",
			},
			"opencode": {
				"sdd-explorer": "claude-sonnet-4.5",
			},
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var restored UserManifest
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(restored.WorkflowModels) != len(original.WorkflowModels) {
		t.Errorf("WorkflowModels agent count after round-trip = %d, want %d", len(restored.WorkflowModels), len(original.WorkflowModels))
	}

	for agent, roles := range original.WorkflowModels {
		restoredRoles, ok := restored.WorkflowModels[agent]
		if !ok {
			t.Errorf("WorkflowModels missing agent %q after round-trip", agent)
			continue
		}
		for role, wantModel := range roles {
			gotModel, ok := restoredRoles[role]
			if !ok {
				t.Errorf("WorkflowModels[%q] missing role %q after round-trip", agent, role)
				continue
			}
			if gotModel != wantModel {
				t.Errorf("WorkflowModels[%q][%q] = %q after round-trip, want %q", agent, role, gotModel, wantModel)
			}
		}
	}
}

// TestUserManifest_WorkflowModels_ValidatePassesWithWorkflowModels tests that Validate() succeeds
// when WorkflowModels is populated.
func TestUserManifest_WorkflowModels_ValidatePassesWithWorkflowModels(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
		WorkflowModels: map[string]map[string]string{
			"claude": {
				"sdd-explorer": "sonnet",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Errorf("Validate() with WorkflowModels populated returned error = %v, want nil", err)
	}
}

// TestUserManifest_LegacySDDModels_Migration tests that the legacy "sddModels" YAML key
// is migrated to WorkflowModels during unmarshalling.
func TestUserManifest_LegacySDDModels_Migration(t *testing.T) {
	yamlData := []byte(`
schemaVersion: devrune/v1
agents:
  - name: claude
sddModels:
  claude:
    sdd-explorer: sonnet
    sdd-planner: opus
`)

	var manifest UserManifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if manifest.WorkflowModels == nil {
		t.Fatal("WorkflowModels is nil after legacy migration")
	}
	if got := manifest.WorkflowModels["claude"]["sdd-explorer"]; got != "sonnet" {
		t.Errorf("WorkflowModels[claude][sdd-explorer] = %q, want %q", got, "sonnet")
	}
	if got := manifest.WorkflowModels["claude"]["sdd-planner"]; got != "opus" {
		t.Errorf("WorkflowModels[claude][sdd-planner] = %q, want %q", got, "opus")
	}
}

// containsString is a simple helper for substring checks (avoids importing strings in test file).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
