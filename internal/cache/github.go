package cache

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/davidarce/devrune/internal/model"
)

// GitHubFetcher downloads package archives from the GitHub API.
// It targets the tarball endpoint: GET /repos/{owner}/{repo}/tarball/{ref}.
// An optional Bearer token is used for private repositories.
type GitHubFetcher struct {
	client *http.Client
	token  string // optional, from GITHUB_TOKEN env var
}

// NewGitHubFetcher creates a GitHubFetcher. If token is empty the GITHUB_TOKEN
// environment variable is read. Pass an explicit empty string to disable auth.
func NewGitHubFetcher(token string) *GitHubFetcher {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &GitHubFetcher{
		client: &http.Client{},
		token:  token,
	}
}

// Supports reports whether this fetcher handles the given scheme.
func (f *GitHubFetcher) Supports(scheme model.Scheme) bool {
	return scheme == model.SchemeGitHub
}

// Fetch downloads the tarball for the given SourceRef from the GitHub API.
// The SourceRef must have Scheme == SchemeGitHub.
// Returns the raw gzip-compressed tar archive bytes.
func (f *GitHubFetcher) Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error) {
	if ref.Scheme != model.SchemeGitHub {
		return nil, fmt.Errorf("github fetcher: unsupported scheme %q", ref.Scheme)
	}

	gitRef := ref.Ref
	if gitRef == "" {
		gitRef = "HEAD"
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/tarball/%s",
		ref.Owner, ref.Repo, gitRef,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github fetcher: build request for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github fetcher: fetch %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github fetcher: %s/%s@%s: server returned %d", ref.Owner, ref.Repo, gitRef, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github fetcher: read body for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	return data, nil
}
