// SPDX-License-Identifier: MIT

package resolve

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// writeFile is a test helper that creates a file and all parent directories.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", full, err)
	}
}

// contentNames returns a sorted slice of Name fields from a []model.ContentItem.
func contentNames(items []model.ContentItem) []string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	sort.Strings(names)
	return names
}

// itemsByKind filters items by kind.
func itemsByKind(items []model.ContentItem, kind model.ContentKind) []model.ContentItem {
	var result []model.ContentItem
	for _, item := range items {
		if item.Kind == kind {
			result = append(result, item)
		}
	}
	return result
}

// TestEnumerateContents_EmptyDirectory verifies that an empty directory returns no items.
func TestEnumerateContents_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("EnumerateContents() = %v items, want 0", len(items))
	}
}

// TestEnumerateContents_NonexistentDirectory verifies that a non-existent directory returns empty, no error.
func TestEnumerateContents_NonexistentDirectory(t *testing.T) {
	items, err := EnumerateContents("/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("EnumerateContents() = %v items, want 0", len(items))
	}
}

// TestEnumerateContents_SkillsWithoutSKILLMD verifies that skill dirs without SKILL.md are not discovered.
func TestEnumerateContents_SkillsWithoutSKILLMD(t *testing.T) {
	dir := t.TempDir()

	// Valid skill: has SKILL.md
	writeFile(t, dir, "skills/git-commit/SKILL.md", "# git-commit skill")
	// Invalid skill: missing SKILL.md
	writeFile(t, dir, "skills/no-skill-file/README.md", "not a skill")
	// Also a non-dir file in skills (should be ignored)
	writeFile(t, dir, "skills/standalone.md", "standalone file")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	skills := itemsByKind(items, model.KindSkill)
	if len(skills) != 1 {
		t.Errorf("got %d skills, want 1; skills = %v", len(skills), skills)
	}
	if len(skills) == 1 && skills[0].Name != "git-commit" {
		t.Errorf("skill name = %q, want %q", skills[0].Name, "git-commit")
	}
}

// TestEnumerateContents_RulesNestedStructure verifies that rules are discovered with path-based names.
func TestEnumerateContents_RulesNestedStructure(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "rules/architecture/clean-architecture/clean-architecture-rules.md", "# rule")
	writeFile(t, dir, "rules/testing/unit-tests/unit-tests-rules.md", "# rule")
	writeFile(t, dir, "rules/api/api-standards/api-standards-rules.md", "# rule")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	rules := itemsByKind(items, model.KindRule)
	if len(rules) != 3 {
		t.Errorf("got %d rules, want 3; rules = %v", len(rules), rules)
	}

	names := contentNames(rules)
	wantNames := []string{
		"api/api-standards/api-standards-rules",
		"architecture/clean-architecture/clean-architecture-rules",
		"testing/unit-tests/unit-tests-rules",
	}
	for i, want := range wantNames {
		if i >= len(names) || names[i] != want {
			t.Errorf("rule[%d] name = %q, want %q", i, names[i], want)
		}
	}
}

// TestEnumerateContents_HiddenFilesIgnored verifies that hidden files and dirs are skipped.
func TestEnumerateContents_HiddenFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Hidden skill directory — should be ignored.
	writeFile(t, dir, "skills/.hidden-skill/SKILL.md", "# hidden skill")
	// Visible skill — should be discovered.
	writeFile(t, dir, "skills/visible-skill/SKILL.md", "# visible skill")
	// Hidden rule file — should be ignored.
	writeFile(t, dir, "rules/.hidden-rule.md", "# hidden rule")
	// Visible rule — should be discovered.
	writeFile(t, dir, "rules/visible-rule.md", "# visible rule")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	skills := itemsByKind(items, model.KindSkill)
	if len(skills) != 1 || skills[0].Name != "visible-skill" {
		t.Errorf("skills = %v, want [visible-skill]", skills)
	}

	rules := itemsByKind(items, model.KindRule)
	if len(rules) != 1 || rules[0].Name != "visible-rule" {
		t.Errorf("rules = %v, want [visible-rule]", rules)
	}
}

// TestEnumerateContents_MixedContent verifies that skills, rules, and prompts are all enumerated.
func TestEnumerateContents_MixedContent(t *testing.T) {
	dir := t.TempDir()

	// Skills
	writeFile(t, dir, "skills/git-commit/SKILL.md", "# skill")
	writeFile(t, dir, "skills/git-pull-request/SKILL.md", "# skill")
	// Rules
	writeFile(t, dir, "rules/architecture/clean-architecture/clean-architecture-rules.md", "# rule")
	// Prompts
	writeFile(t, dir, "prompts/review-pr.md", "# prompt")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	skills := itemsByKind(items, model.KindSkill)
	rules := itemsByKind(items, model.KindRule)
	prompts := itemsByKind(items, model.KindPrompt)

	if len(skills) != 2 {
		t.Errorf("got %d skills, want 2", len(skills))
	}
	if len(rules) != 1 {
		t.Errorf("got %d rules, want 1", len(rules))
	}
	if len(prompts) != 1 {
		t.Errorf("got %d prompts, want 1", len(prompts))
	}

	if len(items) != 4 {
		t.Errorf("got %d total items, want 4", len(items))
	}
}

// TestEnumerateContents_SkillPathFormat verifies the path format of skill items.
func TestEnumerateContents_SkillPathFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skills/git-commit/SKILL.md", "# skill")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	item := items[0]
	if item.Kind != model.KindSkill {
		t.Errorf("Kind = %q, want %q", item.Kind, model.KindSkill)
	}
	if item.Name != "git-commit" {
		t.Errorf("Name = %q, want %q", item.Name, "git-commit")
	}
	if item.Path != "skills/git-commit/" {
		t.Errorf("Path = %q, want %q", item.Path, "skills/git-commit/")
	}
}

// TestApplyFilter_NilFilter verifies that a nil filter returns all items unchanged.
func TestApplyFilter_NilFilter(t *testing.T) {
	items := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/"},
		{Kind: model.KindRule, Name: "architecture/clean", Path: "rules/architecture/clean/"},
		{Kind: model.KindPrompt, Name: "review-pr", Path: "prompts/review-pr.md"},
	}

	result := ApplyFilter(items, nil)
	if len(result) != len(items) {
		t.Errorf("ApplyFilter(nil) returned %d items, want %d", len(result), len(items))
	}
}

// TestApplyFilter_SkillSelection verifies that select filter correctly filters skills.
func TestApplyFilter_SkillSelection(t *testing.T) {
	items := []model.ContentItem{
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/"},
		{Kind: model.KindSkill, Name: "git-pull-request", Path: "skills/git-pull-request/"},
		{Kind: model.KindSkill, Name: "sdd-explore", Path: "skills/sdd-explore/"},
		{Kind: model.KindRule, Name: "architecture/clean", Path: "rules/architecture/clean/"},
	}

	filter := &model.SelectFilter{
		Skills: []string{"git-commit", "sdd-explore"},
	}

	result := ApplyFilter(items, filter)

	// Should have git-commit + sdd-explore (skills only). Rules are excluded
	// when skills are explicitly selected but rules are not.
	if len(result) != 2 {
		t.Errorf("got %d items, want 2; items = %v", len(result), result)
	}

	skillNames := contentNames(itemsByKind(result, model.KindSkill))
	wantSkillNames := []string{"git-commit", "sdd-explore"}
	sort.Strings(wantSkillNames)
	if len(skillNames) != len(wantSkillNames) {
		t.Errorf("skill names = %v, want %v", skillNames, wantSkillNames)
	}
	for i := range wantSkillNames {
		if i < len(skillNames) && skillNames[i] != wantSkillNames[i] {
			t.Errorf("skill[%d] = %q, want %q", i, skillNames[i], wantSkillNames[i])
		}
	}
}

// TestApplyFilter_RuleSelection verifies that select filter correctly filters rules.
func TestApplyFilter_RuleSelection(t *testing.T) {
	items := []model.ContentItem{
		{Kind: model.KindRule, Name: "architecture/clean-architecture", Path: "rules/architecture/clean-architecture/"},
		{Kind: model.KindRule, Name: "testing/unit-tests", Path: "rules/testing/unit-tests/"},
		{Kind: model.KindRule, Name: "api/api-standards", Path: "rules/api/api-standards/"},
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/"},
	}

	filter := &model.SelectFilter{
		Rules: []string{"architecture/clean-architecture"},
	}

	result := ApplyFilter(items, filter)

	rules := itemsByKind(result, model.KindRule)
	if len(rules) != 1 {
		t.Errorf("got %d rules, want 1; rules = %v", len(rules), rules)
	}
	if rules[0].Name != "architecture/clean-architecture" {
		t.Errorf("rule name = %q, want %q", rules[0].Name, "architecture/clean-architecture")
	}

	// git-commit skill should still be present (no skill filter = include all)
	skills := itemsByKind(result, model.KindSkill)
	if len(skills) != 1 {
		t.Errorf("got %d skills, want 1", len(skills))
	}
}

// TestApplyFilter_EmptyFilterLists verifies that empty filter lists include all items of that kind.
func TestApplyFilter_EmptyFilterLists(t *testing.T) {
	items := []model.ContentItem{
		{Kind: model.KindSkill, Name: "skill-a", Path: "skills/skill-a/"},
		{Kind: model.KindSkill, Name: "skill-b", Path: "skills/skill-b/"},
		{Kind: model.KindRule, Name: "rule-a", Path: "rules/rule-a/"},
	}

	// Empty filter (not nil, but no items in lists) means include all.
	filter := &model.SelectFilter{
		Skills: []string{},
		Rules:  []string{},
	}

	result := ApplyFilter(items, filter)
	if len(result) != len(items) {
		t.Errorf("got %d items, want %d", len(result), len(items))
	}
}

// TestApplyFilter_PromptsAlwaysIncluded verifies that prompts pass through any filter.
func TestApplyFilter_PromptsAlwaysIncluded(t *testing.T) {
	items := []model.ContentItem{
		{Kind: model.KindPrompt, Name: "review-pr", Path: "prompts/review-pr.md"},
		{Kind: model.KindSkill, Name: "git-commit", Path: "skills/git-commit/"},
	}

	// Filter that selects no skills and no rules — prompts still pass.
	filter := &model.SelectFilter{
		Skills: []string{"nonexistent-skill"},
	}

	result := ApplyFilter(items, filter)

	prompts := itemsByKind(result, model.KindPrompt)
	if len(prompts) != 1 {
		t.Errorf("got %d prompts, want 1 (prompts should always be included)", len(prompts))
	}

	skills := itemsByKind(result, model.KindSkill)
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

// TestEnumerateContents_SkillDescription verifies that ContentItem.Description is
// populated from the SKILL.md frontmatter's description field (T025).
func TestEnumerateContents_SkillDescription(t *testing.T) {
	dir := t.TempDir()

	// Skill with a description in frontmatter.
	writeFile(t, dir, "skills/git-commit/SKILL.md", `---
name: git-commit
description: Automate git commits following Conventional Commits
---
# git commit skill body
`)

	// Skill with no description in frontmatter.
	writeFile(t, dir, "skills/no-description/SKILL.md", `---
name: no-description
---
# no description body
`)

	// Skill with frontmatter that has no parseable description (empty SKILL.md).
	writeFile(t, dir, "skills/minimal/SKILL.md", "# minimal skill")

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	skills := itemsByKind(items, model.KindSkill)
	if len(skills) != 3 {
		t.Fatalf("got %d skills, want 3", len(skills))
	}

	// Build a map for easier lookup.
	skillMap := make(map[string]model.ContentItem)
	for _, s := range skills {
		skillMap[s.Name] = s
	}

	// Skill with description should have it populated.
	if got := skillMap["git-commit"].Description; got != "Automate git commits following Conventional Commits" {
		t.Errorf("git-commit Description = %q, want %q", got, "Automate git commits following Conventional Commits")
	}

	// Skill without description should have empty Description.
	if got := skillMap["no-description"].Description; got != "" {
		t.Errorf("no-description Description = %q, want empty", got)
	}

	// Skill with no frontmatter should have empty Description.
	if got := skillMap["minimal"].Description; got != "" {
		t.Errorf("minimal Description = %q, want empty", got)
	}
}

// TestEnumerateContents_RuleMetaPopulation verifies that ContentItem.RuleMeta is
// populated from rule file frontmatter (T026).
func TestEnumerateContents_RuleMetaPopulation(t *testing.T) {
	dir := t.TempDir()

	// Rule with full frontmatter metadata (YAML list format for applies-to).
	writeFile(t, dir, "rules/tech/react/react-rules.md", `---
name: react
description: React and TypeScript coding standards
scope: tech
technology: any
applies-to:
  - component-adviser
  - frontend-test-adviser
---
# React rules body
`)

	// Rule with partial frontmatter (scope only).
	writeFile(t, dir, "rules/architecture/clean/clean-rules.md", `---
scope: architecture
---
# clean architecture
`)

	// Rule with no frontmatter.
	writeFile(t, dir, "rules/api/api-standards.md", "# plain rule, no frontmatter")

	// Rule with legacy underscore key format for backward-compat.
	writeFile(t, dir, "rules/api/legacy-rule.md", `---
description: Legacy rule with underscore key
scope: api
applies_to: adviser-a, adviser-b
---
# Legacy rule body
`)

	items, err := EnumerateContents(dir)
	if err != nil {
		t.Fatalf("EnumerateContents() error = %v", err)
	}

	rules := itemsByKind(items, model.KindRule)
	if len(rules) != 4 {
		t.Fatalf("got %d rules, want 4", len(rules))
	}

	// Build a map by name for easier lookup.
	ruleMap := make(map[string]model.ContentItem)
	for _, rule := range rules {
		ruleMap[rule.Name] = rule
	}

	// Full frontmatter rule should have all RuleMeta fields populated.
	reactRule, ok := ruleMap["tech/react/react-rules"]
	if !ok {
		t.Fatal("rule tech/react/react-rules not found")
	}
	if reactRule.RuleMeta == nil {
		t.Fatal("tech/react/react-rules RuleMeta should not be nil")
	}
	if reactRule.RuleMeta.Scope != "tech" {
		t.Errorf("Scope = %q, want %q", reactRule.RuleMeta.Scope, "tech")
	}
	if reactRule.RuleMeta.Technology != "any" {
		t.Errorf("Technology = %q, want %q", reactRule.RuleMeta.Technology, "any")
	}
	if reactRule.RuleMeta.AppliesTo != "component-adviser, frontend-test-adviser" {
		t.Errorf("AppliesTo = %q, want %q", reactRule.RuleMeta.AppliesTo, "component-adviser, frontend-test-adviser")
	}
	if reactRule.RuleMeta.DisplayName != "react" {
		t.Errorf("DisplayName = %q, want %q", reactRule.RuleMeta.DisplayName, "react")
	}
	if reactRule.RuleMeta.Description != "React and TypeScript coding standards" {
		t.Errorf("RuleMeta.Description = %q, want %q", reactRule.RuleMeta.Description, "React and TypeScript coding standards")
	}
	// ContentItem.Description should also be populated from frontmatter.
	if reactRule.Description != "React and TypeScript coding standards" {
		t.Errorf("ContentItem.Description = %q, want %q", reactRule.Description, "React and TypeScript coding standards")
	}

	// Partial frontmatter rule should have RuleMeta with only scope set.
	cleanRule, ok := ruleMap["architecture/clean/clean-rules"]
	if !ok {
		t.Fatal("rule architecture/clean/clean-rules not found")
	}
	if cleanRule.RuleMeta == nil {
		t.Fatal("architecture/clean/clean-rules RuleMeta should not be nil (has scope)")
	}
	if cleanRule.RuleMeta.Scope != "architecture" {
		t.Errorf("Scope = %q, want %q", cleanRule.RuleMeta.Scope, "architecture")
	}
	if cleanRule.RuleMeta.Technology != "" {
		t.Errorf("Technology = %q, want empty", cleanRule.RuleMeta.Technology)
	}

	// Rule with no frontmatter should have nil RuleMeta.
	apiRule, ok := ruleMap["api/api-standards"]
	if !ok {
		t.Fatal("rule api/api-standards not found")
	}
	if apiRule.RuleMeta != nil {
		t.Errorf("api/api-standards RuleMeta should be nil for rule without frontmatter, got %+v", apiRule.RuleMeta)
	}
	if apiRule.Description != "" {
		t.Errorf("api/api-standards Description = %q, want empty", apiRule.Description)
	}

	// Legacy rule with underscore applies_to key should be backward-compatible.
	legacyRule, ok := ruleMap["api/legacy-rule"]
	if !ok {
		t.Fatal("rule api/legacy-rule not found")
	}
	if legacyRule.RuleMeta == nil {
		t.Fatal("api/legacy-rule RuleMeta should not be nil")
	}
	if legacyRule.RuleMeta.AppliesTo != "adviser-a, adviser-b" {
		t.Errorf("legacy AppliesTo = %q, want %q", legacyRule.RuleMeta.AppliesTo, "adviser-a, adviser-b")
	}
}
