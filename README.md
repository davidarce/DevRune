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
  <strong>AI Agent Configuration Toolkit</strong>
  <br>
  <em>One manifest, every agent. Skills, rules, MCPs, and workflows — resolved, locked, installed.</em>
</p>

---

DevRune configures AI development agents by resolving, fetching, and materializing packages of **skills**, **rules**, **MCP server definitions**, and **workflows** into your workspace. Write one `devrune.yaml` manifest and DevRune generates the correct files for every agent you use.

Think of it as **npm for AI agent instructions** — you declare what you need, DevRune handles the rest.

## Supported Agents

| Agent | Workspace | Format |
|-------|-----------|--------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `.claude/` | Skills as markdown, `CLAUDE.md` catalog, `settings.json`, `.mcp.json` |
| [OpenCode](https://opencode.ai) | `.opencode/` | Skills with YAML frontmatter, `AGENTS.md` catalog, `config.toml` MCP |
| [GitHub Copilot](https://github.com/features/copilot) | `.github/` | Skills + native agents (`.agent.md`), `copilot-instructions.md`, `.mcp.json` |
| [Windsurf (Factory)](https://windsurf.com) | `.factory/` | Skills with camelCase frontmatter, `AGENTS.md` catalog, `mcp.json` |

Each renderer transforms the canonical format (Claude-style markdown with YAML frontmatter) into the agent's native format — handling model name mapping, tool aliases, frontmatter conversion, and MCP config generation automatically.

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

The wizard walks you through selecting agents and package sources, then resolves and installs everything automatically.

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
  - name: opencode
  - name: copilot

workflows:
  - github:davidarce/devrune-starter-catalog@main//workflows/sdd
```

**2. Resolve and install:**

```bash
devrune resolve   # Fetch packages, compute hashes → devrune.lock
devrune install   # Materialize workspace files for all agents
```

That's it. Your workspace now has correctly formatted skills, rules, MCP configs, and workflows for Claude, OpenCode, and Copilot.

## How It Works

DevRune follows a **three-stage pipeline**:

```
devrune.yaml  →  resolve  →  devrune.lock  →  install  →  workspace files
  (manifest)      (fetch)      (lockfile)     (render)    (.claude/, .opencode/, etc.)
```

1. **Resolve** — Reads `devrune.yaml`, fetches packages from their sources (GitHub, GitLab, local), computes content hashes, and writes `devrune.lock`. This is the only stage that touches the network.

2. **Install** — Reads `devrune.lock` and the manifest, then materializes workspace files for each configured agent. Each agent has a dedicated renderer that transforms the canonical format into the agent's native format.

3. **State tracking** — After install, DevRune writes `.devrune/state.yaml` to track what was installed, which agents are active, and the lockfile hash. Running `devrune status` compares the current lockfile against the installed state to detect drift.

## CLI Reference

```
devrune — AI agent configuration manager

Usage:
  devrune [command]

Commands:
  init        Initialize, resolve, and install in one step (interactive TUI wizard)
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

Full setup in one command. Launches an interactive TUI wizard or accepts flags for CI:

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
| `--agents` | Agent names to configure (e.g. `claude,opencode,copilot,factory`) |
| `--source` | Package source refs (repeatable) |
| `--mcp` | MCP server source refs (repeatable) |
| `--workflow` | Workflow source refs (repeatable) |
| `--force` | Overwrite existing `devrune.yaml` without prompting |

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
└── workflows/
    └── sdd/
        ├── workflow.yaml      # Workflow definition with roles
        └── skills/
            ├── sdd-explore/
            │   └── SKILL.md
            └── sdd-plan/
                └── SKILL.md
```

See [devrune-starter-catalog](https://github.com/davidarce/devrune-starter-catalog) for a complete example.

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
│   ├── cli/               # Cobra commands (init, resolve, install, status, version)
│   ├── cache/             # Package fetchers (GitHub, GitLab, local)
│   ├── resolve/           # Dependency resolver → devrune.lock
│   ├── materialize/       # Workspace materializer
│   │   └── renderers/     # Agent-specific renderers (Claude, Copilot, OpenCode, Factory)
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
