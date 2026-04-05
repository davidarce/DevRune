// SPDX-License-Identifier: MIT

package detect

import (
	"fmt"
	"os"
)

// Analyze scans the project directory and returns a ProjectProfile.
// Uses go-enry for language detection and a lightweight walker for
// dependency and config file detection.
// Returns error only on unrecoverable issues (dir not found).
func Analyze(dir string) (*ProjectProfile, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("detect: directory not found: %w", err)
	}

	// Detect languages, total files, and total lines.
	languages, totalFiles, totalLines, err := detectLanguages(dir)
	if err != nil {
		// Non-fatal: return partial result with empty language list.
		languages = nil
	}

	// Detect dependency files.
	dependencies, err := detectDependencies(dir)
	if err != nil {
		// Non-fatal: return partial result with empty deps.
		dependencies = nil
	}

	// Detect config files.
	configFiles, err := detectConfigFiles(dir)
	if err != nil {
		// Non-fatal: return partial result with empty config list.
		configFiles = nil
	}

	// Infer frameworks from dependencies and config files.
	frameworks := inferFrameworks(dependencies, configFiles)

	return &ProjectProfile{
		Languages:    languages,
		Dependencies: dependencies,
		ConfigFiles:  configFiles,
		Frameworks:   frameworks,
		TotalFiles:   totalFiles,
		TotalLines:   totalLines,
	}, nil
}
