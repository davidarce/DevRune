// SPDX-License-Identifier: MIT

package resolve

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// buildTarGz creates a gzip-compressed tar archive from a map of path → content.
// Paths use forward slashes; a "prefix/" is prepended to each entry to mimic
// GitHub/GitLab archive format (owner-repo-sha/file).
func buildTarGz(t *testing.T, prefix string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write a directory entry for the prefix.
	if prefix != "" {
		hdr := &tar.Header{
			Typeflag: tar.TypeDir,
			Name:     prefix + "/",
			Mode:     0o755,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("buildTarGz: write dir header: %v", err)
		}
	}

	for path, content := range files {
		fullPath := path
		if prefix != "" {
			fullPath = prefix + "/" + path
		}
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     fullPath,
			Size:     int64(len(content)),
			Mode:     0o644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("buildTarGz: write header for %q: %v", path, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("buildTarGz: write content for %q: %v", path, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("buildTarGz: close tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("buildTarGz: close gzip: %v", err)
	}
	return buf.Bytes()
}

// mockFetcher is a simple Fetcher implementation that returns pre-built archives.
type mockFetcher struct {
	// archives maps CacheKey → raw archive bytes
	archives map[string][]byte
	// fetchErr overrides all fetches with this error when set
	fetchErr error
}

func (m *mockFetcher) Fetch(_ context.Context, ref model.SourceRef) ([]byte, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	key := ref.CacheKey()
	data, ok := m.archives[key]
	if !ok {
		return nil, fmt.Errorf("mock fetcher: no archive for %q", key)
	}
	return data, nil
}

func (m *mockFetcher) Supports(scheme model.Scheme) bool {
	return true
}

// mockCacheStore is an in-memory CacheStore that extracts archives to a temp dir.
type mockCacheStore struct {
	t       *testing.T
	baseDir string
	mu      sync.Mutex
	stored  map[string]string // hash → dir
}

func newMockCacheStore(t *testing.T) *mockCacheStore {
	t.Helper()
	return &mockCacheStore{
		t:       t,
		baseDir: t.TempDir(),
		stored:  make(map[string]string),
	}
}

func (s *mockCacheStore) Store(key string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := HashBytes(data)
	if dir, ok := s.stored[hash]; ok {
		return dir, nil
	}
	dir := filepath.Join(s.baseDir, strings.TrimPrefix(hash, "sha256:"))
	if err := extractForTest(s.t, data, dir); err != nil {
		return "", fmt.Errorf("mockCacheStore.Store: %w", err)
	}
	s.stored[hash] = dir
	return dir, nil
}

func (s *mockCacheStore) Get(hash string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, ok := s.stored[hash]
	return dir, ok
}

func (s *mockCacheStore) Has(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.stored[hash]
	return ok
}

// extractForTest extracts a tar.gz archive to destDir, stripping the first
// path component (mirroring the production extractTarGz behaviour).
func extractForTest(t *testing.T, data []byte, destDir string) error {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break // io.EOF or error — stop
		}
		rel := stripFirstPathComponent(hdr.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(destDir, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			content, err2 := func() ([]byte, error) {
				var buf bytes.Buffer
				_, err := buf.ReadFrom(tr)
				return buf.Bytes(), err
			}()
			if err2 != nil {
				return fmt.Errorf("read %q: %w", hdr.Name, err2)
			}
			if err := os.WriteFile(target, content, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildMinimalManifest creates a minimal valid UserManifest.
func buildMinimalManifest(packageSource string) model.UserManifest {
	return model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages: []model.PackageRef{
			{Source: packageSource},
		},
	}
}

// ---------------------------------------------------------------------------
// Resolver tests
// ---------------------------------------------------------------------------

// TestResolver_MinimalManifest verifies that resolving a minimal manifest
// produces a valid lockfile with the correct hash and content items.
func TestResolver_MinimalManifest(t *testing.T) {
	archive := buildTarGz(t, "owner-repo-v100", map[string]string{
		"skills/git-commit/SKILL.md": "# git-commit",
		"rules/arch/clean.md":        "# clean arch rule",
	})

	fetcher := &mockFetcher{
		archives: map[string][]byte{
			"github:owner/repo@v1.0.0": archive,
		},
	}
	cache := newMockCacheStore(t)

	r := NewResolver(fetcher, cache, "")
	manifest := buildMinimalManifest("github:owner/repo@v1.0.0")

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Lockfile must have the correct schema version.
	if lockfile.SchemaVersion != "devrune/lock/v1" {
		t.Errorf("SchemaVersion = %q, want %q", lockfile.SchemaVersion, "devrune/lock/v1")
	}

	// ManifestHash must be non-empty and start with "sha256:".
	if !strings.HasPrefix(lockfile.ManifestHash, "sha256:") {
		t.Errorf("ManifestHash = %q, missing sha256: prefix", lockfile.ManifestHash)
	}

	// Should have exactly one locked package.
	if len(lockfile.Packages) != 1 {
		t.Fatalf("Packages count = %d, want 1", len(lockfile.Packages))
	}

	pkg := lockfile.Packages[0]
	if !strings.HasPrefix(pkg.Hash, "sha256:") {
		t.Errorf("Package.Hash = %q, missing sha256: prefix", pkg.Hash)
	}

	// Verify the hash matches the expected hash of the archive.
	expectedHash := HashBytes(archive)
	if pkg.Hash != expectedHash {
		t.Errorf("Package.Hash = %q, want %q", pkg.Hash, expectedHash)
	}

	// Should discover skill and rule from the archive.
	skills := itemsByKind(pkg.Contents, model.KindSkill)
	rules := itemsByKind(pkg.Contents, model.KindRule)

	if len(skills) != 1 || skills[0].Name != "git-commit" {
		t.Errorf("skills = %v, want [git-commit]", skills)
	}
	if len(rules) != 1 || rules[0].Name != "arch/clean" {
		t.Errorf("rules = %v, want [arch/clean]", rules)
	}
}

// TestResolver_SelectFilter verifies that a select filter limits the contents.
func TestResolver_SelectFilter(t *testing.T) {
	archive := buildTarGz(t, "owner-repo-v100", map[string]string{
		"skills/git-commit/SKILL.md":       "# git-commit",
		"skills/git-pull-request/SKILL.md": "# git-pull-request",
		"skills/sdd-explore/SKILL.md":      "# sdd-explore",
	})

	fetcher := &mockFetcher{
		archives: map[string][]byte{
			"github:owner/repo@v1.0.0": archive,
		},
	}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages: []model.PackageRef{
			{
				Source: "github:owner/repo@v1.0.0",
				Select: &model.SelectFilter{Skills: []string{"git-commit"}},
			},
		},
	}

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(lockfile.Packages) != 1 {
		t.Fatalf("Packages count = %d, want 1", len(lockfile.Packages))
	}

	skills := itemsByKind(lockfile.Packages[0].Contents, model.KindSkill)
	if len(skills) != 1 {
		t.Errorf("got %d skills after select filter, want 1", len(skills))
	}
	if len(skills) == 1 && skills[0].Name != "git-commit" {
		t.Errorf("selected skill = %q, want %q", skills[0].Name, "git-commit")
	}
}

// TestResolver_GitLabSource verifies that a gitlab: source produces a correct lockfile entry.
func TestResolver_GitLabSource(t *testing.T) {
	archive := buildTarGz(t, "owner-repo-v100", map[string]string{
		"skills/deploy/SKILL.md": "# deploy",
	})

	fetcher := &mockFetcher{
		archives: map[string][]byte{
			"gitlab:myorg/myrepo@v1.0.0?host=gitlab.com": archive,
		},
	}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages: []model.PackageRef{
			{Source: "gitlab:myorg/myrepo@v1.0.0"},
		},
	}

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(lockfile.Packages) != 1 {
		t.Fatalf("Packages count = %d, want 1", len(lockfile.Packages))
	}

	pkg := lockfile.Packages[0]
	if pkg.Source.Scheme != model.SchemeGitLab {
		t.Errorf("Source.Scheme = %q, want %q", pkg.Source.Scheme, model.SchemeGitLab)
	}
	if pkg.Source.Owner != "myorg" {
		t.Errorf("Source.Owner = %q, want %q", pkg.Source.Owner, "myorg")
	}

	skills := itemsByKind(pkg.Contents, model.KindSkill)
	if len(skills) != 1 || skills[0].Name != "deploy" {
		t.Errorf("skills = %v, want [deploy]", skills)
	}
}

// TestResolver_FetchFailure verifies that a fetch failure is propagated as an error.
func TestResolver_FetchFailure(t *testing.T) {
	fetchErr := errors.New("network timeout")
	fetcher := &mockFetcher{fetchErr: fetchErr}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := buildMinimalManifest("github:owner/repo@v1.0.0")
	_, err := r.Resolve(context.Background(), manifest)
	if err == nil {
		t.Fatal("Resolve() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetch") {
		t.Errorf("error = %q, expected 'fetch' in message", err.Error())
	}
}

// TestResolver_MultiplePackages verifies that multiple packages are all resolved.
func TestResolver_MultiplePackages(t *testing.T) {
	archiveA := buildTarGz(t, "owner-pkg-a-v100", map[string]string{
		"skills/skill-a/SKILL.md": "# skill-a",
	})
	archiveB := buildTarGz(t, "owner-pkg-b-v100", map[string]string{
		"skills/skill-b/SKILL.md": "# skill-b",
	})

	fetcher := &mockFetcher{
		archives: map[string][]byte{
			"github:owner/pkg-a@v1.0.0": archiveA,
			"github:owner/pkg-b@v1.0.0": archiveB,
		},
	}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages: []model.PackageRef{
			{Source: "github:owner/pkg-a@v1.0.0"},
			{Source: "github:owner/pkg-b@v1.0.0"},
		},
	}

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(lockfile.Packages) != 2 {
		t.Fatalf("Packages count = %d, want 2", len(lockfile.Packages))
	}
}

// TestResolver_EmptyPackages verifies that a manifest with no packages produces an empty lockfile.
func TestResolver_EmptyPackages(t *testing.T) {
	fetcher := &mockFetcher{archives: map[string][]byte{}}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages:      nil,
	}

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(lockfile.Packages) != 0 {
		t.Errorf("Packages count = %d, want 0", len(lockfile.Packages))
	}
}

// TestResolver_WithWorkflow verifies that a manifest with a workflow source
// resolves correctly (the workflow is parsed and its name is recorded).
func TestResolver_WithWorkflow(t *testing.T) {
	// Build a workflow archive with a valid workflow.yaml.
	workflowYAMLContent := `apiVersion: devrune/workflow/v1
metadata:
  name: sdd
  version: 1.0.0
components:
  skills:
    - sdd-explore
  commands:
    - name: sdd-explore
      action: Explore and investigate
`
	wfArchive := buildTarGz(t, "owner-wf-v100", map[string]string{
		"workflow.yaml": workflowYAMLContent,
	})

	pkgArchive := buildTarGz(t, "owner-pkg-v100", map[string]string{
		"skills/git-commit/SKILL.md": "# git-commit",
	})

	fetcher := &mockFetcher{
		archives: map[string][]byte{
			"github:owner/pkg@v1.0.0": pkgArchive,
			"github:owner/wf@v1.0.0":  wfArchive,
		},
	}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages:      []model.PackageRef{{Source: "github:owner/pkg@v1.0.0"}},
		Workflows: map[string]model.WorkflowEntry{
			"wf": {Source: "github:owner/wf@v1.0.0"},
		},
	}

	lockfile, err := r.Resolve(context.Background(), manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(lockfile.Workflows) != 1 {
		t.Fatalf("Workflows count = %d, want 1", len(lockfile.Workflows))
	}

	wf := lockfile.Workflows[0]
	if wf.Name != "sdd" {
		t.Errorf("Workflow.Name = %q, want %q", wf.Name, "sdd")
	}
	if !strings.HasPrefix(wf.Hash, "sha256:") {
		t.Errorf("Workflow.Hash = %q, missing sha256: prefix", wf.Hash)
	}
}

// TestResolver_InvalidWorkflowSourceRef verifies that an invalid workflow source ref causes an error.
func TestResolver_InvalidWorkflowSourceRef(t *testing.T) {
	fetcher := &mockFetcher{archives: map[string][]byte{}}
	cache := newMockCacheStore(t)
	r := NewResolver(fetcher, cache, "")

	manifest := model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        []model.AgentRef{{Name: "claude"}},
		Packages:      nil,
		Workflows: map[string]model.WorkflowEntry{
			"test": {Source: "not-a-valid-source-ref"},
		},
	}

	_, err := r.Resolve(context.Background(), manifest)
	if err == nil {
		t.Fatal("Resolve() expected error for invalid workflow source, got nil")
	}
}

// countingFetcher wraps mockFetcher to count how many times Fetch is called
// per CacheKey. Used by the mutable-ref cache-bypass regression test to
// distinguish a cache hit (no additional fetch) from a cache miss that
// re-hits the network.
type countingFetcher struct {
	inner         mockFetcher
	calls         map[string]int
	revisions     map[string]string // CacheKey → SHA to return from ResolveRevision
	revisionCalls map[string]int    // CacheKey → ResolveRevision call count
	revisionErr   error             // when non-nil, ResolveRevision fails
}

func (f *countingFetcher) Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[ref.CacheKey()]++
	return f.inner.Fetch(ctx, ref)
}

func (f *countingFetcher) Supports(scheme model.Scheme) bool { return f.inner.Supports(scheme) }

// ResolveRevision satisfies RevisionResolver so countingFetcher can drive the
// SHA-check path. Tests that want to simulate a fetcher WITHOUT revision
// support should use plainFetcher (below) instead.
func (f *countingFetcher) ResolveRevision(_ context.Context, ref model.SourceRef) (string, error) {
	if f.revisionCalls == nil {
		f.revisionCalls = map[string]int{}
	}
	f.revisionCalls[ref.CacheKey()]++
	if f.revisionErr != nil {
		return "", f.revisionErr
	}
	sha, ok := f.revisions[ref.CacheKey()]
	if !ok {
		return "", fmt.Errorf("counting fetcher: no revision for %q", ref.CacheKey())
	}
	return sha, nil
}

// plainFetcher is a Fetcher that deliberately does NOT implement
// RevisionResolver. Used to verify the fallback path (re-fetch mutable refs
// when the fetcher can't cheap-check the SHA).
type plainFetcher struct {
	inner mockFetcher
	calls map[string]int
}

func (f *plainFetcher) Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[ref.CacheKey()]++
	return f.inner.Fetch(ctx, ref)
}

func (f *plainFetcher) Supports(scheme model.Scheme) bool { return f.inner.Supports(scheme) }

// TestResolver_EmptyRefBypassesCache is the regression test for the silent
// "sync doesn't pull upstream changes" bug reported against `devrune sync`.
//
// Scenario: the user pins a package with an empty ref (e.g.
// `github:org/repo` without an `@ref` suffix), which resolves to the remote's
// HEAD. On the first resolve the tarball is fetched, hashed, and cached. On
// subsequent resolves, the prior lockfile carries that content hash under the
// CacheKey `github:org/repo@` — if the resolver blindly trusts the CacheKey
// mapping, upstream commits on the default branch are invisible because the
// resolver never re-contacts the remote.
//
// The fix: empty-ref (and literal "HEAD") sources skip the cache shortcut,
// forcing a fetch every resolve so the new content hash (and therefore the
// new cached tree) lands in the updated lockfile. Pinned refs (tags, commit
// SHAs, explicit branch names) are unaffected — they still reuse the cache.
func TestResolver_EmptyRefBypassesCache(t *testing.T) {
	t.Run("empty ref re-fetches even when prior lockfile has a hash", func(t *testing.T) {
		archiveV1 := buildTarGz(t, "owner-repo-head-v1", map[string]string{
			"skills/git-commit/SKILL.md": "# v1",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": archiveV1,
			}},
		}
		cache := newMockCacheStore(t)
		r := NewResolver(f, cache, "")

		// Seed a prior lockfile whose CacheKey matches an empty ref. If the
		// resolver honours the cache shortcut for this (buggy) case it won't
		// call Fetch; the counter proves it does.
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash: HashBytes(archiveV1),
				},
			},
		})

		manifest := buildMinimalManifest("github:owner/repo")
		if _, err := r.Resolve(context.Background(), manifest); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 1 {
			t.Errorf("Fetch called %d times for empty-ref source, want 1 (cache shortcut must NOT apply for mutable HEAD)", got)
		}
	})

	t.Run("pinned tag reuses cache", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-v100", map[string]string{
			"skills/git-commit/SKILL.md": "# pinned",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@v1.0.0": archive,
			}},
		}
		cache := newMockCacheStore(t)

		// Prime the cache so the prior-lockfile shortcut can fire.
		if _, err := cache.Store("github:owner/repo@v1.0.0", archive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
						Ref:    "v1.0.0",
					},
					Hash: HashBytes(archive),
				},
			},
		})

		manifest := buildMinimalManifest("github:owner/repo@v1.0.0")
		if _, err := r.Resolve(context.Background(), manifest); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@v1.0.0"]; got != 0 {
			t.Errorf("Fetch called %d times for pinned tag, want 0 (cache shortcut must apply for immutable refs)", got)
		}
	})

	t.Run("literal HEAD ref is treated as mutable", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-head-literal", map[string]string{
			"skills/x/SKILL.md": "# head",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@HEAD": archive,
			}},
		}
		cache := newMockCacheStore(t)
		r := NewResolver(f, cache, "")

		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
						Ref:    "HEAD",
					},
					Hash: HashBytes(archive),
				},
			},
		})

		manifest := buildMinimalManifest("github:owner/repo@HEAD")
		if _, err := r.Resolve(context.Background(), manifest); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@HEAD"]; got != 1 {
			t.Errorf("Fetch called %d times for literal HEAD ref, want 1 (HEAD is mutable)", got)
		}
	})
}

// TestResolver_MutableRefSHARevalidation covers the SHA-check optimisation
// that avoids downloading the full tarball on every sync when the upstream
// default branch hasn't moved. The previous mutable-ref bypass always
// re-fetched; this path keeps correctness while restoring cache reuse.
func TestResolver_MutableRefSHARevalidation(t *testing.T) {
	t.Run("matching SHA keeps cache (no Fetch, one ResolveRevision)", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-head", map[string]string{
			"skills/git-commit/SKILL.md": "# cached content",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": archive,
			}},
			revisions: map[string]string{
				"github:owner/repo@": "abc123def456",
			},
		}
		cache := newMockCacheStore(t)

		// Seed the cache so a hit can actually find the extracted dir.
		if _, err := cache.Store("github:owner/repo@", archive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash:     HashBytes(archive),
					Revision: "abc123def456",
				},
			},
		})

		lf, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo"))
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 0 {
			t.Errorf("Fetch calls = %d, want 0 (SHA matched, cache reused)", got)
		}
		if got := f.revisionCalls["github:owner/repo@"]; got != 1 {
			t.Errorf("ResolveRevision calls = %d, want 1 (one cheap HEAD check)", got)
		}
		if len(lf.Packages) != 1 {
			t.Fatalf("packages = %d, want 1", len(lf.Packages))
		}
		if lf.Packages[0].Revision != "abc123def456" {
			t.Errorf("Revision = %q, want %q (preserved from prior lockfile)", lf.Packages[0].Revision, "abc123def456")
		}
	})

	t.Run("mismatched SHA forces re-fetch and records new revision", func(t *testing.T) {
		oldArchive := buildTarGz(t, "owner-repo-old", map[string]string{
			"skills/x/SKILL.md": "# old",
		})
		newArchive := buildTarGz(t, "owner-repo-new", map[string]string{
			"skills/x/SKILL.md": "# new",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": newArchive, // upstream moved
			}},
			revisions: map[string]string{
				"github:owner/repo@": "newsha00000", // remote HEAD points here now
			},
		}
		cache := newMockCacheStore(t)
		if _, err := cache.Store("github:owner/repo@", oldArchive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash:     HashBytes(oldArchive),
					Revision: "oldsha00000",
				},
			},
		})

		lf, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo"))
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 1 {
			t.Errorf("Fetch calls = %d, want 1 (SHA changed, tarball re-fetched)", got)
		}
		if lf.Packages[0].Hash != HashBytes(newArchive) {
			t.Errorf("Hash = %q, want new archive hash (upstream moved)", lf.Packages[0].Hash)
		}
		if lf.Packages[0].Revision != "newsha00000" {
			t.Errorf("Revision = %q, want %q (new SHA recorded)", lf.Packages[0].Revision, "newsha00000")
		}
	})

	t.Run("missing prior revision falls back to re-fetch (backward compat with pre-revision lockfiles)", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-migrate", map[string]string{
			"skills/x/SKILL.md": "# content",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": archive,
			}},
			revisions: map[string]string{
				"github:owner/repo@": "firstsha00000",
			},
		}
		cache := newMockCacheStore(t)
		r := NewResolver(f, cache, "")

		// Prior lockfile entry exists but has no Revision (old schema).
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash: HashBytes(archive),
					// Revision intentionally empty
				},
			},
		})

		lf, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo"))
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 1 {
			t.Errorf("Fetch calls = %d, want 1 (no prior revision → refetch)", got)
		}
		if got := f.revisionCalls["github:owner/repo@"]; got != 1 {
			t.Errorf("ResolveRevision calls = %d, want 1 (post-fetch capture)", got)
		}
		if lf.Packages[0].Revision != "firstsha00000" {
			t.Errorf("Revision = %q, want %q (recorded for next sync)", lf.Packages[0].Revision, "firstsha00000")
		}
	})

	t.Run("fetcher without RevisionResolver falls back to re-fetch", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-plain", map[string]string{
			"skills/x/SKILL.md": "# content",
		})

		f := &plainFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": archive,
			}},
		}
		cache := newMockCacheStore(t)
		if _, err := cache.Store("github:owner/repo@", archive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash:     HashBytes(archive),
					Revision: "somesha00000",
				},
			},
		})

		if _, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo")); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 1 {
			t.Errorf("Fetch calls = %d, want 1 (no RevisionResolver → always refetch for mutable refs)", got)
		}
	})

	t.Run("ResolveRevision error falls back to re-fetch", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-shaerr", map[string]string{
			"skills/x/SKILL.md": "# content",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@": archive,
			}},
			revisionErr: fmt.Errorf("simulated network failure"),
		}
		cache := newMockCacheStore(t)
		if _, err := cache.Store("github:owner/repo@", archive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
					},
					Hash:     HashBytes(archive),
					Revision: "somesha00000",
				},
			},
		})

		if _, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo")); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@"]; got != 1 {
			t.Errorf("Fetch calls = %d, want 1 (revision lookup errored → refetch, not stale cache)", got)
		}
	})

	t.Run("pinned tag does not trigger any ResolveRevision call", func(t *testing.T) {
		archive := buildTarGz(t, "owner-repo-v1", map[string]string{
			"skills/x/SKILL.md": "# content",
		})

		f := &countingFetcher{
			inner: mockFetcher{archives: map[string][]byte{
				"github:owner/repo@v1.0.0": archive,
			}},
			revisions: map[string]string{
				"github:owner/repo@v1.0.0": "tagsha",
			},
		}
		cache := newMockCacheStore(t)
		if _, err := cache.Store("github:owner/repo@v1.0.0", archive); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		r := NewResolver(f, cache, "")
		r.SetPriorLockfile(model.Lockfile{
			Packages: []model.LockedPackage{
				{
					Source: model.SourceRef{
						Scheme: model.SchemeGitHub,
						Owner:  "owner",
						Repo:   "repo",
						Ref:    "v1.0.0",
					},
					Hash: HashBytes(archive),
				},
			},
		})

		if _, err := r.Resolve(context.Background(), buildMinimalManifest("github:owner/repo@v1.0.0")); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if got := f.calls["github:owner/repo@v1.0.0"]; got != 0 {
			t.Errorf("Fetch calls = %d, want 0 (pinned tag is immutable)", got)
		}
		if got := f.revisionCalls["github:owner/repo@v1.0.0"]; got != 0 {
			t.Errorf("ResolveRevision calls = %d, want 0 (pinned tag doesn't need SHA check)", got)
		}
	})
}
