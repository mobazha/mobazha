package dbstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestCollectionStore(t *testing.T) *database.GormCollectionStore {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := NewMemoryDB(tmpDir)
	require.NoError(t, err)
	require.NoError(t, database.MigrateCollectionModels(db))
	t.Cleanup(func() { db.Close() })
	return database.NewGormCollectionStore(db)
}

func makeTestCollection(id, title string) *models.Collection {
	return &models.Collection{
		ID:        id,
		TenantID:  "_default",
		Title:     title,
		Type:      models.CollectionTypeManual,
		SortOrder: models.CollectionSortManual,
		Published: true,
	}
}

func TestGormCollectionStore_CreateAndGet(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Summer Sale")
	c.Description = "Hot deals"
	require.NoError(t, store.CreateCollection(ctx, c))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", got.ID)
	assert.Equal(t, "Summer Sale", got.Title)
	assert.Equal(t, "Hot deals", got.Description)
	assert.NotZero(t, got.CreatedAt)
}

func TestGormCollectionStore_Get_NotFound(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	_, err := store.GetCollection(ctx, "nonexistent")
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)
}

func TestGormCollectionStore_ListCollections(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		c := makeTestCollection("c"+string(rune('0'+i)), "Col")
		require.NoError(t, store.CreateCollection(ctx, c))
	}

	collections, total, err := store.ListCollections(ctx, 1, 3, false)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, collections, 3)
}

func TestGormCollectionStore_ListCollections_PublishedOnly(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c1 := makeTestCollection("c1", "Published")
	require.NoError(t, store.CreateCollection(ctx, c1))

	c2 := makeTestCollection("c2", "Draft")
	require.NoError(t, store.CreateCollection(ctx, c2))
	// GORM default:1 overrides bool zero-value on Create, so update after creation
	c2.Published = false
	require.NoError(t, store.UpdateCollection(ctx, c2))

	collections, total, err := store.ListCollections(ctx, 1, 20, true)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, collections, 1)
	assert.Equal(t, "c1", collections[0].ID)
}

func TestGormCollectionStore_Update(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Original")
	require.NoError(t, store.CreateCollection(ctx, c))

	c.Title = "Updated Title"
	c.Description = "New desc"
	require.NoError(t, store.UpdateCollection(ctx, c))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", got.Title)
	assert.Equal(t, "New desc", got.Description)
}

func TestGormCollectionStore_Update_NotFound(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("nonexistent", "X")
	err := store.UpdateCollection(ctx, c)
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)
}

func TestGormCollectionStore_SoftDelete(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "To Delete")
	require.NoError(t, store.CreateCollection(ctx, c))

	require.NoError(t, store.DeleteCollection(ctx, "c1"))

	_, err := store.GetCollection(ctx, "c1")
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)

	count, err := store.CountCollections(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestGormCollectionStore_SoftDelete_NotFound(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	err := store.DeleteCollection(ctx, "nonexistent")
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)
}

func TestGormCollectionStore_AddAndGetProducts(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "With Products")
	require.NoError(t, store.CreateCollection(ctx, c))

	require.NoError(t, store.AddProducts(ctx, "c1", []string{"slug-a", "slug-b", "slug-c"}))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, got.Products, 3)
	assert.Equal(t, "slug-a", got.Products[0].ListingSlug)
	assert.Equal(t, 0, got.Products[0].Position)
	assert.Equal(t, "slug-c", got.Products[2].ListingSlug)
	assert.Equal(t, 2, got.Products[2].Position)
}

func TestGormCollectionStore_AddProducts_Incremental(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Incremental")
	require.NoError(t, store.CreateCollection(ctx, c))

	require.NoError(t, store.AddProducts(ctx, "c1", []string{"first"}))
	require.NoError(t, store.AddProducts(ctx, "c1", []string{"second"}))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, got.Products, 2)
	assert.Equal(t, 0, got.Products[0].Position)
	assert.Equal(t, 1, got.Products[1].Position)
}

func TestGormCollectionStore_RemoveProduct(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Remove Test")
	require.NoError(t, store.CreateCollection(ctx, c))
	require.NoError(t, store.AddProducts(ctx, "c1", []string{"a", "b", "c"}))

	require.NoError(t, store.RemoveProduct(ctx, "c1", "b"))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, got.Products, 2)
	slugs := []string{got.Products[0].ListingSlug, got.Products[1].ListingSlug}
	assert.Contains(t, slugs, "a")
	assert.Contains(t, slugs, "c")
}

func TestGormCollectionStore_ReorderProducts(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Reorder")
	require.NoError(t, store.CreateCollection(ctx, c))
	require.NoError(t, store.AddProducts(ctx, "c1", []string{"a", "b", "c"}))

	require.NoError(t, store.ReorderProducts(ctx, "c1", []string{"c", "a", "b"}))

	got, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, got.Products, 3)
	assert.Equal(t, "c", got.Products[0].ListingSlug)
	assert.Equal(t, 0, got.Products[0].Position)
	assert.Equal(t, "a", got.Products[1].ListingSlug)
	assert.Equal(t, 1, got.Products[1].Position)
	assert.Equal(t, "b", got.Products[2].ListingSlug)
	assert.Equal(t, 2, got.Products[2].Position)
}

func TestGormCollectionStore_RemoveProductFromAllCollections(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c1 := makeTestCollection("c1", "Col1")
	c2 := makeTestCollection("c2", "Col2")
	require.NoError(t, store.CreateCollection(ctx, c1))
	require.NoError(t, store.CreateCollection(ctx, c2))

	require.NoError(t, store.AddProducts(ctx, "c1", []string{"shared", "only-c1"}))
	require.NoError(t, store.AddProducts(ctx, "c2", []string{"shared", "only-c2"}))

	require.NoError(t, store.RemoveProductFromAllCollections(ctx, "shared"))

	got1, err := store.GetCollection(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, got1.Products, 1)
	assert.Equal(t, "only-c1", got1.Products[0].ListingSlug)

	got2, err := store.GetCollection(ctx, "c2")
	require.NoError(t, err)
	assert.Len(t, got2.Products, 1)
	assert.Equal(t, "only-c2", got2.Products[0].ListingSlug)
}

func TestGormCollectionStore_IsProductInCollections(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Check")
	require.NoError(t, store.CreateCollection(ctx, c))
	require.NoError(t, store.AddProducts(ctx, "c1", []string{"exists"}))

	found, err := store.IsProductInCollections(ctx, []string{"c1"}, "exists")
	require.NoError(t, err)
	assert.True(t, found)

	found, err = store.IsProductInCollections(ctx, []string{"c1"}, "missing")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestGormCollectionStore_CountCollections(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	count, err := store.CountCollections(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, store.CreateCollection(ctx, makeTestCollection("c1", "A")))
	require.NoError(t, store.CreateCollection(ctx, makeTestCollection("c2", "B")))

	count, err = store.CountCollections(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestGormCollectionStore_CountCollectionProducts(t *testing.T) {
	store := newTestCollectionStore(t)
	ctx := context.Background()

	c := makeTestCollection("c1", "Count Products")
	require.NoError(t, store.CreateCollection(ctx, c))

	count, err := store.CountCollectionProducts(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, store.AddProducts(ctx, "c1", []string{"a", "b"}))

	count, err = store.CountCollectionProducts(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func newMultiTenantStores(t *testing.T) (storeA, storeB *database.GormCollectionStore) {
	t.Helper()
	dsn := fmt.Sprintf("file:memdb_%s?mode=memory&cache=shared", t.Name())
	sharedDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		AllowGlobalUpdate: true,
	})
	require.NoError(t, err)

	dbA, err := NewTenantDBWithPublicData(sharedDB, "tenant-a", pkgdb.PublicData(nil))
	require.NoError(t, err)
	dbB, err := NewTenantDBWithPublicData(sharedDB, "tenant-b", pkgdb.PublicData(nil))
	require.NoError(t, err)

	require.NoError(t, database.MigrateCollectionModels(dbA))
	t.Cleanup(func() {
		dbA.Close()
		dbB.Close()
		sqlDB, _ := sharedDB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})

	return database.NewGormCollectionStore(dbA), database.NewGormCollectionStore(dbB)
}

func TestGormCollectionStore_MultiTenant_CollectionIsolation(t *testing.T) {
	storeA, storeB := newMultiTenantStores(t)
	ctx := context.Background()

	cA := &models.Collection{ID: "c1", Title: "Tenant A Collection", Type: models.CollectionTypeManual, Published: true}
	require.NoError(t, storeA.CreateCollection(ctx, cA))

	cB := &models.Collection{ID: "c2", Title: "Tenant B Collection", Type: models.CollectionTypeManual, Published: true}
	require.NoError(t, storeB.CreateCollection(ctx, cB))

	collectionsA, totalA, err := storeA.ListCollections(ctx, 1, 100, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalA)
	assert.Len(t, collectionsA, 1)
	assert.Equal(t, "c1", collectionsA[0].ID)

	collectionsB, totalB, err := storeB.ListCollections(ctx, 1, 100, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalB)
	assert.Len(t, collectionsB, 1)
	assert.Equal(t, "c2", collectionsB[0].ID)

	_, err = storeA.GetCollection(ctx, "c2")
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)
}

func TestGormCollectionStore_MultiTenant_ProductIsolation(t *testing.T) {
	storeA, storeB := newMultiTenantStores(t)
	ctx := context.Background()

	cA := &models.Collection{ID: "coll-a", Title: "Coll A", Type: models.CollectionTypeManual, Published: true}
	require.NoError(t, storeA.CreateCollection(ctx, cA))
	require.NoError(t, storeA.AddProducts(ctx, "coll-a", []string{"shared-slug", "slug-a-only"}))

	cB := &models.Collection{ID: "coll-b", Title: "Coll B", Type: models.CollectionTypeManual, Published: true}
	require.NoError(t, storeB.CreateCollection(ctx, cB))
	require.NoError(t, storeB.AddProducts(ctx, "coll-b", []string{"shared-slug", "slug-b-only"}))

	gotA, err := storeA.GetCollection(ctx, "coll-a")
	require.NoError(t, err)
	assert.Len(t, gotA.Products, 2)
	assert.Equal(t, "shared-slug", gotA.Products[0].ListingSlug)
	assert.Equal(t, "slug-a-only", gotA.Products[1].ListingSlug)

	gotB, err := storeB.GetCollection(ctx, "coll-b")
	require.NoError(t, err)
	assert.Len(t, gotB.Products, 2)
	assert.Equal(t, "shared-slug", gotB.Products[0].ListingSlug)
	assert.Equal(t, "slug-b-only", gotB.Products[1].ListingSlug)

	_, err = storeA.GetCollection(ctx, "coll-b")
	assert.ErrorIs(t, err, database.ErrCollectionNotFound)

	countA, err := storeA.CountCollectionProducts(ctx, "coll-a")
	require.NoError(t, err)
	assert.Equal(t, int64(2), countA)

	countB, err := storeB.CountCollectionProducts(ctx, "coll-b")
	require.NoError(t, err)
	assert.Equal(t, int64(2), countB)

	collectionsA, _, err := storeA.ListCollections(ctx, 1, 100, false)
	require.NoError(t, err)
	assert.Len(t, collectionsA, 1)
	assert.Len(t, collectionsA[0].Products, 2)

	collectionsB, _, err := storeB.ListCollections(ctx, 1, 100, false)
	require.NoError(t, err)
	assert.Len(t, collectionsB, 1)
	assert.Len(t, collectionsB[0].Products, 2)
}
