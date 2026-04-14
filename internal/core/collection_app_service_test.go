package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCollectionStore struct {
	collections map[string]*models.Collection
	products    map[string][]models.CollectionProduct // keyed by collectionID

	createErr error
}

func newMockCollectionStore() *mockCollectionStore {
	return &mockCollectionStore{
		collections: make(map[string]*models.Collection),
		products:    make(map[string][]models.CollectionProduct),
	}
}

func (m *mockCollectionStore) CreateCollection(_ context.Context, c *models.Collection) error {
	if m.createErr != nil {
		return m.createErr
	}
	cp := *c
	m.collections[c.ID] = &cp
	return nil
}

func (m *mockCollectionStore) GetCollection(_ context.Context, id string) (*models.Collection, error) {
	c, ok := m.collections[id]
	if !ok || c.DeletedAt != nil {
		return nil, database.ErrCollectionNotFound
	}
	cp := *c
	cp.Products = m.products[id]
	return &cp, nil
}

func (m *mockCollectionStore) ListCollections(_ context.Context, page, pageSize int, publishedOnly bool) ([]*models.Collection, int64, error) {
	var result []*models.Collection
	for _, c := range m.collections {
		if c.DeletedAt != nil {
			continue
		}
		if publishedOnly && !c.Published {
			continue
		}
		cp := *c
		result = append(result, &cp)
	}
	total := int64(len(result))
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return nil, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockCollectionStore) UpdateCollection(_ context.Context, c *models.Collection) error {
	existing, ok := m.collections[c.ID]
	if !ok || existing.DeletedAt != nil {
		return database.ErrCollectionNotFound
	}
	existing.Title = c.Title
	existing.Description = c.Description
	existing.Published = c.Published
	return nil
}

func (m *mockCollectionStore) DeleteCollection(_ context.Context, id string) error {
	c, ok := m.collections[id]
	if !ok || c.DeletedAt != nil {
		return database.ErrCollectionNotFound
	}
	delete(m.collections, id)
	return nil
}

func (m *mockCollectionStore) AddProducts(_ context.Context, collectionID string, slugs []string) error {
	existing := m.products[collectionID]
	pos := len(existing)
	for i, slug := range slugs {
		existing = append(existing, models.CollectionProduct{
			CollectionID: collectionID,
			ListingSlug:  slug,
			Position:     pos + i,
		})
	}
	m.products[collectionID] = existing
	return nil
}

func (m *mockCollectionStore) RemoveProduct(_ context.Context, collectionID, slug string) error {
	prods := m.products[collectionID]
	for i, p := range prods {
		if p.ListingSlug == slug {
			m.products[collectionID] = append(prods[:i], prods[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockCollectionStore) ReorderProducts(_ context.Context, collectionID string, orderedSlugs []string) error {
	var reordered []models.CollectionProduct
	for i, slug := range orderedSlugs {
		reordered = append(reordered, models.CollectionProduct{
			CollectionID: collectionID,
			ListingSlug:  slug,
			Position:     i,
		})
	}
	m.products[collectionID] = reordered
	return nil
}

func (m *mockCollectionStore) IsProductInCollections(_ context.Context, collectionIDs []string, slug string) (bool, error) {
	idSet := make(map[string]bool)
	for _, id := range collectionIDs {
		idSet[id] = true
	}
	for cid, prods := range m.products {
		if !idSet[cid] {
			continue
		}
		for _, p := range prods {
			if p.ListingSlug == slug {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *mockCollectionStore) RemoveProductFromAllCollections(_ context.Context, slug string) error {
	for cid, prods := range m.products {
		var filtered []models.CollectionProduct
		for _, p := range prods {
			if p.ListingSlug != slug {
				filtered = append(filtered, p)
			}
		}
		m.products[cid] = filtered
	}
	return nil
}

func (m *mockCollectionStore) CountCollections(_ context.Context) (int64, error) {
	var count int64
	for _, c := range m.collections {
		if c.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (m *mockCollectionStore) CountCollectionProducts(_ context.Context, collectionID string) (int64, error) {
	return int64(len(m.products[collectionID])), nil
}

type mockEventBus struct {
	emitted []interface{}
}

func (b *mockEventBus) Subscribe(_ interface{}, _ ...events.SubscriptionOpt) (events.Subscription, error) {
	return nil, nil
}
func (b *mockEventBus) Emit(evt interface{}) { b.emitted = append(b.emitted, evt) }

func TestCollectionAppService_Create_Valid(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "tenant1")
	ctx := context.Background()

	c := &models.Collection{Title: "Summer Sale"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "tenant1", c.TenantID)
	assert.Equal(t, models.CollectionTypeManual, c.Type)
	assert.Equal(t, models.CollectionSortManual, c.SortOrder)
	assert.Len(t, bus.emitted, 2) // CollectionCreated + CollectionsChanged
}

func TestCollectionAppService_Create_TitleRequired(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")

	err := svc.CreateCollection(context.Background(), &models.Collection{})
	assert.ErrorIs(t, err, database.ErrCollectionTitleRequired)
}

func TestCollectionAppService_Create_PreservesExplicitType(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "Auto", Type: models.CollectionTypeAuto, SortOrder: models.CollectionSortCreatedDesc}
	require.NoError(t, svc.CreateCollection(ctx, c))

	assert.Equal(t, models.CollectionTypeAuto, c.Type)
	assert.Equal(t, models.CollectionSortCreatedDesc, c.SortOrder)
}

func TestCollectionAppService_Create_TenantQuota(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	for i := 0; i < maxCollectionsPerTenant; i++ {
		c := &models.Collection{Title: "C"}
		require.NoError(t, svc.CreateCollection(ctx, c))
	}

	err := svc.CreateCollection(ctx, &models.Collection{Title: "Over Limit"})
	assert.ErrorIs(t, err, database.ErrCollectionMaxReached)
}

func TestCollectionAppService_Update_Valid(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "Original"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	c.Title = "Updated"
	require.NoError(t, svc.UpdateCollection(ctx, c))
	assert.Len(t, bus.emitted, 4) // (CollectionCreated + CollectionsChanged) + (CollectionUpdated + CollectionsChanged)
}

func TestCollectionAppService_Update_TitleRequired(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "X"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	c.Title = ""
	err := svc.UpdateCollection(ctx, c)
	assert.ErrorIs(t, err, database.ErrCollectionTitleRequired)
}

func TestCollectionAppService_Delete(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "To Delete"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	require.NoError(t, svc.DeleteCollection(ctx, c.ID))
	assert.Len(t, bus.emitted, 4) // (CollectionCreated + CollectionsChanged) + (CollectionDeleted + CollectionsChanged)
}

func TestCollectionAppService_AddProducts_Valid(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "C1"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	require.NoError(t, svc.AddProducts(ctx, c.ID, []string{"slug-a", "slug-b"}))
	assert.Len(t, store.products[c.ID], 2)
	assert.Len(t, bus.emitted, 4) // (CollectionCreated + CollectionsChanged) + (CollectionProductsChanged + CollectionsChanged)
}

func TestCollectionAppService_AddProducts_EmptySlugs(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")

	err := svc.AddProducts(context.Background(), "col1", nil)
	assert.ErrorIs(t, err, database.ErrCollectionProductRequired)
}

func TestCollectionAppService_AddProducts_ExceedsLimit(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "Full"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	bigBatch := make([]string, maxProductsPerCollection+1)
	for i := range bigBatch {
		bigBatch[i] = "slug"
	}
	err := svc.AddProducts(ctx, c.ID, bigBatch)
	assert.ErrorIs(t, err, database.ErrCollectionProductMaxExceeded)
}

func TestCollectionAppService_RemoveProduct(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "C"}
	require.NoError(t, svc.CreateCollection(ctx, c))
	require.NoError(t, svc.AddProducts(ctx, c.ID, []string{"a", "b"}))

	require.NoError(t, svc.RemoveProduct(ctx, c.ID, "a"))
	assert.Len(t, store.products[c.ID], 1)
	assert.Equal(t, "b", store.products[c.ID][0].ListingSlug)
}

func TestCollectionAppService_ReorderProducts(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "C"}
	require.NoError(t, svc.CreateCollection(ctx, c))
	require.NoError(t, svc.AddProducts(ctx, c.ID, []string{"a", "b", "c"}))

	require.NoError(t, svc.ReorderProducts(ctx, c.ID, []string{"c", "a", "b"}))
	prods := store.products[c.ID]
	assert.Equal(t, "c", prods[0].ListingSlug)
	assert.Equal(t, 0, prods[0].Position)
	assert.Equal(t, "a", prods[1].ListingSlug)
	assert.Equal(t, "b", prods[2].ListingSlug)
}

func TestCollectionAppService_RemoveProductFromAllCollections(t *testing.T) {
	store := newMockCollectionStore()
	bus := &mockEventBus{}
	svc := NewCollectionAppService(store, bus, "t1")
	ctx := context.Background()

	c1 := &models.Collection{Title: "C1"}
	c2 := &models.Collection{Title: "C2"}
	require.NoError(t, svc.CreateCollection(ctx, c1))
	require.NoError(t, svc.CreateCollection(ctx, c2))

	require.NoError(t, svc.AddProducts(ctx, c1.ID, []string{"shared", "only-c1"}))
	require.NoError(t, svc.AddProducts(ctx, c2.ID, []string{"shared", "only-c2"}))

	eventsBefore := len(bus.emitted)
	require.NoError(t, svc.RemoveProductFromAllCollections(ctx, "shared"))

	assert.Len(t, store.products[c1.ID], 1)
	assert.Equal(t, "only-c1", store.products[c1.ID][0].ListingSlug)
	assert.Len(t, store.products[c2.ID], 1)
	assert.Equal(t, "only-c2", store.products[c2.ID][0].ListingSlug)

	assert.Greater(t, len(bus.emitted), eventsBefore)
	// The second-to-last event is CollectionProductsChanged; the last is CollectionsChanged (NetDB sync)
	productsEvt := bus.emitted[len(bus.emitted)-2].(events.CollectionProductsChanged)
	assert.Equal(t, events.CollectionProductActionBulkRemove, productsEvt.Action)
	assert.Equal(t, []string{"shared"}, productsEvt.Slugs)
}

func TestCollectionAppService_ReorderProducts_EmptySlugs(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")

	err := svc.ReorderProducts(context.Background(), "col1", nil)
	assert.ErrorIs(t, err, database.ErrCollectionProductRequired)
}

func TestCollectionAppService_ListCollections_Pagination(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		c := &models.Collection{Title: "C"}
		require.NoError(t, svc.CreateCollection(ctx, c))
	}

	items, total, err := svc.ListCollections(ctx, 1, 3, false)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, items, 3)
}

func TestCollectionAppService_ListCollections_NormalizesParams(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "C"}
	require.NoError(t, svc.CreateCollection(ctx, c))

	items, _, err := svc.ListCollections(ctx, 0, -1, false)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, _, err = svc.ListCollections(ctx, 1, 999, false)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestCollectionAppService_NilBus(t *testing.T) {
	store := newMockCollectionStore()
	svc := NewCollectionAppService(store, nil, "t1")
	ctx := context.Background()

	c := &models.Collection{Title: "No Bus"}
	require.NoError(t, svc.CreateCollection(ctx, c))
	require.NoError(t, svc.UpdateCollection(ctx, c))
	require.NoError(t, svc.DeleteCollection(ctx, c.ID))
}
