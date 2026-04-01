package api

import (
	"net/http"

	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

// handleGETPaymentMethods returns the seller's accepted payment methods
// (both crypto currencies and fiat providers) as a single public endpoint.
//
// GET /v1/payment-methods/{peerID}
func (g *Gateway) handleGETPaymentMethods(w http.ResponseWriter, r *http.Request) {
	walletSvc := getWalletService(r)
	accounts, err := walletSvc.GetReceivingAccounts()
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
			if !seen[c] {
				seen[c] = true
				crypto = append(crypto, c)
			}
		}
	}

	type fiatEntry struct{}
	var fiat any
	if svc, ok := getFiatService(r); ok {
		providers, fiatErr := svc.EnabledProviders(r.Context())
		if fiatErr == nil {
			fiat = providers
		}
	}
	if fiat == nil {
		fiat = []fiatEntry{}
	}
	if crypto == nil {
		crypto = []string{}
	}

	responsePkg.Success(w, map[string]any{
		"crypto": crypto,
		"fiat":   fiat,
	})
}
