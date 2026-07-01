package core

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type immutableWatchTestProjector struct {
	watch distribution.ManagedEscrowWatch
}

func (p immutableWatchTestProjector) PrepareManagedEscrowGuestFunding(context.Context, distribution.ManagedEscrowGuestFundingRequest) (distribution.ManagedEscrowGuestFundingTarget, error) {
	return distribution.ManagedEscrowGuestFundingTarget{}, nil
}

func (p immutableWatchTestProjector) ProjectManagedEscrowGuestWatch(context.Context, distribution.ManagedEscrowGuestProjection) (distribution.ManagedEscrowWatch, error) {
	return p.watch, nil
}

func (p immutableWatchTestProjector) ProjectManagedEscrowGuestSettlement(context.Context, distribution.ManagedEscrowGuestProjection) (distribution.ManagedEscrowGuestSettlementRequest, error) {
	return distribution.ManagedEscrowGuestSettlementRequest{}, nil
}

func TestDistributionManagedEscrowWatchSourceRejectsProjectorMutation(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	require.NoError(t, rawProvider.RawDB().AutoMigrate(&models.GuestOrder{}))
	order := models.GuestOrder{
		OrderToken: "gst_immutable_watch", State: models.GuestOrderAwaitingPayment,
		PaymentCoin: "crypto:eip155:1:native", PaymentAmount: "1000",
		PaymentAddress: "0x1111111111111111111111111111111111111111", ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, order.SetManagedEscrowGuestMetadata([]byte(`{"provider":"test"}`)))
	require.NoError(t, rawProvider.RawDB().Create(&order).Error)

	source := &distributionManagedEscrowWatchSource{
		node: &MobazhaNode{storageFields: storageFields{db: db}},
		projector: immutableWatchTestProjector{watch: distribution.ManagedEscrowWatch{
			OrderID: "different-order", Chain: "ETH", ChainID: 1,
			Address: order.PaymentAddress, ExpectedAmount: order.PaymentAmount, Deadline: order.ExpiresAt,
		}},
	}
	watches, err := source.guestOrderWatches(context.Background())
	require.NoError(t, err)
	require.Empty(t, watches)
}

var _ distribution.ManagedEscrowGuestProjector = immutableWatchTestProjector{}
