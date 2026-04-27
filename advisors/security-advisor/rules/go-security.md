# Go Security Rules

## Input Validation

- Validate all CLI flag values at the boundary (Cobra RunE), not deep in business logic.
- Reject empty strings where non-empty is required — don't silently default.
- Reject paths containing `..` components before resolving them.

## Shell Execution

- Use `exec.Command(binary, args...)` — NEVER `exec.Command("sh", "-c", combined)` with user data.
- Pass user-controlled values as separate arguments, never interpolated into a command string.
- Use `cmd.CombinedOutput()` and wrap errors with context (include the command args, NOT the output if it could contain secrets).

## File System

- Always use `os.MkdirAll` with mode `0o755` for directories.
- Always use `os.WriteFile` / `os.Create` with mode `0o644` for regular files.
- Never use `0o777` — it grants world-write to sensitive config files.
- Temp files: use `os.CreateTemp` and defer removal; clean up in error paths too.

## Dependency Management

- Run `govulncheck ./...` before release.
- Pin indirect dependencies when a CVE affects them.
- Prefer stdlib over third-party for crypto operations.

## Logging and Errors

- Never include `os.Getenv("TOKEN")` or similar in error messages.
- Never log raw HTTP responses that may contain auth headers.
- Use structured logging levels — debug output should be opt-in (`--verbose`).
