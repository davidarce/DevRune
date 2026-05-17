// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/backup"
	"github.com/davidarce/devrune/internal/diff"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// RestoreResult holds metadata about a completed restore operation.
type RestoreResult struct {
	// RestoredEntry is the backup entry that was restored.
	RestoredEntry backup.BackupEntry
}

// restoreStep is the step key for the in-progress spinner model.
type restoreStepKind string

const (
	restoreStepBackup   restoreStepKind = "backup"
	restoreStepReplace  restoreStepKind = "replace"
	restoreStepInstall  restoreStepKind = "install"
)

// restorePhaseMsg signals a transition to the next restore step.
type restorePhaseMsg struct{ step restoreStepKind }

// restoreDoneMsg signals that the restore goroutine has completed.
type restoreDoneMsg struct{ err error }

// restoreModel is a bubbletea model for the restore progress spinner.
// It transitions through three sequential steps:
//  1. Creating pre-restore backup of current state
//  2. Replacing devrune.yaml with backup content
//  3. Running install
type restoreModel struct {
	spinner      spinner.Model
	phase        restoreStepKind // current phase
	done         bool
	err          error
	entry        backup.BackupEntry
	wd           string
	manifestPath string
	installFn    func() error
	// completed tracks which steps have finished (for display).
	completed map[restoreStepKind]bool
}

// newRestoreModel creates a fresh restoreModel.
func newRestoreModel(wd, manifestPath string, entry backup.BackupEntry, installFn func() error) restoreModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary)
	return restoreModel{
		spinner:      s,
		phase:        restoreStepBackup,
		wd:           wd,
		manifestPath: manifestPath,
		entry:        entry,
		installFn:    installFn,
		completed:    make(map[restoreStepKind]bool),
	}
}

func (m restoreModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.doBackup())
}

func (m restoreModel) doBackup() tea.Cmd {
	wd, manifestPath := m.wd, m.manifestPath
	return func() tea.Msg {
		if err := backup.CreateBackup(wd, manifestPath); err != nil {
			return restoreDoneMsg{err: fmt.Errorf("pre-restore backup failed: %w", err)}
		}
		return restorePhaseMsg{step: restoreStepReplace}
	}
}

func (m restoreModel) doReplace() tea.Cmd {
	manifestPath, entry := m.manifestPath, m.entry
	return func() tea.Msg {
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			return restoreDoneMsg{err: fmt.Errorf("read backup file: %w", err)}
		}
		if err := backup.WriteFileAtomic(manifestPath, data, 0o644); err != nil {
			return restoreDoneMsg{err: fmt.Errorf("write restored manifest: %w", err)}
		}
		return restorePhaseMsg{step: restoreStepInstall}
	}
}

func (m restoreModel) doInstall() tea.Cmd {
	fn := m.installFn
	return func() tea.Msg {
		if err := fn(); err != nil {
			return restoreDoneMsg{err: err}
		}
		return restoreDoneMsg{}
	}
}

func (m restoreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case restorePhaseMsg:
		// Mark previous phase complete.
		m.completed[m.phase] = true
		m.phase = msg.step
		switch msg.step {
		case restoreStepReplace:
			return m, m.doReplace()
		case restoreStepInstall:
			return m, m.doInstall()
		}
		return m, nil

	case restoreDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.completed[m.phase] = true
		m.done = true
		return m, tea.Quit

	case tea.KeyPressMsg:
		if m.err != nil {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m restoreModel) View() tea.View {
	var content string

	switch {
	case m.err != nil:
		content = m.viewError()
	case m.done:
		content = m.viewSuccess()
	default:
		content = m.viewProgress()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// stepLine renders one progress line: done (✓), active (spinner), or pending (dimmed).
func (m restoreModel) stepLine(step restoreStepKind, label string) string {
	switch {
	case m.completed[step]:
		checkmark := lipgloss.NewStyle().Foreground(tuistyles.ColorSuccess).Bold(true).Render("  ✓")
		text := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(" " + label)
		return checkmark + text
	case m.phase == step && !m.done:
		spin := lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary).Render("  " + m.spinner.View())
		text := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(" " + label)
		return spin + text
	default:
		return lipgloss.NewStyle().Foreground(tuistyles.ColorDim).Render("    " + label)
	}
}

func (m restoreModel) viewProgress() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.stepLine(restoreStepBackup, "Creating backup of current state..."))
	b.WriteString("\n")
	b.WriteString(m.stepLine(restoreStepReplace, "Replacing devrune.yaml..."))
	b.WriteString("\n")
	b.WriteString(m.stepLine(restoreStepInstall, "Running install..."))
	b.WriteString("\n")
	return b.String()
}

func (m restoreModel) viewSuccess() string {
	checkmark := lipgloss.NewStyle().Foreground(tuistyles.ColorSuccess).Bold(true).Render("  ✓")
	doneMsg := lipgloss.NewStyle().Foreground(tuistyles.ColorAccent).Render(" Restore complete")

	detail1 := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(
		fmt.Sprintf("  devrune.yaml restored to snapshot %s", m.entry.Name),
	)
	detail2 := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(
		"  Install completed successfully.",
	)
	hint := lipgloss.NewStyle().Foreground(tuistyles.ColorDim).Italic(true).Render(
		"  Press any key to return to the menu",
	)

	return fmt.Sprintf("\n%s%s\n\n%s\n%s\n\n%s\n", checkmark, doneMsg, detail1, detail2, hint)
}

func (m restoreModel) viewError() string {
	errLine := tuistyles.StyleError.Render("  Error")
	msgLine := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(
		fmt.Sprintf("  %s", m.err.Error()),
	)
	restoredNote := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(
		fmt.Sprintf("  devrune.yaml has been restored to snapshot %s.", m.entry.Name),
	)
	retryHint := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted).Render(
		"  You can retry from the menu using Sync project.",
	)
	hintLine := lipgloss.NewStyle().Foreground(tuistyles.ColorDim).Italic(true).Render(
		"  Press any key to exit",
	)
	return fmt.Sprintf("\n%s\n\n%s\n\n%s\n%s\n\n%s\n", errLine, msgLine, restoredNote, retryHint, hintLine)
}

// runRestoreSpinner runs the three-phase restore progress model.
func runRestoreSpinner(wd, manifestPath string, entry backup.BackupEntry, installFn func() error) error {
	m := newRestoreModel(wd, manifestPath, entry, installFn)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(restoreModel); ok {
		return fm.err
	}
	return nil
}

// executeRestore performs the full restore sequence without any TUI interaction.
// It is extracted as a pure function so that tests can inject a stub installFn.
// Steps: pre-restore backup → atomic write of backup content → installFn().
// If installFn fails, devrune.yaml has already been replaced — no auto-revert.
func executeRestore(wd, manifestPath string, entry backup.BackupEntry, installFn func() error) error {
	// Step 1: pre-restore backup of current state.
	if err := backup.CreateBackup(wd, manifestPath); err != nil {
		return fmt.Errorf("pre-restore backup failed: %w", err)
	}

	// Step 2: replace devrune.yaml with backup content.
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return fmt.Errorf("read backup file: %w", err)
	}
	if err := backup.WriteFileAtomic(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write restored manifest: %w", err)
	}

	// Step 3: run install. On failure, manifest is already replaced — no rollback.
	if err := installFn(); err != nil {
		return fmt.Errorf("install after restore: %w", err)
	}
	return nil
}

// renderDiff formats a []diff.DiffLine into a coloured string for display in a huh Note.
// Lines of kind "added" are green, "removed" red, "context" gray.
// At most maxDiffLines are shown; if more exist, a truncation indicator is appended.
func renderDiff(lines []diff.DiffLine) string {
	const maxDiffLines = 40

	addedStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorSuccess)
	removedStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorError)
	contextStyle := lipgloss.NewStyle().Foreground(tuistyles.ColorMuted)

	var b strings.Builder
	shown := 0
	for _, l := range lines {
		if shown >= maxDiffLines {
			b.WriteString(contextStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(lines)-maxDiffLines)))
			b.WriteString("\n")
			break
		}
		switch l.Kind {
		case "added":
			b.WriteString(addedStyle.Render(fmt.Sprintf("  + %s", l.Text)))
		case "removed":
			b.WriteString(removedStyle.Render(fmt.Sprintf("  - %s", l.Text)))
		default:
			b.WriteString(contextStyle.Render(fmt.Sprintf("    %s", l.Text)))
		}
		b.WriteString("\n")
		shown++
	}
	if b.Len() == 0 {
		b.WriteString(contextStyle.Render("  (no differences)"))
		b.WriteString("\n")
	}
	return b.String()
}

// RunRestoreStep runs the full backup restore TUI flow.
//
// Flow:
//  1. List backups (newest first, max 5). Empty state if zero entries.
//  2. User picks a backup entry via huh.Select.
//  3. Diff current devrune.yaml vs selected backup entry.
//  4. Show diff in a huh Note.
//  5. huh.Confirm — "Yes, restore it" / "Cancel".
//  6. Run restore + install inside a bubbletea progress spinner.
//
// Returns huh.ErrUserAborted if the user cancels at any step.
// Returns a descriptive error if backup creation, file write, or install fails.
func RunRestoreStep(wd, manifestPath string, installFn func() error) (RestoreResult, error) {
	// Step 1: list backups.
	entries, err := backup.ListBackups(wd)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("list backups: %w", err)
	}

	// Empty state: show informational message and return.
	if len(entries) == 0 {
		return RestoreResult{}, showEmptyBackupsMessage()
	}

	// Limit to 5 most recent entries (ListBackups already returns newest-first).
	if len(entries) > backup.MaxBackups {
		entries = entries[:backup.MaxBackups]
	}

	// Step 2: build select options (newest first, with (latest)/(oldest) labels).
	opts := make([]huh.Option[backup.BackupEntry], len(entries))
	for i, e := range entries {
		label := e.Name
		switch {
		case i == 0 && len(entries) == 1:
			label = e.Name + "   (only backup)"
		case i == 0:
			label = e.Name + "   (latest)"
		case i == len(entries)-1:
			label = e.Name + "   (oldest)"
		}
		opts[i] = huh.NewOption(label, e)
	}

	var selected backup.BackupEntry
	selectForm := huh.NewForm(
		huh.NewGroup(
			BannerNote(),
			huh.NewSelect[backup.BackupEntry]().
				Title("Backups — select a snapshot to restore").
				Options(opts...).
				Value(&selected),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := selectForm.Run(); err != nil {
		return RestoreResult{}, err
	}

	// Step 3: read current manifest and backup, compute diff.
	currentBytes, err := os.ReadFile(manifestPath)
	if err != nil && !os.IsNotExist(err) {
		return RestoreResult{}, fmt.Errorf("read current manifest: %w", err)
	}
	backupBytes, err := os.ReadFile(selected.Path)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("read backup file: %w", err)
	}

	diffLines := diff.Diff(currentBytes, backupBytes)
	diffText := renderDiff(diffLines)

	// Step 4: show diff preview + confirm.
	confirmed := true
	diffTitle := fmt.Sprintf("Diff — current vs. %s", selected.Name)
	confirmTitle := fmt.Sprintf("Restore devrune.yaml to snapshot %s?", selected.Name)
	confirmNote := "A backup of the current state will be created before restoring.\nInstall will run automatically after the restore."

	confirmForm := huh.NewForm(
		huh.NewGroup(
			BannerNote(),
			huh.NewNote().
				Title(diffTitle).
				Description(diffText),
			huh.NewNote().
				Title("Confirm Restore").
				Description(confirmNote),
			huh.NewConfirm().
				Title(confirmTitle).
				Affirmative("Yes, restore it").
				Negative("Cancel").
				Value(&confirmed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := confirmForm.Run(); err != nil {
		return RestoreResult{}, err
	}

	if !confirmed {
		return RestoreResult{}, huh.ErrUserAborted
	}

	// Step 5: run restore with spinner.
	if err := runRestoreSpinner(wd, manifestPath, selected, installFn); err != nil {
		return RestoreResult{}, err
	}

	return RestoreResult{RestoredEntry: selected}, nil
}

// showEmptyBackupsMessage displays a TUI message when no backups are available.
// Returns huh.ErrUserAborted when the user navigates back.
func showEmptyBackupsMessage() error {
	description := "No backups available.\nBackups are created automatically when running Setup or Sync."
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			BannerNote(),
			huh.NewNote().
				Title("Backups").
				Description(description),
			huh.NewSelect[string]().
				Options(huh.NewOption("Back to menu", "back")).
				Value(&action),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil && err != huh.ErrUserAborted {
		return err
	}
	// Treat "Back to menu" as a user abort so the caller returns cleanly.
	return huh.ErrUserAborted
}

