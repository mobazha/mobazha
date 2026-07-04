# ADR-018: Open Core extension architecture

Status: Accepted

Date: 2026-07-02

## Context

Mobazha Open Core already has several extension mechanisms: public Go ports,
statically composed first-party modules, payment plugins, policies, callbacks,
and product-specific Collectibles integration points. These mechanisms solve
real problems, but they do not yet share a single governance model. Adding a
new product can therefore leak product vocabulary into Core APIs, bypass a
Core state machine, or create an unbounded hook surface.

The goal is not to reproduce WordPress-style global hooks. Commerce and
financial operations need explicit authority, typed contracts, deterministic
recovery, and trust-dependent isolation. Open Core must remain stable while
first-party and third-party capabilities can be independently composed.

## Decision

Mobazha adopts the following extension architecture:

1. **Ports provide replaceability.** A Port is a narrow, Core-owned contract
   for a required capability. Core defines the semantics and call site; an
   adapter supplies the implementation.
2. **Modules provide composition.** A Module declares its identity, contract
   versions, dependencies, capabilities, configuration, runtime type, and
   lifecycle. A composition root validates the complete module graph before
   opening resources.
3. **Functions provide decision customization.** A Function is a pure,
   deterministic rule with bounded input and output. It cannot access the
   database, network, keys, clock, or perform state transitions. Untrusted
   merchant-authored Functions use a sandbox such as Wasm when introduced.
4. **Controllers perform external side effects.** A Controller consumes
   durable Core facts, reconciles an external system, and reports observations
   or attestations. It does not own Core order or financial state.
5. **Core accepts declarations, decisions, observations, and attestations.**
   Extensions never write Core tables or directly invoke internal state
   transitions. Core validates every input and decides whether to issue a
   command to a Core-owned state machine.
6. **Governance is uniform; business contracts are domain-specific.** All
   modules share identity, versioning, dependency, lifecycle, capability,
   health, and security rules. Their business interfaces remain small typed
   contracts such as payment observation or resource reservation.
7. **Extension points are deliberate and domain-scoped.** New extension
   points require an owner, versioned schema, authority boundary, failure
   semantics, idempotency model, timeout/retry behavior, tests, and removal
   plan. There is no global event hook bus or generic service locator.
8. **Runtime isolation follows trust.** Reviewed first-party modules are
   statically linked by default. Independently distributed or third-party
   infrastructure runs out of process by default. Merchant-authored decision
   rules run in a restricted sandbox. Exceptions require a security review
   and ADR.
9. **All financial mutations re-enter Core.** Payment, refund, dispute, and
   settlement changes are Core commands guarded by expected versions,
   idempotency, policy, and state-machine validation. An extension cannot
   originate or authorize a release. After validation, a Core command may
   delegate the external execution leg to a narrowly authorized payment
   adapter, which reports the result back to Core. A domain Controller such as
   Collectibles may only submit evidence that a condition is satisfied.
   Attested policies are enforced at every confirmation surface, not only the
   automatic dispatcher, and are rejected from payment rails that cannot
   execute the declared conditional settlement.
10. **Capabilities are closed by default.** Effective capability is:

    ```text
    distribution allowlist
      ∩ contract compatible
      ∩ installed or statically composed
      ∩ authorized
      ∩ configured
      ∩ healthy
    ```

    A capability is not externally visible unless every gate passes.

### Current implementation boundary

The static order-extension v1 slice currently implements exact contract
compatibility, immutable startup composition, dependency validation, typed
capability/interface agreement, and fail-closed invocation. Distribution
allowlists, per-tenant authorization/configuration, structured module health,
drain, and upgrade orchestration remain governance targets; they are not
claimed as implemented runtime gates by this ADR.

## Module types

| Type | Responsibility | Allowed output | Prohibited authority |
|---|---|---|---|
| Port adapter | Implement a Core-owned capability | Typed result or error | Change unrelated Core state |
| Module | Declare and compose one or more capabilities | Registration and lifecycle status | Discover arbitrary Core services |
| Function | Customize a bounded decision | Deterministic decision value | I/O, secrets, state mutation |
| Controller | Reconcile external systems | Observation or attestation | Direct financial transition |

A package may implement more than one role, but each exported contract must
have one role and authority boundary.

## Order-extension scope and the Collectibles implementation

`OrderExtension` is a generic contract for order-associated domain resources
whose declaration, reservation, external lifecycle, or evidence must remain
durable across several Core order stages. It is neither a synonym for
Collectibles nor a universal product model. Candidate resource categories
include collectible Hub slots, limited inventory, gift-card redemption quotas,
event tickets, regulated product lots, and made-to-order production capacity.
These examples describe the intended contract scope; they are not claims that
all such providers are implemented.

The current Collectibles/NFT implementation is the first implemented
first-party domain module using this contract. It is not a generic Core order
type and not a payment plugin. Its capabilities map as follows:

- metadata attached to an order becomes a versioned `OrderExtension`
  declaration;
- inventory or token allocation becomes a generic synchronous `Reserve`
  capability before funding; commit/release work consumes typed durable
  Controller events carrying the persisted reservation binding;
- minting or delivery becomes a Controller consuming a durable lifecycle
  event and reporting an observation;
- primary-sale release becomes conditional settlement: the extension submits
  a typed attestation and Core executes the standard settlement command;
- product-specific hooks and settlement commands are removed in a direct
  development-time cutover; no compatibility adapter is retained.

The Collectibles extension type remains concrete. A future ticket, quota, lot,
or production-capacity provider receives its own namespaced type and payload;
it does not inherit NFT vocabulary. Shared product taxonomies must wait for at
least one additional implementation to prove them. The envelope, reservation,
durable delivery, and attestation contracts are shared because their authority
and recovery semantics are stable, not because the products are the same.

The detailed contract and phased migration are defined in
[`ORDER_EXTENSION_CONTRACT.md`](../extensions/ORDER_EXTENSION_CONTRACT.md) and
[`ORDER_EXTENSION_EVOLUTION_PLAN.md`](../extensions/ORDER_EXTENSION_EVOLUTION_PLAN.md).

## Invariants

- Open Core owns orders, payments, refunds, disputes, settlement, key custody,
  and their state machines.
- Extensions import documented `pkg/...` contracts only and never Open Core
  `internal/...` packages.
- Extension code receives projections and scoped handles, not unrestricted
  database access or the complete Node object.
- Extension registration is complete before the Node starts serving traffic;
  no mutable global registry changes composition at runtime.
- Durable side effects are idempotent, replayable, observable, and recoverable.
- Removing or disabling an extension preserves Core financial history and
  enough extension envelope data for audit and recovery.
- Public extension contracts are versioned independently from concrete module
  implementations.

## Consequences

New products can be assembled without adding product-specific fields and
callbacks throughout Core. The cost is more explicit contract design,
capability gating, compatibility testing, and migration work. A small amount
of duplication is preferable to a premature universal abstraction.

Because this integration has not been released, the current Collectibles entry
points are removed directly. Signed-order codecs and business validation move
behind the module declaration capability; Core retains only generic extension,
reservation, delivery, and attestation records.

## Rejected alternatives

- Global named hooks with arbitrary payloads: weak authority and compatibility
  boundaries, and unpredictable ordering.
- A giant module runtime exposing all Core services: becomes a service locator
  and makes least privilege untestable.
- Direct database access for trusted modules: couples extensions to migrations
  and bypasses state-machine enforcement.
- A separate process for every extension: isolation is useful for untrusted
  code but is unnecessary overhead for reviewed first-party composition.
- Treating every callback as a Port: obscures whether the caller needs a
  replaceable capability, a decision, or an external side effect.

## Related decisions

- ADR-015: Out-of-process payment plugin boundary.
- ADR-016: In-process first-party distribution composition.
- ADR-017: Community v0.3 payment chain scope.
