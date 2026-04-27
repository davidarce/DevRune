# Changelog

All notable changes to DevRune will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.29] — 2026-04-27

### Added — Workspace Command Rules section in generated catalog

The `RenderRootCatalog` renderer now emits an agent-agnostic
`## Workspace Command Rules` section into every generated `CLAUDE.md` /
`AGENTS.md` / equivalent root catalog. The rules instruct the agent to use
`git -C <path>` (and `gh -R <owner>/<repo>` for GitHub) when CWD is a
workspace containing nested git repos rather than a repo itself, and to
never use `cd <path> && git ...` (which Claude Code blocks with a
hardcoded anti-pattern alert that no allowlist can silence).

This block is unconditional (always emitted) and complements the
per-skill `Step 0: Resolve Target Repository` algorithm now present in
`git-commit`, `git-pull-request`, `review-pr`, and `sdd-review` skills in
the starter catalog. For ad-hoc commands outside of skills, the
catalog-level rules apply directly.

**Why**

- Catches improvised `cd <repo> && git ...` calls the agent might
  generate outside of skill execution, where per-skill Step 0 does not
  apply.
- Encodes the workspace contract in always-loaded context, so users who
  work in a parent dir with multiple nested repos (rather than `cd`-ing
  into one repo) get correct guidance without having to remember the
  convention themselves.

**Implementation**

- `internal/materialize/renderers/catalog.go` — new
  `workspaceCommandRulesSection()` helper, invoked unconditionally
  between Invocation Controls and Project Rules sections in
  `RenderRootCatalog`.
- `internal/materialize/renderers/catalog_test.go` — new
  `TestRenderRootCatalog_WorkspaceCommandRules` verifies the section is
  present with the expected canonical snippets.

### Internal — Modernize generics and copy patterns in renderers

Sweep across renderer test files and renderer source: replaced 120
occurrences of `interface{}` with the modern `any` alias (Go 1.18+)
across `claude_test.go`, `copilot_test.go`, `helpers_test.go`, and
`opencode_test.go`. Replaced two manual map-copy loops in `opencode.go`
with `maps.Copy` from the standard library. Restored per-call
operational contract into per-variant launch-template files
(`claude`/`copilot`/`opencode`) and added variant-suffix stripping to
all three cloud-native Go renderers via the shared helper
`copyDirRecursiveStripVariant` in `helpers.go`.

Build clean. 256/256 renderer tests pass.

### Breaking Changes — Advisor `tier` replaced by `scope` (list of strings)

The single-valued `tier` field has been removed from `devrune.yaml`
`customAdvisors` entries and from SKILL.md frontmatter. It is replaced by a
multi-valued `scope` list.

**What changed**

- `tier: <string>` is deleted from `model.AdvisorDef` and from the
  `customAdvisors` YAML schema. Any manifest or SKILL.md file that still
  contains `tier:` will have the unknown field silently ignored by the YAML
  decoder, but the value is no longer acted upon — the advisor will be treated
  as universal.
- `scope: [<list>]` is the new field. It accepts any subset of the controlled
  vocabulary: `frontend`, `backend`, `testing`, `architecture`, `api`,
  `security`, `performance`, `accessibility`.
- **Soft-fallback policy**: unknown scope values (typos, future vocabulary not
  yet recognized by this build) are silently dropped during normalization.
  If all values are dropped the advisor falls back to universal. This is
  intentional — adding a new vocabulary tag is a non-breaking change.
- **Empty / omitted scope = universal**: an advisor with no scope (or an empty
  scope after normalization) applies to every project.
- `recommend.AdvisorClassifications` map removed entirely. Scope is now
  a first-class field of `model.AdvisorDef`, populated from SKILL.md
  frontmatter. The deprecated `AdviserClassifications` alias is also removed.
- `recommend.AdvisorTier` type, the three `AdvisorTierFrontend/Backend/Universal`
  constants, the `AdviserTier` type alias, `BuildAdvisorClassifications()`, and
  `KnownNativeAdvisors()` are all deleted from the `recommend` package.
- A new `internal/advisormeta/` package handles SKILL.md frontmatter reads and
  native advisor scope discovery. `recommend` and `cli` packages remain
  filesystem-free; `advisormeta` is the only new filesystem-aware package.
- TUI advisor toggle rows now display scope as `advisor-name (scope1, scope2)`.
  Advisors with empty scope (universal) display as just `advisor-name`.
- `performance-advisor` moved from universal to `scope: [performance]`.
- `security-advisor` moved from universal to `scope: [security]`.

**Migration guide**

| Before | After |
|--------|-------|
| `tier: frontend` | `scope: [frontend]` |
| `tier: backend` | `scope: [backend]` |
| `tier: universal` | _(omit `scope` entirely, or use a domain-specific list)_ |
| _(no `tier`)_ | _(no change — already treated as universal)_ |

In SKILL.md frontmatter:

```yaml
# Before
---
name: my-advisor
tier: backend
---

# After
---
name: my-advisor
scope: [backend]
---
```

In `devrune.yaml` `customAdvisors`:

```yaml
# Before
customAdvisors:
  - name: security-advisor
    skillSource: ./advisors/security-advisor
    tier: universal

# After
customAdvisors:
  - name: security-advisor
    skillSource: ./advisors/security-advisor
    scope: [security]
```

### Added — `devrune sdd-advisors` command

A new interactive command for managing SDD advisors after initial setup. Previously,
changing the advisor set required a manual edit of `devrune.yaml` followed by a full
`devrune resolve` + install pipeline. The new command makes advisor management a
first-class, lightweight operation.

**New command: `devrune sdd-advisors`**

Interactive TUI plus non-interactive CLI flags for every action:

- **Toggle advisors** — install or uninstall individual native advisors without running a
  full resolve. Equivalent flag: `--install <name>` / `--uninstall <name>`.
- **Add advisor** — a unified flow that accepts a source scheme (`local:`, `github:`,
  `gitlab:`) and installs one or many advisors from that source. Local sources (e.g.
  `local:./advisors/security-advisor`) cover the single-user custom-advisor case; git
  sources (`github:owner/repo`, `gitlab:group/repo`) cover shared advisor catalogs.
  Equivalent flag: `--add-advisor source=<SCHEME:PATH>`.
- **Remove advisor** — permanently removes a local or catalog-imported advisor, deletes
  `.claude/skills/<name>/` recursively, and re-renders all downstream files.
  Equivalent flag: `--remove-advisor <name>`.
- **Manage catalogs** — add, remove, or refresh catalog sources.
  Equivalent flags: `--add-catalog`, `--remove-catalog`, `--refresh-catalogs`.

The command is also reachable from the `devrune` interactive main menu as
**"Manage SDD advisors"**, placed below the existing "Configure role models" entry.

**New manifest fields in `devrune.yaml`**

```yaml
customAdvisors:
  - name: security-advisor
    source: local:./advisors/security-advisor

advisorCatalogs:
  - source: github:acme/advisor-catalog@main
```

- `customAdvisors:` — list of non-native advisors added via `--add-advisor`. Each entry
  records the advisor name and its original source so the catalog can be refreshed later.
- `advisorCatalogs:` — list of external catalog sources (local directory or git repository)
  registered for future `--refresh-catalogs` runs.

**New catalog cache: `.devrune/advisor-catalogs/`**

Git-based catalog sources are cloned or fetched into
`.devrune/advisor-catalogs/<sha1-of-url>/` (alongside the existing package cache). Local
sources are scanned in place. The cache is populated automatically the first time a
catalog source is used and is updated by `--refresh-catalogs` / the "Refresh catalogs"
TUI action.

**Full-catalog sync on every change**

After any add, remove, or toggle operation, DevRune regenerates:
- `.claude/agents/<name>.md` (Claude) and `.github/agents/<name>.agent.md` (Copilot)
  for each affected advisor.
- The advisor rows in the Skills table of `CLAUDE.md` and `AGENTS.md` via the existing
  catalog renderer.
- The advisor tables inside SDD skill instruction files (`sdd-plan/SKILL.md`,
  `sdd-review/SKILL.md`, `sdd-orchestrator/ORCHESTRATOR.md`), updated within the
  `<!-- devrune advisors:begin -->` / `<!-- devrune advisors:end -->` managed block.

### Changed — `-adviser` → `-advisor` suffix rename (user-facing)

All native skills shipped with DevRune now use the **`-advisor`** suffix (American
English). The previous `-adviser` (British English) suffix has been retired from all
user-facing names, CLI output, manifest keys, and generated file paths.

**What changed**

- Native advisor skill names: `architect-advisor`, `api-first-advisor`,
  `unit-test-advisor`, `integration-test-advisor`, `frontend-test-advisor`,
  `component-advisor`, `web-accessibility-advisor`.
- Generated agent wrapper paths: `.claude/agents/*-advisor.md` (was `*-adviser.md`).
- Manifest package selection keys now use `*-advisor` names.
- `AdvisorClassifications` replaces `AdviserClassifications` in the internal Go API
  (the old name is retained as a deprecated alias — see below).

**Deprecation: `sdd-advisers` command alias**

The old `devrune sdd-advisers` command still works in this release but prints a
deprecation notice on every invocation:

```
DEPRECATED: "sdd-advisers" is deprecated; use "sdd-advisors" instead.
This alias will be removed in the next minor release.
```

The alias will be removed in the next minor release. Update any scripts or tooling to
use `devrune sdd-advisors`.

**Deprecation: `AdviserClassifications` Go symbol**

The internal `AdviserClassifications` map (used by external resolver integrations) is
retained as a deprecated alias of `AdvisorClassifications` for this release and will be
deleted in the next minor release alongside the command alias removal.

**Starter catalog follow-up**

The `devrune-starter-catalog` repository still uses `*-adviser/` directory names in
this release. A follow-up PR will rename all `*-adviser/` directories to `*-advisor/`
and update SKILL.md frontmatter accordingly. Track progress in the linked PR.

### Changed — BREAKING: Claude SDD layout migration (Claude-native)

The Claude renderer has been rewritten to always produce a Claude-native SDD
layout. There is no opt-in flag — this is a total migration. Existing Claude SDD
installations will have their `.claude/skills/sdd-*/` layout replaced on the
next `devrune install`. Subagents now live in `.claude/agents/*.md` with a
`skills:` preload field, so the orchestrator no longer emits per-launch
`Skill()` boilerplate.

**What changed**

- The Claude renderer (`internal/materialize/renderers/claude.go`,
  `InstallWorkflow`) is fully rewritten. It now writes:
  - **Phase subagents** at `.claude/agents/sdd-explorer.md`,
    `.claude/agents/sdd-planner.md`, `.claude/agents/sdd-implementer.md`,
    and `.claude/agents/sdd-reviewer.md`. Each file declares a
    `skills:` preload pointing at the matching `sdd-{phase}` skill and
    intentionally omits `tools:` so the subagent inherits the parent
    allowlist.
  - **Adviser subagents** at `.claude/agents/architect-adviser.md`,
    `.claude/agents/api-first-adviser.md`,
    `.claude/agents/unit-test-adviser.md`,
    `.claude/agents/integration-test-adviser.md`,
    `.claude/agents/component-adviser.md`,
    `.claude/agents/frontend-test-adviser.md`, and
    `.claude/agents/web-accessibility-adviser.md`. Each adviser wrapper
    is read-only with an explicit `tools: [Read, Grep, Glob]` allowlist.
  - A slim `.claude/skills/sdd-orchestrator/ORCHESTRATOR.md`, materialized
    from the new source file
    `devrune-starter-catalog/workflows/sdd/ORCHESTRATOR.claude.md` (the
    `.claude` suffix is stripped at install time). This variant drops the
    per-launch `Skill()` boilerplate and inlines the implement-phase wave
    mechanics and the adviser invocation snippet directly, so the
    orchestrator no longer references `_shared/launch-templates.md` or
    `_shared/adviser-templates.md` at runtime. Those shared files remain
    in the source tree for Codex/Factory installs but are unused by the
    Claude-native orchestrator body.

**Why**

- Saves roughly ~3,000-4,500 tokens per orchestrator launch by removing the
  per-launch `Skill()` preamble and cross-document reads.
- Establishes a single source of truth: model, tools, and MCP server
  assignments live in `.claude/agents/*.md` only — not duplicated between
  the orchestrator prompt and the skill frontmatter.
- Cleaner runtime: subagents boot without an inline `Skill()` call, so
  the orchestrator prompt is shorter and the subagent context starts
  smaller.

**User action required**

- Re-run `devrune install` to materialize the new layout. The
  materializer's `ManagedPaths` cleanup will remove the legacy
  `.claude/skills/sdd-*` artifacts that are no longer part of the managed
  set. Any user-authored files outside DevRune's managed paths
  (including hand-authored files in `.claude/agents/`) are preserved.

[0.1.29]: https://github.com/davidarce/devrune/releases/tag/v0.1.29
