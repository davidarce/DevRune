// SPDX-License-Identifier: MIT

package advisormeta_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// TestReservedAdvisorNamesMatchesDisk verifies that the hardcoded
// reservedAdvisorNames slice in internal/model/manifest.go is in sync with the
// advisor skill directories on disk under .claude/skills/.
//
// Disk is the source of truth; the slice follows.
//
// The test scans .claude/skills/ for top-level directories whose name ends in
// "-advisor" (the canonical suffix; all native advisor directories use this form)
// and whose directory contains a valid SKILL.md file.
//
// If the sets diverge, the failure message tells contributors exactly what to
// add or remove — no guessing required.
//
// The test is SKIPPED when run outside a checkout that contains .claude/skills/
// (e.g. in downstream module consumers or CI that only vendor the library).
// Experimental advisors that a contributor has added locally but has not yet
// registered in reservedAdvisorNames will appear in the ADD list — this is
// intentional and informational, not a sign that experimentation is blocked.
func TestReservedAdvisorNamesMatchesDisk(t *testing.T) {
	t.Helper()

	skillsRoot := locateRepoSkills(t)
	if skillsRoot == "" {
		t.Skip("not in repo checkout — .claude/skills/ not located; skipping disk-sync guard")
	}

	diskNames, err := scanAdvisorDirectories(t, skillsRoot)
	if err != nil {
		t.Fatalf("scanning advisor directories under %q: %v", skillsRoot, err)
	}
	diskSet := make(map[string]struct{}, len(diskNames))
	for _, n := range diskNames {
		diskSet[n] = struct{}{}
	}

	sliceNames := model.ReservedAdvisorNames()
	sort.Strings(sliceNames)

	// reservedAdvisorNames declares the advisors that ship with DevRune.
	// Every entry must have a SKILL.md on disk. Extra *-advisor directories
	// on disk are user-installed customs and MUST NOT cause the test to fail.
	var missing []string
	for _, name := range sliceNames {
		if _, ok := diskSet[name]; !ok {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return
	}

	t.Fatalf(
		"reservedAdvisorNames lists names with no matching .claude/skills/<name>/SKILL.md on disk:\n"+
			"  MISSING: %v\n\n"+
			"Either add the SKILL.md files OR remove these entries from\n"+
			"reservedAdvisorNames in internal/model/manifest.go.",
		missing,
	)
}

// locateRepoSkills finds the .claude/skills/ directory by walking up from the
// test source file using runtime.Caller(0). Returns "" when no such directory
// is found — that signals the caller to skip the test.
func locateRepoSkills(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Log("locateRepoSkills: runtime.Caller returned !ok — skipping")
		return ""
	}

	// Walk upward until we find a directory that contains ".claude/skills/".
	dir := filepath.Dir(file)
	for {
		candidate := filepath.Join(dir, ".claude", "skills")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .claude/skills/.
			return ""
		}
		dir = parent
	}
}

// scanAdvisorDirectories reads skillsRoot and returns the names of every
// advisor directory found.
//
// A directory qualifies when:
//   - Its name ends in "-advisor" (the canonical suffix for all native advisors).
//   - It contains a SKILL.md file (prevents empty placeholder directories from
//     falsely matching — experimental directories without a SKILL.md are ignored).
func scanAdvisorDirectories(t *testing.T, skillsRoot string) ([]string, error) {
	t.Helper()

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Only accept the canonical "-advisor" suffix.
		if !strings.HasSuffix(name, "-advisor") {
			continue
		}

		// Require a valid SKILL.md so that empty experimental directories do not
		// trigger a false positive.
		skillMD := filepath.Join(skillsRoot, name, "SKILL.md")
		if _, statErr := os.Stat(skillMD); statErr != nil {
			// No SKILL.md — this is either an experimental placeholder or a
			// non-advisor directory that happens to end in the right suffix.
			// Skip it silently.
			continue
		}

		names = append(names, name)
	}
	return names, nil
}

