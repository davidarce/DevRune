package materialize

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyLinker copies files or directories from source to destination.
// Permissions are preserved. Parent directories are created automatically.
type CopyLinker struct{}

// Link copies src to dst recursively.
// If src is a directory, all contents are copied recursively.
// If src is a file, it is copied with its permissions preserved.
func (l *CopyLinker) Link(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: stat %q: %w", src, err)
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

// Mode returns "copy".
func (l *CopyLinker) Mode() string { return "copy" }

// HardlinkLinker creates hard links from source to destination.
// Falls back to copy for directories (hard links cannot span directories).
type HardlinkLinker struct{}

// Link creates a hard link at dst pointing to src.
// For directories, falls back to recursive copy since hard-linking directories
// is not supported on most platforms.
func (l *HardlinkLinker) Link(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("hardlink: stat %q: %w", src, err)
	}
	if info.IsDir() {
		// Directories cannot be hard-linked; fall back to copy.
		return copyDir(src, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("hardlink: create parent dir for %q: %w", dst, err)
	}
	// Remove any existing target.
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("hardlink: remove existing %q: %w", dst, err)
		}
	}
	if err := os.Link(src, dst); err != nil {
		// Fall back to copy if hard link fails (e.g., cross-device).
		fi, statErr := os.Stat(src)
		if statErr != nil {
			return fmt.Errorf("hardlink: %w", err)
		}
		return copyFile(src, dst, fi.Mode())
	}
	return nil
}

// Mode returns "hardlink".
func (l *HardlinkLinker) Mode() string { return "hardlink" }

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy dir: stat %q: %w", src, err)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("copy dir: mkdir %q: %w", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("copy dir: read %q: %w", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("copy dir: entry info %q: %w", srcPath, err)
			}
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file from src to dst with the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("copy file: create parent dir for %q: %w", dst, err)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy file: open %q: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("copy file: create %q: %w", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file: write %q: %w", dst, err)
	}
	return nil
}
