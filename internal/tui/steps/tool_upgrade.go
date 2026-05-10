// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ---------------------------------------------------------------------------
// T012 — Types and parallel upgrade engine
// ---------------------------------------------------------------------------

// ToolUpgradeStatus represents the outcome of upgrading a single tool.
type ToolUpgradeStatus string

const (
	// ToolUpgradeOK indica que el comando de upgrade completó sin errores.
	ToolUpgradeOK ToolUpgradeStatus = "ok"
	// ToolUpgradeFail indica que el comando de upgrade retornó un error.
	ToolUpgradeFail ToolUpgradeStatus = "fail"
)

// ToolUpgradeResult holds the outcome for a single tool upgrade.
type ToolUpgradeResult struct {
	Name   string
	Status ToolUpgradeStatus
	Error  string
}

// ToolUpgradeSummary aggregates results for all executed tool upgrades.
// Results are in stable input order (non-upgradable tools are excluded).
type ToolUpgradeSummary struct {
	Results []ToolUpgradeResult
}

// ToolCommandExecutor is a function that runs a shell command string and
// returns nil on success or an error on failure. Inject a test double to
// avoid executing real commands in unit tests.
type ToolCommandExecutor func(command string) error

// upgradeToolItem is the internal representation of a single tool during
// the upgrade flow. Upgradable is false when no effective command exists.
type upgradeToolItem struct {
	Name       string
	Command    string
	Upgradable bool
}

// defaultToolCommandExecutor runs the given command string via `sh -c`.
func defaultToolCommandExecutor(command string) error {
	cmd := exec.Command("sh", "-c", command) //nolint:gosec
	return cmd.Run()
}

// upgradeToolsParallel runs each upgradable item concurrently and returns a
// ToolUpgradeSummary whose Results slice is ordered to match the input slice
// (non-upgradable items are omitted from Results entirely).
//
// If exec is nil, defaultToolCommandExecutor is used.
func upgradeToolsParallel(items []upgradeToolItem, execFn ToolCommandExecutor) ToolUpgradeSummary {
	if execFn == nil {
		execFn = defaultToolCommandExecutor
	}

	// Collect only upgradable items, preserving relative order.
	type indexedItem struct {
		idx  int
		item upgradeToolItem
	}
	var upgradable []indexedItem
	for i, it := range items {
		if it.Upgradable {
			upgradable = append(upgradable, indexedItem{idx: i, item: it})
		}
	}

	results := make([]ToolUpgradeResult, len(upgradable))
	var wg sync.WaitGroup
	ch := make(chan struct {
		pos int
		res ToolUpgradeResult
	}, len(upgradable))

	for pos, ii := range upgradable {
		wg.Add(1)
		go func(pos int, it upgradeToolItem) {
			defer wg.Done()
			err := execFn(it.Command)
			res := ToolUpgradeResult{Name: it.Name}
			if err != nil {
				res.Status = ToolUpgradeFail
				res.Error = err.Error()
			} else {
				res.Status = ToolUpgradeOK
			}
			ch <- struct {
				pos int
				res ToolUpgradeResult
			}{pos: pos, res: res}
		}(pos, ii.item)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		results[r.pos] = r.res
	}

	return ToolUpgradeSummary{Results: results}
}

// ---------------------------------------------------------------------------
// T013 — buildUpgradeToolItems helper (pure, exported for T016 tests)
// ---------------------------------------------------------------------------

// buildUpgradeToolItems resolves the effective upgrade command for each tool
// reference and classifies it as upgradable or not.
//
// Resolution order:
//  1. strings.TrimSpace(ref.Command) non-empty → use it.
//  2. strings.TrimSpace(catalog[ref.Name].Command) non-empty → use it.
//  3. otherwise → Upgradable = false.
func buildUpgradeToolItems(tools []model.ToolRef, catalog map[string]model.ToolDef) []upgradeToolItem {
	items := make([]upgradeToolItem, len(tools))
	for i, ref := range tools {
		cmd := strings.TrimSpace(ref.Command)
		if cmd == "" {
			if def, ok := catalog[ref.Name]; ok {
				cmd = strings.TrimSpace(def.Command)
			}
		}
		items[i] = upgradeToolItem{
			Name:       ref.Name,
			Command:    cmd,
			Upgradable: cmd != "",
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// T013 — RunToolUpgradeStep (public entry point)
// ---------------------------------------------------------------------------

// RunToolUpgradeStep presents the upgrade-tools TUI flow:
//  1. Empty state when len(tools)==0.
//  2. Preview list (upgradable vs. "no upgradable").
//  3. Explicit yes/no confirmation.
//  4. Parallel upgrade with spinner (T014).
//  5. Result summary.
//
// Returns an empty summary and nil error when the user cancels or there is
// nothing to run.
func RunToolUpgradeStep(tools []model.ToolRef, catalog map[string]model.ToolDef) (ToolUpgradeSummary, error) {
	return runToolUpgradeStepWithExecutor(tools, catalog, nil)
}

// runToolUpgradeStepWithExecutor is the testable variant that accepts an
// injected executor.
func runToolUpgradeStepWithExecutor(
	tools []model.ToolRef,
	catalog map[string]model.ToolDef,
	execFn ToolCommandExecutor,
) (ToolUpgradeSummary, error) {

	// ── Empty state ──────────────────────────────────────────────────────────
	if len(tools) == 0 {
		return showUpgradeEmptyState()
	}

	// ── Build items ──────────────────────────────────────────────────────────
	items := buildUpgradeToolItems(tools, catalog)

	// ── Preview (huh Note) ───────────────────────────────────────────────────
	var previewLines strings.Builder
	upgradableCount := 0
	for _, it := range items {
		if it.Upgradable {
			fmt.Fprintf(&previewLines, "  ✓ %s\n", it.Name)
			upgradableCount++
		} else {
			fmt.Fprintf(&previewLines, "  - %s (no upgradable)\n", it.Name)
		}
	}

	confirmed, err := showUpgradePreviewAndConfirm(previewLines.String(), items, upgradableCount)
	if err != nil {
		return ToolUpgradeSummary{}, err
	}
	if !confirmed {
		return ToolUpgradeSummary{}, nil
	}

	// ── All non-upgradable ───────────────────────────────────────────────────
	if upgradableCount == 0 {
		return showAllNonUpgradable()
	}

	// ── Spinner + execute ────────────────────────────────────────────────────
	summary, err := runUpgradeSpinner(items, execFn)
	if err != nil {
		return ToolUpgradeSummary{}, err
	}

	// ── Summary screen ───────────────────────────────────────────────────────
	if err := showUpgradeSummary(summary); err != nil {
		return ToolUpgradeSummary{}, err
	}

	return summary, nil
}

// showUpgradeEmptyState renders the "no tools" note and waits for user.
func showUpgradeEmptyState() (ToolUpgradeSummary, error) {
	body := "Añade tools desde Setup o edita\ndevrune.yaml con una sección tools:."

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Upgrade Tools").
				Description("No hay tools para actualizar.\n\n" + body),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return ToolUpgradeSummary{}, err
	}
	return ToolUpgradeSummary{}, nil
}

// showAllNonUpgradable renders a clear message when every tool is non-upgradable.
func showAllNonUpgradable() (ToolUpgradeSummary, error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Upgrade Tools").
				Description("Ninguna tool tiene un comando de upgrade efectivo.\n\nRevisa que tus tools tengan un campo 'command' en devrune.yaml o en el catálogo embebido."),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return ToolUpgradeSummary{}, err
	}
	return ToolUpgradeSummary{}, nil
}

// showUpgradePreviewAndConfirm renders the preview note then an explicit yes/no
// confirmation. Returns true only when the user selects "Yes".
func showUpgradePreviewAndConfirm(previewLines string, items []upgradeToolItem, upgradableCount int) (bool, error) {
	// Preview step.
	previewForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Upgrade Tools").
				Description("Estas tools están declaradas en devrune.yaml.\nSolo se ejecutarán las que tengan command efectivo.\n\n" + previewLines),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := previewForm.Run(); err != nil {
		return false, err
	}

	if upgradableCount == 0 {
		// No need for the confirm step; caller handles the "all non-upgradable" case.
		return true, nil
	}

	// Build confirm description with commands.
	var confirmLines strings.Builder
	fmt.Fprintf(&confirmLines, "DevRune ejecutará comandos de upgrade en\ntu sistema para %d tool(s) upgradable(s).\n\n", upgradableCount)
	for _, it := range items {
		if it.Upgradable {
			fmt.Fprintf(&confirmLines, "  %s  → %s\n", it.Name, it.Command)
		}
	}
	confirmLines.WriteString("\n¿Ejecutar upgrades ahora?")

	var choice string
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Confirm Upgrade Tools").
				Description(confirmLines.String()).
				Options(
					huh.NewOption("Yes, upgrade tools", "yes"),
					huh.NewOption("No, back to menu", "no"),
				).
				Value(&choice),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := confirmForm.Run(); err != nil {
		return false, err
	}

	return choice == "yes", nil
}

// ---------------------------------------------------------------------------
// T014 — Spinner model and summary rendering
// ---------------------------------------------------------------------------

// toolUpgradeDoneMsg is sent when all parallel upgrades have completed.
type toolUpgradeDoneMsg struct {
	summary ToolUpgradeSummary
}

// toolUpgradeModel is a Bubbletea model that shows a spinner while tools
// are upgraded in parallel in the background.
type toolUpgradeModel struct {
	spinner spinner.Model
	items   []upgradeToolItem // all items (for display)
	summary ToolUpgradeSummary
	done    bool
	execFn  ToolCommandExecutor
}

func newToolUpgradeModel(items []upgradeToolItem, execFn ToolCommandExecutor) toolUpgradeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(tuistyles.ColorSecondary)
	return toolUpgradeModel{
		spinner: s,
		items:   items,
		execFn:  execFn,
	}
}

func (m toolUpgradeModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.doUpgrade(),
	)
}

func (m toolUpgradeModel) doUpgrade() tea.Cmd {
	items := m.items
	fn := m.execFn
	return func() tea.Msg {
		summary := upgradeToolsParallel(items, fn)
		return toolUpgradeDoneMsg{summary: summary}
	}
}

func (m toolUpgradeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case toolUpgradeDoneMsg:
		m.summary = msg.summary
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m toolUpgradeModel) View() tea.View {
	if m.done {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(tuistyles.StyleTitle.Render("Upgrade Tools"))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(m.spinner.View())
	sb.WriteString(" ")
	sb.WriteString(tuistyles.StyleInfo.Render("Upgrading selected tools..."))
	sb.WriteString("\n\n")

	for _, it := range m.items {
		if it.Upgradable {
			fmt.Fprintf(&sb, "   %s  running\n", it.Name)
		}
	}

	sb.WriteString("\n")
	sb.WriteString(tuistyles.StyleSubtitle.Render("  Los upgrades corren en paralelo; un fallo no cancela las demás tools."))
	sb.WriteString("\n")

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// runUpgradeSpinner launches the bubbletea spinner for parallel upgrades and
// returns the aggregated ToolUpgradeSummary.
func runUpgradeSpinner(items []upgradeToolItem, execFn ToolCommandExecutor) (ToolUpgradeSummary, error) {
	m := newToolUpgradeModel(items, execFn)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return ToolUpgradeSummary{}, fmt.Errorf("tool upgrade model: %w", err)
	}

	result, ok := finalModel.(toolUpgradeModel)
	if !ok {
		return ToolUpgradeSummary{}, fmt.Errorf("tool upgrade model: unexpected model type")
	}

	return result.summary, nil
}

// showUpgradeSummary renders the final per-tool ok/fail summary using a huh
// Note and waits for the user to press Continue / Back to menu.
func showUpgradeSummary(summary ToolUpgradeSummary) error {
	var sb strings.Builder
	sb.WriteString("Results\n\n")

	for _, r := range summary.Results {
		switch r.Status {
		case ToolUpgradeOK:
			okMark := tuistyles.StyleSuccess.Render("✓")
			fmt.Fprintf(&sb, "  %s %s  ok\n", okMark, r.Name)
		case ToolUpgradeFail:
			failMark := tuistyles.StyleError.Render("✗")
			fmt.Fprintf(&sb, "  %s %s  fail: %s\n", failMark, r.Name, r.Error)
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Upgrade Tools Complete").
				Description(sb.String()),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	return form.Run()
}
