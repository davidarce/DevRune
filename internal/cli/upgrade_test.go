// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
)

// TestNewUpgradeCmd verifies the upgrade command is configured correctly.
func TestNewUpgradeCmd(t *testing.T) {
	cmd := newUpgradeCmd()

	if cmd.Use != "upgrade" {
		t.Errorf("Use = %q, want %q", cmd.Use, "upgrade")
	}
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE must be set")
	}
}

// TestUpgradeRegistered verifies upgrade is a subcommand of root.
func TestUpgradeRegistered(t *testing.T) {
	root := NewRootCmd("v1.0.0", "abc")
	found, _, err := root.Find([]string{"upgrade"})
	if err != nil {
		t.Fatalf("Find upgrade: %v", err)
	}
	if found == nil || found.Use != "upgrade" {
		t.Error("upgrade subcommand not registered on root")
	}
}
