// SPDX-License-Identifier: MIT

package materialize

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune"
	"github.com/davidarce/devrune/internal/materialize/renderers"
	"github.com/davidarce/devrune/internal/model"
)

// LoadBuiltinAgents reads all agent definition YAML files embedded in the binary
// and returns the parsed AgentDefinition slice.
//
// The embedded files are resolved from agents/*.yaml relative to the module root.
// Each file must contain a valid AgentDefinition YAML document.
func LoadBuiltinAgents() ([]model.AgentDefinition, error) {
	var agents []model.AgentDefinition

	err := fs.WalkDir(devrune.BuiltinAgentsFS, "agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		data, err := devrune.BuiltinAgentsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load built-in agent %q: %w", path, err)
		}

		var def model.AgentDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("parse built-in agent %q: %w", path, err)
		}
		if err := def.Validate(); err != nil {
			return fmt.Errorf("invalid built-in agent %q: %w", path, err)
		}
		agents = append(agents, def)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load built-in agents: %w", err)
	}
	return agents, nil
}

// NewRendererRegistry constructs a map[agentName]AgentRenderer from a slice of
// AgentDefinition values. The renderer type is selected by AgentDefinition.Type.
//
// Supported types: "claude", "opencode", "copilot", "factory".
// Returns an error if an unknown type is encountered.
func NewRendererRegistry(agents []model.AgentDefinition) (map[string]AgentRenderer, error) {
	registry := make(map[string]AgentRenderer, len(agents))
	for _, def := range agents {
		renderer, err := newRenderer(def)
		if err != nil {
			return nil, fmt.Errorf("renderer registry: %w", err)
		}
		registry[def.Name] = renderer
	}
	return registry, nil
}

// newRenderer constructs the appropriate AgentRenderer for the given AgentDefinition.
func newRenderer(def model.AgentDefinition) (AgentRenderer, error) {
	switch def.Type {
	case "claude":
		return renderers.NewClaudeRenderer(def), nil
	case "opencode":
		return renderers.NewOpenCodeRenderer(def), nil
	case "copilot":
		return renderers.NewCopilotRenderer(def), nil
	case "factory":
		return renderers.NewFactoryRenderer(def), nil
	default:
		return nil, fmt.Errorf("unknown agent type %q for agent %q", def.Type, def.Name)
	}
}

// LoadDefaultRegistry is a convenience function that loads the built-in agent
// definitions and constructs the full renderer registry in one call.
// This is the primary entry point for production use.
func LoadDefaultRegistry() (map[string]AgentRenderer, error) {
	agents, err := LoadBuiltinAgents()
	if err != nil {
		return nil, err
	}
	return NewRendererRegistry(agents)
}
