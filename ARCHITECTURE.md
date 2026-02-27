# AetherCore Master Architecture & Strategy Plan

"The Minimal Agent Kernel for a Distributed AI Future."

## Golden Principle

**Minimal Core. Infinite Extension.**

## Executive Summary

AetherCore is a next-generation, ultra-lightweight AI agent kernel engineered for a distributed, security-first future. It solves the fundamental flaw of today's agent frameworks: they are architecturally bloated, trust tools implicitly, and conflate the runtime kernel with business logic.

AetherCore is not another agent library. It is a kernel — a sacred, minimal, high-performance execution core that can run in a sub-10MB binary on an edge device or scale horizontally across a secure mesh network using the same binary and a single flag.

### Why Most AI Agents Are Architecturally Wrong

Current agent frameworks couple the LLM orchestration layer directly with tool execution, memory, and networking. This creates bloat, unpredictable performance, and unverifiable security. AetherCore enforces a strict layered separation: the kernel stays sacred, capabilities are loaded on demand, and every tool execution is sandboxed and cryptographically verified.

## Architecture

### Layered System Design (Non-Negotiable)

AetherCore enforces a four-layer architecture. Layers are not suggestions — they are hard boundaries enforced at build time and at runtime.

| Layer | Name                | Language      | Responsibility                                                                           | Target                              |
| ----- | ------------------- | ------------- | ---------------------------------------------------------------------------------------- | ----------------------------------- |
| 0     | Kernel (Sacred)     | Go            | Event loop, LLM adapter, tool interface, task dispatcher, config loader                  | <15MB RAM / <10MB bin / <50ms start |
| 1     | Capability Modules  | Go            | Scheduler, local memory, metrics, logging, distributed mode (feature-flag loaded)        | On-demand                           |
| 2     | Rust Secure Runtime | Rust          | Sandbox, WASM plugin runtime, shell isolation, resource limiting, capability enforcement | sidecar via gRPC/Unix socket        |
| 3     | Mesh/Orchestration  | Go + Protobuf | Peer discovery, mTLS mesh, distributed task routing, consensus primitives                | Optional / `--kernel`               |

#### Layer 0 — The Sacred Kernel

**Inviolable Rule**: No PR may add a dependency to Layer 0 that is not Go stdlib. No framework. No global state. No reflection-heavy code. Violations are reverted without discussion.

The kernel is written in pure Go with a stdlib-first mandate. It runs an event-driven worker pool with bounded goroutines, preallocated buffers, and minimal JSON allocations. The LLM adapter is an interface — any provider (OpenAI, Anthropic, local Ollama) plugs in without touching the kernel.

Performance targets are hard constraints, not aspirations:

- RAM at rest: < 15 MB
- Binary size: < 10 MB
- Cold start: < 50 ms
- 1,000 concurrent lightweight tasks without degradation
- Stable execution under 24-hour stress test

#### Layer 1 — Capability Modules

Capability modules are optional extensions loaded via compile-time feature flags. The default build excludes all of them, keeping the binary minimal. This is the extension point — not the kernel.

- `aether-lite` — Kernel only, API orchestration, basic tools
- `aether-net` — Adds networking, HTTP tool, webhook listener
- `aether-sandbox` — Activates Rust sidecar for untrusted tool execution
- `aether-distributed` — Full mesh mode, peer discovery, distributed task routing

#### Layer 2 — Rust Secure Runtime (Sidecar)

Unsafe tools never execute inside the Go kernel. They are delegated to a Rust sidecar process communicating over an encrypted gRPC channel on a Unix socket. Rust was chosen for its memory safety guarantees, zero-cost abstractions, and predictable performance profile.
The sidecar provides: WASM plugin runtime, shell isolation, CPU/memory/timeout enforcement, capability checks, and signed request validation. The kernel stays clean; the sidecar takes the blast radius of any misbehaving tool.

#### Layer 3 — Mesh Orchestration (Optional)

Activated with `--kernel`, the mesh layer adds distributed execution without touching the kernel binary. Each node is stateless — state lives in an external store. Peer communication uses mutual TLS, and task routing uses a consistent hashing strategy to minimize cross-node latency. This layer is the rare engineering achievement in the current ecosystem: true horizontal scale with a binary under 20MB.

### Dual Runtime Modes

#### Mode A — Pico Mode (Default)

Pico Mode is the hacker-grade single-node runtime. It starts in milliseconds, orchestrates LLM calls, executes native Go tools, and maintains ephemeral per-task memory. No WASM. No mesh. No daemon processes. This directly competes with and outperforms PicoClaw on startup time, binary size, and security posture.

#### Mode B — Kernel Mode (`--kernel`)

The same binary. A single flag. Kernel Mode activates: the Rust sandbox sidecar, WASM plugin execution, multi-agent messaging bus, capability enforcement, and distributed mesh networking. No separate install. No different binary. This is the differentiator that does not exist in the current ecosystem.

**Architectural Insight**: Same Binary, Two Personalities. The engineering discipline required to achieve this is non-trivial. The core must be designed from day one to accept the extension points without the kernel knowing about them. Feature detection at startup, not at compile time for the binary user. This is how you ship a tool that both the solo hacker and the enterprise infrastructure team can use.

## Security Model

### Zero Trust Tool Architecture

Every tool must declare its capabilities before execution. The kernel and sandbox enforce these declarations — a tool that declares no network access cannot make network calls, regardless of what its code attempts. This is capability-based security, not permission-based security.

**Tool Manifest Declaration**:

```yaml
name: http_request
network: true
filesystem: false
max_runtime: 3s
memory_limit: 20mb
```

**Kernel Enforcement**: CPU time enforcement, Memory hard limit (cgroup or Rust enforced), Execution timeout (SIGKILL on breach), Capability checks before any syscall, Sandbox delegation for unsafe tools.

### Cryptographic Integrity

Every artifact that enters or configures the system is cryptographically verified:

- Signed plugins — Ed25519 signatures on all plugin binaries and WASM modules
- Signed config files — HMAC-SHA256 on all config changes
- Binary checksum validation — reproducible builds with published checksums
- Secure update channel — signed manifest with version pinning
- SBOM generation — every build outputs a signed Software Bill of Materials

### Isolation Strategy

Blast Radius Minimization. Trusted Go-native tools execute in-process for zero overhead. Untrusted, network-capable, or filesystem-touching tools are delegated to the Rust sandbox. A compromised tool cannot escape its declared capability envelope.

## Ephemeral Agent Model

One of AetherCore's rarest innovations: agents are not long-lived daemons. Every task spawns a fresh runtime, loads scoped memory, executes, saves output, and self-destructs. This is not a workaround — it is an architectural principle.

The benefits compound over time. Long-lived agents accumulate memory leaks, stale state, and unpredictable performance degradation. Ephemeral agents give you:

- Zero memory leak accumulation
- Zero state corruption
- Predictable performance
- Trivial horizontal scaling
- Simpler failure recovery

## Memory Architecture

Memory is tiered and isolated per agent instance.

- **Tier 0 — Ephemeral**: In-memory map, scoped to task lifetime, zero persistence overhead
- **Tier 1 — Persistent**: SQLite embedded, single-file, no external dependency
- **Tier 2 — Semantic (Optional)**: Vector extension module, pluggable backend (SQLite-vec, pgvector, Qdrant)

**Engineering Discipline on Vector DBs**: Vector databases are NOT in the core. They are not in Month 1. They are not in Month 2. They are a capability module loaded only when the workload requires semantic search.

## Competitive Differentiation

| Feature                            | AetherCore | PicoClaw | LangChain/CrewAI |
| ---------------------------------- | ---------- | -------- | ---------------- |
| Ultra-lightweight binary           | YES        | YES      | NO               |
| Dual Runtime Mode (Pico + Kernel)  | YES        | NO       | NO               |
| Rust Secure Sandbox                | YES        | NO       | NO               |
| Capability-Based Build Targets     | YES        | NO       | NO               |
| Ephemeral Agent Execution          | YES        | NO       | NO               |
| Zero Trust Tool Declaration        | YES        | NO       | NO               |
| Distributed Mesh Networking        | YES        | LIMITED  | NO               |
| Signed Plugins + SBOM              | YES        | NO       | NO               |
| WASM Plugin Runtime                | YES        | NO       | LIMITED          |
| Cryptographic Config Integrity     | YES        | NO       | NO               |
| Layer 3 Mesh Orchestration         | YES        | NO       | NO               |
| Reproducible Builds + Fuzz Testing | YES        | NO       | LIMITED          |

## Development Timeline

Five months. Focused grind. No feature creep. No UI dashboard. No premature vector DB.

| Phase   | Theme          | Deliverables                                                                                                                |
| ------- | -------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Month 1 | Foundation     | Go kernel, LLM adapter (OpenAI/Anthropic/local), basic tool interface, CLI, CI/CD pipeline, first benchmarks                |
| Month 2 | Security Layer | Rust sandbox sidecar, capability declaration system, ephemeral task execution, secure gRPC IPC, tool signature verification |
| Month 3 | Distribution   | Distributed mesh mode, WASM plugin runtime, peer discovery + mTLS, architecture whitepaper, security audit                  |
| Month 4 | Ecosystem      | Plugin SDK + marketplace foundation, vector memory extension, advanced CLI tooling, DevOps hardening                        |
| Month 5 | Launch         | Public release, benchmark blog post, demo video, community outreach, comparison report vs PicoClaw + LangChain              |

**Non-Negotiable Timeline Rules**: No feature enters Month N's milestone if it is not on the plan. If a great idea arrives in Month 2, it goes to the backlog for Month 4+. The kernel stays sacred.

## Engineering & DevOps Stack

### CI/CD Pipeline

- GitHub Actions — cross-compile matrix (linux/amd64, linux/arm64, darwin/amd64, windows/amd64)
- Fuzz testing — go-fuzz on all input parsing paths in the kernel
- Static analysis — staticcheck, gosec, golangci-lint on every PR
- Dependency vulnerability scan — govulncheck + Nancy on every merge
- Reproducible builds — BUILD_DATE stripped; checksums published per tag
- SBOM generation — SPDX format, signed with project key, published with each release

### Release Hygiene

- Semantic versioning — strict, with a public changelog
- Security advisories — GitHub Security Advisories for any CVE
- Bug bounty — post-launch, covering kernel and sandbox layers
- Security badge — visible in README, linked to audit summary

## Open Source Strategy

### Repository Structure

| Directory     | Contents                                                                      |
| ------------- | ----------------------------------------------------------------------------- |
| `/core`       | Layer 0 kernel — sacred, stdlib-only, event loop, LLM adapter, tool interface |
| `/kernel`     | Layer 3 mesh orchestration, peer discovery, distributed routing               |
| `/runtime`    | Rust sidecar source — sandbox, WASM runtime, capability enforcement           |
| `/modules`    | Layer 1 capability modules — scheduler, memory persistence, metrics           |
| `/plugins`    | Official plugin examples and plugin manifest spec                             |
| `/sdk`        | Go SDK for building custom tools and plugins                                  |
| `/examples`   | Real-world use cases — log summarizer, code reviewer, data pipeline           |
| `/benchmarks` | Public benchmark suite — reproducible, automated, CI-integrated               |
| `/docs`       | Architecture whitepaper, security design doc, API reference                   |

**License**: Apache 2.0.

## Personal Assistant UX — Beating PicoClaw at Its Own Game

PicoClaw's real moat is its UX simplicity. AetherCore must match this simplicity in Pico Mode and then exceed it.

### Principle: Invisible Power

Maximum power, minimum ceremony. Every feature below is designed to feel effortless while being architecturally superior underneath.

### Channel Support

AetherCore targets 10+ channels at launch including Telegram, Discord, WhatsApp, iMessage/SMS, Slack, Email, Web Chat Widget, CLI, QQ/DingTalk/LINE.

### Voice

Provider-agnostic universal transcription (Groq, OpenAI, Whisper.cpp). Text-to-speech responses. Wake-word support on always-on edge devices.

### Proactive Intelligence

Event watchers, daily briefings, smart reminders, anomaly detection, context-aware follow-ups.

### Autonomous Workflow Engine

Multi-step task planning, email automation, calendar integration, web automation, code workflows, chained tool calls.

### Onboard UX

Under 60 seconds for Pico Mode, under 5 minutes for Kernel Mode. Single binary, `aether onboard` wizard, use case profiles, QR code pairing.

### Multi-User & Shared Assistants

Per-user memory namespace, admin controls, team mode in Kernel Mode.

## Risk Register & Mitigations

| Risk                              | Severity | Mitigation                                                                             |
| --------------------------------- | -------- | -------------------------------------------------------------------------------------- |
| Scope creep into core layer       | HIGH     | Enforce architectural governance — no PRs touching Layer 0 without 2-engineer review   |
| LLM provider API instability      | MED      | Abstraction layer for LLM adapters; support local inference (Ollama) as fallback       |
| Rust/Go IPC latency overhead      | MED      | Benchmark early; fallback to in-process for trusted tools if latency exceeds threshold |
| Community adoption slow           | MED      | Launch with polished docs + real benchmarks; seed with demo use cases                  |
| Security vulnerability in sandbox | HIGH     | Third-party security audit before public release; bug bounty program post-launch       |

## Final Assessment

This is not a weekend project. This is serious engineering. If the kernel stays sacred, isolation is enforced, feature bloat is rejected, and distribution is designed carefully — AetherCore will be faster to start, more secure to run, more scalable to deploy, and more innovative in architecture than anything in the current AI agent ecosystem.

**Minimal Core. Infinite Extension.**
