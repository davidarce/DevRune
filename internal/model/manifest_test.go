// SPDX-License-Identifier: MIT

package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ─────────────────────────────────────────────────────────────────────────────
// Local test builder (Mother pattern)
// ─────────────────────────────────────────────────────────────────────────────
// anAdvisorDef returns an AdvisorDef pre-populated with safe defaults.
// The cli package has the full Mother builder; this local variant exists so
// package-internal tests (package model) can construct fixtures without
// importing cli (which would create a circular dependency).
//
// Default values:
//
//	Name:        "test-advisor"
//	Scope:       nil (universal)
//
// AdvisorDef no longer carries SkillSource or Origin — those concepts live
// on AdvisorSource (the persisted shape). Tests that previously seeded
// SkillSource on an AdvisorDef literal are obsolete; AdvisorDef is now a
// runtime-only struct populated by Scanner output.
type advisorDefFixture struct {
	def AdvisorDef
}

func anAdvisorDef() *advisorDefFixture {
	return &advisorDefFixture{
		def: AdvisorDef{
			Name: "test-advisor",
		},
	}
}

func (f *advisorDefFixture) named(name string) *advisorDefFixture {
	f.def.Name = name
	return f
}

// withScope sets Scope to the provided tags (variadic).
// Passing no arguments sets Scope to nil (universal — applies to every project).
func (f *advisorDefFixture) withScope(scope ...string) *advisorDefFixture {
	if len(scope) == 0 {
		f.def.Scope = nil
		return f
	}
	f.def.Scope = append([]string{}, scope...)
	return f
}

func (f *advisorDefFixture) build() AdvisorDef {
	return f.def
}

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

// ---------------------------------------------------------------------------
// AdvisorDef.Validate
// ---------------------------------------------------------------------------

// TestAdvisorDef_Validate_HappyPath covers baseline valid AdvisorDef values.
func TestAdvisorDef_Validate_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		def  AdvisorDef
	}{
		{
			name: "shouldAcceptWhenAllFieldsValid",
			def: anAdvisorDef().
				named("security-advisor").
				withScope("security").
				build(),
		},
		{
			name: "shouldAcceptWhenScopeIsEmpty",
			def: anAdvisorDef().
				named("security-advisor").
				build(),
		},
		{
			name: "shouldAcceptWhenScopeIsBackendOnly",
			def: anAdvisorDef().
				named("security-advisor").
				withScope("backend").
				build(),
		},
		{
			name: "shouldAcceptWhenScopeIsFrontend",
			def: anAdvisorDef().
				named("ui-advisor").
				withScope("frontend").
				build(),
		},
		{
			name: "shouldAcceptWhenScopeIsBackend",
			def: anAdvisorDef().
				named("db-advisor").
				withScope("backend").
				build(),
		},
		{
			name: "shouldAcceptWhenAdvisorIsForCatalog",
			def: anAdvisorDef().
				named("perf-advisor").
				build(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.def.Validate(); err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

// TestAdvisorDef_Validate_InvalidName covers name validation rejections.
func TestAdvisorDef_Validate_InvalidName(t *testing.T) {
	tests := []struct {
		name    string
		def     AdvisorDef
		wantMsg string
	}{
		{
			name:    "shouldRejectWhenNameIsEmpty",
			def:     AdvisorDef{Name: ""},
			wantMsg: "name must not be empty",
		},
		{
			name:    "shouldRejectWhenNameMissingSuffix",
			def:     AdvisorDef{Name: "foo"},
			wantMsg: `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenNameIsBareAdvisorSuffix",
			def:     AdvisorDef{Name: "-advisor"},
			wantMsg: `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenNameHasUppercaseSuffix",
			def:     AdvisorDef{Name: "security-Advisor"},
			wantMsg: `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenNameHasTrailingWhitespace",
			def:     AdvisorDef{Name: "my-advisor "},
			wantMsg: `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenNameHasLeadingWhitespace",
			def:     AdvisorDef{Name: " my-advisor"},
			wantMsg: `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenNameUsesLegacyAdviserSuffix",
			def:     AdvisorDef{Name: "security-adviser"},
			wantMsg: `must end in "-advisor"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if err == nil {
				t.Errorf("Validate() returned nil, want error containing %q", tt.wantMsg)
				return
			}
			if !containsString(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// SkillSource validation tests were removed: AdvisorDef no longer carries
// SkillSource. Source URL grammar is now validated by AdvisorSource.Validate
// (see TestAdvisorSource_Validate below) and CatalogSource.Validate.

// TestAdvisorDefValidate_NoScopeChecks asserts that Validate is a no-op for
// scope content. Vocabulary checking and deduplication happen in
// NormalizeAdvisorScope at the loader boundary — Validate trusts that the scope
// has already been normalized and never inspects its vocabulary.
func TestAdvisorDefValidate_NoScopeChecks(t *testing.T) {
	tests := []struct {
		name    string
		def     AdvisorDef
		wantErr bool
		errMsg  string
	}{
		{
			// nil scope (universal) — Validate does not care.
			name:    "nilScope",
			def:     anAdvisorDef().named("test-advisor").withScope().build(),
			wantErr: false,
		},
		{
			// Valid scope tag — Validate still doesn't inspect it.
			name:    "validScope",
			def:     anAdvisorDef().named("test-advisor").withScope("backend").build(),
			wantErr: false,
		},
		{
			// Unknown scope tag: Validate must NOT reject it. Responsibility lies
			// with NormalizeAdvisorScope; the scope seen here is whatever the loader
			// produced. Validate trusts the loader boundary.
			name:    "unknownScopeStillPassesValidate",
			def:     anAdvisorDef().named("test-advisor").withScope("foo").build(),
			wantErr: false,
		},
		{
			// Invalid name + valid scope — name error must still surface.
			name:    "invalidNameWithValidScope",
			def:     anAdvisorDef().named("not-ending-correctly").withScope("backend").build(),
			wantErr: true,
			errMsg:  `must end in "-advisor"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
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

// ─────────────────────────────────────────────────────────────────────────────
// NormalizeAdvisorScope
// ─────────────────────────────────────────────────────────────────────────────

// TestNormalizeAdvisorScope is the table-driven heart of the soft-fallback
// contract. Unknown values and non-canonical forms are silently dropped;
// empty-after-normalization means universal (nil return, not empty slice).
func TestNormalizeAdvisorScope(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		// ── nil / empty input ──────────────────────────────────────────────────
		{
			name: "nilInput",
			in:   nil,
			want: nil, // universal
		},
		{
			name: "emptySliceInput",
			in:   []string{},
			want: nil, // universal; never returns empty non-nil slice
		},
		// ── single valid ───────────────────────────────────────────────────────
		{
			name: "singleValid",
			in:   []string{"backend"},
			want: []string{"backend"},
		},
		// ── multiple valid ─────────────────────────────────────────────────────
		{
			name: "multipleValid",
			in:   []string{"frontend", "testing"},
			want: []string{"frontend", "testing"}, // order preserved
		},
		// ── mixed valid + unknown ──────────────────────────────────────────────
		{
			name: "mixedValidAndUnknown",
			in:   []string{"backend", "foo"},
			want: []string{"backend"}, // unknown silently dropped, no error
		},
		// ── all unknown ────────────────────────────────────────────────────────
		{
			name: "allUnknown",
			in:   []string{"foo", "bar"},
			want: nil, // universal fallback
		},
		{
			name: "unknownOnlySingle",
			in:   []string{"foo"},
			want: nil, // universal fallback
		},
		// ── whitespace handling ────────────────────────────────────────────────
		{
			name: "whitespaceTrim",
			in:   []string{"  backend  "},
			want: []string{"backend"}, // trim before vocab check
		},
		{
			name: "whitespaceOnlyElement",
			in:   []string{"   "},
			want: nil, // dropped silently → universal
		},
		{
			name: "emptyStringElement",
			in:   []string{""},
			want: nil, // dropped silently → universal
		},
		{
			name: "mixedValidAndEmpty",
			in:   []string{"backend", ""},
			want: []string{"backend"}, // empty dropped silently
		},
		// ── deduplication ──────────────────────────────────────────────────────
		{
			name: "duplicates",
			in:   []string{"backend", "backend"},
			want: []string{"backend"}, // dedup, first-seen wins
		},
		{
			name: "duplicatesAfterTrim",
			in:   []string{"backend", " backend "},
			want: []string{"backend"}, // trim happens before dedup
		},
		{
			name: "dedupPreservesOrder",
			in:   []string{"testing", "backend", "testing"},
			want: []string{"testing", "backend"}, // first-seen order
		},
		// ── mixed-case (case-sensitive) ─────────────────────────────────────────
		{
			name: "mixedCaseUnknown",
			in:   []string{"Frontend"},
			want: nil, // case-sensitive — "Frontend" treated as unknown, dropped
		},
		{
			name: "mixedCasePlusValid",
			in:   []string{"Frontend", "backend"},
			want: []string{"backend"}, // only canonical form kept
		},
		// ── all 8 vocabulary values ────────────────────────────────────────────
		{
			// Pin: breaks if a vocab value is removed or renamed.
			name: "acceptsAll8Vocab",
			in: []string{
				"frontend", "backend", "testing", "architecture",
				"api", "security", "performance", "accessibility",
			},
			want: []string{
				"frontend", "backend", "testing", "architecture",
				"api", "security", "performance", "accessibility",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAdvisorScope(tt.in)
			if !slicesEqual(got, tt.want) {
				t.Errorf("NormalizeAdvisorScope(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestNormalizeAdvisorScope_DoesNotMutateInput verifies that the function
// does not modify the input slice. Passing a slice with duplicates must leave
// the original slice unchanged.
func TestNormalizeAdvisorScope_DoesNotMutateInput(t *testing.T) {
	in := []string{"backend", "backend"}
	original := make([]string, len(in))
	copy(original, in)

	_ = NormalizeAdvisorScope(in)

	if len(in) != len(original) {
		t.Fatalf("input slice length changed: got %d, want %d", len(in), len(original))
	}
	for i := range in {
		if in[i] != original[i] {
			t.Errorf("input[%d] = %q after call, want %q", i, in[i], original[i])
		}
	}
}

// TestNormalizeAdvisorScope_AllUnknownReturnsNil locks the universal-fallback
// contract: when all input values are unknown, the return must be nil (not
// an empty slice). Downstream filter semantics depend on nil == universal.
func TestNormalizeAdvisorScope_AllUnknownReturnsNil(t *testing.T) {
	got := NormalizeAdvisorScope([]string{"foo", "bar"})
	if got != nil {
		t.Errorf("NormalizeAdvisorScope([unknown values]) = %v (len=%d), want nil (universal)",
			got, len(got))
	}
}

// slicesEqual compares two string slices for equality, treating nil and nil
// as equal and distinguishing nil from empty ([]string{}).
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Origin validation tests were removed: AdvisorDef no longer carries Origin.
// "origin" is now derived at runtime from the AdvisorSource.Source scheme
// prefix (see resolveAdvisorOriginFromSource in cli) and is never persisted.

// ---------------------------------------------------------------------------
// CatalogSource.Validate
// ---------------------------------------------------------------------------

// TestCatalogSource_Validate_HappyPath covers valid CatalogSource URL formats.
func TestCatalogSource_Validate_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		src  CatalogSource
	}{
		{
			name: "shouldAcceptWhenLocalAbsolutePath",
			src:  CatalogSource{URL: "local:/abs/path"},
		},
		{
			name: "shouldAcceptWhenLocalRelativePath",
			src:  CatalogSource{URL: "local:./rel"},
		},
		{
			name: "shouldAcceptWhenGitHubOwnerRepo",
			src:  CatalogSource{URL: "github:acme/repo"},
		},
		{
			name: "shouldAcceptWhenGitHubOwnerRepoAtRef",
			src:  CatalogSource{URL: "github:acme/repo@main"},
		},
		{
			name: "shouldAcceptWhenGitLabGroupSubgroupRepo",
			src:  CatalogSource{URL: "gitlab:grp/subgrp/repo"},
		},
		{
			name: "shouldAcceptWhenNameIsPopulated",
			src:  CatalogSource{URL: "github:acme/repo", Name: "Acme catalog"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.src.Validate(); err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

// TestCatalogSource_Validate_Rejected covers invalid CatalogSource URL formats.
func TestCatalogSource_Validate_Rejected(t *testing.T) {
	tests := []struct {
		name    string
		src     CatalogSource
		wantMsg string
	}{
		{
			name:    "shouldRejectWhenURLIsEmpty",
			src:     CatalogSource{URL: ""},
			wantMsg: "url must not be empty",
		},
		{
			name:    "shouldRejectWhenSchemeMissing",
			src:     CatalogSource{URL: "acme/repo"},
			wantMsg: "unrecognised scheme",
		},
		{
			name:    "shouldRejectWhenSchemeIsHTTP",
			src:     CatalogSource{URL: "http:acme/repo"},
			wantMsg: "unrecognised scheme",
		},
		{
			name:    "shouldRejectWhenSchemeIsBitbucket",
			src:     CatalogSource{URL: "bitbucket:acme/repo"},
			wantMsg: "unrecognised scheme",
		},
		{
			name:    "shouldRejectWhenSchemIsBareFile",
			src:     CatalogSource{URL: "file:/some/path"},
			wantMsg: "unrecognised scheme",
		},
		{
			name:    "shouldRejectWhenGitHubBodyIsEmpty",
			src:     CatalogSource{URL: "github:"},
			wantMsg: "body must not be empty",
		},
		{
			name:    "shouldRejectWhenGitHubPathIsMissingSlash",
			src:     CatalogSource{URL: "github:owner-only"},
			wantMsg: `must be in the form "owner/repo"`,
		},
		{
			name:    "shouldRejectWhenGitHubRefIsEmpty",
			src:     CatalogSource{URL: "github:acme/repo@"},
			wantMsg: `ref after "@" must not be empty`,
		},
		{
			name:    "shouldRejectWhenGitLabBodyIsEmpty",
			src:     CatalogSource{URL: "gitlab:"},
			wantMsg: "body must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.src.Validate()
			if err == nil {
				t.Errorf("Validate() returned nil, want error containing %q", tt.wantMsg)
				return
			}
			if !containsString(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UserManifest.Validate — custom advisors and advisor catalog extensions
// ---------------------------------------------------------------------------

// TestUserManifest_Validate_Advisors covers UserManifest.Advisors[] validation
// paths under the new schema. The legacy CustomAdvisors / AdvisorCatalogs
// fields no longer exist; the new model collapses both into a single
// Advisors []AdvisorSource list.
func TestUserManifest_Validate_Advisors(t *testing.T) {
	base := func() UserManifest {
		return UserManifest{
			SchemaVersion: "devrune/v1",
			Agents:        []AgentRef{{Name: "claude"}},
		}
	}

	tests := []struct {
		name    string
		setup   func() UserManifest
		wantErr bool
		errMsg  string
	}{
		{
			name: "shouldAcceptWhenAdvisorsIsNil",
			setup: func() UserManifest {
				m := base()
				m.Advisors = nil
				return m
			},
			wantErr: false,
		},
		{
			name: "shouldAcceptWhenAdvisorsIsEmptySlice",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{}
				return m
			},
			wantErr: false,
		},
		{
			name: "shouldAcceptWhenAdvisorSourceIsValidGitHub",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{Source: "github:acme/advisor-catalog"},
				}
				return m
			},
			wantErr: false,
		},
		{
			name: "shouldAcceptWhenAdvisorSourceHasSelectList",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{Source: "github:acme/advisor-catalog", Select: []string{"custom-db-advisor"}},
				}
				return m
			},
			wantErr: false,
		},
		{
			name: "shouldRejectWhenAdvisorSourceURLIsDuplicated",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{Source: "github:acme/advisor-catalog"},
					{Source: "github:acme/advisor-catalog"},
				}
				return m
			},
			wantErr: true,
			errMsg:  "duplicate advisor source",
		},
		{
			name: "shouldRejectWhenAdvisorSourceURLIsInvalidScheme",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{Source: "http://example.com/catalog"},
				}
				return m
			},
			wantErr: true,
			errMsg:  "unrecognised scheme",
		},
		{
			name: "shouldRejectWhenSelectEntryShadowsNative",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{
						Source: "github:acme/advisor-catalog",
						// architect-advisor is reserved for native advisors.
						Select: []string{"architect-advisor"},
					},
				}
				return m
			},
			wantErr: true,
			errMsg:  "conflicts with a native DevRune advisor",
		},
		{
			name: "shouldRejectWhenSelectEntryHasInvalidSuffix",
			setup: func() UserManifest {
				m := base()
				m.Advisors = []AdvisorSource{
					{Source: "github:acme/advisor-catalog", Select: []string{"not-ending-correctly"}},
				}
				return m
			},
			wantErr: true,
			errMsg:  `must end in "-advisor"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			err := m.Validate()
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

// TestAdvisorSource_Validate covers AdvisorSource.Validate end-to-end.
func TestAdvisorSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		src     AdvisorSource
		wantErr bool
		errMsg  string
	}{
		{
			name:    "shouldAcceptWhenLocalSource",
			src:     AdvisorSource{Source: "local:/abs/path"},
			wantErr: false,
		},
		{
			name:    "shouldAcceptWhenGitHubSourceWithRef",
			src:     AdvisorSource{Source: "github:acme/repo@main"},
			wantErr: false,
		},
		{
			name:    "shouldAcceptWhenSelectIsEmpty",
			src:     AdvisorSource{Source: "github:acme/repo", Select: nil},
			wantErr: false,
		},
		{
			name:    "shouldAcceptWhenSelectHasValidNames",
			src:     AdvisorSource{Source: "github:acme/repo", Select: []string{"custom-db-advisor", "custom-perf-advisor"}},
			wantErr: false,
		},
		{
			name:    "shouldRejectWhenSourceIsEmpty",
			src:     AdvisorSource{Source: ""},
			wantErr: true,
			errMsg:  "url must not be empty",
		},
		{
			name:    "shouldRejectWhenSourceUsesUnknownScheme",
			src:     AdvisorSource{Source: "http://example.com"},
			wantErr: true,
			errMsg:  "unrecognised scheme",
		},
		{
			name:    "shouldRejectWhenSelectEntryIsEmpty",
			src:     AdvisorSource{Source: "github:acme/repo", Select: []string{""}},
			wantErr: true,
			errMsg:  "must not be empty",
		},
		{
			name:    "shouldRejectWhenSelectEntryMissingSuffix",
			src:     AdvisorSource{Source: "github:acme/repo", Select: []string{"foo"}},
			wantErr: true,
			errMsg:  `must end in "-advisor"`,
		},
		{
			name:    "shouldRejectWhenSelectEntryHasWhitespace",
			src:     AdvisorSource{Source: "github:acme/repo", Select: []string{" my-advisor"}},
			wantErr: true,
			errMsg:  `must end in "-advisor"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.src.Validate()
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

// TestAdvisorSource_AsCatalogSource verifies the lossless conversion to
// CatalogSource consumed by the Fetcher.
func TestAdvisorSource_AsCatalogSource(t *testing.T) {
	src := AdvisorSource{
		Source:      "github:acme/repo@main",
		LastFetched: "2024-01-15T10:00:00Z",
		Select:      []string{"custom-db-advisor"},
	}
	cs := src.AsCatalogSource()
	if cs.URL != src.Source {
		t.Errorf("URL = %q, want %q", cs.URL, src.Source)
	}
	if cs.LastFetched != src.LastFetched {
		t.Errorf("LastFetched = %q, want %q", cs.LastFetched, src.LastFetched)
	}
}
