// SPDX-License-Identifier: MIT

package detect

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// skipDirs contains directory names to skip during filesystem traversal.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".cache":       true,
}

// detectLanguages walks the directory tree and returns language statistics,
// total file count, and total line count.
func detectLanguages(dir string) ([]LanguageInfo, int, int, error) {
	langFiles := make(map[string]int)
	langLines := make(map[string]int)
	totalFiles := 0
	totalLines := 0

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process regular files
		if !d.Type().IsRegular() {
			return nil
		}

		// Read first 8KB for language detection
		content, readErr := readFirstBytes(path, 8192)
		if readErr != nil {
			return nil // skip unreadable files
		}

		lang := enry.GetLanguage(filepath.Base(path), content)
		if lang == "" || enry.IsVendor(path) || enry.IsDocumentation(path) {
			return nil
		}

		lines := countLines(content)
		totalFiles++
		totalLines += lines
		langFiles[lang]++
		langLines[lang] += lines

		return nil
	})
	if err != nil {
		return nil, 0, 0, err
	}

	// Build LanguageInfo slice
	langs := make([]LanguageInfo, 0, len(langFiles))
	for name, files := range langFiles {
		pct := 0.0
		if totalLines > 0 {
			pct = float64(langLines[name]) / float64(totalLines) * 100.0
		}
		langs = append(langs, LanguageInfo{
			Name:       name,
			Files:      files,
			Lines:      langLines[name],
			Percentage: pct,
		})
	}

	// Sort by lines descending
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Lines > langs[j].Lines
	})

	return langs, totalFiles, totalLines, nil
}

// readFirstBytes reads up to n bytes from a file.
func readFirstBytes(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:read], nil
}

// countLines counts the number of newline characters in a byte slice.
// Note: content is the first 8KB read from the file (see readFirstBytes), so
// counts are approximate for large files — intentional for performance.
func countLines(content []byte) int {
	return strings.Count(string(content), "\n") + 1
}
