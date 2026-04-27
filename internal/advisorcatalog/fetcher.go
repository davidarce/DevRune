// SPDX-License-Identifier: MIT

package advisorcatalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/davidarce/devrune/internal/model"
)

// execGit is the package-level git executor. It is a var so tests can inject
// a fake implementation without spawning a real git process.
//
// The function receives the full git argument list (e.g. ["clone", "--depth=1",
// "https://...", "/tmp/cache"]) and returns the combined stdout+stderr output
// and any error.  Production code uses exec.Command("git", ...).CombinedOutput().
var execGit = func(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.CombinedOutput()
}

// LocalFetcher resolves "local:<path>" catalog sources against a workspace root.
// No cloning or caching is performed — the resolved path is returned verbatim
// after validating that it exists and is a directory.
type LocalFetcher struct {
	WorkspaceRoot string
}

// Fetch validates and returns the absolute path for a local: catalog source.
func (f LocalFetcher) Fetch(_ context.Context, src model.CatalogSource) (string, error) {
	_, body, _, err := ParseCatalogURL(src.URL)
	if err != nil {
		return "", fmt.Errorf("local fetcher: %w", err)
	}

	path := body
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.WorkspaceRoot, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("local catalog path %q does not exist: %w", path, err)
		}
		return "", fmt.Errorf("local catalog path %q: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local catalog path %q is a file, not a directory", path)
	}

	return path, nil
}

// GitFetcher handles "github:" and "gitlab:" catalog sources.
// On first use it clones --depth=1 into the advisor-catalog cache; on subsequent
// calls it fetches and checks out the target ref.
type GitFetcher struct {
	WorkspaceRoot string
}

// isSHA returns true if ref looks like a full 40-character hex SHA.
var isSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`).MatchString

// Fetch clones or fast-forwards a remote git catalog source and returns the cache path.
func (f GitFetcher) Fetch(ctx context.Context, src model.CatalogSource) (string, error) {
	scheme, body, ref, err := ParseCatalogURL(src.URL)
	if err != nil {
		return "", fmt.Errorf("git fetcher: %w", err)
	}

	var remote string
	switch scheme {
	case "github":
		remote = "https://github.com/" + body + ".git"
	case "gitlab":
		remote = "https://gitlab.com/" + body + ".git"
	default:
		return "", fmt.Errorf("git fetcher: unsupported scheme %q for %q", scheme, src.URL)
	}

	cacheRoot := filepath.Join(f.WorkspaceRoot, ".devrune", "advisor-catalogs", CacheKey(src))

	// Acquire advisory file lock to prevent concurrent fetches into the same cache dir.
	lockPath := filepath.Join(f.WorkspaceRoot, ".devrune", "advisor-catalogs", "fetch.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return "", fmt.Errorf("git fetcher: create lock dir: %w", err)
	}
	unlock, err := acquireFileLock(ctx, lockPath)
	if err != nil {
		return "", fmt.Errorf("git fetcher: acquire lock: %w", err)
	}
	defer unlock()

	dotGit := filepath.Join(cacheRoot, ".git")
	if _, statErr := os.Stat(dotGit); os.IsNotExist(statErr) {
		// First fetch: clone the repository.
		if err := gitClone(ctx, src.URL, remote, ref, cacheRoot); err != nil {
			return "", err
		}
	} else {
		// Subsequent fetch: update to the target ref.
		if err := gitUpdate(ctx, src.URL, ref, cacheRoot); err != nil {
			return "", err
		}
	}

	return cacheRoot, nil
}

// gitClone performs the initial git clone into cacheRoot.
func gitClone(ctx context.Context, srcURL, remote, ref, cacheRoot string) error {
	if isSHA(ref) {
		// SHA refs cannot be passed to --branch; clone the default branch then checkout.
		args := []string{"clone", "--depth=1", remote, cacheRoot}
		if err := runGit(ctx, srcURL, "clone", args...); err != nil {
			return err
		}
		checkoutArgs := []string{"-C", cacheRoot, "checkout", ref}
		return runGit(ctx, srcURL, "checkout", checkoutArgs...)
	}

	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, remote, cacheRoot)
	return runGit(ctx, srcURL, "clone", args...)
}

// gitUpdate fetches and checks out the target ref in an already-cloned repo.
func gitUpdate(ctx context.Context, srcURL, ref, cacheRoot string) error {
	fetchTarget := ref
	if fetchTarget == "" {
		fetchTarget = "HEAD"
	}

	fetchArgs := []string{"-C", cacheRoot, "fetch", "--depth=1", "origin", fetchTarget}
	if err := runGit(ctx, srcURL, "fetch", fetchArgs...); err != nil {
		return err
	}

	checkoutArgs := []string{"-C", cacheRoot, "checkout", "FETCH_HEAD"}
	return runGit(ctx, srcURL, "checkout", checkoutArgs...)
}

// runGit executes a git command and wraps any error with context.
// It delegates to execGit (a package-level var) so tests can inject a fake.
func runGit(_ context.Context, srcURL, cmdName string, args ...string) error {
	out, err := execGit(args...)
	if err != nil {
		return fmt.Errorf("git %s for %q: %w; output: %s", cmdName, srcURL, err, string(out))
	}
	return nil
}

// acquireFileLock obtains an advisory file lock using O_CREATE|O_EXCL, retrying
// up to 30 seconds (polling every 500 ms). Returns a release function.
func acquireFileLock(ctx context.Context, path string) (func(), error) {
	const (
		timeout  = 30 * time.Second
		interval = 500 * time.Millisecond
	)

	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			// Lock acquired.
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("open lock file: %w", err)
		}

		// Lock is held by another process; check context and deadline.
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while waiting for lock: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for advisory lock %q after %s", path, timeout)
		}

		time.Sleep(interval)
	}
}
