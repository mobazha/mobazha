package adapters_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tronchain "github.com/mobazha/mobazha/internal/chains/tron"
	"github.com/mobazha/mobazha/internal/payment/adapters"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTRX    = "crypto:tron:mainnet:native"
	testETHEVM = "crypto:eip155:1:native"
)

type tronMultiwallet struct {
	wallet iwallet.Wallet
}

func (m *tronMultiwallet) WalletForCurrencyCode(string) (iwallet.Wallet, error) { return m.wallet, nil }
func (m *tronMultiwallet) SupportedChains() []iwallet.ChainType                 { return nil }
func (m *tronMultiwallet) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	if chain == iwallet.ChainTRON && m.wallet != nil {
		return m.wallet, true
	}
	return nil, false
}
func (m *tronMultiwallet) Start() error { return nil }
func (m *tronMultiwallet) Close() error { return nil }

func TestTRONReceipt_NonTRONCoinNoop(t *testing.T) {
	v := adapters.NewTRONReceiptVerifier(&tronMultiwallet{})
	err := v.VerifyTransactionReceipt(context.Background(), testETHEVM, "0xabc")
	require.NoError(t, err, "non-TRON coin should be noop")
}

func TestTRONReceipt_NoWalletReturnsError(t *testing.T) {
	v := adapters.NewTRONReceiptVerifier(&tronMultiwallet{wallet: nil})
	err := v.VerifyTransactionReceipt(context.Background(), testTRX, "abc123")
	require.Error(t, err, "missing TRON wallet should return config error, not silently pass")
	assert.Contains(t, err.Error(), "misconfigured")
}

func TestRegistry_DispatchesToTRON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(&tronchain.TronTransactionInfo{
			ID:      "abc123",
			Receipt: tronchain.TronReceipt{Result: "REVERT"},
		})
	}))
	defer server.Close()

	client := tronchain.NewTronClient([]string{server.URL}, tronchain.RetryConfig{})

	mockVerifier := &directTronVerifier{client: client}

	registry := adapters.NewReceiptVerifierRegistryFromMap(map[iwallet.ChainType]contracts.ReceiptVerifier{
		iwallet.ChainTRON: mockVerifier,
	})

	err := registry.VerifyTransactionReceipt(context.Background(), testTRX, "abc123")
	assert.ErrorIs(t, err, payment.ErrTransactionReverted)

	err = registry.VerifyTransactionReceipt(context.Background(), "BTC", "txhash")
	require.NoError(t, err, "BTC should be noop when only TRON registered")
}

// directTronVerifier bypasses the wallet layer for registry dispatch testing.
type directTronVerifier struct {
	client *tronchain.TronClient
}

func (v *directTronVerifier) VerifyTransactionReceipt(ctx context.Context, _ string, txHash string) error {
	info, err := v.client.GetTransactionInfo(ctx, txHash)
	if err != nil {
		return nil
	}
	if info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS" {
		return payment.ErrTransactionReverted
	}
	return nil
}

func (v *directTronVerifier) WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error {
	return v.VerifyTransactionReceipt(ctx, coinCode, txHash)
}

// Regression: BSC/Polygon/Base/Conflux must hit the EVM verifier, not noop.
func TestRegistry_EVMChainsAllDispatched(t *testing.T) {
	var called []string
	spy := &spyReceiptVerifier{calls: &called}

	registry := adapters.NewReceiptVerifierRegistryFromMap(map[iwallet.ChainType]contracts.ReceiptVerifier{
		iwallet.ChainEthereum: spy,
		iwallet.ChainBSC:      spy,
		iwallet.ChainPolygon:  spy,
		iwallet.ChainBase:     spy,
		iwallet.ChainConflux:  spy,
	})

	coins := []string{
		"crypto:eip155:1:native",    // ETH
		"crypto:eip155:56:native",   // BSC
		"crypto:eip155:137:native",  // Polygon
		"crypto:eip155:8453:native", // Base
		"crypto:eip155:1030:native", // Conflux
	}
	for _, coin := range coins {
		_ = registry.VerifyTransactionReceipt(context.Background(), coin, "0xtx")
	}
	assert.Equal(t, len(coins), len(called), "all EVM chains must dispatch to verifier")
}

type spyReceiptVerifier struct{ calls *[]string }

func (s *spyReceiptVerifier) VerifyTransactionReceipt(_ context.Context, coinCode string, _ string) error {
	*s.calls = append(*s.calls, coinCode)
	return nil
}
func (s *spyReceiptVerifier) WaitAndVerifyReceipt(_ context.Context, coinCode string, _ string) error {
	*s.calls = append(*s.calls, coinCode)
	return nil
}
