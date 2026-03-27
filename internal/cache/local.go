// SPDX-License-Identifier: MIT

package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidarce/devrune/internal/model"
)

// LocalFetcher creates a gzip-compressed tar archive from a local directory.
// It implements the Fetcher interface for the "local:" scheme.
// The archive format mirrors what GitHub/GitLab tarballs produce, so the same
// extraction logic (ExtractTarGz) can be used for all schemes.
type LocalFetcher struct{}

// NewLocalFetcher creates a LocalFetcher.
func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{}
}

// Supports reports whether this fetcher handles the given scheme.
func (f *LocalFetcher) Supports(scheme model.Scheme) bool {
	return scheme == model.SchemeLocal
}

// Fetch creates a tar.gz archive of the directory or single file at ref.Path.
// The SourceRef must have Scheme == SchemeLocal and a non-empty Path.
// For directories, returned bytes are a gzip-compressed tar stream.
// For single files, the file is wrapped in a tar with a synthetic "local/" prefix.
func (f *LocalFetcher) Fetch(_ context.Context, ref model.SourceRef) ([]byte, error) {
	if ref.Scheme != model.SchemeLocal {
		return nil, fmt.Errorf("local fetcher: unsupported scheme %q", ref.Scheme)
	}

	src := ref.Path
	if src == "" {
		return nil, fmt.Errorf("local fetcher: path is empty")
	}

	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("local fetcher: stat %q: %w", src, err)
	}

	if info.IsDir() {
		return tarGzDir(src)
	}

	// Single file: wrap in a tar with "local/{filename}" path so that
	// extractTarGz's stripFirstComponent yields the filename directly.
	return tarGzFile(src, info)
}

// tarGzFile wraps a single file in a gzip-compressed tar archive.
// The entry is stored as "local/{filename}" so stripFirstComponent yields the filename.
func tarGzFile(path string, info os.FileInfo) ([]byte, error) {
	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, fmt.Errorf("local fetcher: file header %q: %w", path, err)
	}
	hdr.Name = "local/" + info.Name()

	if err := tw.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("local fetcher: write header %q: %w", path, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("local fetcher: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(tw, f); err != nil {
		return nil, fmt.Errorf("local fetcher: copy %q: %w", path, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("local fetcher: close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("local fetcher: close gzip: %w", err)
	}

	return buf.Bytes(), nil
}

// tarGzDir produces a gzip-compressed tar of srcDir.
// All paths inside the archive are prefixed with "local/" so that the
// generic extractTarGz (which strips the first path component for
// GitHub/GitLab tarball compatibility) produces the correct directory tree.
func tarGzDir(srcDir string) ([]byte, error) {
	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Compute path relative to srcDir
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Normalise to forward slashes (tar convention)
		rel = filepath.ToSlash(rel)

		// Skip the root directory itself
		if rel == "." {
			return nil
		}

		// Skip hidden files/directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		// Prefix with "local/" so extractTarGz's stripFirstComponent
		// removes "local/" instead of the actual first directory (e.g. "skills/").
		hdr.Name = "local/" + rel

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !d.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("local fetcher: walk %q: %w", srcDir, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("local fetcher: close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("local fetcher: close gzip: %w", err)
	}

	return buf.Bytes(), nil
}
