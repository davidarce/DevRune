// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ── syncCatalogs backup integration tests (T014) ──────────────────────────────

// listSyncBackupFiles returns the base names of all files in .devrune/backups/
// for the given project directory. Returns nil when the directory does not exist.
func listSyncBackupFiles(t *testing.T, dir string) []string {
	t.Helper()
	bakDir := filepath.Join(dir, ".devrune", "backups")
	entries, err := os.ReadDir(bakDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("listSyncBackupFiles: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestSyncCatalogs_Backup_CreatedBeforeWrite verifies that when syncCatalogs
// must update the manifest (missing catalog root), it creates a backup
// containing the PRE-mutation content before overwriting devrune.yaml.
func TestSyncCatalogs_Backup_CreatedBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	// A manifest that references a github package. deriveCatalogRoots will
	// derive "github:myorg/my-catalog" as the catalog root. Since catalogs: is
	// empty, syncCatalogs will attempt to write it back to disk.
	preMutationContent := `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:myorg/my-catalog//skills/foo
`
	if err := os.WriteFile(manifestPath, []byte(preMutationContent), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	// Parse and call syncCatalogs directly.
	manifest := aMinimalManifestWithPackage("github:myorg/my-catalog//skills/foo")

	if err := syncCatalogs(manifest, manifestPath); err != nil {
		t.Fatalf("syncCatalogs returned unexpected error: %v", err)
	}

	// Exactly one backup file must exist.
	backups := listSyncBackupFiles(t, dir)
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup after syncCatalogs, got %d: %v", len(backups), backups)
	}

	// Backup must contain the PRE-mutation content.
	bakPath := filepath.Join(dir, ".devrune", "backups", backups[0])
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if !strings.Contains(string(bakData), "github:myorg/my-catalog//skills/foo") {
		t.Errorf("backup does not contain pre-mutation content;\nbackup:\n%s", string(bakData))
	}
	if strings.Contains(string(bakData), "catalogs:") {
		t.Errorf("backup should NOT already have 'catalogs:' key (it must be the pre-mutation state);\nbackup:\n%s", string(bakData))
	}

	// devrune.yaml must now contain the derived catalog root.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest after syncCatalogs: %v", err)
	}
	if !strings.Contains(string(manifestData), "catalogs:") {
		t.Errorf("manifest missing 'catalogs:' key after syncCatalogs;\ncontent:\n%s", string(manifestData))
	}
}

// TestSyncCatalogs_Backup_FailureAbortsWrite verifies that when the backup
// cannot be created (non-writable .devrune/ dir), syncCatalogs returns an
// error and devrune.yaml is NOT modified.
func TestSyncCatalogs_Backup_FailureAbortsWrite(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — chmod restrictions do not apply")
	}

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	originalContent := `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:myorg/my-catalog//skills/foo
`
	if err := os.WriteFile(manifestPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	// Make .devrune/ non-writable so CreateBackup cannot create backups/.
	devruneDir := filepath.Join(dir, ".devrune")
	if err := os.MkdirAll(devruneDir, 0o755); err != nil {
		t.Fatalf("failed to create .devrune/: %v", err)
	}
	if err := os.Chmod(devruneDir, 0o555); err != nil {
		t.Fatalf("failed to chmod .devrune/: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(devruneDir, 0o755) })

	manifest := aMinimalManifestWithPackage("github:myorg/my-catalog//skills/foo")

	err := syncCatalogs(manifest, manifestPath)
	if err == nil {
		t.Fatal("expected syncCatalogs to fail when backup cannot be created, but got nil error")
	}

	// devrune.yaml must be unchanged.
	currentData, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("failed to read manifest after failed syncCatalogs: %v", readErr)
	}
	if string(currentData) != originalContent {
		t.Errorf("devrune.yaml was modified despite backup failure;\nwant:\n%s\ngot:\n%s",
			originalContent, string(currentData))
	}
}

// TestSyncCatalogs_NoChange_NoBackupCreated verifies that when all catalog
// roots are already present in the manifest, syncCatalogs returns nil without
// writing any backup.
func TestSyncCatalogs_NoChange_NoBackupCreated(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	// Manifest already has the catalog root, so no write needed.
	content := `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:myorg/my-catalog//skills/foo
catalogs:
  - github:myorg/my-catalog
`
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	// Build a manifest that already has the catalog in it.
	manifestWithCatalog := aMinimalManifestWithPackageAndCatalog(
		"github:myorg/my-catalog//skills/foo",
		"github:myorg/my-catalog",
	)

	if err := syncCatalogs(manifestWithCatalog, manifestPath); err != nil {
		t.Fatalf("syncCatalogs returned error when no change needed: %v", err)
	}

	// No backup should be created when syncCatalogs does nothing.
	backups := listSyncBackupFiles(t, dir)
	if len(backups) != 0 {
		t.Errorf("expected no backups when no catalog change needed, got %d: %v",
			len(backups), backups)
	}
}

// TestSyncCatalogs_WriteFileAtomic_ProducesExpectedContent verifies that
// WriteFileAtomic (used by syncCatalogs) produces the correct final content
// and leaves no .tmp file behind.
func TestSyncCatalogs_WriteFileAtomic_ProducesExpectedContent(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "devrune.yaml")

	preMutation := `schemaVersion: devrune/v1
agents:
  - name: claude
packages:
  - source: github:myorg/my-catalog//skills/bar
`
	if err := os.WriteFile(manifestPath, []byte(preMutation), 0o644); err != nil {
		t.Fatalf("failed to seed manifest: %v", err)
	}

	manifest := aMinimalManifestWithPackage("github:myorg/my-catalog//skills/bar")

	if err := syncCatalogs(manifest, manifestPath); err != nil {
		t.Fatalf("syncCatalogs returned error: %v", err)
	}

	// No .tmp file should remain.
	tmpPath := manifestPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf(".tmp file still exists after WriteFileAtomic: %s", tmpPath)
	}

	// Final manifest must include the catalog root.
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if !strings.Contains(string(data), "github:myorg/my-catalog") {
		t.Errorf("manifest does not contain expected catalog root;\ncontent:\n%s", string(data))
	}
}

// ── sync manifest builder helpers ────────────────────────────────────────────

// aMinimalManifestWithPackage builds a minimal UserManifest with one package
// and no catalogs, suitable for triggering syncCatalogs to add a catalog root.
func aMinimalManifestWithPackage(source string) model.UserManifest {
	return model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages:      []model.PackageRef{{Source: source}},
	}
}

// aMinimalManifestWithPackageAndCatalog builds a manifest that already
// contains both the package and its catalog root, so syncCatalogs is a no-op.
func aMinimalManifestWithPackageAndCatalog(source, catalog string) model.UserManifest {
	return model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages:      []model.PackageRef{{Source: source}},
		Catalogs:      []string{catalog},
	}
}
