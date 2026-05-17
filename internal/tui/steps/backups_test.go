// SPDX-License-Identifier: MIT

package steps

// Tests for executeRestore — the pure restore logic extracted for testability.
//
// These tests cover T015 (restore → install reentrancy integration).
// executeRestore lives in this package so the tests are co-located here.
// They inject installFn stubs so no external services, real TUI, or real
// install pipeline is required.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidarce/devrune/internal/backup"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// writeManifest creates devrune.yaml in dir with the given content.
func writeManifest(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "devrune.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	return p
}

// makeBackupEntry creates a real backup file in .devrune/backups/ and returns
// the corresponding BackupEntry. Useful to set up a "restorable" snapshot.
// The entry timestamp is set 10 seconds in the past so that when executeRestore
// calls CreateBackup (which uses time.Now()), the pre-restore backup gets a
// newer timestamp and never collides with or overwrites this entry's file.
func makeBackupEntry(t *testing.T, projectDir, content string) backup.BackupEntry {
	t.Helper()
	bakDir := filepath.Join(projectDir, ".devrune", "backups")
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		t.Fatalf("makeBackupEntry mkdir: %v", err)
	}
	// Use a past timestamp to avoid collision with the pre-restore backup that
	// executeRestore will create with time.Now().
	entryTime := time.Now().UTC().Add(-10 * time.Second)
	ts := entryTime.Format(time.RFC3339)
	ts = strings.ReplaceAll(ts, ":", "-")
	name := ts
	path := filepath.Join(bakDir, "devrune.yaml."+ts)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("makeBackupEntry write: %v", err)
	}
	return backup.BackupEntry{
		Path:      path,
		Name:      name,
		Timestamp: entryTime,
	}
}

// listBackupNames returns the base names of all backup files in .devrune/backups/.
func listBackupNames(t *testing.T, projectDir string) []string {
	t.Helper()
	bakDir := filepath.Join(projectDir, ".devrune", "backups")
	entries, err := os.ReadDir(bakDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("listBackupNames ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// readManifest reads and returns the current content of devrune.yaml.
func readManifest(t *testing.T, manifestPath string) string {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	return string(data)
}

// ── T015 tests ────────────────────────────────────────────────────────────────

// TestExecuteRestore_Success verifies the happy path:
//  1. A pre-restore backup of the current state is created.
//  2. devrune.yaml is replaced atomically with the backup content.
//  3. installFn is called exactly once.
//  4. After the operation, devrune.yaml contains the backup content.
func TestExecuteRestore_Success(t *testing.T) {
	dir := t.TempDir()
	currentContent := "version: 1\nagents: []\n"
	backupContent := "version: 1\nagents:\n  - name: claude\n"

	manifestPath := writeManifest(t, dir, currentContent)
	entry := makeBackupEntry(t, dir, backupContent)

	installCalled := 0
	installFn := func() error {
		installCalled++
		return nil
	}

	if err := executeRestore(dir, manifestPath, entry, installFn); err != nil {
		t.Fatalf("executeRestore returned unexpected error: %v", err)
	}

	// installFn must have been called exactly once.
	if installCalled != 1 {
		t.Errorf("installFn called %d times, want 1", installCalled)
	}

	// devrune.yaml must contain the backup content (not the original).
	got := readManifest(t, manifestPath)
	if got != backupContent {
		t.Errorf("devrune.yaml content after restore:\ngot:  %q\nwant: %q", got, backupContent)
	}

	// A pre-restore backup must have been created (in addition to the one we
	// pre-seeded in makeBackupEntry).
	names := listBackupNames(t, dir)
	if len(names) < 2 {
		t.Errorf("expected at least 2 backup files (pre-seeded + pre-restore), got %d: %v", len(names), names)
	}
}

// TestExecuteRestore_InstallFails verifies that when installFn returns an error:
//  1. devrune.yaml is left with the RESTORED content (no auto-revert, per Q4).
//  2. The pre-restore backup is still present on disk.
//  3. The error from installFn is propagated to the caller.
func TestExecuteRestore_InstallFails(t *testing.T) {
	dir := t.TempDir()
	currentContent := "version: 1\nagents: []\n"
	backupContent := "version: 1\nagents:\n  - name: claude\n"

	manifestPath := writeManifest(t, dir, currentContent)
	entry := makeBackupEntry(t, dir, backupContent)

	installErr := errors.New("cannot resolve package: not found")
	installFn := func() error { return installErr }

	err := executeRestore(dir, manifestPath, entry, installFn)
	if err == nil {
		t.Fatal("executeRestore should have returned an error when installFn fails")
	}
	if !strings.Contains(err.Error(), installErr.Error()) {
		t.Errorf("error message %q should contain %q", err.Error(), installErr.Error())
	}

	// devrune.yaml must contain the RESTORED content (not the original) — no auto-revert.
	got := readManifest(t, manifestPath)
	if got != backupContent {
		t.Errorf("devrune.yaml should contain restored content after install failure (no revert)\ngot:  %q\nwant: %q", got, backupContent)
	}

	// Pre-restore backup must exist.
	names := listBackupNames(t, dir)
	if len(names) < 2 {
		t.Errorf("expected at least 2 backup files (pre-seeded + pre-restore), got %d: %v", len(names), names)
	}
}

// TestExecuteRestore_FIFOBudget verifies that when 5 backups already exist,
// adding the pre-restore backup via executeRestore counts toward the 5-file
// limit and rotates out the oldest entry (net result: still 5 files).
func TestExecuteRestore_FIFOBudget(t *testing.T) {
	dir := t.TempDir()
	manifestContent := "version: 1\nagents: []\n"
	writeManifest(t, dir, manifestContent)

	// Pre-seed MaxBackups (5) backup files with distinct, ordered timestamps.
	// We space them 2 seconds apart to guarantee lexicographic ordering.
	type seeded struct {
		name    string
		content string
	}
	seeded5 := make([]seeded, backup.MaxBackups)
	bakDir := filepath.Join(dir, ".devrune", "backups")
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < backup.MaxBackups; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		ts = strings.ReplaceAll(ts, ":", "-")
		name := "devrune.yaml." + ts
		content := fmt.Sprintf("# backup %d\n", i)
		path := filepath.Join(bakDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("seed backup %d: %v", i, err)
		}
		seeded5[i] = seeded{name: name, content: content}
	}

	// The oldest file is seeded5[0].
	oldestName := seeded5[0].name

	// Use the most recent seeded backup as the entry to restore.
	restoreContent := seeded5[backup.MaxBackups-1].content
	restoreEntry := backup.BackupEntry{
		Path:      filepath.Join(bakDir, seeded5[backup.MaxBackups-1].name),
		Name:      seeded5[backup.MaxBackups-1].name,
		Timestamp: base.Add(time.Duration(backup.MaxBackups-1) * time.Second),
	}

	// Verify we have exactly MaxBackups before the restore.
	before := listBackupNames(t, dir)
	if len(before) != backup.MaxBackups {
		t.Fatalf("expected %d backups before restore, got %d: %v", backup.MaxBackups, len(before), before)
	}

	// Run restore; installFn is a no-op stub.
	if err := executeRestore(dir, filepath.Join(dir, "devrune.yaml"), restoreEntry, func() error { return nil }); err != nil {
		t.Fatalf("executeRestore: %v", err)
	}

	// After the pre-restore backup is created + rotated, there should still
	// be exactly MaxBackups files (the oldest was evicted).
	after := listBackupNames(t, dir)
	if len(after) != backup.MaxBackups {
		t.Errorf("expected %d backups after restore (FIFO rotation), got %d: %v", backup.MaxBackups, len(after), after)
	}

	// The oldest pre-seeded backup must have been rotated out.
	for _, n := range after {
		if n == oldestName {
			t.Errorf("oldest backup %q should have been rotated out, but it still exists: %v", oldestName, after)
			break
		}
	}

	// devrune.yaml must contain the restored content.
	got := readManifest(t, filepath.Join(dir, "devrune.yaml"))
	if got != restoreContent {
		t.Errorf("devrune.yaml content mismatch after FIFO test\ngot:  %q\nwant: %q", got, restoreContent)
	}
}

// TestExecuteRestore_PreRestoreBackupContentMatchesCurrent verifies that the
// pre-restore backup created by executeRestore contains the exact bytes that
// were in devrune.yaml before the restore (not the restored content).
func TestExecuteRestore_PreRestoreBackupContentMatchesCurrent(t *testing.T) {
	dir := t.TempDir()
	currentContent := "version: 1\nagents: []\n# current state\n"
	backupContent := "version: 1\nagents:\n  - name: claude\n"

	manifestPath := writeManifest(t, dir, currentContent)
	entry := makeBackupEntry(t, dir, backupContent)

	if err := executeRestore(dir, manifestPath, entry, func() error { return nil }); err != nil {
		t.Fatalf("executeRestore: %v", err)
	}

	// Find the backup that was created by executeRestore (the pre-restore one).
	// The entry we pre-seeded has a known path; the pre-restore backup is the
	// *other* file in .devrune/backups/.
	names := listBackupNames(t, dir)
	var preRestoreContent []byte
	for _, n := range names {
		p := filepath.Join(dir, ".devrune", "backups", n)
		if p == entry.Path {
			continue // skip the pre-seeded backup
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read pre-restore backup %s: %v", p, err)
		}
		preRestoreContent = data
		break
	}

	if preRestoreContent == nil {
		t.Fatalf("no pre-restore backup found; backup files: %v", names)
	}
	if string(preRestoreContent) != currentContent {
		t.Errorf("pre-restore backup content mismatch\ngot:  %q\nwant: %q", string(preRestoreContent), currentContent)
	}
}
