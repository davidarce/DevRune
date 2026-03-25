package parse_test

import (
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/parse"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantFMKeys      []string
		wantBodyContains string
		wantErr         bool
		errContains     string
	}{
		{
			name: "standard skill file with frontmatter",
			input: `---
name: git:commit
description: Automate git commits
allowed-tools:
  - Bash
  - Read
---
# Body content here

Some markdown body.
`,
			wantFMKeys:      []string{"name", "description", "allowed-tools"},
			wantBodyContains: "# Body content here",
		},
		{
			name: "file with no frontmatter returns empty map and full content as body",
			input: `# Just a markdown file

No frontmatter here.
`,
			wantFMKeys:      []string{},
			wantBodyContains: "# Just a markdown file",
		},
		{
			name:             "empty file returns empty frontmatter and empty body",
			input:            "",
			wantFMKeys:       []string{},
			wantBodyContains: "",
		},
		{
			name: "frontmatter with multiline string values",
			input: `---
name: my-skill
description: "A skill with a\nmultiline description"
argument-hint: "[topic] [extra]"
---
# Skill body
`,
			wantFMKeys:      []string{"name", "description", "argument-hint"},
			wantBodyContains: "# Skill body",
		},
		{
			name: "frontmatter only no body",
			input: `---
name: no-body-skill
description: "No body content"
---
`,
			wantFMKeys: []string{"name", "description"},
		},
		{
			name: "opening delimiter not at start is treated as no frontmatter",
			input: `Some text before
---
name: trick
---
Body here
`,
			wantFMKeys:      []string{},
			wantBodyContains: "Some text before",
		},
		{
			name: "dashes in body do not affect frontmatter parsing",
			input: `---
name: my-skill
---
# Body

--- separator in body

More content.
`,
			wantFMKeys:      []string{"name"},
			wantBodyContains: "--- separator in body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := parse.ParseFrontmatter([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check expected frontmatter keys are present.
			for _, key := range tt.wantFMKeys {
				if _, ok := fm[key]; !ok {
					t.Errorf("frontmatter missing key %q; got: %v", key, fm)
				}
			}

			// If no keys expected, frontmatter map should be empty.
			if len(tt.wantFMKeys) == 0 && len(fm) != 0 {
				t.Errorf("expected empty frontmatter map but got %v", fm)
			}

			// Check body content.
			if tt.wantBodyContains != "" && !strings.Contains(body, tt.wantBodyContains) {
				t.Errorf("body does not contain %q; body: %q", tt.wantBodyContains, body)
			}
		})
	}
}

func TestParseFrontmatter_FieldValues(t *testing.T) {
	input := `---
name: git:commit
description: Automate git commits following Conventional Commits
allowed-tools:
  - Bash
  - Read
  - Edit
---
# Git Commit Skill

Skill body content here.
`

	fm, body, err := parse.ParseFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm["name"] != "git:commit" {
		t.Errorf("fm[\"name\"] = %v, want %q", fm["name"], "git:commit")
	}
	if fm["description"] == nil || fm["description"] == "" {
		t.Error("fm[\"description\"] must not be empty")
	}

	tools, ok := fm["allowed-tools"]
	if !ok {
		t.Fatal("fm[\"allowed-tools\"] must be present")
	}
	toolsList, ok := tools.([]interface{})
	if !ok {
		t.Fatalf("fm[\"allowed-tools\"] type = %T, want []interface{}", tools)
	}
	if len(toolsList) != 3 {
		t.Errorf("len(allowed-tools) = %d, want 3", len(toolsList))
	}

	if !strings.Contains(body, "# Git Commit Skill") {
		t.Errorf("body missing heading; body: %q", body)
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	input := `---
name: [invalid yaml
key: : broken
---
body
`

	_, _, err := parse.ParseFrontmatter([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML frontmatter but got none")
	}
	if !strings.Contains(err.Error(), "frontmatter") {
		t.Errorf("error %q does not contain %q", err.Error(), "frontmatter")
	}
}

func TestParseFrontmatter_EmptyFrontmatterBlock(t *testing.T) {
	// "---\n---" is valid — empty frontmatter block.
	input := "---\n---\n# Body\n"

	fm, body, err := parse.ParseFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if !strings.Contains(body, "# Body") {
		t.Errorf("body does not contain %q; body: %q", "# Body", body)
	}
}

func TestSerializeFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		frontmatter     map[string]interface{}
		body            string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "frontmatter with body produces delimited output",
			frontmatter: map[string]interface{}{
				"name":        "git:commit",
				"description": "Automate commits",
			},
			body: "# Body\n",
			wantContains: []string{
				"---\n",
				"name:",
				"description:",
				"# Body",
			},
		},
		{
			name:        "nil frontmatter returns body only",
			frontmatter: nil,
			body:        "# Just body\n",
			wantContains: []string{
				"# Just body",
			},
			wantNotContains: []string{"---"},
		},
		{
			name:        "empty frontmatter map returns body only",
			frontmatter: map[string]interface{}{},
			body:        "# Just body\n",
			wantContains: []string{
				"# Just body",
			},
			wantNotContains: []string{"---"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parse.SerializeFrontmatter(tt.frontmatter, tt.body)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			s := string(result)
			for _, want := range tt.wantContains {
				if !strings.Contains(s, want) {
					t.Errorf("output does not contain %q; output: %q", want, s)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(s, notWant) {
					t.Errorf("output must not contain %q; output: %q", notWant, s)
				}
			}
		})
	}
}

func TestParseFrontmatter_SerializeFrontmatter_RoundTrip(t *testing.T) {
	original := `---
allowed-tools:
    - Bash
    - Read
description: Automate git commits following Conventional Commits
name: git:commit
---
# Git Commit Skill

Skill body content.
`

	fm, body, err := parse.ParseFrontmatter([]byte(original))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}

	serialized, err := parse.SerializeFrontmatter(fm, body)
	if err != nil {
		t.Fatalf("SerializeFrontmatter: %v", err)
	}

	// Re-parse the serialized output.
	fm2, body2, err := parse.ParseFrontmatter(serialized)
	if err != nil {
		t.Fatalf("ParseFrontmatter (reparsed): %v", err)
	}

	// Verify keys are preserved.
	for key, val := range fm {
		if fm2[key] == nil {
			t.Errorf("round-trip lost key %q (value: %v)", key, val)
		}
	}

	// Verify body content is preserved (trimming leading/trailing whitespace differences).
	if strings.TrimSpace(body2) != strings.TrimSpace(body) {
		t.Errorf("body changed after round-trip:\noriginal: %q\nreparsed: %q", body, body2)
	}
}
