# AetherCore 30-Day V1.0 Launch Plan

_The Daily Execution Path to the "Minimal Agent Kernel"_

This document contains the exact day-by-day micro-tasks required to build and ship AetherCore V1.0, focusing heavily on Layer 0 (Go Kernel) and Layer 2 (Rust Sandbox).

## Week 1: Foundation & Telemetry (Days 1-7)

**Goal:** Prove the `<50ms` boot and `<10MB` RAM claims. Stabilize the CLI.

- **Day 1:** Write the CLI telemetry engine to measure cold-boots accurately via `time.Now()`. Run profiling across the worker pool to ensure zero goroutine leaks on exit.
- **Day 2:** Finalize the GitHub Actions benchmark script. We need a CI job that measures and records binary size and RAM allocations on every single commit.
- **Day 3:** Build out the `aether tool list` and `aether run --tool` commands to allow static, Go-native tools to be called and tested from the CLI.
- **Day 4:** Finalize the OpenTelemetry/JSON logging standard for all Layer 0 events.
- **Day 5:** Memory Optimization pass on the Event Loop dispatcher—replace all map allocations with zero-copy syncing.
- **Day 6:** Complete the core `LLMAdapter` interface functionality locally by mocking Ollama responses directly into the worker pool.
- **Day 7:** Code freeze for Layer 0 (The Sacred Kernel). Ensure 100% test coverage and zero external dependencies via strictly configured `golangci-lint`.

---

## Week 2: The Rust Shield (Days 8-14)

**Goal:** Establish the Layer 2 Sandbox boundary. Untrusted code starts here.

- **Day 8:** Initialize the `/runtime` directory with `cargo init`. Build the basic Rust skeleton capable of booting sub-10ms.
- **Day 9:** Design and implement the strict Capability Declaration manifest (`manifest.toml`). No process spans without declaring its filesystem/network intent.
- **Day 10:** Implement gRPC communication over Unix Domain Sockets (`.sock`) between the Go kernel and the Rust sidecar holding the capability proofs.
- **Day 11:** Implement Linux `cgroups` and lightweight `namespaces` directly in the Rust sidecar to enforce severe memory limits during tool execution.
- **Day 12:** Build the WebAssembly (WASM) execution engine inside the Rust sidecar utilizing `wasmtime`.
- **Day 13:** Write out the formal IPC (Inter-Process Communication) security specification document.
- **Day 14:** Refactor the Go kernel to dispatch _all_ unknown tool calls directly to the verified Rust Sandbox.

---

## Week 3: Distributed Mesh & State (Days 15-21)

**Goal:** Allow multiple AetherCore nodes to orchestrate tasks together.

- **Day 15:** Implement the mTLS (Mutual TLS) certificate generation logic inside the Layer 0 Kernel for identity anchoring.
- **Day 16:** Build the Layer 3 Mesh peer discovery over standard UDP broadcasts (local networking).
- **Day 17:** Write the Task Propagation protocol, allowing a local AetherCore node to securely pass an LLM context to an idle edge node via gRPC.
- **Day 18:** Build the Vector Memory storage layer (`/memory`). This should use a hyper-minimal local embedding DB (e.g., using pure Go for cosine similarity).
- **Day 19:** Extend the memory module to be queryable by any agent in the mTLS mesh network based on cryptographically signed permissions.
- **Day 20:** Implement strict timeouts and ephemeral destruction tracking for tasks spanning across the distributed mesh.
- **Day 21:** Build integration tests simulating a 5-node AetherCore mesh network executing a multi-part goal.

---

## Week 4: The Developer Ecosystem (Days 22-28)

**Goal:** The AetherCore Plugin SDK and platform gateways.

- **Day 22:** Write the public `aether-sdk` for Go and Rust. Allow developers to scaffold "Layer 1 Modules" effortlessly.
- **Day 23:** Build the native Telegram adapter (`/gateway/telegram`). Agents should instantly plug into chats.
- **Day 24:** Build the native Discord adapter (`/gateway/discord`).
- **Day 25:** Implement silent background task scheduling (Cron-like syntax native to AetherCore) for proactive agent behavior without a user prompt.
- **Day 26:** Hardening pass targeting Go concurrency races, Rust unsafe blocks, and network timeout edge-cases.
- **Day 27:** Write the `AETHERCORE_WHITEPAPER.md` mathematically proving the security characteristics and latency advantages over LangChain/PicoClaw.
- **Day 28:** Write the final `api_docs` and ensure the `website/` pre-launch metrics match our actual local benchmarks.

---

## The Launch (Days 29-30)

- **Day 29:** Final Release Candidate (`RC1`). Complete end-to-end sandbox QA on Mac, Windows, and Linux CI runners.
- **Day 30:** Tag `V1.0.0`. Push binaries to GitHub Releases via automated Actions. Go live with `aethercore.brainexia.com`.
