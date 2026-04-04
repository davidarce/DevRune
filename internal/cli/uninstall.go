// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/state"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// errNothingToUninstall is returned when no DevRune installation is found.
var errNothingToUninstall = errors.New("nothing to uninstall")

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove all DevRune-managed files from the workspace",
		Long: `Uninstall removes all files that DevRune installed into the workspace:
agent configuration directories (.claude/, .agents/, .codex/, .opencode/,
.factory/, .github/ managed files), devrune.yaml, devrune.lock, and the
.devrune/ state directory.

The devrune binary itself is NOT removed. Use your package manager or
delete it manually if desired.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runUninstall(cmd, args)
			if err == huh.ErrUserAborted {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
			return err
		},
	}
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	wd := workingDir(cmd)
	out := cmd.OutOrStdout()
	nonInteractive := isNonInteractive(cmd)

	stateMgr := state.NewFileStateManager(wd)
	s, err := stateMgr.Read()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	// Zero-value state means no installation exists.
	if s.SchemaVersion == "" && len(s.ManagedPaths) == 0 {
		_, _ = fmt.Fprintln(out, "Nothing to uninstall.")
		return errNothingToUninstall
	}

	managedCount := len(s.ManagedPaths)

	if nonInteractive {
		return fmt.Errorf("uninstall requires interactive confirmation; use interactive mode or confirm manually")
	}

	// Interactive confirmation via huh.Confirm.
	var confirmed bool
	confirmTitle := fmt.Sprintf(
		"Remove %d managed file(s), devrune.yaml, devrune.lock, and .devrune/? This cannot be undone.",
		managedCount,
	)
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewConfirm().
				Title(confirmTitle).
				Affirmative("Yes, remove everything").
				Negative("Cancel").
				Value(&confirmed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return huh.ErrUserAborted
		}
		return fmt.Errorf("confirmation form: %w", err)
	}

	if !confirmed {
		return huh.ErrUserAborted
	}

	// Remove managed paths from state.
	removed := 0
	for _, p := range s.ManagedPaths {
		target := p
		if !filepath.IsAbs(target) {
			target = filepath.Join(wd, p)
		}
		if removeErr := os.RemoveAll(target); removeErr != nil && !os.IsNotExist(removeErr) {
			_, _ = fmt.Fprintf(out, "%s\n", tuistyles.StyleInfo.Render(fmt.Sprintf("  warning: could not remove %s: %v", target, removeErr)))
		} else if removeErr == nil {
			removed++
		}
	}

	// Remove DevRune config files.
	for _, rel := range []string{"devrune.yaml", "devrune.lock", ".mcp.json"} {
		target := filepath.Join(wd, rel)
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			_, _ = fmt.Fprintf(out, "%s\n", tuistyles.StyleInfo.Render(fmt.Sprintf("  warning: could not remove %s: %v", target, removeErr)))
		}
	}

	// Remove .devrune/ state directory entirely.
	devruneDir := filepath.Join(wd, ".devrune")
	if removeErr := os.RemoveAll(devruneDir); removeErr != nil {
		_, _ = fmt.Fprintf(out, "%s\n", tuistyles.StyleInfo.Render(fmt.Sprintf("  warning: could not remove .devrune/: %v", removeErr)))
	}

	// Clean DevRune-managed markers from .gitignore, AGENTS.md, and CLAUDE.md.
	for _, file := range []string{".gitignore", "AGENTS.md", "CLAUDE.md"} {
		if cleanErr := cleanManagedBlock(wd, file); cleanErr != nil {
			_, _ = fmt.Fprintf(out, "%s\n", tuistyles.StyleInfo.Render(fmt.Sprintf("  warning: could not clean %s: %v", file, cleanErr)))
		}
	}

	// Remove empty workspace directories left behind after managed path cleanup.
	for _, dir := range []string{".claude", ".agents", ".codex", ".opencode", ".factory", ".github", ".vscode"} {
		target := filepath.Join(wd, dir)
		// os.Remove only succeeds on empty directories — safe to call always.
		_ = os.Remove(target)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Render(fmt.Sprintf("  ✓ Uninstalled %d managed file(s). DevRune configuration removed.", removed)))
	_, _ = fmt.Fprintln(out)

	return nil
}

// cleanManagedBlock removes DevRune-managed marker blocks from a file.
// Supports both marker formats:
//   - "# >>> devrune managed — do not edit" / "# <<< devrune managed" (gitignore, AGENTS.md)
//   - "# devrune:start" / "# devrune:end" (legacy gitignore)
//
// If the file becomes empty (only whitespace) after cleaning, it is deleted.
func cleanManagedBlock(wd, filename string) error {
	filePath := filepath.Join(wd, filename)
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Pairs of start/end markers to strip.
	markerPairs := [][2]string{
		{"# >>> devrune managed — do not edit", "# <<< devrune managed"},
		{"# devrune:start", "# devrune:end"},
	}

	var lines []string
	inBlock := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a managed block.
		isStart := false
		isEnd := false
		for _, pair := range markerPairs {
			if trimmed == pair[0] {
				isStart = true
				break
			}
			if trimmed == pair[1] {
				isEnd = true
				break
			}
		}

		if isStart {
			inBlock = true
			continue
		}
		if isEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	result := strings.Join(lines, "\n")

	// If file is now empty (only whitespace), delete it entirely.
	if strings.TrimSpace(result) == "" {
		return os.Remove(filePath)
	}

	// Preserve trailing newline if original had one.
	if len(data) > 0 && data[len(data)-1] == '\n' && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return os.WriteFile(filePath, []byte(result), 0o644)
}
