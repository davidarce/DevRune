// SPDX-License-Identifier: MIT

package steps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// TestRunToolInstallStep_NoTools verifies that RunToolInstallStep returns an
// empty result and no error when the tools slice is empty.
func TestRunToolInstallStep_NoTools(t *testing.T) {
	result, err := RunToolInstallStep([]model.ToolDef{})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(result.Installed) != 0 {
		t.Errorf("expected empty Installed, got: %v", result.Installed)
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected empty Failed, got: %v", result.Failed)
	}
}

// TestRunToolInstallStep_NoBrew verifies that RunToolInstallStep returns an
// empty result and no error when brew is not available on PATH.
// The step should be silently skipped rather than failing.
func TestRunToolInstallStep_NoBrew(t *testing.T) {
	// Clear PATH so brew cannot be found.
	t.Setenv("PATH", "")

	tools := []model.ToolDef{
		{Name: "engram", Command: "brew install gentleman-programming/tap/engram"},
	}

	result, err := RunToolInstallStep(tools)
	if err != nil {
		t.Fatalf("expected nil error when brew is absent, got: %v", err)
	}
	if len(result.Installed) != 0 {
		t.Errorf("expected empty Installed when brew absent, got: %v", result.Installed)
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected empty Failed when brew absent, got: %v", result.Failed)
	}
}

// TestIsToolInstalled_BinaryOnPath verifies that isToolInstalled returns true
// when the tool's binary is found on PATH (using a temp dummy executable).
func TestIsToolInstalled_BinaryOnPath(t *testing.T) {
	// Create a temp dir with a fake binary.
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "fake-tool")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Add temp dir to PATH.
	t.Setenv("PATH", tmp)

	td := model.ToolDef{Name: "fake", Binary: "fake-tool", Command: "brew install fake"}
	if !isToolInstalled(td) {
		t.Error("expected isToolInstalled to return true for binary on PATH")
	}
}

// TestIsToolInstalled_BinaryNotOnPath verifies that isToolInstalled returns false
// when the tool's binary is not found on PATH.
func TestIsToolInstalled_BinaryNotOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	td := model.ToolDef{Name: "missing", Binary: "nonexistent-tool-xyz", Command: "brew install missing"}
	if isToolInstalled(td) {
		t.Error("expected isToolInstalled to return false for binary not on PATH")
	}
}

// TestIsToolInstalled_EmptyBinary verifies that isToolInstalled returns false
// when the Binary field is empty (tool doesn't declare a binary check).
func TestIsToolInstalled_EmptyBinary(t *testing.T) {
	td := model.ToolDef{Name: "nobinary", Command: "brew install nobinary"}
	if isToolInstalled(td) {
		t.Error("expected isToolInstalled to return false when Binary is empty")
	}
}

// TestContains verifies the contains helper.
func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !contains(slice, "b") {
		t.Error("expected contains to find 'b'")
	}
	if contains(slice, "d") {
		t.Error("expected contains to NOT find 'd'")
	}
	if contains(nil, "a") {
		t.Error("expected contains to return false for nil slice")
	}
}
