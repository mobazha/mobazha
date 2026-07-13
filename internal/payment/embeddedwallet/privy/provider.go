// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package privy

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// ProviderID is the Privy module identifier.
const ProviderID = "privy"

// Buyer-authorization schemes this adapter understands.
const (
	// SchemeUserJWT is the production path: the token is a buyer-obtained Privy
	// access credential authorizing a signature on the buyer's own wallet. It
	// requires the Casdoor->Privy identity link and a delegated-signing session
	// (batch 3) and is not wired yet.
	SchemeUserJWT = "privy-user-jwt"

	// SchemeServerWalletFixture is the Phase 0 reproduction path: signing an
	// application-owned wallet with app credentials. It is NOT a production
	// custody model (RFC-0012 Proposal 2 forbids a standing platform signer for
	// real funds) and is refused unless explicitly enabled in Config.
	SchemeServerWalletFixture = "privy-server-fixture"
)

// ErrServerWalletFixtureDisabled is returned when the gated dev fixture path is
// used on a provider that did not opt into it.
var ErrServerWalletFixtureDisabled = errors.New("privy: server-wallet fixture is disabled; it is a non-production Phase 0 proof, not a custody model")

// ErrProductionAuthNotWired marks the production buyer-JWT signing path, which
// depends on the Casdoor->Privy identity link (batch 3) not yet implemented.
var ErrProductionAuthNotWired = errors.New("privy: production buyer-authorized signing is not wired yet (requires Casdoor->Privy identity link)")

// Config configures the Privy provider.
type Config struct {
	AppID     string
	AppSecret string
	BaseURL   string
	Client    *Client

	// AllowServerWalletFixture opts into the gated, non-production Phase 0
	// reproduction path. Leave false in any real deployment.
	AllowServerWalletFixture bool

	// JWKSURL is the app's Privy access-token JWKS endpoint (e.g.
	// https://auth.privy.io/api/v1/apps/{app_id}/jwks.json). Setting it wires the
	// Casdoor->Privy identity link and enables the production buyer-authorized
	// paths, tolerating key rotation. This is the preferred production wiring.
	JWKSURL string

	// VerificationKeyPEM is an offline alternative to JWKSURL: the app's Privy
	// access-token verification key (PEM-encoded ECDSA public key, from the Privy
	// dashboard). Use it only when a JWKS fetch is undesirable; it does not
	// tolerate key rotation. JWKSURL takes precedence when both are set.
	VerificationKeyPEM string

	// HTTPClient overrides the client used for JWKS fetches (tests inject a fake).
	HTTPClient *http.Client

	// Verifier overrides the identity verifier (tests inject a fake user
	// directory). When nil and VerificationKeyPEM is set, a REST-backed verifier
	// is constructed.
	Verifier *IdentityVerifier

	// Capabilities is the fail-closed capability surface keyed by rail id. It is
	// empty by default: the RFC-0012 Proposal 6 capability gate has not closed
	// for Privy, so the adapter advertises nothing until an operator asserts a
	// proven rail here.
	Capabilities map[string]contracts.EmbeddedWalletCapabilities
}

// Provider implements contracts.EmbeddedWalletProvider over Privy.
type Provider struct {
	client        *Client
	allowFixture  bool
	verifier      *IdentityVerifier
	caps          map[string]contracts.EmbeddedWalletCapabilities
	mu            sync.Mutex
	fixtureWallet map[string]contracts.EmbeddedWallet // key: rail|subject
}

// New builds a Privy provider from Config. It requires either a preconstructed
// Client or an AppID/AppSecret pair.
func New(cfg Config) (*Provider, error) {
	client := cfg.Client
	if client == nil {
		if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
			return nil, fmt.Errorf("privy: Config requires a Client or AppID+AppSecret")
		}
		client = NewClient(cfg.AppID, cfg.AppSecret, cfg.BaseURL, nil)
	}
	caps := make(map[string]contracts.EmbeddedWalletCapabilities, len(cfg.Capabilities))
	for rail, c := range cfg.Capabilities {
		caps[rail] = c
	}

	verifier := cfg.Verifier
	if verifier == nil {
		appID := cfg.AppID
		if appID == "" {
			appID = client.appID
		}
		dir := restUserDirectory{client: client}
		switch {
		case strings.TrimSpace(cfg.JWKSURL) != "":
			v, err := newIdentityVerifierJWKS(appID, cfg.JWKSURL, cfg.HTTPClient, dir)
			if err != nil {
				return nil, err
			}
			verifier = v
		case strings.TrimSpace(cfg.VerificationKeyPEM) != "":
			v, err := newIdentityVerifier(appID, cfg.VerificationKeyPEM, dir)
			if err != nil {
				return nil, err
			}
			verifier = v
		}
	}

	return &Provider{
		client:        client,
		allowFixture:  cfg.AllowServerWalletFixture,
		verifier:      verifier,
		caps:          caps,
		fixtureWallet: make(map[string]contracts.EmbeddedWallet),
	}, nil
}

// ProviderID implements contracts.EmbeddedWalletProvider.
func (p *Provider) ProviderID() string { return ProviderID }

// Capabilities returns the fail-closed capability surface for a rail. Privy's
// default is all-closed until an operator asserts a proven rail in Config.
func (p *Provider) Capabilities(_ context.Context, railID string) (contracts.EmbeddedWalletCapabilities, error) {
	if c, ok := p.caps[railID]; ok {
		return c, nil
	}
	return contracts.EmbeddedWalletCapabilities{RailID: railID}, nil
}

// EnsureWallet returns the buyer's wallet. In the gated fixture mode it creates
// and caches an app-owned server wallet (Phase 0 proof). The production path
// (buyer-owned wallet keyed to the buyer's Privy identity) is not wired yet.
func (p *Provider) EnsureWallet(ctx context.Context, req contracts.EnsureWalletRequest) (contracts.EmbeddedWallet, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWallet{}, err
	}

	// Production: resolve the buyer's own embedded wallet through the
	// Casdoor->Privy identity link. Signs/creates nothing here — the wallet is
	// provisioned by the buyer's client at custom-auth login; we resolve which
	// address is theirs to admit as an escrow co-owner.
	if p.verifier != nil {
		family := chainFamilyForRail(req.RailID)
		w, err := p.verifier.ResolveBuyerWallet(ctx, req.Buyer.Subject, family)
		if err != nil {
			return contracts.EmbeddedWallet{}, err
		}
		return contracts.EmbeddedWallet{
			ProviderID:  ProviderID,
			WalletID:    w.ID,
			Address:     w.Address,
			RailID:      req.RailID,
			ChainFamily: family,
		}, nil
	}

	if !p.allowFixture {
		return contracts.EmbeddedWallet{}, ErrProductionAuthNotWired
	}

	key := req.RailID + "|" + req.Buyer.Subject
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.fixtureWallet[key]; ok {
		return w, nil
	}
	id, address, err := p.client.CreateServerWallet(ctx, "ethereum")
	if err != nil {
		return contracts.EmbeddedWallet{}, err
	}
	w := contracts.EmbeddedWallet{
		ProviderID:  ProviderID,
		WalletID:    id,
		Address:     address,
		RailID:      req.RailID,
		ChainFamily: contracts.ChainFamilyEVM,
	}
	p.fixtureWallet[key] = w
	return w, nil
}

// SignTypedData produces one structured signature. Contract guards (structured
// payload, buyer authorization present) run first via req.Validate. The
// authorization scheme then selects the production path (not wired) or the
// gated fixture path.
func (p *Provider) SignTypedData(ctx context.Context, req contracts.EmbeddedWalletSignRequest) (contracts.EmbeddedWalletSignature, error) {
	if err := req.Validate(); err != nil {
		return contracts.EmbeddedWalletSignature{}, err
	}
	if req.Wallet.ChainFamily != contracts.ChainFamilyEVM {
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("privy: unsupported chain family %q", req.Wallet.ChainFamily)
	}

	switch req.Authorization.Scheme {
	case SchemeUserJWT:
		if p.verifier == nil {
			return contracts.EmbeddedWalletSignature{}, ErrProductionAuthNotWired
		}
		// Prove the buyer's access token authorizes this wallet before signing.
		// RFC-0012 Proposal 2: no signature is produced on platform authority
		// alone; the buyer's token is what carries consent.
		if _, err := p.verifier.AuthorizeSigner(ctx, req.Authorization.Token, "", req.Wallet.Address); err != nil {
			return contracts.EmbeddedWalletSignature{}, err
		}
		sigHex, err := p.client.SignTypedDataV4WithUserWallet(ctx, req.Wallet.WalletID, req.Authorization.Token, req.Payload.Document)
		if err != nil {
			return contracts.EmbeddedWalletSignature{}, err
		}
		raw, err := decodeHexSignature(sigHex)
		if err != nil {
			return contracts.EmbeddedWalletSignature{}, err
		}
		return contracts.EmbeddedWalletSignature{Signer: req.Wallet.Address, Signature: raw}, nil
	case SchemeServerWalletFixture:
		if !p.allowFixture {
			return contracts.EmbeddedWalletSignature{}, ErrServerWalletFixtureDisabled
		}
		sigHex, err := p.client.SignTypedDataV4WithServerWallet(ctx, req.Wallet.WalletID, req.Payload.Document)
		if err != nil {
			return contracts.EmbeddedWalletSignature{}, err
		}
		raw, err := decodeHexSignature(sigHex)
		if err != nil {
			return contracts.EmbeddedWalletSignature{}, err
		}
		return contracts.EmbeddedWalletSignature{Signer: req.Wallet.Address, Signature: raw}, nil
	default:
		return contracts.EmbeddedWalletSignature{}, fmt.Errorf("%w: unknown authorization scheme %q", contracts.ErrEmbeddedWalletNoBuyerAuthorization, req.Authorization.Scheme)
	}
}

// chainFamilyForRail maps a rail id to its structured-signing chain family.
// Rails are network-qualified (e.g. "crypto:eip155:1:native",
// "crypto:solana:mainnet:..."); anything Solana-qualified is the Solana family,
// otherwise EVM.
func chainFamilyForRail(railID string) contracts.ChainFamily {
	if strings.Contains(strings.ToLower(railID), "solana") {
		return contracts.ChainFamilySolana
	}
	return contracts.ChainFamilyEVM
}

func decodeHexSignature(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("privy: signature is not valid hex: %w", err)
	}
	return raw, nil
}

var _ contracts.EmbeddedWalletProvider = (*Provider)(nil)
