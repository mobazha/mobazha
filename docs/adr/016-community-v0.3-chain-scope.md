# ADR-016: Community v0.3 Payment Chain Scope

Status: Accepted

Date: 2026-07-01

## Context

The first Community release must advertise only payment rails that complete
monitoring, confirmation, signed sweep, restart recovery, and seller-destination
verification. BTC, BCH, and LTC use the shared UTXO architecture and have
production Electrum sources. Zcash requires a separate lightwalletd- or
zcashd-compatible monitor source and has not completed that release journey.

Keeping ZEC in the positive allowlist before that source is available would make
the runtime capability response stronger than the behavior the distribution can
actually provide.

## Decision

1. Community v0.3 enables BTC, BCH, and LTC with the `utxo_transparent` rail.
2. ZEC is not an enabled Community v0.3 capability and must not appear in
   runtime payment projections, seller configuration, or release claims.
3. Core may continue to recognize ZEC identifiers for existing data and wire
   compatibility, but recognition does not imply availability.
4. Enabling ZEC later requires a production monitor source, complete funding and
   settlement tests, security review, and a new manifest decision.

## Consequences

- The Community manifest and frontend allowlist contain exactly BTC, BCH, and
  LTC.
- Release gates require complete chain journeys for those three chains.
- ZEC-compatible domain types can remain in Open Core without being composed or
  advertised by the Community distribution.

## Superseded decision

This ADR narrows the first-release chain list in ADR-015, decision 8. It does not
change ADR-015's plugin security or compatibility boundaries.
