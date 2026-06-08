package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestResolveBuyerRefundAddress_ExplicitWins(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Order: &models.Order{RefundAddress: " 0xexplicit "},
		PaymentSent: &pb.PaymentSent{
			Coin:          "crypto:eip155:1:native",
			RefundAddress: "0xlegacy",
			PayerAddress:  "0xpayer",
		},
	})

	require.Equal(t, "0xexplicit", result.Address)
	require.Equal(t, RefundAddressSourceExplicit, result.Source)
	require.False(t, result.RequiresUserInput)
}

func TestResolveBuyerRefundAddress_CustodialRequiresExplicit(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin:             iwallet.CoinType("crypto:eip155:1:native"),
		PaymentSent:      &pb.PaymentSent{PayerAddress: "0xpayer"},
		PayFromCustodial: true,
	})

	require.Empty(t, result.Address)
	require.True(t, result.RequiresUserInput)
	require.Equal(t, RefundResolveReasonExchangeDeclared, result.Reason)
}

func TestResolveBuyerRefundAddress_AccountChainUsesPayer(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin:        iwallet.CoinType("crypto:eip155:1:native"),
		PaymentSent: &pb.PaymentSent{PayerAddress: "0xpayer"},
	})

	require.Equal(t, "0xpayer", result.Address)
	require.Equal(t, RefundAddressSourcePayer, result.Source)
	require.False(t, result.RequiresUserInput)
}

func TestResolveBuyerRefundAddress_AccountChainRequiresInputForMultipleObservedPayers(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:eip155:1:native"),
		Observations: []models.PaymentObservation{
			{FromAddress: "0xpayer-a"},
			{FromAddress: "0xpayer-b"},
		},
		PaymentSent: &pb.PaymentSent{PayerAddress: "0xpayer-b"},
	})

	require.Empty(t, result.Address)
	require.True(t, result.RequiresUserInput)
	require.Equal(t, RefundResolveReasonMultiInput, result.Reason)
}

func TestResolveBuyerRefundAddress_AccountChainIgnoresRevertedFundingFacts(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:eip155:1:native"),
		PaymentSent: &pb.PaymentSent{
			FundingFacts: []*pb.PaymentSent_FundingFact{
				{FromAddress: "0xreverted", Status: models.PaymentObservationStatusReverted},
				{FromAddress: "0xconfirmed", Status: models.PaymentObservationStatusConfirmed},
			},
		},
	})

	require.Equal(t, "0xconfirmed", result.Address)
	require.Equal(t, RefundAddressSourcePayer, result.Source)
}

func TestResolveBuyerRefundAddress_UTXOUsesSingleFundingInput(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
		PaymentSent: &pb.PaymentSent{
			Coin: "crypto:bitcoincash:mainnet:native",
			FundingFacts: []*pb.PaymentSent_FundingFact{
				{FromAddress: "bitcoincash:qqpayeraddress"},
				{FromAddress: "qqpayeraddress"},
			},
		},
	})

	require.Equal(t, "bitcoincash:qqpayeraddress", result.Address)
	require.Equal(t, RefundAddressSourceSingleUTXOInput, result.Source)
	require.False(t, result.RequiresUserInput)
}

func TestResolveBuyerRefundAddress_UTXORequiresInputForMultipleInputs(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
		PaymentSent: &pb.PaymentSent{
			Coin: "crypto:bitcoincash:mainnet:native",
			FundingFacts: []*pb.PaymentSent_FundingFact{
				{FromAddress: "qqpayeraddress-a"},
				{FromAddress: "qqpayeraddress-b"},
			},
		},
	})

	require.Empty(t, result.Address)
	require.True(t, result.RequiresUserInput)
	require.Equal(t, RefundResolveReasonMultiInput, result.Reason)
}

func TestResolveBuyerRefundAddress_UTXOIgnoresRevertedFundingFacts(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
		PaymentSent: &pb.PaymentSent{
			Coin: "crypto:bitcoincash:mainnet:native",
			FundingFacts: []*pb.PaymentSent_FundingFact{
				{FromAddress: "qqreverted", Status: models.PaymentObservationStatusReverted},
				{FromAddress: "qqconfirmed", Status: models.PaymentObservationStatusConfirmed},
			},
		},
	})

	require.Equal(t, "qqconfirmed", result.Address)
	require.Equal(t, RefundAddressSourceSingleUTXOInput, result.Source)
}

func TestResolveBuyerRefundAddress_NotObservedYetDoesNotRequireInput(t *testing.T) {
	result := ResolveBuyerRefundAddress(ResolveBuyerRefundAddressParams{
		Coin: iwallet.CoinType("crypto:eip155:1:native"),
	})

	require.Empty(t, result.Address)
	require.False(t, result.RequiresUserInput)
	require.Equal(t, RefundResolveReasonNotObservedYet, result.Reason)
}

func TestUniqueUTXOInputAddress_UsesOrderTransactions(t *testing.T) {
	addr := iwallet.NewAddress("ltc-input-address", iwallet.CoinType("crypto:litecoin:mainnet:native"))
	got, ok, reason := UniqueUTXOInputAddress(nil, nil, []iwallet.Transaction{{
		From: []iwallet.SpendInfo{{Address: addr}},
	}})

	require.True(t, ok)
	require.Empty(t, reason)
	require.Equal(t, "ltc-input-address", got)
}

func TestUniqueUTXOInputAddress_RejectsMultipleTransactionInputs(t *testing.T) {
	addr := iwallet.NewAddress("ltc-input-address", iwallet.CoinType("crypto:litecoin:mainnet:native"))
	got, ok, reason := UniqueUTXOInputAddress(nil, nil, []iwallet.Transaction{{
		From: []iwallet.SpendInfo{
			{Address: addr},
			{Address: addr},
		},
	}})

	require.False(t, ok)
	require.Empty(t, got)
	require.Equal(t, RefundResolveReasonMultiInput, reason)
}

func TestUniqueUTXOInputAddress_BlankObservedInputIsUnparseable(t *testing.T) {
	got, ok, reason := UniqueUTXOInputAddress([]models.PaymentObservation{{FromAddress: ""}}, nil, nil)

	require.False(t, ok)
	require.Empty(t, got)
	require.Equal(t, RefundResolveReasonUnparseable, reason)
}
