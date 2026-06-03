//go:build !private_distribution

package order

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPostProcessOrderOpen_ReservesStandardOrderSupplyForVendor(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "camera", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-standard-1"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-1", listing, listingHash))
	}))

	require.Len(t, recorder.reserveTxRequests, 1)
	req := recorder.reserveTxRequests[0]
	require.Equal(t, "order-standard-1", req.OrderRef)
	require.Equal(t, models.OrderTypeStandard, req.OrderType)
	require.Len(t, req.Lines, 1)
	require.Equal(t, contracts.SupplyKindSkuQuantity, req.Lines[0].SupplyKind)
	require.Equal(t, "camera", req.Lines[0].ListingSlug)
	require.Equal(t, 1, req.Lines[0].Quantity)
	require.True(t, req.Lines[0].StockTracked)
	require.Equal(t, int64(3), req.Lines[0].StockLimit)
	require.NotEmpty(t, req.Lines[0].VariantHash)
}

func TestPostProcessOrderOpen_LeavesExternalSupplyLineForManualAction(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "supplier-shirt", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.SyncedProductMapping{}); err != nil {
			return err
		}
		if err := tx.Save(&models.SyncedProductMapping{
			ID:            "spm-1",
			ProviderID:    "printful",
			ListingSlug:   "supplier-shirt",
			SyncProductID: "sync-123",
			Status:        "synced",
			LastSyncAt:    time.Now(),
		}); err != nil {
			return err
		}
		order := &models.Order{
			ID:     models.OrderID("order-standard-external"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		exp := time.Now().Add(time.Hour)
		order.ExpiresAt = &exp
		order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
		if err := tx.Save(order); err != nil {
			return err
		}
		lines, err := standardOrderSupplyLinesFromOrderOpen(tx, "order-standard-external", orderSupplyOrderOpen(listing, listingHash))
		require.NoError(t, err)
		require.Len(t, lines, 1)
		require.Equal(t, contracts.SupplyKindExternalSupply, lines[0].SupplyKind)
		require.Equal(t, "supplier-shirt", lines[0].ListingSlug)
		require.Equal(t, 1, lines[0].Quantity)
		require.Equal(t, "printful", lines[0].ProviderID)
		require.Equal(t, "sync-123", lines[0].ProviderRef)
		require.False(t, lines[0].StockTracked)
		require.Empty(t, lines[0].VariantHash)
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-external", listing, listingHash))
	}))

	require.Empty(t, recorder.reserveTxRequests)
}

func TestPostProcessOrderOpen_SkipsClosedOrDeclinedStandardOrder(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})
	listing := orderSupplyListing(t, "camera", "Color", "Red", "3")
	listingHash := orderSupplyListingHash(t, listing)

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing); err != nil {
			return err
		}
		order := &models.Order{
			ID:                     models.OrderID("order-standard-declined"),
			MyRole:                 string(models.RoleVendor),
			Open:                   false,
			SerializedOrderDecline: []byte{1},
			SerializedOrderOpen:    []byte{1},
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		return svc.postProcessOrderOpenInTx(tx, orderSupplyOrderOpenMessage(t, "order-standard-declined", listing, listingHash))
	}))

	require.Empty(t, recorder.reserveTxRequests)
}

func TestStandardOrderSupplyLifecycle_CommitAndReleaseUseTransactionalService(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-lifecycle"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := svc.commitStandardOrderSupplyInTx(tx, "order-standard-lifecycle"); err != nil {
			return err
		}
		return svc.releaseStandardOrderSupplyInTx(tx, "order-standard-lifecycle", "cancelled")
	}))

	require.Len(t, recorder.commitTxRequests, 1)
	require.Equal(t, standardOrderSupplyCommitRequest{
		orderRef:  "order-standard-lifecycle",
		orderType: models.OrderTypeStandard,
		txSeen:    true,
	}, recorder.commitTxRequests[0])
	require.Len(t, recorder.releaseTxRequests, 1)
	require.Equal(t, standardOrderSupplyReleaseRequest{
		orderRef:  "order-standard-lifecycle",
		orderType: models.OrderTypeStandard,
		reason:    "cancelled",
		txSeen:    true,
	}, recorder.releaseTxRequests[0])
}

func TestPostProcessInTx_DispatchesStandardOrderSupplyLifecycle(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-dispatch"),
			MyRole: string(models.RoleVendor),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		for _, msgType := range []npb.OrderMessage_MessageType{
			npb.OrderMessage_ORDER_CONFIRMATION,
			npb.OrderMessage_ORDER_CANCEL,
			npb.OrderMessage_ORDER_DECLINE,
		} {
			msg := &npb.OrderMessage{
				OrderID:     "order-standard-dispatch",
				MessageType: msgType,
			}
			if err := svc.postProcessInTx(tx, msg, nil, &models.Order{}); err != nil {
				return err
			}
		}
		return nil
	}))

	require.Len(t, recorder.commitTxRequests, 1)
	require.Equal(t, "order-standard-dispatch", recorder.commitTxRequests[0].orderRef)
	require.Equal(t, models.OrderTypeStandard, recorder.commitTxRequests[0].orderType)
	require.Len(t, recorder.releaseTxRequests, 2)
	require.Equal(t, "cancelled", recorder.releaseTxRequests[0].reason)
	require.Equal(t, "declined", recorder.releaseTxRequests[1].reason)
}

func TestStandardOrderSupplyLifecycle_SkipsBuyerRole(t *testing.T) {
	recorder := &recordingOrderSupplyAvailability{}
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Resolver:           orderSupplyAlwaysEnabledResolver{},
		SupplyAvailability: recorder,
	})

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		order := &models.Order{
			ID:     models.OrderID("order-standard-buyer"),
			MyRole: string(models.RoleBuyer),
			Open:   true,
		}
		if err := tx.Save(order); err != nil {
			return err
		}
		if err := svc.commitStandardOrderSupplyInTx(tx, "order-standard-buyer"); err != nil {
			return err
		}
		return svc.releaseStandardOrderSupplyInTx(tx, "order-standard-buyer", "cancelled")
	}))

	require.Empty(t, recorder.commitTxRequests)
	require.Empty(t, recorder.releaseTxRequests)
}

type standardOrderSupplyCommitRequest struct {
	orderRef  string
	orderType string
	txSeen    bool
}

type standardOrderSupplyReleaseRequest struct {
	orderRef  string
	orderType string
	reason    string
	txSeen    bool
}

type recordingOrderSupplyAvailability struct {
	reserveTxRequests []contracts.ReserveOrderSupplyRequest
	commitTxRequests  []standardOrderSupplyCommitRequest
	releaseTxRequests []standardOrderSupplyReleaseRequest
}

func (r *recordingOrderSupplyAvailability) Quote(context.Context, contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	return &contracts.SupplyQuoteResult{CanSell: true}, nil
}

func (r *recordingOrderSupplyAvailability) ReserveOrder(context.Context, contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (r *recordingOrderSupplyAvailability) ReserveOrderTx(_ context.Context, _ database.Tx, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	r.reserveTxRequests = append(r.reserveTxRequests, req)
	return &contracts.ReserveOrderSupplyResult{}, nil
}

func (r *recordingOrderSupplyAvailability) CommitOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string) error {
	r.commitTxRequests = append(r.commitTxRequests, standardOrderSupplyCommitRequest{
		orderRef:  orderRef,
		orderType: orderType,
		txSeen:    tx != nil,
	})
	return nil
}

func (r *recordingOrderSupplyAvailability) ReleaseOrderTx(_ context.Context, tx database.Tx, orderRef string, orderType string, reason string) error {
	r.releaseTxRequests = append(r.releaseTxRequests, standardOrderSupplyReleaseRequest{
		orderRef:  orderRef,
		orderType: orderType,
		reason:    reason,
		txSeen:    tx != nil,
	})
	return nil
}

func (r *recordingOrderSupplyAvailability) CommitOrder(context.Context, string, string) error {
	return nil
}

func (r *recordingOrderSupplyAvailability) ReleaseOrder(context.Context, string, string, string) error {
	return nil
}

var _ contracts.SupplyAvailabilityService = (*recordingOrderSupplyAvailability)(nil)

type orderSupplyAlwaysEnabledResolver struct{}

func (orderSupplyAlwaysEnabledResolver) IsEnabled(context.Context, string) bool { return true }

func (orderSupplyAlwaysEnabledResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{Enabled: true}
}

func (orderSupplyAlwaysEnabledResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return nil
}

func orderSupplyListing(t *testing.T, slug, option, variant, quantity string) *pb.SignedListing {
	t.Helper()
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				Version:      ListingVersion,
				ContractType: pb.Listing_Metadata_PHYSICAL_GOOD,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
				Options: []*pb.Listing_Item_Option{{
					Name: option,
					Variants: []*pb.Listing_Item_Option_Variant{{
						Name: variant,
					}},
				}},
				Skus: []*pb.Listing_Item_Sku{{
					Selections: []*pb.Listing_Item_Sku_Selection{{
						Option:  option,
						Variant: variant,
					}},
					Quantity: quantity,
					Price:    "1000",
				}},
			},
		},
	}
}

func orderSupplyListingHash(t *testing.T, sl *pb.SignedListing) string {
	t.Helper()
	ser, err := proto.Marshal(sl)
	require.NoError(t, err)
	hash, err := utils.MultihashSha256(ser)
	require.NoError(t, err)
	return hash.B58String()
}

func orderSupplyOrderOpen(listing *pb.SignedListing, listingHash string) *pb.OrderOpen {
	return &pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		BuyerID: &pb.ID{
			PeerID: "buyer",
		},
		Listings: []*pb.SignedListing{listing},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: listingHash,
			Quantity:    "1",
			Options: []*pb.OrderOpen_Item_Option{{
				Name:  "Color",
				Value: "Red",
			}},
		}},
		RatingKeys: [][]byte{{1}},
	}
}

func orderSupplyOrderOpenMessage(t *testing.T, orderID string, listing *pb.SignedListing, listingHash string) *npb.OrderMessage {
	t.Helper()
	orderOpen := orderSupplyOrderOpen(listing, listingHash)
	anyMsg, err := anypb.New(orderOpen)
	require.NoError(t, err)
	return &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     anyMsg,
	}
}
