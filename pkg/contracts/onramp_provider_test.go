// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"errors"
	"testing"
)

func TestOnrampQuoteRequestValidate(t *testing.T) {
	valid := OnrampQuoteRequest{RailID: "base-usdc", SettlementAsset: "usdc", SettlementAmount: "125.00", FiatCurrency: "USD"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid quote rejected: %v", err)
	}

	noTerms := OnrampQuoteRequest{RailID: "base-usdc", FiatCurrency: "USD"}
	if err := noTerms.Validate(); !errors.Is(err, ErrOnrampTermsNotFrozen) {
		t.Fatalf("expected terms-not-frozen, got %v", err)
	}

	noFiat := OnrampQuoteRequest{RailID: "base-usdc", SettlementAsset: "usdc", SettlementAmount: "1"}
	if err := noFiat.Validate(); err == nil {
		t.Fatalf("expected rejection of missing fiat currency")
	}
}

func TestOnrampPurchaseRequestValidate(t *testing.T) {
	base := OnrampPurchaseRequest{
		OrderID: "o", AttemptID: "a", RailID: "base-usdc",
		SettlementAsset: "usdc", SettlementAmount: "125.00",
		DeliveryTarget: "0xtarget", IdempotencyKey: "idem-1",
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid purchase rejected: %v", err)
	}

	noIdem := base
	noIdem.IdempotencyKey = ""
	if err := noIdem.Validate(); !errors.Is(err, ErrOnrampMissingIdemponent) {
		t.Fatalf("expected missing-idempotency, got %v", err)
	}

	noDelivery := base
	noDelivery.DeliveryTarget = ""
	if err := noDelivery.Validate(); !errors.Is(err, ErrOnrampDeliveryUnbound) {
		t.Fatalf("expected delivery-unbound, got %v", err)
	}

	toWalletNoAddr := base
	toWalletNoAddr.DeliveryTarget = ""
	toWalletNoAddr.DeliverToBuyerWallet = true
	if err := toWalletNoAddr.Validate(); !errors.Is(err, ErrOnrampDeliveryUnbound) {
		t.Fatalf("expected delivery-unbound for wallet mode without address, got %v", err)
	}

	toWallet := base
	toWallet.DeliveryTarget = ""
	toWallet.DeliverToBuyerWallet = true
	toWallet.BuyerWalletAddress = "0xbuyer"
	if err := toWallet.Validate(); err != nil {
		t.Fatalf("valid wallet-delivery purchase rejected: %v", err)
	}

	noTerms := base
	noTerms.SettlementAmount = ""
	if err := noTerms.Validate(); !errors.Is(err, ErrOnrampTermsNotFrozen) {
		t.Fatalf("expected terms-not-frozen, got %v", err)
	}
}

func TestOnrampStatusActive(t *testing.T) {
	active := []OnrampStatus{OnrampStatusCreated, OnrampStatusAwaitingPayment, OnrampStatusProcessing, OnrampStatusDelivering}
	for _, s := range active {
		if !s.Active() {
			t.Fatalf("%q should be active", s)
		}
	}
	inactive := []OnrampStatus{OnrampStatusDelivered, OnrampStatusFailed, OnrampStatusReversed}
	for _, s := range inactive {
		if s.Active() {
			t.Fatalf("%q should not be active", s)
		}
	}
}
