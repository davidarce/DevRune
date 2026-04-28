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

// TestSerializeManifest_Advisors_RoundTrip verifies round-trip behavior for
// the unified Advisors []AdvisorSource schema. The legacy customAdvisors /
// advisorCatalogs fields are gone; everything is collapsed into Advisors[].
func TestSerializeManifest_Advisors_RoundTrip(t *testing.T) {
	t.Run("no advisors — field absent after round-trip", func(t *testing.T) {
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

		if strings.Contains(string(serialized), "advisors:") {
			t.Errorf("serialized output must not contain 'advisors:' when field is empty, got:\n%s", serialized)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}
		if len(reparsed.Advisors) != 0 {
			t.Errorf("Advisors after round-trip = %d, want 0", len(reparsed.Advisors))
		}
	})

	t.Run("local source with select — round-trips correctly", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
advisors:
  - source: local:./advisors/custom-security-advisor
    select:
      - custom-security-advisor
`))
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		if len(original.Advisors) != 1 {
			t.Fatalf("Advisors length = %d, want 1", len(original.Advisors))
		}
		if original.Advisors[0].Source != "local:./advisors/custom-security-advisor" {
			t.Errorf("Source = %q, want %q", original.Advisors[0].Source, "local:./advisors/custom-security-advisor")
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		reparsed, err := parse.ParseManifest(serialized)
		if err != nil {
			t.Fatalf("ParseManifest (reparsed): %v", err)
		}

		if len(reparsed.Advisors) != 1 {
			t.Fatalf("Advisors length after round-trip = %d, want 1", len(reparsed.Advisors))
		}
		got := reparsed.Advisors[0]
		want := original.Advisors[0]
		if got.Source != want.Source {
			t.Errorf("Source = %q, want %q", got.Source, want.Source)
		}
		if len(got.Select) != len(want.Select) {
			t.Errorf("Select length = %d, want %d", len(got.Select), len(want.Select))
		} else {
			for i := range want.Select {
				if got.Select[i] != want.Select[i] {
					t.Errorf("Select[%d] = %q, want %q", i, got.Select[i], want.Select[i])
				}
			}
		}
	})

	t.Run("github source with lastFetched — round-trips correctly", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
advisors:
  - source: github:org/catalog@main
    lastFetched: "2024-01-15T10:00:00Z"
    select:
      - perf-advisor
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

		if len(reparsed.Advisors) != 1 {
			t.Fatalf("Advisors length after round-trip = %d, want 1", len(reparsed.Advisors))
		}
		got := reparsed.Advisors[0]
		if got.Source != "github:org/catalog@main" {
			t.Errorf("Source = %q, want %q", got.Source, "github:org/catalog@main")
		}
		if got.LastFetched != "2024-01-15T10:00:00Z" {
			t.Errorf("LastFetched = %q, want %q", got.LastFetched, "2024-01-15T10:00:00Z")
		}
	})

	t.Run("explicit empty list advisors — key dropped after round-trip (omitempty)", func(t *testing.T) {
		rawYAML := []byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
advisors: []
`)
		original, err := parse.ParseManifest(rawYAML)
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}

		serialized, err := parse.SerializeManifest(original)
		if err != nil {
			t.Fatalf("SerializeManifest: %v", err)
		}

		// omitempty must drop the key entirely when the slice is nil/empty.
		if strings.Contains(string(serialized), "advisors:") {
			t.Errorf("serialized output must not contain 'advisors:' for empty slice (omitempty), got:\n%s", serialized)
		}
	})

	t.Run("populated multi-source advisors — serialize then parse deep-equal", func(t *testing.T) {
		original, err := parse.ParseManifest([]byte(`schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:owner/repo@v1.0.0
advisors:
  - source: local:./advisors/custom-security-advisor
    select:
      - custom-security-advisor
  - source: github:org/catalog@main
    lastFetched: "2024-01-15T10:00:00Z"
    select:
      - perf-advisor
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

		if len(reparsed.Advisors) != len(original.Advisors) {
			t.Fatalf("Advisors length mismatch: %d != %d", len(reparsed.Advisors), len(original.Advisors))
		}
		for i, want := range original.Advisors {
			got := reparsed.Advisors[i]
			if got.Source != want.Source {
				t.Errorf("Advisors[%d].Source = %q, want %q", i, got.Source, want.Source)
			}
			if got.LastFetched != want.LastFetched {
				t.Errorf("Advisors[%d].LastFetched = %q, want %q", i, got.LastFetched, want.LastFetched)
			}
			if len(got.Select) != len(want.Select) {
				t.Errorf("Advisors[%d].Select length = %d, want %d", i, len(got.Select), len(want.Select))
				continue
			}
			for j := range want.Select {
				if got.Select[j] != want.Select[j] {
					t.Errorf("Advisors[%d].Select[%d] = %q, want %q", i, j, got.Select[j], want.Select[j])
				}
			}
		}
	})
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
