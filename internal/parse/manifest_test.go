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
		t.Errorf("len(Workflows) = %d, want 1", len(m.Workflows))
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
