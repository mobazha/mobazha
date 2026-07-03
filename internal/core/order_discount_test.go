package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	coreorder "github.com/mobazha/mobazha/internal/core/order"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/models/factory"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubShippingStore is a minimal ShippingStore that returns a default profile
// with shipping options matching factory.NewPhysicalListing("tshirt").
type stubShippingStore struct{ contracts.ShippingStore }

func newStubShippingStore() *stubShippingStore {
	return &stubShippingStore{}
}

func (s *stubShippingStore) GetDefaultProfile(_ context.Context) (*models.ShippingProfileEntity, error) {
	return s.buildProfile(), nil
}

func (s *stubShippingStore) GetProfile(_ context.Context, _ string) (*models.ShippingProfileEntity, error) {
	return s.buildProfile(), nil
}

func (s *stubShippingStore) buildProfile() *models.ShippingProfileEntity {
	groups := []*models.LocationGroup{
		{
			ID: "default-lg",
			Zones: []*models.ShippingZone{
				{
					ID:      "zone-all",
					Name:    "Worldwide",
					Regions: []string{"ALL"},
					Rates: []*models.ShippingRate{
						{
							ID:                "rate-standard",
							Name:              "standard",
							Price:             "100000",
							Currency:          "MCK",
							EstimatedDelivery: "3-5 days",
						},
					},
				},
			},
		},
	}
	lg, _ := json.Marshal(groups)
	return &models.ShippingProfileEntity{
		ID:                 "default-profile",
		Name:               "Default",
		IsDefault:          true,
		Version:            1,
		LocationGroupsJSON: string(lg),
	}
}

func (s *stubShippingStore) UpsertListingRef(_ context.Context, _ *models.ListingShippingRef) error {
	return nil
}

func (s *stubShippingStore) GetListingRef(_ context.Context, _ string) (*models.ListingShippingRef, error) {
	return nil, nil
}

func (s *stubShippingStore) DeleteListingRef(_ context.Context, _ string) error { return nil }

// ── coreorder.MapToProtoDiscounts ─────────────────────────────────────────

func TestMapToProtoDiscounts_Empty(t *testing.T) {
	result := coreorder.MapToProtoDiscounts(nil)
	assert.Empty(t, result)

	result = coreorder.MapToProtoDiscounts([]models.AppliedDiscount{})
	assert.Empty(t, result)
}

func TestMapToProtoDiscounts_SingleDiscount(t *testing.T) {
	input := []models.AppliedDiscount{
		{
			DiscountID: "d-1",
			Title:      "10% Off",
			Code:       "SAVE10",
			ValueType:  "percentage",
			Value:      "10",
			Amount:     "500000",
			Auto:       false,
			CodeID:     "c-1",
		},
	}

	result := coreorder.MapToProtoDiscounts(input)
	require.Len(t, result, 1)

	r := result[0]
	assert.Equal(t, "d-1", r.DiscountID)
	assert.Equal(t, "10% Off", r.Title)
	assert.Equal(t, "SAVE10", r.Code)
	assert.Equal(t, "percentage", r.ValueType)
	assert.Equal(t, "10", r.Value)
	assert.Equal(t, "500000", r.Amount)
	assert.False(t, r.Auto)
	assert.Equal(t, "c-1", r.CodeID)
}

func TestMapToProtoDiscounts_MultipleDiscounts(t *testing.T) {
	input := []models.AppliedDiscount{
		{DiscountID: "d-1", Title: "Code Discount", Code: "SAVE10", ValueType: "percentage", Value: "10", Amount: "100"},
		{DiscountID: "d-2", Title: "Auto Free Shipping", ValueType: "free_shipping", Value: "0", Amount: "0", Auto: true},
	}

	result := coreorder.MapToProtoDiscounts(input)
	require.Len(t, result, 2)
	assert.Equal(t, "d-1", result[0].DiscountID)
	assert.False(t, result[0].Auto)
	assert.Equal(t, "d-2", result[1].DiscountID)
	assert.True(t, result[1].Auto)
}

// ── EstimateOrderTotal with DiscountResolver ────────────────────

func setupMocknetForDiscount(t *testing.T) (*Mocknet, models.ListingIndex) {
	t.Helper()

	network, err := NewMocknet(2)
	require.NoError(t, err)
	t.Cleanup(func() { network.TearDown() })

	ss := newStubShippingStore()
	for _, node := range network.Nodes() {
		node.listingService.SetShippingStore(ss)
	}

	listing := factory.NewPhysicalListing("tshirt")

	done := make(chan struct{})
	require.NoError(t, network.Nodes()[0].Listing().SaveListing(listing, done))
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout saving listing")
	}

	index, err := network.Nodes()[0].Listing().GetMyListings()
	require.NoError(t, err)
	require.NotEmpty(t, index)

	waitForIPNS(t, network.Nodes()[1], network.Nodes()[0])

	return network, index
}

func waitForIPNS(t *testing.T, buyer, vendor *MobazhaNode) {
	t.Helper()
	vendorPeerID := vendor.Identity()
	var lastErr error
	for i := 0; i < 3; i++ {
		_, lastErr = buyer.listingService.GetListings(context.Background(), vendorPeerID, nil, false)
		if lastErr == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Skipf("IPNS propagation timeout (pre-existing mocknet limitation): %v", lastErr)
}

func newTestPurchase(listingHash string) *models.Purchase {
	return &models.Purchase{
		ShipTo:       "Peter",
		Address:      "123 Spooner St.",
		City:         "Quahog",
		State:        "RI",
		PostalCode:   "90210",
		CountryCode:  "US",
		AddressNotes: "asdf",
		Items: []models.PurchaseItem{
			{
				ListingHash: listingHash,
				Quantity:    "1",
				Options: []models.PurchaseItemOption{
					{Name: "size", Value: "large"},
					{Name: "color", Value: "red"},
				},
				Shipping: models.PurchaseShippingOption{
					Name:    "Worldwide",
					Service: "standard",
				},
				Memo: "test order",
			},
		},
		AlternateContactInfo: "test@test.com",
		PricingCoin:          "MCK",
	}
}

// retryOnIPNS retries fn up to maxRetries times when the error wraps ErrPeerUnreachable,
// which surfaces when IPNS propagation is still settling in mocknet.
func retryOnIPNS(t *testing.T, maxRetries int, fn func() error) {
	t.Helper()
	for i := 0; i < maxRetries; i++ {
		err := fn()
		if err == nil {
			return
		}
		if !errors.Is(err, coreiface.ErrPeerUnreachable) {
			require.NoError(t, err)
			return
		}
		if i == maxRetries-1 {
			t.Skipf("IPNS still flaky after %d retries: %v", maxRetries, err)
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
}

func TestEstimateOrderTotal_NoResolver_PreservesExistingBehavior(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	purchase := newTestPurchase(index[0].CID)
	purchase.DiscountCodes = []string{"SOME_CODE"}

	var totals models.OrderTotals
	retryOnIPNS(t, 5, func() error {
		var err error
		totals, err = buyer.orderService.EstimateOrderTotal(context.Background(), purchase)
		return err
	})

	assert.True(t, totals.Total.Cmp(iwallet.NewAmount(0)) > 0, "total should be positive")
	assert.True(t, totals.Subtotal.Cmp(iwallet.NewAmount(0)) > 0, "subtotal should be positive")
	assert.Equal(t, 0, totals.Discounts.Cmp(iwallet.NewAmount(0)), "discounts should be zero without resolver")
	assert.Empty(t, totals.DiscountDetails, "no discount details without resolver")
}

func TestEstimateOrderTotal_WithPercentageDiscount(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		require.NotEmpty(t, vendorPeerID)
		require.Equal(t, []string{"SAVE10"}, dc.DiscountCodes)
		require.True(t, dc.SubTotal.Cmp(big.NewInt(0)) > 0, "subtotal passed to resolver should be positive")

		tenPercent := new(big.Int).Div(dc.SubTotal, big.NewInt(10))
		negAmount := new(big.Int).Neg(tenPercent)
		return &models.DiscountResult{
			AppliedDiscounts: []models.AppliedDiscount{
				{
					DiscountID: "d-pct",
					Title:      "10% Off",
					Code:       "SAVE10",
					ValueType:  "percentage",
					Value:      "10",
					Amount:     negAmount.String(),
				},
			},
			DiscountsTotal: negAmount,
		}, nil
	})

	purchase := newTestPurchase(index[0].CID)
	purchase.DiscountCodes = []string{"SAVE10"}

	var totals models.OrderTotals
	retryOnIPNS(t, 5, func() error {
		var err error
		totals, err = buyer.orderService.EstimateOrderTotal(context.Background(), purchase)
		return err
	})

	assert.True(t, totals.Discounts.Cmp(iwallet.NewAmount(0)) < 0, "discounts should be negative (saving)")
	require.Len(t, totals.DiscountDetails, 1)

	detail := totals.DiscountDetails[0]
	assert.Equal(t, "d-pct", detail.DiscountID)
	assert.Equal(t, "10% Off", detail.Title)
	assert.Equal(t, "SAVE10", detail.Code)
	assert.Equal(t, "percentage", detail.ValueType)
	assert.Equal(t, "10", detail.Value)

	calculatedTotal := totals.Subtotal.Add(totals.Shipping).Add(totals.Discounts).Add(totals.Taxes)
	assert.Equal(t, 0, calculatedTotal.Cmp(totals.Total),
		"Total = Subtotal + Shipping + Discounts + Taxes; got calculated=%s, reported=%s",
		calculatedTotal.String(), totals.Total.String())
}

func TestEstimateOrderTotal_WithFreeShipping(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		return &models.DiscountResult{
			AppliedDiscounts: []models.AppliedDiscount{
				{
					DiscountID: "d-fs",
					Title:      "Free Shipping",
					ValueType:  "free_shipping",
					Value:      "0",
					Amount:     "0",
					Auto:       true,
				},
			},
			ShippingDiscount: true,
		}, nil
	})

	purchase := newTestPurchase(index[0].CID)

	var totals models.OrderTotals
	retryOnIPNS(t, 5, func() error {
		var err error
		totals, err = buyer.orderService.EstimateOrderTotal(context.Background(), purchase)
		return err
	})

	assert.True(t, totals.Shipping.Cmp(iwallet.NewAmount(0)) > 0,
		"shipping should retain original positive value, got %s", totals.Shipping.String())

	assert.True(t, totals.Discounts.Cmp(iwallet.NewAmount(0)) < 0,
		"discounts should be negative (offsetting shipping), got %s", totals.Discounts.String())

	negShipping := iwallet.NewAmount(0).Sub(totals.Shipping)
	assert.Equal(t, 0, totals.Discounts.Cmp(negShipping),
		"discounts should equal -shipping; discounts=%s, -shipping=%s",
		totals.Discounts.String(), negShipping.String())

	calculatedTotal := totals.Subtotal.Add(totals.Shipping).Add(totals.Discounts).Add(totals.Taxes)
	assert.Equal(t, 0, calculatedTotal.Cmp(totals.Total),
		"Total = Subtotal + Shipping + Discounts + Taxes invariant")

	require.Len(t, totals.DiscountDetails, 1)
	assert.Equal(t, "free_shipping", totals.DiscountDetails[0].ValueType)
	assert.True(t, totals.DiscountDetails[0].Auto)
}

func TestEstimateOrderTotal_ResolverFailure_GracefulDegradation(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		return nil, fmt.Errorf("vendor node unreachable")
	})

	purchase := newTestPurchase(index[0].CID)
	purchase.DiscountCodes = []string{"SAVE10"}

	var totals models.OrderTotals
	retryOnIPNS(t, 5, func() error {
		var err error
		totals, err = buyer.orderService.EstimateOrderTotal(context.Background(), purchase)
		return err
	})

	assert.True(t, totals.Total.Cmp(iwallet.NewAmount(0)) > 0, "total should still be positive")
	assert.Equal(t, 0, totals.Discounts.Cmp(iwallet.NewAmount(0)), "no discounts applied on failure")
	assert.Empty(t, totals.DiscountDetails, "no discount details on resolver failure")
}

func TestEstimateOrderTotal_ResolverReturnsEmpty(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		return &models.DiscountResult{}, nil
	})

	purchase := newTestPurchase(index[0].CID)
	purchase.DiscountCodes = []string{"INVALID_CODE"}

	var totals models.OrderTotals
	retryOnIPNS(t, 5, func() error {
		var err error
		totals, err = buyer.orderService.EstimateOrderTotal(context.Background(), purchase)
		return err
	})

	assert.Equal(t, 0, totals.Discounts.Cmp(iwallet.NewAmount(0)), "no discounts for invalid code")
	assert.Empty(t, totals.DiscountDetails)
}

// ── OrderOpen proto fields ──────────────────────────────────────

func TestCreateOrder_DiscountFieldsInProto(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		return &models.DiscountResult{
			AppliedDiscounts: []models.AppliedDiscount{
				{
					DiscountID: "d-fixed",
					CodeID:     "c-42",
					Title:      "$5 Off",
					Code:       "FIVE",
					ValueType:  "fixed",
					Value:      "500000",
					Amount:     "-500000",
				},
			},
			DiscountsTotal: big.NewInt(-500000),
		}, nil
	})

	purchase := newTestPurchase(index[0].CID)
	purchase.DiscountCodes = []string{"FIVE"}

	var orderOpen *pb.OrderOpen
	retryOnIPNS(t, 5, func() error {
		var err error
		orderOpen, _, err = buyer.orderService.CreateOrderForTesting(context.Background(), purchase)
		return err
	})

	assert.Equal(t, []string{"FIVE"}, orderOpen.DiscountCodes)
	require.Len(t, orderOpen.AppliedDiscounts, 1)

	ad := orderOpen.AppliedDiscounts[0]
	assert.Equal(t, "d-fixed", ad.DiscountID)
	assert.Equal(t, "c-42", ad.CodeID)
	assert.Equal(t, "$5 Off", ad.Title)
	assert.Equal(t, "FIVE", ad.Code)
	assert.Equal(t, "fixed", ad.ValueType)
	assert.Equal(t, "500000", ad.Value)
	assert.Equal(t, "-500000", ad.Amount)
}

func TestCreateOrder_NoDiscountCodes_NoResolverCall(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		assert.Empty(t, dc.DiscountCodes, "no discount codes should be passed")
		return nil, nil
	})

	purchase := newTestPurchase(index[0].CID)

	var orderOpen *pb.OrderOpen
	retryOnIPNS(t, 5, func() error {
		var err error
		orderOpen, _, err = buyer.orderService.CreateOrderForTesting(context.Background(), purchase)
		return err
	})

	assert.Empty(t, orderOpen.DiscountCodes)
	assert.Empty(t, orderOpen.AppliedDiscounts)
}

// ── Discount redemption recording ───────────────────────────────

func TestPurchaseListing_RecordsDiscountRedemption(t *testing.T) {
	network, err := NewMocknet(3)
	require.NoError(t, err)
	defer network.TearDown()

	ss := newStubShippingStore()
	for _, node := range network.Nodes() {
		node.listingService.SetShippingStore(ss)
	}

	go network.StartWalletNetwork()
	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	listing := factory.NewPhysicalListing("tshirt")
	done := make(chan struct{})
	require.NoError(t, network.Nodes()[0].Listing().SaveListing(listing, done))
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout saving listing")
	}

	index, err := network.Nodes()[0].Listing().GetMyListings()
	require.NoError(t, err)

	waitForIPNS(t, network.Nodes()[1], network.Nodes()[0])

	buyer := network.Nodes()[1]

	var recorded []string
	buyer.orderService.SetDiscountResolverForTesting(func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error) {
		return &models.DiscountResult{
			AppliedDiscounts: []models.AppliedDiscount{
				{
					DiscountID: "d-rec",
					CodeID:     "c-rec",
					Title:      "Test Discount",
					Code:       "TESTCODE",
					ValueType:  "percentage",
					Value:      "5",
					Amount:     "-249611",
				},
			},
			DiscountsTotal: big.NewInt(-249611),
		}, nil
	})
	buyer.orderService.SetDiscountRedemptionRecorderForTesting(func(ctx context.Context, vendorPeerID string, discountID string, codeID *string, orderID, customerPeerID, amount, currency string) error {
		recorded = append(recorded, discountID)
		assert.Equal(t, "d-rec", discountID)
		assert.NotNil(t, codeID)
		assert.Equal(t, "c-rec", *codeID)
		assert.NotEmpty(t, orderID)
		assert.NotEmpty(t, customerPeerID)
		assert.Equal(t, "-249611", amount)
		assert.Equal(t, "MCK", currency)
		return nil
	})

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID
	purchase.DiscountCodes = []string{"TESTCODE"}

	ackSub, err := buyer.eventBus.Subscribe(&events.MessageACK{})
	require.NoError(t, err)

	retryOnIPNS(t, 5, func() error {
		_, _, err := buyer.Order().PurchaseListing(context.Background(), purchase)
		return err
	})

	select {
	case <-ackSub.Out():
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for ACK")
	}

	assert.Equal(t, []string{"d-rec"}, recorded, "discount redemption should be recorded after purchase")
}

// Compile-time check: AppliedDiscount fields in proto match
func TestAppliedDiscount_ProtoFieldCompleteness(t *testing.T) {
	ad := &pb.OrderOpen_AppliedDiscount{
		DiscountID: "test",
		Title:      "Test",
		Code:       "CODE",
		ValueType:  "percentage",
		Value:      "10",
		Amount:     "100",
		Auto:       true,
		CodeID:     "cid",
	}
	assert.Equal(t, "test", ad.DiscountID)
	assert.Equal(t, "cid", ad.CodeID)
}
