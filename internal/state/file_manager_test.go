package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/state"
)

// TestFileStateManager_ReadWriteRoundTrip verifies state can be written and re-read.
func TestFileStateManager_ReadWriteRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	s := state.State{
		SchemaVersion:   "devrune/state/v1",
		LockHash:        "sha256:abc123",
		ManagedPaths:    []string{"/path/a", "/path/b"},
		ActiveAgents:    []string{"claude", "opencode"},
		ActiveWorkflows: []string{"sdd"},
	}

	if err := mgr.Write(s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.LockHash != s.LockHash {
		t.Errorf("LockHash = %q, want %q", got.LockHash, s.LockHash)
	}
	if got.SchemaVersion != s.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, s.SchemaVersion)
	}
	if len(got.ManagedPaths) != len(s.ManagedPaths) {
		t.Errorf("len(ManagedPaths) = %d, want %d", len(got.ManagedPaths), len(s.ManagedPaths))
	}
	if len(got.ActiveAgents) != len(s.ActiveAgents) {
		t.Errorf("len(ActiveAgents) = %d, want %d", len(got.ActiveAgents), len(s.ActiveAgents))
	}
}

// TestFileStateManager_ReadMissingFile returns zero State when file does not exist.
func TestFileStateManager_ReadMissingFile(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	got, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read on missing file: unexpected error: %v", err)
	}

	// Zero value State expected.
	if got.LockHash != "" {
		t.Errorf("LockHash = %q, want empty", got.LockHash)
	}
	if len(got.ManagedPaths) != 0 {
		t.Errorf("ManagedPaths = %v, want empty slice", got.ManagedPaths)
	}
}

// TestFileStateManager_Write_SetsDefaultSchemaVersion verifies that SchemaVersion
// is auto-populated when missing.
func TestFileStateManager_Write_SetsDefaultSchemaVersion(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	// Write with empty schema version.
	if err := mgr.Write(state.State{LockHash: "sha256:xyz"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, _ := mgr.Read()
	if got.SchemaVersion == "" {
		t.Error("SchemaVersion should be auto-populated but is empty")
	}
	if !strings.Contains(got.SchemaVersion, "devrune") {
		t.Errorf("SchemaVersion = %q, expected to contain 'devrune'", got.SchemaVersion)
	}
}

// TestFileStateManager_Write_SetsInstalledAt verifies InstalledAt is auto-populated.
func TestFileStateManager_Write_SetsInstalledAt(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	if err := mgr.Write(state.State{LockHash: "sha256:xyz"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, _ := mgr.Read()
	if got.InstalledAt == "" {
		t.Error("InstalledAt should be auto-populated but is empty")
	}
}

// TestFileStateManager_Write_CreatesParentDirs verifies that the .devrune directory
// is created automatically.
func TestFileStateManager_Write_CreatesParentDirs(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	if err := mgr.Write(state.State{LockHash: "sha256:test"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	stateFile := filepath.Join(baseDir, ".devrune", "state.yaml")
	if _, err := os.Stat(stateFile); err != nil {
		t.Errorf("state file not created at expected path: %v", err)
	}
}

// TestFileStateManager_ManagedPaths returns the managed paths from saved state.
func TestFileStateManager_ManagedPaths(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	s := state.State{
		LockHash:     "sha256:x",
		ManagedPaths: []string{"/a", "/b", "/c"},
	}
	_ = mgr.Write(s)

	paths, err := mgr.ManagedPaths()
	if err != nil {
		t.Fatalf("ManagedPaths: %v", err)
	}
	if len(paths) != 3 {
		t.Errorf("len(ManagedPaths) = %d, want 3", len(paths))
	}
}

// TestFileStateManager_ManagedPaths_NoState returns empty slice when no state exists.
func TestFileStateManager_ManagedPaths_NoState(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	paths, err := mgr.ManagedPaths()
	if err != nil {
		t.Fatalf("ManagedPaths on empty: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("ManagedPaths on empty = %v, want []", paths)
	}
}

// TestFileStateManager_LockLifecycle verifies acquire → release cycle.
func TestFileStateManager_LockLifecycle(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	if err := mgr.AcquireLock(); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	// Double-acquire should fail.
	mgr2 := state.NewFileStateManager(baseDir)
	if err := mgr2.AcquireLock(); err == nil {
		t.Error("double AcquireLock should fail but succeeded")
		_ = mgr2.ReleaseLock()
	}

	// Release and re-acquire should succeed.
	if err := mgr.ReleaseLock(); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	mgr3 := state.NewFileStateManager(baseDir)
	if err := mgr3.AcquireLock(); err != nil {
		t.Fatalf("re-AcquireLock after release: %v", err)
	}
	_ = mgr3.ReleaseLock()
}

// TestFileStateManager_ReleaseLock_Idempotent verifies that releasing a non-held lock
// does not error.
func TestFileStateManager_ReleaseLock_Idempotent(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	// Release without prior acquire should not panic or error.
	if err := mgr.ReleaseLock(); err != nil {
		t.Errorf("ReleaseLock without acquire: unexpected error: %v", err)
	}
}

// TestFileStateManager_Write_OverwritesPreviousState verifies that Write replaces
// the existing state file.
func TestFileStateManager_Write_OverwritesPreviousState(t *testing.T) {
	baseDir := t.TempDir()
	mgr := state.NewFileStateManager(baseDir)

	// Write first state.
	_ = mgr.Write(state.State{LockHash: "sha256:first", ManagedPaths: []string{"/old"}})

	// Overwrite with second state.
	_ = mgr.Write(state.State{LockHash: "sha256:second", ManagedPaths: []string{"/new1", "/new2"}})

	got, _ := mgr.Read()
	if got.LockHash != "sha256:second" {
		t.Errorf("LockHash = %q, want %q", got.LockHash, "sha256:second")
	}
	if len(got.ManagedPaths) != 2 {
		t.Errorf("len(ManagedPaths) = %d, want 2", len(got.ManagedPaths))
	}
}
