// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ConfirmSummary displays a summary of all selections and asks the user to confirm.
// Returns the populated UserManifest on confirmation, or ErrUserAborted (from huh)
// if the user declines.
//
// workflowModels is the optional map of agent→role→model from the workflow model selection step.
// May be nil if the step was skipped or no models were selected.
func ConfirmSummary(agents []string, selection SelectionResult, workflowModels map[string]map[string]string, catalogSources []string) (model.UserManifest, error) {
	description := buildSelectionSummary(agents, selection, workflowModels, catalogSources)

	confirmed := true

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(TotalSteps, TotalSteps, "Confirm"),
			huh.NewNote().
				Title("Summary").
				Description(description),
			huh.NewConfirm().
				Title("Create devrune.yaml with these settings?").
				Affirmative("Yes, create it").
				Negative("Cancel").
				Value(&confirmed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return model.UserManifest{}, err
	}

	if !confirmed {
		return model.UserManifest{}, huh.ErrUserAborted
	}

	return buildManifestFromSelection(agents, selection, workflowModels, catalogSources), nil
}

// buildSelectionSummary produces a human-readable description of the selections.
func buildSelectionSummary(agents []string, selection SelectionResult, workflowModels map[string]map[string]string, catalogSources []string) string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "Agents: %s\n\n", formatList(agents, "(none)"))

	for _, repo := range selection.Repos {
		_, _ = fmt.Fprintf(&b, "%s:\n", repo.Source)

		if len(repo.SelectedSkills) > 0 || len(repo.SelectedRules) > 0 ||
			len(repo.SelectedMCPs) > 0 || len(repo.SelectedWorkflows) > 0 {

			writeCountLine(&b, "  Skills", repo.SelectedSkills)
			writeCountLine(&b, "  Rules", repo.SelectedRules)
			writeCountLine(&b, "  MCPs", repo.SelectedMCPs)

			if len(repo.SelectedWorkflows) > 0 {
				_, _ = fmt.Fprintf(&b, "  Workflows: %s\n", strings.Join(repo.SelectedWorkflows, ", "))
			}
		} else {
			b.WriteString("  (nothing selected)\n")
		}
		b.WriteString("\n")
	}

	// Append workflow model selections when non-nil.
	if len(workflowModels) > 0 {
		b.WriteString("Workflow Models:\n")
		// Sort agent names for deterministic output.
		agentNames := make([]string, 0, len(workflowModels))
		for agentName := range workflowModels {
			agentNames = append(agentNames, agentName)
		}
		sort.Strings(agentNames)
		for _, agentName := range agentNames {
			roleModels := workflowModels[agentName]
			if len(roleModels) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(&b, "  %s:\n", agentName)
			// Sort role names for deterministic output.
			roleNames := make([]string, 0, len(roleModels))
			for roleName := range roleModels {
				roleNames = append(roleNames, roleName)
			}
			sort.Strings(roleNames)
			for _, roleName := range roleNames {
				if m := roleModels[roleName]; m != "" {
					_, _ = fmt.Fprintf(&b, "    %s: %s\n", roleName, m)
				}
			}
		}
		b.WriteString("\n")
	}

	// Append catalog sources when configured.
	if len(catalogSources) > 0 {
		_, _ = fmt.Fprintf(&b, "Catalogs: %s\n\n", strings.Join(catalogSources, ", "))
	}

	return b.String()
}

// writeCountLine writes a "  Label: N" line if items is non-empty.
func writeCountLine(b *strings.Builder, label string, items []string) {
	if len(items) > 0 {
		_, _ = fmt.Fprintf(b, "%s: %d\n", label, len(items))
	}
}

// formatList formats a string slice for display; returns fallback when empty.
func formatList(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	return strings.Join(items, ", ")
}

// buildManifestFromSelection constructs a UserManifest from the wizard selections.
func buildManifestFromSelection(agents []string, selection SelectionResult, workflowModels map[string]map[string]string, catalogSources []string) model.UserManifest {
	agentRefs := make([]model.AgentRef, 0, len(agents))
	for _, a := range agents {
		agentRefs = append(agentRefs, model.AgentRef{Name: a})
	}

	var pkgRefs []model.PackageRef
	var mcpRefs []model.MCPRef
	workflowEntries := make(map[string]model.WorkflowEntry)

	for _, repo := range selection.Repos {
		if len(repo.SelectedSkills) == 0 && len(repo.SelectedRules) == 0 &&
			len(repo.SelectedMCPs) == 0 && len(repo.SelectedWorkflows) == 0 {
			continue
		}

		// Build SelectFilter only if not all items are selected.
		// For simplicity in MVP, always include a SelectFilter with chosen items.
		// A nil filter means "all items"; explicit lists mean "only these".
		var sel *model.SelectFilter
		if len(repo.SelectedSkills) > 0 || len(repo.SelectedRules) > 0 {
			sel = &model.SelectFilter{
				Skills: repo.SelectedSkills,
				Rules:  repo.SelectedRules,
			}
		}

		pkgRefs = append(pkgRefs, model.PackageRef{
			Source: repo.Source,
			Select: sel,
		})

		// MCPs from this repo become separate MCPRef entries.
		for _, mcp := range repo.SelectedMCPs {
			mcpSrc := buildMCPSourceRef(repo.Source, mcp, repo.MCPFiles)
			mcpRefs = append(mcpRefs, model.MCPRef{Source: mcpSrc})
		}

		// Workflows from this repo become WorkflowEntry values in the map.
		// The workflow model overrides (workflowModels) are attached to every workflow entry.
		for _, wf := range repo.SelectedWorkflows {
			wfSrc := appendSubpath(repo.Source, "workflows/"+wf)
			entry := model.WorkflowEntry{Source: wfSrc}
			if len(workflowModels) > 0 {
				entry.Roles = workflowModels
			}
			workflowEntries[wf] = entry
		}
	}

	var workflows map[string]model.WorkflowEntry
	if len(workflowEntries) > 0 {
		workflows = workflowEntries
	}

	return model.UserManifest{
		SchemaVersion: "devrune/v1",
		Agents:        agentRefs,
		Packages:      pkgRefs,
		MCPs:          mcpRefs,
		Workflows:     workflows,
		Catalogs:      catalogSources,
	}
}

// appendSubpath appends a subpath to a source ref string.
// For remote sources (github/gitlab), it uses the "//" separator convention.
// For local sources, it appends as a filesystem path.
func appendSubpath(source, subpath string) string {
	if strings.HasPrefix(source, "local:") {
		// For local sources, join as a filesystem path.
		path := strings.TrimPrefix(source, "local:")
		return "local:" + path + "/" + subpath
	}
	if idx := strings.Index(source, "//"); idx >= 0 {
		// Replace existing subpath.
		return source[:idx+2] + subpath
	}
	return source + "//" + subpath
}

// buildMCPSourceRef constructs the source ref for an individual MCP.
// For local sources, uses the full path including file extension.
// For remote sources, uses the // subpath convention with the short name.
func buildMCPSourceRef(repoSource, mcpName string, mcpFiles map[string]string) string {
	if strings.HasPrefix(repoSource, "local:") {
		// Use the full filename if known (e.g. "engram.yaml").
		filename := mcpName + ".yaml" // fallback
		if mcpFiles != nil {
			if f, ok := mcpFiles[mcpName]; ok {
				filename = f
			}
		}
		path := strings.TrimPrefix(repoSource, "local:")
		return "local:" + path + "/mcps/" + filename
	}
	return appendSubpath(repoSource, "mcps/"+mcpName)
}
