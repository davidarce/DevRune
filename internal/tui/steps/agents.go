package steps

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// knownAgents lists the agents that DevRune can configure.
var knownAgents = []string{"claude", "opencode", "copilot", "factory"}

// SelectAgents presents a multi-select form for agent selection and returns
// the chosen agent names. At least one agent must be selected.
func SelectAgents() ([]string, error) {
	var selected []string

	options := make([]huh.Option[string], len(knownAgents))
	for i, a := range knownAgents {
		options[i] = huh.NewOption(a, a)
	}

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(1, 4, "Agent selection"),
			huh.NewMultiSelect[string]().
				Title("Which agents do you want to configure?").
				Description("Select one or more AI agents. Use space to toggle, enter to confirm.").
				Options(options...).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return fmt.Errorf("please select at least one agent")
					}
					return nil
				}).
				Value(&selected),
		),
	).WithProgramOptions(tea.WithAltScreen())

	if err := form.Run(); err != nil {
		return nil, err
	}

	return selected, nil
}
