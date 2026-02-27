# Security Policy

## Supported Versions

| Version      | Supported |
| ------------ | --------- |
| latest       | ✅        |
| < latest - 1 | ❌        |

## Reporting a Vulnerability

**DO NOT open a public GitHub issue.**

Email: security@aethercore.dev
PGP key: [coming soon]

Include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

**Response time:** 48 hours acknowledgement, 7 days triage.
**Disclosure policy:** 90 days from report to public disclosure.
**Credit:** We will credit you in the security advisory unless you prefer anonymity.

## Security Design

AetherCore is built security-first:

- Zero Trust tool execution — every tool declares capabilities
- Rust sandbox — untrusted tools never run in the kernel
- Signed plugins — Ed25519 signatures on all plugins
- Encrypted local storage — SQLite encrypted at rest
- Reproducible builds — checksums published per release

See [docs/security.md](docs/security.md) for full security model.
