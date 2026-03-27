package renderers

import "github.com/davidarce/devrune/internal/model"

// BuildWorkflowPlaceholderReplacements re-exports buildWorkflowPlaceholderReplacements
// for use by external test packages (package renderers_test).
// This is the standard Go pattern for test-only access to unexported symbols.
var BuildWorkflowPlaceholderReplacements = func(
	wf model.WorkflowManifest,
	workspaceDir string,
	skillDir string,
	modelResolver func(string) string,
	modelOverrides map[string]string,
) map[string]string {
	return buildWorkflowPlaceholderReplacements(wf, workspaceDir, skillDir, modelResolver, modelOverrides)
}

// BuildWorkflowPathReplacements re-exports buildWorkflowPathReplacements
// for use by external test packages.
var BuildWorkflowPathReplacements = buildWorkflowPathReplacements

// RemoveModelPlaceholderLines re-exports removeModelPlaceholderLines
// for use by external test packages.
var RemoveModelPlaceholderLines = removeModelPlaceholderLines

// ResolveModel re-exports resolveModel for use by external test packages.
var ResolveModel = resolveModel

// ResolveOpenCodeModel re-exports resolveOpenCodeModel for use by external test packages.
var ResolveOpenCodeModel = resolveOpenCodeModel
