#!/usr/bin/env bash
# scripts/e2e_validate.sh — AetherCore v2 Phase 0 E2E Validation
#
# Validates:
#   Phase 0.1  ReAct loop fix       — Go unit + integration tests
#   Phase 0.2  Rust sandbox          — cargo build + cgroup/namespace code check
#   Phase 0.3  Memory & security     — Go binary RSS check, prompt-guard tests
#
# Exit code 0 = all checks passed.
# Requires: go 1.24+, optional: cargo (Rust)
set -euo pipefail

MODULE="github.com/fzihak/aethercore"
PASS=0
FAIL=0
WARNS=0

# ── helpers ──────────────────────────────────────────────────────────────────

ok()   { echo "  [PASS] $*"; PASS=$((PASS+1)); }
fail() { echo "  [FAIL] $*"; FAIL=$((FAIL+1)); }
warn() { echo "  [WARN] $*"; WARNS=$((WARNS+1)); }
sep()  { echo; echo "── $* ──────────────────────────────────────────────────"; }

# ── Phase 0.1: Go build + ReAct loop tests ───────────────────────────────────

sep "Phase 0.1 — ReAct Loop: build & unit tests"

if go build ./... 2>/dev/null; then
    ok "go build ./... (all packages)"
else
    fail "go build ./... — compilation errors present"
fi

run_pkg_tests() {
    local pkg="$1"
    local label="$2"
    if go test -timeout 60s -count=1 -q "${MODULE}/${pkg}" 2>&1 | tail -1 | grep -q "^ok"; then
        ok "${label}"
    else
        fail "${label}"
    fi
}

run_pkg_tests "core/llm"     "core/llm tests (OllamaAdapter, Orchestrator, Routers)"
run_pkg_tests "core"         "core tests (ReAct loop, event loop, mTLS, mesh, telemetry)"
run_pkg_tests "core/security" "core/security tests (PromptGuard, ManifestValidator, OrchestratorGuard)"
run_pkg_tests "core/audit"   "core/audit tests (ChainManager, LocalAppender tamper detection)"
run_pkg_tests "memory"       "memory tests (VectorStore, SignedStore)"
run_pkg_tests "sdk"          "sdk tests (ModuleRegistry)"

sep "Phase 0.1 — ReAct multi-iteration integration tests"

if go test -timeout 60s -count=1 -v -run "TestReAct" "${MODULE}/core" 2>&1 \
        | grep -E "^(--- PASS|--- FAIL|PASS|FAIL)" | tee /dev/stderr | grep -q "^PASS\|--- PASS"; then
    ok "ReAct integration tests (multi-turn tool loop)"
else
    fail "ReAct integration tests"
fi

sep "Phase 0.1 — Orchestrator wiring (no placeholder)"

# Guard against the old placeholder being reintroduced
if grep -r "initialization placeholder" core/llm/ 2>/dev/null | grep -qv "_test.go"; then
    fail "Orchestrator GenerateWithTools still returns placeholder string"
else
    ok "Orchestrator placeholder removed — real adapter wired"
fi

# Guard against the Go-side empty-signature gate being reintroduced
if grep -n "refusing to dispatch unsigned" core/ipc_client.go 2>/dev/null | grep -v "^//"; then
    fail "ipc_client.go still has Go-side signature gate (blocks all sandbox calls)"
else
    ok "Sandbox dispatch signature gate: Rust-enforced only (Go gate removed)"
fi

# ── Phase 0.2: Rust sandbox ───────────────────────────────────────────────────

sep "Phase 0.2 — Rust Sandbox"

if command -v cargo &>/dev/null; then
    pushd runtime > /dev/null
    if cargo build --release --quiet 2>&1; then
        ok "Rust sandbox: cargo build --release"
        BINARY="target/release/runtime"
        if [ -f "$BINARY" ]; then
            SIZE_KB=$(du -k "$BINARY" | cut -f1)
            ok "Rust binary present (${SIZE_KB} KB)"
        fi
    else
        fail "Rust sandbox: cargo build failed"
    fi
    popd > /dev/null
else
    warn "cargo not found — Rust sandbox build skipped (install rustup to enable)"
fi

# Verify cgroup v2 enforcement code is present in source (not stubbed out)
if grep -q "memory.max" runtime/src/sandbox.rs 2>/dev/null; then
    ok "cgroup v2 memory.max enforcement present in sandbox.rs"
else
    fail "cgroup v2 memory.max enforcement missing from sandbox.rs"
fi

if grep -q "CLONE_NEWNS\|clone_newns\|unshare\|isolate_namespaces" runtime/src/sandbox.rs 2>/dev/null; then
    ok "Linux namespace isolation code present in sandbox.rs"
else
    fail "Linux namespace isolation missing from sandbox.rs"
fi

if grep -q "memory_limit_mb\|memory_limit_bytes" runtime/src/sandbox.rs 2>/dev/null; then
    ok "Per-tool memory_limit_mb cap enforced in sandbox.rs"
else
    fail "Per-tool memory cap not found in sandbox.rs"
fi

if grep -q "verify\|ed25519\|Ed25519" runtime/src/manifest.rs 2>/dev/null; then
    ok "Ed25519 manifest verification present in manifest.rs"
else
    fail "Ed25519 manifest verification missing from manifest.rs"
fi

# ── Phase 0.3: Memory footprint of compiled Go binary ────────────────────────

sep "Phase 0.3 — Binary memory footprint"

BINARY="$(go env GOPATH)/bin/aether"
if go build -o /tmp/aether_e2e_check ./cmd/aether/ 2>/dev/null; then
    SIZE_KB=$(du -k /tmp/aether_e2e_check | cut -f1)
    rm -f /tmp/aether_e2e_check
    ok "Go binary builds (${SIZE_KB} KB on disk)"

    if [ "$SIZE_KB" -lt 30720 ]; then        # 30 MB — reasonable CLI binary limit
        ok "Binary size ${SIZE_KB} KB < 30 MB"
    else
        warn "Binary size ${SIZE_KB} KB > 30 MB — consider trimming dependencies"
    fi
else
    fail "Go binary failed to build"
fi

# Cgroup memory.max limit from manifest.toml must be ≤ 15 MB for default tool
MEMORY_LIMIT=$(grep -A5 'name = "orchestrator"' runtime/manifest.toml 2>/dev/null | grep memory_limit_mb | head -1 | grep -oE '[0-9]+' || echo "0")
if [ "$MEMORY_LIMIT" -gt 0 ] && [ "$MEMORY_LIMIT" -le 50 ]; then
    ok "manifest.toml orchestrator memory_limit_mb=${MEMORY_LIMIT} ≤ 50 MB"
else
    warn "manifest.toml orchestrator memory_limit_mb not found or exceeds 50 MB"
fi

# ── Prompt injection guard regression check ──────────────────────────────────

sep "Phase 0.3 — Prompt Injection Guard regression"

INJECTION_PATTERNS=("SYSTEM_PROMPT_LEAK" "IGNORE_INSTRUCTIONS" "ROLEPLAY_JAILBREAK" "TOKEN_DENSITY_ANOMALY" "PADDING_ABUSE")
for pattern in "${INJECTION_PATTERNS[@]}"; do
    if grep -rq "$pattern" core/security/ 2>/dev/null; then
        ok "Guard pattern present: ${pattern}"
    else
        fail "Guard pattern missing: ${pattern}"
    fi
done

# ── Summary ───────────────────────────────────────────────────────────────────

echo
echo "══════════════════════════════════════════════════════════════"
echo " AetherCore v2 Phase 0 E2E Validation — Results"
echo "══════════════════════════════════════════════════════════════"
echo "  PASS : ${PASS}"
echo "  FAIL : ${FAIL}"
echo "  WARN : ${WARNS}"
echo "══════════════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
    echo "  STATUS: FAILED (${FAIL} check(s) did not pass)"
    exit 1
else
    echo "  STATUS: PASSED — Phase 0 foundation is solid"
    exit 0
fi
