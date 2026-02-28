# AetherCore Runtime (Layer 2)

The `runtime` module is the **security and execution sandbox** of AetherCore, written purely in memory-safe Rust.

While the Go Kernel (Layer 0) is responsible for rapid event orchestration and LLM translation, it is categorically not allowed to execute untrusted code or touch the outer system unilaterally.

The Rust Runtime acts as the impenetrable shield.

## Architectural Responsibilities

1.  **Untrusted Capability Enforcement:**
    When an LLM attempts to execute a tool that interacts with the filesystem or network, the Go Kernel forwards the execution request to this Rust sandbox via secure gRPC over stdin/stdout.
2.  **Resource Limits:**
    The sandbox strictly enforces CPU cycle limits and RAM constraints on every execution to prevent LLM hallucination-induced DDoS attacks.
3.  **WASM Execution:**
    Future community plugins will be executed here inside isolated WebAssembly VMs (e.g., using `wasmtime`), guaranteeing that 3rd-party code cannot crash the core kernel or silently exfiltrate user data.

## Performance Mandate

The sandbox must boot from a cold start and become ready to receive gRPC capabilities in **< 10ms**, drawing minimal memory footprint.
