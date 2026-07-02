# Order extension evolution plan

Status: Approved migration plan; implementation pending

## Scope

This plan evolves the current first-party Collectibles/NFT integration into
the generic contracts in
[`ORDER_EXTENSION_CONTRACT.md`](ORDER_EXTENSION_CONTRACT.md). It changes Open
Core incrementally and preserves existing behavior until downstream consumers
migrate. Hosting-specific implementation is outside this plan.

## Current classification

| Current Collectibles concern | Target role |
|---|---|
| Order metadata | Versioned `OrderExtension` declaration |
| Token/inventory allocation | Resource reservation Port |
| Payment/order callback | Durable extension lifecycle event |
| Minting/delivery worker | Controller |
| Delivery result | Observation |
| Primary-sale release request | Settlement-condition attestation followed by a Core command |
| `Collectible*` public APIs | Temporary compatibility adapters |

The initial Open Core inventory is concrete and intentionally visible:

| Current surface | Migration destination |
|---|---|
| `pkg/models/collectibles_metadata.go` | Versioned order-extension envelope and Collectibles codec adapter |
| `internal/core/collectibles_hook.go` and `internal/core/options.go` | Module composition plus reservation/delivery contracts |
| `internal/core/payment/session_policy.go` and `internal/core/builder.go` | Generic reservation Port and explicit module composition |
| `pkg/core/node.go` Collectibles aliases/options | Deprecated public compatibility adapters |
| `internal/collectiblesdelivery/`, `internal/core/collectibles_reservation_listener.go`, and Collectibles-specific outbox call sites | Generic extension-event outbox and Controller delivery |
| `internal/core/settlement/collectible_primary_sale.go` | Settlement-condition validation followed by the standard settlement command |
| `pkg/contracts/contracts.go` `ExecuteCollectiblePrimarySaleRelease` | Deprecated compatibility method over conditional settlement |

OE-0 must regenerate this inventory from the repository and include any newly
found surface before its guard becomes mandatory.

## Migration rules

- Preserve one Core order/payment/settlement state machine throughout.
- Introduce generic storage and contracts before changing callers.
- Use dual-read/dual-write only for a bounded, observable migration window.
- Attach schema version and idempotency keys before replaying any side effect.
- Do not delete legacy data until read fallback, downgrade, and audit recovery
  have been verified on production-shaped fixtures.
- Freeze the product-specific surface: new `WithCollectible*`,
  `ExecuteCollectible*`, or equivalent public Core entry points are rejected.
- Source-boundary tests maintain an allowlist of existing compatibility
  symbols; the allowlist may only shrink.

## Phases

### OE-0: Governance and guardrails

- Accept ADR-018 and this document set.
- Inventory existing Collectibles entry points and downstream consumers.
- Add source-boundary tests that block new product-specific public symbols and
  private-module imports of Open Core `internal/...`.
- Capture current behavior with characterization tests.

Exit: the legacy surface and behavior are measurable and cannot grow.

Rollback: remove guard-only changes; no runtime or data change exists.

### OE-1: Durable extension delivery

- Extract a versioned generic extension-event envelope and transactional
  outbox from the existing Collectibles-specific durable delivery path.
- Adapt the current Collectibles enqueue and listener paths to the generic
  outbox and Controller contract.
- Add idempotent retry, dead-letter, replay, and reconciliation operations.
- Keep the current hook as an adapter that writes or consumes the durable path;
  it must no longer be the source of truth.

Exit: a process crash between order transition and delivery cannot lose work,
and duplicate delivery is harmless.

Rollback: pause the new consumer and resume the compatibility consumer from
the same durable cursor; retain outbox records.

### OE-2: Versioned order extension envelopes

- Add generic envelope storage and public read/write contracts.
- Map existing Collectibles metadata to a namespaced extension type.
- Dual-write legacy and envelope representations with hash comparison.
- Read the envelope first and fall back to legacy data while metrics show
  mismatches and unsupported schema versions.

Exit: all new orders have equivalent envelope data, backfill is complete, and
the mismatch rate is zero for the declared observation window.

Rollback: restore legacy-first reads; do not discard envelopes.

### OE-3: Generic resource reservation

- Introduce `Reserve`, `Commit`, `Release`, and status reconciliation Ports.
- Adapt Collectibles allocation behind the Port.
- Define expiry, duplicate request, timeout, cancellation, and compensation
  behavior with fault-injection tests.

Exit: Core orchestration contains no Collectibles allocation vocabulary and
all ambiguous outcomes can be reconciled.

Rollback: switch the adapter to the legacy implementation while preserving
reservation IDs and idempotency records.

### OE-4: Conditional settlement

- Introduce a versioned settlement-condition and attestation contract.
- Validate issuer, tenant/resource binding, condition version, evidence,
  freshness, idempotency, expected state version, and authorization in Core.
- Route accepted attestations through the existing Core settlement command and
  state machine.
- Replace direct Collectibles release calls with the compatibility adapter.

Exit: no extension path can directly mutate settlement state; duplicate,
stale, unauthorized, and conflicting attestations fail safely.

Rollback: disable the new condition capability and hold affected settlements
for reconciliation; never fall back to an unchecked release.

### OE-5: Consumer migration

- Move first-party Collectibles composition to the public module descriptor,
  envelope, reservation, Controller, and attestation contracts.
- Update public projections and client capability checks.
- Publish the conformance kit and compatibility support window.

Exit: production distributions use only compatibility adapters at the old API
boundary, not internally.

Rollback: pin the prior compatible Open Core version and replay durable events;
extension envelope data remains readable.

### OE-6: Legacy removal

- Announce deprecation across at least the documented compatibility window.
- Remove legacy hooks, product-specific contract methods, dual writes, and
  legacy reads after all supported consumers migrate.
- Remove migration metrics and allowlist entries only after final audit and
  downgrade cutoff approval.

Exit: Open Core exposes generic order-extension contracts only; concrete NFT
vocabulary is owned by the Collectibles module.

Rollback: removal occurs only after the downgrade cutoff; emergency recovery
uses the last supported release and retained migration data, not reintroduced
unchecked callbacks.

## Required evidence per phase

Each phase PR must include:

- the affected public contract and compatibility classification;
- data migration and downgrade behavior;
- unit, contract, fault-injection, and cross-distribution tests as applicable;
- capability-gate and unauthorized-path negative tests;
- observability, alert thresholds, and reconciliation runbook changes;
- rollout order for Open Core and downstream consumers;
- an explicit stop/go decision based on the phase exit criteria.
