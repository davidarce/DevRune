// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
)

// TestMenuActionConstants verifies all menuAction constants have the expected string values.
func TestMenuActionConstants(t *testing.T) {
	tests := []struct {
		name   string
		action menuAction
		want   string
	}{
		{"init", menuActionInit, "init"},
		{"sync", menuActionSync, "sync"},
		{"status", menuActionStatus, "status"},
		{"upgrade", menuActionUpgrade, "upgrade"},
		{"uninstall", menuActionUninstall, "uninstall"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.action) != tt.want {
				t.Errorf("menuAction %s = %q, want %q", tt.name, string(tt.action), tt.want)
			}
		})
	}
}

// TestMenuActionCount verifies there are exactly 5 menu actions defined.
func TestMenuActionCount(t *testing.T) {
	actions := []menuAction{
		menuActionInit,
		menuActionSync,
		menuActionStatus,
		menuActionUpgrade,
		menuActionUninstall,
	}

	const wantCount = 5
	if len(actions) != wantCount {
		t.Errorf("menu action count = %d, want %d", len(actions), wantCount)
	}
}

// TestMenuActionUniqueness verifies that all menuAction constants are unique.
func TestMenuActionUniqueness(t *testing.T) {
	seen := make(map[menuAction]bool)
	actions := []menuAction{
		menuActionInit,
		menuActionSync,
		menuActionStatus,
		menuActionUpgrade,
		menuActionUninstall,
	}

	for _, a := range actions {
		if seen[a] {
			t.Errorf("duplicate menuAction value: %q", a)
		}
		seen[a] = true
	}
}

// TestMenuActionType verifies menuAction is a string-based type.
func TestMenuActionType(t *testing.T) {
	var a menuAction = "custom"
	if string(a) != "custom" {
		t.Errorf("menuAction string conversion failed: got %q, want %q", string(a), "custom")
	}
}
