# ADR-016: In-process first-party distribution composition

Status: Accepted

Date: 2026-07-01

## Context

Mobazha ships a public self-hosted distribution, a private Hosting backend,
and private products composed from public extension points. An earlier private
distribution selected a second Node implementation with a Go build tag. That duplicated fields,
lifecycle methods, API registration, accessors, and empty domain stubs. The
two binaries could compile while silently implementing different state
machines and route semantics.

First-party private modules also differ from third-party plugins. They are
reviewed and released with a commercial distribution, may need low-latency
in-process access, and can implement product-specific protocols such as ManagedEscrow,
Solana, or ExternalPayment. Requiring those trusted modules to be separate processes
would add operational failure modes without creating a meaningful trust
boundary. Untrusted ecosystem extensions still need ADR-015's out-of-process
boundary.

## Decision

1. `mobazha3.0` owns one Node type, one lifecycle state machine, shared domain
   services, migrations, and public API semantics.
2. A composition root selects a distribution before resources are opened.
   Selection is an explicit, validated configuration, not an edition-name
   branch and not a product build tag.
3. A distribution may choose a resource profile. The sovereign profile omits
   P2P and Hosting workers but uses the same Node object, accessors, order
   state, guest checkout, and gateway lifecycle.
4. Product decisions enter through narrow policies and trusted module ports.
   Core owns enforcement call sites and fails closed when a required policy or
   runtime is absent.
5. Private HTTP operations are registered by trusted modules on the Core-owned
   authenticated Huma gateway. The selected product-surface policy determines
   which Core route groups are registered.
6. First-party commercial modules may be statically linked in process. They
   import only public `pkg/...` contracts and never Open Core `internal/...`.
7. Third-party or independently trusted payment extensions remain
   out-of-process under ADR-015.
8. Commercial repositories pin an audited Open Core commit and run shared
   conformance tests. Compatible Open Core fixes flow forward through that
   dependency; private implementation does not flow backward into public
   history.

## Invariants

- No distribution-specific Go build tag exists in Open Core or private products.
- No parallel Node field, lifecycle, accessor, or empty-stub implementation is
  selected per product.
- Distribution configuration is copied and validated before a repository,
  listener, or payment runtime is opened.
- Hosting cannot be combined with a sovereign Node profile in one Node.
- Restricted API profiles are allowlists and are tested for both missing and
  unexpectedly exposed operations.
- Private modules own concrete chain clients and secret administration; Core
  sees only the narrow runtime behavior it must coordinate.

## Consequences

The commercial binary includes reachable Open Core packages even when a
profile does not start every subsystem. This is acceptable: product security
comes from explicit composition, route allowlists, and runtime capability
checks rather than assuming that unreachable source code is a policy control.
The Go linker can still remove unreachable code.

Adding a profile-specific service now requires a public port and an explicit
composition decision. It cannot be implemented by adding another shadow Node
or a no-op stub. Larger shared fixes are made once in Open Core and adopted by
commercial distributions by advancing their pinned dependency.

## Rejected alternatives

- Product-wide Go build tags: create a parallel type system and hide drift.
- A long-lived public downgrade fork: duplicates fixes and creates ambiguous
  product and licensing semantics.
- Separate processes for every first-party module: useful for untrusted
  extensions, but unnecessary overhead for reviewed statically linked modules.
- Runtime reflection or optional-method discovery: failures appear only at
  runtime and can silently omit capabilities.
