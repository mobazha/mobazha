# Order extension contract

Status: Target v1 contract; not yet fully implemented

## Purpose

The order extension contract lets a domain attach versioned data and react to
an order lifecycle without adding product-specific fields, callbacks, or
financial commands to Core. It is initially motivated by Collectibles, but its
names and guarantees are limited to concepts already stable across domains.

## 1. Extension declaration

An order stores zero or more envelopes with this logical shape:

```text
OrderExtension {
  extension_id
  provider_id
  type
  schema_version
  revision
  resource_id
  payload
  payload_hash
  created_at
}
```

- `extension_id` is the stable identity of the logical attachment.
- `provider_id` identifies the module responsible for interpreting the
  payload.
- `type` is a namespaced concrete type such as
  `io.mobazha.collectibles.primary-sale`, not a Core enum claiming every order
  implements that domain.
- `schema_version` versions the payload independently from Core and the module.
- `revision` is a Core-assigned, monotonically increasing optimistic-lock
  version.
- `resource_id` is the stable external or extension-owned resource binding.
- `payload_hash` uses a contract-declared canonical encoding and digest
  algorithm, making the declaration auditable and detecting unintended
  mutation.
- `created_at` is assigned by Core rather than trusted from extension input.

Core validates envelope size, uniqueness, authorization, supported contract
version, and immutable fields. It treats the payload as opaque except for
explicitly promoted cross-domain fields. Updates create a new version rather
than silently replacing evidence used by completed financial actions.

## 2. Resource reservation

Domains that allocate scarce resources implement this lifecycle:

```text
Reserve(order, extension, idempotency_key, expiry)
  -> reservation_id, reservation_version, status

Commit(reservation_id, reservation_version, order_version, idempotency_key)
  -> new reservation_version, committed status

Release(reservation_id, reservation_version, reason, idempotency_key)
  -> new reservation_version, released status
```

Core owns when each operation is requested. The provider owns its resource
store. Repeated requests are idempotent and return the same logical outcome.
Expiry and ambiguous timeouts are reconciled through status lookup or
observation; Core never assumes a timeout means failure. Every mutation checks
the tenant, order, extension, and expected reservation version binding.

Collectibles token or inventory allocation maps to this contract. The generic
contract must not expose chain, token, collection, or mint vocabulary.

## 3. Durable lifecycle delivery

Extension side effects are driven by a transactional Core outbox, not an
in-memory callback. A logical event envelope contains:

```text
ExtensionEvent {
  event_id
  event_type
  event_version
  tenant_id
  order_id
  order_version
  extension_ref
  occurred_at
  payload
}
```

Core commits the domain transition and outbox record atomically. A Controller
consumes at least once and deduplicates by `event_id`. The delivery system
defines bounded backoff, visibility timeout, dead-letter state, replay, and
operator reconciliation. Ordering is guaranteed only for the documented
aggregate key; consumers must reject or defer stale order versions.

Minting, fulfillment, and external delivery belong in Controllers. A delivery
result returns as an idempotent observation and may advance only a
Core-defined non-financial transition unless an independently validated
settlement condition applies.

## 4. Conditional settlement

An extension never receives a `ReleaseFunds` or internal settlement handle.
It may submit an attestation with this logical shape:

```text
SettlementAttestation {
  attestation_id
  idempotency_key
  issuer
  tenant_id
  order_id
  settlement_id
  expected_version
  condition_type
  condition_version
  evidence_digest
  observed_at
  expires_at
}
```

Core verifies issuer authorization, tenant/order/settlement binding, condition
schema and version, evidence digest, freshness, idempotency and replay
protection, policy, and expected state. If valid, Core issues the existing
standard settlement command. The state machine remains the only writer of
settlement state.

For a Collectibles primary sale, successful delivery can satisfy a declared
condition, but the Controller does not execute release. Disputes, refunds,
expiry, and reorg/reversal policy remain Core decisions.

## 5. Compatibility and removal

Contract versions are negotiated per capability. Additive optional fields are
compatible; changed meaning, authority, idempotency, or ordering requires a
new major contract version. Core preserves unknown envelopes and exposes only
safe metadata when their provider is absent.

The existing Collectibles metadata, hook, delivery queue, and primary-sale
release APIs are compatibility adapters during migration. No new public
product-specific entry points are added. Their allowlist is explicit and may
only shrink after consumers adopt the v1 contracts.

## Non-goals

- A universal fulfillment taxonomy.
- A global event bus for arbitrary subscribers.
- Executable extension payloads.
- Cross-extension access to another provider's private data.
- A way to bypass Core commands, policy, or state machines.
