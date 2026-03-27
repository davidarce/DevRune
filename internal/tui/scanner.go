// SPDX-License-Identifier: MIT

package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidarce/devrune/internal/cache"
	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/resolve"
)

// ScannedRepo holds the scan results for one repository source ref.
type ScannedRepo struct {
	Source    string            // original source ref string
	Skills    []string          // discovered skill names
	Rules     []string          // discovered rule names
	MCPs      []string          // discovered MCP names (files in mcps/ dir)
	Workflows []string          // discovered workflow names (dirs with workflow.yaml)
	Descs     map[string]string // item name → description (for skills, workflows, MCPs)
	MCPFiles  map[string]string // MCP name → filename with extension (e.g. "engram" → "engram.yaml")
	Error     error             // scan error (nil if ok)
}

// ScanRepositories fetches and enumerates content from each source ref.
// It creates a shared cache at cachePath and returns one ScannedRepo per source.
// Errors from individual repos are recorded in ScannedRepo.Error and do not
// abort the whole scan.
func ScanRepositories(ctx context.Context, sources []string, cp string) ([]ScannedRepo, error) {
	cacheStore := cache.NewFileCacheStore(cp)
	githubFetcher := cache.NewGitHubFetcher("")
	gitlabFetcher := cache.NewGitLabFetcher("")
	localFetcher := cache.NewLocalFetcher()
	multiFetcher := cache.NewMultiFetcher(githubFetcher, gitlabFetcher, localFetcher)

	results := make([]ScannedRepo, 0, len(sources))

	for _, src := range sources {
		repo := ScannedRepo{Source: src, Descs: make(map[string]string)}

		sourceRef, err := model.ParseSourceRef(src, ".")
		if err != nil {
			repo.Error = fmt.Errorf("parse source ref: %w", err)
			results = append(results, repo)
			continue
		}

		data, err := multiFetcher.Fetch(ctx, sourceRef)
		if err != nil {
			repo.Error = fmt.Errorf("fetch: %w", err)
			results = append(results, repo)
			continue
		}

		dir, err := cacheStore.Store(sourceRef.CacheKey(), data)
		if err != nil {
			repo.Error = fmt.Errorf("cache: %w", err)
			results = append(results, repo)
			continue
		}

		items, err := resolve.EnumerateContents(dir)
		if err != nil {
			repo.Error = fmt.Errorf("enumerate: %w", err)
			results = append(results, repo)
			continue
		}

		for _, item := range items {
			switch item.Kind {
			case model.KindSkill:
				repo.Skills = append(repo.Skills, item.Name)
				// Extract description from SKILL.md frontmatter.
				if desc := readFrontmatterDesc(filepath.Join(dir, item.Path, "SKILL.md")); desc != "" {
					repo.Descs[item.Name] = desc
				}
			case model.KindRule:
				repo.Rules = append(repo.Rules, item.Name)
			}
		}

		// Discover MCPs: files in mcps/ directory.
		repo.MCPs, repo.MCPFiles = discoverMCPs(dir)

		// Discover workflows: subdirectories containing workflow.yaml.
		repo.Workflows = discoverWorkflows(dir)

		// Read workflow descriptions from workflow.yaml metadata.
		for _, wf := range repo.Workflows {
			if desc := readWorkflowDesc(filepath.Join(dir, "workflows", wf, "workflow.yaml")); desc != "" {
				repo.Descs[wf] = desc
			}
		}

		results = append(results, repo)
	}

	return results, nil
}

// readFrontmatterDesc reads a YAML frontmatter block from a markdown file and
// extracts the "description" field. Returns "" if not found or on error.
func readFrontmatterDesc(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		if !inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = true
			}
			continue
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		if strings.HasPrefix(line, "description:") {
			val := strings.TrimPrefix(line, "description:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// readWorkflowDesc reads the metadata.description field from a workflow.yaml file.
func readWorkflowDesc(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	inMetadata := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata {
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				break // left metadata block
			}
			if strings.HasPrefix(trimmed, "description:") {
				val := strings.TrimPrefix(trimmed, "description:")
				val = strings.TrimSpace(val)
				val = strings.Trim(val, `"'`)
				return val
			}
		}
	}
	return ""
}

// discoverMCPs returns the names and filename map of MCP config files in extractedDir/mcps/.
// Each *.yaml or *.json file becomes an MCP name (filename without extension).
// fileMap maps name → original filename (e.g. "engram" → "engram.yaml").
func discoverMCPs(extractedDir string) (names []string, fileMap map[string]string) {
	mcpsDir := filepath.Join(extractedDir, "mcps")
	entries, err := os.ReadDir(mcpsDir)
	if err != nil {
		return nil, nil
	}
	fileMap = make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fname := e.Name()
		if strings.HasPrefix(fname, ".") {
			continue
		}
		if strings.HasSuffix(fname, ".yaml") || strings.HasSuffix(fname, ".yml") || strings.HasSuffix(fname, ".json") {
			short := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(fname, ".json"), ".yml"), ".yaml")
			names = append(names, short)
			fileMap[short] = fname
		}
	}
	return names, fileMap
}

// discoverWorkflows returns the names of workflows found in extractedDir/workflows/.
// Each subdirectory that contains a workflow.yaml file is treated as a workflow.
func discoverWorkflows(extractedDir string) []string {
	wfDir := filepath.Join(extractedDir, "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		wfFile := filepath.Join(wfDir, name, "workflow.yaml")
		if _, err := os.Stat(wfFile); err == nil {
			names = append(names, name)
		}
	}
	return names
}
