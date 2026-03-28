// SPDX-License-Identifier: MIT

package renderers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/davidarce/devrune/internal/materialize/matypes"
	"github.com/davidarce/devrune/internal/model"
)

// WriteManagedBlock writes content between beginMarker and endMarker in filePath.
// If the markers already exist in the file, the block between them (inclusive) is replaced.
// If the markers are absent, the managed block is appended to the file.
// If the file does not exist, it is created with only the managed block.
// The file is written with 0o644 permissions.
func WriteManagedBlock(filePath, beginMarker, endMarker, content string) error {
	existing := ""
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("WriteManagedBlock: read %q: %w", filePath, err)
	}
	if err == nil {
		existing = string(data)
	}

	block := beginMarker + "\n" + content
	if !strings.HasSuffix(block, "\n") {
		block += "\n"
	}
	block += endMarker + "\n"

	var result string
	beginIdx := strings.Index(existing, beginMarker)
	endIdx := strings.Index(existing, endMarker)
	if beginIdx >= 0 && endIdx > beginIdx {
		// Replace existing managed block (inclusive of markers and trailing newline).
		after := existing[endIdx+len(endMarker):]
		after = strings.TrimPrefix(after, "\n")
		result = existing[:beginIdx] + block + after
	} else {
		// Append managed block.
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		result = existing + "\n" + block
		if existing == "" {
			result = block
		}
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("WriteManagedBlock: mkdir %q: %w", filepath.Dir(filePath), err)
	}
	return os.WriteFile(filePath, []byte(result), 0o644)
}

// RemoveManagedBlock removes the content between beginMarker and endMarker (inclusive)
// from filePath. If the file does not exist or the markers are not found, returns nil.
// If the remaining content is only whitespace after removal, the file is deleted entirely.
func RemoveManagedBlock(filePath, beginMarker, endMarker string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("RemoveManagedBlock: read %q: %w", filePath, err)
	}
	content := string(data)

	beginIdx := strings.Index(content, beginMarker)
	endIdx := strings.Index(content, endMarker)
	if beginIdx < 0 || endIdx <= beginIdx {
		return nil // markers not found
	}

	// Remove from beginMarker to endMarker (inclusive) plus trailing newline.
	end := endIdx + len(endMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	result := content[:beginIdx] + content[end:]

	if strings.TrimSpace(result) == "" {
		return os.Remove(filePath)
	}
	return os.WriteFile(filePath, []byte(result), 0o644)
}

// CreateSymlinkOrCopy creates a symlink at linkPath pointing to target on Unix,
// or copies the file content from target to linkPath on Windows.
// If linkPath already exists as a symlink pointing to the correct target, it is a no-op.
// If linkPath exists as a regular file (not a symlink), an error is returned to avoid
// clobbering user-owned files.
// The symlink target uses just the base filename since both files are in the same directory.
func CreateSymlinkOrCopy(target, linkPath string) error {
	// Check if linkPath already exists.
	linfo, err := os.Lstat(linkPath)
	if err == nil {
		// linkPath exists — inspect it.
		if linfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink — check where it points.
			dest, err := os.Readlink(linkPath)
			if err != nil {
				return fmt.Errorf("CreateSymlinkOrCopy: readlink %q: %w", linkPath, err)
			}
			// Accept both relative (base filename) and absolute targets.
			if dest == filepath.Base(target) || dest == target {
				return nil // already correct — no-op
			}
			// Points elsewhere — remove and recreate.
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("CreateSymlinkOrCopy: remove stale symlink %q: %w", linkPath, err)
			}
		} else {
			// Regular file — refuse to clobber.
			return fmt.Errorf("CreateSymlinkOrCopy: %q already exists as a regular file; rename or remove it first", linkPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("CreateSymlinkOrCopy: lstat %q: %w", linkPath, err)
	}

	if runtime.GOOS == "windows" {
		// On Windows, copy file content instead of creating a symlink.
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("CreateSymlinkOrCopy: stat target %q: %w", target, err)
		}
		return copySingleFile(target, linkPath, info.Mode())
	}
	// Unix: use relative target (just the filename) since both are in the same dir.
	return os.Symlink(filepath.Base(target), linkPath)
}

// RemoveSymlinkOrCopy removes linkPath if it is a symlink pointing to target (Unix),
// or if it is a regular file with identical content to target (Windows).
// If linkPath does not exist, returns nil.
// If linkPath is a user-owned file (different content or symlink to different target),
// it is left untouched.
func RemoveSymlinkOrCopy(target, linkPath string) error {
	linfo, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("RemoveSymlinkOrCopy: lstat %q: %w", linkPath, err)
	}

	if linfo.Mode()&os.ModeSymlink != 0 {
		// It's a symlink — remove only if it points to the expected target.
		dest, err := os.Readlink(linkPath)
		if err != nil {
			return fmt.Errorf("RemoveSymlinkOrCopy: readlink %q: %w", linkPath, err)
		}
		if dest == filepath.Base(target) || dest == target {
			return os.Remove(linkPath)
		}
		// Points elsewhere — leave it alone.
		return nil
	}

	// Regular file (Windows copy case): compare content.
	linkData, err := os.ReadFile(linkPath)
	if err != nil {
		return fmt.Errorf("RemoveSymlinkOrCopy: read %q: %w", linkPath, err)
	}
	targetData, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("RemoveSymlinkOrCopy: read target %q: %w", target, err)
	}
	if string(linkData) == string(targetData) {
		return os.Remove(linkPath)
	}
	// Content differs — user-owned file, leave it alone.
	return nil
}

// parseYAML decodes YAML data into the target value.
func parseYAML(data []byte, target interface{}) error {
	return yaml.Unmarshal(data, target)
}

// copyDirRecursive copies a directory tree from src to dst.
func copyDirRecursive(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy dir: stat %q: %w", src, err)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("copy dir: mkdir %q: %w", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("copy dir: read %q: %w", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copySingleFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// copySingleFile copies a single file from src to dst with the given mode.
func copySingleFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("copy file: parent dir %q: %w", dst, err)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy file: open %q: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("copy file: create %q: %w", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// modelShortToFull maps short model names to their fully qualified IDs.
// This table is shared across renderers that need model name resolution.
var modelShortToFull = map[string]string{
	"sonnet": "anthropic/claude-sonnet-4-20250514",
	"opus":   "anthropic/claude-opus-4-20250514",
	"haiku":  "anthropic/claude-haiku-4-5-20250929",
}

// resolveModel maps a short model name to its fully qualified ID.
// If the name is not in the lookup table, it is returned unchanged.
func resolveModel(model string) string {
	if full, ok := modelShortToFull[model]; ok {
		return full
	}
	return model
}

// modelShortToOpenCode maps short model names to their OpenCode model IDs.
// OpenCode expects the format: github-copilot/{bareModelID}.
var modelShortToOpenCode = map[string]string{
	"sonnet": "github-copilot/claude-sonnet-4.6",
	"opus":   "github-copilot/claude-opus-4.6",
	"haiku":  "github-copilot/claude-haiku-4.5",
}

// resolveOpenCodeModel maps a short model name to its OpenCode model ID.
// If the name is in the lookup table, the github-copilot/{bareModelID} form is returned.
// If the name already contains a provider prefix ("/"), it is returned unchanged.
// Otherwise, the github-copilot/ prefix is prepended.
func resolveOpenCodeModel(m string) string {
	if full, ok := modelShortToOpenCode[m]; ok {
		return full
	}
	if strings.Contains(m, "/") {
		return m
	}
	return "github-copilot/" + m
}

// colonToHyphen replaces all colons in a name with hyphens.
// Example: "git:commit" → "git-commit"
func colonToHyphen(name string) string {
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			result[i] = '-'
		} else {
			result[i] = name[i]
		}
	}
	return string(result)
}

// capitalizeFirst returns the string with its first letter uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// getStringField returns the string value of a frontmatter field, or "" if absent/wrong type.
func getStringField(fm map[string]interface{}, key string) string {
	v, ok := fm[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// captureRegistryContent reads the registry file from a workflow cache directory
// and applies placeholder replacements to the content. It returns the processed
// string ready for injection into a catalog file. If the registry file does not
// exist or cannot be read, it returns ("", nil) so callers can skip injection
// gracefully.
//
// replacements maps placeholder strings to their resolved values, e.g.:
//
//	{"{SKILLS_PATH}": ".opencode/skills/sdd/"}
//
// This is the shared alternative to each renderer reading and transforming
// REGISTRY.md individually; all renderers should use this helper instead of
// copying the file loose into the workspace.
func captureRegistryContent(cachePath, registryFile string, replacements map[string]string) (string, error) {
	if registryFile == "" {
		return "", nil
	}
	registryPath := filepath.Join(cachePath, registryFile)
	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("captureRegistryContent: read %q: %w", registryPath, err)
	}
	content := string(data)
	for placeholder, value := range replacements {
		content = strings.ReplaceAll(content, placeholder, value)
	}
	return content, nil
}

// resolvePlaceholders walks all .md files under rootDir and replaces occurrences
// of each placeholder key with the corresponding value from the replacements map.
// Only files that contain at least one placeholder are rewritten.
//
// This is the shared generalisation of Claude's postProcessWorkflow {SKILLS_PATH}
// replacement logic so that Factory, OpenCode, and Copilot can use it too.
// Each renderer passes its own replacements map, e.g.:
//
//	{"{SKILLS_PATH}": ".opencode/skills/sdd/"}
//
// Claude-specific replacements (e.g. <!-- ADVISER_TABLE_PLACEHOLDER -->) remain
// in the Claude renderer and are NOT handled here.
func resolvePlaceholders(rootDir string, replacements map[string]string) error {
	if len(replacements) == 0 {
		return nil
	}
	return filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("resolvePlaceholders: read %q: %w", path, err)
		}

		content := string(data)
		modified := false
		for placeholder, value := range replacements {
			if strings.Contains(content, placeholder) {
				content = strings.ReplaceAll(content, placeholder, value)
				modified = true
			}
		}

		if modified {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("resolvePlaceholders: write %q: %w", path, err)
			}
		}
		return nil
	})
}

// findWorkflowRoleByKind returns the first WorkflowRole in roles whose Kind equals kind,
// or nil if no matching role is found. This is the generalised alternative to
// findWorkflowRole — renderers should prefer this when looking up roles by their
// functional kind (e.g. "orchestrator") rather than by a hardcoded name, so the
// lookup works for any workflow, not just SDD.
func findWorkflowRoleByKind(roles []model.WorkflowRole, kind string) *model.WorkflowRole {
	for i := range roles {
		if roles[i].Kind == kind {
			return &roles[i]
		}
	}
	return nil
}

// normalizedMCP holds the separated concerns of an MCP definition:
// the runtime server config (safe to serialize into MCP config files) and
// the catalog-only agent instructions (never written into JSON config).
type normalizedMCP struct {
	// Name is the logical name used as the MCP server key.
	Name string
	// ServerConfig contains only transport/runtime fields safe for MCP config files.
	// Allowed keys: command, args, env, environment, type, url, headers.
	ServerConfig map[string]interface{}
	// AgentInstructions is the extracted agentInstructions value from the MCP definition.
	// It is catalog-only and must never be written into MCP server config JSON.
	AgentInstructions string
}

// mcpAllowedKeys is the set of runtime/transport keys that may appear in a
// serialized MCP server config. All other keys (e.g. agentInstructions, name)
// are renderer-external metadata and must be stripped.
var mcpAllowedKeys = map[string]bool{
	"command":     true,
	"args":        true,
	"env":         true,
	"environment": true,
	"type":        true,
	"url":         true,
	"headers":     true,
}

// sanitizeMCPDefinition strips renderer-external metadata fields from an MCP
// definition map and returns the clean runtime config alongside the extracted
// agentInstructions string.
//
// Fields removed:
//   - agentInstructions (returned separately as the second return value)
//   - name (MCP logical name is always stored in the lockfile, not the config)
//   - any other key not in mcpAllowedKeys
//
// The original map is NOT modified; a new map is returned.
func sanitizeMCPDefinition(def map[string]interface{}) (serverConfig map[string]interface{}, agentInstructions string) {
	if instructions, ok := def["agentInstructions"]; ok {
		if s, ok := instructions.(string); ok {
			agentInstructions = s
		}
	}

	serverConfig = make(map[string]interface{}, len(def))
	for k, v := range def {
		if mcpAllowedKeys[k] {
			serverConfig[k] = v
		}
	}
	return serverConfig, agentInstructions
}

// normalizeMCPDefinitions reads each locked MCP from cacheStore, strips metadata
// fields via sanitizeMCPDefinition, and returns the normalized list ready for
// renderer use. Returns an error if any MCP is not in the cache or cannot be read.
func normalizeMCPDefinitions(mcps []model.LockedMCP, cacheStore matypes.CacheStore) ([]normalizedMCP, error) {
	result := make([]normalizedMCP, 0, len(mcps))
	for _, mcp := range mcps {
		if !cacheStore.Has(mcp.Hash) {
			return nil, fmt.Errorf("normalizeMCPDefinitions: MCP %q not in cache", mcp.Name)
		}
		cacheDir, ok := cacheStore.Get(mcp.Hash)
		if !ok {
			return nil, fmt.Errorf("normalizeMCPDefinitions: get MCP %q: not in cache", mcp.Name)
		}

		// Resolve the effective directory containing the MCP definition file.
		// mcp.Dir holds the Subpath from the source ref (e.g. "mcps/atlassian").
		// The definition may live as a YAML file alongside the subpath basename
		// (e.g. cacheDir/mcps/atlassian.yaml) or inside a subdirectory
		// (e.g. cacheDir/mcps/atlassian/mcp.yaml).
		mcpDefDir := resolveMCPDefDir(cacheDir, mcp.Dir)

		rawDef, err := readMCPDefinition(mcpDefDir)
		if err != nil {
			return nil, fmt.Errorf("normalizeMCPDefinitions: read MCP definition %q: %w", mcp.Name, err)
		}

		serverConfig, agentInstructions := sanitizeMCPDefinition(rawDef)
		result = append(result, normalizedMCP{
			Name:              mcp.Name,
			ServerConfig:      serverConfig,
			AgentInstructions: agentInstructions,
		})
	}
	return result, nil
}

// ResolveMCPDefDir returns the directory path to use when reading an MCP definition
// from a cached archive. dir is the Subpath from the LockedMCP (e.g. "mcps/atlassian").
//
//   - If dir is empty, cacheRoot is returned as-is (standalone MCP repo).
//   - If cacheRoot/dir is an existing directory, it is returned directly.
//   - Otherwise, cacheRoot/dir is returned as a path stem: readMCPDefinition will
//     probe <stem>.yaml and <stem>.yml for single-file catalog MCPs.
//
// This is the exported equivalent of resolveMCPDefDir. External callers (e.g.
// materializer.ensureRootMCPJSON) should use this function.
func ResolveMCPDefDir(cacheRoot, dir string) string {
	return resolveMCPDefDir(cacheRoot, dir)
}

// resolveMCPDefDir is the internal implementation of ResolveMCPDefDir.
func resolveMCPDefDir(cacheRoot, dir string) string {
	if dir == "" {
		return cacheRoot
	}
	candidate := filepath.Join(cacheRoot, dir)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		// dir is a real subdirectory — use it directly.
		return candidate
	}
	// dir is not a directory — the definition is a single YAML file.
	// Return the path stem (without extension); readMCPDefinition will probe
	// <stem>.yaml and <stem>.yml before falling back to a directory scan.
	return candidate
}

// buildWorkflowPlaceholderReplacements constructs the shared placeholder replacement
// map for a workflow. All renderers should use this helper to avoid double-slash
// bugs and unresolved model placeholder markers.
//
// Produced replacements:
//   - {SKILLS_PATH} → "<workspaceDir>/<skillDir>" (no trailing slash)
//   - {WORKFLOW_MODEL_<KEY>} → resolved model value for each subagent role
//   - {SDD_MODEL_*} legacy aliases when wf.Metadata.Name == "sdd"
//
// The KEY for each role is derived via model.PlaceholderKeyFromRole:
//   - Auto-derived: strip "<workflowName>-" prefix, uppercase, hyphens → underscores
//   - Explicit: role.Placeholder overrides auto-derivation
//
// modelResolver is an optional function that maps a raw model value to the
// fully qualified ID expected by the target platform. When nil, the raw
// role.Model value is used as-is — needed for Claude Code where the Agent
// tool expects short names ("sonnet", "opus", "haiku"). Typical values:
//   - nil              → raw short names (Claude Code)
//   - resolveModel     → anthropic/... format (Factory, Copilot)
//   - resolveOpenCodeModel → github-copilot/... format (OpenCode)
//
// modelOverrides is an optional map of role-name → model-value. When non-nil,
// an override for a role takes precedence over the role's own Model field from
// workflow.yaml. The ModelInheritOption sentinel is treated as "no override"
// (falls back to role.Model). Passing nil is fully backward-compatible.
//
// Model placeholders are only added when the corresponding role has a non-empty Model field.
// workspaceDir and skillDir must not have trailing slashes; the helper joins them cleanly.
func buildWorkflowPlaceholderReplacements(
	wf model.WorkflowManifest,
	workspaceDir string,
	skillDir string,
	modelResolver func(string) string,
	modelOverrides map[string]string,
) map[string]string {
	// Normalise: strip trailing slashes so joins are always clean.
	workspaceDir = strings.TrimRight(workspaceDir, "/")
	skillDir = strings.TrimRight(skillDir, "/")

	skillsPath := workspaceDir + "/" + skillDir
	if skillDir == "" {
		skillsPath = workspaceDir
	}

	replacements := map[string]string{
		"{SKILLS_PATH}": skillsPath,
	}

	wfName := wf.Metadata.Name

	// Legacy SDD placeholder-to-role mapping for backward compatibility.
	// When wfName == "sdd", we also emit these aliases alongside the new
	// {WORKFLOW_MODEL_*} placeholders.
	sddLegacyMap := map[string]string{
		"EXPLORER":    "{SDD_MODEL_EXPLORE}",
		"PLANNER":     "{SDD_MODEL_PLAN}",
		"IMPLEMENTER": "{SDD_MODEL_IMPLEMENT}",
		"REVIEWER":    "{SDD_MODEL_REVIEW}",
	}

	for _, role := range wf.Components.Roles {
		if role.Kind != "subagent" {
			continue
		}

		// Check TUI-selected override first, then fall back to role.Model from workflow.yaml.
		modelValue := ""
		if modelOverrides != nil {
			if v, ok := modelOverrides[role.Name]; ok && v != "" && v != model.ModelInheritOption {
				modelValue = v
			}
		}
		if modelValue == "" && role.Model != "" {
			modelValue = role.Model
		}
		if modelValue == "" {
			continue
		}

		resolved := modelValue
		if modelResolver != nil {
			resolved = modelResolver(modelValue)
		}

		key := model.PlaceholderKeyFromRole(wfName, role.Name, role.Placeholder)
		replacements["{WORKFLOW_MODEL_"+key+"}"] = resolved

		// Emit legacy SDD aliases for backward compatibility.
		if wfName == "sdd" {
			if legacyPlaceholder, ok := sddLegacyMap[key]; ok {
				replacements[legacyPlaceholder] = resolved
			}
		}
	}

	return replacements
}

// buildWorkflowPathReplacements constructs a minimal replacement map containing
// only the {SKILLS_PATH} placeholder. It is used by renderers that do not support
// model routing (Factory, Copilot) so that model placeholders are never resolved
// from workflow role metadata. The caller is responsible for removing unresolved
// model placeholder lines from installed files via removeModelPlaceholderLines
// after calling resolvePlaceholders.
//
// workspaceDir and skillDir must not have trailing slashes.
func buildWorkflowPathReplacements(workspaceDir, skillDir string) map[string]string {
	workspaceDir = strings.TrimRight(workspaceDir, "/")
	skillDir = strings.TrimRight(skillDir, "/")
	skillsPath := workspaceDir + "/" + skillDir
	if skillDir == "" {
		skillsPath = workspaceDir
	}
	return map[string]string{
		"{SKILLS_PATH}": skillsPath,
	}
}

// removeModelPlaceholderLines walks all .md files under rootDir and removes any
// line that contains an unresolved model placeholder. This is used by renderers
// that do not support model routing (Factory, Copilot): their ORCHESTRATOR.md
// templates contain model placeholder lines that must be stripped entirely so
// sub-agents inherit the session's active model instead of receiving an invalid
// model value.
//
// A line is removed if it contains the literal substring "{SDD_MODEL_" or
// "{WORKFLOW_MODEL_". The removal is line-granular — the surrounding content
// is left intact. Only files that contain at least one such line are rewritten.
func removeModelPlaceholderLines(rootDir string) error {
	return filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("removeModelPlaceholderLines: read %q: %w", path, err)
		}

		content := string(data)
		if !strings.Contains(content, "{SDD_MODEL_") && !strings.Contains(content, "{WORKFLOW_MODEL_") {
			return nil // nothing to remove
		}

		lines := strings.Split(content, "\n")
		filtered := lines[:0]
		for _, line := range lines {
			if !strings.Contains(line, "{SDD_MODEL_") && !strings.Contains(line, "{WORKFLOW_MODEL_") {
				filtered = append(filtered, line)
			}
		}
		result := strings.Join(filtered, "\n")

		if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
			return fmt.Errorf("removeModelPlaceholderLines: write %q: %w", path, err)
		}
		return nil
	})
}

// copySkillSubdirs copies all subdirectories within a skill source directory
// to the destination. SKILL.md itself is NOT copied here (already handled by RenderSkill).
// Only directories are copied (templates/, references/, etc.)
//
// This is called by each renderer's InstallWorkflow() immediately after RenderSkill()
// to ensure that subdirectory assets (e.g. templates/, references/) are installed
// alongside SKILL.md, not silently dropped.
//
// If srcDir is not a directory (e.g. the caller passed a SKILL.md file path), the
// function returns nil without error — callers do not need to special-case this.
func copySkillSubdirs(srcDir, dstDir string) error {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return nil // source is a file, not a directory — nothing to copy
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // skip files — SKILL.md already handled by RenderSkill
		}
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if err := copyDirRecursive(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy skill subdir %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// CopySkillExtras copies all extra files and subdirectories from a skill source
// directory to the destination. SKILL.md is skipped (already handled by RenderSkill).
//
// Unlike copySkillSubdirs (which only copies subdirectories), this function also
// copies extra files at the root level (e.g. gotchas.md, references/, templates/).
//
// This is exported for use by the materializer during Step 4 skill installation,
// ensuring that catalog-installed skills get all their assets — not just SKILL.md.
//
// If srcDir is not a directory (e.g. the caller passed a SKILL.md file path), the
// function returns nil without error — callers do not need to special-case this.
func CopySkillExtras(srcDir, dstDir string) error {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return nil // source is a file, not a directory — nothing to copy
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.EqualFold(name, "SKILL.md") {
			continue // already handled by RenderSkill
		}
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)
		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return fmt.Errorf("copy skill extra dir %s: %w", name, err)
			}
		} else {
			entryInfo, err := entry.Info()
			if err != nil {
				return fmt.Errorf("copy skill extra file %s: %w", name, err)
			}
			if err := copySingleFile(srcPath, dstPath, entryInfo.Mode()); err != nil {
				return fmt.Errorf("copy skill extra file %s: %w", name, err)
			}
		}
	}
	return nil
}

// ReadMCPDefinitionFromDir reads an MCP server definition map from a cached directory.
// It is the exported equivalent of the package-private readMCPDefinition used by Claude
// and normalizeMCPDefinitions. Callers outside this package (e.g. materializer) should
// use this function instead of duplicating the YAML-reading logic.
// Returns an empty map if no definition file exists.
func ReadMCPDefinitionFromDir(mcpDir string) (map[string]interface{}, error) {
	return readMCPDefinition(mcpDir)
}

// TransformEnvVarValues walks the "env" and/or "environment" keys of a server config
// and transforms `${VAR_NAME}` patterns to the target agent format.
// Supported formats:
//   - "copilot":  ${VAR}  → ${env:VAR}
//   - "opencode": ${VAR}  → {env:VAR}
//
// Returns a new map (does not mutate the original).
// The env/environment sub-map is deep-copied so callers own the result.
//
// This function is exported so it can be used in external integration tests.
// Internal renderers should call the unexported alias transformEnvVarValues.
func TransformEnvVarValues(serverConfig map[string]interface{}, agentFormat string) map[string]interface{} {
	return transformEnvVarValues(serverConfig, agentFormat)
}

// transformEnvVarValues is the internal implementation of TransformEnvVarValues.
func transformEnvVarValues(serverConfig map[string]interface{}, agentFormat string) map[string]interface{} {
	out := make(map[string]interface{}, len(serverConfig))
	for k, v := range serverConfig {
		out[k] = v
	}

	for _, envKey := range []string{"env", "environment"} {
		raw, ok := out[envKey]
		if !ok {
			continue
		}
		envMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		transformed := make(map[string]interface{}, len(envMap))
		for varName, varVal := range envMap {
			if s, ok := varVal.(string); ok {
				transformed[varName] = transformEnvVarPlaceholder(s, agentFormat)
			} else {
				transformed[varName] = varVal
			}
		}
		out[envKey] = transformed
	}

	return out
}

// transformEnvVarPlaceholder converts a single env var value from Claude format
// (${VAR_NAME}) to the target agent format:
//   - "copilot":  ${VAR_NAME} → ${env:VAR_NAME}
//   - "opencode": ${VAR_NAME} → {env:VAR_NAME}
//
// Non-matching values are returned unchanged.
func transformEnvVarPlaceholder(value, agentFormat string) string {
	if !strings.HasPrefix(value, "${") || !strings.HasSuffix(value, "}") {
		return value
	}
	// Extract the variable name between ${ and }.
	inner := value[2 : len(value)-1]
	// Only transform plain VAR_NAME patterns (no nested ${} or spaces).
	if strings.ContainsAny(inner, "{} ") {
		return value
	}
	switch agentFormat {
	case "copilot":
		return "${env:" + inner + "}"
	case "opencode":
		return "{env:" + inner + "}"
	default:
		return value
	}
}

// EffectiveMCPConfig returns the MCP configuration to use for rendering.
// If mcp is nil, returns Claude-compatible defaults.
// This is the SINGLE source of truth for MCP conventions — renderers and
// ManagedConfigPaths() should both use this function.
//
// Defaults:
//   - FilePath:    "../.mcp.json"  (Claude-relative path from workspace dir)
//   - RootKey:     "mcpServers"
//   - EnvKey:      "env"
//   - EnvVarStyle: "${VAR}"        (Claude/canonical placeholder format)
func EffectiveMCPConfig(mcp *model.MCPConfig) model.MCPConfig {
	defaults := model.MCPConfig{
		FilePath:    "../.mcp.json",
		RootKey:     "mcpServers",
		EnvKey:      "env",
		EnvVarStyle: "${VAR}",
	}
	if mcp == nil {
		return defaults
	}
	result := *mcp
	if result.FilePath == "" {
		result.FilePath = defaults.FilePath
	}
	if result.RootKey == "" {
		result.RootKey = defaults.RootKey
	}
	if result.EnvKey == "" {
		result.EnvKey = defaults.EnvKey
	}
	if result.EnvVarStyle == "" {
		result.EnvVarStyle = defaults.EnvVarStyle
	}
	return result
}

// ResolveMCPOutputPath resolves the absolute output path for the MCP config file
// from the workspace root and the MCPConfig.FilePath.
func ResolveMCPOutputPath(workspaceRoot string, mcpConfig model.MCPConfig) string {
	return filepath.Join(workspaceRoot, mcpConfig.FilePath)
}

// ApplyMCPEnvTransform is the exported equivalent of applyMCPEnvTransform.
// Renderers should call the unexported version directly; external callers
// (e.g. integration tests) should use this exported form.
func ApplyMCPEnvTransform(serverConfig map[string]interface{}, mcpConfig model.MCPConfig) map[string]interface{} {
	return applyMCPEnvTransform(serverConfig, mcpConfig)
}

// applyMCPEnvTransform transforms env var values in a server config map according to
// the MCPConfig conventions: renames the env key if needed and reformats placeholders.
// Source format is always ${VAR_NAME} (Claude/canonical format).
// Returns a new map (does not mutate the original).
//
// The function handles both "env" and "environment" source keys, consolidating them
// into the configured mcpConfig.EnvKey in the output map. Placeholder values in the
// form ${VAR_NAME} are rewritten by replacing the VAR token in mcpConfig.EnvVarStyle
// with the actual variable name.
//
// Additionally, the "headers" key (used by HTTP-type MCPs such as ref and context7)
// is transformed in-place: ${VAR_NAME} placeholder values are rewritten using the
// same envVarStyle as env/environment entries.
func applyMCPEnvTransform(serverConfig map[string]interface{}, mcpConfig model.MCPConfig) map[string]interface{} {
	out := make(map[string]interface{}, len(serverConfig))
	for k, v := range serverConfig {
		out[k] = v
	}

	// Collect the env map from whichever source key is present ("env" or "environment").
	// If both are present the last one wins (env first, then environment overrides).
	var envMap map[string]interface{}
	var sourceKey string
	for _, k := range []string{"env", "environment"} {
		raw, ok := out[k]
		if !ok {
			continue
		}
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		envMap = m
		sourceKey = k
	}

	if envMap != nil {
		// Build transformed env map.
		transformed := make(map[string]interface{}, len(envMap))
		for varName, varVal := range envMap {
			if s, ok := varVal.(string); ok {
				transformed[varName] = applyEnvVarStyleTransform(s, mcpConfig.EnvVarStyle)
			} else {
				transformed[varName] = varVal
			}
		}

		// Remove the source key if it differs from the target key.
		if sourceKey != "" && sourceKey != mcpConfig.EnvKey {
			delete(out, sourceKey)
		}
		out[mcpConfig.EnvKey] = transformed
	}

	// Transform "headers" values in-place using the same envVarStyle.
	// HTTP-type MCPs (e.g. ref, context7) pass API keys via headers rather than
	// env/environment, so they must receive the same placeholder transformation.
	if raw, ok := out["headers"]; ok {
		if headersMap, ok := raw.(map[string]interface{}); ok {
			transformedHeaders := make(map[string]interface{}, len(headersMap))
			for headerName, headerVal := range headersMap {
				if s, ok := headerVal.(string); ok {
					transformedHeaders[headerName] = applyEnvVarStyleTransform(s, mcpConfig.EnvVarStyle)
				} else {
					transformedHeaders[headerName] = headerVal
				}
			}
			out["headers"] = transformedHeaders
		}
	}

	return out
}

// applyEnvVarStyleTransform converts a single env var value from Claude canonical
// format (${VAR_NAME}) to the target style defined by envVarStyle.
//
// The envVarStyle pattern uses "VAR" as the variable name token. For example:
//   - "${env:VAR}" → "${env:VAR_NAME}"  (Copilot style)
//   - "{env:VAR}"  → "{env:VAR_NAME}"   (OpenCode style)
//   - "${VAR}"     → "${VAR_NAME}"       (Claude/default — passes through unchanged)
//
// Non-matching values (not in ${...} form, or containing nested ${ or spaces) are
// returned unchanged.
func applyEnvVarStyleTransform(value, envVarStyle string) string {
	if !strings.HasPrefix(value, "${") || !strings.HasSuffix(value, "}") {
		return value
	}
	// Extract the variable name between ${ and }.
	inner := value[2 : len(value)-1]
	// Only transform plain VAR_NAME patterns (no nested ${} or spaces).
	if strings.ContainsAny(inner, "{} ") {
		return value
	}
	// Replace the VAR token in the style pattern with the actual variable name.
	return strings.ReplaceAll(envVarStyle, "VAR", inner)
}
