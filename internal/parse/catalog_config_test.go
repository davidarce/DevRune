// SPDX-License-Identifier: MIT

package parse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/parse"
)

func TestParseCatalogConfig(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid single source",
			fixture: "valid-single-source.yaml",
			wantErr: false,
		},
		{
			name:    "valid multiple sources",
			fixture: "valid-multi-source.yaml",
			wantErr: false,
		},
		{
			name:        "missing schemaVersion returns error",
			fixture:     "invalid-no-schema.yaml",
			wantErr:     true,
			errContains: "schemaVersion is required",
		},
		{
			name:        "unsupported schemaVersion returns error",
			fixture:     "invalid-bad-schema.yaml",
			wantErr:     true,
			errContains: "unsupported schemaVersion",
		},
		{
			name:        "empty sources returns error",
			fixture:     "invalid-empty-sources.yaml",
			wantErr:     true,
			errContains: "sources must not be empty",
		},
		{
			name:        "invalid source ref syntax returns error",
			fixture:     "invalid-bad-source-ref.yaml",
			wantErr:     true,
			errContains: "sources[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, "catalog-configs", tt.fixture)

			result, err := parse.ParseCatalogConfig(data)

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

func TestParseCatalogConfig_ValidSingleSource_Fields(t *testing.T) {
	data := mustReadFixture(t, "catalog-configs", "valid-single-source.yaml")

	cfg, err := parse.ParseCatalogConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SchemaVersion != "devrune-catalog/v1" {
		t.Errorf("SchemaVersion = %q, want %q", cfg.SchemaVersion, "devrune-catalog/v1")
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(cfg.Sources))
	}
	if cfg.Sources[0] != "github:davidarce/devrune-starter-catalog" {
		t.Errorf("Sources[0] = %q, want %q", cfg.Sources[0], "github:davidarce/devrune-starter-catalog")
	}
}

func TestParseCatalogConfig_ValidMultiSource_Fields(t *testing.T) {
	data := mustReadFixture(t, "catalog-configs", "valid-multi-source.yaml")

	cfg, err := parse.ParseCatalogConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Sources) != 3 {
		t.Fatalf("len(Sources) = %d, want 3", len(cfg.Sources))
	}
}

func TestParseCatalogConfig_MalformedYAML(t *testing.T) {
	data := []byte("schemaVersion: devrune-catalog/v1\nsources:\n  - [invalid")

	_, err := parse.ParseCatalogConfig(data)
	if err == nil {
		t.Fatal("expected error for malformed YAML but got none")
	}
}

func TestDetectCatalogConfig_FileNotFound_ReturnsNil(t *testing.T) {
	dir := t.TempDir()

	cfg, err := parse.DetectCatalogConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got: %+v", cfg)
	}
}

func TestDetectCatalogConfig_ValidFile_ReturnsParsedConfig(t *testing.T) {
	dir := t.TempDir()
	content := []byte("schemaVersion: devrune-catalog/v1\nsources:\n  - github:myorg/repo\n")
	if err := os.WriteFile(filepath.Join(dir, "devrune.catalog.yaml"), content, 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	cfg, err := parse.DetectCatalogConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config but got nil")
	}
	if cfg.SchemaVersion != "devrune-catalog/v1" {
		t.Errorf("SchemaVersion = %q, want %q", cfg.SchemaVersion, "devrune-catalog/v1")
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(cfg.Sources))
	}
}

func TestDetectCatalogConfig_MalformedFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := []byte("schemaVersion: devrune-catalog/v99\nsources:\n  - github:myorg/repo\n")
	if err := os.WriteFile(filepath.Join(dir, "devrune.catalog.yaml"), content, 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	cfg, err := parse.DetectCatalogConfig(dir)
	if err == nil {
		t.Fatalf("expected error for malformed catalog config but got nil; cfg: %+v", cfg)
	}
	if cfg != nil {
		t.Fatalf("expected nil config on error, got: %+v", cfg)
	}
	if !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Errorf("error %q does not contain %q", err.Error(), "unsupported schemaVersion")
	}
}
