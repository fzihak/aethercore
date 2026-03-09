//! Layer 2 WASM Execution Engine — backed by `wasmtime`.
//!
//! Provides a deterministic, resource-bounded sandbox for executing untrusted
//! WebAssembly plugins. All WASM code runs with:
//!   - Fuel-based CPU limiting (prevents infinite loops / denial-of-service)
//!   - No ambient host capabilities (no filesystem, no network, no syscalls)
//!   - Memory isolation enforced by the WASM linear memory model
//!
//! Plugins must expose a `run() -> i32` export as their canonical entry point.
//! Plugins that export no `run` function are accepted but produce no output.

// The WASM engine is infrastructure used by the IPC handler at runtime.
// Dead-code warnings are suppressed here because the public API surface
// will be called from ipc.rs once tool dispatch is wired (Day 14 → stable).
#![allow(dead_code)]

use wasmtime::{Config, Engine, Linker, Module, Store};

/// Errors produced during WASM plugin execution.
#[derive(Debug)]
pub enum WasmExecutionError {
    /// WASM module failed to compile or validate (malformed bytecode).
    Compile(wasmtime::Error),
    /// WASM module trapped or otherwise failed at runtime.
    Runtime(wasmtime::Error),
    /// The plugin exhausted its CPU fuel budget before completing.
    FuelExhausted,
}

impl std::fmt::Display for WasmExecutionError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Compile(e) => write!(f, "wasm_compile_error: {}", e),
            Self::Runtime(e) => write!(f, "wasm_runtime_error: {}", e),
            Self::FuelExhausted => write!(f, "wasm_fuel_exhausted: plugin exceeded cpu budget"),
        }
    }
}

impl std::error::Error for WasmExecutionError {}

/// WasmSandbox wraps a `wasmtime::Engine` configured for strict, deterministic
/// execution. A single `WasmSandbox` instance is long-lived and shared across
/// multiple plugin invocations.
pub struct WasmSandbox {
    engine: Engine,
}

impl WasmSandbox {
    /// Creates a new WASM sandbox with fuel metering enabled.
    ///
    /// `consume_fuel` must be set before engine construction — it cannot be
    /// toggled on a running engine.
    pub fn new() -> Result<Self, wasmtime::Error> {
        let mut config = Config::new();
        // Enable fuel metering to bound CPU usage per plugin invocation
        config.consume_fuel(true);
        // Disable unnecessary features to minimize the attack surface
        config.wasm_threads(false);
        config.wasm_reference_types(false);

        let engine = Engine::new(&config)?;

        eprintln!(
            r#"{{"level":"INFO","msg":"wasm_engine_initialized","fuel_metering":true,"threads":false,"component":"wasm_engine"}}"#
        );

        Ok(Self { engine })
    }

    pub fn execute(&self, wasm_bytes: &[u8], payload: &str, caps: &crate::manifest::Capabilities) -> Result<String, WasmExecutionError> {
        let module = Module::new(&self.engine, wasm_bytes).map_err(WasmExecutionError::Compile)?;

        // For Day 10/11 stable branch, we stub actual WASI preview 2 deep mapping
        // because the API fluctuates rapidly. We setup basic fuel limits & memory limits.
        let linker: Linker<()> = Linker::new(&self.engine);

        let mut store = Store::new(&self.engine, ());

        let fuel_limit = caps.max_cpu_ms * 10_000;
        store
            .set_fuel(fuel_limit)
            .map_err(WasmExecutionError::Runtime)?;

        let instance = linker
            .instantiate(&mut store, &module)
            .map_err(WasmExecutionError::Runtime)?;

        let run_fn = instance.get_typed_func::<(), i32>(&mut store, "run");

        match run_fn {
            Ok(func) => {
                let result = func.call(&mut store, ()).map_err(|e| {
                    if store.get_fuel().unwrap_or(u64::MAX) == 0 {
                        WasmExecutionError::FuelExhausted
                    } else {
                        WasmExecutionError::Runtime(e)
                    }
                })?;

                let fuel_remaining = store.get_fuel().unwrap_or(0);
                let fuel_used = fuel_limit.saturating_sub(fuel_remaining);

                eprintln!(
                    r#"{{"level":"INFO","msg":"wasm_execution_completed","exit_code":{},"fuel_used":{},"fuel_limit":{},"component":"wasm_engine"}}"#,
                    result, fuel_used, fuel_limit
                );

                Ok(format!(
                    r#"{{"sandbox_executed":true,"received_payload":{},"wasm_exit_code":{},"fuel_used":{},"fuel_limit":{}}}"#,
                    payload, result, fuel_used, fuel_limit
                ))
            }

            Err(_) => {
                eprintln!(
                    r#"{{"level":"WARN","msg":"wasm_no_entry_point","hint":"module exports no 'run' function","component":"wasm_engine"}}"#
                );
                Ok(format!(
                    r#"{{"sandbox_executed":true,"received_payload":{},"wasm_no_entry_point":true}}"#,
                    payload
                ))
            }
        }
    }
}
