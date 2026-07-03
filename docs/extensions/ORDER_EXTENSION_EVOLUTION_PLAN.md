# Order extension evolution plan

Status: Open Core direct cutover implemented

## Scope

This plan moves the first-party Collectibles/NFT integration onto the generic
contracts in [`ORDER_EXTENSION_CONTRACT.md`](ORDER_EXTENSION_CONTRACT.md).
Open Core is still under development, so the implementation performs a direct
cutover rather than carrying product-specific compatibility APIs or storage.
Hosting-specific module implementation remains outside this plan.

## Final classification

| Collectibles concern | Open Core role |
|---|---|
| Signed order metadata | Collectibles codec producing a versioned `OrderExtension` |
| Token/inventory allocation | Module-owned `ReservationPort` |
| Payment and terminal-order notification | Transactional `ExtensionDelivery` event |
| Minting/delivery worker | Module-owned `Controller` |
| Delivery evidence | Module-verified `SettlementAttestation` |
| Seller payout | Existing Core conditional settlement command |

Concrete NFT, chain, mint, certificate, and Hub vocabulary stays in the
Collectibles codec/module. Generic persistence, delivery, orchestration, and
financial state transitions stay in Core.

## Implemented cutover

- `OrderExtensionRecord` stores append-only, hash-verified, Core-revisioned
  envelopes. Collectibles data is not copied into `FiatMetadata`.
- The product codec is invoked only through
  `order-extension.declaration/v1`; purchase, incoming-order, payment-session,
  and delivery composition no longer parse Collectibles metadata directly.
- Signed `RWA_TOKEN` orders fail closed when no declaration module is installed
  or when the module produces no extension envelope.
- `OrderExtensionReservationRecord` persists the exact provider reservation
  ID/version and provisioning context before a funding target is created.
- The generic `extension-attested` settlement policy replaces Collectibles
  inspection in the payment dispatcher and remains fail closed if a module is
  later removed.
- `ExtensionDelivery` is the only lifecycle outbox. It provides bounded retry,
  database-leased delivery, monotonic order versions, dead-letter state,
  replay support, and provider-scoped Controller dispatch without holding Core
  locks across module calls.
- Payment provisioning reserves through `ReservationPort`; missing modules
  fail closed before a funding target is created.
- Conditional settlement requires an explicit extension ID, expected revision,
  Core-issued order-state version, fresh evidence, and a provider-owned
  `AttestationVerifier`. Core reloads and rechecks state under the settlement
  command lock after verification.
- Module registration accepts only the four exact v1 capability contract
  strings, requires descriptor/interface agreement and non-nil capabilities,
  and snapshots descriptors before runtime dispatch.
- Accepted attestations are audited before Core invokes its standard settlement
  state machine. Core derives the seller payout destination, and a durable
  evidence fingerprint rejects replay under different request IDs.
- `WithOrderExtensionModules` is the only public composition entry point.
- A source-boundary test scans every production Go file in `pkg/core` and
  `pkg/contracts` and rejects exported `Collectible*` functions, methods,
  types, variables, and constants.

The cutover deliberately removes:

- `WithCollectible*` hooks and signals;
- `ExecuteCollectiblePrimarySaleRelease`;
- the Collectibles-specific lifecycle queue and dual-read consumer;
- Collectibles-to-`FiatMetadata` dual writes and backfill;
- settlement fallback from missing extension records;
- implicit trust of Collectibles attestations when no verifier is installed;
- physical-listing compatibility classification for old Hub orders.

## Invariants

1. Core owns order, payment, and settlement state machines.
2. Modules receive declared ports and events; they never mutate Core state.
3. Every financial attestation binds tenant, order, settlement, extension,
   provider, condition version, evidence digest, expiry, expected extension
   revision, and the Core-issued financial order-state version.
4. Extension payloads are size-limited, hash-verified, and append-only.
5. Event delivery is at least once; event IDs bind tenant, source actor, local
   order role, order, extension, and event type, and Core assigns monotonic
   aggregate versions under a durable lease.
6. Missing reservation, Controller, or attestation capabilities fail closed.
7. Product-specific public Core APIs are rejected by tests.

## Remaining evolution

- Add provider-independent status reconciliation for ambiguous reservation
  timeouts.
- Add operator APIs and metrics for dead-letter replay.
- Add distribution allowlist, tenant authorization/configuration, health,
  drain, and upgrade gates from the lifecycle governance target.
- Publish a module conformance kit covering descriptor validation,
  reservation idempotency, event replay, and attestation authorization.
- Promote only genuinely cross-domain fields from opaque payloads into future
  major contract versions.

Any incompatible change to authority, ordering, idempotency, or financial
semantics requires a new major extension contract version. Because no released
legacy Collectibles contract is supported, old product-specific APIs must not
be reintroduced as transitional shortcuts.
