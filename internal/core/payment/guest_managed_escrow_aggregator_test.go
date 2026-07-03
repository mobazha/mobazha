// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

type recordingGuestPayment struct {
	detected []string
}

func (r *recordingGuestPayment) IsEnabled(context.Context) bool { return true }
func (r *recordingGuestPayment) QuoteGuestOrderSupply(context.Context, contracts.QuoteGuestOrderSupplyRequest) (*contracts.GuestOrderSupplyQuoteResponse, error) {
	return nil, nil
}
func (r *recordingGuestPayment) CreateGuestOrder(context.Context, contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
	return nil, nil
}
func (r *recordingGuestPayment) GetGuestOrderStatus(context.Context, string) (*contracts.GuestOrderStatusResponse, error) {
	return nil, nil
}
func (r *recordingGuestPayment) ListGuestOrders(context.Context, contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
	return nil, 0, nil
}
func (r *recordingGuestPayment) ShipGuestOrder(context.Context, string, string, string) error {
	return nil
}
func (r *recordingGuestPayment) CompleteGuestOrder(context.Context, string) error { return nil }
func (r *recordingGuestPayment) HandlePaymentDetected(orderToken, txHash string, _ *contracts.PaymentDetectedOpts) error {
	r.detected = append(r.detected, orderToken+":"+txHash)
	return nil
}
func (r *recordingGuestPayment) HandleConfirmationUpdate(string, int) error     { return nil }
func (r *recordingGuestPayment) HandlePoolPayment(string, string, uint64) error { return nil }
func (r *recordingGuestPayment) HandleLatePayment(string, string, string, uint64, uint64) error {
	return nil
}
func (r *recordingGuestPayment) CleanupExpiredOrders(context.Context) {}
func (r *recordingGuestPayment) AutoCompleteOrders(context.Context)   {}
func (r *recordingGuestPayment) RunGuestCleanupOnce()                 {}
func (r *recordingGuestPayment) GetGuestCheckoutConfig(context.Context) (*models.GuestCheckoutConfig, error) {
	return nil, nil
}
func (r *recordingGuestPayment) SaveGuestCheckoutConfig(context.Context, *models.GuestCheckoutConfig) error {
	return nil
}
func (r *recordingGuestPayment) GetGuestCheckoutReadiness(context.Context) (*contracts.GuestCheckoutReadiness, error) {
	return nil, nil
}
func (r *recordingGuestPayment) GetAdminGuestOrder(context.Context, string) (*models.GuestOrder, error) {
	return nil, nil
}

type guestAggObsRepo struct {
	*fakeObsRepo
}

func (r *guestAggObsRepo) ListDeduplicatedConfirmed(_ context.Context, tenantID, orderID string) ([]models.PaymentObservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.PaymentObservation
	for _, obs := range r.inserted {
		if obs.TenantID == tenantID && obs.OrderID == orderID {
			out = append(out, *obs)
		}
	}
	return out, nil
}

func newGuestAggObsRepo() *guestAggObsRepo {
	return &guestAggObsRepo{fakeObsRepo: newFakeObsRepo()}
}

func seedGuestOrderForAggregator(t *testing.T, vdb *vTestDB, order models.GuestOrder) {
	t.Helper()
	require.NoError(t, vdb.gormDB.AutoMigrate(&models.GuestOrder{}))
	require.NoError(t, vdb.gormDB.Create(&order).Error)
}

func guestAggregatorOrder(t *testing.T, tenantID, orderToken, paymentAmount string, withManagedEscrow bool) models.GuestOrder {
	t.Helper()
	order := models.GuestOrder{
		TenantMixin:   models.TenantMixin{TenantID: tenantID},
		OrderToken:    orderToken,
		PaymentAmount: paymentAmount,
	}
	if withManagedEscrow {
		require.NoError(t, order.SetManagedEscrowGuestMetadata([]byte(`{"provider":"test","address":"0x1234567890123456789012345678901234567890"}`)))
	}
	return order
}

func TestGuestManagedEscrowPaymentAggregator_FundsWhenObservationsMeetExpected(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "agg_test"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(1000)
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", true))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))
	require.Len(t, guest.detected, 1)
	require.Equal(t, orderID+":0xabc", guest.detected[0])
}

func TestGuestManagedEscrowPaymentAggregator_RecordsOverpaymentTotals(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "overpaid"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(1250)
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", true))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))

	var order models.GuestOrder
	require.NoError(t, vdb.gormDB.Where("order_token = ?", orderID).First(&order).Error)
	require.Equal(t, "1250", order.TotalReceived)
	require.Equal(t, "250", order.OverpaidAmount)
	require.Equal(t, []string{orderID + ":0xabc"}, guest.detected)
}

func TestGuestManagedEscrowPaymentAggregator_RecordsPartialTotalWithoutFunding(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "partial_total"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(500)
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", true))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))

	var order models.GuestOrder
	require.NoError(t, vdb.gormDB.Where("order_token = ?", orderID).First(&order).Error)
	require.Equal(t, "500", order.TotalReceived)
	require.Empty(t, order.OverpaidAmount)
	require.Empty(t, guest.detected)
}

func TestGuestManagedEscrowPaymentAggregator_DoesNotExposeSyntheticTxHash(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "synthetic"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(1000)
	obs.TxHash = "0xsyntheticbalancepoll"
	obs.TxHashSource = models.PaymentTxHashSourceBalancePoll
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", true))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))
	require.Equal(t, []string{orderID + ":"}, guest.detected)
}

func TestGuestManagedEscrowPaymentAggregator_SkipsNonManagedEscrowOrders(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "non_safe"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(1000)
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", false))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))
	require.Empty(t, guest.detected)
}

func TestGuestManagedEscrowPaymentAggregator_PartialDoesNotFund(t *testing.T) {
	repo := newGuestAggObsRepo()
	tenantID := "_default"
	orderID := GuestOrderTokenPrefix + "partial"
	obs := validFundingEvent()
	obs.OrderID = orderID
	obs.Amount = big.NewInt(500)
	require.NoError(t, repo.InsertObservation(context.Background(), buildObservation(tenantID, obs, models.PaymentObservationSourceMonitor, "managed-monitor")))

	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, guestAggregatorOrder(t, tenantID, orderID, "1000", true))
	guest := &recordingGuestPayment{}
	agg := NewGuestManagedEscrowPaymentAggregator(vdb, guest, repo)
	require.NoError(t, agg.AggregateAndEmit(context.Background(), tenantID, orderID))
	require.Empty(t, guest.detected)
}

func TestRoutingPaymentAggregator_RoutesByPrefix(t *testing.T) {
	vdb := newVerifierTestDB(t)
	seedGuestOrderForAggregator(t, vdb, models.GuestOrder{
		TenantMixin:   models.TenantMixin{TenantID: "_default"},
		OrderToken:    GuestOrderTokenPrefix + "route",
		PaymentAmount: "1",
	})
	guest := &recordingGuestPayment{}
	p2pCalled := false
	route := NewRoutingPaymentAggregator(
		NewGuestManagedEscrowPaymentAggregator(vdb, guest, newGuestAggObsRepo()),
		PaymentAggregatorFunc(func(context.Context, string, string) error {
			p2pCalled = true
			return nil
		}),
	)
	require.NoError(t, route.AggregateAndEmit(context.Background(), "_default", "QmP2POrder"))
	require.True(t, p2pCalled)
	p2pCalled = false
	require.NoError(t, route.AggregateAndEmit(context.Background(), "_default", GuestOrderTokenPrefix+"route"))
	require.False(t, p2pCalled)
}

// PaymentAggregatorFunc adapts a function to PaymentAggregator.
type PaymentAggregatorFunc func(ctx context.Context, tenantID, orderID string) error

func (f PaymentAggregatorFunc) AggregateAndEmit(ctx context.Context, tenantID, orderID string) error {
	return f(ctx, tenantID, orderID)
}
