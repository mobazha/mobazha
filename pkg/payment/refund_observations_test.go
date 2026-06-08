package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestMergeRefundResolutionObservations_PrefersFundingFacts(t *testing.T) {
	order := &models.Order{}
	order.ID = models.OrderID("order-1")

	merged := MergeRefundResolutionObservations(nil, order, &pb.PaymentSent{
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{FromAddress: "0xpayer"},
		},
	})

	require.Len(t, merged, 1)
	require.Equal(t, "0xpayer", merged[0].FromAddress)
}

func TestFundingFactsAsObservations_SkipsRevertedFacts(t *testing.T) {
	order := &models.Order{}
	order.ID = models.OrderID("order-1")

	rows := FundingFactsAsObservations(order, &pb.PaymentSent{
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{FromAddress: "0xreverted", Status: models.PaymentObservationStatusReverted},
			{FromAddress: "0xconfirmed", Status: models.PaymentObservationStatusConfirmed},
		},
	})

	require.Len(t, rows, 1)
	require.Equal(t, "0xconfirmed", rows[0].FromAddress)
}
