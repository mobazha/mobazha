package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/fulfillment"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ---------------------------------------------------------------------------
// P2-1: parseUSDDollarsToCents — robust decimal parsing
// ---------------------------------------------------------------------------

func TestParseUSDDollarsToCents(t *testing.T) {
	cases := []struct {
		in     string
		want   uint64
		wantOk bool
	}{
		// Whole dollars.
		{"4", 400, true},
		{"100", 10000, true},
		{"0", 0, true},
		// Fractional.
		{"4.5", 450, true},  // single fractional digit padded → 50 cents
		{"4.50", 450, true}, // two fractional digits
		{"0.99", 99, true},  // sub-dollar
		{"0.05", 5, true},   // small change
		{".5", 50, true},    // bare fractional accepted as 50 cents
		{"123.45", 12345, true},
		// Whitespace tolerated.
		{"  4.50  ", 450, true},
		// Bad input — must reject.
		{"", 0, false},
		{"abc", 0, false},
		{"4.5.0", 0, false}, // multiple decimals
		{"4.567", 0, false}, // 3 fractional digits
		{"-4.50", 0, false}, // negative
		{"+4.50", 0, false}, // sign prefix
		{"$4.50", 0, false}, // currency symbol
		{"4,50", 0, false},  // comma separator
		{"4.", 0, false},    // dangling dot
	}
	for _, tc := range cases {
		got, ok := parseUSDDollarsToCents(tc.in)
		if ok != tc.wantOk {
			t.Errorf("parseUSDDollarsToCents(%q) ok=%v want=%v (got=%d)", tc.in, ok, tc.wantOk, got)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("parseUSDDollarsToCents(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// P1-1 / P1-2: evaluateMarginGate — shared margin protection
// ---------------------------------------------------------------------------

// seedSyncedProduct seeds a SyncedProductMapping into the in-memory db. When
// variantCosts is non-nil, builds the private metadata blob mapping each
// catalog variantID → supplierCostCents (mirrors buildVariantMetadata).
func seedSyncedProduct(
	t *testing.T,
	tdb *scTestDatabase,
	slug, providerID, supplierCost string,
	variantCosts map[string]uint64,
) {
	t.Helper()
	var metaJSON []byte
	if len(variantCosts) > 0 {
		entries := make([]variantMetadataEntry, 0, len(variantCosts))
		for vid, cost := range variantCosts {
			entries = append(entries, variantMetadataEntry{
				CatalogVariantID:  vid,
				SupplierCostCents: cost,
			})
		}
		metaJSON, _ = json.Marshal(entries)
	}
	if err := tdb.gormDB.Create(&models.SyncedProductMapping{
		ID:           "spm-" + slug,
		ListingSlug:  slug,
		ProviderID:   providerID,
		SupplierCost: supplierCost,
		Status:       "synced",
		Metadata:     metaJSON,
	}).Error; err != nil {
		t.Fatalf("seed synced product %s: %v", slug, err)
	}
}

// makeOrderOpenSingleSKU builds an OrderOpen whose listing has exactly one SKU.
// SKU carries the public retail price; supplier cost lives in private mapping
// metadata (seeded separately).
func makeOrderOpenSingleSKU(slug, retailCents string, qty int) *pb.OrderOpen {
	return &pb.OrderOpen{
		PricingCoin: "USD",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug: slug,
				Item: &pb.Listing_Item{
					Price: retailCents,
					Skus: []*pb.Listing_Item_Sku{{
						ProductID: slug + "-sku-1",
						Price:     retailCents,
					}},
				},
				Metadata: &pb.Listing_Metadata{
					PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{Quantity: itoa(qty)}},
	}
}

// skuSpec describes one SKU's option selection + per-variant economics.
// costCents lives only in test plumbing — it gets seeded into the private
// SyncedProductMapping.Metadata, never into the public Listing fields.
type skuSpec struct {
	productID   string
	option      string // e.g. "size"
	variant     string // e.g. "L"
	retailCents string
	costCents   uint64
}

// makeOrderOpenMultiSKU builds an OrderOpen whose listing has multiple SKUs
// (different size/color). Public listing carries Selections + Price only; no
// supplier cost is leaked. The buyer chooses the SKU at `pickIdx` via
// OrderOpen.Items[0].Options. Listing.Item.Price = cheapest variant's retail
// (mirrors buildListingFromCatalog).
func makeOrderOpenMultiSKU(slug string, specs []skuSpec, pickIdx, qty int) *pb.OrderOpen {
	skus := make([]*pb.Listing_Item_Sku, len(specs))
	cheapestRetail := ""
	for i, sp := range specs {
		skus[i] = &pb.Listing_Item_Sku{
			ProductID: sp.productID,
			Price:     sp.retailCents,
			Selections: []*pb.Listing_Item_Sku_Selection{
				{Option: sp.option, Variant: sp.variant},
			},
		}
		if cheapestRetail == "" || sp.retailCents < cheapestRetail {
			cheapestRetail = sp.retailCents
		}
	}
	pick := specs[pickIdx]
	return &pb.OrderOpen{
		PricingCoin: "USD",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug: slug,
				Item: &pb.Listing_Item{
					Price: cheapestRetail,
					Skus:  skus,
				},
				Metadata: &pb.Listing_Metadata{
					PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			Quantity: itoa(qty),
			Options: []*pb.OrderOpen_Item_Option{{
				Name:  pick.option,
				Value: pick.variant,
			}},
		}},
	}
}

// variantCostsFromSpecs extracts the private cost map for seeding into
// SyncedProductMapping.Metadata.
func variantCostsFromSpecs(specs []skuSpec) map[string]uint64 {
	out := make(map[string]uint64, len(specs))
	for _, sp := range specs {
		out[sp.productID] = sp.costCents
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func newSCSvcWithProvider(t *testing.T, tdb *scTestDatabase, p *stubFulfillmentProvider) *SupplyChainAppService {
	t.Helper()
	reg := fulfillment.NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatalf("register stub: %v", err)
	}
	return NewSupplyChainAppService(reg, tdb, "n", testPrivKeyBytes)
}

func TestEvaluateMarginGate_Pass_SingleSKU(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Single-SKU listing path uses the import-time snapshot (SupplierCost +
	// Item.Price). Cost = $5.00, retail = $20.00, shipping = $4.50.
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo:         oo,
		providerID: "p1",
		recipient:  contracts.FulfillmentRecipient{CountryCode: "US"},
		items:      []contracts.FulfillmentItem{{Quantity: 1, CatalogVariantID: "v1"}},
	})
	if !ok {
		t.Fatalf("expected margin gate to pass; reason=%s msg=%s", reason, msg)
	}
}

func TestEvaluateMarginGate_MultiSKU_UsesResolvedSKUCost(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Listing has 3 size variants. Buyer chooses XL (highest cost). Margin
	// gate must use XL's PRIVATE cost from SyncedProductMapping.Metadata, not
	// the cheapest-variant snapshot.
	//
	//   S  : retail $25.00, cost $4.00
	//   M  : retail $25.00, cost $5.00
	//   XL : retail $30.00, cost $14.00   ← buyer picks
	//
	// XL cost+shipping = 1400+450 = 1850 cents, retail = 3000 cents → 61.6% ✓
	specs := []skuSpec{
		{productID: "tee-S", option: "size", variant: "S", retailCents: "2500", costCents: 400},
		{productID: "tee-M", option: "size", variant: "M", retailCents: "2500", costCents: 500},
		{productID: "tee-XL", option: "size", variant: "XL", retailCents: "3000", costCents: 1400},
	}
	seedSyncedProduct(t, tdb, "tee", "p1", "400", variantCostsFromSpecs(specs))
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenMultiSKU("tee", specs, 2 /* XL */, 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("expected pass on XL pick: reason=%s msg=%s", reason, msg)
	}
}

func TestEvaluateMarginGate_MultiSKU_RejectsHighCostVariant(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// XL is a loss-leader misconfiguration: cost $24, retail $25. The OLD
	// snapshot-based logic would have used the cheapest variant cost ($4) and
	// let this through. With per-variant private cost map → must reject.
	specs := []skuSpec{
		{productID: "tee-S", option: "size", variant: "S", retailCents: "2500", costCents: 400},
		{productID: "tee-XL", option: "size", variant: "XL", retailCents: "2500", costCents: 2400},
	}
	seedSyncedProduct(t, tdb, "tee", "p1", "400", variantCostsFromSpecs(specs))
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenMultiSKU("tee", specs, 1 /* XL */, 1)
	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("XL with cost ≥ retail must be rejected — snapshot fallback would have hidden this")
	}
	if reason != contracts.FailureReasonMarginProtectionFailed {
		t.Errorf("reason: want margin_protection_failed, got %s", reason)
	}
}

func TestEvaluateMarginGate_MultiSKU_UnresolvedSelectionFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	specs := []skuSpec{
		{productID: "tee-S", option: "size", variant: "S", retailCents: "2500", costCents: 400},
		{productID: "tee-M", option: "size", variant: "M", retailCents: "2500", costCents: 500},
	}
	seedSyncedProduct(t, tdb, "tee", "p1", "400", variantCostsFromSpecs(specs))
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenMultiSKU("tee", specs, 0, 1)
	// Override the selection with one that doesn't match any SKU.
	oo.Items[0].Options[0].Value = "XXL"

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("unresolvable SKU on multi-SKU listing must fail closed (not silently fall back to snapshot)")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("reason: want manual_action_required, got %s", reason)
	}
	if !contains(msg, "buyer-selected SKU") {
		t.Errorf("msg should mention SKU resolution failure, got %q", msg)
	}
}

// TestEvaluateMarginGate_DoesNotLeakCostViaCompareAtPrice asserts the headline
// privacy invariant: even if a malicious or buggy listing carries supplier
// cost in the public `CompareAtPrice` field, the margin gate IGNORES it. The
// only authoritative supplier cost source is the private metadata blob.
func TestEvaluateMarginGate_DoesNotLeakCostViaCompareAtPrice(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Note: NO variant cost map seeded → private metadata is empty.
	seedSyncedProduct(t, tdb, "tee", "p1", "" /* no snapshot */, nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	// Build an OrderOpen where the SKU has a (now-illegal) CompareAtPrice
	// claiming $1.00 cost — this would let a thin-margin order pass if the
	// gate were still trusting CompareAtPrice.
	oo := &pb.OrderOpen{
		PricingCoin: "USD",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				Slug: "tee",
				Item: &pb.Listing_Item{
					Price: "2000",
					Skus: []*pb.Listing_Item_Sku{{
						ProductID:      "tee-A",
						Price:          "2000",
						CompareAtPrice: "100", // public field; must not be read as cost
						Selections: []*pb.Listing_Item_Sku_Selection{
							{Option: "size", Variant: "S"},
						},
					}, {
						ProductID:      "tee-B",
						Price:          "2000",
						CompareAtPrice: "100",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{Option: "size", Variant: "M"},
						},
					}},
				},
				Metadata: &pb.Listing_Metadata{
					PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			Quantity: "1",
			Options: []*pb.OrderOpen_Item_Option{
				{Name: "size", Value: "S"},
			},
		}},
	}

	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("margin gate must NOT trust public CompareAtPrice as supplier cost")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("expected manual_action_required (no private cost map), got %s", reason)
	}
}

func TestEvaluateMarginGate_RejectsThinMargin(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Snapshot path: cost = $15.00, retail = $1.99, shipping = $4.50.
	seedSyncedProduct(t, tdb, "tee", "p1", "1500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "199", 1)
	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("expected margin gate to reject thin margin")
	}
	if reason != contracts.FailureReasonMarginProtectionFailed {
		t.Errorf("reason: want margin_protection_failed, got %s", reason)
	}
}

func TestEvaluateMarginGate_SingleSKU_FallsBackToSnapshot(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Single SKU + no per-variant metadata → must use SupplierCost snapshot.
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("single-SKU listing should fall back to snapshot; reason=%s msg=%s", reason, msg)
	}
}

func TestEvaluateMarginGate_NoSKUEconomicsAndNoSnapshot(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "" /* no snapshot */, nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("no per-variant cost AND no snapshot should reject")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s", reason)
	}
}

func TestEvaluateMarginGate_UnparseableShippingFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	// P2-1: unparseable shipping rate must NOT be silently treated as 0.
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{{ID: "weird", Rate: "not-a-number", Currency: "USD"}}, nil
		},
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("unparseable shipping rate must fail closed (not pass with 0 cost)")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s (msg=%s)", reason, msg)
	}
}

func TestEvaluateMarginGate_PicksCheapestRate(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{
				{ID: "express", Rate: "20.00", Currency: "USD"},
				{ID: "cheap", Rate: "1.00", Currency: "USD"},
				{ID: "standard", Rate: "4.50", Currency: "USD"},
			}, nil
		},
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, _, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatal("should pass when cheapest rate keeps cost+ship under threshold")
	}
}

// ---------------------------------------------------------------------------
// P1-3: Reconcile worker recovers failed AutoConfirmAndShip via
// OrderAdvancementStatus
// ---------------------------------------------------------------------------

func TestApplyFulfillmentStatus_ShippedSetsAdvancementPending(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedFOMapping(t, tdb, models.FulfillmentOrderMapping{
		ID: "m1", MobazhaOrderID: "o1", ProviderID: "p1",
		Status:           string(contracts.FulfillmentStatusInProcess),
		RetryLockedUntil: time.Now().Add(retryLeaseDuration),
	})
	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "n", testPrivKeyBytes)
	svc.SetOrderOps(&noOpOrderOps{}) // ConfirmOrder/ShipOrder return nil

	mapping := loadFOMapping(t, tdb, "m1")
	fo := &contracts.FulfillmentOrder{
		Status: contracts.FulfillmentStatusShipped,
		Shipments: []contracts.FulfillmentShipment{{
			TrackingNumber: "TRACK-1",
			Carrier:        "USPS",
			TrackingURL:    "https://example/track",
		}},
	}
	svc.applyFulfillmentStatus(context.Background(), mapping, fo)

	got := loadFOMapping(t, tdb, "m1")
	if got.Status != string(contracts.FulfillmentStatusShipped) {
		t.Fatalf("status: want shipped, got %s", got.Status)
	}
	// noOpOrderOps reports IsOrderConfirmed=true, IsOrderShipped=false. So
	// autoConfirmAndShip should call ShipOrder successfully and we mark done.
	// Either way, OrderAdvancementStatus must NOT be empty (it must reflect
	// that we tried).
	if got.OrderAdvancementStatus == "" {
		t.Errorf("expected OrderAdvancementStatus to be set, got empty")
	}
}

func TestReconcileStaleOrders_RecoversPendingAdvancement(t *testing.T) {
	tdb := newSCTestDatabase(t)
	stale := time.Now().Add(-2 * reconcileStaleThreshold)
	// Mapping is `shipped` with advancement still pending and stale → reconcile
	// must pick it up and re-attempt AutoConfirmAndShip.
	seedFOMapping(t, tdb, models.FulfillmentOrderMapping{
		ID: "m1", MobazhaOrderID: "o1", ProviderID: "p1",
		FulfillmentOrderID:     "ext-1",
		Status:                 string(contracts.FulfillmentStatusShipped),
		OrderAdvancementStatus: advancementStatusPending,
		TrackingNumber:         "TRACK-1",
		Carrier:                "USPS",
		TrackingURL:            "https://example/track",
		UpdatedAt:              stale,
	})
	// Force updated_at to old value (autoUpdate may overwrite).
	if err := tdb.gormDB.Model(&models.FulfillmentOrderMapping{}).
		Where("id = ?", "m1").
		UpdateColumn("updated_at", stale).Error; err != nil {
		t.Fatalf("force updated_at: %v", err)
	}

	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "n", testPrivKeyBytes)
	svc.SetOrderOps(&noOpOrderOps{})

	svc.reconcileStaleOrders(context.Background())

	got := loadFOMapping(t, tdb, "m1")
	if got.OrderAdvancementStatus != advancementStatusDone {
		t.Errorf("OrderAdvancementStatus: want %s, got %s",
			advancementStatusDone, got.OrderAdvancementStatus)
	}
}

func TestReconcileStaleOrders_PendingAdvancementWithoutTrackingMarksPermanent(t *testing.T) {
	tdb := newSCTestDatabase(t)
	stale := time.Now().Add(-2 * reconcileStaleThreshold)
	seedFOMapping(t, tdb, models.FulfillmentOrderMapping{
		ID: "m1", MobazhaOrderID: "o1", ProviderID: "p1",
		FulfillmentOrderID:     "ext-1",
		Status:                 string(contracts.FulfillmentStatusShipped),
		OrderAdvancementStatus: advancementStatusPending,
		// No tracking info — recovery cannot proceed.
	})
	if err := tdb.gormDB.Model(&models.FulfillmentOrderMapping{}).
		Where("id = ?", "m1").
		UpdateColumn("updated_at", stale).Error; err != nil {
		t.Fatalf("force updated_at: %v", err)
	}

	svc := NewSupplyChainAppService(fulfillment.NewRegistry(), tdb, "n", testPrivKeyBytes)
	svc.SetOrderOps(&noOpOrderOps{})

	svc.reconcileStaleOrders(context.Background())

	got := loadFOMapping(t, tdb, "m1")
	if got.OrderAdvancementStatus != advancementStatusPermanentFail {
		t.Errorf("OrderAdvancementStatus: want %s, got %s",
			advancementStatusPermanentFail, got.OrderAdvancementStatus)
	}
}

// ---------------------------------------------------------------------------
// P1 (4th pass): order-level discount must lower effective revenue
// ---------------------------------------------------------------------------

// TestEvaluateMarginGate_DiscountReducesEffectiveRevenue exercises the case
// from reviewer feedback: SKU price $20, supplier cost $14, shipping $1, and
// a 30% percentage discount applied. Without prorate, ($14+$1)/$20 = 75% PASS.
// With prorate, effective revenue = $14 → ($14+$1)/$14 = 107% REJECT (loss).
func TestEvaluateMarginGate_DiscountReducesEffectiveRevenue(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// Single-SKU listing, snapshot path. Cost = $14.00, retail = $20.00.
	seedSyncedProduct(t, tdb, "tee", "p1", "1400", nil)
	// Tight shipping ($1.00) so the only thing pushing past the gate is the
	// discount.
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{{ID: "std", Rate: "1.00", Currency: "USD"}}, nil
		},
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	// 30% off → -600 cents in payment currency (USD smallest unit).
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d1",
		ValueType:  "percentage",
		Value:      "30",
		Amount:     "-600",
	}}

	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("post-discount revenue ($14) is below cost+ship ($15) — must reject")
	}
	if reason != contracts.FailureReasonMarginProtectionFailed {
		t.Errorf("want margin_protection_failed, got %s", reason)
	}
}

// TestEvaluateMarginGate_FreeShippingDiscountIgnored asserts free_shipping
// discounts do NOT eat into item revenue (they reduce shipping fees, which
// are added on the cost side, not subtracted from retail).
func TestEvaluateMarginGate_FreeShippingDiscountIgnored(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	// Free shipping: amount = -450 (matches the cheapest rate). Must NOT be
	// counted as item revenue lost.
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d-ship",
		ValueType:  "free_shipping",
		Amount:     "-450",
	}}

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("free_shipping discount must not lower item revenue; reason=%s msg=%s", reason, msg)
	}
}

// TestEvaluateMarginGate_DiscountProratedAcrossMixedOrder verifies that when
// a non-supply-chain listing is also present, the order-level discount is
// prorated by retail share so the supply-chain side only absorbs its
// proportional cut.
func TestEvaluateMarginGate_DiscountProratedAcrossMixedOrder(t *testing.T) {
	tdb := newSCTestDatabase(t)
	// SC listing: retail $20 (scRetail = 2000).
	seedSyncedProduct(t, tdb, "tee", "p1", "1000", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{{ID: "std", Rate: "2.00", Currency: "USD"}}, nil
		},
	})

	// Add a second listing that is NOT supply-chain-managed (no SyncedProductMapping).
	// Its retail = $80, so allRetail = 100, scRetail = 20.
	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	oo.Listings = append(oo.Listings, &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: "tshirt-self",
			Item: &pb.Listing_Item{
				Price: "8000",
				Skus:  []*pb.Listing_Item_Sku{{ProductID: "self-1", Price: "8000"}},
			},
		},
	})
	oo.Items = append(oo.Items, &pb.OrderOpen_Item{Quantity: "1"})

	// $10 off the whole order. Prorate to SC: ceil(1000 * 2000 / 10000) = 200.
	// Effective SC revenue = 2000 - 200 = 1800 cents.
	// Cost+ship = 1000 + 200 = 1200. Ratio = 1200/1800 = 66.7% < 80% → pass.
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d-ten",
		ValueType:  "fixed",
		Amount:     "-1000",
	}}

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("mixed order: SC share of discount should keep margin OK; reason=%s msg=%s", reason, msg)
	}
}

// TestEvaluateMarginGate_DiscountUnparseableFailsClosed asserts the gate
// rejects the order rather than silently treating an unparseable applied
// discount as zero.
func TestEvaluateMarginGate_DiscountUnparseableFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d-bad",
		ValueType:  "percentage",
		Amount:     "not-a-number",
	}}

	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("unparseable discount amount must fail closed (not silently ignored)")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s", reason)
	}
}

// ---------------------------------------------------------------------------
// P2 (2nd pass): exact-decimal cents conversion + ceil markup
// ---------------------------------------------------------------------------

func TestComputeRetailCents_CeilsAwayFromFloatPrecisionLoss(t *testing.T) {
	cases := []struct {
		name       string
		costCents  uint64
		markup     float64
		wantRetail uint64
	}{
		{"zero cost short circuits", 0, 2.0, 0},
		{"non-positive markup short circuits", 100, 0, 0},
		{"clean integer math", 500, 2.0, 1000},
		// 199 cents * 1.5 = 298.5 → must round UP to 299, not truncate to 298.
		{"half-cent ceil", 199, 1.5, 299},
		// 829 cents * 1.42 → mathematically 1177.18, must ceil to 1178.
		// (Old truncate path would have given 1177.)
		{"non-binary markup ceil", 829, 1.42, 1178},
		// Exact integer result: must NOT bump by 1 cent.
		{"exact integer leaves alone", 200, 1.5, 300},
	}
	for _, tc := range cases {
		got := computeRetailCents(tc.costCents, tc.markup)
		if got != tc.wantRetail {
			t.Errorf("%s: computeRetailCents(%d, %g) = %d, want %d",
				tc.name, tc.costCents, tc.markup, got, tc.wantRetail)
		}
	}
}

func TestBuildVariantMetadata_UsesExactCentsAndCeilsRetail(t *testing.T) {
	// $8.29 is the canonical binary-float-imprecise value: float64(8.29)*100
	// = 828.99999..., which `uint64(...)` truncates to 828 — silently shaving
	// 1 cent off every imported variant.
	variants := []contracts.CatalogVariant{
		{ID: "v1", Price: "8.29"},
		{ID: "v2", Price: "2.55"},
		{ID: "v3", Price: "10.00"},
	}
	got, err := buildVariantMetadata(variants, 1.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len: got %d", len(got))
	}

	wantCost := map[string]uint64{"v1": 829, "v2": 255, "v3": 1000}
	// retail = ceil(cost * 1.5). v1: ceil(1243.5)=1244, v2: ceil(382.5)=383,
	// v3: 1500.
	wantRetail := map[string]uint64{"v1": 1244, "v2": 383, "v3": 1500}

	for _, e := range got {
		if e.SupplierCostCents != wantCost[e.CatalogVariantID] {
			t.Errorf("%s cost: got %d want %d", e.CatalogVariantID,
				e.SupplierCostCents, wantCost[e.CatalogVariantID])
		}
		if e.RetailCents != wantRetail[e.CatalogVariantID] {
			t.Errorf("%s retail: got %d want %d", e.CatalogVariantID,
				e.RetailCents, wantRetail[e.CatalogVariantID])
		}
	}
}

func TestBuildVariantMetadata_RejectsUnparseablePrice(t *testing.T) {
	// Reviewer P2 (2nd pass): provider may return malformed price strings;
	// silently writing 0-cost SKUs would let thin-margin orders past the gate
	// and pollute the public listing.
	cases := []struct {
		name    string
		price   string
		wantErr bool
	}{
		{"empty price", "", true},
		{"non-numeric", "free", true},
		{"three decimals", "4.999", true},
		{"valid integer", "10", false},
		{"valid decimal", "8.29", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildVariantMetadata([]contracts.CatalogVariant{{ID: "v", Price: tc.price}}, 1.5)
			if (err != nil) != tc.wantErr {
				t.Fatalf("price %q: gotErr=%v wantErr=%v", tc.price, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// P1 (5th pass): currency unit consistency between listing and PricingCoin
// ---------------------------------------------------------------------------

// TestEvaluateMarginGate_CrossCurrencyDiscountFailsClosed exercises the unit
// invariant: when exchangeRates is nil (TD-078 fallback), cross-currency
// discounts still fail closed rather than silently mixing units.
func TestEvaluateMarginGate_CrossCurrencyDiscountFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "1400", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	// Listing is USD; buyer pays BTC.
	oo.PricingCoin = "BTC"
	// Force a USD listing currency on the SignedListing.
	oo.Listings[0].Listing.Metadata = &pb.Listing_Metadata{
		PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
	}
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d1",
		ValueType:  "percentage",
		Amount:     "-50000", // 50000 satoshis ≠ 50000 USD cents
	}}

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("cross-currency order with discounts must fail closed (no FX in margin gate)")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s (msg=%s)", reason, msg)
	}
}

// TestEvaluateMarginGate_CrossCurrencyDiscountConverted verifies that when
// exchangeRates is available, cross-currency discounts are converted to the
// listing currency before prorate. Setup: listing $20 USD, cost $10,
// buyer pays EUR, -500 EUR-cents discount. At 1 EUR = 1.1 USD, the
// converted discount ≈ 550 USD-cents. Effective revenue = 2000-550 = 1450.
// cost+ship = 1000+450 = 1450 → 100% ≥ 80% → REJECT.
func TestEvaluateMarginGate_CrossCurrencyDiscountConverted(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "1000", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})
	svc.SetExchangeRates(newTestFXProvider(t))

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	oo.PricingCoin = "EUR"
	oo.Listings[0].Listing.Metadata = &pb.Listing_Metadata{
		PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
	}
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d1", ValueType: "fixed", Amount: "-500",
	}}

	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("converted discount should push cost ratio past 80%, must reject")
	}
	if reason != contracts.FailureReasonMarginProtectionFailed {
		t.Errorf("want margin_protection_failed, got %s", reason)
	}
}

// TestEvaluateMarginGate_SameCurrencyDiscountStillProrates verifies the
// unit gate does NOT block the common case (USD listing, USD payment via
// Stripe/PayPal): same currency → direct subtract is managed_escrow.
func TestEvaluateMarginGate_SameCurrencyDiscountStillProrates(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	oo.PricingCoin = "USD"
	oo.Listings[0].Listing.Metadata = &pb.Listing_Metadata{
		PricingCurrency: &pb.Currency{Code: "USD", Divisibility: 2},
	}
	// $5 off USD listing, paid in USD → safe to prorate directly.
	oo.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{{
		DiscountID: "d1", ValueType: "fixed", Amount: "-500",
	}}

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("USD/USD order should still prorate normally; reason=%s msg=%s", reason, msg)
	}
}

// TestEvaluateMarginGate_ShippingCurrencyMismatchFailsClosed exercises the
// case where the supplier returns shipping rates in a different currency
// than the listing (EUR shipping for a USD-priced supply-chain listing)
// and no ExchangeRates is available.
func TestEvaluateMarginGate_ShippingCurrencyMismatchFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{{ID: "std", Rate: "4.50", Currency: "EUR"}}, nil
		},
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("EUR shipping rate on USD listing must fail closed")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s (msg=%s)", reason, msg)
	}
}

// TestEvaluateMarginGate_ShippingCurrencyConverted verifies that when
// exchangeRates is available, a EUR shipping rate is converted to USD before
// adding to supplier cost. Setup: cost $5 USD, retail $20 USD, shipping
// 4.50 EUR → ~4.95 USD. Total cost = 500+495 = 995. 995/2000 = 49.7% < 80% → PASS.
func TestEvaluateMarginGate_ShippingCurrencyConverted(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			return []contracts.ShippingEstimate{{ID: "std", Rate: "4.50", Currency: "EUR"}}, nil
		},
	})
	svc.SetExchangeRates(newTestFXProvider(t))

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if !ok {
		t.Fatalf("EUR shipping converted to USD should keep margin OK; reason=%s msg=%s", reason, msg)
	}
}

// TestEvaluateMarginGate_ShippingMissingCurrencyFailsClosed asserts an
// estimate with empty Currency is rejected — provider adapters MUST declare
// the currency explicitly.
func TestEvaluateMarginGate_ShippingMissingCurrencyFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{
		id: "p1", provType: "pod",
		estimateFn: func(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
			// Currency intentionally left empty.
			return []contracts.ShippingEstimate{{ID: "std", Rate: "4.50"}}, nil
		},
	})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	ok, reason, _ := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("missing shipping Currency must fail closed (no silent default)")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s", reason)
	}
}

// TestEvaluateMarginGate_MissingPricingCurrencyFailsClosed asserts a
// supply-chain listing with no PricingCurrency.Code is rejected — we cannot
// validate the unit invariant in that state.
func TestEvaluateMarginGate_MissingPricingCurrencyFailsClosed(t *testing.T) {
	tdb := newSCTestDatabase(t)
	seedSyncedProduct(t, tdb, "tee", "p1", "500", nil)
	svc := newSCSvcWithProvider(t, tdb, &stubFulfillmentProvider{id: "p1", provType: "pod"})

	oo := makeOrderOpenSingleSKU("tee", "2000", 1)
	// Strip Metadata entirely.
	oo.Listings[0].Listing.Metadata = nil

	ok, reason, msg := svc.evaluateMarginGate(context.Background(), supplyMarginInputs{
		oo: oo, providerID: "p1",
	})
	if ok {
		t.Fatal("listing without PricingCurrency must fail closed")
	}
	if reason != contracts.FailureReasonManualActionRequired {
		t.Errorf("want manual_action_required, got %s (msg=%s)", reason, msg)
	}
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// newTestFXProvider returns an ExchangeRateProvider pre-loaded with a fixed
// USD → EUR rate (1 EUR = 1.10 USD). This makes cross-currency margin gate
// tests deterministic without network calls.
func newTestFXProvider(t *testing.T) *wallet.ExchangeRateProvider {
	t.Helper()
	return wallet.NewFixedRateProvider("USD", map[models.CurrencyCode]iwallet.Amount{
		"EUR": iwallet.NewAmount(110), // 1 EUR = 1.10 USD (divisibility 2)
	})
}
