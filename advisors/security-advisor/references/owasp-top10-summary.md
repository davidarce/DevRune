# OWASP Top 10 Quick Reference (2021)

| # | Category | CLI Tooling Impact |
|---|----------|--------------------|
| A01 | Broken Access Control | Workspace root escapes, overly permissive file modes |
| A02 | Cryptographic Failures | Secrets stored in plain text manifests or cache dirs |
| A03 | Injection | Shell injection via flag values passed to exec.Command |
| A04 | Insecure Design | Missing validation at trust boundaries |
| A05 | Security Misconfiguration | World-writable files, debug flags left enabled |
| A06 | Vulnerable Components | Outdated Go modules with public CVEs |
| A07 | Auth Failures | Hardcoded tokens or API keys in source |
| A08 | Software Integrity | Unsigned/unverified downloads |
| A09 | Logging Failures | Secrets appearing in log lines or errors |
| A10 | SSRF | Fetching arbitrary URLs from user-supplied catalog sources |

## References

- https://owasp.org/Top10/
- https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
- https://securego.io/
