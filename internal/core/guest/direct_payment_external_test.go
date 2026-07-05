// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type directPaymentExternalRuntimeStub struct {
	mu       sync.Mutex
	requests []distribution.ExternalPaymentAddressRequest
	address  distribution.ExternalPaymentAddress
	failures int
	before   func(distribution.ExternalPaymentAddressRequest)
}

func (*directPaymentExternalRuntimeStub) Start(context.Context) error { return nil }
func (*directPaymentExternalRuntimeStub) Close() error                { return nil }
func (*directPaymentExternalRuntimeStub) PaymentHealth(context.Context) distribution.ExternalPaymentHealth {
	return distribution.ExternalPaymentHealth{State: distribution.ExternalPaymentReady}
}
func (s *directPaymentExternalRuntimeStub) EnsurePaymentAddress(_ context.Context, request distribution.ExternalPaymentAddressRequest) (distribution.ExternalPaymentAddress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
	if s.before != nil {
		s.before(request)
	}
	if s.failures > 0 {
		s.failures--
		return distribution.ExternalPaymentAddress{}, errors.New("temporary allocation failure")
	}
	if s.address.Address == "" {
		s.address = distribution.ExternalPaymentAddress{Address: "external-address-7", Index: 7, RequiredConfirmations: 3}
	}
	return s.address, nil
}

func (s *directPaymentExternalRuntimeStub) requestSnapshot() []distribution.ExternalPaymentAddressRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]distribution.ExternalPaymentAddressRequest(nil), s.requests...)
}
func (*directPaymentExternalRuntimeStub) WatchPayment(*distribution.ExternalPaymentWatch) error {
	return nil
}
func (*directPaymentExternalRuntimeStub) UnwatchPayment(uint32)              {}
func (*directPaymentExternalRuntimeStub) ReapPayment(uint32)                 {}
func (*directPaymentExternalRuntimeStub) PaymentPollInterval() time.Duration { return time.Second }
func (*directPaymentExternalRuntimeStub) PaymentGracePeriod(iwallet.CoinType) time.Duration {
	return time.Hour
}
func (*directPaymentExternalRuntimeStub) PaymentHeight(context.Context) (uint64, error) {
	return 1, nil
}
func TestDirectPaymentService_ExternalRuntimeAllocatesAddress(t *testing.T) {
	db := newGuestTestDB(t)
	runtime := &directPaymentExternalRuntimeStub{}
	service := NewDirectPaymentService(db, nil)
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: "crypto:monero:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	service.SetExternalPaymentRuntimeCatalog(catalog)

	runtime.before = func(request distribution.ExternalPaymentAddressRequest) {
		var attempt models.PaymentAttempt
		require.NoError(t, db.gormDB.Where("idempotency_key = ?", request.IdempotencyKey).First(&attempt).Error)
		require.Equal(t, models.PaymentAttemptPendingExternal, attempt.State)
	}
	request := PaymentAddressRequest{
		CoinType:   iwallet.CoinType("crypto:monero:mainnet:native"),
		OrderToken: "gst_order_7",
		Amount:     "1000000000000",
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	result, err := service.GeneratePaymentAddress(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "external-address-7", result.Address)
	require.Equal(t, uint32(7), result.AddressIndex)
	require.Equal(t, 3, result.RequiredConfs)
	require.Equal(t, route, result.Route)
	require.Equal(t, time.Hour, result.GracePeriod)
	require.NotEmpty(t, result.AttemptID)
	requests := runtime.requestSnapshot()
	require.Len(t, requests, 1)
	require.NotEmpty(t, requests[0].IdempotencyKey)

	retried, err := service.GeneratePaymentAddress(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, result, retried)
	require.Len(t, runtime.requestSnapshot(), 1, "a committed external result must be reused without another RPC")

	var attempt models.PaymentAttempt
	require.NoError(t, db.gormDB.Where("attempt_id = ?", result.AttemptID).First(&attempt).Error)
	require.Equal(t, models.PaymentAttemptKindDirectObservedAddress, attempt.Kind)
	require.Equal(t, models.PaymentAttemptExternalCreated, attempt.State)
	require.Equal(t, result.Address, attempt.ExternalReference)
	require.Equal(t, result.AddressIndex, attempt.ExternalIndex)
	require.Equal(t, result.RequiredConfs, attempt.RequiredConfs)
}

func TestDirectPaymentService_ExternalRuntimeRecoversWithStableIdempotencyKey(t *testing.T) {
	db := newGuestTestDB(t)
	runtime := &directPaymentExternalRuntimeStub{failures: 1}
	service := NewDirectPaymentService(db, nil)
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(distribution.PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: "crypto:monero:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	service.SetExternalPaymentRuntimeCatalog(catalog)

	request := PaymentAddressRequest{
		CoinType: iwallet.CoinType(route.AssetID), Amount: "200", OrderToken: "gst_recover_8", ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err := service.GeneratePaymentAddress(context.Background(), request)
	require.ErrorContains(t, err, "temporary allocation failure")
	requests := runtime.requestSnapshot()
	require.Len(t, requests, 1)
	stableKey := requests[0].IdempotencyKey

	var pending models.PaymentAttempt
	require.NoError(t, db.gormDB.Where("order_id = ?", request.OrderToken).First(&pending).Error)
	require.Equal(t, models.PaymentAttemptReconcileRequired, pending.State)
	require.NotEmpty(t, pending.LastError)

	recovered, err := service.RecoverPendingExternalPaymentAddresses(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	requests = runtime.requestSnapshot()
	require.Len(t, requests, 2)
	require.Equal(t, stableKey, requests[1].IdempotencyKey)

	require.NoError(t, db.gormDB.Where("attempt_id = ?", pending.AttemptID).First(&pending).Error)
	require.Equal(t, models.PaymentAttemptExternalCreated, pending.State)
	require.Equal(t, "external-address-7", pending.ExternalReference)
	require.Empty(t, pending.LastError)
}

func TestDirectPaymentService_RecoveryUsesHistoricalRuntime(t *testing.T) {
	db := newGuestTestDB(t)
	oldRuntime := &directPaymentExternalRuntimeStub{address: distribution.ExternalPaymentAddress{Address: "historical-address", Index: 41}}
	newRuntime := &directPaymentExternalRuntimeStub{address: distribution.ExternalPaymentAddress{Address: "current-address", Index: 42}}
	service := NewDirectPaymentService(db, nil)
	oldRoute := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct", ImplementationGeneration: "v1",
		RailKind: string(distribution.PaymentRailDirectObserved), NetworkID: "TEST",
		AssetID: "crypto:monero:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	newRoute := oldRoute
	newRoute.ImplementationGeneration = "v2"
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: oldRoute, Runtime: oldRuntime}))
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: newRoute, Runtime: newRuntime, ActiveForNewWork: true}))
	service.SetExternalPaymentRuntimeCatalog(catalog)

	attempt := models.PaymentAttempt{
		TenantID: testTenantID, AttemptID: "pa_historical", Kind: models.PaymentAttemptKindDirectObservedAddress,
		PaymentSessionID: "guest:gst_historical", OrderID: "gst_historical", AmountValue: "300",
		RouteBindingID: "prb_historical", IdempotencyKey: "dpa_historical", State: models.PaymentAttemptPendingExternal,
	}
	binding := paymentRouteBindingFromIdentity(attempt.RouteBindingID, attempt.AttemptID, oldRoute)
	binding.TenantID = testTenantID
	require.NoError(t, db.gormDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		return tx.Create(&attempt).Error
	}))

	recovered, err := service.RecoverPendingExternalPaymentAddresses(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Len(t, oldRuntime.requestSnapshot(), 1)
	require.Empty(t, newRuntime.requestSnapshot())
	require.NoError(t, db.gormDB.Where("attempt_id = ?", attempt.AttemptID).First(&attempt).Error)
	require.Equal(t, "historical-address", attempt.ExternalReference)
}

func TestDirectPaymentService_RecoveryDoesNotAllocateExpiredAttempt(t *testing.T) {
	db := newGuestTestDB(t)
	runtime := &directPaymentExternalRuntimeStub{}
	service := NewDirectPaymentService(db, nil)
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct", ImplementationGeneration: "v1",
		RailKind: string(distribution.PaymentRailDirectObserved), NetworkID: "TEST",
		AssetID: "crypto:monero:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	service.SetExternalPaymentRuntimeCatalog(catalog)

	expiresAt := time.Now().Add(-time.Minute)
	attempt := models.PaymentAttempt{
		TenantID: testTenantID, AttemptID: "pa_expired", Kind: models.PaymentAttemptKindDirectObservedAddress,
		PaymentSessionID: "guest:gst_expired", OrderID: "gst_expired", AmountValue: "300",
		RouteBindingID: "prb_expired", IdempotencyKey: "dpa_expired",
		State: models.PaymentAttemptPendingExternal, ExpiresAt: &expiresAt,
	}
	binding := paymentRouteBindingFromIdentity(attempt.RouteBindingID, attempt.AttemptID, route)
	binding.TenantID = testTenantID
	require.NoError(t, db.gormDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		return tx.Create(&attempt).Error
	}))

	recovered, err := service.RecoverPendingExternalPaymentAddresses(context.Background())
	require.NoError(t, err)
	require.Zero(t, recovered)
	require.Empty(t, runtime.requestSnapshot())
	require.NoError(t, db.gormDB.Where("attempt_id = ?", attempt.AttemptID).First(&attempt).Error)
	require.Equal(t, models.PaymentAttemptExpired, attempt.State)
}

func TestDirectPaymentService_ConcurrentRetriesShareDurableAttempt(t *testing.T) {
	db := newGuestTestDB(t)
	runtime := &directPaymentExternalRuntimeStub{}
	service := NewDirectPaymentService(db, nil)
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct", ImplementationGeneration: "v1",
		RailKind: string(distribution.PaymentRailDirectObserved), NetworkID: "TEST",
		AssetID: "crypto:monero:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	catalog := distribution.NewExternalPaymentRuntimeCatalog()
	require.NoError(t, catalog.Register(distribution.ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	service.SetExternalPaymentRuntimeCatalog(catalog)
	request := PaymentAddressRequest{
		CoinType: iwallet.CoinType(route.AssetID), Amount: "400", OrderToken: "gst_concurrent", ExpiresAt: time.Now().Add(time.Hour),
	}

	results := make(chan *PaymentAddressResult, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.GeneratePaymentAddress(context.Background(), request)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var expected *PaymentAddressResult
	for result := range results {
		if expected == nil {
			expected = result
			continue
		}
		require.Equal(t, expected, result)
	}
	require.NotNil(t, expected)
	var attemptCount, routeCount int64
	require.NoError(t, db.gormDB.Model(&models.PaymentAttempt{}).Where("order_id = ?", request.OrderToken).Count(&attemptCount).Error)
	require.NoError(t, db.gormDB.Model(&models.PaymentRouteBinding{}).Where("attempt_id = ?", expected.AttemptID).Count(&routeCount).Error)
	require.Equal(t, int64(1), attemptCount)
	require.Equal(t, int64(1), routeCount)
}
