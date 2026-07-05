# Order extension contract

Status: v1 implemented; Collectibles is the first provider

## Purpose

The order extension contract lets a domain bind a versioned resource or
domain process to an order and participate in its lifecycle without adding
product-specific fields, callbacks, or financial commands to Core.
Collectibles is the first implementation, not the definition of the contract.
The names and guarantees are limited to concepts already stable across
domains.

Use this contract when the binding must survive restarts and provider absence,
when scarce capacity must be reserved before funding, when external work must
be delivered durably, or when Core must validate an observation or attestation
before a Core-owned transition. Candidate applications include:

| Resource category | Possible order-extension responsibility |
|---|---|
| Collectible Hub slot | Reserve capacity, deliver or mint, and attest completion |
| Limited inventory | Bind an allocation and commit or release it from durable events |
| Gift-card redemption quota | Reserve provider quota and report issuance evidence |
| Event ticket | Reserve admission capacity and report ticket issuance |
| Regulated product lot | Persist a lot binding and return permitted fulfillment evidence |
| Made-to-order capacity | Reserve a production slot and report production or delivery milestones |

The table is a modeling guide, not a list of shipped providers. A candidate
does not need every sub-capability: for example, a reservation-only resource
does not gain settlement-attestation authority. Simple implementation
replacement remains a Port; pure policy remains a Function; ordinary external
reconciliation remains a Controller. Stable commerce concepts that Core owns
should be modeled in Core rather than hidden in an extension payload.

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
  reservation_required
  settlement_policy
  lifecycle_events
  payload
  payload_hash
  created_at
}
```

The persisted identity is scoped to the order as well as provider, type, and
resource, so the same external resource identifier cannot collide across two
orders owned by one tenant.

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
- `reservation_required` persists the fail-closed requirement independently
  from whether the provider module is currently installed.
- `settlement_policy` is either Core default or `extension-attested`. The
  latter blocks automatic settlement even when the module is unavailable and
  requires a validated attestation through the Core command path.
- `lifecycle_events` is a sorted, duplicate-free allowlist of durable events
  requested by this extension. Core does not enqueue undeclared events, and a
  non-empty subscription requires the module's delivery contract.
- `payload_hash` uses a contract-declared canonical encoding and digest
  algorithm, making the declaration auditable and detecting unintended
  mutation.
- `created_at` is assigned by Core rather than trusted from extension input.

Core validates envelope size, uniqueness, authorization, supported contract
version, and immutable fields. It treats the payload as opaque except for
explicitly promoted cross-domain fields. Updates create a new version rather
than silently replacing evidence used by completed financial actions.

Declarations are produced by the provider's
`order-extension.declaration/v1` capability from the signed `OrderOpen`.
Core no longer calls a Collectibles codec from order or payment composition.
The declaration capability receives no database, network, key, or Node handle.

A module may separately declare
`order-extension.declaration-admission/v1`. Core invokes this policy Function
only after that module has produced non-empty, validated declarations and
before any declaration is persisted. The Function receives detached copies of
the signed order and envelopes, may allow or deny the new declaration, and
cannot mutate Core state. Runtime distribution policy and feature governance
belong here rather than inside the pure declaration codec. Admission is not
consulted for already-persisted lifecycle delivery, release, or settlement, so
disabling new work does not strand existing orders.

## 2. Resource reservation

Domains that allocate scarce resources implement a synchronous provisioning
gate:

```text
Reserve(order, extension, idempotency_key, expiry)
  -> reservation_id, reservation_version, status
```

Core invokes `Reserve` before creating a funding target. The provider owns its
resource store. Repeated requests are idempotent and return the same logical
outcome. Core persists `reservation_id`, `reservation_version`, the bound
extension revision, status,
payment coin, idempotency key, and expiry before continuing funding
provisioning. Later commit/release work is driven by durable payment and
terminal-order events carrying that exact binding through the module
Controller, so Core does not expose a second partially used mutation API.
Expiry and ambiguous timeouts are reconciled by the provider; Core never
assumes a timeout means failure. Every operation checks the tenant, order,
extension, and expected reservation binding.

Collectibles token or inventory allocation is the first mapping to this
contract. Other providers retain their own quota, seat, lot, or capacity
vocabulary inside their namespaced payloads. The generic contract must not
expose chain, token, collection, mint, ticket, batch, or provider-specific
vocabulary.

An `extension-attested` declaration is compatible only with a `CANCELABLE`
settlement rail. Core rejects Fiat, DIRECT, MODERATED, and other incompatible
methods before exposing a funding target or accepting a payment message.

## 3. Durable lifecycle delivery

Extension side effects are driven by a transactional Core outbox, not an
in-memory callback. A logical event envelope contains:

```text
ExtensionEvent {
  event_id
  event_type
  event_version
  tenant_id
  source_id
  order_role
  order_id
  order_version
  extension_ref
  occurred_at
  payload
}
```

`event_id` is derived from tenant, source actor, local order role, order,
extension, and event type, so buyer/seller and multi-tenant copies cannot
collide. Core commits the domain transition and outbox record atomically. A
Controller consumes at least once and deduplicates by `event_id`. The delivery
system assigns a monotonic `order_version` from a durable per-extension aggregate
sequence and defines bounded backoff, a database compare-and-swap visibility
lease, dead-letter state, and a transactional requeue primitive. Module code
is invoked only after the lease transaction commits; Core never holds an
internal mutex or database transaction across a Controller call. Versions are
monotonic for each order-extension aggregate, but at-least-once execution may
overlap after lease expiry; consumers must deduplicate and reject or defer
stale order versions. A public operator reconciliation API remains future
evolution.

Minting, fulfillment, and external delivery belong in Controllers. A delivery
result returns as an idempotent observation and may advance only a
Core-defined non-financial transition unless an independently validated
settlement condition applies.

`payment-verified/v1` payloads contain the extension envelope, persisted
reservation binding, settlement ID, payment coin/amount, and a Core-issued
opaque `order_state_version`. Terminal release payloads contain the extension,
reservation binding, and reason. Controllers do not reconstruct either
binding from product metadata.

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
  extension_id
  settlement_id
  expected_extension_revision
  expected_order_state_version
  condition_type
  condition_version
  evidence_digest
  observed_at
  expires_at
}
```

Core verifies issuer authorization, tenant/order/settlement binding, condition
schema and version, evidence digest, freshness, idempotency and replay
protection, policy, expected extension revision, and the opaque financial
order-state version issued in the payment event. After module verification,
Core re-reads the order and extension while holding the settlement submission
lock and compares both versions again, closing the verifier-to-command TOCTOU
window. If valid, Core issues the existing
standard settlement command. The module cannot provide a payout address: Core
resolves the seller's active receiving account for the settlement coin and
passes that Core-owned destination to the command. Accepted evidence is also
claimed by a durable fingerprint over issuer, order, settlement, extension,
condition, expected revision, and evidence digest, so changing request IDs does
not bypass replay protection. The state machine remains the only writer of
settlement state. Composed distributions reach this capability through the
typed `NodeService.ConditionalSettlement()` accessor.

After execution, Core binds the accepted attestation to the exact settlement
action or transaction and Core-owned payout address. The later internal order
confirmation revalidates that binding and the opaque order-state version under
the order lock. Public confirmation, explicit default settlement actions,
Fiat auto-confirm, and client-signed confirmation instructions all fail closed
for `extension-attested` orders.

For a Collectibles primary sale, successful delivery can satisfy a declared
condition, but the Controller does not execute release. Disputes, refunds,
expiry, and reorg/reversal policy remain Core decisions.

## 5. Versioning and change policy

Contract versions are negotiated per capability. Additive optional fields may
remain within a major version; changed meaning, authority, idempotency,
ordering, or financial semantics requires a new major contract version. Core
preserves unknown envelopes and exposes only safe metadata when their provider
is absent.

Open Core performs a direct development-time cutover to this contract. It does
not retain Collectibles hooks, a Collectibles-specific outbox, FiatMetadata
mirrors, or product-specific settlement commands. New domain integrations must
compose through the generic module contract from their first implementation.

## Resource collateral boundary

Independent resource collateral is not an Order Extension v1 reservation or
settlement policy. A provider reservation owns scarce domain capacity, while
collateral is separately funded money whose account, allocation, release,
claim, slash, audit, and rail reconciliation state belongs to Core. An
extension cannot mark collateral funded or select a compensation destination.

Order Extension v1 therefore remains unchanged. A future major contract may
carry a Core-issued collateral allocation reference after Core has validated
tenant, provider, resource, principal, asset, amount, state, and revisions.
Opaque provider payload fields and Hosting projections are not proof of
funding. The staged design is tracked by
[mobazha-docs RFC-0004](https://github.com/mobazha/mobazha-docs/blob/main/rfcs/0004-core-owned-resource-collateral.md).

Existing Solana Anchor and EVM Safe adapters are order-settlement adapters:
they require order escrow data and interpret actions as seller payout, buyer
refund, or dispute release. They do not implicitly implement the dedicated
`collateral.Rail` contract and must not be registered as collateral rails
without a separate provider, conformance evidence, and security review.

## Non-goals

- A universal fulfillment taxonomy.
- A global event bus for arbitrary subscribers.
- Executable extension payloads.
- Cross-extension access to another provider's private data.
- A way to bypass Core commands, policy, or state machines.
- A provider-owned collateral balance or a way to reuse seller proceeds as an
  implicit guarantee.
