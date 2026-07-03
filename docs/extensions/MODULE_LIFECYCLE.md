# Extension module lifecycle

Status: Target governance contract; static v1 subset implemented

The implemented static subset validates `id`, module `version`, exact
capability contract strings, dependencies, cycles, duplicate IDs, non-nil
capabilities, and agreement between declared contracts and Go interfaces. Core
invokes each capability accessor exactly once and snapshots both the canonical
descriptor and those validated capability instances before runtime use. Runtime type,
configuration schema, allowlist, health, drain, upgrade, and rollback states
below remain planned governance work.

## Module descriptor

Every module, whether statically linked or independently distributed, has a
validated descriptor with equivalent fields:

```yaml
schemaVersion: 1
id: io.mobazha.collectibles
version: 1.0.0
runtime: static
contracts:
  - order-extension.declaration/v1
  - order-extension.reservation/v1
  - order-extension.delivery/v1
  - order-extension.attestation/v1
provides:
  - order-extension.collectibles
requires:
  - order-extension.delivery.v1
configurationSchema: collectibles.config/v1
```

The target descriptor separates module version, descriptor schema version, and
each public contract version. The current Go descriptor contains `ID`,
`Version`, `Contracts`, and `Dependencies`; other fields in the example are
not yet runtime inputs.

## Composition and dependency rules

The composition root builds an immutable module graph before opening a
database, listener, worker, or payment runtime. It rejects:

- duplicate module or capability providers where the contract is singular;
- missing or incompatible required contracts;
- dependency cycles;
- capabilities outside the distribution allowlist;
- unresolved authorization, configuration, or permission requirements;
- product modules importing Open Core `internal/...` packages.

Optional dependencies are explicit and must have defined degraded behavior.
Runtime reflection, optional-method discovery, and mutable global registries
are not composition mechanisms.

## Lifecycle states

```text
discovered -> verified -> validated -> configured -> starting -> ready
       |          |           |            |           |        |
       +----------+-----------+------------+-----------+----> failed

ready -> draining -> stopped -> upgraded -> starting
```

- **Discovered/verified** applies to external artifacts; static modules are
  verified by the reviewed build and dependency provenance process.
- **Validated** means identity, contracts, dependencies, permissions, and
  distribution policy are consistent.
- **Configured** means configuration and secret descriptors validate without
  exposing raw secrets to Core logs or unrelated modules.
- **Starting** acquires only module-scoped resources.
- **Ready** means health gates permit capability publication.
- **Draining** rejects new work, finishes or checkpoints bounded in-flight
  work, and preserves durable delivery state.
- **Failed** removes capabilities and records a stable diagnostic reason.

Core owns state transitions and deadlines. A module reports readiness and
health facts but cannot publish its own capability directly.

## Upgrade and rollback

An upgrade must declare compatible contract versions, configuration migration,
extension-owned data migration, and rollback limits. Core drains the previous
provider, verifies the replacement, negotiates contracts, starts it in a
non-visible state, and publishes capabilities only after readiness succeeds.

Core-owned schemas and financial records cannot depend on a module rollback.
Extension payload envelopes remain readable even when the implementation is
absent. A module data migration that cannot roll back requires an explicit
operator checkpoint and backup plan.

## Health and observability

Health is structured by capability rather than one process-wide boolean. Each
provider exposes readiness, dependency status, lag, last successful
reconciliation, and degraded reasons without leaking secrets. Core records:

- module identity and negotiated contract versions;
- lifecycle transition and reason;
- effective capability gate failures;
- request/observation/attestation IDs and latency;
- retry, dead-letter, reconciliation, and version-conflict counts.

Logs and traces carry module ID, capability, tenant/resource scope, and
idempotency key where safe.
