//go:build !private_distribution

package settlement

import (
	"context"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExecuteSettlementAction_RejectsUnimplementedActionsBeforeDB(t *testing.T) {
	svc := &SettlementService{}
	_, _, err := svc.ExecuteSettlementAction(context.Background(), "dispute_release", models.OrderID("order-1"), "")
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

type utxoActionStatusStub struct {
	model payment.PaymentModel
}

func (s *utxoActionStatusStub) Model() payment.PaymentModel { return s.model }
func (s *utxoActionStatusStub) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (s *utxoActionStatusStub) GetActionStatus(context.Context, string) (*payment.ActionStatus, error) {
	return nil, payment.ErrUnsupportedAction
}

func (s *utxoActionStatusStub) SetupPayment(context.Context, payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Confirm(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Cancel(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Complete(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) DisputeRelease(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (s *utxoActionStatusStub) SignEscrowRelease(context.Context, payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *utxoActionStatusStub) VerifyDeposit(context.Context, payment.DepositVerifyParams) error {
	return nil
}
func (s *utxoActionStatusStub) ValidatePaymentMessage(payment.PaymentMessageParams) error { return nil }
func (s *utxoActionStatusStub) VerifyPreRelease(context.Context, payment.PreReleaseParams) error {
	return nil
}

func TestGetSettlementActionStatus_UTXOSyncActionFromStore(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const orderID = "utxo-sync-status-order"
	const actionID = "sync-complete-utxo-sync-status-order-deadbeef"
	order := seedModeratedSettlementActionOrder(t, db, orderID, models.RoleVendor)
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewUTXOSpec(true).ToPaymentSent(),
		Amount:         "1000",
		Coin:           "BCH",
		PayerAddress:   "bitcoincash:qpayer",
		RefundAddress:  "bitcoincash:qpayer",
		Moderator:      "12D3KooWModerator",
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.SettlementAction{
			ActionID:       actionID,
			OrderID:        orderID,
			ActionKind:     "complete",
			State:          "confirmed",
			TxHash:         "utxo-tx-hash-abc",
			SettlementCoin: "BCH",
			GrossAmount:    "1000",
		})
	}))

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainBitcoinCash, &utxoActionStatusStub{model: payment.PaymentModelMonitored})

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)

	status, coinType, err := svc.GetSettlementActionStatus(
		context.Background(),
		"complete",
		order.ID,
		actionID,
	)
	require.NoError(t, err)
	require.Equal(t, iwallet.CoinType("crypto:bitcoincash:mainnet:native"), coinType)
	require.NotNil(t, status)
	require.Equal(t, "confirmed", status.State)
	require.Equal(t, "utxo-tx-hash-abc", status.TxHash)
	require.Equal(t, orderID, status.OrderID)
	require.Equal(t, "complete", status.SettlementAction)
}
