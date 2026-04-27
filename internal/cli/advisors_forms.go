// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// advisorsAction represents a top-level action chosen from the sdd-advisors menu.
type advisorsAction int

const (
	advisorsActionToggle advisorsAction = iota
	advisorsActionAddAdvisor
	advisorsActionRefreshCatalogs
	advisorsActionQuit
)

// runTopLevelActionForm presents Screen 1: the main sdd-advisors menu.
// It returns the action the user selected.
func runTopLevelActionForm(rows []advisorRow, catalogCount int) (advisorsAction, error) {
	installed := 0
	for _, r := range rows {
		if r.Installed {
			installed++
		}
	}

	desc := fmt.Sprintf("Manage SDD advisors and advisor catalogs — %d installed, %d catalog(s)", installed, catalogCount)

	var selected advisorsAction
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewSelect[advisorsAction]().
				Title("What would you like to do?").
				Description(desc).
				Options(
					huh.NewOption("Toggle advisors (install / uninstall)", advisorsActionToggle),
					huh.NewOption("Add advisor (from local directory, github, or gitlab)", advisorsActionAddAdvisor),
					huh.NewOption("Refresh catalogs (re-fetch github/gitlab sources)", advisorsActionRefreshCatalogs),
					huh.NewOption("Quit", advisorsActionQuit),
				).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return 0, err
	}

	return selected, nil
}

// runToggleForm presents Screen 2: a multi-select where the user toggles which
// advisors are installed. It returns the computed diff:
//   - toAdd: advisors that were not installed and are now selected
//   - toRemove: advisors that were installed and are now deselected
func runToggleForm(rows []advisorRow) (toAdd, toRemove []string, err error) {
	// Build pre-installed set for diff computation.
	wasInstalled := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.Installed {
			wasInstalled[r.Name] = true
		}
	}

	// Build options; pre-select currently installed advisors.
	options := make([]huh.Option[string], len(rows))
	for i, r := range rows {
		var label string
		if len(r.Scope) == 0 {
			label = r.Name
		} else {
			label = r.Name + " (" + strings.Join(r.Scope, ", ") + ")"
		}
		options[i] = huh.NewOption(label, r.Name).Selected(r.Installed)
	}

	var selected []string

	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewMultiSelect[string]().
				Title("Toggle advisors").
				Description("Space toggles items. Enter submits the current selection. Esc cancels.").
				Options(options...).
				Height(steps.DynamicHeight(len(rows)+2)).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err = form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, nil, huh.ErrUserAborted
		}
		return nil, nil, err
	}

	// Compute diff.
	nowSelected := make(map[string]bool, len(selected))
	for _, name := range selected {
		nowSelected[name] = true
	}

	for _, r := range rows {
		switch {
		case nowSelected[r.Name] && !wasInstalled[r.Name]:
			toAdd = append(toAdd, r.Name)
		case !nowSelected[r.Name] && wasInstalled[r.Name]:
			toRemove = append(toRemove, r.Name)
		}
	}

	return toAdd, toRemove, nil
}
