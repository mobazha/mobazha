// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package cdp implements contracts.OnrampProvider over Coinbase Onramp (CDP).
//
// Integration shape (phase-1 direct-to-target):
//   - InitiatePurchase asks the backend Session Token API for a single-use
//     token (5-minute TTL, mandatory since 2025-07-31) whose addresses entry
//     is the app-specified delivery address — the attempt's frozen funding
//     target — then hands the buyer a pay.coinbase.com one-click-buy URL as
//     BuyerActionURL. The buyer pays by debit card / Apple Pay (US guest
//     checkout) or a Coinbase account; nothing in this package moves money,
//     and funded/verified still come only from the on-chain observation.
//   - PurchaseStatus polls the Onramp transactions API by partnerUserId,
//     which we mint deterministically per purchase.
//
// Docs (verify against current versions during the sandbox pass):
//
//	https://docs.cdp.coinbase.com/api-reference/rest-api/onramp-offramp/create-session-token
//	https://docs.cdp.coinbase.com/onramp-&-offramp/onramp-apis/generating-onramp-url
package cdp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the stable module identifier.
const ProviderID = "coinbase-onramp"

// Rail describes one settlement rail Coinbase Onramp may fund.
type Rail struct {
	// AssetSymbol is Coinbase's asset code (e.g. "USDC"); Network is the
	// Onramp network id (e.g. "base"). The rail id -> pair mapping is
	// deployment configuration; this package never guesses it.
	AssetSymbol string
	Network     string
	// FiatCurrencies advertised for the rail (advisory).
	FiatCurrencies []string
}

// Config wires the provider. Zero-value fields fail closed.
type Config struct {
	// PayBaseURL hosts the buyer-facing checkout; defaults to production.
	PayBaseURL string
	// RedirectURL optionally returns the buyer to checkout after paying.
	RedirectURL string
	// Rails maps canonical rail ids to Coinbase asset/network pairs. A rail
	// absent here is fail-closed (zero Capabilities), per RFC-0012 Proposal 6.
	Rails map[string]Rail
	// Disclosure is the buyer-facing text describing the buyer<->Coinbase
	// relationship (KYC, fees, reversals). Required by RFC-0012 Proposal 7.
	Disclosure string
	// Client calls the CDP backend APIs. Required: the session token cannot
	// be minted client-side.
	Client Client
}

// Client is the CDP API surface the provider consumes. The production
// implementation carries CDP API-key JWT authentication (see client.go).
type Client interface {
	// CreateSessionToken mints the single-use session token for one checkout
	// session, bound to the delivery address and asset.
	CreateSessionToken(ctx context.Context, req SessionTokenRequest) (string, error)
	// BuyTransactionsByPartnerUser lists onramp transactions correlated with
	// one partnerUserId, newest first. Empty means the buyer has not
	// completed the hosted checkout yet.
	BuyTransactionsByPartnerUser(ctx context.Context, partnerUserID string) ([]Transaction, error)
}

// SessionTokenRequest is the subset of the Session Token API we use.
type SessionTokenRequest struct {
	// Address and Networks form the single addresses entry: where purchased
	// funds are delivered. Phase 1 points this at the frozen funding target.
	Address  string
	Networks []string
	// Assets restricts the purchasable assets for the session.
	Assets []string
	// PartnerUserID correlates the session with our purchase record.
	PartnerUserID string
}

// Transaction is the subset of the Onramp transaction we consume.
type Transaction struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

// Provider implements contracts.OnrampProvider.
type Provider struct {
	cfg Config
}

// New validates the config and builds the provider.
func New(cfg Config) (*Provider, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("cdp: a Client is required (session tokens are mandatory)")
	}
	if cfg.PayBaseURL == "" {
		cfg.PayBaseURL = "https://pay.coinbase.com"
	}
	if cfg.Disclosure == "" {
		cfg.Disclosure = "You are buying crypto from Coinbase; its fees, KYC, and reversals are between you and Coinbase."
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
		// The session token binds delivery to the app-specified address,
		// which for phase 1 is the frozen funding target itself.
		DeliverToTarget: true,
		FiatCurrencies:  fiat,
	}, nil
}

// Quote implements contracts.OnrampProvider.
//
// TODO(sandbox): wire the Buy Quote API once sandbox keys exist. Until then
// quoting is fail-closed; the checkout still works because the hosted page
// shows Coinbase's own price before the buyer commits (the RFC only requires
// disclosure before commitment, which the hosted checkout provides).
func (p *Provider) Quote(_ context.Context, req contracts.OnrampQuoteRequest) (contracts.OnrampQuote, error) {
	if err := req.Validate(); err != nil {
		return contracts.OnrampQuote{}, err
	}
	if _, ok := p.cfg.Rails[req.RailID]; !ok {
		return contracts.OnrampQuote{}, contracts.ErrOnrampCapabilityClosed
	}
	return contracts.OnrampQuote{}, fmt.Errorf("cdp: buy quote is not wired yet; the hosted checkout displays pricing")
}

// InitiatePurchase implements contracts.OnrampProvider. The purchase id is
// deterministic on (AttemptID, IdempotencyKey) so a resume correlates with
// the same provider-side history; the session token itself is single-use and
// short-lived, so a resume after expiry legitimately mints a fresh URL for
// the SAME purchase — that is a new session, not a second onramp order.
func (p *Provider) InitiatePurchase(ctx context.Context, req contracts.OnrampPurchaseRequest) (contracts.OnrampPurchase, error) {
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
	partnerUserID := PurchasePartnerUserID(req.AttemptID, req.IdempotencyKey)

	token, err := p.cfg.Client.CreateSessionToken(ctx, SessionTokenRequest{
		Address:       address,
		Networks:      []string{rail.Network},
		Assets:        []string{rail.AssetSymbol},
		PartnerUserID: partnerUserID,
	})
	if err != nil {
		return contracts.OnrampPurchase{}, fmt.Errorf("cdp: create session token: %w", err)
	}

	query := url.Values{}
	query.Set("sessionToken", token)
	query.Set("defaultAsset", rail.AssetSymbol)
	query.Set("defaultNetwork", rail.Network)
	// One-click-buy: preset the RECEIVE amount to the frozen settlement
	// amount so the buyer pays fiat = amount + Coinbase's fees and the
	// target receives exactly what the attempt froze. (Phase-1 gate
	// question 2 confirms this mode.)
	query.Set("presetCryptoAmount", req.SettlementAmount)
	query.Set("fiatCurrency", strings.ToUpper(req.FiatCurrency))
	query.Set("partnerUserId", partnerUserID)
	if p.cfg.RedirectURL != "" {
		query.Set("redirectUrl", p.cfg.RedirectURL)
	}

	return contracts.OnrampPurchase{
		ProviderID:           ProviderID,
		OnrampOrderID:        partnerUserID,
		Status:               contracts.OnrampStatusAwaitingPayment,
		BuyerActionURL:       p.cfg.PayBaseURL + "/buy/select-asset?" + query.Encode(),
		DeliveryTarget:       req.DeliveryTarget,
		DeliverToBuyerWallet: req.DeliverToBuyerWallet,
		BuyerWalletAddress:   req.BuyerWalletAddress,
		Disclosure:           p.cfg.Disclosure,
	}, nil
}

// PurchaseStatus implements contracts.OnrampProvider by polling the Onramp
// transactions API. No transaction yet means the buyer has not finished the
// hosted checkout: still awaiting payment, not failed.
func (p *Provider) PurchaseStatus(ctx context.Context, onrampOrderID string) (contracts.OnrampPurchase, error) {
	if strings.TrimSpace(onrampOrderID) == "" {
		return contracts.OnrampPurchase{}, fmt.Errorf("cdp: onramp order id is required")
	}
	txs, err := p.cfg.Client.BuyTransactionsByPartnerUser(ctx, onrampOrderID)
	if err != nil {
		return contracts.OnrampPurchase{}, fmt.Errorf("cdp: transaction status: %w", err)
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

// PurchasePartnerUserID deterministically correlates one attempt-scoped
// purchase with Coinbase's partnerUserId, so initiate retries and status
// polls agree without provider-side state.
func PurchasePartnerUserID(attemptID, idempotencyKey string) string {
	return "cdp-" + attemptID + "-" + idempotencyKey
}

// mapStatus translates Onramp transaction statuses into the provider-neutral
// lifecycle. Unknown statuses map to processing rather than failing the poll.
func mapStatus(s string) contracts.OnrampStatus {
	switch s {
	case "ONRAMP_TRANSACTION_STATUS_CREATED":
		return contracts.OnrampStatusAwaitingPayment
	case "ONRAMP_TRANSACTION_STATUS_IN_PROGRESS":
		return contracts.OnrampStatusProcessing
	case "ONRAMP_TRANSACTION_STATUS_SUCCESS":
		return contracts.OnrampStatusDelivered
	case "ONRAMP_TRANSACTION_STATUS_FAILED":
		return contracts.OnrampStatusFailed
	default:
		return contracts.OnrampStatusProcessing
	}
}

// ParseRails parses the deployment rail mapping
// "railID=ASSET:network[:FIAT1|FIAT2],...". Malformed entries error out
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
			return nil, fmt.Errorf("cdp: malformed rail entry %q (want railID=ASSET:network[:FIAT|FIAT])", entry)
		}
		parts := strings.Split(spec, ":")
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("cdp: rail %q needs ASSET:network, got %q", railID, spec)
		}
		rail := Rail{AssetSymbol: strings.TrimSpace(parts[0]), Network: strings.TrimSpace(parts[1])}
		if len(parts) > 2 && parts[2] != "" {
			for _, fiat := range strings.Split(parts[2], "|") {
				if fiat = strings.TrimSpace(fiat); fiat != "" {
					rail.FiatCurrencies = append(rail.FiatCurrencies, strings.ToUpper(fiat))
				}
			}
		}
		rails[railID] = rail
	}
	if len(rails) == 0 {
		return nil, fmt.Errorf("cdp: no rails configured")
	}
	return rails, nil
}

var _ contracts.OnrampProvider = (*Provider)(nil)
