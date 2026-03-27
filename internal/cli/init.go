package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
	"github.com/davidarce/devrune/internal/tui"
	"github.com/davidarce/devrune/internal/tui/steps"
	"github.com/davidarce/devrune/internal/tui/tuistyles"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize, resolve, and install in one step (interactive TUI wizard or --non-interactive)",
		Long: `Initialize a new devrune.yaml manifest for the current project, then automatically
resolve packages and install the workspace.

In interactive mode, a TUI wizard guides you through selecting agents and repository
sources. In non-interactive mode, use flags to specify all options.`,
		RunE:          runInit,
		SilenceErrors: true, // we handle error display ourselves
		SilenceUsage:  true,
	}

	cmd.Flags().StringSlice("agents", nil, "Agent names to configure (e.g. claude,opencode)")
	cmd.Flags().StringArray("source", nil, "Repository source refs (repeatable, e.g. github:owner/repo@v1)")
	cmd.Flags().StringArray("mcp", nil, "MCP server source refs (repeatable)")
	cmd.Flags().StringArray("workflow", nil, "Workflow source refs (repeatable)")
	cmd.Flags().Bool("force", false, "Overwrite existing devrune.yaml without prompting")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	wd := workingDir(cmd)
	nonInteractive := isNonInteractive(cmd)
	verbose := isVerbose(cmd)
	force, _ := cmd.Flags().GetBool("force")

	agentNames, _ := cmd.Flags().GetStringSlice("agents")
	sources, _ := cmd.Flags().GetStringArray("source")
	mcps, _ := cmd.Flags().GetStringArray("mcp")
	workflows, _ := cmd.Flags().GetStringArray("workflow")

	// Determine whether any flags were explicitly provided.
	hasFlags := cmd.Flags().Changed("agents") ||
		cmd.Flags().Changed("source") ||
		cmd.Flags().Changed("mcp") ||
		cmd.Flags().Changed("workflow")

	out := cmd.OutOrStdout()
	destPath := filepath.Join(wd, "devrune.yaml")

	// Check if devrune.yaml already exists.
	if _, statErr := os.Stat(destPath); statErr == nil {
		if nonInteractive || hasFlags {
			if !force {
				printError(out, fmt.Sprintf("%s already exists — use --force to overwrite", destPath))
				return fmt.Errorf("%s already exists", destPath)
			}
		} else {
			// Interactive mode: ask user.
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
				_, _ = fmt.Fprintln(out, "Aborted.")
				return nil
			}
		}
	}

	var manifest model.UserManifest

	if !nonInteractive && !hasFlags {
		// Interactive TUI wizard path.
		var tuiErr error
		manifest, tuiErr = tui.Run()
		if tuiErr != nil {
			if errors.Is(tuiErr, huh.ErrUserAborted) {
				_, _ = fmt.Fprintln(out, "Aborted.")
				return nil
			}
			return fmt.Errorf("tui wizard: %w", tuiErr)
		}
	} else {
		// Non-interactive / flag-based path.
		agentRefs := make([]model.AgentRef, 0, len(agentNames))
		for _, name := range agentNames {
			if name != "" {
				agentRefs = append(agentRefs, model.AgentRef{Name: name})
			}
		}

		pkgRefs := make([]model.PackageRef, 0, len(sources))
		for _, src := range sources {
			if src != "" {
				pkgRefs = append(pkgRefs, model.PackageRef{Source: src})
			}
		}

		mcpRefs := make([]model.MCPRef, 0, len(mcps))
		for _, m := range mcps {
			if m != "" {
				mcpRefs = append(mcpRefs, model.MCPRef{Source: m})
			}
		}

		manifest = model.UserManifest{
			SchemaVersion: "devrune/v1",
			Agents:        agentRefs,
			Packages:      pkgRefs,
			MCPs:          mcpRefs,
			Workflows:     workflows,
		}
	}

	if err := manifest.Validate(); err != nil {
		printError(out, "Validation failed: "+err.Error())
		return err
	}

	data, err := parse.SerializeManifest(manifest)
	if err != nil {
		printError(out, "Serialize manifest: "+err.Error())
		return err
	}

	// --- Styled installation output ---
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSubtitle.Render("  Installing..."))
	_, _ = fmt.Fprintln(out)

	// Write manifest.
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		printError(out, "Write manifest: "+err.Error())
		return err
	}
	printDone(out, "Manifest written: "+destPath)

	agentSummary := make([]string, 0, len(manifest.Agents))
	for _, a := range manifest.Agents {
		agentSummary = append(agentSummary, a.Name)
	}

	// Resolve + install if there are packages to fetch.
	if len(manifest.Packages) > 0 || len(manifest.MCPs) > 0 || len(manifest.Workflows) > 0 {
		// Resolve.
		printProgress(out, "Resolving packages...")
		lockfile, err := RunResolve(ctx, wd, destPath, verbose, nopWriter{})
		if err != nil {
			printError(out, "Resolve failed: "+err.Error())
			return err
		}
		skillCount, ruleCount := countContents(lockfile)
		printDone(out, fmt.Sprintf("Resolved %d package(s), %d MCP(s), %d workflow(s) — %d skills, %d rules",
			len(lockfile.Packages), len(lockfile.MCPs), len(lockfile.Workflows), skillCount, ruleCount))

		// Install.
		printProgress(out, "Installing workspace...")
		lockPath := filepath.Join(wd, "devrune.lock")
		if err := RunInstall(ctx, wd, lockPath, manifest, verbose, nopWriter{}); err != nil {
			printError(out, "Install failed: "+err.Error())
			return err
		}
		printDone(out, fmt.Sprintf("Installed for agents: %s", strings.Join(agentSummary, ", ")))

		_ = lockfile
	} else {
		_, _ = fmt.Fprintln(out, tuistyles.StyleInfo.Render("  No packages to resolve."))
	}

	// --- Interactive completion screen ---
	if !nonInteractive && !hasFlags {
		info := tui.CompletionInfo{
			Agents:    agentSummary,
			Packages:  len(manifest.Packages),
			MCPs:      len(manifest.MCPs),
			Workflows: len(manifest.Workflows),
			Manifest:  destPath,
			Lockfile:  filepath.Join(wd, "devrune.lock"),
		}
		return tui.RunCompletion(info)
	}

	// Non-interactive: plain text summary.
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Render("  Installation complete!"))
	_, _ = fmt.Fprintln(out)
	printSummaryLine(out, "Agents", strings.Join(agentSummary, ", "))
	if len(manifest.Packages) > 0 {
		printSummaryLine(out, "Repos", fmt.Sprintf("%d", len(manifest.Packages)))
	}
	if len(manifest.MCPs) > 0 {
		printSummaryLine(out, "MCPs", fmt.Sprintf("%d", len(manifest.MCPs)))
	}
	if len(manifest.Workflows) > 0 {
		printSummaryLine(out, "Workflows", fmt.Sprintf("%d", len(manifest.Workflows)))
	}
	printSummaryLine(out, "Manifest", destPath)
	printSummaryLine(out, "Lockfile", filepath.Join(wd, "devrune.lock"))
	_, _ = fmt.Fprintln(out)

	return nil
}

// printProgress writes a styled "in progress" step line.
func printProgress(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, tuistyles.StyleInfo.Render("  ⧗ "+msg))
}

// printDone writes a styled "completed" step line with a green checkmark.
func printDone(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, tuistyles.StyleSuccess.Render("  ✓ ")+tuistyles.StyleSummaryValue.Render(msg))
}

// printError writes a styled error line with a red cross.
func printError(out io.Writer, msg string) {
	_, _ = fmt.Fprintln(out, tuistyles.StyleError.Render("  ✗ "+msg))
}

// printSummaryLine writes a styled key-value summary line.
func printSummaryLine(out io.Writer, key, value string) {
	padded := fmt.Sprintf("%-12s", key+":")
	_, _ = fmt.Fprintln(out, "  "+tuistyles.StyleSummaryKey.Render(padded)+tuistyles.StyleSummaryValue.Render(value))
}

// nopWriter discards all writes. Used to suppress pipeline output when the
// init command provides its own styled output.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
