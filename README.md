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

### Interactive (TUI wizard)

```bash
devrune init
```

The wizard walks you through selecting agents and package sources, then resolves and installs everything automatically. If the catalog includes developer tools, the wizard will also offer to install them via Homebrew.

### Manual

**1. Create `devrune.yaml`:**

```yaml
schemaVersion: devrune/v1

packages:
  - source: github:davidarce/devrune-starter-catalog@main
    select:
      skills:
        - architect-adviser
        - git-commit
        - unit-test-adviser
      rules:
        - architecture/clean-architecture-rules
        - tech/react-rules
        - testing/mother-pattern-rules

mcps:
  - source: github:davidarce/devrune-starter-catalog@main//mcps/engram.yaml
  - source: github:davidarce/devrune-starter-catalog@main//mcps/context7.yaml

agents:
  - name: claude
  - name: codex
  - name: opencode
  - name: copilot

workflows:
  - github:davidarce/devrune-starter-catalog@main//workflows/sdd
```

**2. Sync (resolve + install):**

```bash
devrune sync   # Fetch packages, update lockfile, materialize workspace
```

That's it. Your workspace now has correctly formatted skills, rules, MCP configs, and workflows for Claude, Codex, OpenCode, and Copilot. If the catalog includes developer tools (e.g. Crit, Engram), `devrune init` will also offer to install them via Homebrew.

> You can also run `devrune resolve` and `devrune install` separately for advanced workflows (CI/CD, offline installs).

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

## Manifest Format

The `devrune.yaml` manifest declares everything DevRune needs:

```yaml
schemaVersion: devrune/v1

# Packages contain skills and rules
packages:
  - source: github:owner/repo@ref
  - source: github:owner/repo@ref//subpath
    select:                          # Optional: pick specific items
      skills: [skill-a, skill-b]
      rules: [category/rule-name]
  - source: gitlab:owner/repo@ref?host=gitlab.example.com
  - source: local:./path/to/catalog

# MCP server definitions
mcps:
  - source: github:owner/repo@ref//mcps/server.yaml

# Target agents
agents:
  - name: claude
  - name: opencode
  - name: copilot
  - name: codex
  - name: factory

# Workflows (e.g. SDD — Spec-Driven Development)
workflows:
  - github:owner/repo@ref//workflows/sdd

# Optional install preferences
install:
  linkMode: copy          # copy | symlink | hardlink
  rulesMode:
    claude: concat        # concat | individual | both
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

`devrune sync` reads `catalogs:` from the manifest but does not use it — catalog sources are init-only metadata. The `packages:` list is the operative field for sync and install.

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
│   ├── cli/               # Cobra commands (init, sync, resolve, install, status, version)
│   ├── cache/             # Package fetchers (GitHub, GitLab, local)
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
