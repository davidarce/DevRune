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

// runSyncCmd is a helper that executes the devrune sync command against dir.
func runSyncCmd(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	root := cli.NewRootCmd("test", "abc123")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	fullArgs := append([]string{"--dir", dir, "sync"}, args...)
	root.SetArgs(fullArgs)

	err := root.ExecuteContext(context.Background())
	return buf.String(), err
}

// writeCatalogConfig writes a valid devrune.catalog.yaml to dir with the given sources.
func writeCatalogConfig(t *testing.T, dir string, sources []string) string {
	t.Helper()
	content := "schemaVersion: devrune-catalog/v1\nsources:\n"
	for _, s := range sources {
		content += "  - " + s + "\n"
	}
	path := filepath.Join(dir, "devrune.catalog.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write devrune.catalog.yaml: %v", err)
	}
	return path
}

// TestInit_ImportCatalogFlag_ReadsSpecifiedFile verifies that --import-catalog
// reads the specified catalog config file and includes its sources in the manifest.
// The manifest is written before the resolve step, so we check it even if resolve
// fails (resolve would fail on a fake source ref in tests without network mocking).
func TestInit_ImportCatalogFlag_ReadsSpecifiedFile(t *testing.T) {
	dir := t.TempDir()
	// Write the catalog config to a sub-directory (not the project root) to
	// ensure auto-detection is bypassed and only the flag path is used.
	catalogDir := filepath.Join(dir, "catalogs")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("failed to create catalogs dir: %v", err)
	}
	catalogPath := filepath.Join(catalogDir, "devrune.catalog.yaml")
	content := "schemaVersion: devrune-catalog/v1\nsources:\n  - github:myorg/my-catalog\n"
	if err := os.WriteFile(catalogPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write catalog config: %v", err)
	}

	// Run init with --agents and --import-catalog. We don't check the top-level
	// error here: the manifest is written before resolve/install, so we can verify
	// the manifest content even if the resolve step fails (e.g. network not available
	// in tests). We only fail if the manifest itself was not written at all.
	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--import-catalog", catalogPath,
	)

	// The manifest must have been written (before resolve failure).
	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)
	if !strings.Contains(manifestContent, "myorg/my-catalog") {
		t.Errorf("manifest does not contain catalog source; got:\n%s", manifestContent)
	}
}

// TestInit_AutoDetect_CatalogConfigInWorkingDir verifies that when
// devrune.catalog.yaml is in the working directory, its sources are
// automatically included in the manifest.
func TestInit_AutoDetect_CatalogConfigInWorkingDir(t *testing.T) {
	dir := t.TempDir()
	writeCatalogConfig(t, dir, []string{"github:davidarce/devrune-starter-catalog"})

	_, err := runInitCmd(t, dir, "--agents", "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)
	if !strings.Contains(manifestContent, "devrune-starter-catalog") {
		t.Errorf("manifest does not contain auto-detected catalog source; got:\n%s", manifestContent)
	}
}

// TestInit_MalformedCatalogConfig_WarnsAndContinues verifies that when the
// auto-detected devrune.catalog.yaml is malformed, init warns but does not fail.
func TestInit_MalformedCatalogConfig_WarnsAndContinues(t *testing.T) {
	dir := t.TempDir()
	// Write a catalog config with an unsupported schema version (malformed).
	malformed := "schemaVersion: devrune-catalog/v99\nsources:\n  - github:myorg/repo\n"
	if err := os.WriteFile(filepath.Join(dir, "devrune.catalog.yaml"), []byte(malformed), 0o644); err != nil {
		t.Fatalf("failed to write catalog config: %v", err)
	}

	out, err := runInitCmd(t, dir, "--agents", "claude")
	if err != nil {
		t.Fatalf("expected init to succeed despite malformed catalog config, got error: %v", err)
	}
	if !strings.Contains(out, "warning") {
		t.Errorf("expected a warning in output for malformed catalog config; got:\n%s", out)
	}
}

// TestInit_ImportCatalogFlag_MalformedFile_ReturnsError verifies that when
// --import-catalog points to a malformed file, init returns an error.
func TestInit_ImportCatalogFlag_MalformedFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "bad.catalog.yaml")
	malformed := "schemaVersion: devrune-catalog/v99\nsources:\n  - github:myorg/repo\n"
	if err := os.WriteFile(catalogPath, []byte(malformed), 0o644); err != nil {
		t.Fatalf("failed to write catalog config: %v", err)
	}

	_, err := runInitCmd(t, dir, "--agents", "claude", "--import-catalog", catalogPath)
	if err == nil {
		t.Fatal("expected error for malformed --import-catalog file, got none")
	}
	if !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Errorf("error %q does not contain 'unsupported schemaVersion'", err.Error())
	}
}

// TestInit_ImportCatalogFlag_NotFound_ReturnsError verifies that --import-catalog
// with a path that doesn't exist returns an error.
func TestInit_ImportCatalogFlag_NotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, err := runInitCmd(t, dir, "--agents", "claude", "--import-catalog", filepath.Join(dir, "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing --import-catalog file, got none")
	}
}

// TestInit_CatalogSourcesMerged_WithExplicitSources verifies that catalog sources
// are merged with --source flag values without duplicates.
// The manifest is written before resolve, so we verify content even if resolve fails.
func TestInit_CatalogSourcesMerged_WithExplicitSources(t *testing.T) {
	dir := t.TempDir()
	// Catalog has one source; --source flag provides the same source plus another.
	writeCatalogConfig(t, dir, []string{"github:myorg/catalog-repo"})

	// Ignore top-level error — the manifest is written before the resolve step
	// which will fail for fake source refs in a unit test environment.
	runInitCmd(t, dir, //nolint:errcheck
		"--agents", "claude",
		"--source", "github:myorg/catalog-repo",
		"--source", "github:myorg/extra-repo",
	)

	manifestPath := filepath.Join(dir, "devrune.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	manifestContent := string(data)

	// Both repos should appear, but catalog-repo should appear only once.
	catalogCount := strings.Count(manifestContent, "catalog-repo")
	if catalogCount != 1 {
		t.Errorf("expected catalog-repo to appear exactly once in manifest (deduplication), got %d occurrences; manifest:\n%s",
			catalogCount, manifestContent)
	}
	if !strings.Contains(manifestContent, "extra-repo") {
		t.Errorf("manifest does not contain explicit --source entry; got:\n%s", manifestContent)
	}
}

// TestSync_SuggestionMessage_WhenCatalogExistsWithoutManifest verifies that
// sync prints a helpful message when devrune.catalog.yaml exists but no
// devrune.yaml is present.
func TestSync_SuggestionMessage_WhenCatalogExistsWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	writeCatalogConfig(t, dir, []string{"github:myorg/catalog-repo"})
	// No devrune.yaml written — sync should print the suggestion.

	out, _ := runSyncCmd(t, dir)
	// Sync will fail (no manifest), but the suggestion must appear in output before the error.
	if !strings.Contains(out, "devrune.catalog.yaml") {
		t.Errorf("expected suggestion message mentioning devrune.catalog.yaml in output; got:\n%s", out)
	}
	if !strings.Contains(out, "devrune init") {
		t.Errorf("expected suggestion message to mention `devrune init`; got:\n%s", out)
	}
}

// TestSync_NoSuggestion_WhenManifestExists verifies that sync does NOT print the
// catalog suggestion when devrune.yaml already exists.
func TestSync_NoSuggestion_WhenManifestExists(t *testing.T) {
	dir := t.TempDir()
	writeCatalogConfig(t, dir, []string{"github:myorg/catalog-repo"})
	// Write a minimal (but real) devrune.yaml so sync finds it.
	manifest := "schemaVersion: devrune/v1\nagents: []\npackages: []\nmcps: []\nworkflows: []\n"
	if err := os.WriteFile(filepath.Join(dir, "devrune.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	out, _ := runSyncCmd(t, dir)
	// The suggestion should NOT be printed since the manifest exists.
	if strings.Contains(out, "Found devrune.catalog.yaml but no devrune.yaml") {
		t.Errorf("unexpected suggestion message when manifest exists; got:\n%s", out)
	}
}

// TestInit_FlagRegistered_ImportCatalog verifies the --import-catalog flag is
// registered on the init command.
func TestInit_FlagRegistered_ImportCatalog(t *testing.T) {
	root := cli.NewRootCmd("test", "abc123")
	initCmd, _, err := root.Find([]string{"init"})
	if err != nil {
		t.Fatalf("init command not found: %v", err)
	}
	flag := initCmd.Flags().Lookup("import-catalog")
	if flag == nil {
		t.Fatal("--import-catalog flag not registered on init command")
	}
	if flag.DefValue != "" {
		t.Errorf("--import-catalog default value = %q, want empty string", flag.DefValue)
	}
}
