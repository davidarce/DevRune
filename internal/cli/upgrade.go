// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade DevRune to the latest version",
		Long: `Upgrade downloads and installs the latest version of DevRune by running
the official install script from GitHub.`,
		RunE: runUpgrade,
	}
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	// Print current version.
	currentVersion := cmd.Root().Version
	if currentVersion == "" {
		currentVersion = "(unknown)"
	}
	// Confirm before executing remote script.
	var proceed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title("Upgrade DevRune").
				Description(fmt.Sprintf("Current version: %s", currentVersion)),
			huh.NewConfirm().
				Title("Download and install the latest version?").
				Description("This will run the official install script from GitHub.").
				Affirmative("Yes, upgrade").
				Negative("Cancel").
				Value(&proceed),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := confirmForm.Run(); err != nil || !proceed {
		_, _ = fmt.Fprintln(out, "  Upgrade cancelled.")
		return nil
	}

	_, _ = fmt.Fprintln(out, tuistyles.StyleSubtitle.Render("  Upgrading DevRune..."))
	_, _ = fmt.Fprintln(out)

	// Execute the install script via bash.
	// The script is piped from curl to bash, matching the official installation method.
	installURL := "https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh"
	upgradeCmd := exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", installURL))
	upgradeCmd.Stdout = out
	upgradeCmd.Stderr = os.Stderr

	if err := upgradeCmd.Run(); err != nil {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, tuistyles.StyleError.Render(fmt.Sprintf("  ✗ Upgrade failed: %v", err)))
		return fmt.Errorf("upgrade: %w", err)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Render("  ✓ DevRune updated. Run `devrune` again to continue."))
	_, _ = fmt.Fprintln(out)

	return nil
}
