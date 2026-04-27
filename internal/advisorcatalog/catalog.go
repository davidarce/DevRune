// SPDX-License-Identifier: MIT

// Package advisorcatalog provides domain types, scanning logic, and fetcher ports
// for the advisor catalog subsystem introduced by the sdd-advisors command.
// It is intentionally narrow in scope — only the catalog fetch+scan concern lives
// here so it does not bleed into the main resolver.
package advisorcatalog

import (
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is used only as a short deterministic cache-key hash, not for security
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/davidarce/devrune/internal/model"
)

// CatalogEntry is one advisor discovered inside a catalog source.
// It is produced by Scanner.Scan and consumed by the TUI multi-select
// and the full-directory copy step in SyncAdvisors.
type CatalogEntry struct {
	// Name is the canonical advisor identifier (e.g. "security-advisor").
	// It MUST end in "-advisor"; entries that do not are skipped by Scanner.
	Name string

	// Description is the human-readable summary extracted from the advisor's
	// SKILL.md frontmatter. Empty when the frontmatter lacks a description field.
	Description string

	// Scope is the subset of valid scope values for this advisor (e.g. "frontend",
	// "backend", "testing"). An empty slice means the advisor applies universally
	// to every project (no scope restriction). Values are drawn from the controlled
	// vocabulary in model (AdvisorScopeFrontend, AdvisorScopeBackend, etc.).
	Scope []string

	// SKILLPath is the absolute path to the SKILL.md file inside the cached
	// (or local) catalog root, e.g. "/.../.devrune/advisor-catalogs/abc123/security-advisor/SKILL.md".
	SKILLPath string

	// DirPath is the absolute path to the advisor's root directory (the parent
	// of SKILLPath). This is what copyAdvisorDir receives as its src argument.
	DirPath string
}

// Scanner walks a catalog root directory and returns every advisor it finds.
//
// Contract: only directories matching "<root>/<name>/SKILL.md" where <name>
// ends in "-advisor" are returned. Deeper nestings (e.g. "<root>/backend/security-advisor/SKILL.md")
// are NOT scanned in v1 — that is a follow-up. Subdirectories whose names do not
// end in "-advisor" are silently skipped with a warning log line (not an error).
type Scanner interface {
	Scan(catalogRoot string) ([]CatalogEntry, error)
}

// Fetcher hydrates a model.CatalogSource into a local directory path (cache-aware).
//
// Implementations:
//   - LocalFetcher — resolves "local:<path>" against a workspace root. No clone,
//     no cache. Returns the resolved absolute path after verifying it exists.
//   - GitFetcher — handles "github:" and "gitlab:". Builds an HTTPS git URL,
//     clones --depth=1 into the advisor-catalog cache on first call, then
//     fast-forwards on subsequent calls. Updates CatalogSource.LastFetched.
type Fetcher interface {
	// Fetch returns the absolute path to the catalog root directory.
	//
	// For github:/gitlab: sources this triggers git clone --depth=1 the first
	// time, then git fetch + checkout on subsequent calls. For local: sources
	// it validates that the path exists and is a directory, then returns it
	// verbatim (no cache copy is made).
	//
	// On success for remote sources, callers MUST persist the updated
	// CatalogSource.LastFetched timestamp to devrune.yaml — Fetch does NOT
	// mutate the manifest itself.
	Fetch(ctx context.Context, src model.CatalogSource) (rootDir string, err error)
}

// ParseCatalogURL splits a scheme-prefixed catalog URL into its constituent parts.
//
// Supported formats:
//
//	local:/abs/path                — scheme="local",  body="/abs/path",         ref=""
//	local:./relative/path          — scheme="local",  body="./relative/path",   ref=""
//	local:.                        — scheme="local",  body=".",                  ref=""
//	github:owner/repo              — scheme="github", body="owner/repo",         ref=""
//	github:owner/repo@main         — scheme="github", body="owner/repo",         ref="main"
//	github:owner/repo@v1.2.3       — scheme="github", body="owner/repo",         ref="v1.2.3"
//	github:owner/repo@abc123       — scheme="github", body="owner/repo",         ref="abc123"
//	gitlab:group/repo              — scheme="gitlab", body="group/repo",          ref=""
//	gitlab:group/subgroup/repo     — scheme="gitlab", body="group/subgroup/repo", ref=""
//	gitlab:group/subgroup/repo@ref — scheme="gitlab", body="group/subgroup/repo", ref="ref"
//
// Error conditions:
//   - Empty url → error.
//   - No recognised scheme prefix (local:, github:, gitlab:) → error.
//   - github:/gitlab: with empty body → error.
//   - github:/gitlab: with no slash in body (missing repo component) → error.
//   - Trailing "@" with empty ref (e.g. "github:owner/repo@") → error.
//
// ParseCatalogURL is pure (no I/O, no side effects).
func ParseCatalogURL(url string) (scheme, body, ref string, err error) {
	if url == "" {
		return "", "", "", fmt.Errorf("catalogURL: url must not be empty")
	}

	var prefix string
	for _, p := range []string{"local:", "github:", "gitlab:"} {
		if strings.HasPrefix(url, p) {
			prefix = p
			break
		}
	}
	if prefix == "" {
		return "", "", "", fmt.Errorf("catalogURL: %q has an unrecognised scheme (must be one of local:, github:, gitlab:)", url)
	}

	scheme = strings.TrimSuffix(prefix, ":")
	rest := url[len(prefix):]

	switch scheme {
	case "local":
		// Local paths are not further validated here — the fetcher resolves them.
		// ref is always empty for local sources.
		body = rest
		ref = ""

	case "github", "gitlab":
		if rest == "" {
			return "", "", "", fmt.Errorf("catalogURL: %q: %s body must not be empty", url, scheme)
		}

		// Split off optional @ref suffix.
		if atIdx := strings.Index(rest, "@"); atIdx >= 0 {
			ref = rest[atIdx+1:]
			rest = rest[:atIdx]
			if ref == "" {
				return "", "", "", fmt.Errorf("catalogURL: %q: ref after \"@\" must not be empty", url)
			}
		}

		// body must contain at least one slash (owner/repo).
		if !strings.Contains(rest, "/") {
			return "", "", "", fmt.Errorf("catalogURL: %q: %s body must be in the form \"owner/repo\" or \"owner/repo@ref\"", url, scheme)
		}

		body = rest
	}

	return scheme, body, ref, nil
}

// CacheKey returns the deterministic cache directory name for a catalog URL.
//
// For "github:" and "gitlab:" sources it returns the first 10 hex characters of
// the SHA-1 hash of the raw URL string — long enough to be collision-free in
// practice for the number of catalogs a typical user registers.
//
// For "local:" sources it returns an empty string: local catalogs are used
// in-place and do not require a cache directory.
//
// CacheKey is pure and deterministic: the same url always produces the same key
// across processes and Go versions (sha1 of UTF-8 bytes is stable).
func CacheKey(src model.CatalogSource) string {
	scheme, _, _, err := ParseCatalogURL(src.URL)
	if err != nil {
		// Malformed URL — return empty string so callers get no cache entry.
		return ""
	}
	if scheme == "local" {
		return ""
	}

	//nolint:gosec // sha1 used as a short deterministic identifier, not for cryptographic security
	h := sha1.New()
	_, _ = h.Write([]byte(src.URL))
	return hex.EncodeToString(h.Sum(nil))[:10]
}
