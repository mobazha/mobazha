package guest

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestGetGuestOrderStatusResolvesTenantSellerPeerID(t *testing.T) {
	db := newGuestTestDB(t)
	const tenantID = "tenant-uuid"
	const sellerPeerID = "12D3KooWDGT8UHgtjPGdiS8k9P3C7KM8kaXFzd75ZmBuobJ71xCu"

	order := models.GuestOrder{
		TenantMixin:       models.TenantMixin{TenantID: tenantID},
		ID:                1,
		OrderToken:        "gst_status_seller",
		State:             models.GuestOrderFunded,
		PaymentAddress:    "addr",
		PaymentAmount:     "100",
		PaymentCoin:       "crypto:eip155:1:native",
		PriceCurrency:     "USD",
		PriceDivisibility: 2,
		ExpiresAt:         time.Now().Add(time.Hour),
		Items: []models.GuestOrderItem{
			{
				TenantMixin:  models.TenantMixin{TenantID: tenantID},
				ID:           1,
				OrderToken:   "gst_status_seller",
				ListingSlug:  "planner",
				ListingTitle: "Planner",
				SellerPeerID: tenantID,
				Quantity:     1,
				UnitPrice:    100,
				ItemTotal:    100,
			},
		},
	}
	require.NoError(t, db.gormDB.Create(&order).Error)

	svc := NewGuestOrderAppService(GuestOrderAppServiceConfig{
		DB:     db,
		NodeID: tenantID,
		PeerID: sellerPeerID,
	})
	status, err := svc.GetGuestOrderStatus(context.Background(), order.OrderToken)
	require.NoError(t, err)
	require.Equal(t, sellerPeerID, status.SellerPeerID)
	require.Len(t, status.Items, 1)
	require.Equal(t, sellerPeerID, status.Items[0].SellerPeerID)
}

func TestResolveSellerPeerIDKeepsStandaloneFallback(t *testing.T) {
	svc := NewGuestOrderAppService(GuestOrderAppServiceConfig{})
	require.Equal(t, repo.DefaultNodeID, svc.resolveSellerPeerID(database.StandaloneTenantID, repo.DefaultNodeID))
}

func TestResolveSellerPeerIDPrefersConfiguredPeerID(t *testing.T) {
	const sellerPeerID = "12D3KooWDGT8UHgtjPGdiS8k9P3C7KM8kaXFzd75ZmBuobJ71xCu"
	svc := NewGuestOrderAppService(GuestOrderAppServiceConfig{PeerID: sellerPeerID})
	require.Equal(t, sellerPeerID, svc.resolveSellerPeerID("tenant-uuid", "tenant-uuid"))
}
