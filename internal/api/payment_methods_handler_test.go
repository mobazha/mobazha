//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/edition"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type paymentProjectionPolicyStub struct {
	coins []iwallet.CoinType
}

func (paymentProjectionPolicyStub) SupportsGuestPaymentCoin(iwallet.CoinType) bool { return false }
func (paymentProjectionPolicyStub) ValidateGuestPaymentCoin(iwallet.CoinType) error {
	return nil
}
func (policy paymentProjectionPolicyStub) AdvertisedPaymentCoins() []iwallet.CoinType {
	return append([]iwallet.CoinType(nil), policy.coins...)
}
func (paymentProjectionPolicyStub) ValidateCrossCurrencyCheckout(string, string) error {
	return nil
}

func TestHandleGETPaymentMethods_IncludesDistributionProjection(t *testing.T) {
	coin := iwallet.CoinType("crypto:external_payment:mainnet:native")
	node := &mockNode{raListFunc: func() ([]models.ReceivingAccount, error) { return nil, nil }}
	request := httptest.NewRequest(http.MethodGet, "/v1/payment-methods/seller", nil)
	request = request.WithContext(context.WithValue(request.Context(), nodeContextKey, contracts.NodeService(node)))
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	(&Gateway{
		config:        &GatewayConfig{GuestPaymentPolicy: paymentProjectionPolicyStub{coins: []iwallet.CoinType{coin}}},
		editionPolicy: policy,
	}).handleGETPaymentMethods(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Crypto []string `json:"crypto"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Crypto) != 1 || response.Data.Crypto[0] != string(coin) {
		t.Fatalf("crypto = %#v, want %s", response.Data.Crypto, coin)
	}
}

func TestHandleGETPaymentMethods_FiltersProductDisabledZEC(t *testing.T) {
	zecAccount := models.ReceivingAccount{
		ChainType: iwallet.ChainZCash,
		IsActive:  true,
	}
	if err := zecAccount.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL}); err != nil {
		t.Fatal(err)
	}
	bchAccount := models.ReceivingAccount{
		ChainType: iwallet.ChainBitcoinCash,
		IsActive:  true,
	}
	if err := bchAccount.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL}); err != nil {
		t.Fatal(err)
	}

	node := &mockNode{
		raListFunc: func() ([]models.ReceivingAccount, error) {
			return []models.ReceivingAccount{zecAccount, bchAccount}, nil
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/payment-methods/seller", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node)))

	w := httptest.NewRecorder()
	(&Gateway{}).handleGETPaymentMethods(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var res struct {
		Data struct {
			Crypto []string `json:"crypto"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data.Crypto) != 1 || res.Data.Crypto[0] != string(iwallet.ChainBitcoinCash) {
		t.Fatalf("crypto = %#v, want only BCH", res.Data.Crypto)
	}
}

func TestHandleGETPaymentMethods_CommunityAllowsTransparentZEC(t *testing.T) {
	zecAccount := models.ReceivingAccount{ChainType: iwallet.ChainZCash, IsActive: true}
	if err := zecAccount.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL}); err != nil {
		t.Fatal(err)
	}
	node := &mockNode{raListFunc: func() ([]models.ReceivingAccount, error) {
		return []models.ReceivingAccount{zecAccount}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/payment-methods/seller", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node)))
	policy, err := edition.ResolvePolicy(edition.CommunityName)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	(&Gateway{editionPolicy: policy}).handleGETPaymentMethods(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Crypto []string `json:"crypto"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data.Crypto) != 1 || res.Data.Crypto[0] != string(iwallet.ChainZCash) {
		t.Fatalf("crypto = %#v, want ZEC", res.Data.Crypto)
	}
}

func TestHandleGETPaymentMethods_FiltersFiatByCapability(t *testing.T) {
	node := &mockNode{raListFunc: func() ([]models.ReceivingAccount, error) {
		return nil, nil
	}}
	fiatService := &mockFiatService{enabledResult: []contracts.ProviderInfo{
		{ProviderID: "stripe", Status: "active", AccountID: "acct_1"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/payment-methods/seller", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(
		&mockNodeWithFiat{NodeService: node, fiatSvc: fiatService},
	)))

	manifest, err := edition.CommunityManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Edition = "self-hosted"
	policy, err := edition.NewPolicy(manifest)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	(&Gateway{editionPolicy: policy}).handleGETPaymentMethods(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Fiat []contracts.ProviderInfo `json:"fiat"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data.Fiat) != 0 {
		t.Fatalf("fiat = %#v, want capability-filtered empty list", res.Data.Fiat)
	}
}
