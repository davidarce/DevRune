// SPDX-License-Identifier: MIT

package renderers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// deepMergeJSON merges src into dst recursively.
// Nested maps are merged (not overwritten). Arrays and scalars in src overwrite dst.
// The dst map is modified in-place and also returned for convenience.
func deepMergeJSON(dst, src map[string]interface{}) map[string]interface{} {
	for k, srcVal := range src {
		if dstVal, exists := dst[k]; exists {
			// Both sides have this key — check if both are maps.
			dstMap, dstIsMap := dstVal.(map[string]interface{})
			srcMap, srcIsMap := srcVal.(map[string]interface{})
			if dstIsMap && srcIsMap {
				// Recursively merge nested maps.
				dst[k] = deepMergeJSON(dstMap, srcMap)
				continue
			}
		}
		// Scalar, array, or only-one-side-is-map: src wins.
		dst[k] = srcVal
	}
	return dst
}

// ReadAndValidateHookJSON reads a JSON definition file, validates its syntax,
// and returns the parsed map. Returns an error if the file is not found or
// the JSON is invalid, so the caller can warn the user and skip.
func ReadAndValidateHookJSON(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hook JSON %q: %w", path, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON in hook file %q", path)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal hook JSON %q: %w", path, err)
	}
	return result, nil
}

// copyHookScriptAssets scans hookData recursively for string values ending with
// the given suffix (e.g. ".sh" or ".ts") and copies those files from the workflow
// cache directory to the agent workspace directory with the given file mode.
//
// agentWorkspace is the agent's workspace prefix (e.g. ".claude", ".factory").
// The JSON paths reference the destination (e.g. ".claude/hooks/script.sh") but
// the source files in the cache omit the workspace prefix (e.g. "hooks/script.sh").
func copyHookScriptAssets(hookData map[string]interface{}, cachePath, workspaceRoot, agentWorkspace, suffix string, mode os.FileMode) error {
	for _, scriptPath := range collectStringValues(hookData, suffix) {
		// scriptPath may contain shell variable prefixes like "$CLAUDE_PROJECT_DIR/...".
		// Strip any leading variable reference to get the bare relative path.
		cleanPath := stripShellVarPrefix(scriptPath)
		if cleanPath == "" {
			continue
		}

		// cleanPath is the destination-relative path (e.g. ".claude/hooks/script.sh").
		// The source in the cache omits the agent workspace prefix (e.g. "hooks/script.sh").
		srcRelative := cleanPath
		wsPrefix := agentWorkspace + "/"
		if strings.HasPrefix(srcRelative, wsPrefix) {
			srcRelative = srcRelative[len(wsPrefix):]
		}

		srcFile := filepath.Join(cachePath, srcRelative)
		// workspaceRoot already includes the agent workspace (e.g. ".claude/"),
		// so use srcRelative (without workspace prefix) for the destination too.
		dstFile := filepath.Join(workspaceRoot, srcRelative)
		if _, err := os.Stat(srcFile); err != nil {
			// Source file does not exist in the cache — skip silently.
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dstFile), 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", filepath.Dir(dstFile), err)
		}
		if err := copyFileWithMode(srcFile, dstFile, mode); err != nil {
			return fmt.Errorf("copy hook asset %q: %w", cleanPath, err)
		}
	}
	return nil
}

// collectStringValues recursively walks v (which may be a map, slice, or scalar)
// and returns all string values whose content ends with suffix.
func collectStringValues(v interface{}, suffix string) []string {
	var results []string
	switch val := v.(type) {
	case map[string]interface{}:
		for _, child := range val {
			results = append(results, collectStringValues(child, suffix)...)
		}
	case []interface{}:
		for _, item := range val {
			results = append(results, collectStringValues(item, suffix)...)
		}
	case string:
		if strings.HasSuffix(val, suffix) {
			results = append(results, val)
		}
	}
	return results
}

// stripShellVarPrefix removes a leading shell variable reference (e.g.
// `"$CLAUDE_PROJECT_DIR"/` or `"$VAR"/`) from a path string, returning
// the remainder. If no such prefix is found, the original string is returned.
// This handles the Claude security best-practice pattern of quoting the
// project-dir variable: `"$CLAUDE_PROJECT_DIR"/.claude/hooks/script.sh`.
func stripShellVarPrefix(path string) string {
	// Pattern: optional quote, $VAR_NAME, optional quote, slash.
	// Walk forward past any leading `"$VAR"/` prefix.
	s := path
	for strings.HasPrefix(s, "\"$") || strings.HasPrefix(s, "$") {
		// Find the end of the variable name / quoted segment.
		end := strings.Index(s, "/")
		if end < 0 {
			return "" // entire string is a variable, no path
		}
		s = s[end+1:]
	}
	return s
}

// copyFileWithMode copies src to dst with the specified file mode.
func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
