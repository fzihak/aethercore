## 2024-05-30 - Fix Unix Domain Socket Creation TOCTOU Race Condition
**Vulnerability:** The Unix Domain Socket for IPC was created using default process umask permissions, making it vulnerable to unauthorized local connections. A naive fix using `set_permissions` immediately after `bind()` creates a Time-Of-Check to Time-Of-Use (TOCTOU) race condition window where an attacker can connect.
**Learning:** In Rust (and POSIX generally), creating a Unix Domain Socket with specific permissions securely and atomically requires manipulating the process `umask` *before* calling `bind()`, as `bind()` applies the current umask to the newly created socket file.
**Prevention:** Always temporarily set `umask(0o177)` before binding a UDS, and restore it immediately afterward. Ensure the `bind()` call is synchronous (not `.await`) to prevent async task cancellation from leaving the umask permanently modified.

## 2026-06-02 - Fix Insecure HTTP Server Configuration
**Vulnerability:** HTTP server without proper read/write timeouts set, which can lead to resource exhaustion if an attacker keeps connections open indefinitely without completing them.
**Learning:** In Go, the `http.Server` struct needs explicit `ReadTimeout` and `WriteTimeout` (in addition to `ReadHeaderTimeout`) to protect against unclosed connection attacks.
**Prevention:** Always initialize `http.Server` with explicit `ReadTimeout` and `WriteTimeout` values appropriate for the server's expected workload.
