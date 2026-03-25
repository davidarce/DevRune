package steps

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// EnterRepositories prompts the user to enter repository source refs one at a time.
// The user enters one source ref per prompt; they are asked "Add another?" after each.
// This avoids the huh textarea resize bug by using single-line inputs only.
// An empty input on first entry is valid (no repositories required).
func EnterRepositories() ([]string, error) {
	var sources []string

	for {
		var src string
		var addMore bool

		prompt := "Repository source ref"
		desc := "Enter a source ref (e.g. github:owner/repo@v1). Leave blank to skip."
		if len(sources) > 0 {
			prompt = fmt.Sprintf("Repository source ref #%d", len(sources)+1)
			desc = "Enter another source ref, or leave blank to finish."
		}

		inputForm := huh.NewForm(
			huh.NewGroup(
				stepHeader(2, 4, "Repository sources"),
				huh.NewInput().
					Title(prompt).
					Description(desc).
					Placeholder("github:owner/repo@v1").
					Value(&src),
			),
		).WithProgramOptions(tea.WithAltScreen())
		if err := inputForm.Run(); err != nil {
			return nil, err
		}

		trimmed := trimSpace(src)

		// If first entry and blank, allow skipping entirely.
		if trimmed == "" && len(sources) == 0 {
			return sources, nil
		}

		// If blank after at least one entry, stop.
		if trimmed == "" {
			break
		}

		sources = append(sources, trimmed)

		// Ask whether to add more.
		moreForm := huh.NewForm(
			huh.NewGroup(
				BannerNote(),
				huh.NewConfirm().
					Title("Add another repository?").
					Affirmative("Yes").
					Negative("No, continue").
					Value(&addMore),
			),
		).WithProgramOptions(tea.WithAltScreen())
		if err := moreForm.Run(); err != nil {
			return nil, err
		}

		if !addMore {
			break
		}
	}

	return sources, nil
}

// trimSpace removes leading/trailing whitespace from s.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
