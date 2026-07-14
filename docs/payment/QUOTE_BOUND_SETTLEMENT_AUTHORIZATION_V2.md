# Quote-bound settlement authorization v2

Status: implementation design; governed by public RFC-0014.

Current implementation boundary (2026-07-14):

- the v1 route gate admits same-currency attempts only and cross-currency
  orders fall through to the existing conversion route;
- `PaymentAttemptFundingBasis` canonical validation, hashing, immutable
  attempt persistence, settlement-terms v2 commitment, and final-snapshot v2
  validation are implemented;
- v2 funding-basis transport, quote-issuer delegation verification, and
  rail-scoped writer enablement remain disabled until their protocol and
  conformance work lands. No v2 target is exposed yet.

## Required invariant

An actionable cross-currency funding target is valid only when one immutable
payment attempt binds all of the following:

```text
signed OrderOpen
  -> authoritative PaymentSelectionQuote
  -> PaymentAttemptFundingBasis (canonical bytes + SHA-256)
  -> seller-authorized PaymentAttemptSettlementTerms v2
  -> participant authorization bundle + funding target commitment
  -> retained PaymentAttemptSettlementAuthorization v2
```

Pricing currency and payment currency do not need to match. A rail without v2
must be routed before draft creation to an explicitly admitted conversion path;
it must not start v1 and return a cross-currency implementation error.

## Canonical model

`PaymentAttemptFundingBasis` owns:

- version, order ID, attempt ID, and signed `OrderOpen` hash;
- pricing currency, pricing atomic amount, and divisibility;
- payment asset ID, payment currency, divisibility, subtotal, explicit costs,
  and buyer total in payment atomic units;
- conversion-required flag, rate, base, quote, quote divisibility,
  rate-source update time, and `ceil_to_payment_atomic_v1` rounding policy;
- quote ID, policy version, issuer/delegation identity, issue time, expiry time.

Canonical JSON follows struct field order. Atomic integers are unsigned base-10
strings with no leading zeros except `0`. Timestamps are UTC Unix seconds.
`CanonicalBytesAndHash` validates before hashing with SHA-256.

The v2 settlement terms add `fundingBasisHash`. The v2 final authorization
contains the complete funding basis and requires:

```text
basis.orderID       == terms.orderID       == bundle.orderID
basis.attemptID     == terms.attemptID     == bundle.attemptID
basis.paymentAsset  == terms.assetID       == bundle.railID == target.assetID
basis.buyerTotal    == terms.fundingAmount == target.amountAtomic
hash(basis)         == terms.fundingBasisHash
hash(terms)         == bundle.settlementTermsHash
hash(target)        == bundle.fundingTargetHash
```

The seller terms signature covers the canonical v2 terms and therefore the
funding-basis hash. `SettlementKeyOffer` remains economic-data-free.

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
- Gate new writers by rail capability `quote_bound_authorization_v2`.
- Ship persistence/read/validation before enabling writers.
- Retain the admitted conversion route until each rail passes v2 conformance.
- Rollback disables new v2 admission but continues reading and recovering
  already authorized v2 attempts.

Required conformance cases include USD-to-BTC, USD-to-ETH, USD-to-token,
crypto-to-crypto, round-up boundaries, stale quote, wrong issuer, wrong order,
revision/rail/amount mismatch, replay, moderated and Affiliate allocations,
restart before and after quote expiry, and tampered canonical bytes.
