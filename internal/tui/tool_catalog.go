// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"io/fs"
	"strings"

	devrune "github.com/davidarce/devrune"
	"github.com/davidarce/devrune/internal/model"
	"gopkg.in/yaml.v3"
)

// LoadBuiltinTools reads every .yaml file embedded under tools/ and returns
// the parsed []model.ToolDef slice. Unlike the external catalog scanner, this
// loader is strict: a YAML file that fails to parse OR whose Name field is
// empty causes an error that includes the file path, so catalog authors are
// notified of broken entries at development time.
func LoadBuiltinTools() ([]model.ToolDef, error) {
	var tools []model.ToolDef

	err := fs.WalkDir(devrune.BuiltinToolsFS, "tools", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip directories (only process files).
		if d.IsDir() {
			return nil
		}

		// Only process YAML files.
		if !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		data, err := devrune.BuiltinToolsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("tool_catalog: read %s: %w", path, err)
		}

		var tool model.ToolDef
		if err := yaml.Unmarshal(data, &tool); err != nil {
			return fmt.Errorf("tool_catalog: parse %s: %w", path, err)
		}

		if strings.TrimSpace(tool.Name) == "" {
			return fmt.Errorf("tool_catalog: %s: tool name must not be empty", path)
		}

		tools = append(tools, tool)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return tools, nil
}

// BuiltinToolMap indexes a slice of ToolDef by Name for O(1) lookup.
// Callers should load the slice once with LoadBuiltinTools and then pass it
// here to build the lookup map.
func BuiltinToolMap(tools []model.ToolDef) map[string]model.ToolDef {
	m := make(map[string]model.ToolDef, len(tools))
	for _, t := range tools {
		m[t.Name] = t
	}
	return m
}
