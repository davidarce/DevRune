// SPDX-License-Identifier: MIT

// Package advisormeta owns the filesystem-facing concerns for advisor metadata:
// native advisor scope discovery, frontmatter list parsing, and sentinel errors
// used by callers and tests. It collapses the parallel native and custom
// ingestion pipelines into a single path.
//
// Dependency direction: advisormeta → parse + model + stdlib only.
// recommend and cli may import advisormeta; advisormeta MUST NOT import them.
// model MUST NOT import advisormeta (model stays filesystem-pure).
package advisormeta

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidarce/devrune/internal/model"
	"github.com/davidarce/devrune/internal/parse"
)

// Sentinel errors for frontmatter list parsing.
//
// These cover only hard YAML-shape errors: a scalar where a list was expected,
// a null value, or a non-string element in a list. Unknown vocabulary values are
// NOT sentinel errors — they are dropped silently by model.NormalizeAdvisorScope
// (soft fallback). ErrFrontmatterEmptyElement exists as the documented sentinel
// for callers that need strict empty-element detection; it is NOT returned by
// FrontmatterStringList itself (empty elements are dropped silently per the
// soft-fallback philosophy shared with NormalizeAdvisorScope).
var (
	// ErrFrontmatterNotList is returned when a frontmatter field exists but its
	// value is a scalar (string, int, bool, etc.) rather than a YAML list.
	ErrFrontmatterNotList = errors.New("advisormeta: frontmatter field is not a YAML list")

	// ErrFrontmatterNotString is returned when an element in a frontmatter list
	// is not a string (e.g. an integer or boolean). Positional context (e.g.
	// "scope[2]: …") is added by FrontmatterStringList via fmt.Errorf with %w.
	ErrFrontmatterNotString = errors.New("advisormeta: frontmatter list element is not a string")

	// ErrFrontmatterEmptyElement is the sentinel for "an element was empty after
	// whitespace trimming". It is NOT currently returned by FrontmatterStringList
	// (empty elements are dropped silently). It exists so that future callers
	// needing strict empty-element detection have a stable sentinel to errors.Is
	// against without adding a new public symbol.
	ErrFrontmatterEmptyElement = errors.New("advisormeta: frontmatter list element is empty after trimming")

	// ErrFrontmatterNullValue is returned when a frontmatter field is present but
	// its value is null (nil in the parsed map).
	ErrFrontmatterNullValue = errors.New("advisormeta: frontmatter field has null value")
)

// LoadNativeAdvisorScopes walks the <skillsRoot>/*-advisor/SKILL.md files,
// parses each frontmatter, and returns a map from advisor name to scope slice.
//
// The advisor name is taken from the directory name (e.g. "architect-advisor")
// — only directories whose name ends in "-advisor" are considered.
//
// Per-file errors (unreadable file, malformed frontmatter, invalid scope shape)
// are non-fatal: they are recorded and returned as a combined error, but the
// walk continues. If skillsRoot does not exist or cannot be read, a single
// error is returned immediately.
//
// The scope slice for each entry is normalised via model.NormalizeAdvisorScope:
// unknown vocabulary values are silently dropped, and an empty-after-filtering
// result maps to nil (universal). Callers MUST NOT assume a non-nil scope means
// "non-universal" without checking len > 0.
func LoadNativeAdvisorScopes(skillsRoot string) (map[string][]string, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, fmt.Errorf("advisormeta: reading skills root %q: %w", skillsRoot, err)
	}

	result := make(map[string][]string)
	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, "-advisor") {
			continue
		}

		skillMD := filepath.Join(skillsRoot, name, "SKILL.md")
		data, readErr := os.ReadFile(skillMD)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("advisormeta: %s: reading SKILL.md: %w", name, readErr))
			continue
		}

		fm, _, parseErr := parse.ParseFrontmatter(data)
		if parseErr != nil {
			errs = append(errs, fmt.Errorf("advisormeta: %s: parsing frontmatter: %w", name, parseErr))
			continue
		}

		rawScope, fmErr := FrontmatterStringList(fm, "scope")
		if fmErr != nil {
			errs = append(errs, fmt.Errorf("advisormeta: %s: %w", name, fmErr))
			continue
		}

		result[name] = model.NormalizeAdvisorScope(rawScope)
	}

	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}

// FrontmatterStringList extracts a list-of-strings field from a parsed
// frontmatter map.
//
// Behaviour by case:
//   - Missing key: returns (nil, nil) — key not present is not an error.
//   - Null value (YAML null / Go nil): returns (nil, fmt.Errorf("%s: %w", key, ErrFrontmatterNullValue)).
//   - Scalar value (not a list): returns (nil, fmt.Errorf("%s: %w", key, ErrFrontmatterNotList)).
//   - Non-string element in list: returns (nil, fmt.Errorf("%s[%d]: %w", key, i, ErrFrontmatterNotString)).
//   - Valid list: trims whitespace from each element; drops empty (post-trim) elements
//     silently (soft drop — matches model.NormalizeAdvisorScope philosophy).
//     Returns the remaining elements and nil error.
//
// All error sentinels are wrapped with %w so callers can use errors.Is without
// coupling to message text.
//
// Note: ErrFrontmatterEmptyElement is NOT returned by this function. It exists
// in the sentinel set for future callers that need strict empty-element detection.
func FrontmatterStringList(fm map[string]any, key string) ([]string, error) {
	raw, ok := fm[key]
	if !ok {
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("%s: %w", key, ErrFrontmatterNullValue)
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: %w", key, ErrFrontmatterNotList)
	}
	out := make([]string, 0, len(list))
	for i, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, ErrFrontmatterNotString)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue // soft drop — see Godoc above
		}
		out = append(out, s)
	}
	return out, nil
}
