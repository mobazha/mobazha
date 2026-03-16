package dbstore

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDiscountStore(t *testing.T) *database.GormDiscountStore {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := NewMemoryDB(tmpDir)
	require.NoError(t, err)
	require.NoError(t, database.MigrateDiscountModels(db))
	t.Cleanup(func() { db.Close() })
	return database.NewGormDiscountStore(db)
}

func makeTestDiscount(id string) *models.Discount {
	now := time.Now()
	return &models.Discount{
		ID:              id,
		TenantID:        "_default",
		Title:           "Test Discount " + id,
		Method:          models.DiscountMethodCode,
		Status:          models.DiscountStatusActive,
		ValueType:       models.DiscountValuePercentage,
		Value:           "10",
		Scope:           models.DiscountScopeOrder,
		AppliesTo:       models.DiscountAppliesToAll,
		MinPurchaseType: models.DiscountMinPurchaseNone,
		StartsAt:        now.Add(-time.Hour),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestGormDiscountStore_CreateAndGet(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	d.Codes = []models.DiscountCode{
		{ID: "c1", DiscountID: "d1", Code: "SAVE10", CodeHash: "hash1"},
	}

	require.NoError(t, store.CreateDiscount(ctx, d))

	got, err := store.GetDiscount(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, "d1", got.ID)
	assert.Equal(t, "Test Discount d1", got.Title)
	assert.Len(t, got.Codes, 1)
	assert.Equal(t, "SAVE10", got.Codes[0].Code)
}

func TestGormDiscountStore_GetDiscount_NotFound(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	_, err := store.GetDiscount(ctx, "nonexistent")
	assert.ErrorIs(t, err, database.ErrDiscountNotFound)
}

func TestGormDiscountStore_ListDiscounts(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		d := makeTestDiscount("d" + string(rune('0'+i)))
		require.NoError(t, store.CreateDiscount(ctx, d))
	}

	discounts, total, err := store.ListDiscounts(ctx, contracts.DiscountFilter{Page: 1, PageSize: 3})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, discounts, 3)
}

func TestGormDiscountStore_ListDiscounts_FilterByStatus(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d1 := makeTestDiscount("d1")
	d1.Status = models.DiscountStatusActive
	require.NoError(t, store.CreateDiscount(ctx, d1))

	d2 := makeTestDiscount("d2")
	d2.Status = models.DiscountStatusDraft
	require.NoError(t, store.CreateDiscount(ctx, d2))

	active := models.DiscountStatusActive
	discounts, total, err := store.ListDiscounts(ctx, contracts.DiscountFilter{
		Status: &active, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, discounts, 1)
	assert.Equal(t, "d1", discounts[0].ID)
}

func TestGormDiscountStore_Update(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	require.NoError(t, store.CreateDiscount(ctx, d))

	d.Title = "Updated Title"
	require.NoError(t, store.UpdateDiscount(ctx, d))

	got, err := store.GetDiscount(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", got.Title)
}

func TestGormDiscountStore_SoftDelete(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	require.NoError(t, store.CreateDiscount(ctx, d))

	require.NoError(t, store.SoftDeleteDiscount(ctx, "d1"))

	_, err := store.GetDiscount(ctx, "d1")
	assert.ErrorIs(t, err, database.ErrDiscountNotFound)

	count, err := store.CountDiscounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestGormDiscountStore_Codes_CRUD(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	require.NoError(t, store.CreateDiscount(ctx, d))

	codes := []models.DiscountCode{
		{ID: "c1", DiscountID: "d1", Code: "A", CodeHash: "hashA"},
		{ID: "c2", DiscountID: "d1", Code: "B", CodeHash: "hashB"},
	}
	require.NoError(t, store.CreateCodes(ctx, codes))

	listed, err := store.ListCodes(ctx, "d1")
	require.NoError(t, err)
	assert.Len(t, listed, 2)

	require.NoError(t, store.DeleteCode(ctx, "c1"))
	listed, err = store.ListCodes(ctx, "d1")
	require.NoError(t, err)
	assert.Len(t, listed, 1)
}

func TestGormDiscountStore_FindCodeByHash(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	d.Codes = []models.DiscountCode{
		{ID: "c1", DiscountID: "d1", Code: "SAVE10", CodeHash: "unique_hash"},
	}
	require.NoError(t, store.CreateDiscount(ctx, d))

	found, err := store.FindCodeByHash(ctx, "unique_hash")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "c1", found.ID)

	notFound, err := store.FindCodeByHash(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestGormDiscountStore_IncrementUsageWithCheck(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	d.UsageLimit = 2
	d.UsageCount = 0
	require.NoError(t, store.CreateDiscount(ctx, d))

	require.NoError(t, store.IncrementUsageWithCheck(ctx, "d1", nil))
	require.NoError(t, store.IncrementUsageWithCheck(ctx, "d1", nil))

	err := store.IncrementUsageWithCheck(ctx, "d1", nil)
	assert.ErrorIs(t, err, database.ErrUsageLimitReached)

	got, err := store.GetDiscount(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, 2, got.UsageCount)
}

func TestGormDiscountStore_IncrementUsageWithCheck_UnlimitedUsage(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	d.UsageLimit = 0
	require.NoError(t, store.CreateDiscount(ctx, d))

	for i := 0; i < 10; i++ {
		require.NoError(t, store.IncrementUsageWithCheck(ctx, "d1", nil))
	}
}

func TestGormDiscountStore_IncrementUsageWithCheck_CodeLimit(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	d.UsageLimit = 0
	d.Codes = []models.DiscountCode{
		{ID: "c1", DiscountID: "d1", Code: "X", CodeHash: "hx", UsageLimit: 1},
	}
	require.NoError(t, store.CreateDiscount(ctx, d))

	codeID := "c1"
	require.NoError(t, store.IncrementUsageWithCheck(ctx, "d1", &codeID))

	err := store.IncrementUsageWithCheck(ctx, "d1", &codeID)
	assert.ErrorIs(t, err, database.ErrUsageLimitReached)
}

func TestGormDiscountStore_Redemptions(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	d := makeTestDiscount("d1")
	require.NoError(t, store.CreateDiscount(ctx, d))

	r := &models.DiscountRedemption{
		ID:             "r1",
		DiscountID:     "d1",
		OrderID:        "order1",
		CustomerPeerID: "peer1",
		DiscountAmount: "100",
		Currency:       "USD",
		RedeemedAt:     time.Now(),
	}
	require.NoError(t, store.CreateRedemption(ctx, r))

	count, err := store.CountCustomerRedemptions(ctx, "d1", "peer1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	redemptions, total, err := store.ListRedemptions(ctx, "d1", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, redemptions, 1)
	assert.Equal(t, "order1", redemptions[0].OrderID)
}

func TestGormDiscountStore_GetApplicableDiscounts(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	now := time.Now()

	d1 := makeTestDiscount("d1")
	d1.Method = models.DiscountMethodAutomatic
	d1.Status = models.DiscountStatusActive
	d1.StartsAt = now.Add(-time.Hour)
	require.NoError(t, store.CreateDiscount(ctx, d1))

	d2 := makeTestDiscount("d2")
	d2.Method = models.DiscountMethodCode
	d2.Status = models.DiscountStatusActive
	d2.StartsAt = now.Add(-time.Hour)
	require.NoError(t, store.CreateDiscount(ctx, d2))

	discounts, err := store.GetApplicableDiscounts(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, discounts, 1)
	assert.Equal(t, "d1", discounts[0].ID)
}

func TestGormDiscountStore_GetApplicableDiscounts_ProductFilter(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	now := time.Now()

	d1 := makeTestDiscount("d1")
	d1.Method = models.DiscountMethodAutomatic
	d1.Status = models.DiscountStatusActive
	d1.AppliesTo = models.DiscountAppliesToSpecificProducts
	d1.ProductIDs = models.StringSlice{"prod1", "prod2"}
	d1.StartsAt = now.Add(-time.Hour)
	require.NoError(t, store.CreateDiscount(ctx, d1))

	d2 := makeTestDiscount("d2")
	d2.Method = models.DiscountMethodAutomatic
	d2.Status = models.DiscountStatusActive
	d2.AppliesTo = models.DiscountAppliesToAll
	d2.StartsAt = now.Add(-time.Hour)
	require.NoError(t, store.CreateDiscount(ctx, d2))

	discounts, err := store.GetApplicableDiscounts(ctx, []string{"prod1"})
	require.NoError(t, err)
	assert.Len(t, discounts, 2)

	discounts, err = store.GetApplicableDiscounts(ctx, []string{"prod99"})
	require.NoError(t, err)
	assert.Len(t, discounts, 1)
	assert.Equal(t, "d2", discounts[0].ID)
}

func TestGormDiscountStore_CountDiscounts(t *testing.T) {
	store := newTestDiscountStore(t)
	ctx := context.Background()

	count, err := store.CountDiscounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, store.CreateDiscount(ctx, makeTestDiscount("d1")))
	require.NoError(t, store.CreateDiscount(ctx, makeTestDiscount("d2")))

	count, err = store.CountDiscounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
