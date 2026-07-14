# Quote-bound settlement authorization v2

Status: implementation design; governed by public RFC-0014.

Current implementation boundary (2026-07-15):

- the v1 route gate remains same-currency only;
- a cross-currency request with an immutable payment-selection quote enters
  v2 only when the exact rail has a funding-target projector;
- buyer Core publishes the canonical funding basis in a signed
  `SETTLEMENT_FUNDING_BASIS` order message before its key offer;
- seller Core retains the proposal, verifies the signed order and buyer
  identity, refreshes its own exchange-rate snapshot, and rejects a proposed
  total below the seller-local minimum;
- the seller-signed v2 terms commit to the funding-basis hash, and the final
  v2 authorization transports the complete basis for byte-identical buyer
  validation and recovery;
- delegated quote issuers are not admitted by v2. The issuer must be the
  signed `OrderOpen` buyer; adding delegation requires a later protocol and
  policy version.

## Required invariant

An actionable cross-currency funding target is valid only when one immutable
payment attempt binds all of the following:

```text
signed OrderOpen
  -> buyer-local immutable PaymentSelectionQuote proposal
  -> PaymentAttemptFundingBasis (canonical bytes + SHA-256)
  -> seller-local rate-floor validation
  -> seller-authorized PaymentAttemptSettlementTerms v2
  -> participant authorization bundle + funding target commitment
  -> retained PaymentAttemptSettlementAuthorization v2
```

Pricing currency and payment currency do not need to match. A rail without v2
must be routed before draft creation to an explicitly admitted conversion path;
it must not start v1 and return a cross-currency implementation error.

## Canonical model

`PaymentAttemptFundingBasis` owns:

- version, order ID, attempt ID, authorization context ID, and signed
  `OrderOpen` hash;
- pricing currency, pricing atomic amount, and divisibility;
- payment asset ID, payment currency, divisibility, subtotal, explicit costs,
  and buyer total in payment atomic units;
- conversion-required flag, rate, base, quote, quote divisibility,
  rate-source update time, and `ceil_to_payment_atomic_v1` rounding policy;
- quote ID, policy version, buyer issuer identity, issue time, expiry time.

Canonical JSON follows struct field order. Atomic integers are unsigned base-10
strings with no leading zeros except `0`. Timestamps are UTC Unix seconds.
`CanonicalBytesAndHash` validates before hashing with SHA-256.

The v2 settlement terms add `fundingBasisHash`. The v2 final authorization
contains the complete funding basis and requires:

```text
basis.orderID       == terms.orderID       == bundle.orderID
basis.attemptID     == terms.attemptID     == bundle.attemptID
basis.authContextID == bundle.authorizationContextID
basis.paymentAsset  == terms.assetID       == bundle.railID == target.assetID
basis.buyerTotal    == terms.fundingAmount == target.amountAtomic
hash(basis)         == terms.fundingBasisHash
hash(terms)         == bundle.settlementTermsHash
hash(target)        == bundle.fundingTargetHash
```

The seller terms signature covers the canonical v2 terms and therefore the
funding-basis hash. `SettlementKeyOffer` remains economic-data-free.

`QuoteIssuer` identifies the buyer PeerID that signed and sent the proposal;
it does not by itself make the buyer's rate authoritative. Seller acceptance
is authoritative: before signing, seller Core obtains a fresh local rate for
the same base/quote orientation, applies the same round-up rule, and requires
the proposed payment subtotal to be at least that local minimum. Overpayment
may be accepted; underpayment, stale seller rate state, expiry, wrong buyer,
or any order/asset/hash mismatch fails closed.

## State and expiry

```text
route_admitted
  -> authorization_draft (basis persisted; target non-actionable)
  -> seller_authorized   (quote checked unexpired here)
  -> funding_target_ready
  -> observed / settled / disputed / refunded

authorization_draft -- quote expires --> expired
```

Quote expiry after `seller_authorized` does not invalidate the frozen attempt
or restart recovery. Any quote, rail, amount, policy, or order change starts a
new attempt.

## Rollout contract

- Keep v1 canonical bytes and readers unchanged.
- Gate new writers independently from v1 by exact-rail funding-target
  projector capability; never infer v2 support from another rail.
- Keep v1 readers and canonical bytes unchanged.
- Retain the admitted conversion route until each rail passes v2 conformance.
- Rollback disables new v2 admission but continues reading and recovering
  already authorized v2 attempts.

Implemented protocol cases cover signed wire round trips, immutable proposal
inbox semantics, stale and underfunded seller-rate rejection, v1/v2 downgrade
rejection, token-rail quote binding, and seller-finalize/buyer-adopt v2.
Remaining release conformance includes USD-to-BTC, USD-to-ETH, USD-to-token,
crypto-to-crypto, round-up boundaries, stale quote, wrong issuer, wrong order,
revision/rail/amount mismatch, replay, moderated and Affiliate allocations,
restart before and after quote expiry, and tampered canonical bytes.
