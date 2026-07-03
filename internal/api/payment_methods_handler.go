package api

import (
	"net/http"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/edition"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// handleGETPaymentMethods returns the seller's accepted payment methods
// (both crypto currencies and fiat providers) as a single public endpoint.
//
// GET /v1/payment-methods/{peerID}
func (g *Gateway) handleGETPaymentMethods(w http.ResponseWriter, r *http.Request) {
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
				if policy.AllowsPaymentMethod(edition.PaymentMethod{
					ID:   provider.ProviderID,
					Kind: "fiat",
					Flow: "provider-session",
				}) {
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
