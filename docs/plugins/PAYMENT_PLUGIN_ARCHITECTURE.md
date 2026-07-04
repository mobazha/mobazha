# Payment Plugin Architecture

## 1. Purpose

Define the target public contract for adding payment chains without modifying Mobazha Core. This specification implements ADR-015, follows the shared governance in ADR-018, and is subordinate to the canonical [public release scope](https://docs.mobazha.org/project/release-scope).

The first release may use compatibility adapters around bundled UTXO code. API names below are architectural contracts, not a claim that every RPC type already exists.

This document governs untrusted or independently distributed extensions.
Reviewed first-party modules follow the public in-process composition boundary
and do not need to use this out-of-process protocol.

The shared capability, lifecycle, authority, and security rules are defined in
[`docs/extensions/`](../extensions/README.md). This payment protocol is one
domain-specific extension contract. It is not a generic module API and must
not be reused for order extensions such as Collectibles.

## 2. Trust model

Plugins are untrusted infrastructure extensions. They may contact chain nodes and indexers, parse hostile network data, and construct transactions. They must not receive raw seeds, private keys, unrestricted database handles, the complete `MobazhaNode`, or imports from `internal/`.

Core remains authoritative for edition/seller policy, order and settlement state, amount/destination validation, key custody, audit/idempotency, plugin lifecycle, and final acceptance of observations.

## 3. Distribution unit

```text
plugin.yaml
bin/<platform>/<executable>
schemas/config.schema.json
assets/icon.svg
locales/<locale>.json
LICENSE
NOTICE
checksums.txt
signature.bundle
```

Minimum manifest shape:

```yaml
schemaVersion: 1
id: org.example.dogecoin
name: Dogecoin Payment Plugin
version: 1.0.0
apiVersion: payment.mobazha.io/v1
license: Apache-2.0
chains:
  - chainId: DOGE
    assets: [DOGE]
capabilities:
  - chain.metadata
  - address.validate
  - payment.setup
  - payment.observe
  - transaction.build
optionalCapabilities:
  - fee.estimate
permissions:
  network:
    - tcp:example.org:50002
  signing:
    - algorithm: secp256k1
      purpose: transaction
```

The versioned manifest model and validation package live in `pkg/paymentplugin`. Process supervision and RPC services are intentionally not claimed as implemented yet.

## 4. Lifecycle

```text
discover -> verify artifact -> validate manifest -> start isolated process
        -> handshake -> negotiate -> health gate -> register
        -> serve -> drain -> stop/upgrade -> rollback on failure
```

The supervisor owns process limits, restart policy, logs, health deadlines, socket allocation, data isolation, and rollback. A plugin is not buyer-visible until handshake, capability, configuration, and health gates pass.

## 5. Protocol services

### PluginControl

- `GetInfo`: identity, version, API versions, build metadata.
- `GetCapabilities`: chain and operation capabilities.
- `ValidateConfig`: validate without activation.
- `Health`: sync height, upstream health, degraded reasons.
- `Shutdown`: bounded graceful stop.

### ChainMetadata

- canonical chain/asset identifiers;
- divisibility and amount bounds;
- address families;
- default confirmation/reorg policy;
- explorer templates and QR URI scheme.

Metadata describes capability; it never enables a chain outside the edition manifest.

### AddressService

- `ValidateAddress`: normalized address, family, network, warnings.
- `DerivePublicAddress`: core-provided public derivation context, never a seed.
- `BuildPaymentURI`: canonical QR/deep-link payload.

### PaymentService

- `SetupPayment`: funding target bound to order, asset, amount, expiry.
- `VerifyDeposit`: observation checked against expected funding facts.
- `GetPaymentStatus`: confirmed, pending, expired, or conflicting facts.

### ObservationService

- `Watch`: idempotent watch keyed by tenant/order/funding target.
- `Unwatch`: stop without deleting durable facts.
- `Reconcile`: bounded scan for missed observations.
- `StreamObservations`: append-only facts with deterministic IDs.

Observations do not directly mutate orders. Core verification gates financial events.

### TransactionService

- `EstimateFee`.
- `BuildUnsignedTransaction`.
- `DescribeSigningPayloads`.
- `AttachSignatures`.
- `BroadcastTransaction`.
- `GetTransactionStatus`.

Core checks order binding, destination, asset, amount, fee ceiling, and policy before approving signing payloads.

### SettlementService

- `Confirm`, `Cancel`, `Refund`, `Complete`, `DisputeRelease`.
- `GetActionStatus`.

Every request includes an idempotency key and expected order/settlement version. Results distinguish prepared, submitted, confirmed, failed, and unsupported.

## 6. Signing boundary

```text
Plugin builds unsigned transaction and typed payloads
  -> Core validates policy and payload summary
  -> KeyProvider signs approved payloads
  -> Plugin attaches signatures and broadcasts
  -> Core independently tracks confirmation
```

Signing requests include plugin ID, chain/network, order, action, asset, amount, destinations, digest, expiry, and idempotency key. Audit logs never contain private keys or seed material.

If a chain cannot support this safely, it requires a separately reviewed signing-provider contract before activation.

## 7. Data isolation

- Core assigns plugin-specific state and logical database namespace.
- Plugins cannot query Mobazha order tables directly.
- Core sends minimum typed order/payment projections.
- Secrets use short-lived handles or scoped descriptors, not copied configuration.
- Removing a plugin does not delete core payment facts or order history.

## 8. Frontend integration

API v1 exposes no plugin JavaScript. Backend projects active capabilities through public APIs. Unified renders generic components using metadata, reviewed icons/translations, configuration JSON Schema, funding targets, and standard status enums.

Executable UI extensions require a later ADR covering signatures, sandboxing, CSP, permissions, and version isolation.

## 9. Versioning

- Protocol identifier: `payment.mobazha.io/v1`.
- Manifest uses independent integer `schemaVersion`.
- Additive optional fields are backward compatible.
- Semantic removal/change requires a new major API version.
- Core and plugin select the highest common version.
- Plugins declare required versus optional capabilities.
- Deprecation covers at least two minor Mobazha releases unless security requires faster removal.

## 10. Compatibility kit

Every plugin passes:

- manifest/schema validation;
- handshake/version negotiation;
- address normalization and invalid-address corpus;
- precision/overflow tests;
- duplicate observation and reorg tests;
- idempotent setup/watch/settlement tests;
- timeout, restart, and upstream outage tests;
- signing-policy negative tests;
- malformed RPC and resource-limit tests;
- license and artifact-integrity checks.

Chain suites add consensus and transaction fixtures for every bundled payment chain.

## 11. Migration sequence

1. Split current `ChainEscrow` responsibilities into narrower public contracts.
2. Introduce Core plugin registry and in-process compatibility adapter.
3. Register UTXO implementations through that adapter.
4. Move non-default chain construction behind separately distributed extension factories.
5. Implement out-of-process supervisor and RPC v1.
6. Convert one bundled UTXO chain as reference external plugin.
7. Stabilize the compatibility kit before publishing a catalog.

Payment plugin publication also requires the common extension conformance
suite in [`CONFORMANCE.md`](../extensions/CONFORMANCE.md). Where this document
is more restrictive for payment or signing behavior, the stricter requirement
applies.
