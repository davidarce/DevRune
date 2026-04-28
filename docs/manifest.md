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
| `advisors` | list | No | External advisor sources and which advisors to install from each. |
| `workflows` | object | No | Workflow-level configuration (e.g. SDD model overrides — owned by "Configure role models", not the `sdd-advisors` command). |

---

## `advisors`

List of external advisor sources and which advisors to install from each.
The shape mirrors `packages[]` exactly — one source per entry plus an
optional `select` list of names. The `devrune sdd-advisors` command
writes to this list when you add, remove, or refresh advisors.

Catalogs are **not** fetched automatically on `devrune sync` — fetch is
triggered explicitly via `devrune sdd-advisors --refresh-catalogs` (or
the TUI "Refresh catalogs" action) to avoid surprise network calls in CI.

### Schema

```yaml
advisors:
    - source: <string>          # required; scheme-prefixed URL
      lastFetched: <RFC3339>    # optional; set automatically on fetch
      select:                   # optional; list of advisor names to install
        - <name-1>              #   (each must end in "-advisor")
        - <name-2>              # OMIT or leave empty → install ALL "*-advisor/"
                                #   directories found under the resolved source
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | Yes | Scheme-prefixed source URL. See [Supported URL schemes](#supported-url-schemes). |
| `lastFetched` | string | No | RFC 3339 timestamp of the most recent successful fetch. Written automatically — do not set by hand. |
| `select` | list of strings | No | Names of advisors to install from this source (each must end in `-advisor`). When omitted or empty, every `*-advisor/` directory discovered by the scanner is installed. |

### Supported URL schemes

| Scheme | Format | Example | Behaviour |
|--------|--------|---------|-----------|
| `local:` | `local:<path>` | `local:./advisors` or `local:/abs/path` | Filesystem directory. No fetch, no cache. Source is scanned in place. |
| `github:` | `github:owner/repo` or `github:owner/repo@ref` | `github:acme/advisor-catalog@main` | Public GitHub repository. Fetched via `git clone --depth=1`. On subsequent refreshes: `git pull --ff-only`. `ref` can be a branch, tag, or full SHA. |
| `gitlab:` | `gitlab:owner/repo` or `gitlab:owner/repo@ref` | `gitlab:myorg/advisors@v2` | Public GitLab repository. Same fetch behaviour as `github:`. |

### Validation rules

- `source` must begin with one of the recognised scheme prefixes (`local:`,
  `github:`, `gitlab:`). An unknown prefix is rejected at parse time.
- The path or repository portion after the scheme prefix must be non-empty.
- Duplicate `source` values within the `advisors` list are rejected
  (exact string comparison, case-sensitive).
- Each entry in `select` must end in `-advisor` (case-sensitive). Bare
  `-advisor` (no prefix) is rejected.
- `select` entries **must not** collide with a native advisor name (e.g.
  `architect-advisor`). Native advisors come from the primary package
  catalog via `packages[].select.skills`, not from `advisors[]`.

### Description and scope: NOT persisted

Each advisor's `description` and `scope` (e.g. `[security]`, `[performance]`)
live in the advisor's own `SKILL.md` frontmatter — the single source of
truth. They are never written into `devrune.yaml`. The runtime resolver
populates them automatically when the source is fetched and scanned, so
there is no drift between manifest and disk.

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
- Entries in `advisors[].select[]`
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
2. Update any `advisors[].select[]` entries similarly.
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

advisors:
    # Local source — only the security-advisor is installed; other
    # *-advisor/ subdirectories under ./advisors are ignored.
    - source: local:./advisors
      lastFetched: 2026-04-23T10:15:00Z
      select:
        - security-advisor

    # Remote catalog — performance-advisor is installed from the cached
    # clone under .devrune/advisor-catalogs/<sha1>/. Omitting `select`
    # would install every *-advisor/ directory the catalog contains.
    - source: github:acme/advisor-catalog@main
      lastFetched: 2026-04-23T10:12:00Z
      select:
        - performance-advisor

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
