# Extension capability and security model

Status: Target governance model; static contract gate implemented

Today, order-extension startup enforces the `contract compatible ∩ statically
composed` portion, including exact v1 contract names and capability/interface
agreement. Distribution allowlists, tenant authorization/configuration, and
structured health gates described below are required evolution before those
capabilities can be advertised as generally available.

## Capability activation

An extension capability is externally available only when it belongs to every
set below:

```text
effective = distribution allowlist
          ∩ contract compatible
          ∩ installed or statically composed
          ∩ authorized
          ∩ configured
          ∩ healthy
```

- **Distribution allowlist**: the selected distribution permits the
  capability.
- **Contract compatible**: Core and extension negotiate a supported contract
  version and all required sub-capabilities.
- **Installed/composed**: an artifact is verified and present, or a reviewed
  module is linked into the composition.
- **Authorized**: tenant, operator, seller, and license/entitlement policy
  permit use where applicable.
- **Configured**: required configuration and scoped secrets are valid.
- **Healthy**: the provider satisfies readiness and dependency health gates.

Failure of any gate removes the capability from discovery and causes direct
operations to fail closed with a stable reason. Recognizing an identifier or
having source code in the binary does not enable it.

## Trust and runtime matrix

| Extension source | Default runtime | Typical use | Required controls |
|---|---|---|---|
| Reviewed first-party | Statically linked module | Product composition, low-latency adapters | Public contracts, composition validation, least privilege |
| Third-party or independently distributed | Isolated process | Payment/chain/provider integration | Signed artifacts, protocol negotiation, OS/process isolation, health and restart policy |
| Merchant-authored rule | Wasm sandbox when supported | Pricing, eligibility, routing decision | Determinism, fuel/time/memory limits, no I/O or secrets |

The matrix is a default, not proof of safety. Moving to a less isolated runtime
requires threat analysis and an ADR. Networked remote providers use the same
contract and capability rules as local processes, with transport-specific
authentication and replay protection.

## Authority model

Extensions may submit only four categories of input:

1. **Declaration**: versioned metadata or requested capability attached to a
   Core resource.
2. **Decision**: a bounded deterministic answer to a Core-owned policy
   question.
3. **Observation**: an idempotent fact about an external system.
4. **Attestation**: typed evidence that a declared condition is satisfied.

Core validates identity, authorization, schema version, resource binding,
expected state/version, idempotency, freshness, and domain policy before
accepting any input. Acceptance creates a Core command or durable fact; it
never grants the extension a general mutation handle.

## Financial safety

- Only Core commands may mutate payment, refund, dispute, or settlement state.
- A payment adapter may perform an external settlement side effect only in
  response to an authorized Core command; it cannot originate the financial
  decision and must report an idempotent result for Core reconciliation.
- A Controller cannot call a database repository or internal settlement
  service directly.
- An attestation names its issuer, subject, condition, evidence digest,
  observed time, expiry, and idempotency key.
- Core verifies that the issuer is authorized for that condition and that the
  expected order/settlement version still matches.
- Signing remains behind Core policy and `KeyProvider`; extensions do not
  receive raw seed or private-key material.
- Every accepted or rejected financial input is auditable without logging
  secrets or sensitive payloads unnecessarily.

## Data and permission boundaries

Modules receive minimum typed projections and scoped capability handles.
Permissions are explicit for network destinations, secret descriptors,
filesystem/data namespaces, signing purposes, and event subscriptions.
Unknown or expanded permissions require operator approval.

Product-specific payloads are opaque to unrelated Core components, but their
envelope, hash, schema version, ownership, retention, and maximum size are
validated by Core. Sensitive payload fields should live in the extension's own
store and be referenced by an auditable resource ID when possible.

## Revocation and failure

Disabling or revoking an extension stops new work and hides its capabilities.
It does not erase Core records or abandon in-flight financial obligations.
Core retains enough declarations, observations, attestations, and delivery
state to reconcile or migrate work. Required capabilities that become
unhealthy place affected operations in an explicit blocked or degraded state;
they never silently fall back to a different financial behavior.
