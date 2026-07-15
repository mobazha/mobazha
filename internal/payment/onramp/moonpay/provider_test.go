// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package moonpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
)

type fakeClient struct {
	txs     []Transaction
	txErr   error
	quote   BuyQuote
	lastExt string
}

func (f *fakeClient) TransactionsByExternalID(_ context.Context, ext string) ([]Transaction, error) {
	f.lastExt = ext
	return f.txs, f.txErr
}

func (f *fakeClient) BuyQuote(context.Context, string, string, string) (BuyQuote, error) {
	return f.quote, nil
}

func testProvider(t *testing.T, client Client) *Provider {
	t.Helper()
	p, err := New(Config{
		PublishableKey: "pk_test_123",
		SecretKey:      "sk_test_secret",
		Rails: map[string]Rail{
			"crypto:eip155:8453:erc20:usdc": {CurrencyCode: "usdc_base", FiatCurrencies: []string{"USD", "EUR"}},
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
		FiatCurrency:     "USD",
		DeliveryTarget:   "0x91eB1182B96Ed52794B36B80A72dAE108f84a17c",
		IdempotencyKey:   "primary",
	}
}

// The widget URL is the entire integration: it must carry the frozen receive
// amount, the app-specified delivery address, our correlation id — and a
// valid signature over exactly the query MoonPay will see, or the widget
// refuses to load in production.
func TestInitiateBuildsSignedURLForTheFrozenTerms(t *testing.T) {
	p := testProvider(t, &fakeClient{})

	purchase, err := p.InitiatePurchase(context.Background(), purchaseReq())
	if err != nil {
		t.Fatalf("InitiatePurchase: %v", err)
	}
	if purchase.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("status = %s, want awaiting_payment", purchase.Status)
	}

	parsed, err := url.Parse(purchase.BuyerActionURL)
	if err != nil {
		t.Fatalf("parse BuyerActionURL: %v", err)
	}
	q := parsed.Query()
	if got := q.Get("walletAddress"); got != "0x91eB1182B96Ed52794B36B80A72dAE108f84a17c" {
		t.Fatalf("walletAddress = %q", got)
	}
	if got := q.Get("quoteCurrencyAmount"); got != "43.70" {
		t.Fatalf("quoteCurrencyAmount = %q, want the frozen settlement amount", got)
	}
	if got := q.Get("currencyCode"); got != "usdc_base" {
		t.Fatalf("currencyCode = %q", got)
	}
	if got := q.Get("externalTransactionId"); got != "moonpay-attempt-1-primary" {
		t.Fatalf("externalTransactionId = %q", got)
	}

	// Verify the signature independently: HMAC-SHA256 over '?'+query-without-
	// signature, keyed by the secret. The signature must be the LAST param.
	rawQuery := parsed.RawQuery
	idx := strings.LastIndex(rawQuery, "&signature=")
	if idx < 0 {
		t.Fatal("signature must be appended as the final query parameter")
	}
	unsigned := rawQuery[:idx]
	sig, err := url.QueryUnescape(rawQuery[idx+len("&signature="):])
	if err != nil {
		t.Fatalf("unescape signature: %v", err)
	}
	mac := hmac.New(sha256.New, []byte("sk_test_secret"))
	mac.Write([]byte("?" + unsigned))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); sig != want {
		t.Fatalf("signature mismatch:\n got %s\nwant %s", sig, want)
	}
}

// Initiate must be deterministic on (AttemptID, IdempotencyKey): a buyer who
// leaves and resumes gets the same purchase, never a second onramp order.
func TestInitiateIsIdempotent(t *testing.T) {
	p := testProvider(t, &fakeClient{})
	first, err := p.InitiatePurchase(context.Background(), purchaseReq())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := p.InitiatePurchase(context.Background(), purchaseReq())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Fatalf("repeated initiate diverged:\n first %+v\nsecond %+v", first, second)
	}
}

// An unconfigured rail is fail-closed (RFC-0012 Proposal 6): zero
// capabilities without error, and initiate refuses outright.
func TestUnconfiguredRailFailsClosed(t *testing.T) {
	p := testProvider(t, &fakeClient{})

	caps, err := p.Capabilities(context.Background(), "crypto:eip155:1:native")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Offerable || caps.DeliverToTarget {
		t.Fatalf("unproven rail must be fail-closed, got %+v", caps)
	}

	req := purchaseReq()
	req.RailID = "crypto:eip155:1:native"
	req.SettlementAsset = "crypto:eip155:1:native"
	if _, err := p.InitiatePurchase(context.Background(), req); err == nil {
		t.Fatal("initiate on an unconfigured rail must fail")
	}
}

func TestPurchaseStatusMapsProviderLifecycle(t *testing.T) {
	cases := map[string]contracts.OnrampStatus{
		"waitingPayment":       contracts.OnrampStatusAwaitingPayment,
		"pending":              contracts.OnrampStatusProcessing,
		"waitingAuthorization": contracts.OnrampStatusProcessing,
		"completed":            contracts.OnrampStatusDelivered,
		"failed":               contracts.OnrampStatusFailed,
		"somethingNew":         contracts.OnrampStatusProcessing,
	}
	for provider, want := range cases {
		client := &fakeClient{txs: []Transaction{{ID: "tx1", Status: provider, CreatedAt: "2026-07-15T00:00:00Z"}}}
		p := testProvider(t, client)
		got, err := p.PurchaseStatus(context.Background(), "moonpay-attempt-1-primary")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got.Status != want {
			t.Fatalf("%s -> %s, want %s", provider, got.Status, want)
		}
		if client.lastExt != "moonpay-attempt-1-primary" {
			t.Fatalf("polled wrong external id %q", client.lastExt)
		}
	}
}

// No provider transaction yet means the buyer has not finished the widget —
// the purchase is still awaiting payment, never failed.
func TestPurchaseStatusWithoutTransactionsIsAwaitingPayment(t *testing.T) {
	p := testProvider(t, &fakeClient{})
	got, err := p.PurchaseStatus(context.Background(), "moonpay-attempt-1-primary")
	if err != nil {
		t.Fatalf("PurchaseStatus: %v", err)
	}
	if got.Status != contracts.OnrampStatusAwaitingPayment {
		t.Fatalf("status = %s, want awaiting_payment", got.Status)
	}
}

// The newest transaction wins when a retry created several under one id.
func TestPurchaseStatusUsesNewestTransaction(t *testing.T) {
	p := testProvider(t, &fakeClient{txs: []Transaction{
		{ID: "old", Status: "failed", CreatedAt: "2026-07-14T00:00:00Z"},
		{ID: "new", Status: "completed", CreatedAt: "2026-07-15T00:00:00Z"},
	}})
	got, err := p.PurchaseStatus(context.Background(), "moonpay-attempt-1-primary")
	if err != nil {
		t.Fatalf("PurchaseStatus: %v", err)
	}
	if got.Status != contracts.OnrampStatusDelivered {
		t.Fatalf("status = %s, want delivered (newest transaction)", got.Status)
	}
}

func TestQuotePricesTheFrozenAmount(t *testing.T) {
	p := testProvider(t, &fakeClient{quote: BuyQuote{TotalAmount: 45.12, FeeAmount: 1.2, NetworkFeeAmount: 0.22}})
	quote, err := p.Quote(context.Background(), contracts.OnrampQuoteRequest{
		RailID:           "crypto:eip155:8453:erc20:usdc",
		SettlementAsset:  "crypto:eip155:8453:erc20:usdc",
		SettlementAmount: "43.70",
		FiatCurrency:     "USD",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.FiatAmount != "45.12" || quote.ProviderFee != "1.42" {
		t.Fatalf("quote = %+v", quote)
	}
	if quote.SettlementAmount != "43.70" {
		t.Fatal("the settlement side must pass through unrenegotiated")
	}
}

func TestParseRails(t *testing.T) {
	rails, err := ParseRails("crypto:eip155:8453:erc20:usdc=usdc_base:USD|EUR, crypto:eip155:1:native=eth")
	if err != nil {
		t.Fatalf("ParseRails: %v", err)
	}
	if rails["crypto:eip155:8453:erc20:usdc"].CurrencyCode != "usdc_base" {
		t.Fatalf("rails = %+v", rails)
	}
	if got := rails["crypto:eip155:8453:erc20:usdc"].FiatCurrencies; len(got) != 2 || got[0] != "EUR" {
		t.Fatalf("fiat = %v", got)
	}
	if rails["crypto:eip155:1:native"].CurrencyCode != "eth" {
		t.Fatalf("rails = %+v", rails)
	}
	if _, err := ParseRails("malformed"); err == nil {
		t.Fatal("malformed entries must error loudly, not drop silently")
	}
	if _, err := ParseRails(""); err == nil {
		t.Fatal("empty rail config must error")
	}
}

// The HTTP client treats a 404 as "no transactions yet", sends the secret via
// the Authorization header, and never leaks it into the URL.
func TestHTTPClientTransactionsByExternalID(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		switch r.URL.Path {
		case "/v1/transactions/ext/known-id":
			_ = json.NewEncoder(w).Encode([]Transaction{{ID: "tx1", Status: "completed", CreatedAt: "2026-07-15T00:00:00Z"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewHTTPClient("sk_test_secret", "pk_test_123", server.URL)

	txs, err := client.TransactionsByExternalID(context.Background(), "known-id")
	if err != nil {
		t.Fatalf("known id: %v", err)
	}
	if len(txs) != 1 || txs[0].Status != "completed" {
		t.Fatalf("txs = %+v", txs)
	}
	if gotAuth != "Api-Key sk_test_secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if strings.Contains(gotPath, "sk_test") {
		t.Fatal("secret must never appear in the URL")
	}

	txs, err = client.TransactionsByExternalID(context.Background(), "never-seen")
	if err != nil {
		t.Fatalf("unknown id must not error: %v", err)
	}
	if len(txs) != 0 {
		t.Fatalf("unknown id must yield empty history, got %+v", txs)
	}
}
