package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// StorePolicyStore abstracts the per-store commerce policy persistence.
type StorePolicyStore interface {
	GetPolicy(ctx context.Context) (*models.StorePolicy, error)
	ReplaceModerators(ctx context.Context, expectedRevision *uint64, moderators []models.StoreModerator) (*models.StorePolicy, error)
}

// StorePolicyService is the business-level interface for store policy operations.
type StorePolicyService interface {
	GetPolicy(ctx context.Context) (*models.StorePolicy, error)
	GetPublishedPolicy(ctx context.Context) (*models.StorePolicyPublic, error)
	ReplaceModerators(ctx context.Context, expectedRevision *uint64, moderators []models.StorePolicyModeratorInput) (*models.StorePolicy, error)
	UpsertModerator(ctx context.Context, expectedRevision *uint64, moderator models.StorePolicyModeratorInput) (*models.StorePolicy, error)
	RemoveModerator(ctx context.Context, expectedRevision *uint64, peerID string) (*models.StorePolicy, error)
}

// StorePolicyProvider exposes the per-node StorePolicy subsystem.
// Handlers obtain this via type assertion on NodeService.
type StorePolicyProvider interface {
	StorePolicy() StorePolicyService
}
