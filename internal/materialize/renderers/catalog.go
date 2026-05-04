// SPDX-License-Identifier: MIT

package renderers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/davidarce/devrune/internal/model"
)

// CatalogBeginMarker is the opening delimiter for the DevRune-managed block
// in root catalog files (AGENTS.md, CLAUDE.md).
const CatalogBeginMarker = "# >>> devrune managed — do not edit"

// CatalogEndMarker is the closing delimiter for the DevRune-managed block
// in root catalog files.
const CatalogEndMarker = "# <<< devrune managed"

// RenderRootCatalog produces the agent-agnostic AGENTS.md content string.
// It emits an "## Available Workflows" section followed by a "## Conflict
// Resolution" section, then one "## {Name} Workflow" section per installed
// workflow with the per-workflow command table and registry content. Per-MCP
// agent-instruction sections are appended last.
//
// Skills, decision rules, invocation controls, and project rules are NOT
// rendered: agents discover skills/rules through their own runtime mechanisms,
// and the disambiguation tables duplicate signal that lives in skill bodies
// and advisor logic.
//
// The returned string does not include the managed block markers — the caller
// is responsible for wrapping via WriteManagedBlock.
//
// Parameters:
//   - workflows: installed workflow manifests
//   - mcpInstructions: map of MCP name → agent instructions text
//   - registryContents: map of workflow name → registry content to inject
//     (registry content should use H3 — "### Section" — so it nests correctly
//     under the per-workflow H2 parent)
func RenderRootCatalog(
	workflows []model.WorkflowManifest,
	mcpInstructions map[string]string,
	registryContents map[string]string,
) (string, error) {
	var sb strings.Builder

	// Workflows registry — contains highest-priority instructions
	// (e.g. SDD Evaluation Gate, Role Invariant, Memory Protocols).
	if len(workflows) > 0 {
		sb.WriteString("## Available Workflows\n\n")
		sb.WriteString("You have a set of workflows available within your instructions. Use them according to the context and the specific directions provided by the user.\n\n")

		// Conflict resolution table — declares per-workflow priority + commands so
		// the agent picks the right one when multiple could apply.
		sb.WriteString("## Conflict Resolution\n")
		sb.WriteString("When multiple workflows could apply, prefer the FIRST match in this priority table.\n")
		sb.WriteString("When the user explicitly names a workflow, use that one regardless of priority.\n\n")
		sb.WriteString("| Priority | Workflow | Commands |\n")
		sb.WriteString("|----------|----------|----------|\n")
		for i, wf := range workflows {
			cmds := make([]string, 0, len(wf.Components.Commands))
			for _, cmd := range wf.Components.Commands {
				cmds = append(cmds, "/"+cmd.Name)
			}
			cmdsStr := "—"
			if len(cmds) > 0 {
				cmdsStr = strings.Join(cmds, ", ")
			}
			_, _ = fmt.Fprintf(&sb, "| %d | %s | %s |\n", i+1, wf.Metadata.EffectiveDisplayName(), cmdsStr)
		}
		sb.WriteString("\n")

		// Per-workflow section — H2 parent so registry content (H3 children)
		// nests cleanly underneath.
		for _, wf := range workflows {
			_, _ = fmt.Fprintf(&sb, "## %s Workflow\n\n", wf.Metadata.EffectiveDisplayName())
			if len(wf.Components.Commands) > 0 {
				sb.WriteString("| Command | Action |\n")
				sb.WriteString("|---------|--------|\n")
				for _, cmd := range wf.Components.Commands {
					cmdStr := "/" + cmd.Name
					if cmd.Argument != "" {
						cmdStr += " " + cmd.Argument
					}
					_, _ = fmt.Fprintf(&sb, "| `%s` | %s |\n", cmdStr, cmd.Action)
				}
				sb.WriteString("\n")
			}

			// Inject workflow registry content if available.
			if content, ok := registryContents[wf.Metadata.Name]; ok && content != "" {
				sb.WriteString(content)
				if !strings.HasSuffix(content, "\n") {
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
		}
	}

	// Append MCP agent instructions sections (sorted for deterministic output).
	// Skip MCPs without instructions. If instructions already start with a ##
	// header, don't add a duplicate header.
	mcpNames := make([]string, 0, len(mcpInstructions))
	for name := range mcpInstructions {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)
	for _, name := range mcpNames {
		instructions := strings.TrimSpace(mcpInstructions[name])
		if instructions == "" {
			continue
		}
		if !strings.HasPrefix(instructions, "## ") {
			_, _ = fmt.Fprintf(&sb, "## %s\n\n", capitalizeFirst(name))
		}
		sb.WriteString(instructions)
		if !strings.HasSuffix(instructions, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
