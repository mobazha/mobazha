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

## Module types

| Type | Responsibility | Allowed output | Prohibited authority |
|---|---|---|---|
| Port adapter | Implement a Core-owned capability | Typed result or error | Change unrelated Core state |
| Module | Declare and compose one or more capabilities | Registration and lifecycle status | Discover arbitrary Core services |
| Function | Customize a bounded decision | Deterministic decision value | I/O, secrets, state mutation |
| Controller | Reconcile external systems | Observation or attestation | Direct financial transition |

A package may implement more than one role, but each exported contract must
have one role and authority boundary.

## Collectibles classification

The current Collectibles/NFT implementation is a first-party domain extension,
not a generic Core order type and not a payment plugin. Its capabilities map as
follows:

- metadata attached to an order becomes a versioned `OrderExtension`
  declaration;
- inventory or token allocation becomes a generic resource reservation with
  `Reserve`, `Commit`, and `Release` semantics;
- minting or delivery becomes a Controller consuming a durable lifecycle
  event and reporting an observation;
- primary-sale release becomes conditional settlement: the extension submits
  a typed attestation and Core executes the standard settlement command;
- existing `Collectible*` hooks and contract methods remain temporary
  compatibility adapters while consumers migrate.

`nft` remains a concrete extension type until at least one additional use case
proves a stable shared abstraction. Core must not generalize product nouns
prematurely.

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

Current Collectibles entry points are not removed immediately. They are frozen
and adapted to generic contracts, then removed only after dual-read/dual-write,
consumer migration, rollback validation, and the documented support window.

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
