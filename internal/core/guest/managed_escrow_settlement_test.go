package guest

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
)

func TestGuestOrderEligibleForSettlement(t *testing.T) {
	require.True(t, guestOrderEligibleForSettlement(models.GuestOrderFunded))
	require.True(t, guestOrderEligibleForSettlement(models.GuestOrderShipped))
	require.True(t, guestOrderEligibleForSettlement(models.GuestOrderCompleted))
	require.False(t, guestOrderEligibleForSettlement(models.GuestOrderAwaitingPayment))
}

func TestManagedEscrowGuestSettlementAmount_UsesObservedOverpayment(t *testing.T) {
	order := &models.GuestOrder{PaymentAmount: "1000", TotalReceived: "1250"}
	require.Equal(t, "1250", managedEscrowGuestSettlementAmount(order))
	order.TotalReceived = "999"
	require.Equal(t, "1000", managedEscrowGuestSettlementAmount(order))
}

func TestAfterGuestOrderFunded_WithholdsEntitlementWithoutSettlementRuntime(t *testing.T) {
	db := newGuestTestDB(t)
	order := testManagedEscrowGuestOrder(t, "gst_settlement_unwired", models.GuestOrderAwaitingPayment)
	seedGuestOrder(t, db, 51, order)
	bus := events.NewBus()
	sub, err := bus.Subscribe(&events.OrderConfirmation{}, events.BufSize(4))
	require.NoError(t, err)
	defer sub.Close()

	svc := &GuestOrderAppService{db: db, eventBus: bus, nodeID: "test-node"}
	require.NoError(t, svc.HandlePaymentDetected(order.OrderToken, "0xpay", nil))
	select {
	case evt := <-sub.Out():
		t.Fatalf("OrderConfirmation must not fire without settlement runtime, got %T", evt)
	default:
	}
}

func TestManagedEscrowGuestSettlementSource_ProjectsAndClaimsIntent(t *testing.T) {
	db := newGuestTestDB(t)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))
	order := testManagedEscrowGuestOrder(t, "gst_distribution_request", models.GuestOrderFunded)
	order.TotalReceived = "1250"
	seedGuestOrder(t, db, 53, order)

	source := newTestManagedEscrowGuestSettlementSource(db)
	request, err := source.ClaimManagedEscrowGuestSettlement(context.Background(), order.OrderToken)
	require.NoError(t, err)
	require.NotNil(t, request)
	require.Equal(t, order.OrderToken, request.OrderID)
	require.Equal(t, uint64(1), request.ChainID)
	require.Equal(t, "1250", request.PaymentAmount)
	require.Equal(t, order.PaymentAddress, request.EscrowAddress)
	require.NotEmpty(t, request.IntentID)
	require.NotEmpty(t, request.ClaimToken)

	duplicate, err := source.ClaimManagedEscrowGuestSettlement(context.Background(), order.OrderToken)
	require.NoError(t, err)
	require.Nil(t, duplicate)
}

func testManagedEscrowGuestOrder(t *testing.T, orderID string, state models.GuestOrderState) models.GuestOrder {
	t.Helper()
	const escrow = "0x2222222222222222222222222222222222222222"
	order := models.GuestOrder{
		OrderToken: orderID, State: state, PaymentCoin: "crypto:eip155:1:native",
		PaymentAmount: "1000", PaymentAddress: escrow, ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, order.SetManagedEscrowGuestMetadata(encodeTestManagedEscrowMetadata(t, testManagedEscrowMetadata{
		ChainID: 1, EscrowAddress: escrow, OwnerAddress: "0x3333333333333333333333333333333333333333",
		SaltNonce: "1", SettlementRecipient: "0x4444444444444444444444444444444444444444",
	})))
	return order
}

type repeatingGuestSettlementSource struct{ lists atomic.Int32 }

func (s *repeatingGuestSettlementSource) ClaimManagedEscrowGuestSettlement(
	context.Context,
	string,
) (*distribution.ManagedEscrowGuestSettlementRequest, error) {
	return &distribution.ManagedEscrowGuestSettlementRequest{IntentID: "intent-1", ClaimToken: "claim-1", OrderID: "order-1"}, nil
}

func (s *repeatingGuestSettlementSource) ListPendingManagedEscrowGuestSettlementOrderIDs(context.Context) ([]string, error) {
	s.lists.Add(1)
	return []string{"order-1"}, nil
}

func (s *repeatingGuestSettlementSource) ListConfirmedManagedEscrowGuestSettlements(context.Context) ([]string, error) {
	return nil, nil
}

type countingGuestSettlementExecutor struct{ submits atomic.Int32 }

func (e *countingGuestSettlementExecutor) SubmitManagedEscrowGuestSettlement(
	context.Context,
	distribution.ManagedEscrowGuestSettlementRequest,
) error {
	e.submits.Add(1)
	return nil
}

func TestPendingSettlementRecovery_ReconcilesUntilCancellation(t *testing.T) {
	source := &repeatingGuestSettlementSource{}
	executor := &countingGuestSettlementExecutor{}
	service := NewDistributionManagedEscrowGuestSettlementService(source, executor)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.runPendingSettlementRecovery(ctx, 5*time.Millisecond)
	}()
	require.Eventually(t, func() bool {
		return source.lists.Load() >= 2 && executor.submits.Load() >= 2
	}, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recovery loop did not stop after cancellation")
	}
}
