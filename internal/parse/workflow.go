package parse

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/model"
)

// ParseWorkflow deserializes a workflow.yaml document from raw YAML bytes into a
// WorkflowManifest. It validates that apiVersion is "devrune/workflow/v1" and calls
// Validate() on the result.
//
// Returns a descriptive error for:
//   - Malformed YAML
//   - Missing or unsupported apiVersion
//   - Missing metadata.name
//   - Workflow with neither skills nor commands
func ParseWorkflow(data []byte) (model.WorkflowManifest, error) {
	var wf model.WorkflowManifest
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return model.WorkflowManifest{}, fmt.Errorf("workflow: failed to parse YAML: %w", err)
	}

	if wf.APIVersion == "" {
		return model.WorkflowManifest{}, fmt.Errorf(
			"workflow: apiVersion is required (expected %q)", model.WorkflowAPIVersion,
		)
	}
	if wf.APIVersion != model.WorkflowAPIVersion {
		return model.WorkflowManifest{}, fmt.Errorf(
			"workflow: unsupported apiVersion %q (expected %q)",
			wf.APIVersion, model.WorkflowAPIVersion,
		)
	}

	if err := wf.Validate(); err != nil {
		return model.WorkflowManifest{}, err
	}

	return wf, nil
}
