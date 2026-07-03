# Extension conformance

Status: Normative target; static order-extension v1 subset covered

No module is compatible merely because it compiles against a Go interface or
completes a protocol handshake. It must preserve contract semantics, failure
behavior, authority boundaries, and recovery guarantees.

## Common suite

Every extension mechanism is tested for:

- descriptor/schema validation and stable identity;
- supported, unsupported, missing, and conflicting contract versions;
- distribution, installation/composition, authorization, configuration, and
  health capability gates;
- duplicate IDs, retries, timeouts, cancellation, restart, and partial
  failure;
- idempotent replay and expected-version conflicts;
- bounded payload, malformed input, unknown fields, and incompatible schema;
- permission denial, secret redaction, and absence of `internal/...` imports;
- drain, upgrade, failed upgrade, rollback, and provider removal;
- audit records and stable diagnostic reasons.

## Role-specific suite

### Ports

- Contract fixtures run against every adapter.
- Adapters cannot mutate state outside the Port's documented responsibility.
- Ambiguous external outcomes have a reconciliation operation.

### Functions

- Equal inputs and declared context produce equal outputs.
- Fuel/time/memory and payload limits fail closed.
- Network, filesystem, database, clock, randomness, and secret access are
  unavailable unless a future contract explicitly supplies deterministic
  values.

### Controllers

- Events are delivered at least once and safely deduplicated.
- Concurrent workers claim events through a visibility lease, module callbacks
  run outside Core locks/transactions, and stale aggregate versions are
  rejected or deferred by consumers.
- Crash-before-effect, effect-before-ack, restart, reordering, and dead-letter
  cases converge through reconciliation.
- Observations and attestations cannot call a Core state transition directly.

### Financial attestations

- Wrong issuer, tenant, order, settlement, condition/version, or evidence is
  rejected, including a valid resource identifier borrowed from another
  tenant.
- Expired, replayed, stale-version, and conflicting attestations are rejected.
- Changing attestation and idempotency IDs does not make the same evidence
  acceptable again, and modules cannot select a payout destination.
- A valid attestation produces the same audited Core command as an equivalent
  first-party Core flow.
- Failure after acceptance but before command completion is recoverable without
  duplicate financial action.

## Order extension cutover suite

The Collectibles cutover additionally requires:

- unknown extension types survive read/write and export without corruption;
- absent or unhealthy providers do not erase order or financial history;
- reservation timeout, expiry, cancel, duplicate commit, and compensation
  fixtures;
- durable delivery replay after process and database restart;
- missing reservation, Controller, and attestation capabilities fail closed;
- reservation IDs/versions survive retries and appear unchanged in lifecycle
  payloads;
- buyer/seller and tenant copies of the same order produce distinct event IDs;
- a financial state change during attestation verification is rejected before
  settlement submission;
- exact contract versions, descriptor/capability mismatch, nil capabilities,
  and post-registration descriptor mutation are rejected or isolated;
- no Collectibles data is mirrored into `FiatMetadata`;
- a source-boundary test rejects product-specific public Core APIs.

## Release evidence

A release claiming an extension capability records the provider/module
version, negotiated contract versions, Open Core commit, conformance suite
version, and result. Black-box suites run against every distribution that
claims the capability. Undeclared capabilities are skipped by policy;
declared-but-unavailable capabilities fail the release.
