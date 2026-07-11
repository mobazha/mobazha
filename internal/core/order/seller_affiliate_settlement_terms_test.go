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

func TestRequiresInterimAffiliateDisputeTerms(t *testing.T) {
	tests := []struct {
		name       string
		orderOpen  *pb.OrderOpen
		release    *pb.DisputeClose_ModeratedEscrowRelease
		requireNow bool
	}{
		{
			name:       "ordinary order",
			orderOpen:  &pb.OrderOpen{},
			release:    &pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "100"},
			requireNow: false,
		},
		{
			name:       "affiliate seller award",
			orderOpen:  &pb.OrderOpen{AffiliateReferralSessionID: "referral-session"},
			release:    &pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "100"},
			requireNow: true,
		},
		{
			name:       "affiliate no seller award",
			orderOpen:  &pb.OrderOpen{AffiliateReferralSessionID: "referral-session"},
			release:    &pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "0"},
			requireNow: false,
		},
		{
			name:       "affiliate malformed award fails closed",
			orderOpen:  &pb.OrderOpen{AffiliateReferralSessionID: "referral-session"},
			release:    &pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "invalid"},
			requireNow: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.requireNow, requiresInterimAffiliateDisputeTerms(test.orderOpen, test.release))
		})
	}
}

func TestRequireInterimAffiliatePayout_FailsClosedForReferredOrder(t *testing.T) {
	orderOpen := &pb.OrderOpen{AffiliateReferralSessionID: "referral-session"}

	require.ErrorIs(t, requireInterimAffiliatePayout(orderOpen, nil), models.ErrInvalidSellerAffiliate)
	require.NoError(t, requireInterimAffiliatePayout(orderOpen, &models.AffiliateSettlementPayout{
		Address: affiliateSettlementPayoutAddress,
		Amount:  "125",
	}))
	require.NoError(t, requireInterimAffiliatePayout(&pb.OrderOpen{}, nil))
}

func TestRequireInterimAffiliateDisputePayout_FailsClosedWhenSellerReceivesFunds(t *testing.T) {
	orderOpen := &pb.OrderOpen{AffiliateReferralSessionID: "referral-session"}
	release := &pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "100"}

	require.ErrorIs(
		t,
		requireInterimAffiliateDisputePayout(orderOpen, release, nil),
		models.ErrInvalidSellerAffiliate,
	)
	require.NoError(t, requireInterimAffiliateDisputePayout(
		orderOpen,
		release,
		&models.AffiliateSettlementPayout{Address: affiliateSettlementPayoutAddress, Amount: "10"},
	))
	require.NoError(t, requireInterimAffiliateDisputePayout(
		orderOpen,
		&pb.DisputeClose_ModeratedEscrowRelease{VendorAmount: "0"},
		nil,
	))
}
