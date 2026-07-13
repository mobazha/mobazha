package models

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestPendingEscrowPaymentInfo_RoundTrip(t *testing.T) {
	order := &Order{}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&PendingEscrowPaymentInfo{
		Coin:                   "crypto:eip155:1:native",
		Amount:                 290000000,
		EscrowAddress:          "0xescrow",
		Moderator:              "moderator-peer-id",
		ModeratorAddress:       "moderator-chain-address",
		ModeratorPayoutAddress: "moderator-payout-address",
		ModeratorPayoutAmount:  123,
		AffiliatePayoutAddress: "affiliate-payout-address",
		AffiliatePayoutAmount:  45,
		SettlementSpec: &PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "client_signed",
			EscrowType: "evm_contract",
		},
	}))

	got, err := order.GetPendingEscrowPaymentInfo()
	require.NoError(t, err)
	require.Equal(t, "escrow", got.Type)
	require.EqualValues(t, 290000000, got.Amount)
	require.Equal(t, "0xescrow", got.EscrowAddress)
	require.Equal(t, "moderator-peer-id", got.Moderator)
	require.Equal(t, "moderator-chain-address", got.ModeratorAddress)
	require.Equal(t, "moderator-payout-address", got.ModeratorPayoutAddress)
	require.EqualValues(t, 123, got.ModeratorPayoutAmount)
	require.Equal(t, "affiliate-payout-address", got.AffiliatePayoutAddress)
	require.EqualValues(t, 45, got.AffiliatePayoutAmount)

	// UTXO getter must not mis-read escrow JSON.
	utxo, err := order.GetPendingPaymentInfo()
	require.NoError(t, err)
	require.Nil(t, utxo)
}

func TestExpectedPaymentAmountString_PendingEscrowAmountWinsOverOrderOpen(t *testing.T) {
	rawOpen, err := (protojson.MarshalOptions{}).Marshal(&pb.OrderOpen{
		Amount:      "2900",
		PricingCoin: "USD",
	})
	require.NoError(t, err)

	order := &Order{SerializedOrderOpen: rawOpen}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&PendingEscrowPaymentInfo{
		Coin:          "crypto:solana:mainnet:native",
		Amount:        290000000,
		EscrowAddress: "solana-escrow-address",
		SettlementSpec: &PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "address_monitored",
			EscrowType: "solana_escrow",
		},
	}))

	require.Equal(t, "290000000", order.ExpectedPaymentAmountString())
}

func TestExpectedPaymentAmountString_PendingEscrowWithoutAmountDoesNotFallback(t *testing.T) {
	rawOpen, err := (protojson.MarshalOptions{}).Marshal(&pb.OrderOpen{
		Amount:      "2900",
		PricingCoin: "USD",
	})
	require.NoError(t, err)

	order := &Order{SerializedOrderOpen: rawOpen}
	require.NoError(t, order.SetPendingEscrowPaymentInfo(&PendingEscrowPaymentInfo{
		Coin:          "crypto:solana:mainnet:native",
		EscrowAddress: "solana-escrow-address",
		SettlementSpec: &PendingSettlementSpec{
			Method:     "CANCELABLE",
			PayMode:    "address_monitored",
			EscrowType: "solana_escrow",
		},
	}))

	require.Empty(t, order.ExpectedPaymentAmountString())
}
