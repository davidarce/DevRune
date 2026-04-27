// SPDX-License-Identifier: MIT

package parse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/parse"
)

func TestParseManifest(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
		errContains string
		check       func(t *testing.T, result interface{})
	}{
		{
			name:    "valid minimal manifest",
			fixture: "valid-minimal.yaml",
			wantErr: false,
		},
		{
			name:    "valid full manifest with select filter",
			fixture: "valid-full.yaml",
			wantErr: false,
		},
		{
			name:    "valid gitlab source refs",
			fixture: "valid-gitlab.yaml",
			wantErr: false,
		},
		{
			name:        "missing schemaVersion returns error",
			fixture:     "invalid-no-schema.yaml",
			wantErr:     true,
			errContains: "schemaVersion is required",
		},
		{
			name:        "no agents returns validation error",
			fixture:     "invalid-no-agents.yaml",
			wantErr:     true,
			errContains: "agent",
		},
		{
			name:        "unsupported schemaVersion returns error",
			fixture:     "invalid-bad-schema.yaml",
			wantErr:     true,
			errContains: "unsupported schemaVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, "manifests", tt.fixture)

			result, err := parse.ParseManifest(data)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none; result: %+v", result)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseManifest_ValidMinimal_Fields(t *testing.T) {
	data := mustReadFixture(t, "manifests", "valid-minimal.yaml")

	m, err := parse.ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.SchemaVersion != "devrune/v1" {
		t.Errorf("SchemaVersion = %q, want %q", m.SchemaVersion, "devrune/v1")
	}
	if len(m.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(m.Packages))
	}
	if m.Packages[0].Source == "" {
		t.Error("Packages[0].Source must not be empty")
	}
	if len(m.Agents) != 1 {
		t.Fatalf("len(Agents) = %d, want 1", len(m.Agents))
	}
	if m.Agents[0].Name != "claude" {
		t.Errorf("Agents[0].Name = %q, want %q", m.Agents[0].Name, "claude")
	}
}

func TestParseManifest_ValidFull_Fields(t *testing.T) {
	data := mustReadFixture(t, "manifests", "valid-full.yaml")

	m, err := parse.ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(m.Packages))
	}
	// First package has a select filter
	if m.Packages[0].Select == nil {
		t.Fatal("Packages[0].Select must not be nil")
	}
	if len(m.Packages[0].Select.Skills) != 2 {
		t.Errorf("len(Select.Skills) = %d, want 2", len(m.Packages[0].Select.Skills))
	}
	if len(m.MCPs) != 2 {
		t.Errorf("len(MCPs) = %d, want 2", len(m.MCPs))
	}
	if len(m.Agents) != 2 {
		t.Errorf("len(Agents) = %d, want 2", len(m.Agents))
	}
	if len(m.Workflows) != 1 {
		t.Errorf("len(Workflows) = %d, want 1 (sdd)", len(m.Workflows))
	}
	if m.Install.LinkMode != "copy" {
		t.Errorf("Install.LinkMode = %q, want %q", m.Install.LinkMode, "copy")
	}
}

func TestParseManifest_MalformedYAML(t *testing.T) {
	data := []byte("schemaVersion: devrune/v1\nagents:\n  - name: [invalid")

	_, err := parse.ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for malformed YAML but got none")
	}
}

func TestParseManifest_ValidFull_CatalogsField(t *testing.T) {
	data := mustReadFixture(t, "manifests", "valid-full.yaml")

	m, err := parse.ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Catalogs) == 0 {
		t.Fatal("expected Catalogs to be populated from valid-full.yaml, got empty slice")
	}
	if m.Catalogs[0] != "github:davidarce/devrune-starter-catalog" {
		t.Errorf("Catalogs[0] = %q, want %q", m.Catalogs[0], "github:davidarce/devrune-starter-catalog")
	}
}

func TestSerializeManifest_CatalogsRoundTrip(t *testing.T) {
	original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
catalogs:
  - github:org/catalog-a
  - github:org/catalog-b
`))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	serialized, err := parse.SerializeManifest(original)
	if err != nil {
		t.Fatalf("SerializeManifest: %v", err)
	}

	reparsed, err := parse.ParseManifest(serialized)
	if err != nil {
		t.Fatalf("ParseManifest (reparsed): %v", err)
	}

	if len(reparsed.Catalogs) != 2 {
		t.Fatalf("Catalogs length after round-trip = %d, want 2", len(reparsed.Catalogs))
	}
	if reparsed.Catalogs[0] != "github:org/catalog-a" {
		t.Errorf("Catalogs[0] = %q, want %q", reparsed.Catalogs[0], "github:org/catalog-a")
	}
	if reparsed.Catalogs[1] != "github:org/catalog-b" {
		t.Errorf("Catalogs[1] = %q, want %q", reparsed.Catalogs[1], "github:org/catalog-b")
	}
}

func TestSerializeManifest_RoundTrip(t *testing.T) {
	data := mustReadFixture(t, "manifests", "valid-full.yaml")

	original, err := parse.ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	serialized, err := parse.SerializeManifest(original)
	if err != nil {
		t.Fatalf("SerializeManifest: %v", err)
	}

	reparsed, err := parse.ParseManifest(serialized)
	if err != nil {
		t.Fatalf("ParseManifest (reparsed): %v", err)
	}

	if reparsed.SchemaVersion != original.SchemaVersion {
		t.Errorf("SchemaVersion mismatch: %q != %q", reparsed.SchemaVersion, original.SchemaVersion)
	}
	if len(reparsed.Packages) != len(original.Packages) {
		t.Errorf("Packages length mismatch: %d != %d", len(reparsed.Packages), len(original.Packages))
	}
	if len(reparsed.Agents) != len(original.Agents) {
		t.Errorf("Agents length mismatch: %d != %d", len(reparsed.Agents), len(original.Agents))
	}
}

// TestSerializeManifest_CustomAdvisors_RoundTrip verifies the five round-trip
// scenarios for customAdvisors and advisorCatalogs.
func TestSerializeManifest_CustomAdvisors_RoundTrip(t *testing.T) {
	t.Run("no customAdvisors or advisorCatalogs — fields absent after round-trip", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
`))
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		if strings.Contains(string(serialized), "customAdvisors") {
			t.Errorf("serialized output must not contain 'customAdvisors' when field is empty, got:\n%s", serialized)
		}
		if strings.Contains(string(serialized), "advisorCatalogs") {
			t.Errorf("serialized output must not contain 'advisorCatalogs' when field is empty, got:\n%s", serialized)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}
		if len(reparsed.CustomAdvisors) != 0 {
			t.Errorf("CustomAdvisors after round-trip = %d, want 0", len(reparsed.CustomAdvisors))
		}
		if len(reparsed.AdvisorCatalogs) != 0 {
			t.Errorf("AdvisorCatalogs after round-trip = %d, want 0", len(reparsed.AdvisorCatalogs))
		}
	})

	t.Run("one local custom advisor — round-trips correctly", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: custom-security-advisor
    description: Reviews code for security vulnerabilities
    skillSource: local:./advisors/custom-security-advisor
    scope: [backend]
    origin: local
`))
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		if len(original.CustomAdvisors) != 1 {
			t.Fatalf("CustomAdvisors length = %d, want 1", len(original.CustomAdvisors))
		}
		if original.CustomAdvisors[0].Origin != "local" {
			t.Errorf("Origin = %q, want %q", original.CustomAdvisors[0].Origin, "local")
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}

		if len(reparsed.CustomAdvisors) != 1 {
			t.Fatalf("CustomAdvisors length after round-trip = %d, want 1", len(reparsed.CustomAdvisors))
		}
		got := reparsed.CustomAdvisors[0]
		want := original.CustomAdvisors[0]
		if got.Name != want.Name {
			t.Errorf("Name = %q, want %q", got.Name, want.Name)
		}
		if got.Description != want.Description {
			t.Errorf("Description = %q, want %q", got.Description, want.Description)
		}
		if got.SkillSource != want.SkillSource {
			t.Errorf("SkillSource = %q, want %q", got.SkillSource, want.SkillSource)
		}
		if len(got.Scope) != len(want.Scope) {
			t.Errorf("Scope length = %d, want %d", len(got.Scope), len(want.Scope))
		} else {
			for i := range want.Scope {
				if got.Scope[i] != want.Scope[i] {
					t.Errorf("Scope[%d] = %q, want %q", i, got.Scope[i], want.Scope[i])
				}
			}
		}
		if got.Origin != want.Origin {
			t.Errorf("Origin = %q, want %q", got.Origin, want.Origin)
		}
	})

	t.Run("catalog-origin advisor plus advisorCatalogs entry — both round-trip", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: perf-advisor
    description: Performance analysis advisor
    skillSource: github:org/catalog@main
    origin: catalog
advisorCatalogs:
  - url: github:org/catalog@main
    name: org-catalog
    lastFetched: "2024-01-15T10:00:00Z"
`))
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}

		if len(reparsed.CustomAdvisors) != 1 {
			t.Fatalf("CustomAdvisors length after round-trip = %d, want 1", len(reparsed.CustomAdvisors))
		}
		if reparsed.CustomAdvisors[0].Origin != "catalog" {
			t.Errorf("Origin = %q, want %q", reparsed.CustomAdvisors[0].Origin, "catalog")
		}
		if len(reparsed.AdvisorCatalogs) != 1 {
			t.Fatalf("AdvisorCatalogs length after round-trip = %d, want 1", len(reparsed.AdvisorCatalogs))
		}
		gotCatalog := reparsed.AdvisorCatalogs[0]
		wantCatalog := original.AdvisorCatalogs[0]
		if gotCatalog.URL != wantCatalog.URL {
			t.Errorf("AdvisorCatalogs[0].URL = %q, want %q", gotCatalog.URL, wantCatalog.URL)
		}
		if gotCatalog.Name != wantCatalog.Name {
			t.Errorf("AdvisorCatalogs[0].Name = %q, want %q", gotCatalog.Name, wantCatalog.Name)
		}
		if gotCatalog.LastFetched != wantCatalog.LastFetched {
			t.Errorf("AdvisorCatalogs[0].LastFetched = %q, want %q", gotCatalog.LastFetched, wantCatalog.LastFetched)
		}
	})

	t.Run("explicit empty list customAdvisors/advisorCatalogs — key dropped after round-trip (omitempty)", func(t *testing.T) {
		// Build YAML manually with explicit empty lists.
		rawYAML := []byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors: []
advisorCatalogs: []
`)
		// ParseManifest will reject the manifest if [] leaves invalid state — but
		// empty slices are valid (no entries to validate). Unmarshal produces nil
		// or empty slices; either way the manifest is valid.
		original, err := parse.ParseManifest(rawYAML)
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		// omitempty must drop the key entirely when the slice is nil/empty.
		if strings.Contains(string(serialized), "customAdvisors") {
			t.Errorf("serialized output must not contain 'customAdvisors' for empty slice (omitempty), got:\n%s", serialized)
		}
		if strings.Contains(string(serialized), "advisorCatalogs") {
			t.Errorf("serialized output must not contain 'advisorCatalogs' for empty slice (omitempty), got:\n%s", serialized)
		}
	})

	t.Run("populated customAdvisors and advisorCatalogs — serialize then parse deep-equal", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: custom-security-advisor
    description: Security review advisor
    skillSource: local:./advisors/custom-security-advisor
    scope: [backend]
    origin: local
  - name: perf-advisor
    description: Performance analysis advisor
    skillSource: github:org/catalog@main
    origin: catalog
advisorCatalogs:
  - url: github:org/catalog@main
    name: org-catalog
    lastFetched: "2024-01-15T10:00:00Z"
`))
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}

		if len(reparsed.CustomAdvisors) != len(original.CustomAdvisors) {
			t.Fatalf("CustomAdvisors length mismatch: %d != %d", len(reparsed.CustomAdvisors), len(original.CustomAdvisors))
		}
		for i, want := range original.CustomAdvisors {
			got := reparsed.CustomAdvisors[i]
			if got.Name != want.Name {
				t.Errorf("CustomAdvisors[%d].Name = %q, want %q", i, got.Name, want.Name)
			}
			if got.Description != want.Description {
				t.Errorf("CustomAdvisors[%d].Description = %q, want %q", i, got.Description, want.Description)
			}
			if got.SkillSource != want.SkillSource {
				t.Errorf("CustomAdvisors[%d].SkillSource = %q, want %q", i, got.SkillSource, want.SkillSource)
			}
			if len(got.Scope) != len(want.Scope) {
				t.Errorf("CustomAdvisors[%d].Scope length = %d, want %d", i, len(got.Scope), len(want.Scope))
			} else {
				for j := range want.Scope {
					if got.Scope[j] != want.Scope[j] {
						t.Errorf("CustomAdvisors[%d].Scope[%d] = %q, want %q", i, j, got.Scope[j], want.Scope[j])
					}
				}
			}
			if got.Origin != want.Origin {
				t.Errorf("CustomAdvisors[%d].Origin = %q, want %q", i, got.Origin, want.Origin)
			}
		}
		if len(reparsed.AdvisorCatalogs) != len(original.AdvisorCatalogs) {
			t.Fatalf("AdvisorCatalogs length mismatch: %d != %d", len(reparsed.AdvisorCatalogs), len(original.AdvisorCatalogs))
		}
		for i, want := range original.AdvisorCatalogs {
			got := reparsed.AdvisorCatalogs[i]
			if got.URL != want.URL {
				t.Errorf("AdvisorCatalogs[%d].URL = %q, want %q", i, got.URL, want.URL)
			}
			if got.Name != want.Name {
				t.Errorf("AdvisorCatalogs[%d].Name = %q, want %q", i, got.Name, want.Name)
			}
			if got.LastFetched != want.LastFetched {
				t.Errorf("AdvisorCatalogs[%d].LastFetched = %q, want %q", i, got.LastFetched, want.LastFetched)
			}
		}
	})
}

// TestParseManifest_IgnoresScopeInManifest verifies that any `scope:` field in
// a customAdvisors[] entry is silently ignored on parse. SKILL.md frontmatter
// on disk is the single source of truth for advisor scope; the manifest does
// NOT participate. Older devrune.yaml files with `scope:` (or the legacy
// `tier:`) parse cleanly and the field is left empty.
func TestParseManifest_IgnoresScopeInManifest(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "scope list — ignored",
			yaml: `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: my-advisor
    description: An advisor
    skillSource: local:./advisors/my-advisor
    scope: [backend, testing]
    origin: local
`,
		},
		{
			name: "scope unknown values — ignored",
			yaml: `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: my-advisor
    description: An advisor
    skillSource: local:./advisors/my-advisor
    scope: [foo, Frontend]
    origin: local
`,
		},
		{
			name: "scope absent — universal as before",
			yaml: `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
customAdvisors:
  - name: my-advisor
    description: An advisor
    skillSource: local:./advisors/my-advisor
    origin: local
`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m, err := parse.ParseManifest([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseManifest: %v", err)
			}
			if len(m.CustomAdvisors) != 1 {
				t.Fatalf("CustomAdvisors length = %d, want 1", len(m.CustomAdvisors))
			}
			if got := m.CustomAdvisors[0].Scope; got != nil {
				t.Errorf("Scope = %v, want nil (manifest must NOT carry scope; disk is truth)", got)
			}

			serialized, err := parse.SerializeManifest(m)
			if err != nil {
				t.Fatalf("SerializeManifest: %v", err)
			}
			if strings.Contains(string(serialized), "scope:") {
				t.Errorf("serialized manifest contains 'scope:' — must NOT be persisted:\n%s", serialized)
			}
		})
	}
}


// mustReadFixture reads a test fixture file from testdata/<dir>/<name>.
// It fails the test immediately if the file cannot be read.
func mustReadFixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	// Walk up from the test package to reach testdata at the module root.
	path := filepath.Join("..", "..", "testdata", dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s/%s: %v", dir, name, err)
	}
	return data
}
