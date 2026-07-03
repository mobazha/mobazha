# Open Core extensions

This directory is the normative entry point for extending Mobazha Open Core.
ADR-018 defines the architecture. These documents turn that decision into
contracts, lifecycle rules, security gates, migration steps, and testable
acceptance criteria.

## Start here

| Document | Purpose |
|---|---|
| [ADR-018](../adr/018-open-core-extension-architecture.md) | Roles, authority boundaries, trust model, and architectural invariants |
| [Capability and security model](CAPABILITY_AND_SECURITY_MODEL.md) | Activation gates, permissions, isolation, and financial safety |
| [Module lifecycle](MODULE_LIFECYCLE.md) | Manifest, dependency validation, startup, health, drain, upgrade, and rollback |
| [Order extension contract](ORDER_EXTENSION_CONTRACT.md) | Generic order metadata, reservations, delivery, and conditional settlement |
| [Order extension evolution plan](ORDER_EXTENSION_EVOLUTION_PLAN.md) | Direct cutover of the current Collectibles integration |
| [Conformance](CONFORMANCE.md) | Contract, security, recovery, and compatibility test requirements |

The [Payment Plugin Architecture](../plugins/PAYMENT_PLUGIN_ARCHITECTURE.md)
defines one specialized out-of-process protocol under this governance model.
It is not the universal module interface.

## Choosing the mechanism

Use the narrowest mechanism that matches the responsibility:

| Need | Mechanism |
|---|---|
| Replace a Core-required implementation | Port |
| Assemble capabilities into a distribution | Module |
| Customize a bounded business decision | Function |
| Reconcile an external system or perform external I/O | Controller |
| Extend order-associated domain data and lifecycle | Versioned `OrderExtension` contract |

Do not add a generic hook when a typed domain contract can express the need.
Do not add a public extension point until Core has at least one real consumer,
a stable authority boundary, and conformance tests.

## Proposal checklist

Every new public extension point must document:

- domain owner and business purpose;
- input/output schema and independent contract version;
- who may declare, call, and authorize it;
- synchronous or durable delivery semantics;
- idempotency key and ordering rules;
- timeout, retry, dead-letter, and recovery behavior;
- capability name and activation gates;
- sensitive data and required permissions;
- Core validation and state-machine re-entry point;
- backward compatibility, migration, rollback, and removal plan;
- conformance tests and observability requirements.

Proposals that cannot answer these questions remain private implementation
details rather than public Open Core extension points.
