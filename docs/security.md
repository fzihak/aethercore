# AetherCore Security Model

## The Zero Trust Perimeter

In traditional agent frameworks, tools execute within the same memory space and user-privilege context as the orchestrator. A malicious LLM prompt resulting in arbitrary code execution yields immediate host compromise.

AetherCore implements a strict **Zero Trust** boundary between the Intelligence (LLM) and Execution (Tools).

### The Rust Sidecar

All third-party or autonomous code execution must occur within Layer 2—the Rust Sidecar.

- **Namespaces & Cgroups**: On Linux, the sidecar maps execution into transient namespaces, crippling host visibility and hard-capping CPU/RAM utilization.
- **Capability Manifests**: Tools must declare their intentions (e.g., `fs:read`, `net:outbound`) in a signed `manifest.toml`. If a tool requests a capability it did not declare, the Rust Sidecar terminates the process instantly via `SIGKILL`.

### mTLS Mesh Transport

All inter-node communication (Layer 3) is cryptographically secured via mTLS. A local Certificate Authority (CA) bootstraps ephemeral certificates per-node, verifying identity before a single byte of telemetry or state is synchronized.

### Privacy by Default

AetherCore operates 100% locally.

- Memory and Context vectors never touch cloud providers (unless self-hosted by the user).
- IPC between the Go Kernel and Rust Sidecar happens entirely over protected local Unix Domain Sockets (`.sock`).
