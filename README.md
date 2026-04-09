<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go 1.26+">
  <img src="https://img.shields.io/github/v/release/davidarce/DevRune?style=flat&color=8B5CF6" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/davidarce/DevRune/ci.yml?label=CI&style=flat" alt="CI">
  <img src="https://img.shields.io/github/license/davidarce/DevRune?style=flat&color=gray" alt="License">
</p>

<p align="center">

```
██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗
██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝
██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗
██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝
██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗
╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝
```

</p>

<p align="center">
  <strong>Package manager for AI agent instructions</strong>
  <br>
  <em>One manifest, every agent. Skills, rules, MCPs, workflows, and tools — resolved, locked, installed.</em>
</p>

---

DevRune configures AI development agents by resolving, fetching, and materializing packages of **skills**, **rules**, **MCP server definitions**, **workflows**, and **tools** into your workspace. Write one `devrune.yaml` manifest and DevRune generates the correct files for every agent you use.

Think of it as **npm for AI agent instructions** — you declare what you need, DevRune handles the rest.

## Supported Agents

| Agent | Workspace | Format |
|-------|-----------|--------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `.claude/` | Skills as markdown, `CLAUDE.md` catalog, `settings.json`, `.mcp.json` |
| [Codex (OpenAI)](https://developers.openai.com/codex) | `.agents/` + `.codex/` | Skills in `.agents/skills/`, `AGENTS.md` catalog, `config.toml` (TOML) MCP |
| [OpenCode](https://opencode.ai) | `.agents/` + `.opencode/` | Skills in `.agents/skills/`, `AGENTS.md` catalog, `opencode.json` MCP |
| [GitHub Copilot](https://github.com/features/copilot) | `.github/` | Skills + native agents (`.agent.md`), `copilot-instructions.md`, `.mcp.json` |
| [Factory Droid](https://docs.factory.ai) | `.agents/` + `.factory/` | Skills in `.agents/skills/`, `AGENTS.md` catalog, `mcp.json` |

Each renderer transforms the canonical format (Claude-style markdown with YAML frontmatter) into the agent's native format — handling model name mapping, tool aliases, frontmatter conversion, and MCP config generation automatically.

> **Shared `.agents/` directory**: Codex, OpenCode, and Factory all read skills from `.agents/skills/`. DevRune generates skills there once and deduplicates automatically when multiple agents share the same directory.

## Install

**One-liner** (recommended):

```bash
curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | bash
```

**Pin a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | VERSION=v0.1.0 bash
```

**With Go:**

```bash
go install github.com/davidarce/devrune/cmd/devrune@latest
```

**From source:**

```bash
git clone https://github.com/davidarce/DevRune.git
cd DevRune
make install
```

Installs to `~/.local/bin` by default. Override with `INSTALL_DIR=/usr/local/bin`.

## Quick Start

### Interactive Menu

```bash
devrune
```

```
██████╗ ███████╗██╗   ██╗██████╗ ██╗   ██╗███╗   ██╗███████╗
██╔══██╗██╔════╝██║   ██║██╔══██╗██║   ██║████╗  ██║██╔════╝
██║  ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║█████╗
██║  ██║██╔══╝  ╚██╗ ██╔╝██╔══██╗██║   ██║██║╚██╗██║██╔══╝
██████╔╝███████╗ ╚████╔╝ ██║  ██║╚██████╔╝██║ ╚████║███████╗
╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝
Package manager for AI agent instructions

What would you like to do?
> Setup
  Sync project
  Status
  Upgrade DevRune
  Uninstall
```

The interactive menu gives you quick access to all DevRune operations. Select **Setup** to configure agents and install packages from your catalogs — the wizard walks you through agent selection, package sources, and developer tools.

### Manual

**1. Create `devrune.yaml`:**

```yaml
schemaVersion: devrune/v1

# Packages: skills and rules from one or more catalogs.
# Use select: to pick specific items, or omit for all.
packages:
  - source: github:davidarce/devrune-starter-catalog@main
    select:
      skills:
        - architect-adviser       # Clean architecture patterns
        - git-commit              # Conventional Commits automation
        - git-pull-request        # PR creation (GitHub + GitLab)
        - review-pr               # AI-powered PR code review
        - unit-test-adviser       # Domain unit test patterns
        - api-first-adviser       # OpenAPI / REST design
        - component-adviser       # React component patterns
      rules:
        - architecture/clean-architecture-rules
        - tech/java-spring-rules
        - tech/react-rules
        - testing/mother-pattern-rules
        - testing/adapter-it-patterns-rules

# MCP servers: tool integrations (Jira, memory, docs, search).
mcps:
  - source: github:davidarce/devrune-starter-catalog@main//mcps/atlassian.yaml
  - source: github:davidarce/devrune-starter-catalog@main//mcps/engram.yaml
  - source: github:davidarce/devrune-starter-catalog@main//mcps/context7.yaml
  - source: github:davidarce/devrune-starter-catalog@main//mcps/exa.yaml
  - source: github:davidarce/devrune-starter-catalog@main//mcps/ref.yaml

# Agents: which AI agents to configure. DevRune generates the correct
# files for each one (skills, settings, MCP config, catalog).
agents:
  - name: claude    # .claude/ + CLAUDE.md
  - name: codex     # .agents/ + .codex/ + AGENTS.md
  - name: opencode  # .agents/ + .opencode/ + AGENTS.md
  - name: copilot   # .github/ + copilot-instructions.md
  - name: factory   # .agents/ + .factory/ + AGENTS.md

# Workflows: multi-phase development flows.
workflows:
  - github:davidarce/devrune-starter-catalog@main//workflows/sdd

# Catalogs: source refs auto-derived from your packages/mcps/workflows.
# Used by `devrune init` to pre-load sources in the wizard on re-run.
# Managed automatically by sync — you don't need to edit this manually.
catalogs:
  - github:davidarce/devrune-starter-catalog

# Install preferences (optional).
install:
  linkMode: copy              # copy | symlink | hardlink
  rulesMode:
    claude: concat            # concat | individual | both
    opencode: individual
```

**2. Sync (resolve + install):**

```bash
devrune sync   # Fetch packages, update lockfile, materialize workspace
```

That's it. Your workspace now has correctly formatted skills, rules, MCP configs, settings, and workflows for all configured agents. If the catalog includes developer tools (e.g. Crit, Engram), `devrune init` will also offer to install them via Homebrew.

> You can also run `devrune resolve` and `devrune install` separately for advanced workflows (CI/CD, offline installs).

## AI Recommendations

During `devrune init`, the TUI wizard offers an **AI-powered recommendation** option. After selecting your catalog sources and reviewing the available content, you'll see two action buttons:

- **Confirm selection** — proceed with your manual selection
- **AI Recommendations** — analyze your project and suggest relevant items from the catalog

When you choose AI Recommendations, DevRune:

1. **Detects your project profile** — scans languages, dependencies (`go.mod`, `package.json`, `pom.xml`, etc.), frameworks, and configuration files using [go-enry](https://github.com/go-enry/go-enry)
2. **Invokes an AI agent** — sends the project profile and catalog items to Claude (via `claude -p`) or OpenCode, requesting structured recommendations with confidence scores
3. **Shows the results** — displays recommended items grouped by category (Skills, Rules, MCPs, Workflows) with confidence percentages and reasons
4. **Lets you decide** — apply the recommendations to merge them into your selection, or go back to adjust manually

**Requirements:**
- An AI coding agent must be installed: [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (preferred) or [OpenCode](https://opencode.ai)
- The agent must be authenticated (logged in)

**Performance:**
- First recommendation takes 15-40 seconds depending on the model and catalog size
- Results are cached for 1 hour at `~/.cache/devrune/recommend/` — subsequent runs with the same project and catalogs return instantly
- Use `devrune cache clean` to invalidate cached recommendations

> The AI recommendation feature is optional — the wizard works exactly the same without it. If no AI agent is installed, only the "Confirm selection" button appears.

## Skills.sh Curated Catalog

DevRune integrates with [skills.sh](https://skills.sh) — the open agent skills directory — through a **curated static catalog** of audit-verified skills grouped by technology.

When you run `devrune init`, the TUI wizard shows **Skills.sh Curated Catalog** as a selectable source alongside your other catalogs. If enabled, DevRune detects your project's tech stack (languages, frameworks, and dependencies) and surfaces only the skills that match.

**How it works:**

1. **Tech detection** — uses the same `detect.Analyze()` engine as AI Recommendations to scan your project (package.json, go.mod, pom.xml, build.gradle, etc.)
2. **Static registry** — a compiled-in Go map of curated skills.sh skill paths, grouped by framework/language (React, Next.js, Spring Boot, Java, Go, Python, TypeScript, etc.)
3. **Audit-verified** — every skill in the registry has been manually verified to pass security audits from all three skills.sh providers (Gen Agent Trust Hub, Socket, Snyk)
4. **Zero network calls** — the registry is in-memory; actual skill content is fetched only during `devrune resolve` via the existing cached pipeline

**In the TUI:**

Skills appear in Step 3 under a "Skills.sh Curated" group with skills organized by detected technology. You can toggle individual skills on/off just like starter-catalog items.

**Currently supported technologies:**

| Frontend | Backend | Languages |
|----------|---------|-----------|
| React | Spring Boot | Java |
| Next.js | Django | Kotlin |
| Vue.js | FastAPI | Go |
| Angular | Astro | Python |
| Svelte | Remix | TypeScript |
| Tailwind CSS | | |

> The catalog is team-curated — adding a new technology or skill is a one-line addition to the static registry. All skills must pass the skills.sh security audit before being included.

## How It Works

DevRune follows a **three-stage pipeline**:

```
devrune.yaml  →  resolve  →  devrune.lock  →  install  →  workspace files
  (manifest)      (fetch)      (lockfile)     (render)    (.claude/, .agents/, .codex/, etc.)
```

1. **Resolve** — Reads `devrune.yaml`, fetches packages from their sources (GitHub, GitLab, local), computes content hashes, and writes `devrune.lock`. This is the only stage that touches the network.

2. **Install** — Reads `devrune.lock` and the manifest, then materializes workspace files for each configured agent. Each agent has a dedicated renderer that transforms the canonical format into the agent's native format.

3. **State tracking** — After install, DevRune writes `.devrune/state.yaml` to track what was installed, which agents are active, and the lockfile hash. Running `devrune status` compares the current lockfile against the installed state to detect drift.

4. **Tool install** (init only) — During `devrune init`, after resolving and installing packages, the wizard discovers developer tools from the catalog (e.g. Crit, Engram), filters them based on your selected MCPs and workflows, detects which are already installed, and offers to install the rest via Homebrew in parallel.

## CLI Reference

```
devrune — AI agent configuration manager

Usage:
  devrune [command]

Commands:
  init        Initialize, resolve, and install in one step (interactive TUI wizard)
  sync        Resolve packages and install workspace in one step
  resolve     Resolve packages and produce devrune.lock
  install     Materialize the workspace from devrune.lock
  status      Show workspace installation state
  cache       Manage cached data (packages, AI recommendations)
  version     Print version information

Global Flags:
  -v, --verbose           Enable verbose output
      --non-interactive   Disable interactive prompts (for CI/automation)
      --dir string        Working directory (default ".")
```

### `devrune init`

Full setup in one command. Launches an interactive TUI wizard that guides you through agent selection, package configuration, and developer tool installation — or accepts flags for CI:

```bash
# Interactive
devrune init

# Non-interactive
devrune init --non-interactive \
  --agents claude,opencode \
  --source github:owner/catalog@main \
  --mcp github:owner/catalog@main//mcps/engram.yaml \
  --force
```

| Flag | Description |
|------|-------------|
| `--agents` | Agent names to configure (e.g. `claude,codex,opencode,copilot,factory`) |
| `--source` | Package source refs (repeatable) |
| `--mcp` | MCP server source refs (repeatable) |
| `--workflow` | Workflow source refs (repeatable) |
| `--force` | Overwrite existing `devrune.yaml` without prompting |

### `devrune sync`

Resolve and install in one step — the recommended way to apply changes:

```bash
devrune sync
devrune sync --manifest custom-manifest.yaml
```

| Flag | Description |
|------|-------------|
| `--manifest` | Path to the manifest file (default `devrune.yaml`) |
| `--offline` | Fail if any package is not cached |

### `devrune resolve`

Fetches all packages and writes the lockfile:

```bash
devrune resolve
devrune resolve --manifest custom-manifest.yaml
devrune resolve --offline  # Fail if any package is not cached
```

### `devrune install`

Materializes workspace files from the lockfile:

```bash
devrune install
devrune install --locked   # Fail if devrune.lock doesn't exist
devrune install --offline  # No network access during install
```

### `devrune status`

Shows what's installed and whether the lockfile is in sync:

```bash
devrune status
```

### `devrune cache clean`

Removes cached data to force fresh fetches:

```bash
devrune cache clean                  # Clean all caches (packages + recommendations)
devrune cache clean --packages-only  # Only clean package cache
devrune cache clean --recommend-only # Only clean AI recommendation cache
```

| Flag | Description |
|------|-------------|
| `--packages-only` | Only clean the package download cache (`~/.cache/devrune/packages/`) |
| `--recommend-only` | Only clean the AI recommendation cache (`~/.cache/devrune/recommend/`) |

## Manifest Format

The `devrune.yaml` manifest declares everything DevRune needs:

```yaml
schemaVersion: devrune/v1

# ─── Packages ────────────────────────────────────────────────
# Skills and rules from one or more catalogs.
# Supports GitHub, GitLab (with custom host), and local sources.
packages:
  - source: github:owner/repo@ref                      # All skills + rules
  - source: github:owner/repo@ref//subpath              # Subdirectory within repo
    select:                                              # Pick specific items
      skills: [skill-a, skill-b]
      rules: [category/rule-name]
  - source: gitlab:owner/repo@ref?host=gitlab.example.com  # Self-hosted GitLab
  - source: local:./path/to/catalog                     # Local directory

# ─── MCP Servers ─────────────────────────────────────────────
# Tool integrations. Each MCP YAML can declare permissions: { level: allow }
# for automatic settings generation across all agents.
mcps:
  - source: github:owner/repo@ref//mcps/atlassian.yaml  # Jira + Confluence
  - source: github:owner/repo@ref//mcps/engram.yaml     # Persistent memory
  - source: github:owner/repo@ref//mcps/context7.yaml   # Library docs

# ─── Agents ──────────────────────────────────────────────────
# Which AI agents to configure. Each gets its own renderer.
agents:
  - name: claude    # .claude/skills/, CLAUDE.md, settings.json, .mcp.json
  - name: codex     # .agents/skills/, AGENTS.md, .codex/config.toml (TOML)
  - name: opencode  # .agents/skills/, AGENTS.md, opencode.json
  - name: copilot   # .github/, copilot-instructions.md, .vscode/settings.json
  - name: factory   # .agents/skills/, AGENTS.md, .factory/mcp.json

# ─── Workflows ───────────────────────────────────────────────
# Multi-phase development workflows with orchestrators and phase skills.
workflows:
  - github:owner/repo@ref//workflows/sdd               # Spec-Driven Development

# ─── Catalogs ────────────────────────────────────────────────
# Auto-derived from your sources by sync/init. Pre-loads these catalogs
# in the TUI wizard when re-running `devrune init`. Managed automatically.
catalogs:
  - github:owner/repo
  - gitlab:org/custom-catalog?host=gitlab.example.com

# ─── Install Preferences (optional) ─────────────────────────
install:
  linkMode: copy              # copy | symlink | hardlink
  rulesMode:
    claude: concat            # concat | individual | both
    opencode: individual
```

> **Note on tools:** Developer tools (e.g. Crit, Engram) are defined in the catalog's `tools/` directory, not in the manifest. During `devrune init`, the wizard automatically discovers tools from your selected catalog sources, checks which ones are relevant based on your selections (MCPs, workflows), and offers to install them via Homebrew. Tools are a side-effect of init — they don't appear in `devrune.yaml` or `devrune.lock`.

### Catalog Sources (`catalogs:` in `devrune.yaml`)

The `catalogs:` key in `devrune.yaml` declares which catalog source refs are pre-loaded when running `devrune init`. These sources seed Step 2 of the TUI wizard (source selection) so re-running init preserves your previously configured catalogs.

**Schema** (inline in `devrune.yaml`):

```yaml
schemaVersion: devrune/v1

agents:
  - name: claude

packages:
  - source: github:davidarce/devrune-starter-catalog@main

catalogs:
  - github:davidarce/devrune-starter-catalog
  - github:myorg/custom-catalog@v2
```

| Field | Required | Description |
|-------|----------|-------------|
| `catalogs` | No | List of catalog source ref strings (see [Source Ref Formats](#source-ref-formats)) |

**`--catalog` flag:**

Use `--catalog` to pass catalog source ref strings directly at the command line. The flag is repeatable and accepts the same source ref format used in the manifest:

```bash
devrune init --catalog github:davidarce/devrune-starter-catalog
devrune init --catalog github:org/catalog-a --catalog github:org/catalog-b
```

In non-interactive mode, catalog sources are merged with `--source` flag values to populate `packages:` in the manifest. The `catalogs:` key is also written so re-running init re-seeds the wizard.

**Init behavior with existing manifest:**

When `devrune init` finds an existing `devrune.yaml`, it reads the `catalogs:` key and merges those entries with any `--catalog` flag values. Duplicates are removed. CLI flag values take precedence (appear first in the merged list).

**Sync behavior:**

`devrune sync` automatically derives catalog roots from your `packages:`, `mcps:`, and `workflows:` sources and writes them to `catalogs:`. This keeps the field up to date without manual editing.

### Source Ref Formats

| Scheme | Format | Example |
|--------|--------|---------|
| GitHub | `github:owner/repo@ref[//subpath]` | `github:davidarce/devrune-starter-catalog@main` |
| GitLab | `gitlab:owner/repo@ref[//subpath][?host=...]` | `gitlab:team/catalog@v2?host=gitlab.example.com` |
| Local | `local:./relative/path` | `local:./my-catalog` |

The `@ref` is a git tag, branch, or SHA. The `//subpath` narrows to a subdirectory within the repo.

## Catalog Structure

A catalog (package source) follows this convention:

```
my-catalog/
├── skills/
│   ├── architect-adviser/
│   │   └── SKILL.md           # Markdown with YAML frontmatter
│   └── git-commit/
│       └── SKILL.md
├── rules/
│   ├── architecture/
│   │   └── clean-architecture-rules.md
│   └── testing/
│       └── mother-pattern-rules.md
├── mcps/
│   ├── engram.yaml            # MCP server definitions
│   └── context7.yaml
├── workflows/
│   └── sdd/
│       ├── workflow.yaml      # Workflow definition with roles
│       └── skills/
│           ├── sdd-explore/
│           │   └── SKILL.md
│           └── sdd-plan/
│               └── SKILL.md
└── tools/
    ├── crit.yaml              # Developer tool definitions
    └── engram.yaml            # Auto-installed via Homebrew during init
```

See [devrune-starter-catalog](https://github.com/davidarce/devrune-starter-catalog) for a complete example.

### Tool Definitions

Tools are developer CLI utilities that complement your agent setup. Each tool is a YAML file in the `tools/` directory:

```yaml
name: engram
description: "Persistent memory for AI agents"
command: "brew install gentleman-programming/tap/engram"
binary: engram
depends_on:
  mcp: engram        # Only shown if MCP "engram" is selected
  workflow: sdd      # ...OR workflow "sdd" is selected
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Display name shown in the wizard |
| `description` | Yes | Short description |
| `command` | Yes | Homebrew install command |
| `binary` | No | Binary name to detect if already installed (via `$PATH` lookup) |
| `depends_on.mcp` | No | Only offer this tool if the named MCP is selected |
| `depends_on.workflow` | No | Only offer this tool if the named workflow is selected |

During `devrune init`, the wizard filters tools based on your selections — if you didn't select the SDD workflow, you won't be prompted to install Crit. Already-installed tools (detected via `binary` field) are shown with a ✓ and skipped during installation.

## Platforms

| OS | Architecture | Status |
|----|-------------|--------|
| macOS | Apple Silicon (arm64) | Supported |
| macOS | Intel (amd64) | Supported |
| Linux | amd64 | Supported |
| Linux | arm64 | Not yet |
| Windows | — | Use WSL |

## Development

```bash
git clone https://github.com/davidarce/DevRune.git
cd DevRune

make setup    # Install git hooks (pre-push lint)
make check    # fmt + vet + lint + test (full validation)
make build    # Build optimized binary
make run      # Run via go run
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build optimized binary for current platform |
| `build-debug` | Build with debug symbols (for dlv) |
| `build-all` | Cross-compile for darwin-arm64, darwin-amd64, linux-amd64 |
| `install` | Build and install to `~/.local/bin` |
| `uninstall` | Remove binary from install directory |
| `test` | Run all tests with race detector and coverage |
| `vet` | Run `go vet` |
| `fmt` | Format source code |
| `lint` | Run golangci-lint (same config as CI) |
| `check` | Full pre-push validation: fmt + vet + lint + test |
| `setup` | Install git hooks for local development |
| `clean` | Remove build artifacts |

### Project Structure

```
DevRune/
├── cmd/devrune/           # CLI entrypoint
├── internal/
│   ├── cli/               # Cobra commands (init, sync, resolve, install, status, cache, version)
│   ├── cache/             # Package fetchers (GitHub, GitLab, local)
│   ├── detect/            # Project tech profile detection (go-enry, dependency parsing)
│   ├── recommend/         # AI recommendation engine (Claude/OpenCode integration, caching)
│   ├── resolve/           # Dependency resolver → devrune.lock
│   ├── materialize/       # Workspace materializer
│   │   └── renderers/     # Agent-specific renderers (Claude, Codex, Copilot, OpenCode, Factory)
│   ├── model/             # Domain types (manifest, lockfile, source refs, agents)
│   ├── parse/             # YAML/frontmatter parsing
│   ├── state/             # Installation state tracking (.devrune/state.yaml)
│   └── tui/               # Interactive TUI wizard (Bubble Tea + Huh)
├── agents/                # Built-in agent definitions (YAML)
├── testdata/              # Test fixtures
├── scripts/               # Git hooks
├── .goreleaser.yml        # Release automation
├── golangci-lint.mod      # Pinned linter version
└── Makefile               # Build, test, lint, install targets
```

## License

[MIT](LICENSE) — David Arce
