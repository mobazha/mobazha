package core

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDiscountService implements contracts.DiscountService for engine tests.
type mockDiscountService struct {
	discounts map[string]*models.Discount
	codes     map[string]*codeEntry // keyed by lowercase code
}

type codeEntry struct {
	discount *models.Discount
	code     *models.DiscountCode
}

func newMockDiscountService() *mockDiscountService {
	return &mockDiscountService{
		discounts: make(map[string]*models.Discount),
		codes:     make(map[string]*codeEntry),
	}
}

func (m *mockDiscountService) registerCode(code string, discount *models.Discount, dc *models.DiscountCode) {
	m.discounts[discount.ID] = discount
	m.codes[code] = &codeEntry{discount: discount, code: dc}
}

func (m *mockDiscountService) ValidateCode(_ context.Context, code string, _ string) (*contracts.ValidateCodeResult, error) {
	entry, ok := m.codes[code]
	if !ok {
		return &contracts.ValidateCodeResult{Valid: false, Reason: "INVALID"}, nil
	}
	return &contracts.ValidateCodeResult{Valid: true, Discount: entry.discount, Code: entry.code}, nil
}

// Unused methods — satisfy the interface.
func (m *mockDiscountService) CreateDiscount(_ context.Context, _ *models.Discount) error {
	return nil
}
func (m *mockDiscountService) GetDiscount(_ context.Context, _ string) (*models.Discount, error) {
	return nil, nil
}
func (m *mockDiscountService) ListDiscounts(_ context.Context, _ contracts.DiscountFilter) ([]models.Discount, int64, error) {
	return nil, 0, nil
}
func (m *mockDiscountService) UpdateDiscount(_ context.Context, _ *models.Discount) error {
	return nil
}
func (m *mockDiscountService) DeleteDiscount(_ context.Context, _ string) error { return nil }
func (m *mockDiscountService) AddCodes(_ context.Context, _ string, _ []models.DiscountCode) error {
	return nil
}
func (m *mockDiscountService) GenerateCodes(_ context.Context, _ string, _ int, _ string) ([]models.DiscountCode, error) {
	return nil, nil
}
func (m *mockDiscountService) ListCodes(_ context.Context, _ string) ([]models.DiscountCode, error) {
	return nil, nil
}
func (m *mockDiscountService) DeleteCode(_ context.Context, _ string) error { return nil }
func (m *mockDiscountService) GetApplicableDiscounts(_ context.Context, _ []string) ([]models.Discount, error) {
	return nil, nil
}
func (m *mockDiscountService) RecordRedemption(_ context.Context, _ string, _ *string, _, _, _, _ string) error {
	return nil
}
func (m *mockDiscountService) ListRedemptions(_ context.Context, _ string, _, _ int) ([]models.DiscountRedemption, int64, error) {
	return nil, 0, nil
}
func (m *mockDiscountService) CalculateDiscounts(_ context.Context, _ contracts.CalculateDiscountsRequest) (*contracts.CalculateDiscountsResult, error) {
	return nil, nil
}

// mockEngineStore implements the DiscountStore methods needed by the engine.
type mockEngineStore struct {
	autoDiscounts []models.Discount
}

func (m *mockEngineStore) GetApplicableDiscounts(_ context.Context, _ []string) ([]models.Discount, error) {
	return m.autoDiscounts, nil
}

// Remaining store methods unused by engine.
func (m *mockEngineStore) CreateDiscount(_ context.Context, _ *models.Discount) error { return nil }
func (m *mockEngineStore) GetDiscount(_ context.Context, _ string) (*models.Discount, error) {
	return nil, nil
}
func (m *mockEngineStore) ListDiscounts(_ context.Context, _ contracts.DiscountFilter) ([]models.Discount, int64, error) {
	return nil, 0, nil
}
func (m *mockEngineStore) UpdateDiscount(_ context.Context, _ *models.Discount) error { return nil }
func (m *mockEngineStore) SoftDeleteDiscount(_ context.Context, _ string) error       { return nil }
func (m *mockEngineStore) CreateCodes(_ context.Context, _ []models.DiscountCode) error {
	return nil
}
func (m *mockEngineStore) ListCodes(_ context.Context, _ string) ([]models.DiscountCode, error) {
	return nil, nil
}
func (m *mockEngineStore) DeleteCode(_ context.Context, _ string) error { return nil }
func (m *mockEngineStore) FindCodeByHash(_ context.Context, _ string) (*models.DiscountCode, error) {
	return nil, nil
}
func (m *mockEngineStore) IncrementUsageWithCheck(_ context.Context, _ string, _ *string) error {
	return nil
}
func (m *mockEngineStore) CountCustomerRedemptions(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (m *mockEngineStore) CreateRedemption(_ context.Context, _ *models.DiscountRedemption) error {
	return nil
}
func (m *mockEngineStore) ListRedemptions(_ context.Context, _ string, _, _ int) ([]models.DiscountRedemption, int64, error) {
	return nil, 0, nil
}
func (m *mockEngineStore) CountDiscounts(_ context.Context) (int64, error) { return 0, nil }

func activeDiscount(id, title string, valueType models.DiscountValueType, value string) *models.Discount {
	return &models.Discount{
		ID:                   id,
		Title:                title,
		Method:               models.DiscountMethodCode,
		Status:               models.DiscountStatusActive,
		ValueType:            valueType,
		Value:                value,
		Scope:                models.DiscountScopeOrder,
		AppliesTo:            models.DiscountAppliesToAll,
		MinPurchaseType:      models.DiscountMinPurchaseNone,
		StartsAt:             time.Now().Add(-time.Hour),
		CombinesWithProduct:  true,
		CombinesWithShipping: true,
	}
}

func TestDiscountEngine_PercentageCode(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "10% Off", models.DiscountValuePercentage, "10")
	code := &models.DiscountCode{ID: "c1", Code: "SAVE10"}
	svc.registerCode("SAVE10", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"SAVE10"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.Equal(t, "-1000", result.AppliedDiscounts[0].Amount)
	assert.Equal(t, big.NewInt(-1000), result.DiscountsTotal)
}

func TestDiscountEngine_FixedAmountCode(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "$5 Off", models.DiscountValueFixed, "500")
	d.Currency = "USD"
	code := &models.DiscountCode{ID: "c1", Code: "FLAT5"}
	svc.registerCode("FLAT5", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"FLAT5"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.Equal(t, "-500", result.AppliedDiscounts[0].Amount)
	assert.Equal(t, big.NewInt(-500), result.DiscountsTotal)
}

func TestDiscountEngine_FixedAmount_CappedToSubTotal(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "$100 Off", models.DiscountValueFixed, "10000")
	d.Currency = "USD"
	code := &models.DiscountCode{ID: "c1", Code: "BIG"}
	svc.registerCode("BIG", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"BIG"},
		SubTotal:      big.NewInt(3000),
	})
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(-3000), result.DiscountsTotal)
}

func TestDiscountEngine_PercentageWithMaxCap(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	maxAmt := "500"
	d := activeDiscount("d1", "50% Off max $5", models.DiscountValuePercentage, "50")
	d.MaxDiscountAmount = &maxAmt
	code := &models.DiscountCode{ID: "c1", Code: "HALF"}
	svc.registerCode("HALF", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"HALF"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	// 50% of 10000 = 5000, capped to 500
	assert.Equal(t, big.NewInt(-500), result.DiscountsTotal)
}

func TestDiscountEngine_FreeShipping(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "Free Ship", models.DiscountValueFreeShipping, "")
	code := &models.DiscountCode{ID: "c1", Code: "FREESHIP"}
	svc.registerCode("FREESHIP", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"FREESHIP"},
		SubTotal:      big.NewInt(5000),
	})
	require.NoError(t, err)
	assert.True(t, result.ShippingDiscount)
	assert.Equal(t, big.NewInt(0), result.DiscountsTotal)
}

func TestDiscountEngine_AutomaticDiscount(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{
		autoDiscounts: []models.Discount{
			{
				ID: "auto1", Title: "Auto 5%", Method: models.DiscountMethodAutomatic,
				Status: models.DiscountStatusActive, ValueType: models.DiscountValuePercentage,
				Value: "5", Scope: models.DiscountScopeOrder, AppliesTo: models.DiscountAppliesToAll,
				MinPurchaseType: models.DiscountMinPurchaseNone,
				StartsAt:        time.Now().Add(-time.Hour),
				CombinesWithProduct: true, CombinesWithShipping: true,
			},
		},
	}

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		SubTotal: big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.True(t, result.AppliedDiscounts[0].Auto)
	assert.Equal(t, big.NewInt(-500), result.DiscountsTotal)
}

func TestDiscountEngine_MinPurchaseAmount_NotMet(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	minAmt := "5000"
	d := activeDiscount("d1", "10% Off", models.DiscountValuePercentage, "10")
	d.MinPurchaseType = models.DiscountMinPurchaseAmount
	d.MinAmount = &minAmt
	code := &models.DiscountCode{ID: "c1", Code: "MIN50"}
	svc.registerCode("MIN50", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"MIN50"},
		SubTotal:      big.NewInt(3000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_MinPurchaseAmount_Met(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	minAmt := "5000"
	d := activeDiscount("d1", "10% Off", models.DiscountValuePercentage, "10")
	d.MinPurchaseType = models.DiscountMinPurchaseAmount
	d.MinAmount = &minAmt
	code := &models.DiscountCode{ID: "c1", Code: "MIN50"}
	svc.registerCode("MIN50", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"MIN50"},
		SubTotal:      big.NewInt(6000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
}

func TestDiscountEngine_MinQuantity_NotMet(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	minQty := 3
	d := activeDiscount("d1", "10% Off", models.DiscountValuePercentage, "10")
	d.MinPurchaseType = models.DiscountMinPurchaseQuantity
	d.MinQuantity = &minQty
	code := &models.DiscountCode{ID: "c1", Code: "QTY3"}
	svc.registerCode("QTY3", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"QTY3"},
		SubTotal:      big.NewInt(10000),
		ItemQuantity:  2,
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_Stacking_ProductAndOrder_Combinable(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	dProduct := activeDiscount("dp", "Prod 10%", models.DiscountValuePercentage, "10")
	dProduct.Scope = models.DiscountScopeProduct
	dProduct.CombinesWithOrder = true
	cProduct := &models.DiscountCode{ID: "cp", Code: "PROD10"}
	svc.registerCode("PROD10", dProduct, cProduct)

	dOrder := activeDiscount("do", "Order 5%", models.DiscountValuePercentage, "5")
	dOrder.Scope = models.DiscountScopeOrder
	dOrder.CombinesWithProduct = true
	cOrder := &models.DiscountCode{ID: "co", Code: "ORDER5"}
	svc.registerCode("ORDER5", dOrder, cOrder)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"PROD10", "ORDER5"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 2)
	// 10% + 5% = 1500
	assert.Equal(t, big.NewInt(-1500), result.DiscountsTotal)
}

func TestDiscountEngine_Stacking_ProductAndOrder_NotCombinable(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	dProduct := activeDiscount("dp", "Prod 10%", models.DiscountValuePercentage, "10")
	dProduct.Scope = models.DiscountScopeProduct
	dProduct.CombinesWithOrder = false
	cProduct := &models.DiscountCode{ID: "cp", Code: "PROD10"}
	svc.registerCode("PROD10", dProduct, cProduct)

	dOrder := activeDiscount("do", "Order 20%", models.DiscountValuePercentage, "20")
	dOrder.Scope = models.DiscountScopeOrder
	dOrder.CombinesWithProduct = false
	cOrder := &models.DiscountCode{ID: "co", Code: "ORDER20"}
	svc.registerCode("ORDER20", dOrder, cOrder)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"PROD10", "ORDER20"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	// Not combinable → picks best (20% = 2000 > 10% = 1000)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.Equal(t, "do", result.AppliedDiscounts[0].DiscountID)
	assert.Equal(t, big.NewInt(-2000), result.DiscountsTotal)
}

func TestDiscountEngine_Stacking_ShippingBlockedByNonCombinableDiscount(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	dOrder := activeDiscount("do", "Order 10%", models.DiscountValuePercentage, "10")
	dOrder.Scope = models.DiscountScopeOrder
	dOrder.CombinesWithShipping = false
	cOrder := &models.DiscountCode{ID: "co", Code: "ORDER10"}
	svc.registerCode("ORDER10", dOrder, cOrder)

	dShip := activeDiscount("ds", "Free Ship", models.DiscountValueFreeShipping, "")
	cShip := &models.DiscountCode{ID: "cs", Code: "SHIP"}
	svc.registerCode("SHIP", dShip, cShip)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"ORDER10", "SHIP"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.False(t, result.ShippingDiscount)
}

func TestDiscountEngine_InvalidCodeSkipped(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"NOSUCHCODE"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
	assert.Equal(t, big.NewInt(0), result.DiscountsTotal)
}

func TestDiscountEngine_ExpiredDiscountFiltered(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	ended := time.Now().Add(-time.Hour)
	d := activeDiscount("d1", "Expired", models.DiscountValuePercentage, "10")
	d.EndsAt = &ended
	code := &models.DiscountCode{ID: "c1", Code: "OLD"}
	svc.registerCode("OLD", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"OLD"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_UsageLimitReachedFiltered(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "Used Up", models.DiscountValuePercentage, "10")
	d.UsageLimit = 5
	d.UsageCount = 5
	code := &models.DiscountCode{ID: "c1", Code: "USED"}
	svc.registerCode("USED", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"USED"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_ProductScope_NoMatch(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "Prod Only", models.DiscountValuePercentage, "10")
	d.AppliesTo = models.DiscountAppliesToSpecificProducts
	d.ProductIDs = models.StringSlice{"prod1", "prod2"}
	code := &models.DiscountCode{ID: "c1", Code: "PROD"}
	svc.registerCode("PROD", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"PROD"},
		ProductIDs:    []string{"prod99"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_NoDiscounts(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		SubTotal: big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
	assert.Equal(t, big.NewInt(0), result.DiscountsTotal)
	assert.False(t, result.ShippingDiscount)
}

// mockCollectionStoreForEngine implements contracts.CollectionStore for engine tests.
type mockCollectionStoreForEngine struct {
	membership map[string]map[string]bool // collectionID → slug → bool
}

func (m *mockCollectionStoreForEngine) IsProductInCollections(_ context.Context, collectionIDs []string, slug string) (bool, error) {
	for _, cid := range collectionIDs {
		if slugs, ok := m.membership[cid]; ok {
			if slugs[slug] {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *mockCollectionStoreForEngine) CreateCollection(_ context.Context, _ *models.Collection) error {
	return nil
}
func (m *mockCollectionStoreForEngine) GetCollection(_ context.Context, _ string) (*models.Collection, error) {
	return nil, nil
}
func (m *mockCollectionStoreForEngine) ListCollections(_ context.Context, _, _ int, _ bool) ([]*models.Collection, int64, error) {
	return nil, 0, nil
}
func (m *mockCollectionStoreForEngine) UpdateCollection(_ context.Context, _ *models.Collection) error {
	return nil
}
func (m *mockCollectionStoreForEngine) DeleteCollection(_ context.Context, _ string) error {
	return nil
}
func (m *mockCollectionStoreForEngine) AddProducts(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *mockCollectionStoreForEngine) RemoveProduct(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockCollectionStoreForEngine) ReorderProducts(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *mockCollectionStoreForEngine) RemoveProductFromAllCollections(_ context.Context, _ string) error {
	return nil
}
func (m *mockCollectionStoreForEngine) CountCollections(_ context.Context) (int64, error) {
	return 0, nil
}
func (m *mockCollectionStoreForEngine) CountCollectionProducts(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func TestDiscountEngine_CollectionScope_Match(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}
	colStore := &mockCollectionStoreForEngine{
		membership: map[string]map[string]bool{
			"summer-col": {"tshirt-1": true, "tshirt-2": true},
		},
	}

	d := activeDiscount("d1", "Summer 10%", models.DiscountValuePercentage, "10")
	d.AppliesTo = models.DiscountAppliesToSpecificCollections
	d.CollectionIDs = models.StringSlice{"summer-col"}
	code := &models.DiscountCode{ID: "c1", Code: "SUMMER"}
	svc.registerCode("SUMMER", d, code)

	engine := NewDiscountEngine(svc, store, colStore)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"SUMMER"},
		ProductIDs:    []string{"tshirt-1"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 1)
	assert.Equal(t, big.NewInt(-1000), result.DiscountsTotal)
}

func TestDiscountEngine_CollectionScope_NoMatch(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}
	colStore := &mockCollectionStoreForEngine{
		membership: map[string]map[string]bool{
			"summer-col": {"tshirt-1": true},
		},
	}

	d := activeDiscount("d1", "Summer 10%", models.DiscountValuePercentage, "10")
	d.AppliesTo = models.DiscountAppliesToSpecificCollections
	d.CollectionIDs = models.StringSlice{"summer-col"}
	code := &models.DiscountCode{ID: "c1", Code: "SUMMER"}
	svc.registerCode("SUMMER", d, code)

	engine := NewDiscountEngine(svc, store, colStore)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"SUMMER"},
		ProductIDs:    []string{"pants-1"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}

func TestDiscountEngine_CollectionScope_NilStore(t *testing.T) {
	svc := newMockDiscountService()
	store := &mockEngineStore{}

	d := activeDiscount("d1", "Col 10%", models.DiscountValuePercentage, "10")
	d.AppliesTo = models.DiscountAppliesToSpecificCollections
	d.CollectionIDs = models.StringSlice{"col1"}
	code := &models.DiscountCode{ID: "c1", Code: "COL"}
	svc.registerCode("COL", d, code)

	engine := NewDiscountEngine(svc, store, nil)
	result, err := engine.Calculate(context.Background(), DiscountContext{
		DiscountCodes: []string{"COL"},
		ProductIDs:    []string{"prod1"},
		SubTotal:      big.NewInt(10000),
	})
	require.NoError(t, err)
	assert.Len(t, result.AppliedDiscounts, 0)
}
