package tui

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// Run executes the interactive TUI wizard and returns the resulting
// UserManifest on success.
//
// The wizard walks the user through four sequential steps:
//
//	Step 1: Agent selection
//	Step 2: Repository source refs (one at a time)
//	Step 3: Scan + category/item selection
//	Step 4: Summary + confirmation
//
// If the user aborts at any step (Ctrl-C or declining confirmation) the
// function returns huh.ErrUserAborted.
func Run() (model.UserManifest, error) {
	// Step 1 — agents (alt screen, step indicator inside form)
	agents, err := steps.SelectAgents()
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	// Step 2 — repository sources (alt screen, step indicator inside form)
	sources, err := steps.EnterRepositories()
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	// Step 3 — scan + select (alt screen)
	var selection steps.SelectionResult

	if len(sources) > 0 {
		cp := cachePath()

		// Wrap ScanRepositories so the steps package can call it without
		// importing the tui package (avoids import cycle).
		scanFn := func(ctx context.Context, srcs []string, cachePath string) ([]steps.ScanResult, error) {
			scanned, err := ScanRepositories(ctx, srcs, cachePath)
			if err != nil {
				return nil, err
			}
			out := make([]steps.ScanResult, len(scanned))
			for i, r := range scanned {
				out[i] = steps.ScanResult{
					Source:    r.Source,
					Skills:    r.Skills,
					Rules:     r.Rules,
					MCPs:      r.MCPs,
					Workflows: r.Workflows,
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
					Error:     r.Error,
				}
			}
			return out, nil
		}

		scanned, warnings, err := steps.RunScanModel(sources, scanFn, cp)
		if err != nil {
			return model.UserManifest{}, fmt.Errorf("scan repositories: %w", err)
		}

		// Report any per-repo scan warnings.
		for _, w := range warnings {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", w)
		}

		// Convert to ScannedRepoInput for the select model.
		inputs := make([]steps.ScannedRepoInput, 0, len(scanned))
		for _, r := range scanned {
			if r.Error == nil {
				inputs = append(inputs, steps.ScannedRepoInput{
					Source:    r.Source,
					Skills:    r.Skills,
					Rules:     r.Rules,
					MCPs:      r.MCPs,
					Workflows: r.Workflows,
					Descs:     r.Descs,
					MCPFiles:  r.MCPFiles,
				})
			}
		}

		if len(inputs) > 0 {
			selection, err = steps.RunSelectModel(inputs)
			if err != nil {
				return model.UserManifest{}, mapErr(err)
			}
		}
	}

	// Step 4 — summary & confirm (alt screen, step indicator inside form)
	manifest, err := steps.ConfirmSummary(agents, selection)
	if err != nil {
		return model.UserManifest{}, mapErr(err)
	}

	return manifest, nil
}

// cachePath returns the default cache directory: ~/.cache/devrune/packages/
func cachePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = home + "/.cache"
	}
	return base + "/devrune/packages"
}

// mapErr normalises huh.ErrUserAborted so callers can use errors.Is.
// Other errors are returned unchanged.
func mapErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return huh.ErrUserAborted
	}
	// Also map the select model's user aborted error to huh.ErrUserAborted
	// so that the caller can use errors.Is uniformly.
	if err != nil && err.Error() == "user aborted" {
		return huh.ErrUserAborted
	}
	return err
}
