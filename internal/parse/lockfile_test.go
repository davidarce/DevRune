package parse_test

import (
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

func TestParseLockfile(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid minimal lockfile",
			fixture: "valid-minimal.yaml",
			wantErr: false,
		},
		{
			name:    "valid full lockfile with MCPs and workflows",
			fixture: "valid-full.yaml",
			wantErr: false,
		},
		{
			name:        "missing schemaVersion returns error",
			fixture:     "invalid-no-schema.yaml",
			wantErr:     true,
			errContains: "schemaVersion is required",
		},
		{
			name:        "missing manifestHash returns error",
			fixture:     "invalid-no-manifest-hash.yaml",
			wantErr:     true,
			errContains: "manifestHash is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, "lockfiles", tt.fixture)

			result, err := parse.ParseLockfile(data)

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

func TestParseLockfile_ValidMinimal_Fields(t *testing.T) {
	data := mustReadFixture(t, "lockfiles", "valid-minimal.yaml")

	lf, err := parse.ParseLockfile(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lf.SchemaVersion != "devrune/lock/v1" {
		t.Errorf("SchemaVersion = %q, want %q", lf.SchemaVersion, "devrune/lock/v1")
	}
	if lf.ManifestHash == "" {
		t.Error("ManifestHash must not be empty")
	}
	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}
	pkg := lf.Packages[0]
	if pkg.Source.Scheme != model.SchemeGitHub {
		t.Errorf("Packages[0].Source.Scheme = %q, want %q", pkg.Source.Scheme, model.SchemeGitHub)
	}
	if pkg.Hash == "" {
		t.Error("Packages[0].Hash must not be empty")
	}
	if len(pkg.Contents) != 1 {
		t.Fatalf("len(Contents) = %d, want 1", len(pkg.Contents))
	}
	if pkg.Contents[0].Kind != model.KindSkill {
		t.Errorf("Contents[0].Kind = %q, want %q", pkg.Contents[0].Kind, model.KindSkill)
	}
}

func TestParseLockfile_ValidFull_Fields(t *testing.T) {
	data := mustReadFixture(t, "lockfiles", "valid-full.yaml")

	lf, err := parse.ParseLockfile(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lf.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(lf.Packages))
	}
	if len(lf.MCPs) != 1 {
		t.Errorf("len(MCPs) = %d, want 1", len(lf.MCPs))
	}
	if len(lf.Workflows) != 1 {
		t.Errorf("len(Workflows) = %d, want 1", len(lf.Workflows))
	}
	if lf.Workflows[0].Name != "sdd" {
		t.Errorf("Workflows[0].Name = %q, want %q", lf.Workflows[0].Name, "sdd")
	}

	// Second package uses gitlab scheme with custom host
	gitlabPkg := lf.Packages[1]
	if gitlabPkg.Source.Scheme != model.SchemeGitLab {
		t.Errorf("Packages[1].Source.Scheme = %q, want %q", gitlabPkg.Source.Scheme, model.SchemeGitLab)
	}
	if gitlabPkg.Source.Host != "gitlab.example.com" {
		t.Errorf("Packages[1].Source.Host = %q, want %q", gitlabPkg.Source.Host, "gitlab.example.com")
	}
}

func TestParseLockfile_MalformedYAML(t *testing.T) {
	data := []byte("schemaVersion: devrune/lock/v1\nmanifestHash: sha256:abc\npackages:\n  - [invalid")

	_, err := parse.ParseLockfile(data)
	if err == nil {
		t.Fatal("expected error for malformed YAML but got none")
	}
}

func TestParseLockfile_WrongSchemaVersion(t *testing.T) {
	data := []byte(`schemaVersion: devrune/lock/v99
manifestHash: sha256:abc123
packages: []
`)

	_, err := parse.ParseLockfile(data)
	if err == nil {
		t.Fatal("expected error for unsupported schemaVersion but got none")
	}
	if !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Errorf("error %q does not contain %q", err.Error(), "unsupported schemaVersion")
	}
}

func TestSerializeLockfile_RoundTrip(t *testing.T) {
	data := mustReadFixture(t, "lockfiles", "valid-full.yaml")

	original, err := parse.ParseLockfile(data)
	if err != nil {
		t.Fatalf("ParseLockfile: %v", err)
	}

	serialized, err := parse.SerializeLockfile(original)
	if err != nil {
		t.Fatalf("SerializeLockfile: %v", err)
	}

	reparsed, err := parse.ParseLockfile(serialized)
	if err != nil {
		t.Fatalf("ParseLockfile (reparsed): %v", err)
	}

	if reparsed.SchemaVersion != original.SchemaVersion {
		t.Errorf("SchemaVersion mismatch: %q != %q", reparsed.SchemaVersion, original.SchemaVersion)
	}
	if reparsed.ManifestHash != original.ManifestHash {
		t.Errorf("ManifestHash mismatch: %q != %q", reparsed.ManifestHash, original.ManifestHash)
	}
	if len(reparsed.Packages) != len(original.Packages) {
		t.Errorf("Packages length mismatch: %d != %d", len(reparsed.Packages), len(original.Packages))
	}
	if len(reparsed.MCPs) != len(original.MCPs) {
		t.Errorf("MCPs length mismatch: %d != %d", len(reparsed.MCPs), len(original.MCPs))
	}
	if len(reparsed.Workflows) != len(original.Workflows) {
		t.Errorf("Workflows length mismatch: %d != %d", len(reparsed.Workflows), len(original.Workflows))
	}
}

func TestSerializeLockfile_IsDeterministic(t *testing.T) {
	// Build a lockfile with unsorted entries and verify serialization is stable.
	lf := model.Lockfile{
		SchemaVersion: "devrune/lock/v1",
		ManifestHash:  "sha256:abc123",
		Packages: []model.LockedPackage{
			{
				Source: model.SourceRef{Scheme: model.SchemeGitHub, Owner: "z-owner", Repo: "z-repo", Ref: "v1.0.0"},
				Hash:   "sha256:zzz",
				Contents: []model.ContentItem{
					{Kind: model.KindRule, Name: "rule-b", Path: "rules/rule-b/"},
					{Kind: model.KindSkill, Name: "skill-a", Path: "skills/skill-a/"},
				},
			},
			{
				Source: model.SourceRef{Scheme: model.SchemeGitHub, Owner: "a-owner", Repo: "a-repo", Ref: "v1.0.0"},
				Hash:   "sha256:aaa",
				Contents: []model.ContentItem{},
			},
		},
		Workflows: []model.LockedWorkflow{
			{Source: model.SourceRef{Scheme: model.SchemeGitHub, Owner: "o", Repo: "r", Ref: "v1"}, Hash: "sha256:wfz", Name: "z-workflow"},
			{Source: model.SourceRef{Scheme: model.SchemeGitHub, Owner: "o", Repo: "r", Ref: "v1"}, Hash: "sha256:wfa", Name: "a-workflow"},
		},
	}

	out1, err := parse.SerializeLockfile(lf)
	if err != nil {
		t.Fatalf("first serialization: %v", err)
	}
	out2, err := parse.SerializeLockfile(lf)
	if err != nil {
		t.Fatalf("second serialization: %v", err)
	}

	if string(out1) != string(out2) {
		t.Error("SerializeLockfile is not deterministic: two calls with same input produced different output")
	}

	// Packages must be sorted by source string: a-owner/a-repo comes before z-owner/z-repo.
	s := string(out1)
	aIdx := strings.Index(s, "a-owner")
	zIdx := strings.Index(s, "z-owner")
	if aIdx < 0 || zIdx < 0 || aIdx > zIdx {
		t.Error("expected a-owner to appear before z-owner (packages sorted by source string)")
	}

	// Workflows must be sorted by name: a-workflow before z-workflow.
	awIdx := strings.Index(s, "a-workflow")
	zwIdx := strings.Index(s, "z-workflow")
	if awIdx < 0 || zwIdx < 0 || awIdx > zwIdx {
		t.Error("expected a-workflow to appear before z-workflow (workflows sorted by name)")
	}
}
