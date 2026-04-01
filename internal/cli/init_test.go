// SPDX-License-Identifier: MIT

package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/cli"
)

// runInitCmd is a helper that executes the devrune init command with the given
// args against a temp directory. It returns (stdout, stderr, error).
func runInitCmd(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	root := cli.NewRootCmd("test", "abc123")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	// Prepend the global --dir flag so all file I/O is isolated to dir.
	fullArgs := append([]string{"--dir", dir, "--non-interactive", "init"}, args...)
	root.SetArgs(fullArgs)

	err := root.ExecuteContext(context.Background())
	return buf.String(), err
}

// TestInit_CatalogFlag_WritesManifestWithCatalogs verifies that --catalog writes
// the manifest with the catalogs: key populated.
func TestInit_CatalogFlag_WritesManifestWithCatalogs(t *testing.T) {
	dir := t.TempDir()

	// We don't check the top-level error: the manifest is written before
	// resolve/install, so we verify manifest content even if resolve fails
	// (no network in tests).
	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--catalog", "github:myorg/my-catalog",
	)

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)
	if !strings.Contains(manifestContent, "myorg/my-catalog") {
		t.Errorf("manifest does not contain catalog source; got:\n%s", manifestContent)
	}
	if !strings.Contains(manifestContent, "catalogs:") {
		t.Errorf("manifest does not contain 'catalogs:' key; got:\n%s", manifestContent)
	}
}

// TestInit_CatalogFlag_MultipleSources verifies that multiple --catalog flags
// each appear in the manifest catalogs: list.
func TestInit_CatalogFlag_MultipleSources(t *testing.T) {
	dir := t.TempDir()

	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--catalog", "github:org/catalog-a",
		"--catalog", "github:org/catalog-b",
	)

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)
	if !strings.Contains(manifestContent, "catalog-a") {
		t.Errorf("manifest does not contain catalog-a; got:\n%s", manifestContent)
	}
	if !strings.Contains(manifestContent, "catalog-b") {
		t.Errorf("manifest does not contain catalog-b; got:\n%s", manifestContent)
	}
}

// TestInit_CatalogFlag_MergesWithExistingManifest verifies that when devrune.yaml
// already exists with catalogs:, running init with --catalog merges both lists.
func TestInit_CatalogFlag_MergesWithExistingManifest(t *testing.T) {
	dir := t.TempDir()

	// Pre-write a manifest with an existing catalog entry.
	existing := "schemaVersion: devrune/v1\nagents:\n  - name: claude\ncatalogs:\n  - github:org/existing-catalog\n"
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write existing manifest: %v", err)
	}

	// Run init with --catalog adding a new entry. --force to overwrite.
	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--catalog", "github:org/new-catalog",
		"--force",
	)

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)
	if !strings.Contains(manifestContent, "existing-catalog") {
		t.Errorf("manifest does not contain existing catalog; got:\n%s", manifestContent)
	}
	if !strings.Contains(manifestContent, "new-catalog") {
		t.Errorf("manifest does not contain new catalog; got:\n%s", manifestContent)
	}
}

// TestInit_CatalogFlag_Deduplicates verifies that if --catalog supplies a source
// already in the existing manifest catalogs:, it appears only once.
func TestInit_CatalogFlag_Deduplicates(t *testing.T) {
	dir := t.TempDir()

	// Pre-write a manifest with a catalog entry.
	existing := "schemaVersion: devrune/v1\nagents:\n  - name: claude\ncatalogs:\n  - github:org/shared-catalog\n"
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write existing manifest: %v", err)
	}

	// Run init with --catalog supplying the same source. --force to overwrite.
	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--catalog", "github:org/shared-catalog",
		"--force",
	)

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)

	// The shared-catalog source should appear in the catalogs: section only once.
	// (It may also appear in packages: because catalog sources are merged into packages.)
	// Count occurrences within the catalogs: block specifically.
	catalogsIdx := strings.Index(manifestContent, "catalogs:")
	if catalogsIdx == -1 {
		t.Fatalf("manifest does not contain 'catalogs:' key; got:\n%s", manifestContent)
	}
	catalogsSection := manifestContent[catalogsIdx:]
	count := strings.Count(catalogsSection, "shared-catalog")
	if count != 1 {
		t.Errorf("expected shared-catalog to appear exactly once in catalogs: section (deduplication), got %d; catalogs section:\n%s",
			count, catalogsSection)
	}
}

// TestInit_FlagRegistered_Catalog verifies that --catalog flag is registered on
// the init command and --import-catalog is not.
func TestInit_FlagRegistered_Catalog(t *testing.T) {
	root := cli.NewRootCmd("test", "abc123")
	initCmd, _, err := root.Find([]string{"init"})
	if err != nil {
		t.Fatalf("init command not found: %v", err)
	}

	catalogFlag := initCmd.Flags().Lookup("catalog")
	if catalogFlag == nil {
		t.Fatal("--catalog flag not registered on init command")
	}

	importCatalogFlag := initCmd.Flags().Lookup("import-catalog")
	if importCatalogFlag != nil {
		t.Error("--import-catalog flag must not be registered on init command (removed)")
	}
}
