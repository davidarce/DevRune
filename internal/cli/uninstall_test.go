// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/state"
)

// newTestCmd builds a minimal cobra.Command wired to the given stdout buffer,
// with the same persistent flags as the real root command.
func newTestCmd(out *bytes.Buffer, dir string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("non-interactive", false, "")
	cmd.PersistentFlags().Bool("verbose", false, "")
	cmd.PersistentFlags().String("dir", dir, "")
	cmd.SetOut(out)
	return cmd
}

// TestUninstall_NoState verifies that runUninstall prints "Nothing to uninstall."
// and returns errNothingToUninstall when no .devrune/state.yaml file exists.
func TestUninstall_NoState(t *testing.T) {
	tmpDir := t.TempDir()

	var out bytes.Buffer
	cmd := newTestCmd(&out, tmpDir)

	err := runUninstall(cmd, nil)
	if err != errNothingToUninstall {
		t.Fatalf("runUninstall: expected errNothingToUninstall, got: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Nothing to uninstall.") {
		t.Errorf("output = %q, want it to contain %q", got, "Nothing to uninstall.")
	}
}

// TestUninstall_NonInteractive verifies that runUninstall returns an error
// when --non-interactive flag is set (interactive confirmation cannot be shown).
func TestUninstall_NonInteractive(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal state file so uninstall doesn't short-circuit.
	stateMgr := state.NewFileStateManager(tmpDir)
	s := state.State{
		SchemaVersion: "devrune/state/v1",
		ManagedPaths:  []string{"some/file.txt"},
	}
	if err := stateMgr.Write(s); err != nil {
		t.Fatalf("Write state: %v", err)
	}

	var out bytes.Buffer
	cmd := newTestCmd(&out, tmpDir)
	if err := cmd.ParseFlags([]string{"--non-interactive"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	err := runUninstall(cmd, nil)
	if err == nil {
		t.Fatal("runUninstall: expected an error in non-interactive mode, got nil")
	}
	if !strings.Contains(err.Error(), "interactive confirmation") {
		t.Errorf("error = %q, want it to mention interactive confirmation", err.Error())
	}
}

// TestCleanManagedBlock_RemovesNewMarkers verifies that cleanManagedBlock
// strips the ">>> devrune managed" block from files.
func TestCleanManagedBlock_RemovesNewMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	content := "# my project\n# >>> devrune managed — do not edit\n.devrune/\n.claude/\n# <<< devrune managed\n*.log\n"
	if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := cleanManagedBlock(tmpDir, ".gitignore"); err != nil {
		t.Fatalf("cleanManagedBlock: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got := string(data)
	if strings.Contains(got, ">>> devrune managed") {
		t.Errorf("start marker still present: %q", got)
	}
	if strings.Contains(got, "<<< devrune managed") {
		t.Errorf("end marker still present: %q", got)
	}
	if strings.Contains(got, ".devrune/") {
		t.Errorf("managed entry still present: %q", got)
	}
	if !strings.Contains(got, "# my project") {
		t.Errorf("pre-existing line removed: %q", got)
	}
	if !strings.Contains(got, "*.log") {
		t.Errorf("post-block line removed: %q", got)
	}
}

// TestCleanManagedBlock_RemovesLegacyMarkers verifies that cleanManagedBlock
// also strips legacy "devrune:start/devrune:end" blocks.
func TestCleanManagedBlock_RemovesLegacyMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	content := "# my project\n# devrune:start\n.devrune/\n# devrune:end\n*.log\n"
	if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := cleanManagedBlock(tmpDir, ".gitignore"); err != nil {
		t.Fatalf("cleanManagedBlock: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got := string(data)
	if strings.Contains(got, "devrune:start") {
		t.Errorf("start marker still present: %q", got)
	}
}

// TestCleanManagedBlock_DeletesEmptyFile verifies that if cleaning markers
// leaves the file empty, it gets deleted entirely.
func TestCleanManagedBlock_DeletesEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")

	content := "# >>> devrune managed — do not edit\nsome content\n# <<< devrune managed\n"
	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := cleanManagedBlock(tmpDir, "AGENTS.md"); err != nil {
		t.Fatalf("cleanManagedBlock: %v", err)
	}

	if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
		t.Error("AGENTS.md should be deleted when only managed content remains")
	}
}

// TestCleanManagedBlock_NoFile verifies cleanManagedBlock returns nil when
// the file does not exist.
func TestCleanManagedBlock_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	if err := cleanManagedBlock(tmpDir, ".gitignore"); err != nil {
		t.Errorf("cleanManagedBlock with no file: unexpected error: %v", err)
	}
}
