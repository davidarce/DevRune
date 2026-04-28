// SPDX-License-Identifier: MIT

package parse

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
)

// manifestSchemaVersion is the only supported schema version for devrune.yaml in MVP.
const manifestSchemaVersion = "devrune/v1"

// ParseManifest deserializes a devrune.yaml document from raw YAML bytes into a
// UserManifest. It validates the schema version and calls Validate() on the result.
//
// All UserManifest fields — including Advisors — are unmarshalled automatically
// via their yaml struct tags. No explicit field handling is required;
// gopkg.in/yaml.v3 populates every tagged field in the struct.
//
// Returns an error if the YAML is malformed, the schema version is not supported, or
// the manifest fails semantic validation (including duplicate Advisors source URLs
// and select entries that collide with native advisor names).
func ParseManifest(data []byte) (model.UserManifest, error) {
	var m model.UserManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return model.UserManifest{}, fmt.Errorf("manifest: failed to parse YAML: %w", err)
	}

	if m.SchemaVersion == "" {
		return model.UserManifest{}, fmt.Errorf("manifest: schemaVersion is required")
	}
	if m.SchemaVersion != manifestSchemaVersion {
		return model.UserManifest{}, fmt.Errorf(
			"manifest: unsupported schemaVersion %q (expected %q)",
			m.SchemaVersion, manifestSchemaVersion,
		)
	}

	// Note: AdvisorDef is the runtime view of an advisor (Name, Description,
	// Scope). It is NEVER loaded from devrune.yaml — SKILL.md frontmatter on
	// disk is the single source of truth, populated by advisorcatalog.Scanner
	// when each Advisors[] entry is resolved.

	if err := m.Validate(); err != nil {
		return model.UserManifest{}, err
	}

	return m, nil
}

// SerializeManifest serializes a UserManifest to canonical YAML bytes.
// The output is suitable for writing to devrune.yaml.
//
// Field ordering in the output matches the declaration order in model.UserManifest:
// Advisors is emitted last (after Install) because it appears last in the struct.
// The field carries omitempty — it is omitted from the output when nil or empty,
// preserving byte-identical round-trips for manifests that do not use advisor
// management. No custom MarshalYAML implementation is required; gopkg.in/yaml.v3
// respects struct field declaration order.
func SerializeManifest(m model.UserManifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("manifest: failed to serialize YAML: %w", err)
	}
	return data, nil
}
