package cache

import (
	"os"
	"os/exec"
	"strings"
)

// resolveToken returns an auth token using a three-tier strategy:
//  1. If explicit is non-empty, use it directly.
//  2. Read from the environment variable named by envVar (e.g. "GITHUB_TOKEN").
//  3. Fall back to `gh auth token` if the gh CLI is installed and authenticated.
//
// Returns "" if no token is available (requests will be unauthenticated,
// which works for public repos).
func resolveToken(explicit, envVar string) string {
	// 1. Explicit token passed by caller.
	if explicit != "" {
		return explicit
	}

	// 2. Environment variable.
	if tok := os.Getenv(envVar); tok != "" {
		return tok
	}

	// 3. gh CLI fallback (works for both github.com and GitLab via gh auth).
	if tok := ghAuthToken(); tok != "" {
		return tok
	}

	return ""
}

// ghAuthToken attempts to read the current token from the gh CLI.
// Returns "" if gh is not installed or not authenticated.
func ghAuthToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
