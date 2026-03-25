package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// writeLocalFile is a test helper that creates a file in dir/rel with the given content.
func writeLocalFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", full, err)
	}
}

// extractArchiveFiles reads all regular files from a gzip tar and returns a
// map of path → content. Paths include the full name as stored in the archive.
func extractArchiveFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := make(map[string]string)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("io.ReadAll(%q): %v", hdr.Name, err)
		}
		files[hdr.Name] = string(content)
	}
	return files
}

// TestLocalFetcher_Supports verifies the Supports method.
func TestLocalFetcher_Supports(t *testing.T) {
	f := NewLocalFetcher()

	tests := []struct {
		scheme model.Scheme
		want   bool
	}{
		{model.SchemeLocal, true},
		{model.SchemeGitHub, false},
		{model.SchemeGitLab, false},
	}

	for _, tt := range tests {
		got := f.Supports(tt.scheme)
		if got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.scheme, got, tt.want)
		}
	}
}

// TestLocalFetcher_FetchDirectory verifies that Fetch produces a valid tar archive.
func TestLocalFetcher_FetchDirectory(t *testing.T) {
	srcDir := t.TempDir()

	writeLocalFile(t, srcDir, "skills/git-commit/SKILL.md", "# git-commit skill")
	writeLocalFile(t, srcDir, "rules/arch/clean.md", "# clean arch rule")

	f := NewLocalFetcher()
	ref := model.SourceRef{
		Scheme: model.SchemeLocal,
		Path:   srcDir,
	}

	data, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Fetch() returned empty bytes")
	}

	// Verify it is a valid gzip stream.
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not a valid gzip stream: %v", err)
	}
	gr.Close()
}

// TestLocalFetcher_FetchedArchiveContainsExpectedFiles verifies that the
// fetched archive contains the expected files with correct content.
func TestLocalFetcher_FetchedArchiveContainsExpectedFiles(t *testing.T) {
	srcDir := t.TempDir()

	writeLocalFile(t, srcDir, "skills/git-commit/SKILL.md", "skill content")
	writeLocalFile(t, srcDir, "rules/arch/clean.md", "rule content")

	f := NewLocalFetcher()
	ref := model.SourceRef{Scheme: model.SchemeLocal, Path: srcDir}

	data, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	files := extractArchiveFiles(t, data)

	// Find each expected file path (ignoring the exact prefix).
	findByName := func(name string) (string, bool) {
		for path, content := range files {
			if strings.HasSuffix(filepath.ToSlash(path), name) {
				return content, true
			}
		}
		return "", false
	}

	skillContent, ok := findByName("skills/git-commit/SKILL.md")
	if !ok {
		t.Error("archive missing skills/git-commit/SKILL.md")
	} else if skillContent != "skill content" {
		t.Errorf("SKILL.md content = %q, want %q", skillContent, "skill content")
	}

	ruleContent, ok := findByName("rules/arch/clean.md")
	if !ok {
		t.Error("archive missing rules/arch/clean.md")
	} else if ruleContent != "rule content" {
		t.Errorf("rule content = %q, want %q", ruleContent, "rule content")
	}
}

// TestLocalFetcher_FetchNonexistentPath verifies that fetching a missing dir returns an error.
func TestLocalFetcher_FetchNonexistentPath(t *testing.T) {
	f := NewLocalFetcher()
	ref := model.SourceRef{
		Scheme: model.SchemeLocal,
		Path:   "/path/that/does/not/exist",
	}

	_, err := f.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("Fetch() expected error for nonexistent path, got nil")
	}
}

// TestLocalFetcher_FetchSingleFile verifies that fetching a single file produces a valid tar
// with the file stored as "local/{filename}".
func TestLocalFetcher_FetchSingleFile(t *testing.T) {
	srcDir := t.TempDir()
	filePath := filepath.Join(srcDir, "engram.yaml")
	fileContent := "name: engram\ncommand: engram-server"
	if err := os.WriteFile(filePath, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f := NewLocalFetcher()
	ref := model.SourceRef{Scheme: model.SchemeLocal, Path: filePath}

	data, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Fetch() returned empty bytes")
	}

	files := extractArchiveFiles(t, data)

	// Should contain exactly one file: "local/engram.yaml"
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1; files: %v", len(files), files)
	}

	content, ok := files["local/engram.yaml"]
	if !ok {
		t.Errorf("archive missing local/engram.yaml; got keys: %v", files)
	} else if content != fileContent {
		t.Errorf("file content = %q, want %q", content, fileContent)
	}
}

// TestLocalFetcher_WrongScheme verifies that fetching with a non-local scheme returns an error.
func TestLocalFetcher_WrongScheme(t *testing.T) {
	f := NewLocalFetcher()
	ref := model.SourceRef{
		Scheme: model.SchemeGitHub,
		Owner:  "owner",
		Repo:   "repo",
		Ref:    "main",
	}

	_, err := f.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("Fetch() expected error for non-local scheme, got nil")
	}
}

// TestLocalFetcher_HiddenFilesExcluded verifies that hidden files are not included in the archive.
func TestLocalFetcher_HiddenFilesExcluded(t *testing.T) {
	srcDir := t.TempDir()

	writeLocalFile(t, srcDir, "visible.md", "visible content")
	writeLocalFile(t, srcDir, ".hidden.md", "hidden content")
	writeLocalFile(t, srcDir, ".git/config", "git config")

	f := NewLocalFetcher()
	ref := model.SourceRef{Scheme: model.SchemeLocal, Path: srcDir}

	data, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	files := extractArchiveFiles(t, data)

	for path := range files {
		// No hidden file should appear in the archive.
		for _, component := range strings.Split(filepath.ToSlash(path), "/") {
			if strings.HasPrefix(component, ".") {
				t.Errorf("archive contains hidden path %q", path)
			}
		}
	}
}

// TestLocalFetcher_EmptyDirectory verifies that fetching an empty dir returns valid (empty) archive.
func TestLocalFetcher_EmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()

	f := NewLocalFetcher()
	ref := model.SourceRef{Scheme: model.SchemeLocal, Path: srcDir}

	data, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Should still be a valid gzip/tar stream.
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not valid gzip: %v", err)
	}
	gr.Close()

	// No regular files.
	files := extractArchiveFiles(t, data)
	if len(files) != 0 {
		t.Errorf("got %d files in empty-dir archive, want 0", len(files))
	}
}

// TestLocalFetcher_EmptyPath verifies that an empty Path field causes an error.
func TestLocalFetcher_EmptyPath(t *testing.T) {
	f := NewLocalFetcher()
	ref := model.SourceRef{Scheme: model.SchemeLocal, Path: ""}

	_, err := f.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("Fetch() expected error for empty path, got nil")
	}
}
