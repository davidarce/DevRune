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
// All UserManifest fields — including CustomAdvisors and AdvisorCatalogs — are
// unmarshalled automatically via their yaml struct tags. No explicit field handling
// is required; gopkg.in/yaml.v3 populates every tagged field in the struct.
//
// Returns an error if the YAML is malformed, the schema version is not supported, or
// the manifest fails semantic validation (including duplicate CustomAdvisors names,
// name collisions with native advisors, and duplicate AdvisorCatalogs URLs).
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

	// Note: AdvisorDef.Scope is intentionally NOT loaded from devrune.yaml.
	// SKILL.md frontmatter on disk is the single source of truth for scope.
	// Callers that need scope load it via advisormeta.LoadNativeAdvisorScopes.

	if err := m.Validate(); err != nil {
		return model.UserManifest{}, err
	}

	return m, nil
}

// SerializeManifest serializes a UserManifest to canonical YAML bytes.
// The output is suitable for writing to devrune.yaml.
//
// Field ordering in the output matches the declaration order in model.UserManifest:
// CustomAdvisors and AdvisorCatalogs are emitted last (after Install) because they
// appear last in the struct. Both fields carry omitempty — they are omitted from the
// output when nil or empty, preserving byte-identical round-trips for manifests that
// do not use advisor management. No custom MarshalYAML implementation is required;
// gopkg.in/yaml.v3 respects struct field declaration order.
func SerializeManifest(m model.UserManifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("manifest: failed to serialize YAML: %w", err)
	}
	return data, nil
}
