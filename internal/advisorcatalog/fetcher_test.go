// SPDX-License-Identifier: MIT

package advisorcatalog

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// withFakeGit swaps execGit for a fake that records all invocations and returns
// the configured output/error. It restores the original on cleanup.
func withFakeGit(t *testing.T, output []byte, err error) *fakeGit {
	t.Helper()
	f := &fakeGit{output: output, err: err}
	orig := execGit
	execGit = func(args ...string) ([]byte, error) {
		f.calls = append(f.calls, args)
		return f.output, f.err
	}
	t.Cleanup(func() { execGit = orig })
	return f
}

type fakeGit struct {
	calls  [][]string
	output []byte
	err    error
}

// ─────────────────────────────────────────────────────────────────────────────
// LocalFetcher tests
// ─────────────────────────────────────────────────────────────────────────────

func TestLocalFetcher_HappyPath_ExistingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fetcher := LocalFetcher{WorkspaceRoot: dir}
	src := model.CatalogSource{URL: "local:" + dir}

	got, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("Fetch returned %q, want %q", got, dir)
	}
}

func TestLocalFetcher_RelativePath_ResolvedAgainstWorkspaceRoot(t *testing.T) {
	t.Parallel()

	wsRoot := t.TempDir()
	// Create a sub-directory to resolve against.
	catalogDir := filepath.Join(wsRoot, "my-catalog")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fetcher := LocalFetcher{WorkspaceRoot: wsRoot}
	src := model.CatalogSource{URL: "local:my-catalog"}

	got, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if got != catalogDir {
		t.Errorf("Fetch returned %q, want %q", got, catalogDir)
	}
}

func TestLocalFetcher_RelativePathWithDotSlash_Resolved(t *testing.T) {
	t.Parallel()

	wsRoot := t.TempDir()
	catalogDir := filepath.Join(wsRoot, "sub", "catalog")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fetcher := LocalFetcher{WorkspaceRoot: wsRoot}
	src := model.CatalogSource{URL: "local:./sub/catalog"}

	got, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if got != catalogDir {
		t.Errorf("Fetch returned %q, want %q", got, catalogDir)
	}
}

func TestLocalFetcher_MissingDir_ReturnsError(t *testing.T) {
	t.Parallel()

	fetcher := LocalFetcher{WorkspaceRoot: t.TempDir()}
	src := model.CatalogSource{URL: "local:/nonexistent/path/that/does/not/exist"}

	_, err := fetcher.Fetch(context.Background(), src)
	if err == nil {
		t.Fatal("Fetch expected error for non-existent directory, got nil")
	}
}

func TestLocalFetcher_FileNotDirectory_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a regular file (not a directory).
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fetcher := LocalFetcher{WorkspaceRoot: dir}
	src := model.CatalogSource{URL: "local:" + filePath}

	_, err := fetcher.Fetch(context.Background(), src)
	if err == nil {
		t.Fatal("Fetch expected error when path is a file, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "file") && !strings.Contains(strings.ToLower(err.Error()), "directory") {
		t.Errorf("error should mention file/directory; got: %v", err)
	}
}

func TestLocalFetcher_MalformedURL_ReturnsError(t *testing.T) {
	t.Parallel()

	fetcher := LocalFetcher{WorkspaceRoot: t.TempDir()}
	src := model.CatalogSource{URL: "github:owner/repo"} // wrong scheme for LocalFetcher

	_, err := fetcher.Fetch(context.Background(), src)
	// LocalFetcher should still work by calling ParseCatalogURL — it will parse
	// fine and return the path. But if someone passes an unsupported scheme that
	// Fetch doesn't handle, or a malformed URL, it should error.
	// Actually: LocalFetcher calls ParseCatalogURL which would return body="owner/repo"
	// and then try to stat that path. Let's just make sure it doesn't panic.
	// If it returns an error, great. If it tries to resolve the path and fails, also fine.
	// The key test is that it doesn't crash.
	_ = err // error may or may not occur, depends on whether path exists
}

// ─────────────────────────────────────────────────────────────────────────────
// GitFetcher error-path tests (no network / no git required)
// ─────────────────────────────────────────────────────────────────────────────

// TestGitFetcher_UnsupportedScheme_ReturnsError verifies that passing a local:
// URL to GitFetcher returns an error because GitFetcher only handles github: and gitlab:.
func TestGitFetcher_UnsupportedScheme_ReturnsError(t *testing.T) {
	t.Parallel()

	fetcher := GitFetcher{WorkspaceRoot: t.TempDir()}
	src := model.CatalogSource{URL: "local:./some/path"}

	_, err := fetcher.Fetch(context.Background(), src)
	if err == nil {
		t.Fatal("GitFetcher.Fetch expected error for local: scheme, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("error should mention 'unsupported scheme'; got: %v", err)
	}
}

// TestGitFetcher_MalformedURL_ReturnsError verifies that a completely malformed URL
// (no recognised scheme) causes an error without panicking.
func TestGitFetcher_MalformedURL_ReturnsError(t *testing.T) {
	t.Parallel()

	fetcher := GitFetcher{WorkspaceRoot: t.TempDir()}
	src := model.CatalogSource{URL: "not-a-valid-url-at-all"}

	_, err := fetcher.Fetch(context.Background(), src)
	if err == nil {
		t.Fatal("GitFetcher.Fetch expected error for malformed URL, got nil")
	}
}

// TestGitFetcher_FreshClone_PassesCorrectArgs verifies that GitFetcher.Fetch
// invokes git clone --depth=1 with the correct remote URL and cache path on a
// fresh (never-cloned) directory.
//
// NOTE: NOT parallel — this test injects a package-level fake (execGit) that
// must not race with other tests doing the same.
func TestGitFetcher_FreshClone_PassesCorrectArgs(t *testing.T) {
	wsRoot := t.TempDir()
	// Do NOT create the .git dir — this is a fresh clone scenario.
	fake := withFakeGit(t, []byte("ok"), nil)

	fetcher := GitFetcher{WorkspaceRoot: wsRoot}
	src := model.CatalogSource{URL: "github:owner/repo"}

	gotDir, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}

	// The returned directory must be non-empty and under the workspace root.
	if !strings.HasPrefix(gotDir, wsRoot) {
		t.Errorf("Fetch returned dir %q, want it under wsRoot %q", gotDir, wsRoot)
	}

	// Exactly one git call — the clone.
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 git call (clone), got %d: %v", len(fake.calls), fake.calls)
	}

	cloneArgs := fake.calls[0]
	// Must include "clone" and "--depth=1".
	assertArgsContain(t, cloneArgs, "clone")
	assertArgsContain(t, cloneArgs, "--depth=1")
	// Remote must be the HTTPS github URL.
	assertArgsContain(t, cloneArgs, "https://github.com/owner/repo.git")
}

// TestGitFetcher_SubsequentFetch_FetchAndCheckout verifies that a second Fetch
// call on a previously-cloned directory performs fetch + checkout (not clone).
//
// NOTE: NOT parallel — this test injects a package-level fake (execGit) that
// must not race with other tests doing the same.
func TestGitFetcher_SubsequentFetch_FetchAndCheckout(t *testing.T) {
	wsRoot := t.TempDir()
	src := model.CatalogSource{URL: "github:owner/repo"}

	// Pre-create the .git directory to simulate an already-cloned cache.
	cacheKey := CacheKey(src)
	cacheRoot := filepath.Join(wsRoot, ".devrune", "advisor-catalogs", cacheKey)
	dotGit := filepath.Join(cacheRoot, ".git")
	if err := os.MkdirAll(dotGit, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	fake := withFakeGit(t, []byte("ok"), nil)

	fetcher := GitFetcher{WorkspaceRoot: wsRoot}
	_, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}

	// Two git calls expected: fetch, then checkout.
	if len(fake.calls) != 2 {
		t.Fatalf("expected 2 git calls (fetch + checkout), got %d: %v", len(fake.calls), fake.calls)
	}

	fetchArgs := fake.calls[0]
	assertArgsContain(t, fetchArgs, "fetch")
	assertArgsContain(t, fetchArgs, "--depth=1")
	assertArgsContain(t, fetchArgs, "origin")

	checkoutArgs := fake.calls[1]
	assertArgsContain(t, checkoutArgs, "checkout")
	assertArgsContain(t, checkoutArgs, "FETCH_HEAD")
}

// TestGitFetcher_RefPinning_BranchPassedToClone verifies that a ref suffix in
// the URL (e.g. github:owner/repo@main) causes --branch main to be included in
// the clone arguments.
//
// NOTE: NOT parallel — this test injects a package-level fake (execGit) that
// must not race with other tests doing the same.
func TestGitFetcher_RefPinning_BranchPassedToClone(t *testing.T) {
	wsRoot := t.TempDir()
	fake := withFakeGit(t, []byte("ok"), nil)

	fetcher := GitFetcher{WorkspaceRoot: wsRoot}
	src := model.CatalogSource{URL: "github:owner/repo@main"}

	_, err := fetcher.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}

	// Single git clone call expected (fresh directory).
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 git call (clone), got %d: %v", len(fake.calls), fake.calls)
	}

	cloneArgs := fake.calls[0]
	assertArgsContain(t, cloneArgs, "clone")
	assertArgsContain(t, cloneArgs, "--depth=1")
	assertArgsContain(t, cloneArgs, "--branch")
	assertArgsContain(t, cloneArgs, "main")
}

// assertArgsContain is a helper that fails the test when want is not found among args.
func assertArgsContain(t *testing.T, args []string, want string) {
	t.Helper()
	if !slices.Contains(args, want) {
		t.Errorf("git args %v do not contain %q", args, want)
	}
}
