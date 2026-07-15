// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package mock

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
)

const rail = "base-usdc"

func purchaseReq(idem string) contracts.OnrampPurchaseRequest {
	return contracts.OnrampPurchaseRequest{
		Buyer:            contracts.BuyerRef{Subject: "buyer@example.com"},
		OrderID:          "order-1",
		AttemptID:        "attempt-1",
		RailID:           rail,
		SettlementAsset:  "usdc",
		SettlementAmount: "125.00",
		FiatCurrency:     "USD",
		DeliveryTarget:   "0xtarget",
		IdempotencyKey:   idem,
	}
}

func TestCapabilitiesFailClosed(t *testing.T) {
	p := New()
	caps, err := p.Capabilities(context.Background(), rail)
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if caps.Offerable {
		t.Fatalf("default provider must not offer any rail")
	}

	p = New(WithRailCapabilities(OpenRail(rail, "USD")))
	caps, _ = p.Capabilities(context.Background(), rail)
	if !caps.Offerable || !caps.DeliverToTarget {
		t.Fatalf("opened rail should be offerable with direct-to-target")
	}
}

func TestInitiateIsIdempotent(t *testing.T) {
	p := New(WithRailCapabilities(OpenRail(rail, "USD")))

	first, err := p.InitiatePurchase(context.Background(), purchaseReq("idem-1"))
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	// Same attempt + same idempotency key: must return the same onramp order.
	again, err := p.InitiatePurchase(context.Background(), purchaseReq("idem-1"))
	if err != nil {
		t.Fatalf("re-initiate: %v", err)
	}
	if again.OnrampOrderID != first.OnrampOrderID {
		t.Fatalf("leave-and-resume must not create a second order: %s vs %s", first.OnrampOrderID, again.OnrampOrderID)
	}

	// A different idempotency key is a genuinely new purchase.
	other, err := p.InitiatePurchase(context.Background(), purchaseReq("idem-2"))
	if err != nil {
		t.Fatalf("initiate other: %v", err)
	}
	if other.OnrampOrderID == first.OnrampOrderID {
		t.Fatalf("distinct idempotency key must yield a distinct order")
	}
}

func TestStatusProgression(t *testing.T) {
	p := New(WithRailCapabilities(OpenRail(rail, "USD")))
	purchase, err := p.InitiatePurchase(context.Background(), purchaseReq("idem-1"))
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if purchase.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("new purchase should await payment, got %q", purchase.Status)
	}

	if err := p.SetStatus(purchase.OnrampOrderID, contracts.OnrampStatusDelivered); err != nil {
		t.Fatalf("set status: %v", err)
	}
	got, err := p.PurchaseStatus(context.Background(), purchase.OnrampOrderID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Status != contracts.OnrampStatusDelivered {
		t.Fatalf("expected delivered, got %q", got.Status)
	}
}

func TestQuoteRequiresFrozenTerms(t *testing.T) {
	p := New(WithRailCapabilities(OpenRail(rail, "USD")))
	_, err := p.Quote(context.Background(), contracts.OnrampQuoteRequest{RailID: rail, FiatCurrency: "USD"})
	if err == nil {
		t.Fatalf("quote without frozen settlement terms must fail")
	}
	q, err := p.Quote(context.Background(), contracts.OnrampQuoteRequest{RailID: rail, SettlementAsset: "usdc", SettlementAmount: "125.00", FiatCurrency: "USD"})
	if err != nil {
		t.Fatalf("valid quote: %v", err)
	}
	if q.Disclosure == "" {
		t.Fatalf("quote must carry buyer<->provider disclosure")
	}
}

func TestAutoAdvanceOffByDefault(t *testing.T) {
	p := New()
	got, err := p.InitiatePurchase(context.Background(), purchaseReq("k"))
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	// Even with real time passing, a provider built without WithAutoAdvance
	// must stay put — this is the behavior every other test relies on.
	after, err := p.PurchaseStatus(context.Background(), got.OnrampOrderID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if after.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("auto-advance must be off by default, got %q", after.Status)
	}
}

func TestAutoAdvanceWalksHappyPath(t *testing.T) {
	now := time.Unix(0, 0)
	p := New(WithAutoAdvance(10*time.Second), withClock(func() time.Time { return now }))
	got, err := p.InitiatePurchase(context.Background(), purchaseReq("k"))
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if got.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("fresh purchase = %q, want awaiting_payment", got.Status)
	}

	for _, tc := range []struct {
		elapsed time.Duration
		want    contracts.OnrampStatus
	}{
		{5 * time.Second, contracts.OnrampStatusAwaitingPayment},
		{10 * time.Second, contracts.OnrampStatusProcessing},
		{20 * time.Second, contracts.OnrampStatusDelivering},
		{30 * time.Second, contracts.OnrampStatusDelivered},
		{300 * time.Second, contracts.OnrampStatusDelivered}, // clamps, never past delivered
	} {
		now = time.Unix(0, 0).Add(tc.elapsed)
		cur, err := p.PurchaseStatus(context.Background(), got.OnrampOrderID)
		if err != nil {
			t.Fatalf("status at %s: %v", tc.elapsed, err)
		}
		if cur.Status != tc.want {
			t.Fatalf("at %s: status = %q, want %q", tc.elapsed, cur.Status, tc.want)
		}
	}
}

func TestAutoAdvanceKeepsIdempotencyAndRespectsPinnedStatus(t *testing.T) {
	now := time.Unix(0, 0)
	p := New(WithAutoAdvance(10*time.Second), withClock(func() time.Time { return now }))
	first, _ := p.InitiatePurchase(context.Background(), purchaseReq("k"))

	// Leave-and-resume must still return the same onramp order, not a second one.
	now = time.Unix(0, 0).Add(25 * time.Second)
	again, err := p.InitiatePurchase(context.Background(), purchaseReq("k"))
	if err != nil {
		t.Fatalf("re-initiate: %v", err)
	}
	if again.OnrampOrderID != first.OnrampOrderID {
		t.Fatalf("idempotency broken: %q != %q", again.OnrampOrderID, first.OnrampOrderID)
	}
	if again.Status != contracts.OnrampStatusDelivering {
		t.Fatalf("resumed purchase = %q, want delivering", again.Status)
	}

	// An explicit SetStatus pins the purchase: auto-advance must not resurrect it.
	if err := p.SetStatus(first.OnrampOrderID, contracts.OnrampStatusFailed); err != nil {
		t.Fatalf("set status: %v", err)
	}
	now = time.Unix(0, 0).Add(300 * time.Second)
	cur, _ := p.PurchaseStatus(context.Background(), first.OnrampOrderID)
	if cur.Status != contracts.OnrampStatusFailed {
		t.Fatalf("pinned status overridden by auto-advance: got %q", cur.Status)
	}
}
