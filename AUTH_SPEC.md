# AetherCore Authentication System

## Technical Specification

**Free Tier · Self-Hosted Runtime · Cloud Auth Only**

### Revenue Model

**Free. No paid tiers.**

---

## 01. The Core Principle

AetherCore is self-hosted. The runtime, all conversations, all memory, all user data — everything lives on the user's own machine. The user's privacy is absolute.
The only exception is authentication. A single, minimal auth server lives in the AetherCore cloud. Its sole job: verify who you are, issue a JWT token, and record basic analytics. Nothing else. Ever.

**Why Cloud Auth at All?**
Without any cloud component, there is no way to know if AetherCore is being used — by anyone, anywhere. A single auth server solves this cleanly: you get the growth data you need (DAU, MAU, countries, versions) without touching a single byte of user content. This is the minimum viable telemetry for running a project responsibly.

## 02. What Lives Where

### USER'S MACHINE (Self-Hosted)

- ✓ AetherCore runtime (Go binary)
- ✓ Dashboard (localhost:9091)
- ✓ Admin panel (localhost:9090)
- ✓ All conversations
- ✓ All task history
- ✓ Memory & preferences
- ✓ SQLite database
- ✓ LLM API calls
- ✓ Channel messages (Telegram etc.)
- ✓ JWT token (stored locally)

### AETHERCORE CLOUD (Auth Only)

- ✓ User email address
- ✓ User display name
- ✓ Hashed password (bcrypt)
- ✓ Account created timestamp
- ✓ Last login timestamp
- ✓ Country (detected from IP)
- ✓ AetherCore version string
- ✓ Login method (email/Google/GitHub)
- ✗ Zero conversation data
- ✗ Zero task or memory data

## 03. Complete Auth Flow

### First-Time Setup (Happens Once)

1. **User runs `aether onboard`** -> CLI detects no token, opens browser.
2. **Browser opens -> aethercore.dev/signup** -> Email+Password, Google, or GitHub.
3. **User creates account** -> Cloud saves minimal analytics data.
4. **Auth cloud issues JWT token** -> Signed with RS256, 30 days valid.
5. **JWT saved to user's machine** -> `~/.aether/token` (chmod 600).
6. **AetherCore is ready** -> 100% local from here.

### Daily Use (Zero Cloud Calls)

- **Every Request**: AetherCore verifies JWT locally using bundled public key.
- **Valid Token**: Proceed locally.
- **Expired Token**: Silent background refresh once per 30 days to `auth.aethercore.dev/refresh`.

## 04. JWT Token Design

### Token Payload

```json
{
  "sub": "user_01HXYZ...",
  "email": "user@example.com",
  "iat": 1740000000,
  "exp": 1742592000,
  "ver": "0.1.0",
  "iss": "auth.aethercore.dev"
}
```

### Verification

- Offline, using bundled public key from `auth.aethercore.dev/.well-known/jwks.json`.

### Storage

- Linux/macOS: `~/.aether/token`
- Windows: `%APPDATA%\aether\token`
- Permissions: `chmod 600`

## 05. Auth Provider — Clerk.dev

Clerk.dev handles the auth server reliably and is GDPR compliant. Provides 10k MAU free tier, standard login methods, and analytics.

## 06. Your Analytics — What You Can See

Total users, DAU/MAU, new signups, country distribution, login method breakdown, version distribution, churn rate. No content or local data is collected.

## 07. User Registration & Login

Covered in First-Time Setup. Upon re-login (`aether login`), a new JWT is issued. Silent refresh occurs recursively within 7 days of expiry.

## 08. Security Rules

- Passwords stored as bcrypt.
- JWT signed with RS256.
- Token file `chmod 600`.
- HTTPS only, rate limiting.
- GDPR data deletion.

## 09. Privacy README Section

"AetherCore is self-hosted. Your conversations, memory, tasks, and data never leave your machine..." (See full text for the README).

## 10. Implementation Checklist

- **Month 1**: Clerk.dev setup, JWT integration into Go binary, `~/.aether/token` storage, silent refresh, `aether onboard`, `aether login`.
- **Month 2**: `aether account delete` command.
- **Month 4**: Channel-based auto-auth.
- **Month 5**: Documentation and Privacy README section live.
