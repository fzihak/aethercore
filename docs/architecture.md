# AetherCore Architecture Whitepaper

## The "Layer 0" Kernel Paradigm

Most agent frameworks (LangChain, AutoGen, PicoClaw) operate as bloated application layers. They intertwine LLM orchestration, memory management, and tool execution into a single, highly-coupled runtime.

AetherCore behaves as a **Kernel**, adhering to a strict 4-layer architecture:

### Layer 0: The Sacred Kernel (Go)

The core event loop, task dispatcher, and telemetry stream.

- **Binary Size**: <10MB.
- **Memory Footprint**: <15MB at rest.
- **Latency**: <50ms cold boot.
- **Rule**: Absolutely no external network calls (other than IPC) or third-party dependencies allowed in Layer 0.

### Layer 1: Native Modules (Go)

Capability extensions (e.g., Scheduler, Web Search) compiled directly into the binary. Managed by the kernel’s Module Registry via `Manifests` and lifecycle hooks (`OnStart`, `OnStop`, `HandleTask`).

### Layer 2: The Rust Sandbox (Rust + WASM)

Untrusted tool execution boundary. The Go Kernel communicates with the Rust Sidecar via gRPC over Unix Domain Sockets (`.sock`). The Rust sidecar implements strict resource quotas (cgroups/namespaces) and enforces Capability Manifests.

### Layer 3: Distributed Mesh

Optional, `--kernel` flagged mode that utilizes mTLS and standard UDP broadcast to form a decentralized swarm of AetherCore nodes for distributed task orchestration.

## Deterministic Zero-Allocation Dispatch

The AetherCore dispatcher leverages a mathematical 0-allocation pattern under heavy concurrency using `sync.Pool`. Memory profiles show practically flat allocation graphs during peak load intervals, enabling embedding on edge devices.

## The IPC Bridge

Communication across the Layer 0 (Go) and Layer 2 (Rust) boundary relies on a strictly typed, versioned IPC protocol. Sub-millisecond messaging ensures that security isolation does not cripple execution latency.
