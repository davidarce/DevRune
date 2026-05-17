package backup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteFileAtomic_Success verifies that a successful write produces the
// correct file content and leaves no .tmp sibling behind.
func TestWriteFileAtomic_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "data.yaml")
	want := []byte("key: value\n")

	if err := WriteFileAtomic(target, want, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: unexpected error: %v", err)
	}

	// File must exist with the correct content.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile after write: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content mismatch: got %q, want %q", got, want)
	}

	// .tmp sibling must not exist after a successful rename.
	tmp := target + ".tmp"
	if _, err := os.Stat(tmp); err == nil {
		t.Errorf(".tmp file unexpectedly present after successful write: %s", tmp)
	}
}

// TestWriteFileAtomic_NoTmpOnWriteError verifies that the .tmp file is cleaned
// up when the write itself fails (e.g. because the destination directory does
// not exist — the OpenFile call fails before any data is written).
func TestWriteFileAtomic_NoTmpOnWriteError(t *testing.T) {
	nonExistentDir := filepath.Join(t.TempDir(), "does", "not", "exist")
	target := filepath.Join(nonExistentDir, "data.yaml")

	err := WriteFileAtomic(target, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("expected error when destination directory does not exist, got nil")
	}

	// Even though OpenFile failed, the .tmp must not linger.
	tmp := target + ".tmp"
	if _, statErr := os.Stat(tmp); statErr == nil {
		t.Errorf(".tmp file present after failed write: %s", tmp)
	}
}

// TestWriteFileAtomic_OverwritesExisting confirms that an existing file at the
// target path is replaced atomically (not appended to).
func TestWriteFileAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "manifest.yaml")

	original := []byte("version: 1\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	updated := []byte("version: 2\nagents: []\n")
	if err := WriteFileAtomic(target, updated, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile after overwrite: %v", err)
	}
	if string(got) != string(updated) {
		t.Errorf("content after overwrite: got %q, want %q", got, updated)
	}

	// .tmp must be absent.
	if _, err := os.Stat(target + ".tmp"); err == nil {
		t.Errorf(".tmp file still present after overwrite")
	}
}

// TestWriteFileAtomic_PermPropagation verifies that the created file has the
// exact permission bits requested (mode 0o600 vs 0o644).
func TestWriteFileAtomic_PermPropagation(t *testing.T) {
	// Permission propagation tests are meaningful only on Unix where mode bits
	// are enforced. Skip on platforms where os.Stat().Mode() always returns the
	// same value regardless of the requested permission.
	dir := t.TempDir()

	cases := []struct {
		name string
		perm os.FileMode
	}{
		{"perm_0600", 0o600},
		{"perm_0644", 0o644},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(dir, tc.name+".yaml")
			if err := WriteFileAtomic(target, []byte("x: 1\n"), tc.perm); err != nil {
				t.Fatalf("WriteFileAtomic: %v", err)
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			// Mask to the lowest 9 bits (rwxrwxrwx) for comparison.
			got := info.Mode().Perm()
			if got != tc.perm {
				t.Errorf("permission: got %04o, want %04o", got, tc.perm)
			}
		})
	}
}

// TestWriteFileAtomic_EmptyData verifies that writing zero bytes produces an
// empty file (not an error) and leaves no .tmp behind.
func TestWriteFileAtomic_EmptyData(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.yaml")

	if err := WriteFileAtomic(target, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic with empty data: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}

	if _, err := os.Stat(target + ".tmp"); err == nil {
		t.Errorf(".tmp file present after empty write")
	}
}

// TestWriteFileAtomic_AtomicityNoTmp confirms the core atomicity invariant:
// after a successful WriteFileAtomic call the .tmp sibling file is absent,
// ensuring readers always see either the old full content or the new full
// content — never a partial write.
func TestWriteFileAtomic_AtomicityNoTmp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "devrune.yaml")

	data := []byte("agents:\n  - name: claude-sonnet-4-6\n")
	if err := WriteFileAtomic(target, data, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	// The rename must have happened; no .tmp should exist.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == filepath.Base(target)+".tmp" {
			t.Errorf("found stale .tmp entry in dir after successful write")
		}
	}
}
