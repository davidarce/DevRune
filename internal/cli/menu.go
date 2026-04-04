// SPDX-License-Identifier: MIT

package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/state"
	"github.com/davidarce/devrune/internal/tui"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

// menuAction represents a user-selected action from the interactive main menu.
type menuAction string

const (
	menuActionInit      menuAction = "init"
	menuActionSync      menuAction = "sync"
	menuActionStatus    menuAction = "status"
	menuActionUpgrade   menuAction = "upgrade"
	menuActionUninstall menuAction = "uninstall"
	menuActionQuit      menuAction = "quit"
)

// RunMenu displays the interactive DevRune main menu in a loop.
// After each action completes, the menu re-displays so the user can
// pick another action. The loop exits on Ctrl+C / Esc or after
// actions that replace the process (upgrade, uninstall, new setup).
func RunMenu(cmd *cobra.Command) error {
	for {
		var selected menuAction

		form := huh.NewForm(
			huh.NewGroup(
				steps.BannerNote(),
				huh.NewSelect[menuAction]().
					Title("What would you like to do?").
					Options(
						huh.NewOption("Setup", menuActionInit),
						huh.NewOption("Sync project", menuActionSync),
						huh.NewOption("Status", menuActionStatus),
						huh.NewOption("Upgrade DevRune", menuActionUpgrade),
						huh.NewOption("Uninstall", menuActionUninstall),
						huh.NewOption("Quit", menuActionQuit),
					).
					Value(&selected),
			),
		).WithTheme(tuistyles.DevRuneThemeFunc).
			WithViewHook(func(v tea.View) tea.View {
				v.AltScreen = true
				return v
			})

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			return err
		}

		switch selected {
		case menuActionInit:
			// Setup: run TUI wizard. If user cancels/aborts, loop back to menu.
			if err := runInitFromMenu(cmd); err != nil {
				_ = showMenuMessage(cmd, "Setup Failed", err.Error())
			}
			// Loop back to menu (user cancelled or completed).

		case menuActionSync:
			// Run sync directly — output goes to terminal, then show result in TUI.
			if err := runSyncFromMenu(cmd); err != nil {
				_ = showMenuMessage(cmd, "Sync Failed", err.Error())
			} else {
				_ = showMenuMessage(cmd, "Sync Complete", "Packages resolved and installed successfully.")
			}
			// Loop back to menu.

		case menuActionStatus:
			if err := showStatusInMenu(cmd); err != nil {
				_ = showMenuMessage(cmd, "Status Error", err.Error())
			}
			// Loop back to menu.

		case menuActionUpgrade:
			// Upgrade: if user confirms, binary is replaced and we exit.
			// If user cancels, loop back to menu.
			if err := runUpgrade(cmd, nil); err != nil {
				_ = showMenuMessage(cmd, "Upgrade Failed", err.Error())
			}
			// If runUpgrade returned nil without executing (user cancelled), loop back.

		case menuActionUninstall:
			err := runUninstall(cmd, nil)
			if err == errNothingToUninstall {
				_ = showMenuMessage(cmd, "Nothing to Uninstall", "No DevRune installation found.\nRun Setup to get started.")
			} else if err == huh.ErrUserAborted {
				// User cancelled uninstall — loop back to menu silently.
			} else if err != nil {
				_ = showMenuMessage(cmd, "Uninstall Failed", err.Error())
			} else {
				_ = showMenuMessage(cmd, "Uninstalled", "All DevRune configuration has been removed.")
			}

		case menuActionQuit:
			return nil
		}
	}
}

// runSyncFromMenu runs the sync pipeline using the working directory.
// It doesn't rely on cobra flags (which aren't registered on the root cmd).
func runSyncFromMenu(cmd *cobra.Command) error {
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	out := cmd.OutOrStdout()
	manifestPath := filepath.Join(wd, "devrune.yaml")

	// Check manifest exists.
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("devrune.yaml not found — run New setup first")
	}

	// Read manifest.
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := parse.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Derive catalogs from sources and update manifest before resolving.
	// This must happen before RunResolve so the lock hash matches the installed state.
	_ = syncCatalogs(manifest, manifestPath)

	// Resolve.
	lockfile, err := RunResolve(cmd.Context(), wd, manifestPath, verbose, out)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	_ = lockfile

	// Install.
	lockPath := filepath.Join(wd, "devrune.lock")
	if err := RunInstall(cmd.Context(), wd, lockPath, manifest, verbose, out); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	return nil
}

// showStatusInMenu renders the workspace status inside a huh form (TUI).
func showStatusInMenu(cmd *cobra.Command) error {
	wd := workingDir(cmd)
	stateMgr := state.NewFileStateManager(wd)

	s, err := stateMgr.Read()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	// Build status text.
	var sb strings.Builder

	if s.SchemaVersion == "" && len(s.ActiveAgents) == 0 {
		sb.WriteString("No installation found.\nRun New setup to get started.")
	} else {
		_, _ = fmt.Fprintf(&sb, "Schema version:   %s\n", s.SchemaVersion)
		if s.InstalledAt != "" {
			_, _ = fmt.Fprintf(&sb, "Installed at:     %s\n", s.InstalledAt)
		}
		_, _ = fmt.Fprintf(&sb, "Lock hash:        %s\n", s.LockHash)

		if len(s.ActiveAgents) > 0 {
			_, _ = fmt.Fprintf(&sb, "Active agents:    %s\n", strings.Join(s.ActiveAgents, ", "))
		} else {
			sb.WriteString("Active agents:    (none)\n")
		}

		if len(s.ActiveWorkflows) > 0 {
			_, _ = fmt.Fprintf(&sb, "Active workflows: %s\n", strings.Join(s.ActiveWorkflows, ", "))
		} else {
			sb.WriteString("Active workflows: (none)\n")
		}

		_, _ = fmt.Fprintf(&sb, "Managed paths:    %d\n", len(s.ManagedPaths))

		// Staleness: compare stored manifest hash with current manifest.
		manifestPath := filepath.Join(wd, "devrune.yaml")
		if manifestData, readErr := os.ReadFile(manifestPath); readErr != nil {
			sb.WriteString("Status:           manifest not found")
		} else if m, parseErr := parse.ParseManifest(manifestData); parseErr != nil {
			sb.WriteString("Status:           manifest parse error")
		} else if serialized, serErr := parse.SerializeManifest(m); serErr != nil {
			sb.WriteString("Status:           serialization error")
		} else {
			sum := sha256.Sum256(serialized)
			currentHash := fmt.Sprintf("sha256:%x", sum)
			if currentHash == s.LockHash {
				sb.WriteString("Status:           ✓ fresh")
			} else {
				sb.WriteString("Status:           ⚠ stale (manifest changed since last install)")
			}
		}
	}

	// Show in TUI.
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title("Workspace Status").
				Description(sb.String()),
			huh.NewSelect[string]().
				Options(huh.NewOption("Back to menu", "back")).
				Value(&action),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return nil
		}
		return err
	}

	return nil
}

// showMenuMessage displays a titled message inside a TUI form with the DevRune
// banner, a note, and a "Back to menu" / "Exit" confirm button.
func showMenuMessage(cmd *cobra.Command, title, message string) error {
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			steps.BannerNote(),
			huh.NewNote().
				Title(title).
				Description(message),
			huh.NewSelect[string]().
				Options(huh.NewOption("Back to menu", "back")).
				Value(&action),
		),
	).WithTheme(tuistyles.DevRuneThemeFunc).
		WithViewHook(func(v tea.View) tea.View {
			v.AltScreen = true
			return v
		})

	if err := form.Run(); err != nil && err != huh.ErrUserAborted {
		return err
	}
	return nil
}

// runInitFromMenu runs the TUI wizard (tui.Run) and then writes the manifest
// and runs resolve+install — equivalent to runInit's interactive path.
func runInitFromMenu(cmd *cobra.Command) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	verbose := isVerbose(cmd)
	out := cmd.OutOrStdout()
	destPath := filepath.Join(wd, "devrune.yaml")

	// Check if devrune.yaml already exists and ask to overwrite.
	if _, statErr := os.Stat(destPath); statErr == nil {
		var overwrite bool
		form := huh.NewForm(
			huh.NewGroup(
				steps.BannerNote(),
				huh.NewConfirm().
					Title("devrune.yaml already exists. Overwrite it?").
					Affirmative("Yes, overwrite").
					Negative("Cancel").
					Value(&overwrite),
			),
		).WithTheme(tuistyles.DevRuneThemeFunc).
			WithViewHook(func(v tea.View) tea.View {
				v.AltScreen = true
				return v
			})
		if err := form.Run(); err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	// Run TUI wizard.
	result, err := tui.Run(nil, nil)
	if err != nil {
		if err == huh.ErrUserAborted {
			return nil
		}
		return err
	}

	manifest := result.Manifest

	if err := manifest.Validate(); err != nil {
		printError(out, "Validation failed: "+err.Error())
		return err
	}

	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		printError(out, "Serialize manifest: "+err.Error())
		return err
	}

	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		printError(out, "Write manifest: "+err.Error())
		return err
	}

	agentSummary := make([]string, 0, len(manifest.Agents))
	for _, a := range manifest.Agents {
		agentSummary = append(agentSummary, a.Name)
	}

	var lockfile model.Lockfile
	lockPath := filepath.Join(wd, "devrune.lock")

	if len(manifest.Packages) > 0 || len(manifest.MCPs) > 0 || len(manifest.Workflows) > 0 {
		// Derive catalogs from sources and update manifest before resolving.
		// This must happen before RunResolve so the lock hash matches the installed state.
		_ = syncCatalogs(manifest, destPath)

		// Run resolve + install inside the bubbletea spinner (alt-screen).
		if err := steps.RunInstallSpinner(
			func() error {
				lf, resolveErr := RunResolve(ctx, wd, destPath, verbose, nopWriter{})
				if resolveErr != nil {
					return resolveErr
				}
				lockfile = lf
				return nil
			},
			func() error {
				return RunInstall(ctx, wd, lockPath, manifest, verbose, nopWriter{})
			},
		); err != nil {
			return err
		}
	}

	// Show the completion summary screen (alt-screen, blocks until Q pressed).
	skillCount, ruleCount := countContents(lockfile)
	if err := tui.RunCompletion(tui.CompletionInfo{
		Agents:         agentSummary,
		Packages:       len(lockfile.Packages),
		MCPs:           len(lockfile.MCPs),
		Workflows:      len(lockfile.Workflows),
		Skills:         skillCount,
		Rules:          ruleCount,
		Manifest:       destPath,
		Lockfile:       lockPath,
		InstalledTools: result.InstalledTools,
	}); err != nil {
		return err
	}

	return nil
}
