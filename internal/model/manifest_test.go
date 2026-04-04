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
				Workflows: map[string]WorkflowEntry{
					"sdd":         {Source: "github:owner/workflows@v1.0.0//sdd"},
					"my-workflow": {Source: "local:./my-workflow"},
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

// TestUserManifest_Catalogs_Serialization tests that Catalogs serializes to YAML with the correct key.
func TestUserManifest_Catalogs_Serialization(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Catalogs:      []string{"github:org/catalog-a", "github:org/catalog-b"},
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if !strings.Contains(yamlStr, "catalogs:") {
		t.Errorf("serialized YAML does not contain 'catalogs:' key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "catalog-a") {
		t.Errorf("serialized YAML does not contain 'catalog-a':\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "catalog-b") {
		t.Errorf("serialized YAML does not contain 'catalog-b':\n%s", yamlStr)
	}
}

// TestUserManifest_Catalogs_OmitWhenNil tests that nil Catalogs omits the key.
func TestUserManifest_Catalogs_OmitWhenNil(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Catalogs:      nil,
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if strings.Contains(yamlStr, "catalogs") {
		t.Errorf("serialized YAML contains 'catalogs' for nil value (omitempty violation):\n%s", yamlStr)
	}
}

// TestUserManifest_Catalogs_Unmarshal tests that YAML with catalogs: key populates the Catalogs field.
func TestUserManifest_Catalogs_Unmarshal(t *testing.T) {
	yamlData := []byte(`
schemaVersion: devrune/v1
agents:
  - name: claude
catalogs:
  - github:org/catalog-a
  - github:org/catalog-b
`)

	var manifest UserManifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(manifest.Catalogs) != 2 {
		t.Fatalf("Catalogs length = %d, want 2", len(manifest.Catalogs))
	}
	if manifest.Catalogs[0] != "github:org/catalog-a" {
		t.Errorf("Catalogs[0] = %q, want %q", manifest.Catalogs[0], "github:org/catalog-a")
	}
	if manifest.Catalogs[1] != "github:org/catalog-b" {
		t.Errorf("Catalogs[1] = %q, want %q", manifest.Catalogs[1], "github:org/catalog-b")
	}
}

// TestUserManifest_Catalogs_AbsentKeyIsNil tests that a manifest without catalogs: key has nil Catalogs.
func TestUserManifest_Catalogs_AbsentKeyIsNil(t *testing.T) {
	yamlData := []byte(`
schemaVersion: devrune/v1
agents:
  - name: claude
`)

	var manifest UserManifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if manifest.Catalogs != nil {
		t.Errorf("Catalogs = %v, want nil when key is absent", manifest.Catalogs)
	}
}

// TestUserManifest_Catalogs_ValidatePassesWithCatalogs tests that Validate() succeeds
// when Catalogs is populated.
func TestUserManifest_Catalogs_ValidatePassesWithCatalogs(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Catalogs:      []string{"github:org/catalog"},
	}

	if err := manifest.Validate(); err != nil {
		t.Errorf("Validate() with Catalogs populated returned error = %v, want nil", err)
	}
}

// TestUserManifest_WorkflowEntry_Serialization tests that WorkflowEntry with roles serializes correctly.
func TestUserManifest_WorkflowEntry_Serialization(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Workflows: map[string]WorkflowEntry{
			"sdd": {
				Source: "github:owner/workflows@v1.0.0//workflows/sdd",
				Roles: map[string]map[string]string{
					"claude": {
						"sdd-explorer": "sonnet",
						"sdd-planner":  "opus",
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if !strings.Contains(yamlStr, "workflows:") {
		t.Errorf("serialized YAML does not contain 'workflows:' key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "sdd:") {
		t.Errorf("serialized YAML does not contain 'sdd:' workflow key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "source:") {
		t.Errorf("serialized YAML does not contain 'source:' key:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "roles:") {
		t.Errorf("serialized YAML does not contain 'roles:' key:\n%s", yamlStr)
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

// TestUserManifest_WorkflowEntry_OmitWhenNil tests that nil Workflows omits the key entirely.
func TestUserManifest_WorkflowEntry_OmitWhenNil(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Workflows:     nil,
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if strings.Contains(yamlStr, "workflows") {
		t.Errorf("serialized YAML contains 'workflows' for nil value (omitempty violation):\n%s", yamlStr)
	}
}

// TestUserManifest_WorkflowEntry_RolesOmitWhenNil tests that a workflow entry without roles
// omits the roles key.
func TestUserManifest_WorkflowEntry_RolesOmitWhenNil(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Workflows: map[string]WorkflowEntry{
			"sdd": {Source: "github:owner/workflows@v1.0.0//workflows/sdd"},
		},
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)
	if strings.Contains(yamlStr, "roles") {
		t.Errorf("serialized YAML contains 'roles' for nil roles (omitempty violation):\n%s", yamlStr)
	}
}

// TestUserManifest_WorkflowEntry_RoundTrip tests that marshal → unmarshal preserves WorkflowEntry.Roles.
func TestUserManifest_WorkflowEntry_RoundTrip(t *testing.T) {
	original := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}, {Name: "opencode"}},
		Workflows: map[string]WorkflowEntry{
			"sdd": {
				Source: "github:owner/workflows@v1.0.0//workflows/sdd",
				Roles: map[string]map[string]string{
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

	restoredEntry, ok := restored.Workflows["sdd"]
	if !ok {
		t.Fatal("Workflows[\"sdd\"] missing after round-trip")
	}

	originalEntry := original.Workflows["sdd"]
	for agent, roles := range originalEntry.Roles {
		restoredAgentRoles, ok := restoredEntry.Roles[agent]
		if !ok {
			t.Errorf("Workflows[sdd].Roles missing agent %q after round-trip", agent)
			continue
		}
		for role, wantModel := range roles {
			gotModel, ok := restoredAgentRoles[role]
			if !ok {
				t.Errorf("Workflows[sdd].Roles[%q] missing role %q after round-trip", agent, role)
				continue
			}
			if gotModel != wantModel {
				t.Errorf("Workflows[sdd].Roles[%q][%q] = %q after round-trip, want %q", agent, role, gotModel, wantModel)
			}
		}
	}
}

// TestUserManifest_WorkflowEntry_ValidatePassesWithRoles tests that Validate() succeeds
// when Workflows has entries with roles.
func TestUserManifest_WorkflowEntry_ValidatePassesWithRoles(t *testing.T) {
	manifest := UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []AgentRef{{Name: "claude"}},
		Packages:      []PackageRef{{Source: "github:owner/repo@v1.0.0"}},
		Workflows: map[string]WorkflowEntry{
			"sdd": {
				Source: "github:owner/workflows@v1.0.0//workflows/sdd",
				Roles: map[string]map[string]string{
					"claude": {
						"sdd-explorer": "sonnet",
					},
				},
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Errorf("Validate() with Workflows populated returned error = %v, want nil", err)
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
