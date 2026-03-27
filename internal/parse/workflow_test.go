// SPDX-License-Identifier: MIT

package parse_test

import (
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/parse"
)

func TestParseWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid SDD workflow with entrypoint",
			fixture: "valid-sdd.yaml",
			wantErr: false,
		},
		{
			name:    "valid minimal workflow",
			fixture: "valid-minimal.yaml",
			wantErr: false,
		},
		{
			name:    "valid workflow without entrypoint",
			fixture: "valid-no-entrypoint.yaml",
			wantErr: false,
		},
		{
			name:        "missing metadata name returns error",
			fixture:     "invalid-no-name.yaml",
			wantErr:     true,
			errContains: "metadata.name is required",
		},
		{
			name:        "unsupported apiVersion returns error",
			fixture:     "invalid-bad-schema.yaml",
			wantErr:     true,
			errContains: "unsupported apiVersion",
		},
		{
			name:        "workflow with no skills or commands returns error",
			fixture:     "invalid-no-skills-or-commands.yaml",
			wantErr:     true,
			errContains: "at least one skill or command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, "workflows", tt.fixture)

			result, err := parse.ParseWorkflow(data)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none; result: %+v", result)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseWorkflow_ValidSDD_Fields(t *testing.T) {
	data := mustReadFixture(t, "workflows", "valid-sdd.yaml")

	wf, err := parse.ParseWorkflow(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wf.APIVersion != "devrune/workflow/v1" {
		t.Errorf("APIVersion = %q, want %q", wf.APIVersion, "devrune/workflow/v1")
	}
	if wf.Metadata.Name != "sdd" {
		t.Errorf("Metadata.Name = %q, want %q", wf.Metadata.Name, "sdd")
	}
	if wf.Metadata.Version != "1.0.0" {
		t.Errorf("Metadata.Version = %q, want %q", wf.Metadata.Version, "1.0.0")
	}
	if len(wf.Components.Skills) != 4 {
		t.Errorf("len(Components.Skills) = %d, want 4", len(wf.Components.Skills))
	}
	if wf.Components.Entrypoint != "ORCHESTRATOR.md" {
		t.Errorf("Components.Entrypoint = %q, want %q", wf.Components.Entrypoint, "ORCHESTRATOR.md")
	}
	if len(wf.Components.Commands) != 4 {
		t.Errorf("len(Components.Commands) = %d, want 4", len(wf.Components.Commands))
	}

	// Check that first command has the expected fields.
	cmd := wf.Components.Commands[0]
	if cmd.Name != "sdd-explore" {
		t.Errorf("Commands[0].Name = %q, want %q", cmd.Name, "sdd-explore")
	}
	if cmd.Action == "" {
		t.Error("Commands[0].Action must not be empty")
	}
	if cmd.Argument == "" {
		t.Error("Commands[0].Argument must not be empty")
	}
}

func TestParseWorkflow_ValidMinimal_Fields(t *testing.T) {
	data := mustReadFixture(t, "workflows", "valid-minimal.yaml")

	wf, err := parse.ParseWorkflow(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wf.Metadata.Name != "code-review" {
		t.Errorf("Metadata.Name = %q, want %q", wf.Metadata.Name, "code-review")
	}
	if len(wf.Components.Skills) != 1 {
		t.Errorf("len(Skills) = %d, want 1", len(wf.Components.Skills))
	}
	// Entrypoint is optional — minimal has no entrypoint.
	if wf.Components.Entrypoint != "" {
		t.Errorf("Components.Entrypoint = %q, want empty", wf.Components.Entrypoint)
	}
}

func TestParseWorkflow_MissingAPIVersion(t *testing.T) {
	data := []byte(`metadata:
  name: test-wf
  version: "1.0.0"
components:
  skills:
    - some-skill
`)

	_, err := parse.ParseWorkflow(data)
	if err == nil {
		t.Fatal("expected error for missing apiVersion but got none")
	}
	if !strings.Contains(err.Error(), "apiVersion is required") {
		t.Errorf("error %q does not contain %q", err.Error(), "apiVersion is required")
	}
}

func TestParseWorkflow_MalformedYAML(t *testing.T) {
	data := []byte("apiVersion: devrune/workflow/v1\nmetadata:\n  name: [invalid")

	_, err := parse.ParseWorkflow(data)
	if err == nil {
		t.Fatal("expected error for malformed YAML but got none")
	}
}

func TestParseWorkflow_SkillsOnly_NoCommands(t *testing.T) {
	// A workflow with only skills and no commands is valid.
	data := []byte(`apiVersion: devrune/workflow/v1
metadata:
  name: skills-only
  version: "1.0.0"
components:
  skills:
    - my-skill
`)

	wf, err := parse.ParseWorkflow(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Metadata.Name != "skills-only" {
		t.Errorf("Metadata.Name = %q, want %q", wf.Metadata.Name, "skills-only")
	}
}

func TestParseWorkflow_CommandsOnly_NoSkills(t *testing.T) {
	// A workflow with only commands and no skills is valid.
	data := []byte(`apiVersion: devrune/workflow/v1
metadata:
  name: commands-only
  version: "1.0.0"
components:
  commands:
    - name: do-something
      action: "Does something"
`)

	wf, err := parse.ParseWorkflow(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Metadata.Name != "commands-only" {
		t.Errorf("Metadata.Name = %q, want %q", wf.Metadata.Name, "commands-only")
	}
	if len(wf.Components.Commands) != 1 {
		t.Errorf("len(Commands) = %d, want 1", len(wf.Components.Commands))
	}
}
