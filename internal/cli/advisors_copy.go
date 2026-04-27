// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// copyAdvisorDir copies every file under src into dst, preserving relative
// paths. It REQUIRES src to be a directory containing a SKILL.md at its
// root. A bare SKILL.md file source is rejected — the caller must pass the
// parent directory so that references/ and templates/ siblings are captured.
//
// Copy rules:
//   - src must be a directory; if it is a regular file the call fails with a
//     descriptive error.
//   - SKILL.md must exist at the root of src; absent → error.
//   - Skip .git/, .DS_Store, .gitignore at ANY depth.
//   - Copy all other dotfiles verbatim (e.g. .env.example).
//   - For symlinks: follow via os.EvalSymlinks, copy the target content as a
//     regular file at the corresponding dst path.
//   - Preserve executable bits via os.Chmod after copy.
//   - Overwrite destination on conflict (source is authoritative).
//   - Create dst recursively if missing.
//
// Returns the list of absolute destination paths written.
func copyAdvisorDir(src, dst string) ([]string, error) {
	// 1. src must be a directory.
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("copyAdvisorDir: stat src %q: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("skillSource must be a directory containing SKILL.md, got file: %s", src)
	}

	// 2. SKILL.md must exist at the root of src.
	skillMDPath := filepath.Join(src, "SKILL.md")
	if _, err := os.Stat(skillMDPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("custom advisor %q: skillSource directory has no SKILL.md at %s", filepath.Base(src), src)
		}
		return nil, fmt.Errorf("copyAdvisorDir: stat SKILL.md in %q: %w", src, err)
	}

	// 3. Ensure dst exists.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, fmt.Errorf("copyAdvisorDir: mkdir dst %q: %w", dst, err)
	}

	var written []string

	// 4. Walk src and copy every entry.
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Compute the path relative to src so we can mirror it under dst.
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("copyAdvisorDir: rel path for %q: %w", path, err)
		}

		// Skip filtered names at any depth.
		name := d.Name()
		if isCopySkipped(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		// Symlink: resolve to the real target and copy its content.
		if d.Type()&fs.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("copyAdvisorDir: eval symlink %q: %w", path, err)
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return fmt.Errorf("copyAdvisorDir: stat symlink target %q: %w", resolved, err)
			}
			if err := copySingleFileWithMode(resolved, dstPath, info.Mode()); err != nil {
				return err
			}
			written = append(written, dstPath)
			return nil
		}

		// Regular file: copy preserving mode.
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("copyAdvisorDir: info %q: %w", path, err)
		}
		if err := copySingleFileWithMode(path, dstPath, info.Mode()); err != nil {
			return err
		}
		written = append(written, dstPath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("copyAdvisorDir: walk %q: %w", src, err)
	}

	return written, nil
}

// isCopySkipped reports whether a file or directory name should be excluded
// from the advisor directory copy. The check is applied at every depth.
//
// Skipped unconditionally:
//   - ".git"        — version-control metadata
//   - ".DS_Store"   — macOS Finder metadata
//   - ".gitignore"  — repository-specific ignore rules
func isCopySkipped(name string) bool {
	switch name {
	case ".git", ".DS_Store", ".gitignore":
		return true
	default:
		return false
	}
}

// copySingleFileWithMode copies a single file from src to dst and applies the
// given permission bits after writing. The destination directory is created if
// it does not exist.
func copySingleFileWithMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("copySingleFileWithMode: mkdir parent of %q: %w", dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copySingleFileWithMode: open %q: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("copySingleFileWithMode: create %q: %w", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copySingleFileWithMode: copy %q → %q: %w", src, dst, err)
	}

	// Preserve executable bits explicitly (in case umask stripped them).
	if mode&0o111 != 0 {
		if err := os.Chmod(dst, mode); err != nil {
			return fmt.Errorf("copySingleFileWithMode: chmod %q: %w", dst, err)
		}
	}

	return nil
}
