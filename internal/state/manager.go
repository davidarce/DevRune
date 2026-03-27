// SPDX-License-Identifier: MIT

// Package state manages the .devrune/state.yaml workspace state file.
// It records managed paths, the active lock hash, installed agents, and
// active workflows so that reinstalls can cleanly remove previous artifacts.
package state

// StateManager provides read/write access to the .devrune/state.yaml workspace state.
// It also manages an advisory file lock to prevent concurrent installs.
type StateManager interface {
	// Read loads the current workspace state from disk.
	// Returns a zero-value State and a nil error if no state file exists yet.
	Read() (State, error)

	// Write persists the given state to disk, replacing any existing state file.
	Write(s State) error

	// ManagedPaths returns the list of workspace paths managed by the last install.
	// These paths are removed at the start of each subsequent install.
	ManagedPaths() ([]string, error)

	// AcquireLock acquires the advisory file lock for the install operation.
	// Returns an error if the lock cannot be acquired (e.g., another install is running).
	AcquireLock() error

	// ReleaseLock releases the advisory file lock.
	// Should be called via defer after AcquireLock succeeds.
	ReleaseLock() error
}

// State represents the contents of .devrune/state.yaml.
type State struct {
	SchemaVersion   string   `yaml:"schemaVersion"`
	LockHash        string   `yaml:"lockHash"`        // hash of the lockfile used for this install
	InstalledAt     string   `yaml:"installedAt"`     // ISO 8601 timestamp
	ManagedPaths    []string `yaml:"managedPaths"`    // paths created by this install (removed on next install)
	ActiveAgents    []string `yaml:"activeAgents"`    // agent names installed
	ActiveWorkflows []string `yaml:"activeWorkflows"` // workflow names installed
}
