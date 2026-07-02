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
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
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

func TestNormalizeSettlementPaymentCoin_AcceptsRuntimeSafeERC20Coin(t *testing.T) {
	coin, ok := NormalizeSettlementPaymentCoin("crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0")

	require.True(t, ok)
	require.Equal(t, iwallet.CoinType("crypto:eip155:1:erc20:0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0"), coin)
}

func TestNormalizeSettlementPaymentCoin_AcceptsRuntimeSPLCoin(t *testing.T) {
	const mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	coin, ok := NormalizeSettlementPaymentCoin(" crypto:SOLANA:DEVNET:SPL:" + mint + " ")

	require.True(t, ok)
	require.Equal(t, iwallet.CoinType("crypto:solana:devnet:spl:"+mint), coin)
}

func TestSettlementChainForCoin_ResolvesRuntimeSafeERC20Coin(t *testing.T) {
	chain, err := SettlementChainForCoin("crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0")

	require.NoError(t, err)
	require.Equal(t, iwallet.ChainEthereum, chain)
}

func TestSettlementChainForCoin_ResolvesRuntimeSPLCoin(t *testing.T) {
	chain, err := SettlementChainForCoin("crypto:solana:devnet:spl:EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	require.NoError(t, err)
	require.Equal(t, iwallet.ChainSolana, chain)
}

func TestSettlementCoinInfoForCoin_ResolvesRuntimeSafeERC20Coin(t *testing.T) {
	info, err := SettlementCoinInfoForCoin("crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0")

	require.NoError(t, err)
	require.Equal(t, iwallet.ChainEthereum, info.Chain)
	require.False(t, info.IsNative)
	require.Equal(t, "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0", info.ContractAddress(false))
	require.Equal(t, "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0", info.ContractAddress(true))
}

func TestSettlementCoinInfoForCoin_ResolvesRuntimeTestnetERC20Coin(t *testing.T) {
	info, err := SettlementCoinInfoForCoin("crypto:eip155:11155111:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0")

	require.NoError(t, err)
	require.Equal(t, iwallet.ChainEthereum, info.Chain)
	require.False(t, info.IsNative)
	require.Equal(t, "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0", info.ContractAddress(false))
	require.Equal(t, "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0", info.ContractAddress(true))
}

func TestSettlementCoinInfoForCoin_ResolvesRuntimeSPLCoin(t *testing.T) {
	const mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	info, err := SettlementCoinInfoForCoin(iwallet.CoinType("crypto:solana:devnet:spl:" + mint))

	require.NoError(t, err)
	require.Equal(t, iwallet.ChainSolana, info.Chain)
	require.False(t, info.IsNative)
	require.Equal(t, mint, info.ContractAddress(false))
	require.Equal(t, mint, info.ContractAddress(true))
	require.Equal(t, uint8(9), info.Decimals)
}
