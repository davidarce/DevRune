// SPDX-License-Identifier: MIT

package steps

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// ScannedRepoInput is the input data for the selection step.
type ScannedRepoInput struct {
	Source            string
	Skills            []string
	Rules             []string
	MCPs              []string
	Workflows         []string
	WorkflowManifests []model.WorkflowManifest // parsed workflow manifests
	Tools             []model.ToolDef          // available tool definitions
	Descs             map[string]string        // item name → description
	MCPFiles          map[string]string        // MCP name → filename (e.g. "engram" → "engram.yaml")
}

// SelectionResult holds the final user selection after the select step.
type SelectionResult struct {
	Repos []RepoSelectionResult
}

// RepoSelectionResult holds the selected items for one repository.
type RepoSelectionResult struct {
	Source            string
	SelectedSkills    []string
	SelectedRules     []string
	SelectedMCPs      []string
	SelectedWorkflows []string
	SelectedTools     []string          // selected tool names
	MCPFiles          map[string]string // MCP name → filename (passed through for manifest building)
}

// CategorySelection holds the state for a single category within a repo.
type CategorySelection struct {
	Kind     string            // "Skills", "Rules", "MCPs", "Workflows"
	Items    []string          // all available items
	Selected map[string]bool   // which items are selected (default: all true)
	IsOn     bool              // category-level toggle (default: true)
	Descs    map[string]string // item name → description (optional)
	Badges   map[string]string // item name → badge text (e.g. "Recommended"), optional
}

// selectedCount returns how many individual items are selected.
func (c *CategorySelection) selectedCount() int {
	count := 0
	for _, item := range c.Items {
		if c.Selected[item] {
			count++
		}
	}
	return count
}

// RepoSelection holds the selection state for one repository.
type RepoSelection struct {
	Source     string
	Categories [5]CategorySelection // Skills, Rules, MCPs, Workflows, Tools
	MCPFiles   map[string]string    // MCP name → filename
}

// ExpandedView tracks the expanded state for a category.
type ExpandedView struct {
	repoIdx   int
	catIdx    int
	cursor    int
	filter    string
	filtering bool
}

// SelectModel is the Bubbletea model for the category selection step.
type SelectModel struct {
	repos    []RepoSelection
	cursor   int // flat index: repoIdx*5 + catIdx
	expanded *ExpandedView
	done     bool
	aborted  bool
	width    int

	// Recommendation support.
	recEnabled         bool // show the "Confirm with AI recommendations" button
	useRecommendations bool // true if user chose "Confirm with AI recommendations"
}

// flatLen returns the total number of navigable rows (categories + confirm buttons).
func (m *SelectModel) flatLen() int {
	buttons := 1
	if m.recEnabled {
		buttons = 2
	}
	return len(m.repos)*5 + buttons
}

// isConfirmRow reports whether the given flat cursor index is the confirm button.
func (m *SelectModel) isConfirmRow(cursor int) bool {
	return cursor >= len(m.repos)*5
}

// isRecConfirmRow reports whether the cursor is on the "Confirm with AI recommendations" button.
func (m *SelectModel) isRecConfirmRow(cursor int) bool {
	return m.recEnabled && cursor == len(m.repos)*5+1
}

// repoAndCat converts a flat cursor index to (repoIdx, catIdx).
func repoAndCat(cursor int) (int, int) {
	return cursor / 5, cursor % 5
}

// NewSelectModel creates a SelectModel from scanned repositories.
// Workflows are pre-expanded by default because they are high-impact and few.
func NewSelectModel(repos []ScannedRepoInput) *SelectModel {
	selections := make([]RepoSelection, len(repos))
	for i, r := range repos {
		descs := r.Descs
		if descs == nil {
			descs = map[string]string{}
		}
		// Build tool name list and descs from ToolDef slice.
		toolNames := make([]string, 0, len(r.Tools))
		for _, td := range r.Tools {
			toolNames = append(toolNames, td.Name)
			if descs[td.Name] == "" && td.Description != "" {
				descs[td.Name] = td.Description
			}
		}
		categories := [5]CategorySelection{
			buildCategoryWithDescs("Skills", r.Skills, descs),
			buildCategoryWithDescs("Rules", r.Rules, descs),
			buildCategoryWithDescs("MCPs", r.MCPs, descs),
			buildCategoryWithDescs("Workflows", r.Workflows, descs),
			buildCategoryWithDescs("Tools", toolNames, descs),
		}
		selections[i] = RepoSelection{
			Source:     r.Source,
			Categories: categories,
			MCPFiles:   r.MCPFiles,
		}
	}

	m := &SelectModel{
		repos:  selections,
		cursor: 0,
		width:  80,
	}

	return m
}

// EnableRecommendations enables the "Confirm with AI recommendations" button.
func (m *SelectModel) EnableRecommendations() {
	m.recEnabled = true
}

// restoreSelection sets each item's Selected state from a previous SelectionResult.
func restoreSelection(m *SelectModel, prev SelectionResult) {
	prevBySource := make(map[string]RepoSelectionResult, len(prev.Repos))
	for _, r := range prev.Repos {
		prevBySource[r.Source] = r
	}

	for ri := range m.repos {
		prevRepo, ok := prevBySource[m.repos[ri].Source]
		if !ok {
			continue
		}

		prevSelected := [5]map[string]bool{
			toStringSet(prevRepo.SelectedSkills),
			toStringSet(prevRepo.SelectedRules),
			toStringSet(prevRepo.SelectedMCPs),
			toStringSet(prevRepo.SelectedWorkflows),
			toStringSet(prevRepo.SelectedTools),
		}

		for ci := range m.repos[ri].Categories {
			cat := &m.repos[ri].Categories[ci]
			prev := prevSelected[ci]
			allOn := len(cat.Items) > 0
			for _, item := range cat.Items {
				cat.Selected[item] = prev[item]
				if !prev[item] {
					allOn = false
				}
			}
			cat.IsOn = allOn
		}
	}
}

func toStringSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

// buildCategoryWithDescs creates a CategorySelection with all items selected by default.
func buildCategoryWithDescs(kind string, items []string, descs map[string]string) CategorySelection {
	sel := make(map[string]bool, len(items))
	catDescs := make(map[string]string)
	for _, item := range items {
		sel[item] = true
		if d, ok := descs[item]; ok {
			catDescs[item] = d
		}
	}
	return CategorySelection{
		Kind:     kind,
		Items:    items,
		Selected: sel,
		IsOn:     true,
		Descs:    catDescs,
		Badges:   map[string]string{},
	}
}

// Result builds a SelectionResult from the current model state.
func (m *SelectModel) Result() SelectionResult {
	result := SelectionResult{
		Repos: make([]RepoSelectionResult, len(m.repos)),
	}
	for i, repo := range m.repos {
		rr := RepoSelectionResult{Source: repo.Source, MCPFiles: repo.MCPFiles}

		skills := repo.Categories[0]
		if skills.IsOn {
			for _, item := range skills.Items {
				if skills.Selected[item] {
					rr.SelectedSkills = append(rr.SelectedSkills, item)
				}
			}
		}

		rules := repo.Categories[1]
		if rules.IsOn {
			for _, item := range rules.Items {
				if rules.Selected[item] {
					rr.SelectedRules = append(rr.SelectedRules, item)
				}
			}
		}

		mcps := repo.Categories[2]
		if mcps.IsOn {
			for _, item := range mcps.Items {
				if mcps.Selected[item] {
					rr.SelectedMCPs = append(rr.SelectedMCPs, item)
				}
			}
		}

		wfs := repo.Categories[3]
		if wfs.IsOn {
			for _, item := range wfs.Items {
				if wfs.Selected[item] {
					rr.SelectedWorkflows = append(rr.SelectedWorkflows, item)
				}
			}
		}

		tools := repo.Categories[4]
		if tools.IsOn {
			for _, item := range tools.Items {
				if tools.Selected[item] {
					rr.SelectedTools = append(rr.SelectedTools, item)
				}
			}
		}

		result.Repos[i] = rr
	}
	return result
}

// Init implements tea.Model.
func (m SelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	// Only react to key presses, not releases. In Bubbletea v2, tea.KeyMsg
	// matches both KeyPressMsg and KeyReleaseMsg — handling both causes
	// toggles to fire twice per keystroke, cancelling themselves out.
	case tea.KeyPressMsg:
		if m.expanded != nil {
			return m.updateExpanded(msg)
		}
		return m.updateCollapsed(msg)
	}
	return m, nil
}

// updateCollapsed handles key events in collapsed (category list) mode.
func (m SelectModel) updateCollapsed(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.aborted = true
		m.done = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < m.flatLen()-1 {
			m.cursor++
		}

	case "left", "h":
		// Navigate between buttons when on a confirm row.
		if m.isConfirmRow(m.cursor) {
			confirmIdx := len(m.repos) * 5
			if m.cursor > confirmIdx {
				m.cursor = confirmIdx
			}
		}

	case "right", "l":
		// Navigate between buttons when on a confirm row.
		if m.isConfirmRow(m.cursor) && m.recEnabled {
			recIdx := len(m.repos)*5 + 1
			if m.cursor < recIdx {
				m.cursor = recIdx
			}
		}

	case " ", "space":
		// Toggle all items in category on/off (not on confirm row).
		if !m.isConfirmRow(m.cursor) {
			ri, ci := repoAndCat(m.cursor)
			cat := &m.repos[ri].Categories[ci]
			// If any item is selected, deselect all. Otherwise, select all.
			anySelected := cat.selectedCount() > 0
			for _, item := range cat.Items {
				cat.Selected[item] = !anySelected
			}
			cat.IsOn = !anySelected
		}

	case "enter":
		if m.isRecConfirmRow(m.cursor) {
			// "Confirm with AI recommendations" button pressed.
			m.useRecommendations = true
			m.done = true
			return m, tea.Quit
		}
		if m.isConfirmRow(m.cursor) {
			// "Confirm selection" button pressed.
			m.done = true
			return m, tea.Quit
		}
		// Expand category if it has items.
		ri, ci := repoAndCat(m.cursor)
		cat := &m.repos[ri].Categories[ci]
		if len(cat.Items) > 0 {
			m.expanded = &ExpandedView{
				repoIdx: ri,
				catIdx:  ci,
			}
		}

	case "ctrl+d", "tab":
		// Confirm and finish.
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

// updateExpanded handles key events in expanded (item list) mode.
func (m SelectModel) updateExpanded(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	exp := m.expanded
	ri, ci := exp.repoIdx, exp.catIdx
	cat := &m.repos[ri].Categories[ci]

	filteredItems := applyFilter(cat.Items, exp.filter)

	switch {
	case exp.filtering:
		// In filter mode, most keys append to filter string.
		switch msg.String() {
		case "enter", "esc":
			exp.filtering = false
			exp.cursor = 0
		case "backspace":
			if len(exp.filter) > 0 {
				exp.filter = exp.filter[:len(exp.filter)-1]
			}
		case "space":
			exp.filter += " "
			exp.cursor = 0
		default:
			if len(msg.String()) == 1 {
				exp.filter += msg.String()
				exp.cursor = 0
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.aborted = true
		m.done = true
		return m, tea.Quit

	case "esc":
		// Collapse back to category view.
		m.expanded = nil

	case "enter":
		// Confirm and return to category view.
		m.expanded = nil

	case "/":
		exp.filtering = true
		exp.filter = ""

	case "up", "k":
		if exp.cursor > 0 {
			exp.cursor--
		}

	case "down", "j":
		if exp.cursor < len(filteredItems) {
			// +1 for the "select all" row.
			exp.cursor++
		}

	case " ", "space":
		if exp.cursor == 0 {
			// Toggle "select all".
			allOn := true
			for _, item := range filteredItems {
				if !cat.Selected[item] {
					allOn = false
					break
				}
			}
			for _, item := range filteredItems {
				cat.Selected[item] = !allOn
			}
		} else {
			idx := exp.cursor - 1
			if idx < len(filteredItems) {
				item := filteredItems[idx]
				cat.Selected[item] = !cat.Selected[item]
			}
		}

	case "ctrl+d", "tab":
		m.expanded = nil
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model.
func (m SelectModel) View() tea.View {
	if m.done {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	var sb strings.Builder

	if m.expanded != nil {
		sb.WriteString(m.renderExpanded())
	} else {
		sb.WriteString(m.renderCollapsed())
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// renderCollapsed renders the category list view.
func (m *SelectModel) renderCollapsed() string {
	var sb strings.Builder

	sb.WriteString(responsiveStepBanner())
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(tuistyles.StyleStepIndicator.Render(fmt.Sprintf("Step 3/%d: Select content", TotalSteps)))
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(tuistyles.StyleInfo.Render("↑/↓ navigate  Space toggle  Enter expand/confirm"))
	sb.WriteString("\n\n")

	for ri, repo := range m.repos {
		sb.WriteString(tuistyles.StyleHighlight.Render("┌ " + repo.Source))
		sb.WriteString("\n")

		for ci, cat := range repo.Categories {
			flatIdx := ri*5 + ci
			isFocused := m.cursor == flatIdx

			cursor := "  "
			if isFocused {
				cursor = tuistyles.StyleSuccess.Render("► ")
			}

			sel := cat.selectedCount()
			var checkBox string
			if sel == len(cat.Items) && sel > 0 {
				checkBox = tuistyles.StyleSuccess.Render("[x]")
			} else if sel > 0 {
				checkBox = tuistyles.StyleHighlight.Render("[-]")
			} else {
				checkBox = tuistyles.StyleError.Render("[ ]")
			}

			var count string
			if len(cat.Items) == 0 {
				count = tuistyles.StyleInfo.Render("(none)")
			} else {
				count = tuistyles.StyleInfo.Render(fmt.Sprintf("(%d/%d)", sel, len(cat.Items)))
			}

			line := fmt.Sprintf("│  %s%s %s %s", cursor, checkBox, cat.Kind, count)

			if isFocused {
				line = tuistyles.StyleHighlight.Render(line)
			} else {
				line = tuistyles.StyleSummaryValue.Render(line)
			}

			sb.WriteString(line)
			sb.WriteString("\n")
		}

		sb.WriteString(tuistyles.StyleHighlight.Render("└"))
		sb.WriteString("\n\n")
	}

	// Action buttons styled like huh confirm buttons.
	focusedBtn := lipgloss.NewStyle().
		Background(tuistyles.ColorSecondary).
		Foreground(tuistyles.ColorBg).
		Bold(true).
		Padding(0, 2)
	blurredBtn := lipgloss.NewStyle().
		Background(tuistyles.ColorDim).
		Foreground(lipgloss.Color("7")).
		Padding(0, 2)

	// Responsive labels: shorter text for narrow terminals.
	confirmLabel := "Confirm selection"
	aiLabel := "✨ AI Recommendations"
	if m.width < 60 {
		confirmLabel = "Confirm"
		aiLabel = "✨ AI Recs"
	}

	sb.WriteString("\n")

	confirmIdx := len(m.repos) * 5
	if m.cursor == confirmIdx {
		sb.WriteString("  " + focusedBtn.Render(confirmLabel))
	} else {
		sb.WriteString("  " + blurredBtn.Render(confirmLabel))
	}

	if m.recEnabled {
		recIdx := confirmIdx + 1
		sb.WriteString("  ")
		if m.cursor == recIdx {
			sb.WriteString(focusedBtn.Render(aiLabel))
		} else {
			sb.WriteString(blurredBtn.Render(aiLabel))
		}
	}

	sb.WriteString("\n\n")

	return sb.String()
}

// renderExpanded renders the expanded item selection view.
func (m *SelectModel) renderExpanded() string {
	exp := m.expanded
	ri, ci := exp.repoIdx, exp.catIdx
	cat := &m.repos[ri].Categories[ci]

	filteredItems := applyFilter(cat.Items, exp.filter)
	selCount := 0
	for _, item := range filteredItems {
		if cat.Selected[item] {
			selCount++
		}
	}

	var sb strings.Builder

	header := fmt.Sprintf("%s — %s  (%d/%d selected)", cat.Kind, m.repos[ri].Source, selCount, len(filteredItems))
	sb.WriteString(tuistyles.StyleStepIndicator.Render("┌ " + header))
	sb.WriteString("\n")

	if exp.filtering {
		sb.WriteString(tuistyles.StyleHighlight.Render("│  Filter: " + exp.filter + "█"))
		sb.WriteString("\n")
	} else if exp.filter != "" {
		sb.WriteString(tuistyles.StyleInfo.Render("│  Filter: " + exp.filter + "  (Esc to clear)"))
		sb.WriteString("\n")
	}

	// "Select all" row.
	allOn := len(filteredItems) > 0
	for _, item := range filteredItems {
		if !cat.Selected[item] {
			allOn = false
			break
		}
	}

	allCheck := tuistyles.StyleError.Render("[ ]")
	if allOn {
		allCheck = tuistyles.StyleSuccess.Render("[x]")
	}

	allCursor := "  "
	if exp.cursor == 0 {
		allCursor = tuistyles.StyleSuccess.Render("► ")
	}

	allLine := fmt.Sprintf("│  %s%s Select all", allCursor, allCheck)
	if exp.cursor == 0 {
		allLine = tuistyles.StyleHighlight.Render(allLine)
	} else {
		allLine = tuistyles.StyleSummaryValue.Render(allLine)
	}
	sb.WriteString(allLine)
	sb.WriteString("\n")
	sb.WriteString(tuistyles.StyleInfo.Render("│  ─────────────"))
	sb.WriteString("\n")

	// Individual item rows.
	maxVisible := 20
	start := 0
	cursorInList := exp.cursor - 1 // 0-based within filteredItems
	if cursorInList >= maxVisible {
		start = cursorInList - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(filteredItems) {
		end = len(filteredItems)
	}

	for idx := start; idx < end; idx++ {
		item := filteredItems[idx]
		isFocused := (exp.cursor == idx+1)

		itemCheck := tuistyles.StyleError.Render("[ ]")
		if cat.Selected[item] {
			itemCheck = tuistyles.StyleSuccess.Render("[x]")
		}

		itemCursor := "  "
		if isFocused {
			itemCursor = tuistyles.StyleSuccess.Render("► ")
		}

		line := fmt.Sprintf("│  %s%s %s", itemCursor, itemCheck, item)
		if isFocused {
			line = tuistyles.StyleHighlight.Render(line)
		} else {
			line = tuistyles.StyleSummaryValue.Render(line)
		}

		sb.WriteString(line)

		// Show badge if present (e.g. "Recommended", "Suggested").
		if badge, ok := cat.Badges[item]; ok && badge != "" {
			sb.WriteString(tuistyles.StyleSuccess.Render(" [" + badge + "]"))
		}

		// Show description on the same line if available.
		if desc, ok := cat.Descs[item]; ok && desc != "" {
			sb.WriteString(tuistyles.StyleInfo.Render(" — " + desc))
		}

		sb.WriteString("\n")
	}

	if len(filteredItems) > maxVisible {
		sb.WriteString(tuistyles.StyleInfo.Render(fmt.Sprintf("│  ... %d more items", len(filteredItems)-maxVisible)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(tuistyles.StyleInfo.Render("│  Enter/Esc: back • Space: toggle • /: filter • Tab/Ctrl+D: finish"))
	sb.WriteString("\n")
	sb.WriteString(tuistyles.StyleHighlight.Render("└"))
	sb.WriteString("\n")

	return sb.String()
}

// applyFilter filters items by a substring match (case-insensitive).
// If filter is empty, all items are returned.
func applyFilter(items []string, filter string) []string {
	if filter == "" {
		return items
	}
	lower := strings.ToLower(filter)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), lower) {
			result = append(result, item)
		}
	}
	return result
}

// SelectModelResult holds the output of RunSelectModel, including whether
// the user chose to include AI recommendations.
type SelectModelResult struct {
	Selection          SelectionResult
	UseRecommendations bool
}

// RunSelectModel runs the Bubbletea selection model and returns the result.
// previousSelection, if non-nil, restores the user's prior selections (for go-back support).
func RunSelectModel(repos []ScannedRepoInput, enableRecommendations bool, previousSelection ...SelectionResult) (SelectModelResult, error) {
	if len(repos) == 0 {
		return SelectModelResult{}, nil
	}

	m := NewSelectModel(repos)
	if enableRecommendations {
		m.EnableRecommendations()
	}

	// Restore previous selection state (for go-back loop).
	if len(previousSelection) > 0 && len(previousSelection[0].Repos) > 0 {
		restoreSelection(m, previousSelection[0])
	}

	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return SelectModelResult{}, fmt.Errorf("select model: %w", err)
	}

	result, ok := finalModel.(SelectModel)
	if !ok {
		return SelectModelResult{}, fmt.Errorf("select model: unexpected model type")
	}

	if result.aborted {
		return SelectModelResult{}, errUserAborted
	}

	return SelectModelResult{
		Selection:          result.Result(),
		UseRecommendations: result.useRecommendations,
	}, nil
}

// errUserAborted is returned when the user quits the selection model.
var errUserAborted = fmt.Errorf("user aborted")
