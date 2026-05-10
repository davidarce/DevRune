// SPDX-License-Identifier: MIT

package tui_test

import (
	"testing"

	"github.com/davidarce/devrune/internal/tui"
)

func TestLoadBuiltinTools_returnsEngranAndCrit(t *testing.T) {
	tools, err := tui.LoadBuiltinTools()
	if err != nil {
		t.Fatalf("LoadBuiltinTools() error: %v", err)
	}

	if len(tools) < 2 {
		t.Fatalf("expected at least 2 built-in tools, got %d", len(tools))
	}

	m := tui.BuiltinToolMap(tools)
	for _, name := range []string{"engram", "crit"} {
		tool, ok := m[name]
		if !ok {
			t.Errorf("expected built-in tool %q not found in catalog", name)
			continue
		}
		if tool.Name != name {
			t.Errorf("tool.Name = %q, want %q", tool.Name, name)
		}
		if tool.Command == "" {
			t.Errorf("tool %q has empty command — built-in catalog entries must have a default command", name)
		}
	}
}

func TestBuiltinToolMap_keyedByName(t *testing.T) {
	tools, err := tui.LoadBuiltinTools()
	if err != nil {
		t.Fatalf("LoadBuiltinTools() error: %v", err)
	}

	m := tui.BuiltinToolMap(tools)
	for _, tool := range tools {
		got, ok := m[tool.Name]
		if !ok {
			t.Errorf("BuiltinToolMap missing key %q", tool.Name)
		}
		if got.Name != tool.Name {
			t.Errorf("map[%q].Name = %q", tool.Name, got.Name)
		}
	}
}
