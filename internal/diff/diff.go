package diff

import "strings"

// DiffLine is one output line from a unified diff.
type DiffLine struct {
	Kind string // "context", "added", "removed"
	Text string
}

// Diff computes a line-level diff between old and new.
// Lines unchanged appear as "context". Lines only in new as "added". Only in old as "removed".
func Diff(old, new []byte) []DiffLine {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	m, n := len(oldLines), len(newLines)

	// Build LCS table.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff lines.
	result := make([]DiffLine, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			result = append(result, DiffLine{Kind: "context", Text: oldLines[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			result = append(result, DiffLine{Kind: "added", Text: newLines[j-1]})
			j--
		default:
			result = append(result, DiffLine{Kind: "removed", Text: oldLines[i-1]})
			i--
		}
	}

	// Reverse to restore original order.
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	return result
}

// splitLines splits text into lines, stripping the trailing newline if present.
func splitLines(b []byte) []string {
	s := string(b)
	if s == "" {
		return []string{}
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}
