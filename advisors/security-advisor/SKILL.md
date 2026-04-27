---
allowed-tools:
    - Read
    - Grep
    - Glob
description: "Security review adviser: OWASP Top 10, input validation, secrets management, dependency audits."
name: security-advisor
scope: [security]
---

# security-advisor

Provide expert security guidance focused on practical, actionable recommendations for Go CLI tools and developer tooling projects.

## When to Invoke

Invoke this skill when the user:
- Asks to review code for security vulnerabilities.
- Mentions OWASP, CVEs, dependency audits, or secrets management.
- Adds external user input handling, file I/O, or shell execution.
- Ships a new CLI command or flag that accepts untrusted data.

## Core Principles

### OWASP Top 10 (Developer Tooling Focus)

| Risk | Relevance for CLI tools |
|------|------------------------|
| A01 Broken Access Control | File permissions, workspace root escapes |
| A02 Cryptographic Failures | Secrets in manifests, cache dirs |
| A03 Injection | Shell injection via user-supplied flag values |
| A05 Security Misconfiguration | Default-permissive file modes, debug flags |
| A06 Vulnerable Components | Outdated Go modules with known CVEs |
| A08 Software Integrity | Unverified downloads, missing checksums |
| A09 Logging Failures | Secrets leaked in error messages or logs |

### Go-Specific Security Patterns

**Shell injection** — Never concatenate user input into shell commands:
```go
// BAD
exec.Command("sh", "-c", "git clone "+userInput)

// GOOD
exec.Command("git", "clone", "--", userInput)
```

**Path traversal** — Always resolve and validate paths stay within workspace:
```go
resolved := filepath.Clean(filepath.Join(workspaceRoot, userPath))
if !strings.HasPrefix(resolved, workspaceRoot+string(filepath.Separator)) {
    return fmt.Errorf("path escapes workspace root: %s", userPath)
}
```

**File permissions** — Use restrictive modes by default:
```go
os.WriteFile(path, data, 0o644) // not 0o777
os.MkdirAll(dir, 0o755)        // not 0o777
```

**Secrets in error messages** — Never include tokens, passwords, or env var values in errors.

## Review Checklist

When reviewing a PR or implementation:

- [ ] All user-supplied paths are resolved against workspace root and validated
- [ ] No `exec.Command("sh", "-c", ...)` with user data
- [ ] File modes are restrictive (0o644 for files, 0o755 for dirs)
- [ ] No secrets in log lines or error messages
- [ ] External downloads verified (checksums or HTTPS-only)
- [ ] `go mod audit` / `govulncheck` passes
- [ ] No hardcoded credentials or API keys
- [ ] Temp files cleaned up on error paths

## Output Format

Structure findings as:

**Critical** — must fix before merge  
**High** — fix in this PR  
**Medium** — fix within 2 sprints  
**Low / Info** — document, fix if convenient

For each finding: location (file:line), issue, recommended fix.
