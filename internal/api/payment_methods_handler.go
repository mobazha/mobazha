package api

import (
	"net/http"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/edition"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// handleGETPaymentMethods returns the seller's accepted payment methods
// (both crypto currencies and fiat providers) as a single public endpoint.
//
// GET /v1/payment-methods/{peerID}
func (g *Gateway) handleGETPaymentMethods(w http.ResponseWriter, r *http.Request) {
	capabilityProvider, capabilityDecisionAvailable := paymentCapabilityDecisionProvider(r)
	raSvc := getReceivingAccountService(r)
	accounts, err := raSvc.List()
	if err != nil {
		log.Warningf("Failed to get receiving accounts: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load payment methods")
		return
	}

	seen := make(map[string]bool)
	var crypto []string
	for _, acct := range accounts {
		if !acct.IsActive {
			continue
		}
		for _, c := range acct.AcceptedCurrencies() {
			if !iwallet.IsPaymentCoinEnabledForPolicy(c, g.editionPolicy) {
				continue
			}
			coin := iwallet.CoinType(c)
			if paymentCoinRequiresModule(acct.ChainType) && (!capabilityDecisionAvailable ||
				!capabilityProvider.DecidePaymentCapability(r.Context(), distribution.PaymentCapabilityRequest{
					Rail: distribution.PaymentRailEscrow, Network: acct.ChainType,
					Asset: coin, Operation: distribution.PaymentOperationSetup,
				}).Allowed()) {
				continue
			}
			if !seen[c] {
				seen[c] = true
				crypto = append(crypto, c)
			}
		}
	}

	// Trusted distributions may advertise direct-payment coins that do not
	// use ReceivingAccount rows. The policy owns availability and literals.
	var additionalCoins []iwallet.CoinType
	if g.config != nil && g.config.GuestPaymentPolicy != nil {
		additionalCoins = g.config.GuestPaymentPolicy.AdvertisedPaymentCoins()
	}
	for _, coin := range additionalCoins {
		c := string(coin)
		if !iwallet.IsPaymentCoinEnabledForPolicy(c, g.editionPolicy) {
			continue
		}
		coinInfo, coinErr := iwallet.CoinInfoFromCoinType(coin)
		if coinErr != nil || !capabilityDecisionAvailable || !capabilityProvider.DecidePaymentCapability(
			r.Context(), distribution.PaymentCapabilityRequest{
				Rail: distribution.PaymentRailDirectObserved, Network: coinInfo.Chain,
				Asset: coin, Operation: distribution.PaymentOperationSetup,
			},
		).Allowed() {
			continue
		}
		if !seen[c] {
			seen[c] = true
			crypto = append(crypto, c)
		}
	}

	fiat := make([]contracts.ProviderInfo, 0)
	if svc, ok := getFiatService(r); ok {
		providers, fiatErr := svc.EnabledProviders(r.Context())
		if fiatErr == nil {
			policy := g.editionPolicy
			if policy == nil {
				policy = edition.CurrentPolicy()
			}
			for _, provider := range providers {
				providerNetwork := iwallet.ChainType("fiat:" + strings.ToLower(strings.TrimSpace(provider.ProviderID)))
				if policy.AllowsPaymentMethod(edition.PaymentMethod{
					ID:   provider.ProviderID,
					Kind: "fiat",
					Flow: "provider-session",
				}) && capabilityDecisionAvailable && capabilityProvider.DecidePaymentCapability(
					r.Context(), distribution.PaymentCapabilityRequest{
						Rail: distribution.PaymentRailProviderSession, Network: providerNetwork,
						Asset: distribution.PaymentAssetAny, Operation: distribution.PaymentOperationSetup,
					},
				).Allowed() {
					fiat = append(fiat, provider)
				}
			}
		}
	}
	if crypto == nil {
		crypto = []string{}
	}

	responsePkg.Success(w, map[string]any{
		"crypto": crypto,
		"fiat":   fiat,
	})
}

func paymentCapabilityDecisionProvider(r *http.Request) (distribution.PaymentCapabilityDecisionProvider, bool) {
	provider, ok := getNodeService(r).(distribution.PaymentCapabilityDecisionProvider)
	if !ok || provider == nil {
		return nil, false
	}
	return provider, true
}

func paymentCoinRequiresModule(chain iwallet.ChainType) bool {
	if chain == iwallet.ChainSolana {
		return true
	}
	native, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		return false
	}
	coin, err := iwallet.CoinInfoFromCoinType(native)
	return err == nil && coin.IsEthTypeChain()
}
