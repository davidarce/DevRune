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

// ── backup integration helpers ────────────────────────────────────────────────

// listBackupFiles returns the names of all files in the .devrune/backups/ dir.
// Returns an empty slice (no error) when the directory does not exist.
func listBackupFiles(t *testing.T, dir string) []string {
	t.Helper()
	bakDir := filepath.Join(dir, ".devrune", "backups")
	entries, err := os.ReadDir(bakDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("listBackupFiles: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

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

// ── backup integration tests (T014) ──────────────────────────────────────────

// TestInit_Backup_FirstInit_NoBackupCreated verifies that on the very first
// init (no pre-existing devrune.yaml) no backup file is created, because
// CreateBackup is a no-op when the manifest does not yet exist.
func TestInit_Backup_FirstInit_NoBackupCreated(t *testing.T) {
	dir := t.TempDir()

	runInitCmd(t, dir, "--agents", "claude") //nolint:errcheck

	// Manifest must exist.
	if _, err := os.Stat(filepath.Join(dir, "devrune.yaml")); err != nil {
		t.Fatalf("devrune.yaml not written on first init: %v", err)
	}

	// No backup should be created on a first-time init.
	backups := listBackupFiles(t, dir)
	if len(backups) != 0 {
		t.Errorf("expected no backups on first init, got %d: %v", len(backups), backups)
	}
}

// TestInit_Backup_SecondInit_BackupCreatedBeforeWrite verifies that when
// devrune.yaml already exists, running init again creates exactly one backup
// with the PRE-mutation content BEFORE overwriting the manifest.
func TestInit_Backup_SecondInit_BackupCreatedBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	// Seed an existing manifest with a known agent name.
	preMutationContent := "schemaVersion: devrune/v1\nagents:\n  - name: original-agent\n"
	if err := os.WriteFile(manifestPath, []byte(preMutationContent), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	// Run init again, replacing the manifest with a new agent name.
	runInitCmd(t, dir, "--agents", "new-agent", "--force") //nolint:errcheck

	// Exactly one backup should have been created.
	backups := listBackupFiles(t, dir)
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup after second init, got %d: %v", len(backups), backups)
	}

	// The backup must contain the PRE-mutation content (original-agent).
	bakPath := filepath.Join(dir, ".devrune", "backups", backups[0])
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if !strings.Contains(string(bakData), "original-agent") {
		t.Errorf("backup does not contain pre-mutation content; backup content:\n%s", string(bakData))
	}

	// The manifest must now contain the new agent name.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read updated manifest: %v", err)
	}
	if !strings.Contains(string(manifestData), "new-agent") {
		t.Errorf("manifest not updated; content:\n%s", string(manifestData))
	}
}

// TestInit_Backup_FailureAbortsInit verifies that if the backup directory
// cannot be created (permissions removed), init returns an error and
// devrune.yaml is NOT modified.
func TestInit_Backup_FailureAbortsInit(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — chmod restrictions do not apply")
	}

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	// Seed an existing manifest.
	originalContent := "schemaVersion: devrune/v1\nagents:\n  - name: should-not-change\n"
	if err := os.WriteFile(manifestPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	// Create .devrune/ and make it non-writable so CreateBackup cannot
	// create the backups/ subdirectory inside it.
	devruneDir := filepath.Join(dir, ".devrune")
	if err := os.MkdirAll(devruneDir, 0o755); err != nil {
		t.Fatalf("failed to create .devrune/: %v", err)
	}
	if err := os.Chmod(devruneDir, 0o555); err != nil {
		t.Fatalf("failed to chmod .devrune/: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(devruneDir, 0o755) })

	_, err := runInitCmd(t, dir, "--agents", "replaced-agent", "--force")
	if err == nil {
		t.Fatal("expected init to fail when backup cannot be created, but got nil error")
	}

	// devrune.yaml must be unchanged.
	currentData, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("failed to read manifest after failed init: %v", readErr)
	}
	if string(currentData) != originalContent {
		t.Errorf("devrune.yaml was modified despite backup failure;\nwant:\n%s\ngot:\n%s",
			originalContent, string(currentData))
	}
}
