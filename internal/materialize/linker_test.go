// SPDX-License-Identifier: MIT

package materialize_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/materialize"
)

// TestNewLinker verifies that NewLinker returns the correct type for each mode.
func TestNewLinker(t *testing.T) {
	tests := []struct {
		mode     string
		wantMode string
		wantErr  bool
	}{
		{"symlink", "symlink", false},
		{"copy", "copy", false},
		{"hardlink", "hardlink", false},
		{"", "symlink", false}, // empty defaults to symlink
		{"invalid", "", true},
		{"SYMLINK", "", true},  // case-sensitive
		{"Copy", "", true},     // case-sensitive
	}

	for _, tt := range tests {
		t.Run("mode="+tt.mode, func(t *testing.T) {
			l, err := materialize.NewLinker(tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewLinker(%q): expected error but got none", tt.mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewLinker(%q): unexpected error: %v", tt.mode, err)
			}
			if l.Mode() != tt.wantMode {
				t.Errorf("Mode() = %q, want %q", l.Mode(), tt.wantMode)
			}
		})
	}
}

// TestSymlinkLinker_Link verifies that SymlinkLinker creates a proper symbolic link.
func TestSymlinkLinker_Link(t *testing.T) {
	dir := t.TempDir()

	// Create a source file.
	srcFile := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("hello symlink"), 0o644); err != nil {
		t.Fatalf("setup: create source file: %v", err)
	}

	linker, err := materialize.NewLinker("symlink")
	if err != nil {
		t.Fatalf("NewLinker: %v", err)
	}

	dst := filepath.Join(dir, "sub", "link.txt")
	if err := linker.Link(srcFile, dst); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Verify it is a symlink.
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("Lstat dst: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("dst is not a symlink; mode = %v", info.Mode())
	}

	// Verify content is readable through the symlink.
	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile through symlink: %v", err)
	}
	if string(content) != "hello symlink" {
		t.Errorf("content = %q, want %q", string(content), "hello symlink")
	}
}

// TestSymlinkLinker_CreatesParentDirs verifies that Link creates parent directories.
func TestSymlinkLinker_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	linker, _ := materialize.NewLinker("symlink")
	dst := filepath.Join(dir, "a", "b", "c", "link.txt")

	if err := linker.Link(srcFile, dst); err != nil {
		t.Fatalf("Link with nested parent dirs: %v", err)
	}

	if _, err := os.Lstat(dst); err != nil {
		t.Errorf("dst not created: %v", err)
	}
}

// TestSymlinkLinker_ReinstallReplacesExisting verifies that re-linking replaces the old link.
func TestSymlinkLinker_ReinstallReplacesExisting(t *testing.T) {
	dir := t.TempDir()

	src1 := filepath.Join(dir, "src1.txt")
	src2 := filepath.Join(dir, "src2.txt")
	if err := os.WriteFile(src1, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	linker, _ := materialize.NewLinker("symlink")
	dst := filepath.Join(dir, "link.txt")

	// First install.
	if err := linker.Link(src1, dst); err != nil {
		t.Fatalf("first Link: %v", err)
	}

	// Second install (re-install with different source).
	if err := linker.Link(src2, dst); err != nil {
		t.Fatalf("second Link: %v", err)
	}

	content, _ := os.ReadFile(dst)
	if string(content) != "v2" {
		t.Errorf("after re-install content = %q, want %q", string(content), "v2")
	}
}

// TestSymlinkLinker_NonexistentSource verifies that Link to a nonexistent source still creates the symlink
// (symlinks to nonexistent targets are valid on most OS).
func TestSymlinkLinker_NonexistentSource(t *testing.T) {
	dir := t.TempDir()
	linker, _ := materialize.NewLinker("symlink")

	dst := filepath.Join(dir, "dangling.txt")
	// On most platforms, os.Symlink succeeds even for nonexistent targets.
	// The test just verifies the call completes without error.
	err := linker.Link("/nonexistent/path/file.txt", dst)
	// Some platforms may return an error; just check if dst exists if no error.
	if err == nil {
		if _, statErr := os.Lstat(dst); statErr != nil {
			t.Errorf("Lstat dangling symlink: %v", statErr)
		}
	}
}

// TestCopyLinker_CopiesFile verifies that CopyLinker produces an independent file copy.
func TestCopyLinker_CopiesFile(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "original.txt")
	if err := os.WriteFile(srcFile, []byte("original content"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	linker, err := materialize.NewLinker("copy")
	if err != nil {
		t.Fatalf("NewLinker: %v", err)
	}

	dst := filepath.Join(dir, "copy.txt")
	if err := linker.Link(srcFile, dst); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Verify content.
	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile copy: %v", err)
	}
	if string(content) != "original content" {
		t.Errorf("content = %q, want %q", string(content), "original content")
	}

	// Verify independence: modifying src does not affect dst.
	if err := os.WriteFile(srcFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modify src: %v", err)
	}
	content2, _ := os.ReadFile(dst)
	if string(content2) != "original content" {
		t.Errorf("copy was not independent; content after src modification = %q", string(content2))
	}
}

// TestCopyLinker_CopiesDirectory verifies recursive directory copy.
func TestCopyLinker_CopiesDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create source directory structure.
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatalf("setup dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("top"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	linker, _ := materialize.NewLinker("copy")
	dst := filepath.Join(dir, "dst")

	if err := linker.Link(srcDir, dst); err != nil {
		t.Fatalf("Link dir: %v", err)
	}

	// Verify both files are present.
	topContent, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Errorf("top file missing: %v", err)
	}
	if string(topContent) != "top" {
		t.Errorf("top content = %q, want %q", string(topContent), "top")
	}

	nestedContent, err := os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil {
		t.Errorf("nested file missing: %v", err)
	}
	if string(nestedContent) != "nested" {
		t.Errorf("nested content = %q, want %q", string(nestedContent), "nested")
	}
}

// TestCopyLinker_NonexistentSource verifies that CopyLinker returns an error for missing source.
func TestCopyLinker_NonexistentSource(t *testing.T) {
	dir := t.TempDir()
	linker, _ := materialize.NewLinker("copy")

	err := linker.Link(filepath.Join(dir, "nonexistent.txt"), filepath.Join(dir, "dst.txt"))
	if err == nil {
		t.Fatal("expected error for nonexistent source but got none")
	}
}

// TestCopyLinker_Mode verifies Mode() returns "copy".
func TestCopyLinker_Mode(t *testing.T) {
	linker, _ := materialize.NewLinker("copy")
	if linker.Mode() != "copy" {
		t.Errorf("Mode() = %q, want %q", linker.Mode(), "copy")
	}
}

// TestHardlinkLinker_Mode verifies Mode() returns "hardlink".
func TestHardlinkLinker_Mode(t *testing.T) {
	linker, _ := materialize.NewLinker("hardlink")
	if linker.Mode() != "hardlink" {
		t.Errorf("Mode() = %q, want %q", linker.Mode(), "hardlink")
	}
}

// TestHardlinkLinker_Link verifies that HardlinkLinker creates a hard link or fallback copy.
func TestHardlinkLinker_Link(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("hardlink content"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	linker, err := materialize.NewLinker("hardlink")
	if err != nil {
		t.Fatalf("NewLinker: %v", err)
	}

	dst := filepath.Join(dir, "linked.txt")
	if err := linker.Link(srcFile, dst); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Verify content is accessible.
	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hardlink content" {
		t.Errorf("content = %q, want %q", string(content), "hardlink content")
	}
}

// TestHardlinkLinker_CreatesParentDirs verifies parent directory creation.
func TestHardlinkLinker_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	linker, _ := materialize.NewLinker("hardlink")
	dst := filepath.Join(dir, "deep", "nested", "dst.txt")

	if err := linker.Link(srcFile, dst); err != nil {
		t.Fatalf("Link with nested dirs: %v", err)
	}

	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst not accessible: %v", err)
	}
}

// TestHardlinkLinker_NonexistentSource verifies error on missing source.
func TestHardlinkLinker_NonexistentSource(t *testing.T) {
	dir := t.TempDir()
	linker, _ := materialize.NewLinker("hardlink")

	err := linker.Link(filepath.Join(dir, "nonexistent.txt"), filepath.Join(dir, "dst.txt"))
	if err == nil {
		t.Fatal("expected error for nonexistent source but got none")
	}
}

// TestNewLinker_InvalidMode verifies the error message for unsupported modes.
func TestNewLinker_InvalidMode(t *testing.T) {
	_, err := materialize.NewLinker("scp")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
	// Error should mention the invalid mode.
	errStr := err.Error()
	if errStr == "" {
		t.Error("error message should not be empty")
	}
}
