package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mobazha/mobazha/internal/embedded/frontend"
	"github.com/mobazha/mobazha/pkg/edition"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterPaymentCapabilitiesCommunityAllowlist(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.CommunityName)
	require.NoError(t, err)

	methods := []frontend.PaymentCapability{
		{ID: "BTC", Kind: "crypto", Flow: "address-transfer"},
		{ID: "BCH", Kind: "crypto", Flow: "address-transfer"},
		{ID: "LTC", Kind: "crypto", Flow: "address-transfer"},
		{ID: "ZEC", Kind: "crypto", Flow: "address-transfer", AddressMode: "transparent"},
		{ID: "XMR", Kind: "crypto", Flow: "address-transfer"},
		{ID: "ETH", Kind: "crypto", Flow: "external-wallet"},
		{ID: "stripe", Kind: "fiat", Flow: "provider-session"},
	}

	assert.Equal(t, methods[:3], filterPaymentCapabilities(methods, policy))
}

func TestFilterPaymentCapabilitiesFullComposition(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	require.NoError(t, err)

	methods := []frontend.PaymentCapability{{ID: "SOL", Kind: "crypto", Flow: "external-wallet"}}
	assert.Equal(t, methods, filterPaymentCapabilities(methods, policy))
}

func TestFilterPaymentCapabilitiesMissingPolicyFailsClosed(t *testing.T) {
	methods := []frontend.PaymentCapability{{ID: "BTC", Kind: "crypto", Flow: "address-transfer"}}
	assert.Empty(t, filterPaymentCapabilities(methods, nil))
}

func TestActiveCryptoPaymentCapabilitiesIntersectsWalletAndRegistry(t *testing.T) {
	methods := activeCryptoPaymentCapabilities(
		[]iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainEthereum, iwallet.ChainZCash, iwallet.ChainBitcoin},
		[]iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainZCash, iwallet.ChainSolana},
	)
	require.Len(t, methods, 2)
	assert.Equal(t, "BTC", methods[0].ID)
	assert.Equal(t, "ZEC", methods[1].ID)
	assert.Equal(t, "transparent", methods[1].AddressMode)
}

func TestActiveCryptoPaymentCapabilitiesMissingRegistryFailsClosed(t *testing.T) {
	methods := activeCryptoPaymentCapabilities(
		[]iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainEthereum},
		nil,
	)
	assert.Empty(t, methods)
}

func TestNewGatewayRejectsUnknownEdition(t *testing.T) {
	_, err := NewGateway(nil, &GatewayConfig{Edition: "commuinty"})
	require.ErrorContains(t, err, "unknown Mobazha edition")
}

func TestCommunityGatewayDoesNotProxyPlatformControlPlane(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	gateway, err := NewGateway(nil, &GatewayConfig{
		Listener:   listener,
		PublicOnly: true,
		Edition:    edition.CommunityName,
		SaaSAPIURL: upstream.URL,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, gateway.Close()) })

	recorder := httptest.NewRecorder()
	gateway.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/platform/v1/server/info", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Zero(t, upstreamCalls.Load())
}
