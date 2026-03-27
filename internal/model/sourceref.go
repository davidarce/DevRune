// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"net/url"
	"strings"
)

// Scheme represents the source scheme for a package reference.
type Scheme string

const (
	SchemeGitHub Scheme = "github"
	SchemeGitLab Scheme = "gitlab"
	SchemeLocal  Scheme = "local"
)

// SourceRef is a first-class value object representing a reference to a package source.
// It supports three schemes: github:, gitlab:, and local:.
//
// Examples:
//
//	github:owner/repo@ref//subpath
//	gitlab:owner/repo@ref//subpath?host=gitlab.example.com
//	local:./path/to/dir
type SourceRef struct {
	Scheme  Scheme
	Owner   string // github/gitlab only
	Repo    string // github/gitlab only
	Ref     string // git tag/branch/sha (github/gitlab only)
	Subpath string // optional path within repo
	Path    string // local only — resolved absolute path
	Host    string // gitlab only — custom host (default: gitlab.com)
}

// ParseSourceRef parses a raw source ref string into a SourceRef value object.
// baseDir is used to resolve relative local paths; it may be empty for absolute paths.
func ParseSourceRef(raw string, baseDir string) (SourceRef, error) {
	if raw == "" {
		return SourceRef{}, fmt.Errorf("source ref must not be empty")
	}

	colonIdx := strings.Index(raw, ":")
	if colonIdx < 0 {
		return SourceRef{}, fmt.Errorf("source ref %q: missing scheme (expected github:, gitlab:, or local:)", raw)
	}

	scheme := Scheme(raw[:colonIdx])
	rest := raw[colonIdx+1:]

	switch scheme {
	case SchemeGitHub:
		return parseGitRef(SchemeGitHub, rest, "")
	case SchemeGitLab:
		return parseGitLabRef(rest)
	case SchemeLocal:
		return parseLocalRef(rest, baseDir)
	default:
		return SourceRef{}, fmt.Errorf("source ref %q: unknown scheme %q (supported: github, gitlab, local)", raw, scheme)
	}
}

// parseGitRef parses the portion after "github:" or "gitlab:" (without query string).
// format: owner/repo[@ref][//subpath]
func parseGitRef(scheme Scheme, raw string, host string) (SourceRef, error) {
	ref := SourceRef{Scheme: scheme, Host: host}

	// Split off subpath: everything after "//"
	subpath := ""
	if idx := strings.Index(raw, "//"); idx >= 0 {
		subpath = raw[idx+2:]
		raw = raw[:idx]
	}
	ref.Subpath = subpath

	// Split ref: owner/repo@ref
	atIdx := strings.Index(raw, "@")
	ownerRepo := raw
	if atIdx >= 0 {
		ref.Ref = raw[atIdx+1:]
		ownerRepo = raw[:atIdx]
	}

	// Split owner and repo
	slashIdx := strings.Index(ownerRepo, "/")
	if slashIdx < 0 {
		return SourceRef{}, fmt.Errorf("source ref: %q scheme requires owner/repo (e.g. owner/repo@ref)", scheme)
	}
	ref.Owner = ownerRepo[:slashIdx]
	ref.Repo = ownerRepo[slashIdx+1:]

	if ref.Owner == "" {
		return SourceRef{}, fmt.Errorf("source ref: owner must not be empty")
	}
	if ref.Repo == "" {
		return SourceRef{}, fmt.Errorf("source ref: repo must not be empty")
	}

	return ref, nil
}

// parseGitLabRef parses the portion after "gitlab:", handling optional ?host= query parameter.
// format: owner/repo[@ref][//subpath][?host=gitlab.example.com]
func parseGitLabRef(raw string) (SourceRef, error) {
	host := "gitlab.com"

	// Extract query string if present
	if qIdx := strings.Index(raw, "?"); qIdx >= 0 {
		queryStr := raw[qIdx+1:]
		raw = raw[:qIdx]

		vals, err := url.ParseQuery(queryStr)
		if err != nil {
			return SourceRef{}, fmt.Errorf("source ref: malformed query in gitlab ref: %w", err)
		}
		if h := vals.Get("host"); h != "" {
			host = h
		}
		// Reject unknown query params? For now we only care about host.
	}

	ref, err := parseGitRef(SchemeGitLab, raw, host)
	if err != nil {
		return SourceRef{}, err
	}
	return ref, nil
}

// parseLocalRef parses the portion after "local:".
// format: ./path or ../path or /absolute/path
func parseLocalRef(raw string, baseDir string) (SourceRef, error) {
	if raw == "" {
		return SourceRef{}, fmt.Errorf("source ref: local path must not be empty")
	}

	ref := SourceRef{
		Scheme: SchemeLocal,
		Path:   raw,
	}
	return ref, nil
}

// String returns the canonical string representation of the SourceRef.
// ParseSourceRef(ref.String(), baseDir) should produce an equivalent SourceRef.
func (s SourceRef) String() string {
	switch s.Scheme {
	case SchemeGitHub:
		return s.gitRefString("github")
	case SchemeGitLab:
		return s.gitLabRefString()
	case SchemeLocal:
		return "local:" + s.Path
	default:
		return string(s.Scheme) + ":<unknown>"
	}
}

func (s SourceRef) gitRefString(scheme string) string {
	var sb strings.Builder
	sb.WriteString(scheme)
	sb.WriteString(":")
	sb.WriteString(s.Owner)
	sb.WriteString("/")
	sb.WriteString(s.Repo)
	if s.Ref != "" {
		sb.WriteString("@")
		sb.WriteString(s.Ref)
	}
	if s.Subpath != "" {
		sb.WriteString("//")
		sb.WriteString(s.Subpath)
	}
	return sb.String()
}

func (s SourceRef) gitLabRefString() string {
	base := s.gitRefString("gitlab")
	if s.Host != "" && s.Host != "gitlab.com" {
		base += "?host=" + s.Host
	}
	return base
}

// CacheKey returns a deterministic string suitable for use as a cache lookup key.
// It is stable across versions and includes all relevant fields.
func (s SourceRef) CacheKey() string {
	switch s.Scheme {
	case SchemeGitHub:
		key := fmt.Sprintf("github:%s/%s@%s", s.Owner, s.Repo, s.Ref)
		if s.Subpath != "" {
			key += "//" + s.Subpath
		}
		return key
	case SchemeGitLab:
		host := s.Host
		if host == "" {
			host = "gitlab.com"
		}
		key := fmt.Sprintf("gitlab:%s/%s@%s?host=%s", s.Owner, s.Repo, s.Ref, host)
		if s.Subpath != "" {
			key += "//" + s.Subpath
		}
		return key
	case SchemeLocal:
		return "local:" + s.Path
	default:
		return s.String()
	}
}

// Validate checks that the SourceRef has all required fields for its scheme.
func (s SourceRef) Validate() error {
	switch s.Scheme {
	case SchemeGitHub, SchemeGitLab:
		if s.Owner == "" {
			return fmt.Errorf("source ref: owner is required for %s scheme", s.Scheme)
		}
		if s.Repo == "" {
			return fmt.Errorf("source ref: repo is required for %s scheme", s.Scheme)
		}
	case SchemeLocal:
		if s.Path == "" {
			return fmt.Errorf("source ref: path is required for local scheme")
		}
	case "":
		return fmt.Errorf("source ref: scheme must not be empty")
	default:
		return fmt.Errorf("source ref: unknown scheme %q", s.Scheme)
	}
	return nil
}
