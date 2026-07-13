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

## Fastest fully-automated proof (no browser, CI-friendly)

`go test ./internal/core/payment/ -run TestOnrampFundingReachesSessionProjection`
drives a real order + frozen attempt + onramp source row through the real
session projector and asserts the funding leg surfaces and refines while the
session status stays `awaiting_funds`. That is the same vertical the UI reads.
