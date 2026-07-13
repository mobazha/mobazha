# ADR-019: Embedded-wallet and onramp buyer funding

Status: Proposed

Date: 2026-07-13

## Context

RFC-0012 defines a third buyer key-provisioning path for moderated escrow: a
buyer with no Mobazha node or Identity key participates as a genuine co-owner
of a 2-of-3 escrow using a signing key custodied by a reviewed third-party
embedded-wallet provider (Privy, Coinbase CDP), optionally funded by a fiat
onramp inside checkout.

Two building blocks were missing from Core:

1. an **embedded-wallet provider abstraction** — buyer-owned, buyer-authorized
   signing keys, distinct from `FiatPaymentProvider` (PSP fiat settlement),
   `KeyProvider` (node master keys), and `SettlementSigner` (node/tenant
   settlement keys);
2. an **onramp provider abstraction** and a way to represent an onramp-funded
   attempt inside the existing unified `PaymentSession` without inventing a new
   settlement mode.

The unified payment session
(`UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md`) is observation-driven: the
top-level `SessionStatus` and the fine-grained `FundingState` are *projected*
from `Order.PaymentVerification*` plus on-chain funding observations
(`session_projector.go`), not stored as a mutable buyer-driven state machine.
The single source of truth for `funded`/`verified` is the on-chain funding
observation at the frozen funding target.

## Decision

### 1. Four buyer-funding provider contracts, mirroring the fiat pattern

Add two contracts alongside `FiatPaymentProvider`, each with a fail-closed
per-rail capability surface, an in-process mock, and a registry for
distribution-profile composition (ADR-016 / ADR-018):

- `contracts.EmbeddedWalletProvider` (`pkg/contracts/embedded_wallet.go`) —
  wallet lifecycle + a **structured-typed-data-only** signing surface. The
  contract makes RFC-0012's custody boundary unrepresentable to violate: a
  non-structured payload (raw hash / look-alike domain) is rejected, and a
  signing request without buyer authorization is rejected, so there is no
  platform-unilateral signing path in the type system.
- `contracts.OnrampProvider` (`pkg/contracts/onramp_provider.go`) — quote /
  idempotent initiate / status, with the settlement side fixed by frozen terms
  (no second competing quote) and the buyer↔provider relationship disclosed.

Provider modules live in `internal/payment/embeddedwallet/{mock,privy,cdp}` and
`internal/payment/onramp/{mock}`. Privy's app-authority "server wallet" path is
gated off by default as a non-production Phase 0 reproduction; the production
buyer-JWT path is stubbed pending the Casdoor→Privy identity link.

### 2. Onramp is a funding source, not a settlement mode

An onramp-funded attempt keeps `settlementMode` on-chain and the
`FundingTarget` as the frozen on-chain address. The onramp purchase is an
ordinary funding observation once the asset arrives. Consequences:

- **No new `FundingTargetType`** (`provider_session` remains fiat-only) and
  **no new `SettlementMode`.**
- **No new top-level `SessionStatus`.** The onramp leg is a *pre-observation
  refinement of `awaiting_funds`*, surfaced only via the fine-grained
  `FundingState`. The new states
  (`onramp_awaiting_payment`, `onramp_processing`, `onramp_delivering`,
  `onramp_forwarding`) map to `SessionStatusAwaitingFunds` through
  `deriveSessionStatus`'s existing `default` case — so they cannot advance a
  session by construction.

### 3. The refinement is a pure, invariant-preserving function

`payment.RefineFundingStateForOnramp(base, observedAmount, source)` overrides
the base funding state **only** when the base is `awaiting_funds`, no funds are
observed at the frozen target, and an onramp source is present. A nil source is
always a no-op; any observed funds or an already-advanced base state wins. This
guarantees onramp status can never claim `funded`/`verified`; the chain
observation always does.

### 4. Leave-and-resume via a durable, idempotent funding source

`payment.OnrampFundingSourceView` is the durable record attached to an attempt.
`OnrampProvider.InitiatePurchase` is idempotent on `(AttemptID,
IdempotencyKey)`: a buyer who closes the page and returns re-reads the existing
source and does not create a second onramp order.

## Status of implementation (2026-07-13)

Delivered and tested (unit + contract; existing `pkg/payment` and
`internal/core/payment` suites unaffected):

- both provider contracts, mocks, registries;
- Privy adapter with a live-verified server-wallet fixture (a real Privy
  EIP-712 signature over a SafeTx payload recovers to the wallet address);
- CDP fail-closed skeleton;
- the onramp `FundingState` values, `OnrampFundingSourceView`, and the pure
  `RefineFundingStateForOnramp` function with invariant tests.

Also delivered (2026-07-13, second change set — after independent review of the
persistence design):

- **Persistence** — `models.PaymentAttemptOnrampFundingSource`
  (`payment_attempt_onramp_funding_sources`), migrated in
  `MigrateFiatModels`. Cardinality is deliberately **1:N per attempt**: failed
  and reversed purchases are retained for reconciliation and dispute forensics
  (a fiat-leg reversal after on-chain delivery must stay auditable), while a
  partial unique index on `(tenant_id, attempt_id) WHERE active` — the same
  technique `PaymentAttempt` already uses — enforces at most one purchase in
  flight. `SetStatus` is the only supported status writer and keeps the
  `active` flag consistent. An idempotency unique index
  `(tenant_id, attempt_id, idempotency_key)` backs leave-and-resume.
- **Selection** — `payment.SelectOnrampFundingSource` picks the record the
  projection surfaces: latest active, else latest delivered-to-buyer-wallet
  (forwarding pending), else nil (terminal history and delivered-to-target
  never drive funding state).
- **Projector wiring** — the projector input loads the attempt's history
  (`HasTable`-guarded, nil-safe) and applies `RefineFundingStateForOnramp`
  after `deriveProgress`; the session view exposes the source as
  `onrampFunding`. The full pre-existing `internal/core/payment` suite passes
  unchanged.

Deliberately **not** in this change (sequenced next):

1. **Initiate/resume app-service + endpoint** — create or resume an onramp
   purchase for an authenticated buyer against a frozen attempt, writing the
   side table through `OnrampProvider.InitiatePurchase`.
2. **Embedded-wallet forwarding** — buyer-wallet→target delivery via EIP-3009
   `transferWithAuthorization` and the platform relayer, driving the
   `onramp_forwarding` state.
3. **Buyer checkout UX** (mobazha-unified) — the email-buyer funding flow.

## Consequences

- Onramp/embedded-wallet buyer funding composes on the existing observation-
  driven session without weakening its core invariant.
- A distribution with no reviewed embedded-wallet or onramp module behaves
  exactly as today (fail-closed capabilities, nil funding source).
- The Privy custody boundary (RFC-0012 Proposal 2) is enforced by the contract
  types, not only by review.
