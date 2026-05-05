//go:build !private_distribution

package order

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEscrowRelay struct {
	called bool
	txHash string
	err    error
}

func (m *mockEscrowRelay) GetPayoutAddress(string) (iwallet.Address, error) {
	return iwallet.NewAddress("ignored", iwallet.CtMock), nil
}

func (m *mockEscrowRelay) ReleaseCancelableFunds(*models.Order, string) (iwallet.TransactionID, string, error) {
	return "", "", nil
}

func (m *mockEscrowRelay) ReleaseFromCancelableAddressWithParams(*models.Order, contracts.ReleaseFromCancelableParams) (iwallet.Tx, *iwallet.Transaction, error) {
	return nil, nil, nil
}

func (m *mockEscrowRelay) CancelPartialPayment(string) (string, uint64, error) {
	return "", 0, nil
}

func (m *mockEscrowRelay) RelayInstructions(_ string, _ iwallet.CoinType, _ any) (string, error) {
	m.called = true
	return m.txHash, m.err
}

func TestNormalizeFiatPaymentCoin_RejectsMissingProviderSegment(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType("fiat:USD"),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
}

func TestNormalizeFiatPaymentCoin_RejectsLegacyAlias(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType("FIAT_STRIPE"),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
}

func TestNormalizeFiatPaymentCoin_RejectsEmptyCoinWithoutProviderHint(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType(""),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fiat provider is empty")
}

func TestNormalizeFiatPaymentCoin_UsesProviderHintWhenCoinEmpty(t *testing.T) {
	coin, err := normalizeFiatPaymentCoin(
		iwallet.CoinType(""),
		pb.PaymentSent_FIAT,
		"paypal",
		"usd",
	)
	require.NoError(t, err)
	assert.Equal(t, "fiat:paypal:USD", string(coin))
}

func TestRelayOrDirect_FiatBypassesEscrowRelay(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	escrow := &mockEscrowRelay{}
	svc.escrow = escrow

	directCalled := false
	relayedCalled := false

	err := svc.relayOrDirect(
		models.OrderID("order-fiat-relay"),
		"decline",
		iwallet.CoinType("fiat:paypal:USD"),
		&FiatRefundInstructions{Provider: "paypal"},
		func() error {
			directCalled = true
			return nil
		},
		func(iwallet.TransactionID) error {
			relayedCalled = true
			return nil
		},
	)

	require.NoError(t, err)
	assert.True(t, directCalled)
	assert.False(t, relayedCalled)
	assert.False(t, escrow.called)
}

func TestRelayOrDirect_CryptoInstructionsUseEscrowRelay(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	escrow := &mockEscrowRelay{txHash: "0xrelay"}
	svc.escrow = escrow

	directCalled := false
	relayedTx := iwallet.TransactionID("")

	err := svc.relayOrDirect(
		models.OrderID("order-crypto-relay"),
		"decline",
		iwallet.CoinType("crypto:eip155:137:erc20:0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		struct{ Payload string }{Payload: "relay-me"},
		func() error {
			directCalled = true
			return nil
		},
		func(txid iwallet.TransactionID) error {
			relayedTx = txid
			return nil
		},
	)

	require.NoError(t, err)
	assert.False(t, directCalled)
	assert.True(t, escrow.called)
	assert.Equal(t, iwallet.TransactionID("0xrelay"), relayedTx)
}
