// SPDX-License-Identifier: MIT

package detect

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectDependencies scans the directory for known dependency manifests and parses them.
func detectDependencies(dir string) ([]DependencyFile, error) {
	var results []DependencyFile

	// manifest candidates: root-level and one level deep
	candidates := []struct {
		relPath string
		dtype   string
		parser  func(string) (map[string]string, error)
	}{
		{"go.mod", "gomod", parseGoMod},
		{"package.json", "npm", parsePackageJSON},
		{"pom.xml", "maven", parsePomXML},
		{"build.gradle", "gradle", parseGradle},
		{"build.gradle.kts", "gradle", parseGradle},
		{"requirements.txt", "pip", parseRequirementsTxt},
		{"Pipfile", "pip", parsePipfile},
		{"Cargo.toml", "cargo", parseCargoToml},
	}

	for _, c := range candidates {
		// Check root
		fullPath := filepath.Join(dir, c.relPath)
		if df, err := tryParseDep(fullPath, c.relPath, c.dtype, c.parser); err == nil {
			results = append(results, df)
			continue
		}

		// Check one level deep
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			nested := filepath.Join(dir, entry.Name(), c.relPath)
			rel := filepath.Join(entry.Name(), c.relPath)
			if df, err := tryParseDep(nested, rel, c.dtype, c.parser); err == nil {
				results = append(results, df)
				break
			}
		}
	}

	return results, nil
}

func tryParseDep(fullPath, relPath, dtype string, parser func(string) (map[string]string, error)) (DependencyFile, error) {
	deps, err := parser(fullPath)
	if err != nil {
		return DependencyFile{}, err
	}
	return DependencyFile{Path: relPath, Type: dtype, Dependencies: deps}, nil
}

// parseGoMod parses require blocks from go.mod.
var goModRequireRe = regexp.MustCompile(`^\s*([\w./-]+)\s+(v[\w.\-+]+)`)

func parseGoMod(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	deps := make(map[string]string)
	inRequire := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "require (" {
			inRequire = true
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		// Single-line require
		if strings.HasPrefix(trimmed, "require ") {
			rest := strings.TrimPrefix(trimmed, "require ")
			if m := goModRequireRe.FindStringSubmatch(rest); m != nil {
				deps[m[1]] = m[2]
			}
			continue
		}
		if inRequire {
			if m := goModRequireRe.FindStringSubmatch(trimmed); m != nil {
				deps[m[1]] = m[2]
			}
		}
	}
	return deps, scanner.Err()
}

// parsePackageJSON extracts dependencies and devDependencies from package.json.
func parsePackageJSON(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	deps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		deps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		deps[k] = v
	}
	return deps, nil
}

// parsePomXML extracts groupId:artifactId from pom.xml <dependency> elements.
func parsePomXML(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	type Dep struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	}
	type Project struct {
		Dependencies []Dep `xml:"dependencies>dependency"`
	}
	var proj Project
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, err
	}
	deps := make(map[string]string)
	for _, d := range proj.Dependencies {
		key := d.GroupID + ":" + d.ArtifactID
		deps[key] = d.Version
	}
	return deps, nil
}

// parseGradle extracts implementation/api declarations from build.gradle files.
var gradleDepRe = regexp.MustCompile(`(?:implementation|api|testImplementation|runtimeOnly|compileOnly)\s+['"]([^'"]+)['"]`)

func parseGradle(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	deps := make(map[string]string)
	for _, m := range gradleDepRe.FindAllStringSubmatch(string(data), -1) {
		// format: group:artifact:version or group:artifact
		parts := strings.SplitN(m[1], ":", 3)
		if len(parts) >= 2 {
			key := parts[0] + ":" + parts[1]
			version := ""
			if len(parts) == 3 {
				version = parts[2]
			}
			deps[key] = version
		}
	}
	return deps, nil
}

// parseRequirementsTxt parses requirements.txt line by line.
var reqLineRe = regexp.MustCompile(`^([A-Za-z0-9_.\-]+)\s*(?:[=><~!]+\s*([^\s#]+))?`)

func parseRequirementsTxt(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	deps := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		if m := reqLineRe.FindStringSubmatch(line); m != nil {
			deps[m[1]] = m[2]
		}
	}
	return deps, scanner.Err()
}

// parsePipfile extracts packages from [packages] and [dev-packages] sections.
func parsePipfile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	deps := make(map[string]string)
	inPackages := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[packages]" || line == "[dev-packages]" {
			inPackages = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPackages = false
			continue
		}
		if inPackages && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			name := strings.TrimSpace(parts[0])
			ver := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			deps[name] = ver
		}
	}
	return deps, scanner.Err()
}

// parseCargoToml parses [dependencies] from Cargo.toml using simple line parsing.
func parseCargoToml(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	deps := make(map[string]string)
	inDeps := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[dependencies]" || line == "[dev-dependencies]" || line == "[build-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if inDeps && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			name := strings.TrimSpace(parts[0])
			ver := strings.Trim(strings.TrimSpace(parts[1]), `"'{}`)
			deps[name] = ver
		}
	}
	return deps, scanner.Err()
}
