// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"context"
	"net/url"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
)

type fakeClient struct {
	token       string
	tokenErr    error
	lastRequest SessionTokenRequest
	tokenCalls  int
	txs         []Transaction
	lastPartner string
}

func (f *fakeClient) CreateSessionToken(_ context.Context, req SessionTokenRequest) (string, error) {
	f.tokenCalls++
	f.lastRequest = req
	return f.token, f.tokenErr
}

func (f *fakeClient) BuyTransactionsByPartnerUser(_ context.Context, partnerUserID string) ([]Transaction, error) {
	f.lastPartner = partnerUserID
	return f.txs, nil
}

func testProvider(t *testing.T, client Client) *Provider {
	t.Helper()
	p, err := New(Config{
		Rails: map[string]Rail{
			"crypto:eip155:8453:erc20:usdc": {AssetSymbol: "USDC", Network: "base"},
		},
		Client: client,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func purchaseReq() contracts.OnrampPurchaseRequest {
	return contracts.OnrampPurchaseRequest{
		OrderID:          "order-1",
		AttemptID:        "attempt-1",
		RailID:           "crypto:eip155:8453:erc20:usdc",
		SettlementAsset:  "crypto:eip155:8453:erc20:usdc",
		SettlementAmount: "43.70",
		FiatCurrency:     "usd",
		DeliveryTarget:   "0x91eB1182B96Ed52794B36B80A72dAE108f84a17c",
		IdempotencyKey:   "primary",
	}
}

// The session token request is the delivery binding: it must carry the frozen
// funding target and the rail's network/asset, and the resulting URL must
// preset the RECEIVE amount to the frozen settlement amount.
func TestInitiateBindsSessionToFrozenTarget(t *testing.T) {
	client := &fakeClient{token: "st-abc123"}
	p := testProvider(t, client)

	purchase, err := p.InitiatePurchase(context.Background(), purchaseReq())
	if err != nil {
		t.Fatalf("InitiatePurchase: %v", err)
	}
	if client.lastRequest.Address != "0x91eB1182B96Ed52794B36B80A72dAE108f84a17c" {
		t.Fatalf("session address = %q, want the frozen funding target", client.lastRequest.Address)
	}
	if len(client.lastRequest.Networks) != 1 || client.lastRequest.Networks[0] != "base" {
		t.Fatalf("session networks = %v", client.lastRequest.Networks)
	}
	if len(client.lastRequest.Assets) != 1 || client.lastRequest.Assets[0] != "USDC" {
		t.Fatalf("session assets = %v", client.lastRequest.Assets)
	}
	if client.lastRequest.PartnerUserID != "cdp-attempt-1-primary" {
		t.Fatalf("partnerUserId = %q", client.lastRequest.PartnerUserID)
	}

	parsed, err := url.Parse(purchase.BuyerActionURL)
	if err != nil {
		t.Fatalf("parse BuyerActionURL: %v", err)
	}
	q := parsed.Query()
	if q.Get("sessionToken") != "st-abc123" {
		t.Fatalf("sessionToken = %q", q.Get("sessionToken"))
	}
	if q.Get("presetCryptoAmount") != "43.70" {
		t.Fatalf("presetCryptoAmount = %q, want the frozen settlement amount", q.Get("presetCryptoAmount"))
	}
	if q.Get("fiatCurrency") != "USD" {
		t.Fatalf("fiatCurrency = %q", q.Get("fiatCurrency"))
	}
	if purchase.OnrampOrderID != "cdp-attempt-1-primary" {
		t.Fatalf("OnrampOrderID = %q", purchase.OnrampOrderID)
	}
	if purchase.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("status = %s", purchase.Status)
	}
}

// An unconfigured rail is fail-closed (RFC-0012 Proposal 6): zero
// capabilities without error, and initiate refuses before any provider call.
func TestUnconfiguredRailFailsClosed(t *testing.T) {
	client := &fakeClient{token: "st-abc123"}
	p := testProvider(t, client)

	caps, err := p.Capabilities(context.Background(), "crypto:eip155:1:native")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Offerable || caps.DeliverToTarget {
		t.Fatalf("unproven rail must be fail-closed, got %+v", caps)
	}

	req := purchaseReq()
	req.RailID = "crypto:eip155:1:native"
	if _, err := p.InitiatePurchase(context.Background(), req); err == nil {
		t.Fatal("initiate on an unconfigured rail must fail")
	}
	if client.tokenCalls != 0 {
		t.Fatal("no session token may be minted for a closed rail")
	}
}

func TestPurchaseStatusMapsOnrampLifecycle(t *testing.T) {
	cases := map[string]contracts.OnrampStatus{
		"ONRAMP_TRANSACTION_STATUS_CREATED":     contracts.OnrampStatusAwaitingPayment,
		"ONRAMP_TRANSACTION_STATUS_IN_PROGRESS": contracts.OnrampStatusProcessing,
		"ONRAMP_TRANSACTION_STATUS_SUCCESS":     contracts.OnrampStatusDelivered,
		"ONRAMP_TRANSACTION_STATUS_FAILED":      contracts.OnrampStatusFailed,
		"ONRAMP_TRANSACTION_STATUS_NEW_STATE":   contracts.OnrampStatusProcessing,
	}
	for provider, want := range cases {
		client := &fakeClient{txs: []Transaction{{TransactionID: "t1", Status: provider, CreatedAt: "2026-07-15T00:00:00Z"}}}
		p := testProvider(t, client)
		got, err := p.PurchaseStatus(context.Background(), "cdp-attempt-1-primary")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got.Status != want {
			t.Fatalf("%s -> %s, want %s", provider, got.Status, want)
		}
		if client.lastPartner != "cdp-attempt-1-primary" {
			t.Fatalf("polled wrong partnerUserId %q", client.lastPartner)
		}
	}
}

// No transaction yet means the buyer has not finished the hosted checkout:
// still awaiting payment, never failed.
func TestPurchaseStatusWithoutTransactionsIsAwaitingPayment(t *testing.T) {
	p := testProvider(t, &fakeClient{})
	got, err := p.PurchaseStatus(context.Background(), "cdp-attempt-1-primary")
	if err != nil {
		t.Fatalf("PurchaseStatus: %v", err)
	}
	if got.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("status = %s, want awaiting_payment", got.Status)
	}
}

func TestParseRails(t *testing.T) {
	rails, err := ParseRails("crypto:eip155:8453:erc20:usdc=USDC:base:USD|EUR")
	if err != nil {
		t.Fatalf("ParseRails: %v", err)
	}
	rail := rails["crypto:eip155:8453:erc20:usdc"]
	if rail.AssetSymbol != "USDC" || rail.Network != "base" {
		t.Fatalf("rail = %+v", rail)
	}
	if len(rail.FiatCurrencies) != 2 {
		t.Fatalf("fiat = %v", rail.FiatCurrencies)
	}
	if _, err := ParseRails("rail=USDC"); err == nil {
		t.Fatal("a rail without a network must error loudly")
	}
	if _, err := ParseRails(""); err == nil {
		t.Fatal("empty rail config must error")
	}
}
