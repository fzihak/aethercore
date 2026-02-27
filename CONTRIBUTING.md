# Contributing to AetherCore

First off — thank you. AetherCore is open source because of people like you.

## Before You Start

Read the [Architecture Whitepaper](docs/architecture.md) first.
Understanding the layer system is non-negotiable before contributing.

## The Sacred Rule

**Layer 0 (`/core`) is sacred.**
No external dependencies. No global state. No framework.
Go stdlib only. PRs violating this will not be merged — no exceptions.

## How to Contribute

### Found a bug?

Open an issue using the Bug Report template.
Include: OS, Go version, binary version, steps to reproduce.

### Want to add a feature?

Open a Discussion first — not an issue, not a PR.
Discuss the idea, get consensus, then build.
This saves everyone's time.

### Good First Issues

Look for the `good-first-issue` label.
These are deliberately chosen to be approachable without deep kernel knowledge.

## Development Setup

```bash
git clone https://github.com/yourusername/aethercore
cd aethercore
make setup      # install dev tools
make build      # build all targets
make test       # run test suite
make bench      # run benchmarks
make lint       # run linter
```

## Commit Convention

We use Conventional Commits:

```
feat(core): add worker pool with bounded goroutines
fix(gateway): handle Telegram webhook timeout
docs(readme): add benchmark table
perf(core): reduce allocations in task dispatcher
security(runtime): enforce memory limit via cgroup
```

## Pull Request Rules

- PRs to `/core` require 2 approvals (one from maintainer)
- All PRs must pass CI (build + test + lint + benchmark)
- Include benchmark comparison for any Layer 0 changes
- Update CHANGELOG.md
- Keep PRs focused — one thing per PR

## Layer Guidelines

| Layer            | Directory  | Language | Dependency Policy |
| ---------------- | ---------- | -------- | ----------------- |
| 0 — Kernel       | `/core`    | Go       | stdlib ONLY       |
| 1 — Modules      | `/modules` | Go       | curated, minimal  |
| 2 — Rust Runtime | `/runtime` | Rust     | safe crates only  |
| 3 — Mesh         | `/kernel`  | Go       | minimal           |

## Code of Conduct

Be kind. Be direct. Disagree on ideas, not people.
See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
