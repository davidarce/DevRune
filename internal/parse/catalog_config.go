// SPDX-License-Identifier: MIT

package parse

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
)

// catalogConfigSchemaVersion is the only supported schema version for devrune.catalog.yaml.
const catalogConfigSchemaVersion = "devrune-catalog/v1"

// catalogConfigFileName is the conventional file name for catalog config files.
const catalogConfigFileName = "devrune.catalog.yaml"

// ParseCatalogConfig deserializes a devrune.catalog.yaml document from raw YAML bytes
// into a CatalogConfig. It validates the schema version and ensures sources is non-empty.
// Each source string is validated via model.ParseSourceRef for syntax.
//
// Returns an error if the YAML is malformed, the schema version is missing or unsupported,
// sources is empty, or any source ref has invalid syntax.
func ParseCatalogConfig(data []byte) (model.CatalogConfig, error) {
	var cfg model.CatalogConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return model.CatalogConfig{}, fmt.Errorf("catalog config: failed to parse YAML: %w", err)
	}

	if cfg.SchemaVersion == "" {
		return model.CatalogConfig{}, fmt.Errorf("catalog config: schemaVersion is required")
	}
	if cfg.SchemaVersion != catalogConfigSchemaVersion {
		return model.CatalogConfig{}, fmt.Errorf(
			"catalog config: unsupported schemaVersion %q (expected %q)",
			cfg.SchemaVersion, catalogConfigSchemaVersion,
		)
	}

	if len(cfg.Sources) == 0 {
		return model.CatalogConfig{}, fmt.Errorf("catalog config: sources must not be empty")
	}

	for i, src := range cfg.Sources {
		if _, err := model.ParseSourceRef(src, ""); err != nil {
			return model.CatalogConfig{}, fmt.Errorf("catalog config: sources[%d] %q: %w", i, src, err)
		}
	}

	return cfg, nil
}

// DetectCatalogConfig looks for a devrune.catalog.yaml file in dir and parses it if found.
//
// Returns:
//   - (nil, nil) if the file does not exist.
//   - (*CatalogConfig, nil) if the file exists and is valid.
//   - (nil, error) if the file exists but is malformed or fails validation.
func DetectCatalogConfig(dir string) (*model.CatalogConfig, error) {
	path := filepath.Join(dir, catalogConfigFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("catalog config: failed to read %s: %w", path, err)
	}

	cfg, err := ParseCatalogConfig(data)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
