// SPDX-License-Identifier: MIT

package model

// CatalogConfig represents the contents of a devrune.catalog.yaml file.
// It declares source refs that should be pre-loaded into the init wizard
// as pre-selected catalog sources.
//
// Example devrune.catalog.yaml:
//
//	schemaVersion: devrune-catalog/v1
//	sources:
//	  - github:davidarce/devrune-starter-catalog
//	  - github:myorg/custom-catalog@v2
type CatalogConfig struct {
	SchemaVersion string   `yaml:"schemaVersion"` // must be "devrune-catalog/v1"
	Sources       []string `yaml:"sources"`       // source ref strings (github:owner/repo@ref)
}
