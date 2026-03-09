# AetherCore Benchmarks

Performance is treated as a security feature in AetherCore. Slow systems enable Denial-of-Service vectors by exhausting connection pools and memory limits.

Below are the benchmark metrics continuously verified by our GitHub Actions pipeline (`benchmark.sh`).

## Core Latency

_Tested on Linux AMD64 GitHub Actions Runner (2 vCPU)_

| Operation                    | LangChain (Python) | AetherCore (Go) | Measurement Bound       |
| ---------------------------- | ------------------ | --------------- | ----------------------- |
| Cold Boot (CLI to Ready)     | ~3,500ms           | **<10ms**       | Binary exec to `main()` |
| Subsystem Init + Log Config  | N/A                | **<5ms**        | Slog + OS Signaling     |
| IPC Ping/Pong (Rust Sidecar) | N/A                | **<1ms**        | Local Domain Socket     |

## Memory Footprint (Alloc/op)

AetherCore restricts garbage collector churn during heavy task loads by pooling memory for event evaluation.

| Benchmark Suite          | Allocations per Op (`allocs/op`) | Memory per Op (`B/op`) |
| ------------------------ | -------------------------------- | ---------------------- |
| `BenchmarkTaskDispatch`  | **0 allocs/op**                  | `0 B/op`               |
| `BenchmarkJSONLogOutput` | **0 allocs/op**                  | `0 B/op`               |
| `BenchmarkCronMatcher`   | **0 allocs/op**                  | `0 B/op`               |

> _Note: By hitting mathematically 0 allocs/op during the core loop dispatch, the inner Event Loop does not pressure the Go Garbage Collector, delivering deterministic latency percentiles._

## Binary Footprint

The AetherCore application is distributed as a single statically-linked binary, heavily pruned of standard framework bloat.

- **Target Size:** `<10 MB`
- **Stripped:** Yes (`-s -w`)

To replicate these benchmarks on your local machine:

```bash
make benchmark
```
