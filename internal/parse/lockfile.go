package parse

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
)

// LockfileSchemaVersion is the only supported schema version for devrune.lock in MVP.
const LockfileSchemaVersion = "devrune/lock/v1"

// ParseLockfile deserializes a devrune.lock document from raw YAML bytes into a
// Lockfile model. It validates the schema version and calls Validate() on the result.
func ParseLockfile(data []byte) (model.Lockfile, error) {
	var l model.Lockfile
	if err := yaml.Unmarshal(data, &l); err != nil {
		return model.Lockfile{}, fmt.Errorf("lockfile: failed to parse YAML: %w", err)
	}

	if l.SchemaVersion == "" {
		return model.Lockfile{}, fmt.Errorf("lockfile: schemaVersion is required")
	}
	if l.SchemaVersion != LockfileSchemaVersion {
		return model.Lockfile{}, fmt.Errorf(
			"lockfile: unsupported schemaVersion %q (expected %q)",
			l.SchemaVersion, LockfileSchemaVersion,
		)
	}

	if err := l.Validate(); err != nil {
		return model.Lockfile{}, err
	}

	return l, nil
}

// SerializeLockfile serializes a Lockfile to deterministic YAML bytes.
//
// Determinism rules:
//   - Packages are sorted by their canonical source ref string (SourceRef.String()).
//   - Within each package, contents are sorted by (kind, name).
//   - MCPs are sorted by name.
//   - Workflows are sorted by name.
func SerializeLockfile(l model.Lockfile) ([]byte, error) {
	l = sortLockfile(l)

	data, err := yaml.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("lockfile: failed to serialize YAML: %w", err)
	}
	return data, nil
}

// sortLockfile returns a copy of the Lockfile with all slices sorted deterministically.
func sortLockfile(l model.Lockfile) model.Lockfile {
	// Sort packages by canonical source ref string.
	sorted := make([]model.LockedPackage, len(l.Packages))
	copy(sorted, l.Packages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Source.String() < sorted[j].Source.String()
	})

	// Sort contents within each package by (kind, name).
	for i := range sorted {
		contents := make([]model.ContentItem, len(sorted[i].Contents))
		copy(contents, sorted[i].Contents)
		sort.Slice(contents, func(a, b int) bool {
			if contents[a].Kind != contents[b].Kind {
				return string(contents[a].Kind) < string(contents[b].Kind)
			}
			return contents[a].Name < contents[b].Name
		})
		sorted[i].Contents = contents
	}

	// Sort MCPs by name.
	sortedMCPs := make([]model.LockedMCP, len(l.MCPs))
	copy(sortedMCPs, l.MCPs)
	sort.Slice(sortedMCPs, func(i, j int) bool {
		return sortedMCPs[i].Name < sortedMCPs[j].Name
	})

	// Sort workflows by name.
	sortedWF := make([]model.LockedWorkflow, len(l.Workflows))
	copy(sortedWF, l.Workflows)
	sort.Slice(sortedWF, func(i, j int) bool {
		return sortedWF[i].Name < sortedWF[j].Name
	})

	return model.Lockfile{
		SchemaVersion: l.SchemaVersion,
		ManifestHash:  l.ManifestHash,
		Packages:      sorted,
		MCPs:          sortedMCPs,
		Workflows:     sortedWF,
	}
}
