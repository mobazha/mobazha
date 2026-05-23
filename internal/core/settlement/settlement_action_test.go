//go:build !private_distribution

package settlement

import (
	"context"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExecuteSettlementAction_RejectsUnimplementedActionsBeforeDB(t *testing.T) {
	svc := &SettlementService{}
	_, _, err := svc.ExecuteSettlementAction(context.Background(), "complete", models.OrderID("order-1"), "")
	if err == nil {
		t.Fatal("expected unsupported action error")
	}
	if !strings.Contains(err.Error(), "supported: confirm, cancel") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteSettlementAction_ConfirmModeratedReturnsNoop(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedModeratedSettlementActionOrder(t, db, "order-moderated-confirm", models.RoleVendor)

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(),
		"confirm",
		order.ID,
		"0x2222222222222222222222222222222222222222",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.NotNil(t, result)
	require.Equal(t, payment.ActionModeCompleted, result.Mode)
}

func TestExecuteSettlementAction_CancelModeratedReturnsNoop(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedModeratedSettlementActionOrder(t, db, "order-moderated-cancel", models.RoleVendor)

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(),
		"cancel",
		order.ID,
		"0x3333333333333333333333333333333333333333",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.NotNil(t, result)
	require.Equal(t, payment.ActionModeCompleted, result.Mode)
}

func seedModeratedSettlementActionOrder(
	t *testing.T,
	db database.Database,
	id string,
	role models.OrderRole,
) *models.Order {
	t.Helper()

	orderOpen, err := anypb.New(&pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		Amount:    "1000",
		BuyerID:   &pb.ID{PeerID: "12D3KooWBuyer"},
		Listings: []*pb.SignedListing{{
			Cid: "listing-moderated",
			Listing: &pb.Listing{
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item:     &pb.Listing_Item{Title: "Moderated order"},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-moderated",
			Quantity:    "1",
		}},
	})
	require.NoError(t, err)

	order := &models.Order{ID: models.OrderID(id)}
	order.SetRole(role)
	order.PaymentVerificationStatus = models.PaymentVerificationStatusVerified
	require.NoError(t, order.PutMessage(&npb.OrderMessage{
		OrderID:     id,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     orderOpen,
	}))
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPaymentSent(),
		Amount:         "1000",
		Coin:           "crypto:eip155:11155111:native",
		PayerAddress:   "0x1111111111111111111111111111111111111111",
		RefundAddress:  "0x1111111111111111111111111111111111111111",
		Moderator:      "12D3KooWModerator",
	}))

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
	return order
}
