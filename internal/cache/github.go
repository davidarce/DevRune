// SPDX-License-Identifier: MIT

package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/davidarce/devrune/internal/model"
)

// GitHubFetcher downloads package archives from the GitHub API.
// It targets the tarball endpoint: GET /repos/{owner}/{repo}/tarball/{ref}.
// An optional Bearer token is used for private repositories.
type GitHubFetcher struct {
	client *http.Client
	token  string // optional, from GITHUB_TOKEN env var
}

// NewGitHubFetcher creates a GitHubFetcher. Token resolution follows a three-tier
// strategy: explicit token → GITHUB_TOKEN env var → gh auth token CLI fallback.
// This allows seamless use with private repos when the gh CLI is authenticated.
func NewGitHubFetcher(token string) *GitHubFetcher {
	return &GitHubFetcher{
		client: &http.Client{},
		token:  resolveToken(token, "GITHUB_TOKEN"),
	}
}

// Supports reports whether this fetcher handles the given scheme.
func (f *GitHubFetcher) Supports(scheme model.Scheme) bool {
	return scheme == model.SchemeGitHub
}

// ResolveRevision returns the commit SHA the given ref currently points at.
// It issues a lightweight `GET /repos/{owner}/{repo}/commits/{ref}` request
// with the `application/vnd.github.sha` media type — the server responds
// with the 40-hex-char SHA as plain text, so the response payload is tiny
// (~40 bytes) and the whole round-trip is typically sub-100ms. Used by the
// resolver to re-validate cached tarballs for mutable refs (HEAD, branches)
// without re-downloading the full archive.
func (f *GitHubFetcher) ResolveRevision(ctx context.Context, ref model.SourceRef) (string, error) {
	if ref.Scheme != model.SchemeGitHub {
		return "", fmt.Errorf("github fetcher: unsupported scheme %q", ref.Scheme)
	}

	gitRef := ref.Ref
	if gitRef == "" {
		gitRef = "HEAD"
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/commits/%s",
		ref.Owner, ref.Repo, gitRef,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("github fetcher: build revision request for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	req.Header.Set("Accept", "application/vnd.github.sha")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github fetcher: resolve revision for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github fetcher: resolve revision for %s/%s@%s: server returned %d", ref.Owner, ref.Repo, gitRef, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("github fetcher: read revision body for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	sha := string(bytes.TrimSpace(body))
	if sha == "" {
		return "", fmt.Errorf("github fetcher: empty revision for %s/%s@%s", ref.Owner, ref.Repo, gitRef)
	}
	return sha, nil
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github fetcher: %s/%s@%s: server returned %d", ref.Owner, ref.Repo, gitRef, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github fetcher: read body for %s/%s@%s: %w", ref.Owner, ref.Repo, gitRef, err)
	}

	return data, nil
}
