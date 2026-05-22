package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestSettlementCoinFromPaymentSent_MapsKnownEVMTestnetToCanonicalChain(t *testing.T) {
	coin, err := SettlementCoinFromPaymentSent(&pb.PaymentSent{Coin: "crypto:eip155:11155111:native"})

	require.NoError(t, err)
	require.Equal(t, iwallet.CoinType("crypto:eip155:1:native"), coin)
}

func TestSettlementCoinFromPaymentSent_RejectsPricingCurrencyWithoutProvider(t *testing.T) {
	_, err := SettlementCoinFromPaymentSent(&pb.PaymentSent{Coin: "USD"})

	require.Error(t, err)
	require.Contains(t, err.Error(), `invalid payment coin "USD"`)
}

func TestPendingPaymentCoinFromOrder_ReadsLockedManagedEscrowPaymentIntent(t *testing.T) {
	order := &models.Order{}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           "crypto:eip155:1:native",
		Address:        "0xmanagedescrow",
		SettlementSpec: NewManagedEscrowSpec(false).ToPending(),
	}))

	coin, ok := PendingPaymentCoinFromOrder(order)

	require.True(t, ok)
	require.Equal(t, iwallet.CoinType("crypto:eip155:1:native"), coin)
}

func TestNormalizeSettlementPaymentCoin_RejectsPricingCurrencyWithoutProvider(t *testing.T) {
	_, ok := NormalizeSettlementPaymentCoin("USD")

	require.False(t, ok)
}

func TestNormalizeSettlementPaymentCoin_AcceptsTestOnlyMockCoin(t *testing.T) {
	coin, ok := NormalizeSettlementPaymentCoin("MCK")

	require.True(t, ok)
	require.Equal(t, iwallet.CtMock, coin)
}

func TestNormalizeSettlementPaymentCoin_MapsKnownEVMTestnetToCanonicalChain(t *testing.T) {
	coin, ok := NormalizeSettlementPaymentCoin("crypto:eip155:11155111:native")

	require.True(t, ok)
	require.Equal(t, iwallet.CoinType("crypto:eip155:1:native"), coin)
}
