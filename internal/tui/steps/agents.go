// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// knownAgents lists the agents that DevRune can configure.
var knownAgents = []string{"claude", "codex", "opencode", "copilot", "factory"}

// SelectAgents presents a multi-select form for agent selection and returns
// the chosen agent names. At least one agent must be selected.
//
// preselected is an optional list of agent names that should start checked.
// Agents in preselected that are not in knownAgents are silently ignored.
// Pass nil for no preselection (fresh init).
func SelectAgents(preselected []string) ([]string, error) {
	var selected []string

	// Build a set for O(1) preselection lookup.
	preSet := make(map[string]bool, len(preselected))
	for _, p := range preselected {
		preSet[p] = true
	}

	options := make([]huh.Option[string], len(knownAgents))
	for i, a := range knownAgents {
		options[i] = huh.NewOption(a, a).Selected(preSet[a])
	}

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(1, TotalSteps, "Agent selection"),
			huh.NewMultiSelect[string]().
				Title("Which agents do you want to configure?").
				Description(responsiveDescription(
					"Select one or more AI agents. Use space to toggle, enter to confirm.",
					"Space to toggle, enter to confirm.",
				)).
				Options(options...).
				Height(dynamicHeight(len(knownAgents)+2)).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return fmt.Errorf("please select at least one agent")
					}
					return nil
				}).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return nil, err
	}

	return selected, nil
}
