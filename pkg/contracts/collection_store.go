package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// CollectionStore abstracts Collection persistence for both standalone and SaaS modes.
// Implementations handle tenant scoping internally via database.Tx.
type CollectionStore interface {
	CreateCollection(ctx context.Context, c *models.Collection) error
	GetCollection(ctx context.Context, id string) (*models.Collection, error)
	ListCollections(ctx context.Context, page, pageSize int, publishedOnly bool) ([]*models.Collection, int64, error)
	UpdateCollection(ctx context.Context, c *models.Collection) error
	DeleteCollection(ctx context.Context, id string) error

	AddProducts(ctx context.Context, collectionID string, slugs []string) error
	RemoveProduct(ctx context.Context, collectionID, slug string) error
	ReorderProducts(ctx context.Context, collectionID string, orderedSlugs []string) error
	IsProductInCollections(ctx context.Context, collectionIDs []string, slug string) (bool, error)

	RemoveProductFromAllCollections(ctx context.Context, slug string) error

	CountCollections(ctx context.Context) (int64, error)
	CountCollectionProducts(ctx context.Context, collectionID string) (int64, error)
}

// CollectionService is the business-level interface for collection operations.
// Implemented by CollectionAppService in internal/core/.
type CollectionService interface {
	CreateCollection(ctx context.Context, c *models.Collection) error
	GetCollection(ctx context.Context, id string) (*models.Collection, error)
	ListCollections(ctx context.Context, page, pageSize int, publishedOnly bool) ([]*models.Collection, int64, error)
	UpdateCollection(ctx context.Context, c *models.Collection) error
	DeleteCollection(ctx context.Context, id string) error

	AddProducts(ctx context.Context, collectionID string, slugs []string) error
	RemoveProduct(ctx context.Context, collectionID, slug string) error
	ReorderProducts(ctx context.Context, collectionID string, orderedSlugs []string) error
}
