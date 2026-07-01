# ADR-015: Out-of-process payment plugin boundary

Status: Accepted

Date: 2026-06-28

## Context

The registry separates chain dispatch conceptually, but chain implementations still rely on internal packages and shared composition. A public ecosystem needs a stable boundary that supports third-party chains without granting node-internal or raw-key access.

Go's in-process plugin mechanism has platform, toolchain, dependency, and failure-isolation constraints unsuitable for payment code. Arbitrary frontend code creates another untrusted execution boundary.

## Decision

1. Define a versioned, out-of-process Payment Plugin API.
2. Prefer Connect/gRPC over a local Unix socket, with loopback TCP as cross-platform fallback.
3. Keep seed/private-key custody in Core `KeyProvider`; plugins submit typed signing requests.
4. Split capabilities into metadata, address validation, setup, observation, transaction construction, fee estimation, and settlement.
5. Require capability negotiation and explicit API versions; unknown required capabilities fail activation.
6. Require manifest, checksums, declared permissions, health endpoints, and compatibility tests.
7. Keep frontend v1 schema-driven; no executable plugin JavaScript.
8. Bundle BTC/BCH/LTC initially, even if an in-process compatibility adapter is needed during migration; ZEC remains outside v0.3 until its production settlement path is complete.

## Security invariants

- Plugin processes run least-privileged with isolated data.
- Core validates chain, amount, destination, order binding, and policy before signing.
- Signing is auditable and idempotent.
- Plugin observations cannot directly trigger financial state transitions without Core verification.
- Plugin failure cannot crash Core or corrupt another plugin.
- Installation/upgrade verifies artifacts and supports rollback.

## Consequences

- Plugins may use Go, Rust, TypeScript, or other languages.
- Deployment gains isolation and independent versioning at the cost of extra process management.
- Existing adapters need migration wrappers.
- Separately licensed plugins can remain outside the core repository without importing public `internal/` packages.

## Rejected alternatives

- `plugin.Open`: platform/toolchain coupling and no isolation.
- Unschematized subprocess protocol: cannot test compatibility/security reliably.
- HTTP callback as the only local protocol: valid for remote providers, not preferred for local signing.
- Frontend micro-frontends in v1: deferred pending signed sandbox and CSP design.
