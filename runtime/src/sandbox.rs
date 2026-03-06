//! Layer 2 Sandbox enforcement: Linux cgroups v2 + namespace isolation.
//!
//! This module applies OS-level resource controls before any untrusted tool
//! executes inside the Rust sidecar. On non-Linux platforms (macOS, Windows),
//! enforcement is replaced with no-op stubs so the binary still compiles for
//! development, while the actual security boundaries only activate on Linux.

// ─── Linux enforcement ────────────────────────────────────────────────────────

#[cfg(target_os = "linux")]
pub use linux_impl::*;

#[cfg(target_os = "linux")]
mod linux_impl {
    use std::fs;
    use std::io::Write;
    use std::path::{Path, PathBuf};

    /// Root path for AetherCore-specific cgroup v2 hierarchy.
    const CGROUP_ROOT: &str = "/sys/fs/cgroup/aether";

    /// CgroupGuard creates a memory-bounded cgroup for a single tool execution
    /// and automatically releases it when dropped (RAII).
    ///
    /// Uses cgroup v2 unified hierarchy. Requires write access to `/sys/fs/cgroup/`.
    pub struct CgroupGuard {
        path: PathBuf,
    }

    impl CgroupGuard {
        /// Apply creates an isolated cgroup for `tool_name` and enforces a hard
        /// memory limit of `memory_limit_bytes`. The current process is added to
        /// the cgroup so all downstream spawned processes inherit the limit.
        pub fn apply(tool_name: &str, memory_limit_bytes: u64) -> Result<Self, std::io::Error> {
            let path = Path::new(CGROUP_ROOT).join(tool_name);
            fs::create_dir_all(&path)?;

            // Enforce hard memory limit via cgroup v2 memory.max
            let memory_max_path = path.join("memory.max");
            let mut f = fs::File::create(&memory_max_path)?;
            write!(f, "{}", memory_limit_bytes)?;

            // Disable swap to prevent processes from bypassing the memory cap
            let swap_max_path = path.join("memory.swap.max");
            if let Ok(mut sf) = fs::File::create(&swap_max_path) {
                let _ = write!(sf, "0");
            }

            // Add current process to the cgroup — all child processes will inherit
            let cgroup_procs_path = path.join("cgroup.procs");
            let mut pf = fs::File::create(&cgroup_procs_path)?;
            write!(pf, "{}", std::process::id())?;

            eprintln!(
                r#"{{"level":"INFO","msg":"cgroup_applied","tool":"{}","memory_limit_bytes":{},"component":"sandbox"}}"#,
                tool_name, memory_limit_bytes
            );

            Ok(Self { path })
        }
    }

    impl Drop for CgroupGuard {
        fn drop(&mut self) {
            // cgroup must have zero member processes before it can be removed.
            // Workers exiting their scope naturally empties it.
            if let Err(e) = fs::remove_dir(&self.path) {
                eprintln!(
                    r#"{{"level":"WARN","msg":"cgroup_release_failed","path":"{:?}","error":"{}","component":"sandbox"}}"#,
                    self.path, e
                );
            } else {
                eprintln!(
                    r#"{{"level":"INFO","msg":"cgroup_released","path":"{:?}","component":"sandbox"}}"#,
                    self.path
                );
            }
        }
    }

    /// Apply lightweight namespace isolation to the current process via `unshare(2)`.
    ///
    /// Isolates the following namespaces from the host:
    /// - `CLONE_NEWNS`  — mount namespace (prevents host filesystem access)
    /// - `CLONE_NEWUTS` — UTS/hostname namespace (prevents hostname spoofing)
    /// - `CLONE_NEWIPC` — IPC namespace (isolates shared memory and semaphores)
    /// - `CLONE_NEWNET` — network namespace (drops all host network interfaces)
    ///
    /// # Safety
    /// `unshare(2)` must be called **before** any threads are spawned. Tokio's
    /// multi-threaded runtime is initialized after this call to meet that requirement.
    ///
    /// Requires `CAP_SYS_ADMIN` on the calling process. If the capability is
    /// absent (e.g., running unprivileged in CI), this function returns an error
    /// which should be logged as a warning rather than treated as fatal.
    pub fn isolate_namespaces() -> Result<(), std::io::Error> {
        // SAFETY: unshare(2) is async-signal-safe and correct to call on a
        // single-threaded process. We verify single-threaded state by placement
        // in main() before the #[tokio::main] multi-thread scheduler starts.
        let flags = libc::CLONE_NEWNS | libc::CLONE_NEWUTS | libc::CLONE_NEWIPC | libc::CLONE_NEWNET;
        let ret = unsafe { libc::unshare(flags) };

        if ret != 0 {
            return Err(std::io::Error::last_os_error());
        }

        eprintln!(
            r#"{{"level":"INFO","msg":"namespaces_isolated","namespaces":["mount","uts","ipc","net"],"component":"sandbox"}}"#
        );

        Ok(())
    }
}

// ─── Non-Linux stub implementations ──────────────────────────────────────────

#[cfg(not(target_os = "linux"))]
pub use stub_impl::*;

#[cfg(not(target_os = "linux"))]
mod stub_impl {
    /// No-op guard on non-Linux platforms. Compiles cleanly for development.
    pub struct CgroupGuard;

    impl CgroupGuard {
        pub fn apply(tool_name: &str, memory_limit_bytes: u64) -> Result<Self, std::io::Error> {
            eprintln!(
                r#"{{"level":"WARN","msg":"cgroup_enforcement_skipped","tool":"{}","limit_bytes":{},"reason":"not_linux","component":"sandbox"}}"#,
                tool_name, memory_limit_bytes
            );
            Ok(Self)
        }
    }

    pub fn isolate_namespaces() -> Result<(), std::io::Error> {
        eprintln!(
            r#"{{"level":"WARN","msg":"namespace_isolation_skipped","reason":"not_linux","component":"sandbox"}}"#
        );
        Ok(())
    }
}
