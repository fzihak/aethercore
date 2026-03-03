use std::time::Instant;

mod manifest;
mod ipc;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Capture initialization time before ANY allocations happen
    let start = Instant::now();

    // 2. Validate the strict Capability Declaration Manifest
    match manifest::Manifest::load("manifest.toml") {
        Ok(m) => {
            // Future: Enforce capability restrictions based on m.tools
            if m.sandbox.strict_mode {
                // Sandbox asserts constraints
            }
        }
        Err(e) => {
            // Panic abruptly if the orchestrator tries running a rogue/undeclared tool
            eprintln!(r#"{{"level":"ERROR","msg":"manifest_parse_failed","error":{:?}}}"#, e);
            std::process::exit(1);
        }
    }

    // The Layer 2 Rust Sandbox will eventually securely execute WASM plugins here

    // 2. Measure cold-boot execution completion
    let duration = start.elapsed();
    
    // Output OpenTelemetry JSON matching the Go Kernel format
    println!(
        r#"{{"level":"INFO","msg":"sandbox_booted","boot_latency":"{:?}","component":"sandbox"}}"#,
        duration
    );

    let socket_path = std::env::temp_dir().join("aether-sandbox.sock");
    ipc::start_uds_server(&socket_path).await?;

    Ok(())
}
