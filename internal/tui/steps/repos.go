// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
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
// extraSources contains additional source ref strings (from catalogs: in devrune.yaml
// or --catalog flags) that are injected alongside knownSources as pre-selected
// options, deduplicated by value. The source ref string is used as the display label.
//
// preselected, when non-nil, restricts which sources start checked to those whose
// value appears in the preselected slice. Sources in preselected that are not
// present in the merged list are silently dropped (smart config merge: only
// preselect what still exists). Pass nil to start all known sources checked
// (default fresh-init behavior).
//
// Returns the combined list of selected predefined and custom sources, deduplicated.
// An empty result is valid (no repositories required).
func EnterRepositories(extraSources []string, preselected []string) ([]string, error) {
	// Phase A: Multi-select from predefined sources.
	// Convert extra source ref strings to knownSource entries (label = value = ref string).
	extra := make([]knownSource, 0, len(extraSources))
	for _, src := range extraSources {
		extra = append(extra, knownSource{label: src, value: src})
	}
	// Merge knownSources with extra, deduplicating by value.
	merged := mergeKnownSources(knownSources, extra)
	var predefined []string

	// Build preselection set. When nil, default all to selected (original behavior).
	preSet := make(map[string]bool, len(preselected))
	usePreselect := preselected != nil
	for _, p := range preselected {
		preSet[p] = true
	}

	options := make([]huh.Option[string], len(merged))
	for i, ks := range merged {
		isSelected := !usePreselect || preSet[ks.value]
		options[i] = huh.NewOption(ks.label, ks.value).Selected(isSelected)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			stepHeader(2, TotalSteps, "Repository sources"),
			huh.NewMultiSelect[string]().
				Title("Select repository catalogs").
				Description(responsiveDescription(
					"Use space to toggle, enter to confirm. You can add custom sources next.",
					"Space to toggle, enter to confirm.",
				)).
				Options(options...).
				Height(dynamicHeight(len(merged)+2)).
				Value(&predefined),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})
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
		).WithTheme(tuistyles.DevRuneThemeFunc).
			WithViewHook(func(v tea.View) tea.View {
				v.AltScreen = true
				return v
			})
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
					stepHeader(2, TotalSteps, "Repository sources"),
					huh.NewInput().
						Title(prompt).
						Description(desc).
						Placeholder("github:owner/repo@v1").
						Value(&src),
				),
			).WithTheme(tuistyles.DevRuneThemeFunc).
				WithViewHook(func(v tea.View) tea.View {
					v.AltScreen = true
					return v
				})
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
			).WithTheme(tuistyles.DevRuneThemeFunc).
				WithViewHook(func(v tea.View) tea.View {
					v.AltScreen = true
					return v
				})
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

// mergeKnownSources appends extra sources to base, skipping any entry whose
// value already appears in base (deduplication by value, order preserved).
func mergeKnownSources(base, extra []knownSource) []knownSource {
	if len(extra) == 0 {
		return base
	}

	seen := make(map[string]bool, len(base))
	for _, ks := range base {
		seen[ks.value] = true
	}

	result := make([]knownSource, len(base))
	copy(result, base)

	for _, ks := range extra {
		if !seen[ks.value] {
			seen[ks.value] = true
			result = append(result, ks)
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
