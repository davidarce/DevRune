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
  <strong>One manifest, every agent.</strong>
  <br>
  <em>A package manager for AI agent instructions — with <a href="#-spec-driven-development-the-first-citizen">Spec-Driven Development</a> built in.</em>
</p>

---

## Why DevRune?

Configuring AI development agents used to mean juggling `.claude/`, `.agents/`, `.codex/`, `.github/`, and `.factory/` by hand — copying skill files, deduplicating rules, wiring MCP servers, hoping every agent sees the same context.

**DevRune makes that a solved problem.** You declare what you want in a single `devrune.yaml`; DevRune resolves it, locks it, and materializes the right files into each agent's native format.

- 📦 **Package-manager model** — sources, selection, lockfile, cache, reproducibility.
- 🧠 **Spec-Driven Development (SDD) as first-citizen** — 4-phase workflow with model routing, adviser guidance, and compaction recovery. See [the SDD section](#-spec-driven-development-the-first-citizen).
- 🤖 **5 agents supported** — Claude Code, OpenCode, Codex, Copilot, Factory. One source of truth, five native workspaces.
- 🪄 **Interactive TUI** — a guided wizard for first-time setup, project-type detection, and model selection.
- 🔄 **Safe updates** — `devrune sync` re-resolves, re-installs, and tracks drift via a state file.

---

## Table of Contents

- [Quick Start](#-quick-start)
- [Supported Agents](#-supported-agents)
- [Spec-Driven Development (the first citizen)](#-spec-driven-development-the-first-citizen)
- [The Starter Catalog](#-the-starter-catalog)
- [CLI Reference](#-cli-reference)
- [Manifest Format](#-manifest-format)
- [How It Works](#-how-it-works)
- [Installation](#-installation)
- [Development](#-development)
- [Community & Support](#-community--support)
- [License](#-license)

---

## 🚀 Quick Start

### 1. Install DevRune

```bash
curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | bash
```

Other methods: [Installation](#-installation).

### 2. Launch the wizard

```bash
devrune init
```

The TUI walks you through **agents → SDD overview → content → model selection → confirm**. It detects your project tech stack and pre-selects the relevant advisers and rules.

### 3. Or start from a manifest

Drop a `devrune.yaml` in your repo root:

```yaml
schemaVersion: devrune/v1

agents: [claude, opencode]

packages:
  - source: github:davidarce/devrune-starter-catalog@main
    select:
      skills: [git-commit, architect-adviser, unit-test-adviser]
      rules: [architecture/clean-architecture, testing/mother-pattern]

mcps:
  - source: github:davidarce/devrune-starter-catalog@main//mcps/engram.yaml

workflows:
  - source: github:davidarce/devrune-starter-catalog@main//workflows/sdd
```

Then:

```bash
devrune sync
```

Your agents pick up the new skills, rules, MCPs, and the SDD workflow on the next turn.

---

## 🤖 Supported Agents

DevRune configures all of these from a single manifest — each agent receives files in its **native format**, not a lowest-common-denominator abstraction.

| Agent               | Workspace            | Catalog File              | MCP Config        | Notes                                                          |
|---------------------|----------------------|---------------------------|-------------------|----------------------------------------------------------------|
| **Claude Code**     | `.claude/`           | `CLAUDE.md`               | `.mcp.json`       | Canonical format. Skill frontmatter is native. Hooks supported. |
| **OpenCode**        | `.opencode/`         | `.agents/AGENTS.md`       | `opencode.json`   | Shares `.agents/skills/` with Codex & Factory. Model routing.   |
| **Codex (OpenAI)**  | `.codex/`            | `.agents/AGENTS.md`       | `config.toml`     | Shares `.agents/skills/`. TOML MCP config.                     |
| **GitHub Copilot**  | `.github/`           | `copilot-instructions.md` | `.mcp.json`       | Generates `.agent.md` wrappers for `@agent-name` invocation.   |
| **Factory Droid**   | `.factory/`          | `.agents/AGENTS.md`       | `mcp.json`        | Shares `.agents/skills/`. Factory-native permissions format.   |

Adding a new agent is a matter of writing an `agents/{name}.yaml` definition and a renderer in `internal/materialize/renderers/`. See [Development](#-development).

---

## 🧠 Spec-Driven Development (the first citizen)

> **SDD is how DevRune thinks about building software.** It's not a template you opt into; it's the flagship workflow installed and orchestrated natively.

Instead of yolo-coding, SDD breaks a change into **four disciplined phases** — each with its own sub-agent, model, and artifacts — and hands orchestration to a dedicated coordinator. You keep control; the agents stay on the rails.

### The 4 phases

```
┌─────────────┐     ┌──────────┐     ┌───────────────┐     ┌─────────────┐
│ ① Explore   │ ──▶ │ ② Plan   │ ──▶ │ ③ Implement   │ ──▶ │ ④ Review    │
│             │     │          │     │   (waves)     │     │             │
│ exploration │     │ plan.md  │     │ code changes  │     │ review.md   │
│    .md      │     │          │     │ [X] markers   │     │             │
└─────────────┘     └──────────┘     └───────────────┘     └─────────────┘
   Sonnet             Opus             Sonnet                 Opus
```

| Phase          | What it does                                                                                     | Output                          |
|----------------|--------------------------------------------------------------------------------------------------|---------------------------------|
| **① Explore**  | Scans the codebase, curates the files that matter, surfaces ambiguities.                         | `.sdd/{change}/exploration.md`  |
| **② Plan**     | Deep interview → machine-parsable task plan with batch table, dependencies, and quality gates.    | `.sdd/{change}/plan.md`         |
| **③ Implement**| Executes tasks in parallel/sequential waves, fails fast, updates `[X]` markers per batch.         | Code + updated `plan.md`        |
| **④ Review**   | Diffs against the plan, flags regressions, decides commit vs. fix.                                | `.sdd/{change}/review.md`       |

### The Advisor Strategy (guidance loop)

During Plan, the planner (Sonnet) identifies when a domain specialist is needed and returns a **`guidance_requested`** envelope. The orchestrator launches adviser sub-agents (Opus) in parallel:

```
sdd-plan (Sonnet) ─┐
                   ├─ detects gap → guidance_requested
                   ▼
Orchestrator ──── launches advisers (Opus) in parallel
                   │       architect · api-first · unit-test · component · a11y ...
                   ▼
Orchestrator ──── re-enters sdd-plan with adviser recommendations
                   │
                   ▼
sdd-plan ──── status: ok → crit review → implement
```

Each adviser returns **Strengths / Issues / Recommendations** and persists full guidance to engram (when available). **Advisers never execute code** — they only advise.

### Compaction recovery

Long SDD sessions survive context compaction. DevRune installs per-agent hooks that preserve `.sdd/{change}/state.yaml` and re-inject a CRITICAL recovery prompt on session restart:

| Agent             | Tier | Mechanism                                                          |
|-------------------|:----:|--------------------------------------------------------------------|
| Claude Code       | 1a   | `PreCompact` + `SessionStart(compact)` hooks in `settings.json`    |
| Factory           | 1b   | `PreCompact` hook in `settings.json`                               |
| OpenCode          | 2    | TypeScript plugin auto-loaded from `.opencode/plugins/`             |
| Codex / Copilot   | 3    | Catalog-only recovery via `REGISTRY.md` instructions               |

### Model routing per phase

DevRune's TUI offers a **SDD Phase Models** step where you pick a model for each role (Explore, Plan, Implement, Review, Adviser) per agent. Pick Opus where depth matters; Sonnet where speed does. Reconfigurable anytime via `devrune` main menu → `Configure Models`.

**GitHub Copilot tier constraint** — Copilot sub-agents cannot use a model with a higher cost tier than the orchestrator (VS Code enforces this). The TUI enforces this at selection time: pick the orchestrator model first and the phase cards automatically filter to only show models within that tier. The orchestrator model is written to the `.agent.md` frontmatter so VS Code picks it up directly.

### Where the SDD logic lives

**DevRune** owns the resolver, renderers, materializer, state tracking, hooks injection, and TUI flow.
**[devrune-starter-catalog](https://github.com/davidarce/devrune-starter-catalog)** owns the SDD skills, orchestrator, templates, hooks, and plugin — the workflow's *content*.

> 📖 Deep dive: the starter catalog's [`workflows/sdd/`](https://github.com/davidarce/devrune-starter-catalog/tree/main/workflows/sdd) directory has `ORCHESTRATOR.md`, the four phase skills, and shared contracts (envelope, launch-templates, adviser-templates, persistence, recovery).

---

## 📚 The Starter Catalog

DevRune works with **any catalog** that follows the directory contract (`skills/`, `rules/`, `mcps/`, `workflows/`, `tools/`). The **[devrune-starter-catalog](https://github.com/davidarce/devrune-starter-catalog)** is the curated default — it ships the SDD workflow plus:

- **11 skills** — 7 advisers (architect, api-first, unit-test, integration-test, component, frontend-test, a11y), `git-commit`, `git-pull-request`, `review-pr`, `arch-flow-explorer`.
- **11 rules** — architecture (clean-arch, Beck's 4 rules), API standards, Java/Spring, React, testing patterns, microfrontends, a11y.
- **6 MCPs** — Atlassian, Context7, Engram, Exa, Playwright, Ref.
- **2 developer tools** — Crit (plan review), Engram (persistent memory) — auto-installed via Homebrew when the matching workflow/MCP is selected.

Fork it, trim it, extend it — it's MIT. Point DevRune at your own `github:org/my-catalog` and the same machinery works.

---

## 🧰 CLI Reference

| Command                | What it does                                                                                         |
|------------------------|------------------------------------------------------------------------------------------------------|
| `devrune`              | Launches the interactive menu (Setup · Sync · Status · Configure Models · Upgrade · Uninstall).     |
| `devrune init`         | Guided TUI wizard: agents → SDD overview → content → models → confirm → install.                     |
| `devrune sync`         | One-shot `resolve` + `install`. Use this after editing `devrune.yaml`.                               |
| `devrune resolve`      | Fetches packages, computes content hashes, writes `devrune.lock`. Network-only stage.                |
| `devrune install`      | Reads `devrune.lock` and materializes workspace files. No network.                                   |
| `devrune status`       | Shows installed state and whether the workspace is in sync with the lockfile.                        |
| `devrune cache clean`  | Drops cached package data. Flags: `--packages-only`, `--recommend-only`.                             |
| `devrune upgrade`      | Self-upgrades the DevRune binary to the latest release.                                              |
| `devrune uninstall`    | Removes materialized skills, rules, MCPs, and workflows from the workspace.                          |
| `devrune version`      | Prints version, commit, and build date.                                                              |

Run `devrune <command> --help` for flags.

---

## 📝 Manifest Format

A minimal, idiomatic `devrune.yaml`:

```yaml
schemaVersion: devrune/v1

agents: [claude, opencode]               # which agents to configure

packages:
  - source: github:davidarce/devrune-starter-catalog@main
    select:
      skills: [git-commit, architect-adviser]
      rules:  [architecture/clean-architecture]

mcps:
  - source: github:davidarce/devrune-starter-catalog@main//mcps/engram.yaml

workflows:
  - source: github:davidarce/devrune-starter-catalog@main//workflows/sdd
    models:                              # optional — per-role overrides
      claude:
        sdd-planner: opus
        sdd-implementer: sonnet
        sdd-adviser:   opus

install:
  linkMode: copy                         # copy | symlink | hardlink
  rulesMode: individual                  # per-agent: concat | individual | both
  autoRecommend: true                    # AI-powered recommendations during init
```

### Source refs

| Form                                       | Example                                                                  |
|--------------------------------------------|--------------------------------------------------------------------------|
| `github:owner/repo@ref[//subpath]`         | `github:davidarce/devrune-starter-catalog@main//workflows/sdd`           |
| `gitlab:owner/repo@ref[//subpath][?host=]` | `gitlab:team/catalog@v2?host=gitlab.example.com`                         |
| `local:./path`                             | `local:./my-catalog`                                                     |

### Generated artifacts

| File                   | Purpose                                                                           |
|------------------------|-----------------------------------------------------------------------------------|
| `devrune.yaml`         | Your source of truth (hand-edited).                                               |
| `devrune.lock`         | Resolved state: content hashes, parsed manifests, workflow metadata.              |
| `.devrune/state.yaml`  | What was materialized. Used for drift detection and clean uninstall.              |

---

## 🛠️ How It Works

DevRune runs as a three-stage pipeline:

```
devrune.yaml  ──► [Resolve]  ──► devrune.lock  ──► [Install]  ──► workspace
                      ▲              (hashes)          ▼           (.claude/,
                      │                                │            .agents/,
                  ~/.cache/devrune/                    │            .github/...)
                   (content-addressed)                 │
                                                       ▼
                                              .devrune/state.yaml
                                               (drift detection)
```

1. **Resolve** — fetches every `source:` (GitHub/GitLab/local), computes SHA-256 hashes, stores archives in a content-addressed cache (`~/.cache/devrune/`), and writes `devrune.lock`.
2. **Install** — reads the lock, runs agent-specific **renderers** that know how each agent's files must look (Claude keeps native frontmatter; OpenCode transforms it; Copilot generates `.agent.md` wrappers; Codex emits TOML).
3. **State** — writes `.devrune/state.yaml` listing every materialized path + the lock hash. `devrune status` compares this to detect drift; `devrune uninstall` uses it to remove exactly what was installed.

Workflows (like SDD) get special treatment: DevRune resolves the `workflow.yaml`, installs the phase skills, renders the orchestrator per agent, deep-merges hook JSON into settings, appends `.gitignore` entries, and registers the workflow in each agent's catalog file.

---

## 📥 Installation

### Curl one-liner

```bash
curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | bash
```

Pin a version:

```bash
VERSION=v0.1.27 curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | bash
```

The script detects your platform (`darwin-arm64`, `darwin-amd64`, `linux-amd64`), downloads the release tarball, and installs to `~/.local/bin` (override with `INSTALL_DIR`). It warns if another `devrune` is shadowing it on `PATH`.

### go install

```bash
go install github.com/davidarce/devrune/cmd/devrune@latest
```

### From source

```bash
git clone https://github.com/davidarce/DevRune.git
cd DevRune
make install          # builds and installs to ~/.local/bin
```

### Supported platforms

| OS        | arch    | Status                         |
|-----------|---------|--------------------------------|
| macOS     | arm64   | ✅ Supported                    |
| macOS     | amd64   | ✅ Supported                    |
| Linux     | amd64   | ✅ Supported                    |
| Linux     | arm64   | 🧪 Build from source            |
| Windows   | —       | 🧪 Use WSL2                    |

---

## 👩‍💻 Development

```bash
git clone https://github.com/davidarce/DevRune.git
cd DevRune

make build     # builds ./devrune
make test      # runs the test suite
make lint      # golangci-lint
make fmt       # go fmt
make install   # builds + installs to ~/.local/bin
```

### Project layout

```
DevRune/
├── cmd/devrune/              # CLI entry point
├── internal/
│   ├── cli/                  # cobra subcommands
│   ├── tui/                  # Bubble Tea + Huh wizard
│   ├── resolve/              # fetch + hash + lockfile
│   ├── materialize/          # rendering + workspace writes
│   │   └── renderers/        # per-agent transforms
│   ├── state/                # .devrune/state.yaml
│   ├── model/                # manifest, workflow, agent schemas
│   └── …
├── agents/                   # agent definitions (claude.yaml, …)
├── testdata/                 # fixtures & golden files
├── scripts/                  # maintenance scripts
└── install.sh                # curl-one-liner installer
```

### Adding an agent

1. Drop an `agents/{name}.yaml` describing workspace dirs, catalog file, MCP format, link-mode constraints, and any special transforms.
2. Add a renderer in `internal/materialize/renderers/{name}.go` implementing the `Renderer` interface.
3. Add fixture tests under `testdata/`.
4. Open a PR — see [CONTRIBUTING.md](CONTRIBUTING.md).

### Adding a workflow, skill, rule, or MCP

These live in **catalogs**, not in DevRune. See the [devrune-starter-catalog README](https://github.com/davidarce/devrune-starter-catalog) for the format, or fork it to start your own.

---

## 🤝 Community & Support

- 🐛 **Bugs & feature requests** — [open an issue](https://github.com/davidarce/DevRune/issues).
- 🔐 **Security disclosures** — see [SECURITY.md](SECURITY.md). Please don't open public issues for vulnerabilities.
- 🧭 **Contributing** — [CONTRIBUTING.md](CONTRIBUTING.md) covers the PR workflow.
- 📜 **Code of Conduct** — [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) (Contributor Covenant).

DevRune is pre-1.0 — the surface is still evolving. Breaking changes are documented in the [CHANGELOG](CHANGELOG.md).

---

## 📄 License

MIT — see [LICENSE](LICENSE).

Copyright (c) 2026 David Arce.
