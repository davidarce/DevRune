package materialize

import (
	"fmt"
	"os"
	"path/filepath"
)

// SymlinkLinker creates symbolic links from source to destination.
// Parent directories of the destination are created automatically.
type SymlinkLinker struct{}

// Link creates a symbolic link at dst pointing to src.
// It ensures the parent directory of dst exists before creating the link.
// Returns an error if the parent directory cannot be created or if os.Symlink fails.
func (l *SymlinkLinker) Link(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("symlink: create parent dir for %q: %w", dst, err)
	}
	// Remove any existing file/link at dst to allow re-installation.
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("symlink: remove existing %q: %w", dst, err)
		}
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("symlink: link %q → %q: %w", src, dst, err)
	}
	return nil
}

// Mode returns "symlink".
func (l *SymlinkLinker) Mode() string { return "symlink" }
