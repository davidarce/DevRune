// SPDX-License-Identifier: MIT

package renderers_test

import (
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
)

// TestRenderRootCatalog_EmptyInputs verifies that empty inputs produce no
// workflow registry section.
func TestRenderRootCatalog_EmptyInputs(t *testing.T) {
	out, err := renderers.RenderRootCatalog(nil, nil, nil)
	if err != nil {
		t.Fatalf("RenderRootCatalog: %v", err)
	}
	if strings.Contains(out, "## Available Workflows") {
		t.Errorf("expected no Available Workflows section with empty workflows, got one")
	}
	if strings.Contains(out, "## Conflict Resolution") {
		t.Errorf("expected no Conflict Resolution section with empty workflows, got one")
	}
}

// TestRenderRootCatalog_WithWorkflows verifies the workflow registry sections
// and per-workflow command table are rendered with the correct heading levels:
//   - "## Available Workflows" + intro
//   - "## Conflict Resolution" with priority+commands table
//   - "## {Name} Workflow" parent per workflow
func TestRenderRootCatalog_WithWorkflows(t *testing.T) {
	workflows := []model.WorkflowManifest{
		{
			Metadata: model.WorkflowMetadata{
				Name:        "sdd",
				DisplayName: "SDD (Spec-Driven Development)",
			},
			Components: model.WorkflowComponents{
				Commands: []model.WorkflowCommand{
					{Name: "sdd-explore", Action: "Start SDD explore phase", Argument: "{topic}"},
				},
			},
		},
	}

	out, err := renderers.RenderRootCatalog(workflows, nil, nil)
	if err != nil {
		t.Fatalf("RenderRootCatalog: %v", err)
	}

	if !strings.Contains(out, "## Available Workflows") {
		t.Errorf("output missing Available Workflows section")
	}
	if !strings.Contains(out, "## Conflict Resolution") {
		t.Errorf("output missing Conflict Resolution section")
	}
	if !strings.Contains(out, "## SDD (Spec-Driven Development) Workflow") {
		t.Errorf("output missing per-workflow H2 heading")
	}
	if !strings.Contains(out, "`/sdd-explore {topic}`") {
		t.Errorf("output missing sdd-explore command")
	}
}

// TestRenderRootCatalog_WithMCPInstructions verifies MCP sections are rendered.
func TestRenderRootCatalog_WithMCPInstructions(t *testing.T) {
	t.Run("instructions with own header", func(t *testing.T) {
		mcpInstructions := map[string]string{
			"my-mcp": "## My Custom Header\n\nSome instructions.\n",
		}
		out, err := renderers.RenderRootCatalog(nil, mcpInstructions, nil)
		if err != nil {
			t.Fatalf("RenderRootCatalog: %v", err)
		}
		if strings.Contains(out, "## My-mcp") {
			t.Errorf("should NOT add duplicate header when instructions already start with ##")
		}
		if !strings.Contains(out, "## My Custom Header") {
			t.Errorf("should preserve the header from instructions")
		}
		if !strings.Contains(out, "Some instructions.") {
			t.Errorf("should include instruction content")
		}
	})

	t.Run("instructions without header", func(t *testing.T) {
		mcpInstructions := map[string]string{
			"my-mcp": "Use this tool for searching.\n",
		}
		out, err := renderers.RenderRootCatalog(nil, mcpInstructions, nil)
		if err != nil {
			t.Fatalf("RenderRootCatalog: %v", err)
		}
		if !strings.Contains(out, "## My-mcp") {
			t.Errorf("should generate header when instructions don't start with ##")
		}
	})

	t.Run("empty instructions skipped", func(t *testing.T) {
		mcpInstructions := map[string]string{
			"my-mcp": "   ",
		}
		out, err := renderers.RenderRootCatalog(nil, mcpInstructions, nil)
		if err != nil {
			t.Fatalf("RenderRootCatalog: %v", err)
		}
		if strings.Contains(out, "my-mcp") {
			t.Errorf("should skip MCP section when instructions are empty/whitespace")
		}
	})
}

// TestRenderRootCatalog_WithRegistryContents verifies registry content is injected.
func TestRenderRootCatalog_WithRegistryContents(t *testing.T) {
	workflows := []model.WorkflowManifest{
		{
			Metadata:   model.WorkflowMetadata{Name: "sdd"},
			Components: model.WorkflowComponents{},
		},
	}
	registryContents := map[string]string{
		"sdd": "## SDD — Evaluation Gate (HIGHEST PRIORITY)\n\nBefore starting ANY implementation...\n",
	}

	out, err := renderers.RenderRootCatalog(workflows, nil, registryContents)
	if err != nil {
		t.Fatalf("RenderRootCatalog: %v", err)
	}

	if !strings.Contains(out, "SDD — Evaluation Gate") {
		t.Errorf("output missing registry content injection")
	}
}

// TestRenderRootCatalog_ReturnsString verifies the function returns a string (not writing to file).
func TestRenderRootCatalog_ReturnsString(t *testing.T) {
	workflows := []model.WorkflowManifest{
		{
			Metadata:   model.WorkflowMetadata{Name: "test", DisplayName: "Test Workflow"},
			Components: model.WorkflowComponents{},
		},
	}

	out, err := renderers.RenderRootCatalog(workflows, nil, nil)
	if err != nil {
		t.Fatalf("RenderRootCatalog: %v", err)
	}
	if out == "" {
		t.Errorf("expected non-empty string result")
	}
}

// TestRenderRootCatalog_CatalogMarkers verifies the exported constants have expected values.
func TestRenderRootCatalog_CatalogMarkers(t *testing.T) {
	if renderers.CatalogBeginMarker == "" {
		t.Errorf("CatalogBeginMarker should not be empty")
	}
	if renderers.CatalogEndMarker == "" {
		t.Errorf("CatalogEndMarker should not be empty")
	}
	if !strings.Contains(renderers.CatalogBeginMarker, "devrune managed") {
		t.Errorf("CatalogBeginMarker should contain 'devrune managed', got %q", renderers.CatalogBeginMarker)
	}
	if !strings.Contains(renderers.CatalogEndMarker, "devrune managed") {
		t.Errorf("CatalogEndMarker should contain 'devrune managed', got %q", renderers.CatalogEndMarker)
	}
}
