# Wanted board: the search-index contract

The wanted board ("委托") is owned by one node — the node the buyer published
the request on — and that node is its only writer. A seller on another node has
no copy of that board, so discovery goes through the search service the same way
listings already do: **the owning node publishes a signed request record to the
index, and readers list and resolve requests from the index.**

This document describes what the node side implements. The **hosted index is
authoritative**: its interfaces are live on the test environment, its own
document (the "buyer-requests" share page, sections 五 and 六) is the spec, and
where this file and that one differ, that one wins. What follows is the half
this repository owns, plus what the index was verified to do.

Everything below is implemented by:

| Piece | Where |
|---|---|
| The record, its limits and its signatures | `pkg/orders/buyer_request.go`, `pkg/orders/mbzpb/buyer_request.proto` |
| The push/withdraw client | `pkg/database/netdb/netdb.go` (`SetOwnBuyRequest`, `DeleteOwnBuyRequest`) |
| When a push happens | `internal/core/netdb_sync_service.go` (`syncBuyRequest`) |
| The rules the node applies before signing | `internal/core/buy_request_app_service.go` (`validateRequest`) |
| A local stand-in for the index | `internal/buyrequestmock`, `cmd/buyrequest-mock-index` |

## What the index is for, and what it is not

- The index is the **discovery** path: which requests exist, what they say, and
  which node owns each one. It lives at
  `https://test-new.mobazha.org/search/v1/buyer-requests` on the test
  environment, and a node reaches it with
  `--netdbendpoint=https://test-new.mobazha.org/search/v1/netdb`.
- The **truth** stays on the owning node: a request's offers, its comment
  thread, accepting an offer, cancelling. A reader that wants any of that
  addresses the owning node directly over the p2p relay (`?peer=<ownerPeerID>`).
- The index never accepts a write from anyone but the node that owns the
  request. There is no cross-node editing path, and no merge rule.

In phase 0 the board carries no structured offer at all: a seller contacts the
buyer and sends a link to their own listing, and the order goes through the
normal escrow flow. Nothing about offers, comment threads or orders is ever sent
to the index.

## The record

One request is one record, and that record is what is signed, published, stored
and read back. `pkg/orders/mbzpb/buyer_request.proto` is its definition; the
bytes that travel are its protojson encoding.

```jsonc
{
  "requestID": "card-projection-check-354c1a57",
  "revision": 1,                       // starts at 1, +1 per edit
  "peerID": "12D3KooW…",               // the owning node: the record's public identity
  "title": "Card projection check",    // ≤ 250 characters
  "quantity": 2,                       // ≥ 1
  "budget": {
    "currency": { "code": "BTC", "divisibility": 8 },
    "amount": "150000"                 // the currency's smallest unit, integer string
  },
  "acceptedCurrencies": ["BTC", "LTC"],
  "shipTo": "NL",                      // ISO 3166-1 alpha-2, country only
  "productType": "video-projectors",   // the listing taxonomy, so sellers can filter
  "description": "Two projectors, please.",  // ≤ 1000 characters, plain text
  "images": [ { "filename": "front.jpg", "original": "Qm…", "tiny": "Qm…" } ],  // ≤ 8
  "status": "open",                    // open | closed
  "createdAt": "2026-10-11T05:19:01.164Z",
  "expiresAt": "2026-10-18T05:19:01.155Z"
}
```

Notes that are easy to get wrong:

- `amount` is an **integer in the currency's smallest unit** — the same way a
  listing carries its price. It is never a decimal.
- `revision` is what makes an edit safe: the index accepts a record only when
  its revision is **greater** than the one it already holds.
- The board is public. `peerID`, `shipTo` and `budget` are visible to everyone
  who can read the index, and the publish page says so before anything is sent.
- The node's own HTTP API describes a request with its own field names (`slug`,
  `categorySlug`, `deliverTo`, `budget.max`); the record above is the cross-node
  shape. `contracts.BuyRequestDTO` is the former, `opb.BuyerRequest` the latter,
  and `BuyRequestAppService.Record` is the one place they meet.
  **Decided 2026-10-11: the node's API keeps those names.** The index contract is
  the only vocabulary two nodes ever have to agree on; renaming the local API to
  match it would touch every client of one node and buy nothing across nodes.

## Signature

A libp2p peer ID carries its own public key, so the index verifies a signature
without any lookup, and a record can only ever be published by the node it names.
Every value is framed with its **byte length**, which is what stops two
neighbouring values from being re-split into a different record.

```
publication payload = "mobazha:buyer-request:v1"
                      "\n" <len(peerID)> ":" <peerID>
                      "\n" <len(record bytes)> ":" <the serialized record, exactly as sent>

withdrawal payload  = "mobazha:buyer-request-withdraw:v1"
                      "\n" <len(peerID)> ":" <peerID>
                      "\n" <len(requestID)> ":" <requestID>
                      "\n" <len(revision)> ":" <revision>
                      "\n"
```

The record's own bytes end the publication payload — there is no trailing
newline — and the index verifies the bytes it received rather than re-encoding
the record, because two encoders that disagree would let through a record nobody
signed. The separate withdrawal tag keeps a publication signature from being
replayed as a withdrawal.

The index publishes a test vector (private key, record bytes and both expected
signatures). `pkg/orders/buyer_request_signature_test.go` checks this node
against it byte for byte, which is what makes "we agree" a fact rather than a
hope.

`pkg/orders/buyer_request.go` is the reference implementation
(`SignBuyerRequest`, `VerifyBuyerRequest`, `BuyerRequestWithdrawPayload`).

## Push: publish or update

```
POST {netdbEndpoint}/buyer-requests
```

`netdbEndpoint` is the base the node already uses for listings
(`https://app.mobazha.org/search/v1/netdb` by default, empty on testnet unless
`--netdbendpoint` is given).

```jsonc
{
  "peerID": "12D3KooW…",
  "serializedRequest": "base64 of the record bytes that were signed",
  "signature": "base64"
}
```

An accepted push answers `200`; a refused one answers `4xx` with a reason, and
the node keeps its "dirty" flag set so the reconcile pass retries.

Rules the index enforces — and the node applies first, so a buyer is told by
their own node instead of by a refused push:

1. **Signature** verifies over the payload above, with the key inside `peerID`.
2. **Owner** — the record's `peerID` equals the publisher's.
3. **Revision** is greater than the stored one, otherwise `409`.
4. **Deadline** is in the future, and at most 30 days after `createdAt`.
5. **Field rules** — title ≤ 250, description ≤ 1000, quantity ≥ 1, budget a
   positive integer in the smallest unit, at least one accepted currency,
   `shipTo` two letters, `productType` non-empty, ≤ 8 images, `status` one of
   `open`/`closed`.
6. **Quota** — at most 10 open requests per node at a time.
7. **Ban list** — a banned node is refused everywhere.

## Push: withdraw

```
DELETE {netdbEndpoint}/buyer-requests/{requestID}
```

```jsonc
{ "peerID": "12D3KooW…", "revision": 2, "signature": "base64" }
```

The signature covers the withdrawal payload for that `peerID`, `requestID` and
`revision`. Only the owning node may withdraw, and only with the revision it
holds or a newer one: an older copy must not be able to take a newer record off
the board.

## Read: the board

```
GET {searchBase}/search/v1/buyer-requests
    ?q=&p=&pageSize=…&sortBy=added-desc|added-asc|expires-asc
    &productType=&shipTo=&currency=&id=&minBudget=&maxBudget=&budgetCurrency=
```

Query parameters follow `/search/v1/listings`: `q` is full text over title,
description and product type; paging is `p` (1-based) and `pageSize` (≤ 100);
`sortBy` defaults to `added-desc`.

The answer is the search service's usual envelope, with each hit wrapped the way
a listing hit is:

```jsonc
{
  "data": [ { "data": { /* the record */ }, "type": "buyer_request" } ],
  "meta": { "total": 3, "hasMore": false }
}
```

Visibility:

- Only records whose `status` is `open` **and** whose `expiresAt` is still in
  the future. A node withdraws an expired request only when something reads its
  own board, so the index must not rely on the withdrawal alone.
- `currency` matches membership in `acceptedCurrencies`; `shipTo` and
  `productType` are exact matches; `minBudget`/`maxBudget` compare the budget in
  the smallest unit and are only meaningful together with `budgetCurrency`.
- A banned node's records are never served.

## Read: one request

```
GET {searchBase}/search/v1/buyer-requests/{peerID}/{requestID}
```

```jsonc
{ "data": { /* one record */ } }
```

`404` when the pair is unknown, withdrawn, closed, expired or banned. The
answer carries the record's fields plus `updatedAt`, and — so a seller can
re-verify a copy they did not publish — the `serializedRequest` and
`signature` the index accepted. A request is addressed by the node that owns it
as well as by its identifier, which is why the board's links carry `?peer=`:
the index lists requests, but everything past the card is read from the owning
node.

## What the node does today

- Publishes a record whenever a request is created, edited, or gains an offer,
  and re-publishes every open request on node start.
- Withdraws when a request is cancelled, expires, or is locked onto an accepted
  offer — none of those are board entries any more.
- Marks the board dirty when a push or withdrawal fails, and retries on the next
  reconcile pass (every 10 minutes).
- Treats a `4xx`/`5xx` answer as a failure rather than assuming the board was
  updated.
- Falls back to its own board when there is no reachable index, so a standalone
  node still shows what its own buyers posted.
- Does **not** yet narrow `acceptedCurrencies` to the currencies the running
  edition can actually settle. The publish form offers a short list; the node
  accepts whatever it is given. That is a known gap, not a rule of this contract.

## Verifying an implementation

`cmd/buyrequest-mock-index` is a local stand-in for the platform side that
implements exactly this document, in memory:

```bash
go run ./cmd/buyrequest-mock-index -addr 127.0.0.1:4310
```

Point a node at it, and the whole path can be exercised without touching a
production index:

```bash
mobazha start --netdbendpoint=http://127.0.0.1:4310/search/v1/netdb …
curl 'http://127.0.0.1:4310/search/v1/requests'
```

`internal/buyrequestmock` holds the contract tests, including a round trip that
sends a record through the real search-service client
(`TestNodePublishesToTheIndex`) and one that proves a stale revision cannot roll
the board back (`TestStaleRevisionIsRefused`).

Not implemented on the node side, and not required by this contract: any crawler
view of a node's board. The owning node publishes; nothing pulls.
