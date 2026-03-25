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
// Returns an error if the YAML is malformed, the schema version is not supported, or
// the manifest fails semantic validation.
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

	if err := m.Validate(); err != nil {
		return model.UserManifest{}, err
	}

	return m, nil
}

// SerializeManifest serializes a UserManifest to canonical YAML bytes.
// The output is suitable for writing to devrune.yaml.
func SerializeManifest(m model.UserManifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("manifest: failed to serialize YAML: %w", err)
	}
	return data, nil
}
