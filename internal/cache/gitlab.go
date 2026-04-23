// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/davidarce/devrune/internal/model"
)

// GitLabFetcher downloads package archives from the GitLab API.
// It targets the repository archive endpoint:
//
//	GET https://{host}/api/v4/projects/{owner}%2F{repo}/repository/archive.tar.gz?sha={ref}
//
// Authentication uses the PRIVATE-TOKEN header (not Bearer).
// Supports self-managed GitLab instances via SourceRef.Host.
type GitLabFetcher struct {
	client *http.Client
	token  string // optional, from GITLAB_TOKEN env var
}

// NewGitLabFetcher creates a GitLabFetcher. Token resolution follows a three-tier
// strategy: explicit token → GITLAB_TOKEN env var → gh auth token CLI fallback.
// This allows seamless use with private repos when the gh CLI is authenticated.
func NewGitLabFetcher(token string) *GitLabFetcher {
	return &GitLabFetcher{
		client: &http.Client{},
		token:  resolveToken(token, "GITLAB_TOKEN"),
	}
}

// Supports reports whether this fetcher handles the given scheme.
func (f *GitLabFetcher) Supports(scheme model.Scheme) bool {
	return scheme == model.SchemeGitLab
}

// ResolveRevision returns the commit SHA the given ref currently points at.
// Issues `GET /api/v4/projects/{id}/repository/commits/{ref}` and parses the
// `id` field from the JSON response. See GitHubFetcher.ResolveRevision for
// the motivation.
func (f *GitLabFetcher) ResolveRevision(ctx context.Context, ref model.SourceRef) (string, error) {
	if ref.Scheme != model.SchemeGitLab {
		return "", fmt.Errorf("gitlab fetcher: unsupported scheme %q", ref.Scheme)
	}

	host := ref.Host
	if host == "" {
		host = "gitlab.com"
	}

	gitRef := ref.Ref
	if gitRef == "" {
		gitRef = "HEAD"
	}

	projectPath := url.PathEscape(ref.Owner + "/" + ref.Repo)
	rawURL := fmt.Sprintf(
		"https://%s/api/v4/projects/%s/repository/commits/%s",
		host, projectPath, url.PathEscape(gitRef),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("gitlab fetcher: build revision request for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	if f.token != "" {
		req.Header.Set("PRIVATE-TOKEN", f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab fetcher: resolve revision for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gitlab fetcher: resolve revision for %s/%s@%s: server returned %d", ref.Owner, ref.Repo, gitRef, resp.StatusCode)
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("gitlab fetcher: decode revision response for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	if payload.ID == "" {
		return "", fmt.Errorf("gitlab fetcher: empty revision for %s/%s@%s", ref.Owner, ref.Repo, gitRef)
	}
	return payload.ID, nil
}

// Fetch downloads the tar.gz archive for the given SourceRef from the GitLab API.
// The SourceRef must have Scheme == SchemeGitLab.
// Returns the raw gzip-compressed tar archive bytes.
func (f *GitLabFetcher) Fetch(ctx context.Context, ref model.SourceRef) ([]byte, error) {
	if ref.Scheme != model.SchemeGitLab {
		return nil, fmt.Errorf("gitlab fetcher: unsupported scheme %q", ref.Scheme)
	}

	host := ref.Host
	if host == "" {
		host = "gitlab.com"
	}

	gitRef := ref.Ref
	if gitRef == "" {
		gitRef = "HEAD"
	}

	// GitLab requires the project path URL-encoded: owner%2Frepo
	projectPath := url.PathEscape(ref.Owner + "/" + ref.Repo)

	rawURL := fmt.Sprintf(
		"https://%s/api/v4/projects/%s/repository/archive.tar.gz?sha=%s",
		host, projectPath, url.QueryEscape(gitRef),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab fetcher: build request for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	if f.token != "" {
		req.Header.Set("PRIVATE-TOKEN", f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab fetcher: fetch %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab fetcher: %s/%s@%s: server returned %d", ref.Owner, ref.Repo, gitRef, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gitlab fetcher: read body for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	return data, nil
}
