# Mobazha compatibility policy

Status: Initial policy implemented; shared conformance automation active

This document defines the compatibility relationship between Mobazha, its self-hosted and hosted deployments, shared frontends, and compatible distributions.

## Repository and distribution identity

The `mobazha` repository is the public upstream for shared Mobazha commerce behavior. It also contains the default self-hosted composition.

The default capability profile is an internal distribution and publication boundary used by manifests, packaging, tests, and release tooling. It does not create a separate product, order model, payment state machine, or public API namespace.

Compatible distributions may compose additional adapters and services. They can have independent artifacts and release versions, but must conform to the public contracts for capabilities they claim to share.

## Compatibility layers

### Public wire contract

The following are compatibility surfaces:

- public `/v1/*` request methods, paths, and schemas;
- response envelopes and stable error codes;
- public event and webhook schemas;
- order, payment, refund, dispute, and settlement states;
- runtime capability schema and negotiation rules;
- canonical asset and payment-method identifiers;
- documented configuration and persisted public data needed for upgrades.

An implementation is not compatible merely because JSON fields have the same names. State transitions, idempotency, confirmation rules, recovery behavior, and financial invariants are part of the contract.

### Source and package compatibility

Public Go packages explicitly documented as extension contracts follow their own version policy. Code under `internal/`, concrete constructors, database internals, and composition-root details are not public extension contracts.

Private or third-party extensions must not import `internal/`, receive `MobazhaNode`, or access raw seed/private-key material. Use documented Ports or versioned out-of-process protocols.

The normative extension architecture and public-contract proposal process are
defined by [ADR-018](../adr/018-open-core-extension-architecture.md) and the
[`docs/extensions/`](../extensions/README.md) document set. A product-specific
hook or exported method is not a supported extension contract merely because
it is reachable from another repository.

### Capability compatibility

Recognized identifiers, distribution policy, contract compatibility,
installation/composition, authorization, operator configuration, and runtime
health are distinct concerns. Effective availability is:

```text
distribution allowlist
  ∩ contract compatible
  ∩ installed or statically composed
  ∩ authorized
  ∩ configured
  ∩ healthy
```

Recognition and source presence are descriptive only and are not activation
gates.

Clients must:

- render and call only capabilities declared by the backend;
- fail closed when capability data is unavailable;
- tolerate additive unknown capability fields;
- avoid inferring support from source files, frontend executors, or recognized identifiers.

Servers must:

- reject operations outside the effective capability set;
- avoid branching on a distribution name when a concrete capability answers the decision;
- expose additional features through explicit capabilities, versions, or separate namespaces;
- preserve shared semantics for every public capability they declare.

## Versioning

Public releases use semantic versioning where practical:

- patch: compatible fixes and security updates;
- minor: backward-compatible APIs, capability fields, events, or extension points;
- major: breaking wire, state-machine, persisted-data, or public package changes.

Additive JSON fields and optional capabilities are normally minor changes. Removing or renaming fields, changing a state's meaning, weakening an invariant, or requiring a previously optional capability is breaking.

Every breaking proposal requires an RFC or ADR, migration guidance, updated conformance tests, and a declared support window.

Compatible distribution versions do not need to match the default release version. Each distribution must record the Mobazha commit or tag and public contract version it implements.

## Test topology

Compatibility is verified at four levels:

1. Mobazha tests cover domain behavior, state machines, public APIs, events, migrations, and security invariants.
2. Self-hosted E2E covers the default distribution without optional Hosting, identity, search, payment-provider, or operations services.
3. Hosted-platform E2E uses the Hosting control plane with its selected Node distribution.
4. Cross-distribution conformance runs the same black-box public contract against each distribution. Tests for undeclared capabilities are skipped by policy, not satisfied by mocks.

Running the Hosting control plane against the default standalone distribution is not a supported deployment or a public release requirement. It may be used temporarily to diagnose contract drift.

## Change process

A change needs compatibility review when it modifies:

- a public endpoint, schema, error, event, or state;
- capability negotiation or effective-set rules;
- a shared persisted model or migration;
- a public Port, plugin protocol, or signing boundary;
- behavior consumed by `mobazha-unified`.

Adding a public extension point also requires a domain owner, versioned
schema, authority boundary, idempotency and failure semantics, capability
gates, conformance tests, migration, rollback, and removal plan. Generic hooks
and mutable runtime service registries are not compatible substitutes for
that review.

The review must identify the compatibility layer, affected distributions, rollout order, downgrade behavior, tests, documentation, and whether an RFC/ADR is required.

Shared fixes should land in the `mobazha` repository. A security fix may remain under a temporary embargo, but the public portion must be released once disclosure is safe. Permanent independent reimplementations of shared order or payment behavior are not supported.

## Non-goals

This policy does not require:

- every extension capability to be open source;
- all distribution artifacts to release simultaneously;
- identical internal database layouts for private extension tables;
- private control-plane endpoints to be part of the public contract;
- every private module to use the external payment-plugin protocol.

It does require one long-term implementation of shared commerce behavior and explicit, testable boundaries for everything distribution-specific.
