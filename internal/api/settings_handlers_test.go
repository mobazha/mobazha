package api

import (
	"net/http"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestSettingsHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get config",
			path:   "/v1/config",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.identityFunc = func() peer.ID {
					p, _ := peer.Decode("12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi")
					return p
				}
				n.usingTestnetFunc = func() bool {
					return true
				}
				n.usingTorFunc = func() bool {
					return true
				}
			n.multiwalletFunc = func() contracts.WalletOperator {
				m := chains.Multiwallet{
					iwallet.ChainBitcoin: nil,
				}
				return &m
			}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				n := nodeConfig{
					PeerID:  "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
					Testnet: true,
					Tor:     true,
					Wallets: []string{"BTC"},
				}
				return wrapDataInEnvelope(&n)
			},
		},
		{
			name:   "Put user preferences",
			path:   "/v1/preferences",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.getUserPreferencesFunc = func() (*models.UserPreferences, error) {
					return &models.UserPreferences{
						RefundPolicy: "asdf1",
					}, nil
				}
				n.saveUserPreferencesFunc = func(prefs *models.UserPreferences, done chan struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"RefundPolicy": "asdf"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(map[string]interface{}{})
			},
		},
		{
			name:   "Put user preferences bad request",
			path:   "/v1/preferences",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.saveUserPreferencesFunc = func(prefs *models.UserPreferences, done chan struct{}) error {
					return coreiface.ErrBadRequest
				}
			},
			body:       []byte(`{"RefundPolicy": "asdf"}`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Get user preferences",
			path:   "/v1/preferences",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getUserPreferencesFunc = func() (*models.UserPreferences, error) {
					return &models.UserPreferences{
						RefundPolicy: "asdf",
					}, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(&models.UserPreferences{RefundPolicy: "asdf", UserAgent: version.UserAgent()})
			},
		},
		{
			name:   "Get exchange rates",
			path:   "/v1/exchange-rates",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getAllRatesFunc = func(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
					erp, err := wallet.NewMockExchangeRates()
					if err != nil {
						return nil, err
					}
					return erp.GetAllRates(base, breakCache)
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				erp, err := wallet.NewMockExchangeRates()
				if err != nil {
					return nil, err
				}
				rates, err := erp.GetAllRates(wallet.ReserveCurrency, false)
				if err != nil {
					return nil, err
				}
				return wrapDataInEnvelope(rates)
			},
		},
	})
}
