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
	dir, ok := s.stored[hash]
	return dir, ok
}

func (s *mockCacheStore) Has(hash string) bool {
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
		case tar.TypeReg, tar.TypeRegA:
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
		Workflows:     []string{"github:owner/wf@v1.0.0"},
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
		Workflows:     []string{"not-a-valid-source-ref"},
	}

	_, err := r.Resolve(context.Background(), manifest)
	if err == nil {
		t.Fatal("Resolve() expected error for invalid workflow source, got nil")
	}
}
