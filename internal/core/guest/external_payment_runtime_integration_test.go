// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const externalObservedCoinType = "crypto:monero:mainnet:native"

// observedPaymentRuntimeStub exercises the provider-neutral direct observed
// rail without importing a chain-specific monitor into Core tests.
type observedPaymentRuntimeStub struct {
	mu      sync.Mutex
	watches map[uint32]*distribution.ExternalPaymentWatch
	height  uint64
}

func newObservedPaymentRuntimeStub() *observedPaymentRuntimeStub {
	return &observedPaymentRuntimeStub{watches: make(map[uint32]*distribution.ExternalPaymentWatch), height: 100}
}

func bindObservedPaymentRuntime(t *testing.T, monitor *GuestPaymentMonitor, runtime distribution.ExternalPaymentRuntime, generation string) payment.RouteIdentity {
	t.Helper()
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: generation, RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: externalObservedCoinType, ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	monitor.SetExternalPaymentRuntimeCatalog(catalog)
	return route
}

func (*observedPaymentRuntimeStub) Start(context.Context) error { return nil }
func (*observedPaymentRuntimeStub) Close() error                { return nil }
func (*observedPaymentRuntimeStub) PaymentHealth(context.Context) distribution.ExternalPaymentHealth {
	return distribution.ExternalPaymentHealth{State: distribution.ExternalPaymentReady}
}
func (*observedPaymentRuntimeStub) EnsurePaymentAddress(context.Context, distribution.ExternalPaymentAddressRequest) (distribution.ExternalPaymentAddress, error) {
	return distribution.ExternalPaymentAddress{Address: "addr_test", Index: 1}, nil
}
func (s *observedPaymentRuntimeStub) WatchPayment(watch *distribution.ExternalPaymentWatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watches[watch.AddressIndex] = watch
	return nil
}
func (s *observedPaymentRuntimeStub) UnwatchPayment(index uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.watches, index)
}
func (*observedPaymentRuntimeStub) ReapPayment(uint32)                 {}
func (*observedPaymentRuntimeStub) PaymentPollInterval() time.Duration { return 50 * time.Millisecond }
func (*observedPaymentRuntimeStub) PaymentGracePeriod(iwallet.CoinType) time.Duration {
	return 10 * time.Second
}
func (s *observedPaymentRuntimeStub) PaymentHeight(context.Context) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.height, nil
}
func (s *observedPaymentRuntimeStub) isWatching(index uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watches[index] != nil
}

func (s *observedPaymentRuntimeStub) emit(index uint32, event distribution.ExternalPaymentEvent) {
	s.mu.Lock()
	watch := s.watches[index]
	s.mu.Unlock()
	if watch != nil {
		watch.OnPayment(event)
	}
}

func (s *observedPaymentRuntimeStub) setHeight(height uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.height = height
}

func TestGuestPaymentMonitor_RestoreUsesPersistedHistoricalRoute(t *testing.T) {
	db := newGuestTestDB(t)
	oldRuntime := newObservedPaymentRuntimeStub()
	newRuntime := newObservedPaymentRuntimeStub()
	oldRoute := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: externalObservedCoinType, ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	newRoute := oldRoute
	newRoute.ImplementationGeneration = "v2"
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: oldRoute, Runtime: oldRuntime}))
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: newRoute, Runtime: newRuntime, ActiveForNewWork: true}))

	order := models.GuestOrder{
		OrderToken: "gst_historical_route", State: models.GuestOrderAwaitingPayment,
		PaymentCoin: externalObservedCoinType, PaymentAddress: "historical_address", PaymentAmount: "100",
		AddressIndex: 41, RequiredConfs: 2, ExpiresAt: time.Now().Add(time.Hour),
	}
	setGuestOrderPaymentRoute(&order, oldRoute, 10*time.Second)
	seedGuestOrder(t, db, 703, order)

	monitor := NewGuestPaymentMonitor(db, nil, nil)
	monitor.SetExternalPaymentRuntimeCatalog(catalog)
	defer monitor.StopAll()
	require.NoError(t, monitor.RestoreWatches(context.Background()))
	require.Eventually(t, func() bool { return oldRuntime.isWatching(41) }, time.Second, 10*time.Millisecond)
	assert.False(t, newRuntime.isWatching(41), "current default must not service historical work")
}

func TestGuestPaymentMonitor_RestoreFailsClosedWithoutHistoricalRoute(t *testing.T) {
	db := newGuestTestDB(t)
	currentRuntime := newObservedPaymentRuntimeStub()
	currentRoute := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v2", RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: externalObservedCoinType, ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	historicalRoute := currentRoute
	historicalRoute.ImplementationGeneration = "v1"
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: currentRoute, Runtime: currentRuntime, ActiveForNewWork: true}))

	order := models.GuestOrder{
		OrderToken: "gst_missing_historical_route", State: models.GuestOrderAwaitingPayment,
		PaymentCoin: externalObservedCoinType, PaymentAddress: "historical_address", PaymentAmount: "100",
		AddressIndex: 42, RequiredConfs: 2, ExpiresAt: time.Now().Add(time.Hour),
	}
	setGuestOrderPaymentRoute(&order, historicalRoute, 10*time.Second)
	seedGuestOrder(t, db, 704, order)

	monitor := NewGuestPaymentMonitor(db, nil, nil)
	monitor.SetExternalPaymentRuntimeCatalog(catalog)
	defer monitor.StopAll()
	require.NoError(t, monitor.RestoreWatches(context.Background()))
	assert.Equal(t, 0, monitor.ActiveWatchCount())
	assert.False(t, currentRuntime.isWatching(42))
}

func TestGuestOrder_DurableRouteIsCreateOnly(t *testing.T) {
	db := newGuestTestDB(t)
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: externalObservedCoinType, ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	order := models.GuestOrder{
		OrderToken: "gst_immutable_route", State: models.GuestOrderAwaitingPayment,
		PaymentCoin: externalObservedCoinType, PaymentAddress: "immutable_address", PaymentAmount: "100",
		AddressIndex: 43, PaymentAttemptID: "pa_immutable", RequiredConfs: 2, ExpiresAt: time.Now().Add(time.Hour),
	}
	setGuestOrderPaymentRoute(&order, route, 10*time.Second)
	seedGuestOrder(t, db, 705, order)

	stored := loadGuestOrder(t, db, order.OrderToken)
	stored.RouteImplementationGeneration = "v2"
	stored.PaymentGracePeriodNanos = int64(time.Minute)
	stored.PaymentAttemptID = "pa_changed"
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(&stored) }))

	reloaded := loadGuestOrder(t, db, order.OrderToken)
	assert.Equal(t, "v1", reloaded.RouteImplementationGeneration)
	assert.Equal(t, int64(10*time.Second), reloaded.PaymentGracePeriodNanos)
	assert.Equal(t, "pa_immutable", reloaded.PaymentAttemptID)
}

// TestExternalPaymentRuntime_PoolThenConfirmed_ToFunded exercises the full externally observed
// guest checkout lifecycle:
//
//	WatchOrder → pool detection → confirmed payment → confirmation
//	polling → funded state without a Core business sweep task.
func TestExternalPaymentRuntime_PoolThenConfirmed_ToFunded(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	runtime := newObservedPaymentRuntimeStub()

	payMon := NewGuestPaymentMonitor(db, svc, nil)
	route := bindObservedPaymentRuntime(t, payMon, runtime, "v1")
	payMon.confirmationInterval = 50 * time.Millisecond
	defer payMon.StopAll()

	token := "gst_external_full_lifecycle"
	orderSeed := models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    externalObservedCoinType,
		PaymentAddress: "external_subaddr_test",
		PaymentAmount:  "1000000000000",
		SweepToAddress: "external_seller_main",
		AddressIndex:   5,
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(10 * time.Second),
	}
	setGuestOrderPaymentRoute(&orderSeed, route, 10*time.Second)
	seedGuestOrder(t, db, 700, orderSeed)

	order := loadGuestOrder(t, db, token)
	payMon.WatchOrder(&order)
	require.Eventually(t, func() bool { return runtime.isWatching(5) }, time.Second, 10*time.Millisecond)

	// Phase 1: inject pool transfer → pool detection populates PoolTxHash
	// but state stays AWAITING_PAYMENT.
	runtime.emit(5, distribution.ExternalPaymentEvent{
		Status: distribution.ExternalPaymentPending, LastTxHash: "external_tx_pool_001",
		TotalPending: 1_000_000_000_000,
	})

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.PoolTxHash == "external_tx_pool_001"
	}, 3*time.Second, 50*time.Millisecond,
		"pool transfer should populate PoolTxHash")

	o := loadGuestOrder(t, db, token)
	assert.Equal(t, models.GuestOrderAwaitingPayment, o.State,
		"pool detection must not advance state past AWAITING_PAYMENT")

	// Phase 2: confirm the transfer on-chain → state transitions to
	// PAYMENT_DETECTED.
	runtime.setHeight(105)
	runtime.emit(5, distribution.ExternalPaymentEvent{
		Status: distribution.ExternalPaymentConfirmed, LastTxHash: "external_tx_pool_001",
		TotalConfirmed: 1_000_000_000_000, MaxBlockHeight: 105,
	})

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.State == models.GuestOrderPaymentDetected
	}, 3*time.Second, 50*time.Millisecond,
		"confirmed transfer should move order to PAYMENT_DETECTED")

	o = loadGuestOrder(t, db, token)
	assert.Equal(t, "external_tx_pool_001", o.PaymentTxHash)
	assert.Equal(t, uint64(105), o.MoneroTxHeight)

	// Phase 3: advance chain height so confirmations reach threshold →
	// order transitions to FUNDED + sweep task created.
	//
	// confirmations = currentHeight - txHeight + 1
	// We need ≥ 3 confs, so height ≥ 105 + 2 = 107.
	runtime.setHeight(108)

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.State == models.GuestOrderFunded
	}, 5*time.Second, 50*time.Millisecond,
		"order should reach FUNDED after sufficient confirmations")

	o = loadGuestOrder(t, db, token)
	assert.NotNil(t, o.FundedAt)

	var taskCount int64
	require.NoError(t, db.gormDB.Model(&models.SweepTask{}).Where("order_token = ?", token).Count(&taskCount).Error)
	assert.Zero(t, taskCount)
}

// TestExternalPaymentRuntime_DirectConfirmNoPool verifies externally observed orders that skip the
// pool stage (tx lands in a block before the first poll) still transition
// correctly through PAYMENT_DETECTED → FUNDED.
func TestExternalPaymentRuntime_DirectConfirmNoPool(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	runtime := newObservedPaymentRuntimeStub()

	payMon := NewGuestPaymentMonitor(db, svc, nil)
	route := bindObservedPaymentRuntime(t, payMon, runtime, "v1")
	payMon.confirmationInterval = 50 * time.Millisecond
	defer payMon.StopAll()

	token := "gst_external_direct_conf"
	orderSeed := models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    externalObservedCoinType,
		PaymentAddress: "external_subaddr_direct",
		PaymentAmount:  "500000000000",
		SweepToAddress: "external_seller_direct",
		AddressIndex:   7,
		RequiredConfs:  2,
		ExpiresAt:      time.Now().Add(10 * time.Second),
	}
	setGuestOrderPaymentRoute(&orderSeed, route, 10*time.Second)
	seedGuestOrder(t, db, 701, orderSeed)

	order := loadGuestOrder(t, db, token)
	payMon.WatchOrder(&order)
	require.Eventually(t, func() bool { return runtime.isWatching(7) }, time.Second, 10*time.Millisecond)

	// Inject a confirmed transfer directly (no pool stage).
	runtime.setHeight(110)
	runtime.emit(7, distribution.ExternalPaymentEvent{
		Status: distribution.ExternalPaymentConfirmed, LastTxHash: "external_tx_direct",
		TotalConfirmed: 500_000_000_000, MaxBlockHeight: 110,
	})

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.State == models.GuestOrderPaymentDetected
	}, 3*time.Second, 50*time.Millisecond,
		"confirmed transfer should move order to PAYMENT_DETECTED")

	// Advance height for 2 confs (height=111, confs = 111-110+1 = 2).
	runtime.setHeight(112)

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.State == models.GuestOrderFunded
	}, 5*time.Second, 50*time.Millisecond,
		"order should reach FUNDED after 2 confirmations")

	var taskCount int64
	require.NoError(t, db.gormDB.Model(&models.SweepTask{}).Where("order_token = ?", token).Count(&taskCount).Error)
	assert.Zero(t, taskCount)
}

// TestExternalPaymentRuntime_InsufficientPayment verifies that a partial externally observed
// payment does not transition the order to PAYMENT_DETECTED.
func TestExternalPaymentRuntime_InsufficientPayment(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	runtime := newObservedPaymentRuntimeStub()

	payMon := NewGuestPaymentMonitor(db, svc, nil)
	route := bindObservedPaymentRuntime(t, payMon, runtime, "v1")
	defer payMon.StopAll()

	token := "gst_external_partial"
	orderSeed := models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    externalObservedCoinType,
		PaymentAddress: "external_subaddr_partial",
		PaymentAmount:  "2000000000000",
		SweepToAddress: "external_seller_partial",
		AddressIndex:   9,
		RequiredConfs:  10,
		ExpiresAt:      time.Now().Add(10 * time.Second),
	}
	setGuestOrderPaymentRoute(&orderSeed, route, 10*time.Second)
	seedGuestOrder(t, db, 702, orderSeed)

	order := loadGuestOrder(t, db, token)
	payMon.WatchOrder(&order)
	require.Eventually(t, func() bool { return runtime.isWatching(9) }, time.Second, 10*time.Millisecond)

	// A pending amount below the expected threshold must not advance state.
	runtime.emit(9, distribution.ExternalPaymentEvent{
		Status: distribution.ExternalPaymentPending, LastTxHash: "external_tx_small",
		TotalPending: 500_000_000_000,
	})

	// Wait a few poll cycles — state should remain unchanged.
	time.Sleep(300 * time.Millisecond)

	o := loadGuestOrder(t, db, token)
	assert.Equal(t, models.GuestOrderAwaitingPayment, o.State,
		"partial payment must not advance state")
}
