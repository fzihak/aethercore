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

    /// Execute runs a WASM plugin binary with strict resource limits.
    ///
    /// # Arguments
    /// - `wasm_bytes`: Raw compiled `.wasm` bytecode
    /// - `fuel_limit`: Maximum CPU instructions before `FuelExhausted` is returned
    ///
    /// # Returns
    /// A JSON string describing the execution outcome, suitable for forwarding
    /// back to the Go kernel via the IPC `ToolResponse.output_json` field.
    pub fn execute(&self, wasm_bytes: &[u8], fuel_limit: u64) -> Result<String, WasmExecutionError> {
        // 1. Compile and validate the WASM module — rejects malformed bytecode
        let module = Module::new(&self.engine, wasm_bytes).map_err(WasmExecutionError::Compile)?;

        // 2. Build a minimal linker with zero host imports — no ambient capabilities
        let linker: Linker<()> = Linker::new(&self.engine);

        // 3. Create an isolated store (linear memory + fuel counter per invocation)
        let mut store = Store::new(&self.engine, ());

        // 4. Load the fuel budget for this execution
        store
            .set_fuel(fuel_limit)
            .map_err(WasmExecutionError::Runtime)?;

        // 5. Instantiate the module against the empty host linker
        let instance = linker
            .instantiate(&mut store, &module)
            .map_err(WasmExecutionError::Runtime)?;

        // 6. Execute the canonical `run` entry point
        let run_fn = instance.get_typed_func::<(), i32>(&mut store, "run");

        match run_fn {
            Ok(func) => {
                let result = func.call(&mut store, ()).map_err(|e| {
                    // Distinguish fuel exhaustion from generic traps
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
                    r#"{{"wasm_exit_code":{},"fuel_used":{},"fuel_limit":{}}}"#,
                    result, fuel_used, fuel_limit
                ))
            }

            Err(_) => {
                // Module exports no `run` function — valid for library/data modules
                eprintln!(
                    r#"{{"level":"WARN","msg":"wasm_no_entry_point","hint":"module exports no 'run' function","component":"wasm_engine"}}"#
                );
                Ok(r#"{"wasm_no_entry_point":true}"#.to_string())
            }
        }
    }
}
