// SPDX-License-Identifier: MIT

package parse

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelimiter = "---"

// ParseFrontmatter extracts and parses YAML frontmatter from the raw bytes of a
// markdown file (e.g. SKILL.md). Frontmatter is delimited by "---" on its own line.
//
// The function returns:
//   - frontmatter: the parsed YAML as a map[string]interface{}, or an empty map if
//     no frontmatter block is present.
//   - body: the remaining markdown content after the closing "---", with any leading
//     newline trimmed.
//   - err: a non-nil error if the frontmatter YAML is malformed.
//
// Examples of valid input:
//
//	---
//	name: git:commit
//	description: "Automate git commits"
//	allowed-tools:
//	  - Bash
//	  - Read
//	---
//	# Body content here
//
// A file with no leading "---" is treated as having no frontmatter — an empty map is
// returned and the entire content is the body.
func ParseFrontmatter(data []byte) (frontmatter map[string]interface{}, body string, err error) {
	content := string(data)

	// Check for opening delimiter at the very start of the file (allowing optional
	// leading newline for robustness, but canonical form starts with "---\n").
	trimmed := strings.TrimLeft(content, "\n")
	if !strings.HasPrefix(trimmed, frontmatterDelimiter) {
		// No frontmatter block: return empty map and full content as body.
		return make(map[string]interface{}), content, nil
	}

	// Strip optional leading newlines to find the first "---".
	leadingNewlines := len(content) - len(trimmed)
	rest := trimmed[len(frontmatterDelimiter):]

	// The opening delimiter must be followed by a newline (or end-of-file).
	if len(rest) == 0 {
		// "---" at EOF with no body — treat as empty frontmatter, empty body.
		return make(map[string]interface{}), "", nil
	}
	if rest[0] != '\n' && rest[0] != '\r' {
		// "---" followed by non-whitespace means this is not a frontmatter block.
		return make(map[string]interface{}), content, nil
	}
	// Consume the newline after the opening delimiter.
	if rest[0] == '\r' && len(rest) > 1 && rest[1] == '\n' {
		rest = rest[2:] // CRLF
	} else {
		rest = rest[1:] // LF
	}

	// Find the closing "---" delimiter on its own line.
	closingIdx := findClosingDelimiter(rest)
	if closingIdx < 0 {
		// No closing delimiter: no frontmatter; treat entire content as body.
		_ = leadingNewlines
		return make(map[string]interface{}), content, nil
	}

	yamlContent := rest[:closingIdx]
	afterClose := rest[closingIdx+len(frontmatterDelimiter):]

	// Parse the YAML frontmatter.
	fm := make(map[string]interface{})
	if strings.TrimSpace(yamlContent) != "" {
		if err = yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
			return nil, "", fmt.Errorf("frontmatter: invalid YAML: %w", err)
		}
	}

	// Body is everything after the closing delimiter, with a single leading newline trimmed.
	bodyStr := afterClose
	if strings.HasPrefix(bodyStr, "\r\n") {
		bodyStr = bodyStr[2:]
	} else if strings.HasPrefix(bodyStr, "\n") {
		bodyStr = bodyStr[1:]
	}

	return fm, bodyStr, nil
}

// findClosingDelimiter locates the "---" closing delimiter within the frontmatter
// content string. It must appear at the start of a line. Returns the byte index of
// the "---" or -1 if not found.
func findClosingDelimiter(s string) int {
	offset := 0
	for offset < len(s) {
		idx := strings.Index(s[offset:], frontmatterDelimiter)
		if idx < 0 {
			return -1
		}
		abs := offset + idx
		// "---" must be at the start of the string or preceded by a newline.
		atLineStart := abs == 0 || s[abs-1] == '\n'
		// "---" must be followed by newline, carriage return, or end-of-string.
		afterDelim := abs + len(frontmatterDelimiter)
		atLineEnd := afterDelim >= len(s) ||
			s[afterDelim] == '\n' || s[afterDelim] == '\r'
		if atLineStart && atLineEnd {
			return abs
		}
		offset = abs + 1
	}
	return -1
}

// SerializeFrontmatter writes a frontmatter map and body back to SKILL.md format.
// The output is:
//
//	---
//	<YAML fields>
//	---
//	<body>
//
// If frontmatter is nil or empty, only the body is returned (no delimiter block).
func SerializeFrontmatter(frontmatter map[string]interface{}, body string) ([]byte, error) {
	if len(frontmatter) == 0 {
		return []byte(body), nil
	}

	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: failed to serialize YAML: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(frontmatterDelimiter)
	buf.WriteByte('\n')
	buf.Write(yamlBytes)
	buf.WriteString(frontmatterDelimiter)
	buf.WriteByte('\n')
	if body != "" {
		buf.WriteString(body)
	}

	return buf.Bytes(), nil
}
