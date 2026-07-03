package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestSupplyAvailabilityAppService_QuoteAggregatesLinesAndManualAction(t *testing.T) {
	provider := &recordingSupplyProvider{
		kind: contracts.SupplyKindSkuQuantity,
		availabilityFunc: func(_ context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
			return &contracts.AvailabilityResult{
				LineID:            req.Line.LineID,
				SupplyKind:        req.Line.SupplyKind,
				Status:            contracts.SupplyAvailabilityLowStock,
				Available:         true,
				AvailableQuantity: int64(req.Line.Quantity),
			}, nil
		},
	}
	service, err := NewSupplyAvailabilityAppService(provider)
	require.NoError(t, err)

	result, err := service.Quote(context.Background(), contracts.SupplyQuoteRequest{
		BuyerPeerID: "buyer-1",
		Lines: []contracts.SupplyLine{
			trackedSkuLine("camera", "red", 1, 5),
			trackedSkuLine("camera", "red", 2, 5),
		},
	})

	require.NoError(t, err)
	require.True(t, result.CanSell)
	require.Len(t, result.Results, 1)
	require.Equal(t, int64(3), result.Results[0].AvailableQuantity)
	require.Len(t, provider.availabilityRequests, 1)
	require.Equal(t, 3, provider.availabilityRequests[0].Line.Quantity)
	require.Equal(t, "buyer-1", provider.availabilityRequests[0].BuyerPeerID)
}

func TestSupplyAvailabilityAppService_QuoteRejectsUnsupportedKind(t *testing.T) {
	service, err := NewSupplyAvailabilityAppService()
	require.NoError(t, err)

	_, err = service.Quote(context.Background(), contracts.SupplyQuoteRequest{
		Lines: []contracts.SupplyLine{{
			ListingSlug: "camera",
			Quantity:    1,
			SupplyKind:  contracts.SupplyKind("unknown_kind"),
		}},
	})

	require.ErrorIs(t, err, contracts.ErrSupplyKindUnsupported)
}

func TestSupplyAvailabilityAppService_ReserveOrderReleasesPriorReservationsOnFailure(t *testing.T) {
	skuProvider := &recordingSupplyProvider{
		kind: contracts.SupplyKindSkuQuantity,
		reserveFunc: func(_ context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
			return &contracts.SupplyReservation{
				OrderRef:    req.OrderRef,
				OrderType:   req.OrderType,
				LineID:      req.Line.LineID,
				SupplyKind:  req.Line.SupplyKind,
				ListingSlug: req.Line.ListingSlug,
				Quantity:    req.Line.Quantity,
				Status:      contracts.SupplyReservationReserved,
			}, nil
		},
	}
	unlimitedProvider := &recordingSupplyProvider{
		kind: contracts.SupplyKindUnlimitedDigital,
		reserveFunc: func(context.Context, contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
			return nil, contracts.ErrSupplyManualActionRequired
		},
	}
	service, err := NewSupplyAvailabilityAppService(skuProvider, unlimitedProvider)
	require.NoError(t, err)

	_, err = service.ReserveOrder(context.Background(), contracts.ReserveOrderSupplyRequest{
		OrderRef:  "order-1",
		OrderType: "guest",
		ExpiresAt: time.Now().Add(time.Hour),
		Lines: []contracts.SupplyLine{
			{
				LineID:      "physical",
				ListingSlug: "camera",
				Quantity:    1,
				SupplyKind:  contracts.SupplyKindSkuQuantity,
			},
			{
				LineID:      "download",
				ListingSlug: "ebook",
				Quantity:    1,
				SupplyKind:  contracts.SupplyKindUnlimitedDigital,
			},
		},
	})

	require.ErrorIs(t, err, contracts.ErrSupplyManualActionRequired)
	require.Len(t, skuProvider.reserveRequests, 1)
	require.Len(t, unlimitedProvider.reserveRequests, 1)
	require.Len(t, skuProvider.releaseRequests, 1)
	require.Equal(t, "reserve_failed", skuProvider.releaseRequests[0].Reason)
}

func TestSupplyAvailabilityAppService_CommitAndReleaseFanOut(t *testing.T) {
	skuProvider := &recordingSupplyProvider{kind: contracts.SupplyKindSkuQuantity}
	unlimitedProvider := &recordingSupplyProvider{kind: contracts.SupplyKindUnlimitedDigital}
	service, err := NewSupplyAvailabilityAppService(unlimitedProvider, skuProvider)
	require.NoError(t, err)

	require.NoError(t, service.CommitOrder(context.Background(), "order-1", "guest"))
	require.NoError(t, service.ReleaseOrder(context.Background(), "order-1", "guest", "cancelled"))

	require.Len(t, skuProvider.commitRequests, 1)
	require.Len(t, unlimitedProvider.commitRequests, 1)
	require.Len(t, skuProvider.releaseRequests, 1)
	require.Len(t, unlimitedProvider.releaseRequests, 1)
	require.Equal(t, "cancelled", skuProvider.releaseRequests[0].Reason)
	require.Equal(t, "cancelled", unlimitedProvider.releaseRequests[0].Reason)
}

func TestSupplyAvailabilityAppService_ReserveOrderTxUsesOuterTransaction(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	service, err := NewSupplyAvailabilityAppService(NewSkuQuantityProvider(db))
	require.NoError(t, err)

	errBoom := errors.New("rollback outer transaction")
	err = db.Update(func(tx database.Tx) error {
		result, err := service.ReserveOrderTx(context.Background(), tx, contracts.ReserveOrderSupplyRequest{
			OrderRef:  "guest-1",
			OrderType: models.OrderTypeGuest,
			ExpiresAt: time.Now().Add(time.Hour),
			Lines: []contracts.SupplyLine{
				trackedSkuLine("camera", "red", 1, 5),
			},
		})
		require.NoError(t, err)
		require.Len(t, result.Reservations, 1)
		return errBoom
	})
	require.ErrorIs(t, err, errBoom)

	var count int64
	require.NoError(t, db.gormDB.Model(&models.InventoryReservation{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplyAvailabilityAppService_CommitAndReleaseOrderTxUseOuterTransaction(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	service, err := NewSupplyAvailabilityAppService(NewSkuQuantityProvider(db))
	require.NoError(t, err)
	now := time.Now()
	seedSkuReservation(t, db, 1, models.InventoryReservation{
		OrderRef:    "guest-1",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "red",
		Quantity:    1,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return service.CommitOrderTx(context.Background(), tx, "guest-1", models.OrderTypeGuest)
	}))
	row := loadSkuReservation(t, db, 1)
	require.True(t, row.Confirmed)
	require.Equal(t, 2099, row.ExpiresAt.Year())

	seedSkuReservation(t, db, 2, models.InventoryReservation{
		OrderRef:    "guest-2",
		OrderType:   models.OrderTypeGuest,
		ListingSlug: "camera",
		VariantHash: "blue",
		Quantity:    1,
		ReservedAt:  now,
		ExpiresAt:   now.Add(time.Hour),
	})
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return service.ReleaseOrderTx(context.Background(), tx, "guest-2", models.OrderTypeGuest, "expired")
	}))
	row = loadSkuReservation(t, db, 2)
	require.NotNil(t, row.ReleasedAt)
}

func TestSupplyAvailabilityAppService_StandardAndGuestCompeteForSameSkuBucket(t *testing.T) {
	db := newFeatureTestDatabase(t, &models.InventoryReservation{})
	service, err := NewSupplyAvailabilityAppService(NewSkuQuantityProvider(db))
	require.NoError(t, err)
	expiresAt := time.Now().Add(time.Hour)
	standardLines := []contracts.SupplyLine{
		trackedSkuLine("camera", "red", 1, 2),
		trackedSkuLine("camera", "red", 1, 2),
	}

	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, err := service.ReserveOrderTx(context.Background(), tx, contracts.ReserveOrderSupplyRequest{
			OrderRef:  "guest-1",
			OrderType: models.OrderTypeGuest,
			ExpiresAt: expiresAt,
			Lines: []contracts.SupplyLine{
				trackedSkuLine("camera", "red", 1, 2),
			},
		})
		return err
	}))

	_, err = reserveOrderSupplyInTxForTest(t, db, service, contracts.ReserveOrderSupplyRequest{
		OrderRef:  "standard-1",
		OrderType: models.OrderTypeStandard,
		ExpiresAt: expiresAt,
		Lines:     standardLines,
	})
	require.ErrorIs(t, err, contracts.ErrSupplyUnavailable)
	var count int64
	require.NoError(t, db.gormDB.Model(&models.InventoryReservation{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	quote, err := service.Quote(context.Background(), contracts.SupplyQuoteRequest{
		Lines: []contracts.SupplyLine{
			trackedSkuLine("camera", "red", 1, 2),
		},
	})
	require.NoError(t, err)
	require.True(t, quote.CanSell)
	require.Len(t, quote.Results, 1)
	require.Equal(t, int64(1), quote.Results[0].AvailableQuantity)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return service.ReleaseOrderTx(context.Background(), tx, "guest-1", models.OrderTypeGuest, "cancelled")
	}))
	result, err := reserveOrderSupplyInTxForTest(t, db, service, contracts.ReserveOrderSupplyRequest{
		OrderRef:  "standard-1",
		OrderType: models.OrderTypeStandard,
		ExpiresAt: expiresAt,
		Lines:     standardLines,
	})
	require.NoError(t, err)
	require.Len(t, result.Reservations, 1)
	require.Equal(t, 2, result.Reservations[0].Quantity)

	var reservations []models.InventoryReservation
	require.NoError(t, db.gormDB.Order("id").Find(&reservations).Error)
	require.Len(t, reservations, 2)
	require.Equal(t, "guest-1", reservations[0].OrderRef)
	require.NotNil(t, reservations[0].ReleasedAt)
	require.Equal(t, "standard-1", reservations[1].OrderRef)
	require.Equal(t, models.OrderTypeStandard, reservations[1].OrderType)
	require.Equal(t, 2, reservations[1].Quantity)
	require.Nil(t, reservations[1].ReleasedAt)
}

func TestSupplyAvailabilityAppService_RejectsDuplicateProvider(t *testing.T) {
	_, err := NewSupplyAvailabilityAppService(
		&recordingSupplyProvider{kind: contracts.SupplyKindSkuQuantity},
		&recordingSupplyProvider{kind: contracts.SupplyKindSkuQuantity},
	)

	require.Error(t, err)
}

func reserveOrderSupplyInTxForTest(t *testing.T, db *featureTestDatabase, service *SupplyAvailabilityAppService, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	t.Helper()
	var result *contracts.ReserveOrderSupplyResult
	err := db.Update(func(tx database.Tx) error {
		var reserveErr error
		result, reserveErr = service.ReserveOrderTx(context.Background(), tx, req)
		return reserveErr
	})
	return result, err
}

type recordingSupplyProvider struct {
	kind contracts.SupplyKind

	availabilityFunc func(context.Context, contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error)
	reserveFunc      func(context.Context, contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error)
	commitFunc       func(context.Context, contracts.CommitSupplyRequest) error
	releaseFunc      func(context.Context, contracts.ReleaseSupplyRequest) error

	availabilityRequests []contracts.AvailabilityRequest
	reserveRequests      []contracts.ReserveSupplyRequest
	commitRequests       []contracts.CommitSupplyRequest
	releaseRequests      []contracts.ReleaseSupplyRequest
}

func (p *recordingSupplyProvider) Kind() contracts.SupplyKind {
	return p.kind
}

func (p *recordingSupplyProvider) GetAvailability(ctx context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
	p.availabilityRequests = append(p.availabilityRequests, req)
	if p.availabilityFunc != nil {
		return p.availabilityFunc(ctx, req)
	}
	return &contracts.AvailabilityResult{
		LineID:     req.Line.LineID,
		SupplyKind: req.Line.SupplyKind,
		Status:     contracts.SupplyAvailabilityAvailable,
		Available:  true,
	}, nil
}

func (p *recordingSupplyProvider) Reserve(ctx context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	p.reserveRequests = append(p.reserveRequests, req)
	if p.reserveFunc != nil {
		return p.reserveFunc(ctx, req)
	}
	return &contracts.SupplyReservation{
		OrderRef:    req.OrderRef,
		OrderType:   req.OrderType,
		LineID:      req.Line.LineID,
		SupplyKind:  req.Line.SupplyKind,
		ListingSlug: req.Line.ListingSlug,
		Quantity:    req.Line.Quantity,
		Status:      contracts.SupplyReservationReserved,
	}, nil
}

func (p *recordingSupplyProvider) Commit(ctx context.Context, req contracts.CommitSupplyRequest) error {
	p.commitRequests = append(p.commitRequests, req)
	if p.commitFunc != nil {
		return p.commitFunc(ctx, req)
	}
	return nil
}

func (p *recordingSupplyProvider) Release(ctx context.Context, req contracts.ReleaseSupplyRequest) error {
	p.releaseRequests = append(p.releaseRequests, req)
	if p.releaseFunc != nil {
		return p.releaseFunc(ctx, req)
	}
	return nil
}
