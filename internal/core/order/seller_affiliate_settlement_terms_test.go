package order

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const (
	affiliateSettlementSellerAddress = "0x1111111111111111111111111111111111111111"
	affiliateSettlementPayoutAddress = "0x2222222222222222222222222222222222222222"
)

func TestAffiliatePayoutFromEscrowRelease_UsesSellerSignedTerms(t *testing.T) {
	payout, err := affiliatePayoutFromEscrowRelease(&pb.EscrowRelease{
		ToAmount:         "1000",
		AffiliateAddress: affiliateSettlementPayoutAddress,
		AffiliateAmount:  "125",
	})
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress(affiliateSettlementPayoutAddress).Hex(), payout.Address)
	require.Equal(t, "125", payout.Amount)
}

func TestAffiliatePayoutFromEscrowRelease_RejectsIncompleteTerms(t *testing.T) {
	_, err := affiliatePayoutFromEscrowRelease(&pb.EscrowRelease{ToAmount: "1000", AffiliateAmount: "125"})
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)
}

func TestAffiliatePayoutFromDisputeRelease_ScalesSellerSignedTerms(t *testing.T) {
	payout, err := affiliatePayoutFromDisputeRelease([]*pb.OrderShipment{{
		ReleaseInfo: &pb.EscrowRelease{
			ToAddress:        affiliateSettlementSellerAddress,
			ToAmount:         "1000",
			AffiliateAddress: affiliateSettlementPayoutAddress,
			AffiliateAmount:  "125",
		},
	}}, &pb.DisputeClose_ModeratedEscrowRelease{
		VendorAddress: affiliateSettlementSellerAddress,
		VendorAmount:  "400",
	})
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress(affiliateSettlementPayoutAddress).Hex(), payout.Address)
	require.Equal(t, "50", payout.Amount)
}

func TestAffiliatePayoutFromDisputeRelease_RejectsDifferentVendor(t *testing.T) {
	_, err := affiliatePayoutFromDisputeRelease([]*pb.OrderShipment{{
		ReleaseInfo: &pb.EscrowRelease{
			ToAddress:        affiliateSettlementSellerAddress,
			ToAmount:         "1000",
			AffiliateAddress: affiliateSettlementPayoutAddress,
			AffiliateAmount:  "125",
		},
	}}, &pb.DisputeClose_ModeratedEscrowRelease{
		VendorAddress: "0x3333333333333333333333333333333333333333",
		VendorAmount:  "400",
	})
	require.ErrorIs(t, err, models.ErrInvalidSellerAffiliate)
}

func TestAffiliatePayoutForDisputeSettlement_UsesUTXOGrossSellerRatio(t *testing.T) {
	coinType, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	require.NoError(t, err)
	payout, err := affiliatePayoutForDisputeSettlement(coinType, []*pb.OrderShipment{{
		ReleaseInfo: &pb.EscrowRelease{
			ToAddress:        "bitcoincash:qvendor",
			ToAmount:         "80",
			AffiliateAddress: "bitcoincash:qaffiliate",
			AffiliateAmount:  "20",
		},
	}}, &pb.DisputeClose_ModeratedEscrowRelease{
		VendorAddress: "bitcoincash:qvendor",
		VendorAmount:  "40",
	})
	require.NoError(t, err)
	require.NotNil(t, payout)
	require.Equal(t, "bitcoincash:qaffiliate", payout.Address)
	require.Equal(t, "8", payout.Amount)
}
