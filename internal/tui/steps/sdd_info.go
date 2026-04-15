// SPDX-License-Identifier: MIT

package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// sddInfoFullContent is the full-width description shown on wide terminals.
const sddInfoFullContent = `Your agent uses SDD (Spec-Driven Development).

DevRune installs the SDD workflow to guide structured development:

  ① Explore    — Discover codebase context before making changes
  ② Plan       — Create a detailed implementation plan
  ③ Implement  — Execute the plan in reviewable batches
  ④ Review     — Verify against the plan before committing

Your agent will evaluate each task and apply SDD automatically
for new features and multi-file changes. Simple fixes skip SDD.`

// sddInfoShortContent is the compact description shown on narrow terminals.
const sddInfoShortContent = `Your agent uses SDD (Spec-Driven Development).
4 phases: Explore → Plan → Implement → Review.
Agent evaluates SDD before any code change.`

// sddInfoContent returns the appropriate description string for the given
// terminal width. Exposed for unit testing without a real TTY.
func sddInfoContent(termWidth int) string {
	return testableResponsiveDescription(sddInfoFullContent, sddInfoShortContent, termWidth)
}

// ShowSDDInfoStep renders the SDD workflow informational step.
// Always shown as step 2 in the wizard sequence.
// Returns huh.ErrUserAborted if the user presses Ctrl-C.
func ShowSDDInfoStep() error {
	content := responsiveDescription(sddInfoFullContent, sddInfoShortContent)

	confirmed := true

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(2, TotalSteps, "SDD Workflow"),
			huh.NewNote().
				Title("SDD Workflow").
				Description(content),
			huh.NewConfirm().
				Affirmative("Continue →").
				Negative("").
				Value(&confirmed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	return form.Run()
}
