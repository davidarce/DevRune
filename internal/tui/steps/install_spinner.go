// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// installPhaseMsg signals a transition from the resolving phase to installing.
type installPhaseMsg struct{}

// installDoneMsg signals that the install goroutine has completed.
// err is non-nil if either resolveFn or installFn returned an error.
type installDoneMsg struct {
	err error
}

// installModel is a bubbletea model for the resolve/install spinner.
// It transitions through three states:
//  1. Resolving — spinner + "Resolving packages..." message
//  2. Installing — spinner + "Installing workspace..." message
//  3. Done or Error — final state, program exits on next key (error) or immediately (success)
type installModel struct {
	spinner   spinner.Model
	phase     string // "resolving" | "installing"
	done      bool
	err       error
	resolveFn func() error // resolve phase function
	installFn func() error // install phase function
}

// newInstallModel creates a fresh installModel starting in the resolving phase.
func newInstallModel(resolveFn func() error, installFn func() error) installModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary)
	return installModel{
		spinner:   s,
		phase:     "resolving",
		resolveFn: resolveFn,
		installFn: installFn,
	}
}

func (m installModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.doResolve(),
	)
}

// doResolve returns a Cmd that runs resolveFn and sends the appropriate next msg.
func (m installModel) doResolve() tea.Cmd {
	fn := m.resolveFn
	return func() tea.Msg {
		if err := fn(); err != nil {
			return installDoneMsg{err: err}
		}
		return installPhaseMsg{}
	}
}

// doInstall returns a Cmd that runs installFn and sends installDoneMsg.
func (m installModel) doInstall() tea.Cmd {
	fn := m.installFn
	return func() tea.Msg {
		if err := fn(); err != nil {
			return installDoneMsg{err: err}
		}
		return installDoneMsg{}
	}
}

func (m installModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case installPhaseMsg:
		m.phase = "installing"
		// Start the install phase command.
		return m, m.doInstall()

	case installDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyPressMsg:
		// Dismiss error screen on any keypress.
		if m.err != nil {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m installModel) View() tea.View {
	var content string

	switch {
	case m.err != nil:
		// Error state: show error inside alt-screen with a keypress prompt.
		errLine := tuistyles.StyleError.Render("  Error")
		msgLine := lipgloss.NewStyle().
			Foreground(tuistyles.ColorMuted).
			Render(fmt.Sprintf("  %s", m.err.Error()))
		hintLine := lipgloss.NewStyle().
			Foreground(tuistyles.ColorDim).
			Italic(true).
			Render("  Press any key to exit")

		content = fmt.Sprintf("\n%s\n\n%s\n\n%s\n", errLine, msgLine, hintLine)

	case m.done:
		checkmark := lipgloss.NewStyle().
			Foreground(tuistyles.ColorSuccess).
			Bold(true).
			Render("  ✓")
		doneMsg := lipgloss.NewStyle().
			Foreground(tuistyles.ColorAccent).
			Render(" Installation complete")
		content = fmt.Sprintf("\n%s%s\n", checkmark, doneMsg)

	default:
		var phaseMsg string
		switch m.phase {
		case "installing":
			phaseMsg = "Installing workspace..."
		default:
			phaseMsg = "Resolving packages..."
		}
		spinLine := lipgloss.NewStyle().
			Foreground(tuistyles.ColorMuted).
			Render(fmt.Sprintf("  %s %s", m.spinner.View(), phaseMsg))
		content = fmt.Sprintf("\n%s\n", spinLine)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// RunInstallSpinner runs a bubbletea spinner for the resolve+install phase.
// resolveFn and installFn are called sequentially via tea.Cmd functions.
// If either returns an error, the spinner transitions to an error state and
// renders the error inside the alt-screen. The user must press any key to exit.
// On success the spinner shows a checkmark and exits cleanly.
// The returned error is the first non-nil error from resolveFn or installFn,
// or nil on success.
func RunInstallSpinner(resolveFn func() error, installFn func() error) error {
	m := newInstallModel(resolveFn, installFn)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if fm, ok := finalModel.(installModel); ok {
		return fm.err
	}
	return nil
}
