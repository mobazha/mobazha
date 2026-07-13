# Onramp funding — local end-to-end demo runbook

> ADR-019 / RFC-0012 Proposal 5. This drives the onramp funding leg end to end
> with the in-process **mock** onramp provider, so you can watch (and record)
> the payment page cycle through the funding states without a real onramp
> vendor or KYB. Nothing here touches real funds.

## What this exercises

`email/wallet buyer → frozen crypto attempt → onramp purchase (mock) → funding
states surface on the payment session → UI shows the funding leg`. The
settlement itself stays on chain; the mock only stands in for the fiat→asset
purchase leg. Payment "completes" only on the on-chain observation, exactly as
in production.

## 1. Enable the mock onramp provider (dev-only, env-gated)

The onramp registry is empty and fail-closed by default. Register the mock for
the rail(s) you will test by setting a comma-separated rail id list before
starting the node:

```bash
export MOBAZHA_DEV_MOCK_ONRAMP_RAILS="crypto:eip155:1:native"
```

Unset ⇒ no provider registered ⇒ the endpoints return `capability closed`. The
node logs a WARNING when the mock is registered so it is obvious in any
transcript that this is a dev build.

## 2. Bring an order to a payable, frozen attempt

Use the normal checkout path to create an order and provision its payment
session so the crypto attempt reaches `funding_target_ready`:

```
POST /v1/orders                                   # create the order (buyer node)
POST /v1/orders/{orderID}/payment-selection-quotes  { "paymentCoin": "<coin>" }
POST /v1/orders/{orderID}/payment-session          # provision → freezes the attempt target
GET  /v1/orders/{orderID}/payment-session          # confirm status=awaiting_funds, fundingTarget.address set
```

## 3. Initiate onramp funding

```
POST /v1/orders/{orderID}/payment-session/onramp
{
  "providerID": "mock-onramp",
  "fiatCurrency": "USD",
  "deliverToBuyerWallet": true          // optional; drives the forwarding state
}
```

Response is the `onrampFunding` view: `status=awaiting_payment` and a
`buyerActionURL` (the mock's placeholder checkout URL). Re-POST with no body
change to prove leave-and-resume returns the **same** `onrampOrderID`.

## 4. Drive the provider forward and watch the funding state refine

The mock exposes a status hook; in a Go harness (or a small test) call
`provider.SetStatus(onrampOrderID, ...)` to advance
`awaiting_payment → processing → delivering → delivered`. From the client,
poll:

```
POST /v1/orders/{orderID}/payment-session/onramp/refresh
GET  /v1/orders/{orderID}/payment-session
```

and watch `paymentProgress.fundingState` refine through
`onramp_awaiting_payment → onramp_processing → onramp_delivering →
onramp_forwarding` (the last only when delivering to the buyer wallet). The
top-level `status` stays `awaiting_funds` the whole time.

## 5. UI

On the payment page (`apps/web/src/app/payment/page.tsx`, mobazha-unified), the
`OnrampFundingSection` renders whenever `session.onrampFunding` is present:
a status card with a "continue purchase" link while awaiting payment, a spinner
+ provider polling while it progresses, the provider disclosure, and the
explicit note that payment completes only on on-chain confirmation. Point a
browser at the buyer checkout for the order above and record the card cycling
as you advance the mock in step 4.

## Driving it on the local E2E Docker stack (testuser1/2/3)

The `tests/e2e/docker` stack (Casdoor `:18000`, hosting `:18080`, node instances,
anvil, solana, postgres) serves the seeded accounts **testuser1** (Alice/seller),
**testuser2** (Bob/buyer), **testuser3** (Carol), password `123`. A SaaS buyer's
`/v1/*` traffic is served by the node **embedded in the hosting binary**, so the
onramp endpoints must be deployed into `e2e-hosting`.

Get a buyer token and probe:

```bash
cd <mobazha_hosting>
TOK=$(E2E_TEST_USERNAME=testuser2 bash scripts/e2e-token.sh)   # Bob, the buyer
curl -s -H "Authorization: Bearer $TOK" http://localhost:18080/v1/purchases   # 200 on the stock stack
```

### Prerequisite: deploy this branch into e2e-hosting

The stock stack returns **404** for `/v1/orders/{id}/payment-session/onramp`
(this branch isn't built into the running image). Two blockers gate the inject:

1. **Build entanglement (must resolve first).** `make dev-hosting` /
   `make dev-node` rebuild through `mobazha-commercial-node`, which references
   the node models `PendingEscrowPaymentInfo.EscrowSeed`,
   `PaymentMessageParams.LocalEscrowIntent`, and `PaymentData.EscrowSeed`. Those
   live in **uncommitted** working-tree changes on the node `main` checkout, not
   on this onramp branch, so a build sourced from this branch alone fails:

   ```
   commercial/internal/payment/solana/adapter.go: binding.EscrowSeed undefined
   commercial/internal/payment/solana/attempt_settlement.go: unknown field EscrowSeed
   ```

   Resolve by committing the EscrowSeed changes (then rebase/merge this branch on
   them), or build from a transient tree that combines this branch with the
   EscrowSeed diff. Once buildable, from `tests/e2e/docker`:

   ```bash
   make dev-hosting \
     HOSTING_SRC=<hosting_worktree>/.claude/worktrees/onramp-endpoint \
     MOBAZHA_SRC=<node_worktree>/.claude/worktrees/embedded-wallet
   ```

2. **Mock provider env (needs container recreate).** The dev mock registers only
   when `MOBAZHA_DEV_MOCK_ONRAMP_RAILS` is in the hosting process env, which is
   fixed at container create time — a binary `docker cp` + restart won't pick it
   up. Add it via a compose override and recreate just hosting:

   ```yaml
   # tests/e2e/docker/docker-compose.override.yml
   services:
     hosting:
       environment:
         MOBAZHA_DEV_MOCK_ONRAMP_RAILS: "crypto:eip155:1:native"
   ```
   ```bash
   docker compose up -d hosting      # recreate with the env, then make dev-hosting to inject the binary
   ```

### The buyer journey (once deployed)

```bash
TOK=$(E2E_TEST_USERNAME=testuser2 bash scripts/e2e-token.sh)
H="Authorization: Bearer $TOK"; J="Content-Type: application/json"
B=http://localhost:18080

# 1. Buy testuser1's listing → get {orderID}
curl -s -X POST "$B/v1/orders"      -H "$H" -H "$J" -d @order.json
# 2. Provision the payment session (freezes the crypto funding target)
curl -s -X POST "$B/v1/orders/$OID/payment-session" -H "$H" -H "$J" -d '{"paymentCoin":"<coin>"}'
curl -s "$B/v1/orders/$OID/payment-session" -H "$H"     # status=awaiting_funds
# 3. Initiate onramp funding (mock provider)
curl -s -X POST "$B/v1/orders/$OID/payment-session/onramp" -H "$H" -H "$J" \
  -d '{"providerID":"mock-onramp","fiatCurrency":"USD","deliverToBuyerWallet":true}'
# 4. Advance the mock (Go harness: provider.SetStatus), then poll
curl -s -X POST "$B/v1/orders/$OID/payment-session/onramp/refresh" -H "$H"
curl -s "$B/v1/orders/$OID/payment-session" -H "$H"     # paymentProgress.fundingState refines
```

Watch `paymentProgress.fundingState` walk
`onramp_awaiting_payment → onramp_processing → onramp_delivering →
onramp_forwarding` while the top-level `status` stays `awaiting_funds`.

## Fastest fully-automated proof (no browser, CI-friendly)

`go test ./internal/core/payment/ -run TestOnrampFundingReachesSessionProjection`
drives a real order + frozen attempt + onramp source row through the real
session projector and asserts the funding leg surfaces and refines while the
session status stays `awaiting_funds`. That is the same vertical the UI reads.
