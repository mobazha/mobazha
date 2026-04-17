package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	maxCollectionsPerTenant   = 50
	maxProductsPerCollection  = 200
	defaultCollectionPageSize = 20
	maxCollectionPageSize     = 100
)

type CollectionAppService struct {
	store    contracts.CollectionStore
	bus      events.Bus
	tenantID string
}

func NewCollectionAppService(store contracts.CollectionStore, bus events.Bus, tenantID string) *CollectionAppService {
	return &CollectionAppService{
		store:    store,
		bus:      bus,
		tenantID: tenantID,
	}
}

func (s *CollectionAppService) Store() contracts.CollectionStore {
	return s.store
}

func (s *CollectionAppService) CreateCollection(ctx context.Context, c *models.Collection) error {
	if c.Title == "" {
		return database.ErrCollectionTitleRequired
	}

	count, err := s.store.CountCollections(ctx)
	if err != nil {
		return fmt.Errorf("count collections: %w", err)
	}
	if count >= maxCollectionsPerTenant {
		return fmt.Errorf("%w: limit %d", database.ErrCollectionMaxReached, maxCollectionsPerTenant)
	}

	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.TenantID = s.tenantID

	if c.Type == "" {
		c.Type = models.CollectionTypeManual
	}
	if c.SortOrder == "" {
		c.SortOrder = models.CollectionSortManual
	}

	if err := s.store.CreateCollection(ctx, c); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionCreated{
			CollectionID: c.ID,
			Title:        c.Title,
			Type:         string(c.Type),
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func (s *CollectionAppService) GetCollection(ctx context.Context, id string) (*models.Collection, error) {
	return s.store.GetCollection(ctx, id)
}

func (s *CollectionAppService) ListCollections(ctx context.Context, page, pageSize int, publishedOnly bool) ([]*models.Collection, int64, error) {
	page, pageSize = normalizePageParams(page, pageSize)
	return s.store.ListCollections(ctx, page, pageSize, publishedOnly)
}

func (s *CollectionAppService) UpdateCollection(ctx context.Context, c *models.Collection) error {
	if c.Title == "" {
		return database.ErrCollectionTitleRequired
	}

	if err := s.store.UpdateCollection(ctx, c); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionUpdated{
			CollectionID: c.ID,
			Title:        c.Title,
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func (s *CollectionAppService) DeleteCollection(ctx context.Context, id string) error {
	if err := s.store.DeleteCollection(ctx, id); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionDeleted{CollectionID: id})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func (s *CollectionAppService) AddProducts(ctx context.Context, collectionID string, slugs []string) error {
	if len(slugs) == 0 {
		return database.ErrCollectionProductRequired
	}

	currentCount, err := s.store.CountCollectionProducts(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("count products: %w", err)
	}
	if currentCount+int64(len(slugs)) > maxProductsPerCollection {
		return fmt.Errorf("%w: limit %d", database.ErrCollectionProductMaxExceeded, maxProductsPerCollection)
	}

	if err := s.store.AddProducts(ctx, collectionID, slugs); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionProductsChanged{
			CollectionID: collectionID,
			Action:       events.CollectionProductActionAdd,
			Slugs:        slugs,
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func (s *CollectionAppService) RemoveProduct(ctx context.Context, collectionID, slug string) error {
	if err := s.store.RemoveProduct(ctx, collectionID, slug); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionProductsChanged{
			CollectionID: collectionID,
			Action:       events.CollectionProductActionRemove,
			Slugs:        []string{slug},
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func (s *CollectionAppService) ReorderProducts(ctx context.Context, collectionID string, orderedSlugs []string) error {
	if len(orderedSlugs) == 0 {
		return database.ErrCollectionProductRequired
	}

	if err := s.store.ReorderProducts(ctx, collectionID, orderedSlugs); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionProductsChanged{
			CollectionID: collectionID,
			Action:       events.CollectionProductActionReorder,
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

// IsProductInCollections forwards to the underlying store. Returns
// (false, nil) when collectionIDs is empty so callers can treat an empty
// filter as "no restriction". MS-Phase-2a · MS2a.2c.
func (s *CollectionAppService) IsProductInCollections(ctx context.Context, collectionIDs []string, slug string) (bool, error) {
	if len(collectionIDs) == 0 {
		return false, nil
	}
	return s.store.IsProductInCollections(ctx, collectionIDs, slug)
}

func (s *CollectionAppService) RemoveProductFromAllCollections(ctx context.Context, slug string) error {
	if err := s.store.RemoveProductFromAllCollections(ctx, slug); err != nil {
		return err
	}

	if s.bus != nil {
		s.bus.Emit(events.CollectionProductsChanged{
			Action: events.CollectionProductActionBulkRemove,
			Slugs:  []string{slug},
		})
	}
	s.pushCollectionsToNetDB()
	return nil
}

func normalizePageParams(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultCollectionPageSize
	}
	if pageSize > maxCollectionPageSize {
		pageSize = maxCollectionPageSize
	}
	return page, pageSize
}

func (s *CollectionAppService) pushCollectionsToNetDB() {
	if s.bus != nil {
		s.bus.Emit(&events.CollectionsChanged{})
	}
}
