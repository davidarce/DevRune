package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileCacheStore is a content-addressed cache that stores extracted package
// directories under a configurable base directory (e.g. ~/.cache/devrune/packages/).
//
// Directory layout:
//
//	baseDir/
//	  {sha256hex}/   ← extracted package directory
//
// The SHA256 hash is computed over the raw archive bytes, ensuring that two
// fetches of the same immutable ref always resolve to the same directory.
type FileCacheStore struct {
	baseDir string // e.g. ~/.cache/devrune/packages/
}

// NewFileCacheStore creates a FileCacheStore rooted at baseDir.
// The directory is created lazily on the first Store call.
func NewFileCacheStore(baseDir string) *FileCacheStore {
	return &FileCacheStore{baseDir: baseDir}
}

// hashData computes the SHA256 of data and returns it as "sha256:<hex>".
func hashData(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}

// Has reports whether the given hash is present in the cache.
// hash must be in "sha256:<hex>" format.
func (s *FileCacheStore) Has(hash string) bool {
	_, ok := s.Get(hash)
	return ok
}

// Get returns the path to the extracted directory for the given hash.
// hash must be in "sha256:<hex>" format.
// Returns ("", false) if not cached.
func (s *FileCacheStore) Get(hash string) (string, bool) {
	hex := strings.TrimPrefix(hash, "sha256:")
	if hex == "" {
		return "", false
	}
	dir := filepath.Join(s.baseDir, hex)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return dir, true
}

// Store writes data to the cache and returns the path to the extracted directory.
// If the entry is already present (same hash), the existing directory path is
// returned immediately without re-extraction.
//
// The returned dir path is stable across runs and can be used directly by the
// materializer.
func (s *FileCacheStore) Store(key string, data []byte) (string, error) {
	hash := hashData(data)
	hex := strings.TrimPrefix(hash, "sha256:")

	// Return immediately if already cached.
	if dir, ok := s.Get(hash); ok {
		return dir, nil
	}

	// Ensure base directory exists.
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return "", fmt.Errorf("cache store: create base dir %q: %w", s.baseDir, err)
	}

	destDir := filepath.Join(s.baseDir, hex)

	// Extract to a temp directory first, then rename atomically.
	tmpDir, err := os.MkdirTemp(s.baseDir, "tmp-"+hex+"-")
	if err != nil {
		return "", fmt.Errorf("cache store: create temp dir: %w", err)
	}

	if err := extractTarGz(data, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("cache store: extract archive for %q: %w", key, err)
	}

	// Atomic rename: if destDir already exists (race condition), that's fine.
	if err := os.Rename(tmpDir, destDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		if _, statErr := os.Stat(destDir); statErr == nil {
			// Another concurrent process beat us; use the existing entry.
			return destDir, nil
		}
		return "", fmt.Errorf("cache store: rename temp to cache dir: %w", err)
	}

	return destDir, nil
}

// extractTarGz decompresses and untars data into destDir.
// All paths are sanitised to prevent directory traversal.
// The first path component is stripped (GitHub/GitLab tarballs include a
// top-level "owner-repo-sha/" prefix; local archives do not, so stripping is
// a no-op when there is only one path component).
func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Strip the first path component.
		rel := stripFirstComponent(hdr.Name)
		if rel == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(rel))

		// Prevent directory traversal.
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return fmt.Errorf("tar entry %q would escape destination directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %q: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent of %q: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("create %q: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("write %q: %w", target, err)
			}
			_ = f.Close()

		default:
			// Skip symlinks, hardlinks, devices etc. in MVP.
		}
	}

	return nil
}

// stripFirstComponent removes the first path component from a tar entry name.
// e.g. "owner-repo-abc123/skills/git-commit/SKILL.md" → "skills/git-commit/SKILL.md"
// If the name has only one component (local archives), the result is empty,
// meaning the entry is the root directory itself and should be skipped.
func stripFirstComponent(name string) string {
	// Normalise to forward slashes and strip leading slash.
	name = strings.TrimPrefix(filepath.ToSlash(name), "/")
	idx := strings.Index(name, "/")
	if idx < 0 {
		// Single component — this is the root directory itself.
		return ""
	}
	return name[idx+1:]
}
