package steps

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// knownSource describes a predefined repository source that appears
// in the multi-select list during Step 2.
type knownSource struct {
	label string // human-readable display name
	value string // raw source ref string (e.g. "github:owner/repo")
}

// knownSources lists predefined repository catalog sources.
var knownSources = []knownSource{
	{label: "DevRune Starter Catalog", value: "github:davidarce/devrune-starter-catalog"},
}

// EnterRepositories presents predefined repository catalog sources for selection,
// then optionally allows adding custom source refs via free-form input.
// Returns the combined list of selected predefined and custom sources, deduplicated.
// An empty result is valid (no repositories required).
func EnterRepositories() ([]string, error) {
	// Phase A: Multi-select from predefined sources.
	var predefined []string

	options := make([]huh.Option[string], len(knownSources))
	for i, ks := range knownSources {
		options[i] = huh.NewOption(ks.label, ks.value)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			stepHeader(2, 4, "Repository sources"),
			huh.NewMultiSelect[string]().
				Title("Select repository catalogs").
				Description("Use space to toggle, enter to confirm. You can add custom sources next.").
				Options(options...).
				Value(&predefined),
		),
	).WithProgramOptions(tea.WithAltScreen())
	if err := selectForm.Run(); err != nil {
		return nil, err
	}

	// Phase B: Determine whether to show custom input loop.
	wantCustom := len(predefined) == 0
	if !wantCustom {
		confirmForm := huh.NewForm(
			huh.NewGroup(
				BannerNote(),
				huh.NewConfirm().
					Title("Add custom repository sources?").
					Affirmative("Yes").
					Negative("No, continue").
					Value(&wantCustom),
			),
		).WithProgramOptions(tea.WithAltScreen())
		if err := confirmForm.Run(); err != nil {
			return nil, err
		}
	}

	// Phase B continued: Custom input loop (same as original).
	var sources []string
	if wantCustom {
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

			// If first custom entry and blank, stop custom input.
			if trimmed == "" && len(sources) == 0 {
				break
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
	}

	return deduplicateSources(predefined, sources), nil
}

// deduplicateSources merges predefined and custom source lists, removing
// custom entries that duplicate a predefined value. Order is preserved:
// predefined sources first, then unique custom sources.
func deduplicateSources(predefined, custom []string) []string {
	if len(predefined) == 0 && len(custom) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(predefined))
	for _, s := range predefined {
		seen[s] = true
	}

	result := make([]string, len(predefined))
	copy(result, predefined)

	for _, s := range custom {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
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
