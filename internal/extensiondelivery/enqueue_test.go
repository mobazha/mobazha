package extensiondelivery

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestEnqueuePaymentVerifiedTx_UsesLatestPersistedExtensionRevision(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	orderOpen := managedCollectibleOrderOpen()
	serialized, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)
	order := &models.Order{ID: "order-extension-event", SerializedOrderOpen: serialized}
	order.SetRole(models.RoleVendor)
	order.MarkPaymentVerified()
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		ToAddress: "settlement-1", Coin: "MATIC", Amount: "100",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{Method: pb.PaymentSent_CANCELABLE},
	}))
	first, ok, err := models.CollectibleOrderExtensionFromOrderOpen(order.ID.String(), orderOpen)
	require.NoError(t, err)
	require.True(t, ok)
	metadata, ok := models.CollectibleOrderMetadataFromExtension(first)
	require.True(t, ok)
	metadata.NFTMint = "mint-revision-2"
	second, err := extensions.NewOrderExtension(order.ID.String(), first.ProviderID, first.Type, first.SchemaVersion, first.ResourceID, metadata)
	require.NoError(t, err)
	second.ReservationRequired = first.ReservationRequired
	second.SettlementPolicy = first.SettlementPolicy
	second.LifecycleEvents = append([]string(nil), first.LifecycleEvents...)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), first); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), second); err != nil {
			return err
		}
		persisted, err := orderextensions.LatestByIDTx(tx, order.ID.String(), second.ExtensionID)
		if err != nil {
			return err
		}
		if err := orderextensions.RecordReservationTx(tx, extensions.ReservationRequest{
			OrderID: order.ID.String(), Extension: persisted, PaymentCoin: "MATIC", IdempotencyKey: "reserve:1", ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, extensions.Reservation{ID: "reservation-1", Version: 2, Status: "reserved"}); err != nil {
			return err
		}
		return EnqueuePaymentVerifiedTx(tx, order)
	}))

	var delivery models.ExtensionDelivery
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ?", order.ID.String()).First(&delivery).Error
	}))
	event := orderextensions.EventFromDelivery(delivery)
	require.Equal(t, extensions.EventOrderPaymentVerified, event.Type)
	var payload extensions.PaymentVerifiedEventPayload
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	require.Equal(t, uint64(2), payload.Extension.Revision)
	require.Equal(t, second.PayloadHash, payload.Extension.PayloadHash)
	require.Equal(t, "reservation-1", payload.Reservation.ReservationID)
	require.Equal(t, "settlement-1", payload.Settlement.SettlementID)
	require.NotEmpty(t, payload.Settlement.OrderStateVersion)
}

func TestEventForOrder_RejectsReservationForStaleExtensionRevision(t *testing.T) {
	order := &models.Order{ID: "order-stale-reservation-event"}
	extension, err := extensions.NewOrderExtension(order.ID.String(), "provider", "type", "v1", "resource", map[string]string{"value": "two"})
	require.NoError(t, err)
	extension.Revision = 2
	now := time.Now().UTC()
	reservation := &extensions.ReservationBinding{
		ReservationID: "reservation-stale", ReservationVersion: 1, ExtensionRevision: 1,
		Status: "reserved", PaymentCoin: "MATIC", IdempotencyKey: "reserve:stale",
		ExpiresAt: now.Add(time.Hour), RecordedAt: now,
	}

	_, err = eventForOrder(order, extension, reservation, extensions.EventOrderPaymentVerified, "")
	require.ErrorContains(t, err, "reservation revision 1 does not match extension revision 2")
}

func TestEnqueuePaymentVerifiedTx_SkipsUnsubscribedExtension(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	order := &models.Order{ID: "order-no-delivery"}
	extension, err := extensions.NewOrderExtension(order.ID.String(), "io.mobazha.metadata", "metadata", "v1", "resource", map[string]string{"value": "only"})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), extension); err != nil {
			return err
		}
		return EnqueuePaymentVerifiedTx(tx, order)
	}))
	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ExtensionDelivery{}).Count(&count).Error
	}))
	require.Zero(t, count)
}

func TestEnqueuePaymentVerifiedTx_RejectsRequiredReservationGap(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	orderOpen := managedCollectibleOrderOpen()
	serialized, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)
	order := &models.Order{ID: "order-required-reservation-gap", SerializedOrderOpen: serialized}
	order.SetRole(models.RoleVendor)
	order.MarkPaymentVerified()
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		ToAddress: "settlement-gap", Coin: "MATIC", Amount: "100",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{Method: pb.PaymentSent_CANCELABLE},
	}))
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen(order.ID.String(), orderOpen)
	require.NoError(t, err)
	require.True(t, ok)

	err = db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), extension); err != nil {
			return err
		}
		return EnqueuePaymentVerifiedTx(tx, order)
	})
	require.ErrorContains(t, err, "required reservation binding")

	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ExtensionDelivery{}).Count(&count).Error
	}))
	require.Zero(t, count)
}

func TestEventForOrder_DistinguishesBuyerSellerAndTenantCopies(t *testing.T) {
	orderOpen := managedCollectibleOrderOpen()
	serialized, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen("shared-order", orderOpen)
	require.NoError(t, err)
	require.True(t, ok)

	buyer := &models.Order{ID: "shared-order", SerializedOrderOpen: serialized, TenantMixin: models.TenantMixin{TenantID: "tenant-buyer"}}
	buyer.SetRole(models.RoleBuyer)
	seller := &models.Order{ID: "shared-order", SerializedOrderOpen: serialized, TenantMixin: models.TenantMixin{TenantID: "tenant-seller"}}
	seller.SetRole(models.RoleVendor)
	reservation := testReservationBinding(extension)
	buyerEvent, err := eventForOrder(buyer, extension, reservation, extensions.EventOrderReservationReleaseRequested, "cancelled")
	require.NoError(t, err)
	sellerEvent, err := eventForOrder(seller, extension, reservation, extensions.EventOrderReservationReleaseRequested, "cancelled")
	require.NoError(t, err)
	require.NotEqual(t, buyerEvent.EventID, sellerEvent.EventID)
	require.NotEqual(t, buyerEvent.SourceID, sellerEvent.SourceID)
	require.NotEqual(t, buyerEvent.OrderRole, sellerEvent.OrderRole)
}

func TestEnqueueTerminalEventTx_WritesGenericReleaseEvent(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	orderOpen := managedCollectibleOrderOpen()
	serialized, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)
	order := &models.Order{ID: "order-extension-release", SerializedOrderOpen: serialized}
	order.SetRole(models.RoleVendor)
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen(order.ID.String(), orderOpen)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), extension); err != nil {
			return err
		}
		if err := orderextensions.RecordReservationTx(tx, extensions.ReservationRequest{
			OrderID: order.ID.String(), Extension: extension, PaymentCoin: "MATIC",
			IdempotencyKey: "reserve-release", ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, extensions.Reservation{ID: "reservation-release", Version: 1, Status: "reserved"}); err != nil {
			return err
		}
		return EnqueueTerminalEventTx(tx, order, &events.OrderCancel{OrderID: order.ID.String()})
	}))
	var delivery models.ExtensionDelivery
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ?", order.ID.String()).First(&delivery).Error
	}))
	require.Equal(t, extensions.EventOrderReservationReleaseRequested, delivery.EventType)
}

func TestEnqueueTerminalEventTx_MissingReservationDoesNotBlockTerminalTransition(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	orderOpen := managedCollectibleOrderOpen()
	serialized, err := protojson.Marshal(orderOpen)
	require.NoError(t, err)
	order := &models.Order{ID: "order-extension-release-without-reservation", SerializedOrderOpen: serialized}
	order.SetRole(models.RoleVendor)
	extension, ok, err := models.CollectibleOrderExtensionFromOrderOpen(order.ID.String(), orderOpen)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := orderextensions.PersistTx(tx, order.ID.String(), extension); err != nil {
			return err
		}
		return EnqueueTerminalEventTx(tx, order, &events.OrderExpired{
			OrderID: order.ID.String(), Reason: "buyer abandoned checkout",
		})
	}))

	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ExtensionDelivery{}).Where("order_id = ?", order.ID.String()).Count(&count).Error
	}))
	require.Zero(t, count)
}

func testReservationBinding(extension extensions.OrderExtension) *extensions.ReservationBinding {
	now := time.Now().UTC()
	return &extensions.ReservationBinding{
		ReservationID: "reservation-role", ReservationVersion: 1, ExtensionRevision: extension.Revision,
		Status: "reserved", PaymentCoin: "MATIC", IdempotencyKey: "reserve-role",
		ExpiresAt: now.Add(time.Hour), RecordedAt: now,
	}
}

func managedCollectibleOrderOpen() *pb.OrderOpen {
	return &pb.OrderOpen{
		BuyerID: &pb.ID{PeerID: "buyer-peer"},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{
			VendorID: &pb.ID{PeerID: "seller-peer"},
			Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
			Item: &pb.Listing_Item{
				Blockchain:    "solana",
				TokenStandard: "metaplex_pnft",
			},
		}}},
		Items: []*pb.OrderOpen_Item{{OptionalFeatures: []string{
			models.CollectibleOptionalFeature(models.CollectibleFeatureFulfillment, models.CollectibleFulfillmentNFT),
			models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "slot-1"),
			models.CollectibleOptionalFeature(models.CollectibleFeatureCertNumber, "cert-1"),
			models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, "11111111111111111111111111111111"),
		}}},
	}
}
