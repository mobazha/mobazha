package core

import (
	"context"
	"testing"

	utils "github.com/mobazha/mobazha/internal/orders/testutil"
	"github.com/mobazha/mobazha/internal/repo"
	pkgdatabase "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDBOrderQuerierGuestMetadataHonorsGuestContractType(t *testing.T) {
	db, raw := newDigitalGuestTestDatabase(t)
	require.NoError(t, raw.AutoMigrate(&models.GuestOrder{}, &models.GuestOrderItem{}))

	require.NoError(t, raw.Create(&models.GuestOrder{
		TenantMixin: models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
		ID:          1,
		OrderToken:  "gst_physical",
		State:       models.GuestOrderFunded,
		Items: []models.GuestOrderItem{{
			TenantMixin:  models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
			ID:           1,
			OrderToken:   "gst_physical",
			ListingSlug:  "poster",
			ContractType: "PHYSICAL_GOOD",
			Quantity:     1,
		}},
	}).Error)
	require.NoError(t, raw.Create(&models.GuestOrder{
		TenantMixin: models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
		ID:          2,
		OrderToken:  "gst_digital",
		State:       models.GuestOrderFunded,
		Items: []models.GuestOrderItem{{
			TenantMixin:  models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
			ID:           2,
			OrderToken:   "gst_digital",
			ListingSlug:  "ebook",
			ContractType: "DIGITAL_GOOD",
			VariantSKU:   "sku-blue",
			Quantity:     2,
		}},
	}).Error)

	q := &dbOrderQuerier{db: db}
	physical, err := q.GetOrderMetadata("gst_physical")
	require.NoError(t, err)
	require.Equal(t, "PHYSICAL_GOOD", physical.ContractType)

	digital, err := q.GetOrderMetadata("gst_digital")
	require.NoError(t, err)
	require.Equal(t, "DIGITAL_GOOD", digital.ContractType)
	require.Len(t, digital.LineItems, 1)
	require.Equal(t, "sku-blue", digital.LineItems[0].VariantSKU)
	require.Equal(t, uint32(2), digital.LineItems[0].Quantity)
}

func TestDBOrderQuerierStandardMetadataResolvesVariantSKU(t *testing.T) {
	db, raw := newDigitalGuestTestDatabase(t)
	require.NoError(t, raw.AutoMigrate(&models.Order{}))

	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
		ID:          models.OrderID("order_standard_variant"),
		MyRole:      string(models.RoleVendor),
		Open:        true,
	}
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug: "ebook",
				Metadata: &pb.Listing_Metadata{
					ContractType: pb.Listing_Metadata_DIGITAL_GOOD,
				},
				Item: &pb.Listing_Item{
					Options: []*pb.Listing_Item_Option{{
						Name: "Color",
					}},
					Skus: []*pb.Listing_Item_Sku{{
						ProductID: "sku-blue",
						Selections: []*pb.Listing_Item_Sku_Selection{{
							Option:  "Color",
							Variant: "Blue",
						}},
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			Quantity: "2",
			Options: []*pb.OrderOpen_Item_Option{{
				Name:  "Color",
				Value: "Blue",
			}},
		}},
	}
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(orderOpen)))
	require.NoError(t, raw.Create(order).Error)

	q := &dbOrderQuerier{db: db}
	meta, err := q.GetOrderMetadata("order_standard_variant")
	require.NoError(t, err)
	require.Equal(t, "DIGITAL_GOOD", meta.ContractType)
	require.Len(t, meta.LineItems, 1)
	require.Equal(t, "ebook", meta.LineItems[0].ListingSlug)
	require.Equal(t, "sku-blue", meta.LineItems[0].VariantSKU)
	require.Equal(t, uint32(2), meta.LineItems[0].Quantity)
}

func TestDBOrderQuerierGuestMetadataMissingContractTypeStaysMissing(t *testing.T) {
	db, raw := newDigitalGuestTestDatabase(t)
	require.NoError(t, raw.AutoMigrate(&models.GuestOrder{}, &models.GuestOrderItem{}))

	require.NoError(t, raw.Create(&models.GuestOrder{
		TenantMixin: models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
		ID:          1,
		OrderToken:  "gst_missing_type",
		State:       models.GuestOrderFunded,
		Items: []models.GuestOrderItem{{
			TenantMixin: models.TenantMixin{TenantID: pkgdatabase.StandaloneTenantID},
			ID:          1,
			OrderToken:  "gst_missing_type",
			ListingSlug: "download",
			Quantity:    1,
		}},
	}).Error)

	q := &dbOrderQuerier{db: db}
	meta, err := q.GetOrderMetadata("gst_missing_type")
	require.NoError(t, err)
	require.Empty(t, meta.ContractType)
}

func TestDigitalOrderShipperRoutesGuestOrdersToGuestService(t *testing.T) {
	guest := &recordingGuestDigitalShipper{}
	order := &recordingDigitalOrderShipper{}
	shipper := newDigitalOrderShipper(order, guest)

	require.NoError(t, shipper.ShipOrder(models.OrderID("gst_order"), nil, nil))
	require.Equal(t, "gst_order", guest.token)
	require.Equal(t, "digital", guest.carrier)
	require.False(t, order.called)

	require.NoError(t, shipper.ShipOrder(models.OrderID("ob_order"), nil, nil))
	require.True(t, order.called)
	require.Equal(t, models.OrderID("ob_order"), order.orderID)
}

type recordingGuestDigitalShipper struct {
	token   string
	carrier string
}

func (s *recordingGuestDigitalShipper) ShipGuestOrder(_ context.Context, token string, _, carrier string) error {
	s.token = token
	s.carrier = carrier
	return nil
}

type recordingDigitalOrderShipper struct {
	called  bool
	orderID models.OrderID
}

func (s *recordingDigitalOrderShipper) ShipOrder(orderID models.OrderID, _ []models.Shipment, _ chan struct{}) error {
	s.called = true
	s.orderID = orderID
	return nil
}

func newDigitalGuestTestDatabase(t *testing.T) (pkgdatabase.Database, *gorm.DB) {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)
	return db, raw
}
