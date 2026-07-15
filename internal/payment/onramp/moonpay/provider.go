// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package moonpay implements contracts.OnrampProvider over MoonPay's hosted
// buy widget (signed URLs) and transactions API.
//
// Integration shape (phase-1 direct-to-target):
//   - InitiatePurchase builds a signed widget URL that locks the receive
//     amount (quoteCurrencyAmount) to the attempt's frozen settlement amount
//     and points walletAddress at the app-specified delivery address — the
//     frozen funding target. The buyer completes card payment and KYC on
//     MoonPay's hosted page (BuyerActionURL); nothing in this package moves
//     money, and funded/verified still come only from the on-chain
//     observation at the target.
//   - PurchaseStatus polls the transactions API by externalTransactionId.
//
// The signature is mandatory in production whenever walletAddress is passed:
// HMAC-SHA256 over the URL's query string (including the leading '?'), keyed
// by the secret API key, base64-encoded, appended as the final parameter.
//
// Docs (verify against current versions during the sandbox pass):
//
//	https://dev.moonpay.com/v1.0/docs/ramps-sdk-url-signing
//	https://dev.moonpay.com/docs/ramps-sdk-buy-params
package moonpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the stable module identifier.
const ProviderID = "moonpay"

// Rail describes one settlement rail MoonPay may fund.
type Rail struct {
	// CurrencyCode is MoonPay's code for the settlement asset on its network
	// (e.g. "usdc_base"). The rail id -> code mapping is deployment
	// configuration; this package never guesses it.
	CurrencyCode string
	// FiatCurrencies advertised for the rail (advisory; MoonPay decides per
	// buyer region at widget time).
	FiatCurrencies []string
}

// Config wires the provider. Zero-value fields fail closed.
type Config struct {
	// PublishableKey (pk_...) rides in the widget URL; SecretKey (sk_...)
	// signs it and authenticates the transactions API. Both come from env —
	// never from a checked-in file.
	PublishableKey string
	SecretKey      string

	// WidgetBaseURL defaults to the production widget; point it at
	// https://buy-sandbox.moonpay.com for sandbox runs.
	WidgetBaseURL string

	// Rails maps canonical rail ids to MoonPay currency codes. A rail absent
	// here is fail-closed (zero Capabilities), per RFC-0012 Proposal 6.
	Rails map[string]Rail

	// Disclosure is the buyer-facing text describing the buyer<->MoonPay
	// relationship (KYC, fees, reversals). Required by RFC-0012 Proposal 7.
	Disclosure string

	// Client polls purchase status. Defaults to the HTTP client; tests inject
	// a fake.
	Client Client
}

// Client is the MoonPay API surface the provider consumes.
type Client interface {
	// TransactionsByExternalID returns all buy transactions correlated with
	// one externalTransactionId, newest first. An empty slice means the buyer
	// has not completed the widget step yet.
	TransactionsByExternalID(ctx context.Context, externalTransactionID string) ([]Transaction, error)
	// BuyQuote prices acquiring quoteAmount of currencyCode in fiatCurrency.
	BuyQuote(ctx context.Context, currencyCode, fiatCurrency, quoteAmount string) (BuyQuote, error)
}

// Transaction is the subset of MoonPay's buy transaction we consume.
type Transaction struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// BuyQuote is the subset of MoonPay's buy quote we consume.
type BuyQuote struct {
	TotalAmount        float64 `json:"totalAmount"`
	FeeAmount          float64 `json:"feeAmount"`
	NetworkFeeAmount   float64 `json:"networkFeeAmount"`
	BaseCurrencyAmount float64 `json:"baseCurrencyAmount"`
}

// Provider implements contracts.OnrampProvider.
type Provider struct {
	cfg Config
}

// New validates the config and builds the provider.
func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.PublishableKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("moonpay: publishable and secret keys are required")
	}
	if cfg.WidgetBaseURL == "" {
		cfg.WidgetBaseURL = "https://buy.moonpay.com"
	}
	if cfg.Disclosure == "" {
		cfg.Disclosure = "You are buying crypto from MoonPay; its fees, KYC, and reversals are between you and MoonPay."
	}
	if cfg.Client == nil {
		cfg.Client = NewHTTPClient(cfg.SecretKey, cfg.PublishableKey, "")
	}
	return &Provider{cfg: cfg}, nil
}

// ProviderID implements contracts.OnrampProvider.
func (p *Provider) ProviderID() string { return ProviderID }

// Capabilities implements contracts.OnrampProvider. Unconfigured rails return
// the zero value: fail-closed, never an error.
func (p *Provider) Capabilities(_ context.Context, railID string) (contracts.OnrampCapabilities, error) {
	rail, ok := p.cfg.Rails[railID]
	if !ok {
		return contracts.OnrampCapabilities{}, nil
	}
	fiat := rail.FiatCurrencies
	if len(fiat) == 0 {
		fiat = []string{"USD"}
	}
	return contracts.OnrampCapabilities{
		RailID:    railID,
		Offerable: true,
		// The widget delivers to the app-specified walletAddress, which for
		// phase 1 is the frozen funding target itself.
		DeliverToTarget: true,
		FiatCurrencies:  fiat,
	}, nil
}

// Quote implements contracts.OnrampProvider: the fiat cost of the frozen
// settlement amount. The settlement side is never re-negotiated here.
func (p *Provider) Quote(ctx context.Context, req contracts.OnrampQuoteRequest) (contracts.OnrampQuote, error) {
	if err := req.Validate(); err != nil {
		return contracts.OnrampQuote{}, err
	}
	rail, ok := p.cfg.Rails[req.RailID]
	if !ok {
		return contracts.OnrampQuote{}, contracts.ErrOnrampCapabilityClosed
	}
	quote, err := p.cfg.Client.BuyQuote(ctx, rail.CurrencyCode, req.FiatCurrency, req.SettlementAmount)
	if err != nil {
		return contracts.OnrampQuote{}, fmt.Errorf("moonpay: buy quote: %w", err)
	}
	return contracts.OnrampQuote{
		ProviderID:       ProviderID,
		FiatCurrency:     req.FiatCurrency,
		FiatAmount:       trimFloat(quote.TotalAmount),
		ProviderFee:      trimFloat(quote.FeeAmount + quote.NetworkFeeAmount),
		SettlementAsset:  req.SettlementAsset,
		SettlementAmount: req.SettlementAmount,
		Disclosure:       p.cfg.Disclosure,
	}, nil
}

// InitiatePurchase implements contracts.OnrampProvider. It is a pure URL
// construction: MoonPay materializes the transaction only when the buyer
// completes the widget, so "initiate" mints the deterministic correlation id
// and the signed BuyerActionURL. Idempotency on (AttemptID, IdempotencyKey)
// holds because the same request always produces the same id and URL.
func (p *Provider) InitiatePurchase(_ context.Context, req contracts.OnrampPurchaseRequest) (contracts.OnrampPurchase, error) {
	if err := req.Validate(); err != nil {
		return contracts.OnrampPurchase{}, err
	}
	rail, ok := p.cfg.Rails[req.RailID]
	if !ok {
		return contracts.OnrampPurchase{}, contracts.ErrOnrampCapabilityClosed
	}
	address := req.DeliveryTarget
	if req.DeliverToBuyerWallet {
		address = req.BuyerWalletAddress
	}
	externalID := PurchaseExternalID(req.AttemptID, req.IdempotencyKey)

	query := url.Values{}
	query.Set("apiKey", p.cfg.PublishableKey)
	query.Set("currencyCode", rail.CurrencyCode)
	query.Set("walletAddress", address)
	// Lock the RECEIVE amount to the frozen settlement amount: the buyer pays
	// fiat = amount + MoonPay's fees, and the target receives exactly what
	// the attempt froze. (Phase-1 gate question 2 confirms this mode.)
	query.Set("quoteCurrencyAmount", req.SettlementAmount)
	query.Set("baseCurrencyCode", strings.ToLower(req.FiatCurrency))
	query.Set("externalTransactionId", externalID)
	encoded := query.Encode()

	actionURL := p.cfg.WidgetBaseURL + "?" + encoded +
		"&signature=" + url.QueryEscape(SignQuery(p.cfg.SecretKey, encoded))

	return contracts.OnrampPurchase{
		ProviderID:           ProviderID,
		OnrampOrderID:        externalID,
		Status:               contracts.OnrampStatusAwaitingPayment,
		BuyerActionURL:       actionURL,
		DeliveryTarget:       req.DeliveryTarget,
		DeliverToBuyerWallet: req.DeliverToBuyerWallet,
		BuyerWalletAddress:   req.BuyerWalletAddress,
		Disclosure:           p.cfg.Disclosure,
	}, nil
}

// PurchaseStatus implements contracts.OnrampProvider by polling the
// transactions API. No transaction yet means the buyer has not finished the
// widget: the purchase is still awaiting payment, not failed. The returned
// record carries no BuyerActionURL — the durable row written at initiate
// already holds it, and the app service only persists status transitions.
func (p *Provider) PurchaseStatus(ctx context.Context, onrampOrderID string) (contracts.OnrampPurchase, error) {
	if strings.TrimSpace(onrampOrderID) == "" {
		return contracts.OnrampPurchase{}, fmt.Errorf("moonpay: onramp order id is required")
	}
	txs, err := p.cfg.Client.TransactionsByExternalID(ctx, onrampOrderID)
	if err != nil {
		return contracts.OnrampPurchase{}, fmt.Errorf("moonpay: transaction status: %w", err)
	}
	status := contracts.OnrampStatusAwaitingPayment
	if len(txs) > 0 {
		newest := txs[0]
		for _, tx := range txs[1:] {
			if tx.CreatedAt > newest.CreatedAt {
				newest = tx
			}
		}
		status = mapStatus(newest.Status)
	}
	return contracts.OnrampPurchase{
		ProviderID:    ProviderID,
		OnrampOrderID: onrampOrderID,
		Status:        status,
		Disclosure:    p.cfg.Disclosure,
	}, nil
}

// PurchaseExternalID deterministically correlates one attempt-scoped purchase
// with MoonPay's externalTransactionId, so initiate retries and status polls
// agree without provider-side state.
func PurchaseExternalID(attemptID, idempotencyKey string) string {
	return "moonpay-" + attemptID + "-" + idempotencyKey
}

// SignQuery computes the mandatory widget signature: base64 HMAC-SHA256 over
// the query string including its leading '?', keyed by the secret API key.
func SignQuery(secretKey, encodedQuery string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte("?" + encodedQuery))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// mapStatus translates MoonPay transaction statuses into the provider-neutral
// lifecycle. Unknown statuses map to processing rather than failing the poll:
// the stored record stands until the provider reports a terminal state.
func mapStatus(s string) contracts.OnrampStatus {
	switch s {
	case "waitingPayment":
		return contracts.OnrampStatusAwaitingPayment
	case "pending", "waitingAuthorization":
		return contracts.OnrampStatusProcessing
	case "completed":
		return contracts.OnrampStatusDelivered
	case "failed":
		return contracts.OnrampStatusFailed
	default:
		return contracts.OnrampStatusProcessing
	}
}

// trimFloat renders provider float amounts as plain decimals.
func trimFloat(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	return s
}

// ParseRails parses the deployment rail mapping
// "railID=currencyCode[:FIAT1|FIAT2],...". Malformed entries error out
// loudly: a silently dropped rail would fail closed in a way that is
// indistinguishable from deliberate configuration.
func ParseRails(raw string) (map[string]Rail, error) {
	rails := make(map[string]Rail)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		railID, spec, ok := strings.Cut(entry, "=")
		railID = strings.TrimSpace(railID)
		spec = strings.TrimSpace(spec)
		if !ok || railID == "" || spec == "" {
			return nil, fmt.Errorf("moonpay: malformed rail entry %q (want railID=currencyCode[:FIAT|FIAT])", entry)
		}
		code, fiatSpec, _ := strings.Cut(spec, ":")
		rail := Rail{CurrencyCode: strings.TrimSpace(code)}
		if rail.CurrencyCode == "" {
			return nil, fmt.Errorf("moonpay: rail %q has an empty currency code", railID)
		}
		if fiatSpec != "" {
			for _, fiat := range strings.Split(fiatSpec, "|") {
				if fiat = strings.TrimSpace(fiat); fiat != "" {
					rail.FiatCurrencies = append(rail.FiatCurrencies, strings.ToUpper(fiat))
				}
			}
			sort.Strings(rail.FiatCurrencies)
		}
		rails[railID] = rail
	}
	if len(rails) == 0 {
		return nil, fmt.Errorf("moonpay: no rails configured")
	}
	return rails, nil
}

var _ contracts.OnrampProvider = (*Provider)(nil)
