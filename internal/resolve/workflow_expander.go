package resolve

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// workflowYAML is the conventional filename for workflow manifests within a
// workflow source package.
const workflowYAML = "workflow.yaml"

// ExpandWorkflows validates and expands workflow sources in the manifest.
//
// In DevRune v3.4 the WorkflowManifest (workflow.yaml) is a pure metadata
// document — it does NOT declare package dependencies. There is nothing to
// merge into the user manifest beyond what is already declared by the user.
//
// ExpandWorkflows parses and validates each workflow source ref, then returns
// the manifest unchanged. No network calls are made here; actual fetching and
// caching is performed later by the Resolver's resolveWorkflows step.
//
// baseDir is used to resolve relative local paths in source refs.
// Returns an error if any workflow source ref is malformed.
func ExpandWorkflows(_ context.Context, manifest model.UserManifest, _ Fetcher, baseDir string) (model.UserManifest, error) {
	for _, wfSource := range manifest.Workflows {
		if _, err := model.ParseSourceRef(wfSource, baseDir); err != nil {
			return model.UserManifest{}, fmt.Errorf("expand workflows: invalid source ref %q: %w", wfSource, err)
		}
	}
	return manifest, nil
}

// extractAndParseWorkflow reads workflow.yaml from the gzip-compressed tar archive.
// wfSource is used only in error messages.
// Returns the parsed WorkflowManifest and the relative directory path (within the
// extracted archive) where workflow.yaml was found (e.g. "workflows/sdd" for a
// catalog archive, or "" for a standalone workflow archive).
func extractAndParseWorkflow(data []byte, wfSource string) (model.WorkflowManifest, string, error) {
	files, err := extractFilesFromTar(data)
	if err != nil {
		return model.WorkflowManifest{}, "", fmt.Errorf("extract archive: %w", err)
	}

	// Look for workflow.yaml at any depth (strip first component like file_store).
	for name, content := range files {
		base := stripFirstPathComponent(name)
		if base == "" {
			continue
		}
		if filepath.Base(filepath.FromSlash(base)) == workflowYAML {
			wf, err := parse.ParseWorkflow(content)
			if err != nil {
				return model.WorkflowManifest{}, "", fmt.Errorf("workflow %q: %w", wfSource, err)
			}
			// Compute the directory containing workflow.yaml (relative to archive root).
			// filepath.Dir("workflow.yaml") == "." → normalise to "".
			dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(base)))
			if dir == "." {
				dir = ""
			}
			return wf, dir, nil
		}
	}

	return model.WorkflowManifest{}, "", fmt.Errorf("workflow %q: no workflow.yaml found in archive", wfSource)
}

// stripFirstPathComponent removes the first "/" separated component from path.
// Returns empty string for single-component paths (the root directory itself).
func stripFirstPathComponent(p string) string {
	p = filepath.ToSlash(p)
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	idx := -1
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ""
	}
	return p[idx+1:]
}
