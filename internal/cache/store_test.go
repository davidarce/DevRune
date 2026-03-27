// SPDX-License-Identifier: MIT

package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestArchive creates a gzip-compressed tar archive for testing.
// Files is a map of archive-relative path (with prefix component) → content.
func buildTestArchive(t *testing.T, prefix string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write prefix directory entry.
	if prefix != "" {
		hdr := &tar.Header{
			Typeflag: tar.TypeDir,
			Name:     prefix + "/",
			Mode:     0o755,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("buildTestArchive: write dir header: %v", err)
		}
	}

	for path, content := range files {
		fullPath := path
		if prefix != "" {
			fullPath = prefix + "/" + path
		}

		// Create parent directory entries if needed.
		dir := filepath.ToSlash(filepath.Dir(fullPath))
		if dir != "." && dir != "/" && dir != prefix {
			dirHdr := &tar.Header{
				Typeflag: tar.TypeDir,
				Name:     dir + "/",
				Mode:     0o755,
			}
			_ = tw.WriteHeader(dirHdr) // ignore duplicate dir errors
		}

		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     fullPath,
			Size:     int64(len(content)),
			Mode:     0o644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("buildTestArchive: write header for %q: %v", path, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("buildTestArchive: write content for %q: %v", path, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("buildTestArchive: close tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("buildTestArchive: close gzip: %v", err)
	}
	return buf.Bytes()
}

// computeTestHash computes "sha256:<hex>" for the given bytes.
func computeTestHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}

// TestFileCacheStore_StoreAndHas verifies basic Store + Has lifecycle.
func TestFileCacheStore_StoreAndHas(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	archive := buildTestArchive(t, "pkg-v1", map[string]string{
		"skills/git-commit/SKILL.md": "# git-commit",
	})

	// Has returns false before store.
	hash := computeTestHash(archive)
	if store.Has(hash) {
		t.Error("Has() returned true before Store()")
	}

	// Store the archive.
	dir, err := store.Store("test-key", archive)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if dir == "" {
		t.Fatal("Store() returned empty dir")
	}

	// Has returns true after store.
	if !store.Has(hash) {
		t.Error("Has() returned false after Store()")
	}

	// Dir must exist on disk.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("Store() dir %q is not a directory", dir)
	}
}

// TestFileCacheStore_Get verifies Get returns the correct path.
func TestFileCacheStore_Get(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	archive := buildTestArchive(t, "pkg-v1", map[string]string{
		"rules/arch/clean.md": "# clean arch",
	})

	// Get returns ("", false) for a missing hash.
	hash := computeTestHash(archive)
	dir, ok := store.Get(hash)
	if ok {
		t.Errorf("Get() returned ok=true for uncached hash, dir=%q", dir)
	}
	if dir != "" {
		t.Errorf("Get() returned non-empty dir for uncached hash: %q", dir)
	}

	// Store, then Get returns correct dir.
	storedDir, err := store.Store("test-key", archive)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	gotDir, ok := store.Get(hash)
	if !ok {
		t.Fatal("Get() returned ok=false after Store()")
	}
	if gotDir != storedDir {
		t.Errorf("Get() = %q, Store() returned %q", gotDir, storedDir)
	}
}

// TestFileCacheStore_StoreIdempotent verifies that storing the same content twice is idempotent.
func TestFileCacheStore_StoreIdempotent(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	archive := buildTestArchive(t, "pkg-v1", map[string]string{
		"skills/my-skill/SKILL.md": "# my skill",
	})

	dir1, err := store.Store("key", archive)
	if err != nil {
		t.Fatalf("first Store() error = %v", err)
	}

	dir2, err := store.Store("key", archive)
	if err != nil {
		t.Fatalf("second Store() error = %v", err)
	}

	if dir1 != dir2 {
		t.Errorf("idempotent Store() returned different dirs: %q vs %q", dir1, dir2)
	}
}

// TestFileCacheStore_ExtractedContents verifies that the extracted directory
// contains the expected files after stripping the first path component.
func TestFileCacheStore_ExtractedContents(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	archive := buildTestArchive(t, "pkg-prefix", map[string]string{
		"skills/git-commit/SKILL.md": "skill content here",
	})

	dir, err := store.Store("test-key", archive)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// After stripping the "pkg-prefix/" component, SKILL.md should be at:
	// dir/skills/git-commit/SKILL.md
	expectedFile := filepath.Join(dir, "skills", "git-commit", "SKILL.md")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedFile, err)
	}
	if string(data) != "skill content here" {
		t.Errorf("file content = %q, want %q", string(data), "skill content here")
	}
}

// TestFileCacheStore_HashHexDirectoryName verifies that the cache directory is named
// by the SHA256 hex (without the "sha256:" prefix).
func TestFileCacheStore_HashHexDirectoryName(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	archive := buildTestArchive(t, "p", map[string]string{"file.txt": "hello"})
	dir, err := store.Store("key", archive)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	hash := computeTestHash(archive)
	hexPart := strings.TrimPrefix(hash, "sha256:")

	expectedDir := filepath.Join(baseDir, hexPart)
	if dir != expectedDir {
		t.Errorf("Store() dir = %q, want %q", dir, expectedDir)
	}
}

// TestFileCacheStore_MissingHash verifies that Get on a completely missing hash returns false.
func TestFileCacheStore_MissingHash(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	fakeHash := "sha256:" + strings.Repeat("0", 64)
	dir, ok := store.Get(fakeHash)
	if ok {
		t.Errorf("Get(unknown hash) returned ok=true, dir=%q", dir)
	}
}

// TestFileCacheStore_InvalidHashFormat verifies that Get with a malformed hash returns false.
func TestFileCacheStore_InvalidHashFormat(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileCacheStore(baseDir)

	tests := []struct {
		name string
		hash string
	}{
		{name: "no prefix", hash: "abc123"},
		{name: "empty string", hash: ""},
		{name: "prefix only", hash: "sha256:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, ok := store.Get(tt.hash)
			if ok {
				t.Errorf("Get(%q) returned ok=true, dir=%q", tt.hash, dir)
			}
		})
	}
}
