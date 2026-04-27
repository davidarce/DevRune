# devrune.yaml Schema Reference

This document describes the schema and validation rules for `devrune.yaml`, the
DevRune user manifest file. Fields are listed in the order they appear in a
typical manifest.

---

## Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `schemaVersion` | string | Yes | Always `devrune/v1`. |
| `packages` | list | No | Package sources and skill/rule selections. |
| `customAdvisors` | list | No | User-defined or catalog-imported advisors. |
| `advisorCatalogs` | list | No | External advisor catalog sources to fetch from. |
| `workflows` | object | No | Workflow-level configuration (e.g. SDD model overrides — owned by "Configure role models", not the `sdd-advisors` command). |

---

## `customAdvisors`

Inline list of user-defined or catalog-imported advisors. The `devrune sdd-advisors`
command writes to this list when you add or remove custom advisors.

### Schema

```yaml
customAdvisors:
    - name: <string>            # required
      description: <string>     # optional; populates the agent frontmatter
      skillSource: <path>       # required; see SkillSource values below
      scope: [<list>]           # optional; default: universal (empty = applies to every project)
      origin: <string>          # optional; "custom" | "catalog"; default: "custom"
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Canonical identifier for the advisor. Must end in `-advisor`. |
| `description` | string | No | Human-readable description, used in agent frontmatter and catalog tables. |
| `skillSource` | string | Yes | Path or URL pointing to the advisor skill directory. See [SkillSource values](#skillsource-values). |
| `scope` | list of strings | No | Project domains this advisor targets. Recognized values: `frontend`, `backend`, `testing`, `architecture`, `api`, `security`, `performance`, `accessibility`. Unknown values are silently dropped. Omit (or leave empty) for a universal advisor that applies to every project. |
| `origin` | string | No | Records how the advisor was added. One of `custom` (user-authored) or `catalog` (imported from an `advisorCatalogs` entry). Defaults to `custom`. |

### Validation rules

- `name` **must** end with the suffix `-advisor`. Names ending in `-adviser` are
  accepted during a one-release compatibility window (see [Migration note](#migration-note-adviser--advisor))
  but emit a deprecation warning and will be rejected in the next minor release.
- `name` **must not** collide with a native advisor name (e.g. `architect-advisor`).
  Registering a custom advisor with the same name as a native advisor is rejected at
  parse time.
- Duplicate `name` values within the `customAdvisors` list are rejected
  (case-sensitive comparison).
- `skillSource` **must** point to a directory that contains a `SKILL.md` file at
  its root. Pointing at a bare `SKILL.md` file is a hard error:
  ```
  skillSource must be a directory containing SKILL.md, got file: <path>
  ```
- `scope` values are validated against the controlled vocabulary (`frontend`,
  `backend`, `testing`, `architecture`, `api`, `security`, `performance`,
  `accessibility`). **Unknown values are silently dropped** (soft-fallback policy)
  rather than rejected — this preserves forward compatibility when a new vocabulary
  tag is added in a future release. Vocabulary matching is case-sensitive; `Frontend`
  is treated as unknown and dropped.
- `origin` must be one of `custom`, `catalog`, or empty (treated as `custom`).

### SkillSource values

`skillSource` accepts three formats:

| Format | Example | Behaviour |
|--------|---------|-----------|
| Relative path | `./advisors/security-advisor` | Resolved against the directory containing `devrune.yaml`. Source is used in place — no copy into cache. |
| Absolute path | `/home/user/advisors/security-advisor` | Used as-is. Accepted, but updates require manual re-sync if the source changes outside the project tree. |
| Catalog cache path | `.devrune/advisor-catalogs/<sha1>/performance-advisor` | Written automatically by the catalog-import flow. Do not edit by hand. |

> **Note**: `github:` / `gitlab:` URLs are **not** valid `skillSource` values.
> Those schemes appear in `advisorCatalogs[].url`. When a catalog advisor is
> imported, `skillSource` is set to the local cache path under
> `.devrune/advisor-catalogs/`.

---

## `advisorCatalogs`

List of external advisor catalog sources. The `devrune sdd-advisors` command
writes to this list when you add or remove catalog sources. Catalogs are
**not** fetched automatically on `devrune sync` — fetch is triggered explicitly
via `devrune sdd-advisors --refresh-catalogs` (or the TUI "Refresh catalogs"
action) to avoid surprise network calls in CI.

### Schema

```yaml
advisorCatalogs:
    - url: <string>             # required; scheme-prefixed URL
      name: <string>            # optional; human-readable alias
      lastFetched: <RFC3339>    # optional; set automatically on fetch
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | Scheme-prefixed catalog URL. See [Supported URL schemes](#supported-url-schemes). |
| `name` | string | No | Human-readable alias shown in the TUI catalog list. |
| `lastFetched` | string | No | RFC 3339 timestamp of the most recent successful fetch. Written automatically — do not set by hand. |

### Supported URL schemes

| Scheme | Format | Example | Behaviour |
|--------|--------|---------|-----------|
| `local:` | `local:<path>` | `local:./advisors` or `local:/abs/path` | Filesystem directory. No fetch, no cache. Source is scanned in place. |
| `github:` | `github:owner/repo` or `github:owner/repo@ref` | `github:acme/advisor-catalog@main` | Public GitHub repository. Fetched via `git clone --depth=1`. On subsequent refreshes: `git pull --ff-only`. `ref` can be a branch, tag, or full SHA. |
| `gitlab:` | `gitlab:owner/repo` or `gitlab:owner/repo@ref` | `gitlab:myorg/advisors@v2` | Public GitLab repository. Same fetch behaviour as `github:`. |

### Validation rules

- `url` must begin with one of the recognised scheme prefixes (`local:`,
  `github:`, `gitlab:`). An unknown prefix is rejected at parse time.
- The path or repository portion after the scheme prefix must be non-empty.
- Duplicate `url` values within the `advisorCatalogs` list are rejected
  (exact string comparison, case-sensitive).

### Catalog layout contract

DevRune v1 recognises only the **top-level flat layout** inside a catalog root:

```
<catalogRoot>/
├── security-advisor/
│   └── SKILL.md           <- recognised
├── performance-advisor/
│   ├── SKILL.md           <- recognised
│   └── references/
│       └── benchmarks.md
└── backend/
    └── db-advisor/
        └── SKILL.md       <- NOT recognised in v1 (nested layout)
```

Subdirectories whose name does not end in `-advisor` are skipped with a warning
log line. Nested layouts (`<root>/backend/db-advisor/SKILL.md`) are not
supported in v1; support is planned for a future release.

### Cache directory layout

Fetched `github:` and `gitlab:` catalogs are stored under:

```
.devrune/
└── advisor-catalogs/
    └── <sha1(url)>/        # deterministic; stable across re-fetches
        ├── .git/
        └── performance-advisor/
            └── SKILL.md
```

The cache key is the SHA-1 of the full catalog URL string (including scheme and
`@ref`). Partial clones can be retried by re-running the same catalog URL — no
manual cleanup is required.

### Authentication (v1 limitation)

`github:` and `gitlab:` sources support **public repositories only** in v1.
Private repos require authentication (tokens, SSH), which is deferred to v2.
If you point at a private repo, the underlying `git clone` will fail with a
403 or 404 error from the host. The error message includes a hint:

```
hint: private repos require authentication, which is a v2 feature
```

---

## Migration note: `-adviser` to `-advisor`

DevRune is renaming all native advisor skill names from the `-adviser` suffix
(British spelling) to `-advisor` (American spelling). This affects:

- Native skill names in `packages[].select.skills[]`
  (e.g. `architect-adviser` → `architect-advisor`)
- The `name` field of any `customAdvisors` entries
- The directory names under `.claude/skills/` and `.claude/agents/`

### Compatibility window

The `-adviser` suffix is accepted for **one release** after the rename lands.
During that window:

- Manifest parsing accepts `-adviser` names without error.
- A deprecation warning is emitted once per process:
  ```
  DEPRECATION: advisor name "architect-adviser" uses legacy '-adviser' suffix;
  rename to '-advisor' before next minor release
  ```
- `SyncAdvisors` processes the legacy name transparently.

After the compatibility window closes (next minor release), manifests containing
`-adviser` names will fail validation.

### How to migrate

1. Update `packages[].select.skills[]` entries: replace every `-adviser` suffix
   with `-advisor`.
2. Update any `customAdvisors[].name` entries similarly.
3. Run `devrune sync` to regenerate agent files under the new names.
4. Remove the old `.claude/agents/*-adviser.md` files if they were not
   overwritten automatically.

The `devrune-starter-catalog` package will ship renamed directories
(`*-advisor/`) in the same release. A `devrune install` after upgrading
DevRune will pull the renamed skill directories into `.claude/skills/`.

---

## Full example

The following is the reference `devrune.yaml` shape after the `sdd-advisors`
feature lands (Contract #7). Fields from unrelated features (`workflows`) are
shown for completeness; they are not written by `sdd-advisors`.

```yaml
schemaVersion: devrune/v1
packages:
    - source: local:/.../devrune-starter-catalog
      select:
        skills:
            - architect-advisor
            - api-first-advisor

customAdvisors:
    - name: security-advisor
      description: OWASP threat-modelling specialist
      skillSource: ./advisors/security-advisor
      scope: [security]
      origin: custom
    - name: performance-advisor
      description: Perf budgets & p99 latency
      skillSource: .devrune/advisor-catalogs/7f3a9b.../performance-advisor
      scope: [performance]
      origin: catalog

advisorCatalogs:
    - url: github:acme/advisor-catalog@main
      name: Acme corporate advisors
      lastFetched: 2026-04-23T10:12:00Z
    - url: local:./advisors
      lastFetched: 2026-04-23T10:15:00Z

workflows:
    sdd:
        modelOverrides:
            sdd-plan: claude-opus-4.6
            sdd-implement: claude-sonnet-4.6
            sdd-review: claude-opus-4.6
            advisor: claude-sonnet-4.6
```

> **Note**: the `workflows.sdd.modelOverrides` block is owned by the
> "Configure role models" flow inside the DevRune main menu. No code in the
> `devrune sdd-advisors` command reads or writes this section. It is shown
> here only so the full post-feature manifest shape is complete.
