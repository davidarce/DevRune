// SPDX-License-Identifier: MIT

package steps

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/recommend"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// RunRecommendGate presents the recommendation summary with all recommended items
// listed, and asks the user to accept or keep their manual selection.
func RunRecommendGate(recs []recommend.Recommendation, profile detect.ProjectProfile) (accepted bool, err error) {
	desc := buildGateDescription(recs, profile)

	choice := true

	form := huh.NewForm(
		huh.NewGroup(
			stepHeader(4, TotalSteps, "AI Recommendations"),
			huh.NewNote().
				Title("").
				Description(desc),
			huh.NewConfirm().
				Title(fmt.Sprintf("Apply %d recommendations to your selection?", len(recs))).
				Affirmative("Yes, apply").
				Negative("No, go back").
				Value(&choice),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		return false, err
	}

	return choice, nil
}

// buildGateDescription builds the summary note shown above the gate select,
// including the list of recommended items with confidence scores.
func buildGateDescription(recs []recommend.Recommendation, profile detect.ProjectProfile) string {
	w, h := termSize()
	return testableBuildGateDescription(recs, profile, w, h)
}

// testableBuildGateDescription builds the gate description with explicit dimensions.
func testableBuildGateDescription(recs []recommend.Recommendation, profile detect.ProjectProfile, w, h int) string {
	successStyle := tuistyles.StyleSuccess.Bold(true)
	highlightStyle := tuistyles.StyleHighlight.Bold(true)
	muteStyle := tuistyles.StyleInfo
	badgeStyle := tuistyles.StyleSuccess

	var b strings.Builder
	b.WriteString(successStyle.Render("AI analysis complete!"))
	b.WriteString("\n")

	// Show detected tech profile.
	langs := formatLanguageNames(profile.Languages, 4)
	frameworks := formatStringSlice(profile.Frameworks, 4)
	_, _ = fmt.Fprintf(&b, "%s %s  %s %s\n\n",
		highlightStyle.Render("Detected:"),
		muteStyle.Render(langs),
		highlightStyle.Render("Frameworks:"),
		muteStyle.Render(frameworks),
	)

	// Group recommendations by kind.
	byKind := map[string][]recommend.Recommendation{}
	kindOrder := []string{"skill", "rule", "mcp", "workflow"}
	kindLabels := map[string]string{"skill": "Skills", "rule": "Rules", "mcp": "MCPs", "workflow": "Workflows"}

	for _, rec := range recs {
		byKind[rec.Kind] = append(byKind[rec.Kind], rec)
	}

	// Show recommendations grouped by kind.
	maxItems := 20 // limit display in small terminals
	if h < 30 {
		maxItems = 8
	}
	shown := 0
	for _, kind := range kindOrder {
		items := byKind[kind]
		if len(items) == 0 {
			continue
		}
		label := kindLabels[kind]
		b.WriteString(highlightStyle.Render("  " + label + ":"))
		b.WriteString("\n")
		for _, rec := range items {
			if shown >= maxItems {
				remaining := len(recs) - shown
				if remaining > 0 {
					_, _ = fmt.Fprintf(&b, "  %s\n", muteStyle.Render(fmt.Sprintf("  ... and %d more", remaining)))
				}
				goto done
			}
			pct := int(rec.Confidence * 100)
			badge := badgeStyle.Render(fmt.Sprintf("[%d%%]", pct))
			if rec.Reason != "" {
				_, _ = fmt.Fprintf(&b, "    %s %s %s\n", badge, rec.Name, muteStyle.Render("— "+rec.Reason))
			} else {
				_, _ = fmt.Fprintf(&b, "    %s %s\n", badge, rec.Name)
			}
			shown++
		}
	}
done:
	b.WriteString("\n")

	return b.String()
}

// formatStringSlice formats a string slice for display, truncating at maxItems.
func formatStringSlice(items []string, maxItems int) string {
	if len(items) == 0 {
		return "(none)"
	}
	if len(items) > maxItems {
		shown := items[:maxItems]
		return strings.Join(shown, ", ") + fmt.Sprintf(" +%d more", len(items)-maxItems)
	}
	return strings.Join(items, ", ")
}

// formatLanguageNames extracts language names from LanguageInfo slice.
func formatLanguageNames(langs []detect.LanguageInfo, maxItems int) string {
	names := make([]string, 0, len(langs))
	for _, l := range langs {
		names = append(names, l.Name)
	}
	return formatStringSlice(names, maxItems)
}

// MergeRecommendationsIntoSelection adds AI-recommended items (above threshold)
// to the user's existing selection. Pure data merge, no TUI interaction.
func MergeRecommendationsIntoSelection(prev SelectionResult, recs []recommend.Recommendation, threshold float64) SelectionResult {
	if threshold <= 0 {
		threshold = 0.7
	}

	// Deep copy the previous selection.
	merged := SelectionResult{
		Repos: make([]RepoSelectionResult, len(prev.Repos)),
	}
	for i, r := range prev.Repos {
		merged.Repos[i] = RepoSelectionResult{
			Source:            r.Source,
			MCPFiles:          r.MCPFiles,
			SelectedSkills:    append([]string{}, r.SelectedSkills...),
			SelectedRules:     append([]string{}, r.SelectedRules...),
			SelectedMCPs:      append([]string{}, r.SelectedMCPs...),
			SelectedWorkflows: append([]string{}, r.SelectedWorkflows...),
			SelectedTools:     append([]string{}, r.SelectedTools...),
		}
	}

	// For each recommendation, add it to the repo whose source matches.
	// Each recommendation carries a Source field identifying which repo it belongs to.
	for ri := range merged.Repos {
		repo := &merged.Repos[ri]

		type listRef struct {
			kind string
			list *[]string
		}
		lists := []listRef{
			{"skill", &repo.SelectedSkills},
			{"rule", &repo.SelectedRules},
			{"mcp", &repo.SelectedMCPs},
			{"workflow", &repo.SelectedWorkflows},
			{"tool", &repo.SelectedTools},
		}

		for _, lr := range lists {
			existing := make(map[string]bool, len(*lr.list))
			for _, item := range *lr.list {
				existing[item] = true
			}
			for _, rec := range recs {
				if rec.Kind == lr.kind && rec.Source == repo.Source && rec.Confidence >= threshold && !existing[rec.Name] {
					*lr.list = append(*lr.list, rec.Name)
					existing[rec.Name] = true
				}
			}
		}
	}

	return merged
}

// reposToSources converts ScannedRepoInput slices to ScannedSource slices,
// filtering out skill headers (non-interactive labels like "Go", "Spring Boot")
// so the AI doesn't recommend them as installable skills.
func reposToSources(repos []ScannedRepoInput) []recommend.ScannedSource {
	sources := make([]recommend.ScannedSource, 0, len(repos))
	for _, r := range repos {
		skills := r.Skills
		if len(r.SkillHeaders) > 0 {
			skills = make([]string, 0, len(r.Skills))
			for _, s := range r.Skills {
				if !r.SkillHeaders[s] {
					skills = append(skills, s)
				}
			}
		}
		sources = append(sources, recommend.ScannedSource{
			Source: r.Source, Skills: skills, Rules: r.Rules,
			MCPs: r.MCPs, Workflows: r.Workflows, Descs: r.Descs,
		})
	}
	return sources
}

// hasValidSourceScheme checks if a repo source has a recognized scheme prefix.
func hasValidSourceScheme(source string) bool {
	return strings.HasPrefix(source, "github:") ||
		strings.HasPrefix(source, "gitlab:") ||
		strings.HasPrefix(source, "local:")
}

// RunRecommendStep shows the gate. Accept → merge recommendations into selection.
// Keep → return original selection unchanged.
func RunRecommendStep(repos []ScannedRepoInput, result *recommend.RecommendResult, profile detect.ProjectProfile, threshold float64, previousSelection SelectionResult) (selection SelectionResult, skipped bool, err error) {
	accepted, err := RunRecommendGate(result.Recommendations, profile)
	if err != nil {
		return SelectionResult{}, false, err
	}

	if !accepted {
		return SelectionResult{}, true, nil
	}

	// User accepted: merge recommendations into their existing selection.
	merged := MergeRecommendationsIntoSelection(previousSelection, result.Recommendations, threshold)
	return merged, false, nil
}

// recommendFlowDoneMsg is sent when the on-demand AI recommendation completes.
type recommendFlowDoneMsg struct {
	result  *recommend.RecommendResult
	profile detect.ProjectProfile
	err     error
}

// recommendFlowPhase tracks the current phase of the combined flow+gate model.
type recommendFlowPhase int

const (
	phaseLoading recommendFlowPhase = iota
	phaseGate
)

// recommendFlowModel combines spinner (loading) + gate (accept/decline) into
// a single Bubbletea program to avoid flickering between separate programs.
type recommendFlowModel struct {
	phase   recommendFlowPhase
	spinner spinner.Model
	repos   []ScannedRepoInput

	// Set when loading completes.
	result  *recommend.RecommendResult
	profile detect.ProjectProfile
	err     error

	// Gate state.
	accepted bool
	skipped  bool
	done     bool
	cursor   int // 0=Yes, 1=No
}

func newRecommendFlowModel(repos []ScannedRepoInput, cached *recommend.RecommendResult, profile detect.ProjectProfile) recommendFlowModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	m := recommendFlowModel{
		spinner: s,
		repos:   repos,
	}

	// If cached, skip straight to gate phase.
	if cached != nil {
		m.phase = phaseGate
		m.result = cached
		m.profile = profile
	}

	return m
}

func (m recommendFlowModel) Init() tea.Cmd {
	if m.phase == phaseGate {
		return nil // already have results
	}
	return tea.Batch(m.spinner.Tick, m.doRecommend())
}

func (m recommendFlowModel) doRecommend() tea.Cmd {
	repos := m.repos
	return func() tea.Msg {
		dir, _ := os.Getwd()
		profile, err := detect.Analyze(dir)
		if err != nil {
			return recommendFlowDoneMsg{err: err}
		}

		binaryPath, agentName, err := recommend.DetectAgent()
		if err != nil {
			return recommendFlowDoneMsg{err: err}
		}

		sources := reposToSources(repos)
		catalog := recommend.BuildCatalogItems(sources)

		cfg := recommend.RecommendConfig{Threshold: 0.7, Enabled: true, Models: recommend.DefaultAgentModels()}
		engine := recommend.NewEngine(binaryPath, agentName, cfg)
		result, err := engine.Recommend(context.Background(), *profile, catalog)
		return recommendFlowDoneMsg{result: result, profile: *profile, err: err}
	}
}

func (m recommendFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.skipped = true
			m.done = true
			return m, tea.Quit
		}

		if m.phase == phaseGate {
			switch msg.String() {
			case "left", "h":
				m.cursor = 0
			case "right", "l":
				m.cursor = 1
			case "y":
				m.accepted = true
				m.done = true
				return m, tea.Quit
			case "n":
				m.skipped = true
				m.done = true
				return m, tea.Quit
			case "enter":
				if m.cursor == 0 {
					m.accepted = true
				} else {
					m.skipped = true
				}
				m.done = true
				return m, tea.Quit
			}
		}

	case recommendFlowDoneMsg:
		if msg.err != nil || msg.result == nil || len(msg.result.Recommendations) == 0 {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}
		// Transition to gate phase.
		m.phase = phaseGate
		m.result = msg.result
		m.profile = msg.profile
		return m, nil

	case spinner.TickMsg:
		if m.phase == phaseLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m recommendFlowModel) View() tea.View {
	var sb strings.Builder
	sb.WriteString(responsiveStepBanner())
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(tuistyles.StyleStepIndicator.Render(fmt.Sprintf("Step 4/%d: AI Recommendations", TotalSteps)))
	sb.WriteString("\n\n")

	if m.phase == phaseLoading {
		sb.WriteString("  ")
		sb.WriteString(m.spinner.View())
		sb.WriteString(" ")
		sb.WriteString(tuistyles.StyleInfo.Render("Analyzing project and generating recommendations..."))
		sb.WriteString("\n")
	} else {
		// Gate phase: show recommendations + buttons.
		desc := buildGateDescription(m.result.Recommendations, m.profile)
		sb.WriteString(desc)
		sb.WriteString("\n")

		// Buttons.
		focusedBtn := lipgloss.NewStyle().
			Background(tuistyles.ColorSecondary).
			Foreground(tuistyles.ColorBg).
			Bold(true).
			Padding(0, 2)
		blurredBtn := lipgloss.NewStyle().
			Background(tuistyles.ColorDim).
			Foreground(lipgloss.Color("7")).
			Padding(0, 2)

		applyLabel := fmt.Sprintf("Yes, apply (%d items)", len(m.result.Recommendations))

		sb.WriteString("  ")
		if m.cursor == 0 {
			sb.WriteString(focusedBtn.Render(applyLabel))
		} else {
			sb.WriteString(blurredBtn.Render(applyLabel))
		}
		sb.WriteString("  ")
		if m.cursor == 1 {
			sb.WriteString(focusedBtn.Render("No, go back"))
		} else {
			sb.WriteString(blurredBtn.Render("No, go back"))
		}
		sb.WriteString("\n\n")
		sb.WriteString(tuistyles.StyleInfo.Render("  ←/→ toggle  enter submit  y Yes  n No"))
		sb.WriteString("\n")
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// RecommendFlowResult holds the output of RunRecommendFlow.
type RecommendFlowResult struct {
	Result   *recommend.RecommendResult
	Accepted bool  // user chose "Yes, apply"
	Skipped  bool  // user chose "No, go back" or error
	Err      error
}

// RunRecommendFlow runs the combined spinner+gate in a single Bubbletea program.
// No flicker between transitions. Returns the result and user decision.
func RunRecommendFlow(repos []ScannedRepoInput) RecommendFlowResult {
	// Quick cache check.
	dir, _ := os.Getwd()
	sources := reposToSources(repos)
	catalog := recommend.BuildCatalogItems(sources)
	cached := recommend.CheckQuickCache(dir, catalog, 0.7)

	// Detect profile for gate display (needed for both cached and fresh).
	var profile detect.ProjectProfile
	if cached != nil {
		if p, err := detect.Analyze(dir); err == nil {
			profile = *p
		}
	}

	m := newRecommendFlowModel(repos, cached, profile)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return RecommendFlowResult{Err: err, Skipped: true}
	}

	result, ok := finalModel.(recommendFlowModel)
	if !ok {
		return RecommendFlowResult{Err: fmt.Errorf("unexpected model type"), Skipped: true}
	}

	return RecommendFlowResult{
		Result:   result.result,
		Accepted: result.accepted,
		Skipped:  result.skipped || result.err != nil,
		Err:      result.err,
	}
}
