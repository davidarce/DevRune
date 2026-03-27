// SPDX-License-Identifier: MIT

package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	stateSchemaVersion = "devrune/state/v1"
	stateDir           = ".devrune"
	stateFileName      = "state.yaml"
	lockFileName       = "install.lock"
)

// FileStateManager implements StateManager using a YAML file at .devrune/state.yaml
// and an advisory lock file at .devrune/locks/install.lock.
type FileStateManager struct {
	baseDir  string   // project root directory (where .devrune/ lives)
	lockFile *os.File // held during Install operations
}

// NewFileStateManager creates a FileStateManager rooted at the given project directory.
func NewFileStateManager(baseDir string) *FileStateManager {
	return &FileStateManager{baseDir: baseDir}
}

// statePath returns the absolute path to .devrune/state.yaml.
func (m *FileStateManager) statePath() string {
	return filepath.Join(m.baseDir, stateDir, stateFileName)
}

// lockPath returns the absolute path to .devrune/locks/install.lock.
func (m *FileStateManager) lockPath() string {
	return filepath.Join(m.baseDir, stateDir, "locks", lockFileName)
}

// Read loads the workspace state from .devrune/state.yaml.
// Returns a zero-value State (not an error) if the file does not exist yet.
func (m *FileStateManager) Read() (State, error) {
	data, err := os.ReadFile(m.statePath())
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("state: read %q: %w", m.statePath(), err)
	}

	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("state: parse %q: %w", m.statePath(), err)
	}
	return s, nil
}

// Write persists the given state to .devrune/state.yaml.
// The parent directory is created if it does not exist.
func (m *FileStateManager) Write(s State) error {
	if s.SchemaVersion == "" {
		s.SchemaVersion = stateSchemaVersion
	}
	if s.InstalledAt == "" {
		s.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}

	stateDir := filepath.Dir(m.statePath())
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("state: mkdir %q: %w", stateDir, err)
	}
	if err := os.WriteFile(m.statePath(), data, 0o644); err != nil {
		return fmt.Errorf("state: write %q: %w", m.statePath(), err)
	}
	return nil
}

// ManagedPaths reads the state file and returns the list of managed paths.
// Returns an empty slice (not an error) if no state exists yet.
func (m *FileStateManager) ManagedPaths() ([]string, error) {
	s, err := m.Read()
	if err != nil {
		return nil, err
	}
	return s.ManagedPaths, nil
}

// AcquireLock creates the advisory lock file at .devrune/locks/install.lock.
// Returns an error if the lock file already exists (another install may be running).
//
// Note: This is an advisory lock — it prevents concurrent devrune installs but
// does not prevent manual filesystem modifications.
func (m *FileStateManager) AcquireLock() error {
	lockDir := filepath.Dir(m.lockPath())
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("state: lock dir: %w", err)
	}

	f, err := os.OpenFile(m.lockPath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("state: another install is already running (lock: %s)", m.lockPath())
		}
		return fmt.Errorf("state: acquire lock: %w", err)
	}
	// Write the PID so it can be inspected if needed.
	_, _ = fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	m.lockFile = f
	return nil
}

// ReleaseLock removes the advisory lock file.
func (m *FileStateManager) ReleaseLock() error {
	if m.lockFile != nil {
		_ = m.lockFile.Close()
		m.lockFile = nil
	}
	if err := os.Remove(m.lockPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("state: release lock: %w", err)
	}
	return nil
}
