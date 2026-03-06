use std::time::Instant;

mod manifest;
mod ipc;
mod sandbox;
mod wasm_engine;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Capture initialization time before ANY allocations happen — before any allocation.
    let start = Instant::now();

    // 2. Apply namespace isolation BEFORE the async runtime spawns any threads.
    //    CLONE_NEWNS | CLONE_NEWUTS | CLONE_NEWIPC | CLONE_NEWNET
    if let Err(e) = sandbox::isolate_namespaces() {
        eprintln!(
            r#"{{"level":"WARN","msg":"namespace_isolation_unavailable","error":"{}","component":"sandbox"}}"#,
            e
        );
    }

    // 3. Validate the strict Capability Declaration Manifest.
    //    No tool may execute without being declared here.
    let manifest = match manifest::Manifest::load("manifest.toml") {
        Ok(m) => {
            if m.sandbox.strict_mode {
                eprintln!(
                    r#"{{"level":"INFO","msg":"strict_mode_active","component":"sandbox"}}"#
                );
            }
            m
        }
        Err(e) => {
            eprintln!(
                r#"{{"level":"ERROR","msg":"manifest_parse_failed","error":"{:?}","component":"sandbox"}}"#,
                e
            );
            std::process::exit(1);
        }
    };

    // 4. Initialize the WASM execution engine (validates wasmtime config).
    match wasm_engine::WasmSandbox::new() {
        Ok(_) => {
            eprintln!(
                r#"{{"level":"INFO","msg":"wasm_engine_ready","component":"sandbox"}}"#
            );
        }
        Err(e) => {
            eprintln!(
                r#"{{"level":"ERROR","msg":"wasm_engine_init_failed","error":"{}","component":"sandbox"}}"#,
                e
            );
            std::process::exit(1);
        }
    }

    // 5. Apply per-tool cgroup v2 memory limits from the manifest.
    //    Guards are kept alive for the full sandbox lifetime via RAII.
    let _cgroup_guards: Vec<_> = manifest
        .tools
        .iter()
        .filter_map(|tool| {
            let limit_bytes = tool.memory_limit_mb * 1024 * 1024;
            match sandbox::CgroupGuard::apply(&tool.name, limit_bytes) {
                Ok(guard) => Some(guard),
                Err(e) => {
                    eprintln!(
                        r#"{{"level":"WARN","msg":"cgroup_apply_failed","tool":"{}","error":"{}","component":"sandbox"}}"#,
                        tool.name, e
                    );
                    None
                }
            }
        })
        .collect();

    // 6. Measure and log the cold-boot latency.
    let duration = start.elapsed();
    println!(
        r#"{{"level":"INFO","msg":"sandbox_booted","boot_latency":"{:?}","component":"sandbox"}}"#,
        duration
    );

    // 7. Boot the async runtime and start the gRPC/UDS server.
    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()?
        .block_on(async {
            let socket_path = std::env::temp_dir().join("aether-sandbox.sock");
            ipc::start_uds_server(&socket_path).await
        })?;

    Ok(())
}
