// SPDX-License-Identifier: MIT

package materialize

import (
	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
)

// AdvisorRenderer is an optional capability port that renderers may implement
// to support lightweight advisor file management without a full resolve cycle.
//
// SyncAdvisors performs a type assertion on each registered AgentRenderer; if
// the assertion succeeds the renderer participates in the sync. Renderers that
// do not implement AdvisorRenderer (OpenCode, Codex, Factory) are silently
// skipped — they do not emit advisor wrappers today.
//
// Detection rule (applied identically by every implementing renderer):
//
//	hasAdvisorSuffix := strings.HasSuffix(strings.ToLower(item.Name), "-advisor")
//	hasLegacySuffix  := strings.HasSuffix(strings.ToLower(item.Name), "-adviser") // compat shim
//	isAdvisor        := hasAdvisorSuffix || hasLegacySuffix || item.Custom
//
// The legacy "-adviser" branch is gated behind a log-once deprecation warning
// (via log.Printf on first detection per process) and will be removed in the
// next minor release.
//
// Statelessness requirement: RegenerateAdvisorFiles MUST be stateless on the
// receiver. Two consecutive calls with different installed sets MUST NOT leak
// state between invocations (i.e. no "r.installedSkills = installed"
// assignment on the receiver).
type AdvisorRenderer interface {
	// RegenerateAdvisorFiles writes or removes advisor agent files for the
	// given workspace.
	//
	//   workspaceRoot  — absolute path to the agent workspace directory
	//                    (e.g. /abs/path/.claude for Claude, /abs/path/.github for Copilot).
	//                    Callers must resolve this from the git repo root using the
	//                    renderer's WorkspacePaths().Workspace field.
	//   installed      — the full set of ContentItems that should result in
	//                    advisor agent files after this call. Non-advisor
	//                    items are ignored silently.
	//   removed        — skill names whose agent files must be deleted (may
	//                    overlap with previously installed items the caller
	//                    has already removed from the manifest).
	//   modelOverrides — optional per-skill model overrides keyed by skill
	//                    name (e.g. "architect-advisor" → "claude-opus-4").
	//                    nil is a valid value meaning "use renderer defaults".
	//
	// The method returns an AdvisorRenderResult listing the absolute paths it
	// wrote and deleted. If ANY write or delete fails, the method returns a
	// non-nil error; partial results are still populated so callers can
	// report which paths succeeded.
	RegenerateAdvisorFiles(
		workspaceRoot string,
		installed []model.ContentItem,
		removed []string,
		modelOverrides map[string]string,
	) (matypes.AdvisorRenderResult, error)
}
