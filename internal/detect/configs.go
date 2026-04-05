// SPDX-License-Identifier: MIT

package detect

import (
	"os"
	"path/filepath"
)

// knownConfigPatterns contains glob patterns (relative to project root) for well-known config files.
var knownConfigPatterns = []string{
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	".eslintrc",
	".eslintrc.js",
	".eslintrc.json",
	".eslintrc.yaml",
	".eslintrc.yml",
	"tsconfig.json",
	".prettierrc",
	".prettierrc.js",
	".prettierrc.json",
	"jest.config.js",
	"jest.config.ts",
	"jest.config.mjs",
	"vite.config.js",
	"vite.config.ts",
	"next.config.js",
	"next.config.mjs",
	"next.config.ts",
	"webpack.config.js",
	"webpack.config.ts",
	"Makefile",
	".env",
	"tailwind.config.js",
	"tailwind.config.ts",
	"tailwind.config.mjs",
}

// detectConfigFiles checks for known config files in the project root and returns
// their paths relative to dir.
func detectConfigFiles(dir string) ([]string, error) {
	var found []string

	// Check root-level patterns
	for _, pattern := range knownConfigPatterns {
		path := filepath.Join(dir, pattern)
		if _, err := os.Stat(path); err == nil {
			found = append(found, pattern)
		}
	}

	// Check .github/workflows/*.yml
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				name := entry.Name()
				if filepath.Ext(name) == ".yml" || filepath.Ext(name) == ".yaml" {
					found = append(found, filepath.Join(".github", "workflows", name))
				}
			}
		}
	}

	return found, nil
}
