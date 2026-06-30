## 2024-05-30 - Fix Unix Domain Socket Creation TOCTOU Race Condition
**Vulnerability:** The Unix Domain Socket for IPC was created using default process umask permissions, making it vulnerable to unauthorized local connections. A naive fix using `set_permissions` immediately after `bind()` creates a Time-Of-Check to Time-Of-Use (TOCTOU) race condition window where an attacker can connect.
**Learning:** In Rust (and POSIX generally), creating a Unix Domain Socket with specific permissions securely and atomically requires manipulating the process `umask` *before* calling `bind()`, as `bind()` applies the current umask to the newly created socket file.
**Prevention:** Always temporarily set `umask(0o177)` before binding a UDS, and restore it immediately afterward. Ensure the `bind()` call is synchronous (not `.await`) to prevent async task cancellation from leaving the umask permanently modified.

## 2024-06-03 - Fix Umask State Leak on Panic
**Vulnerability:** The process `umask` was temporarily altered to create a Unix Domain Socket with specific permissions but was restored manually without RAII, creating a risk that if `UnixListener::bind()` panics, the process permanently retains the restrictive `umask(0o177)`, affecting future file creation permissions for the entire application.
**Learning:** Manual global state manipulation in Rust is not panic-safe.
**Prevention:** Use an RAII guard (struct implementing `Drop`) to ensure the global state (like `umask`) is unconditionally restored when the scope ends, protecting against panics.

## 2024-06-30 - Fix IPC SO_PEERCRED Vulnerability
**Vulnerability:** The Unix Domain Socket connection between the Go kernel and the Rust sandbox failed to enforce identity verification using the `SO_PEERCRED` socket option. Any local user could connect to the IPC socket, bypass authentication, and submit tool execution requests to the sandbox, leading to local privilege escalation.
**Learning:** Enforcing connection authentication using socket properties (like `SO_PEERCRED`) on a Tonic gRPC server acting over UDS requires intervening before handing the listener to `serve_with_incoming()`.
**Prevention:** Filter the incoming `UnixListenerStream` via `.filter_map()` to extract peer credentials using `stream.peer_cred()` on every connection attempt. Only yield connections that match the expected user ID `libc::geteuid()` to prevent unauthorized local access.
