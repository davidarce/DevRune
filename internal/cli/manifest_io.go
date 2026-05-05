// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"path/filepath"

	"github.com/davidarce/devrune/internal/backup"
)

// writeManifestSafe creates a backup of the current devrune.yaml (if it
// exists) and then writes data to manifestPath atomically via a .tmp rename.
//
// This is the canonical write path for every mutation of devrune.yaml inside
// the cli package. Using it everywhere ensures:
//   - PRD US1-US3: a snapshot exists before any overwrite.
//   - R1: the manifest is never partially written (atomic rename).
//   - A failed backup aborts the write so the manifest is left untouched.
//
// dir is derived from manifestPath; callers do not need to pass the project
// root separately.
func writeManifestSafe(manifestPath string, data []byte) error {
	dir := filepath.Dir(manifestPath)
	if err := backup.CreateBackup(dir, manifestPath); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	if err := backup.WriteFileAtomic(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
