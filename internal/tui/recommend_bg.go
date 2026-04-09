// SPDX-License-Identifier: MIT

package tui

import (
	"context"

	"github.com/davidarce/devrune/internal/detect"
	"github.com/davidarce/devrune/internal/recommend"
	"github.com/davidarce/devrune/internal/tui/steps"
)

// RecommendRunner runs project detection and AI recommendation in the background
// while the user completes earlier wizard steps.
//
// Usage:
//
//	runner := NewRecommendRunner()
//	runner.Start(ctx, projectDir)
//	// ... user completes scan step ...
//	runner.SetRepos(scannedRepos)
//	result, err := runner.Wait(ctx)
//
// Synchronization invariant: result and err are written exclusively by the
// goroutine launched in Start and read exclusively by Wait. The close of done
// provides the happens-before guarantee — Wait only reads result/err after
// receiving from done, so no mutex is needed.
type RecommendRunner struct {
	result *recommend.RecommendResult
	err    error
	done   chan struct{}
	repos  chan []steps.ScannedRepoInput
	cfg    recommend.RecommendConfig
}

// NewRecommendRunner creates a new RecommendRunner with the default config.
func NewRecommendRunner() *RecommendRunner {
	return &RecommendRunner{
		done:  make(chan struct{}),
		repos: make(chan []steps.ScannedRepoInput, 1),
		cfg: recommend.RecommendConfig{
			Threshold: 0.7,
			Enabled:   true,
			Models:    recommend.DefaultAgentModels(),
		},
	}
}

// Start begins background detection and recommendation in a goroutine.
// dir is the project directory to analyze.
// The goroutine waits for SetRepos to be called before running the AI step.
// Cancelling ctx will abort the AI call.
func (r *RecommendRunner) Start(ctx context.Context, dir string) {
	go func() {
		defer close(r.done)

		// Step 1: Detect project profile (fast, filesystem only).
		profile, err := detect.Analyze(dir)
		if err != nil {
			r.err = err
			return
		}

		// Step 2: Find the AI agent binary.
		binaryPath, agentName, err := recommend.DetectAgent()
		if err != nil {
			// No agent available — store error and return.
			// The TUI caller treats this as non-fatal.
			r.err = err
			return
		}

		// Step 3: Wait for scanned repos (populated after the scan step).
		var repos []steps.ScannedRepoInput
		select {
		case <-ctx.Done():
			r.err = ctx.Err()
			return
		case repos = <-r.repos:
		}

		// Step 4: Convert scanned repos to catalog items.
		// Filter out skill headers (non-interactive labels like "Go")
		// so the AI doesn't recommend them as installable skills.
		sources := make([]recommend.ScannedSource, 0, len(repos))
		for _, repo := range repos {
			skills := repo.Skills
			if len(repo.SkillHeaders) > 0 {
				skills = make([]string, 0, len(repo.Skills))
				for _, s := range repo.Skills {
					if !repo.SkillHeaders[s] {
						skills = append(skills, s)
					}
				}
			}
			sources = append(sources, recommend.ScannedSource{
				Source:    repo.Source,
				Skills:    skills,
				Rules:     repo.Rules,
				MCPs:      repo.MCPs,
				Workflows: repo.Workflows,
				Descs:     repo.Descs,
			})
		}
		catalog := recommend.BuildCatalogItems(sources)

		// Step 5: Run the AI recommendation engine.
		engine := recommend.NewEngine(binaryPath, agentName, r.cfg)
		result, err := engine.Recommend(ctx, *profile, catalog)
		if err != nil {
			r.err = err
			return
		}
		r.result = result
	}()
}

// SetRepos feeds scanned repositories to the background runner.
// Call this after the scan step completes. May be called at most once.
func (r *RecommendRunner) SetRepos(repos []steps.ScannedRepoInput) {
	r.repos <- repos
}

// Wait blocks until the background runner completes or ctx is cancelled.
// Returns the recommendation result and any error.
func (r *RecommendRunner) Wait(ctx context.Context) (*recommend.RecommendResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return r.result, r.err
	}
}

// Ready returns true if the background runner has already completed.
// Non-blocking; useful to check before calling Wait.
func (r *RecommendRunner) Ready() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}
