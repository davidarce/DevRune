package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxBackups is the maximum number of backup files kept (FIFO rotation).
const MaxBackups = 5

// bakSubDir is the subdirectory inside .devrune/ that stores backups.
const bakSubDir = "backups"

// bakPrefix is the filename prefix used for all backup files.
const bakPrefix = "devrune.yaml."

const (
	gitignoreBeginMarker = "# >>> devrune managed — do not edit"
	gitignoreEndMarker   = "# <<< devrune managed"
)

// BackupEntry represents one backup snapshot on disk.
type BackupEntry struct {
	Path      string    // absolute path to the backup file
	Timestamp time.Time // parsed from filename (RFC3339 UTC, colons replaced by hyphens)
	Name      string    // display name, e.g. "2026-05-04T14-30-00Z"
}

// backupsDir returns the absolute path to .devrune/backups/ for the given project directory.
func backupsDir(projectDir string) string {
	return filepath.Join(projectDir, ".devrune", bakSubDir)
}

// CreateBackup snapshots the current devrune.yaml into .devrune/backups/.
// dir is the project root (where devrune.yaml and .devrune/ live).
// manifestPath is the absolute path to devrune.yaml.
// If manifestPath does not exist (e.g. first init), CreateBackup returns nil (no-op).
// If backup creation fails for any other reason, returns a descriptive error.
// The caller must treat a non-nil error as "abort the operation".
//
// TODO: acquire advisory lock if concurrent writes become a concern.
func CreateBackup(dir, manifestPath string) error {
	// Read the manifest; if it does not exist this is a first-init no-op.
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("backup: read manifest: %w", err)
	}

	// Ensure the backups directory exists.
	bakDir := backupsDir(dir)
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		return fmt.Errorf("backup: create backups dir: %w", err)
	}

	// Write the backup file atomically.
	ts := time.Now().UTC().Format(time.RFC3339)
	ts = strings.ReplaceAll(ts, ":", "-")
	destFile := filepath.Join(bakDir, bakPrefix+ts)
	if err := WriteFileAtomic(destFile, data, 0o644); err != nil {
		return fmt.Errorf("backup: write snapshot: %w", err)
	}

	// Rotate: remove oldest backup(s) if over the limit.
	if err := rotate(bakDir); err != nil {
		return fmt.Errorf("backup: rotate: %w", err)
	}

	// Update .gitignore managed block to include .devrune/backups/.
	if err := ensureGitignoreEntry(dir); err != nil {
		return fmt.Errorf("backup: update gitignore: %w", err)
	}

	return nil
}

// ListBackups returns all backup entries sorted newest-first.
// Returns an empty slice (not an error) if the backups directory does not exist.
func ListBackups(dir string) ([]BackupEntry, error) {
	bakDir := backupsDir(dir)
	entries, err := os.ReadDir(bakDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupEntry{}, nil
		}
		return nil, fmt.Errorf("backup: list backups: %w", err)
	}

	var backups []BackupEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, bakPrefix) {
			continue
		}
		tsPart := strings.TrimPrefix(name, bakPrefix)
		ts, err := parseTimestamp(tsPart)
		if err != nil {
			// Skip files with unrecognised timestamp format.
			continue
		}
		backups = append(backups, BackupEntry{
			Path:      filepath.Join(bakDir, name),
			Timestamp: ts,
			Name:      tsPart,
		})
	}

	// Sort newest-first.
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// rotate removes the oldest backup file(s) if the number of backups exceeds MaxBackups.
func rotate(bakDir string) error {
	entries, err := os.ReadDir(bakDir)
	if err != nil {
		return err
	}

	// Collect backup files only.
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), bakPrefix) {
			files = append(files, e.Name())
		}
	}

	// Files are already in lexicographic order from ReadDir, which equals
	// chronological order given the timestamp format (RFC3339 with hyphens).
	// To remove the OLDEST, remove the front entries.
	for len(files) > MaxBackups {
		oldest := filepath.Join(bakDir, files[0])
		if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
			return err
		}
		files = files[1:]
	}

	return nil
}

// parseTimestamp converts a backup filename timestamp part back to time.Time.
// Format is RFC3339 UTC with ":" replaced by "-" (e.g. "2026-05-04T14-30-00Z").
func parseTimestamp(s string) (time.Time, error) {
	// Restore colons in the time portion: after the "T" separator.
	tIdx := strings.Index(s, "T")
	if tIdx < 0 {
		return time.Time{}, fmt.Errorf("backup: invalid timestamp: %q", s)
	}
	datePart := s[:tIdx+1]
	timePart := s[tIdx+1:]
	// Replace first two "-" occurrences in the time portion with ":".
	timePart = strings.Replace(timePart, "-", ":", 2)
	restored := datePart + timePart
	return time.Parse(time.RFC3339, restored)
}

// ensureGitignoreEntry adds ".devrune/backups/" to the devrune managed block in
// the project's .gitignore. The operation is idempotent: if the entry already
// exists in the managed block nothing is written.
func ensureGitignoreEntry(projectDir string) error {
	const entry = ".devrune/backups/"

	gitignorePath := filepath.Join(projectDir, ".gitignore")
	existing, _ := os.ReadFile(gitignorePath)
	content := string(existing)

	beginIdx := strings.Index(content, gitignoreBeginMarker)
	endIdx := strings.Index(content, gitignoreEndMarker)

	if beginIdx >= 0 && endIdx >= 0 {
		// Managed block exists.
		blockContent := content[beginIdx : endIdx+len(gitignoreEndMarker)]
		if strings.Contains(blockContent, entry) {
			// Entry already present — nothing to do.
			return nil
		}
		// Insert the entry just before the end marker.
		insertAt := beginIdx + strings.Index(content[beginIdx:], gitignoreEndMarker)
		newContent := content[:insertAt] + entry + "\n" + content[insertAt:]
		return WriteFileAtomic(gitignorePath, []byte(newContent), 0o644)
	}

	// No managed block yet — append a new one containing .devrune/ and .devrune/backups/.
	var block strings.Builder
	if len(content) > 0 && !strings.HasSuffix(content, "\n\n") {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n"
	}
	block.WriteString(gitignoreBeginMarker + "\n")
	block.WriteString(".devrune/\n")
	block.WriteString(entry + "\n")
	block.WriteString(gitignoreEndMarker + "\n")
	return WriteFileAtomic(gitignorePath, []byte(content+block.String()), 0o644)
}
