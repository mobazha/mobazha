package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ShippingStore abstracts shipping profile and location persistence for both standalone
// and SaaS modes. Implementations handle tenant scoping internally (database.Tx injects
// tenantID on writes, read queries are pre-scoped). Callers need not pass tenantID explicitly.
type ShippingStore interface {
	// --- Profile CRUD ---

	CreateProfile(ctx context.Context, profile *models.ShippingProfileEntity) error
	GetProfile(ctx context.Context, id string) (*models.ShippingProfileEntity, error)
	GetDefaultProfile(ctx context.Context) (*models.ShippingProfileEntity, error)
	ListProfiles(ctx context.Context) ([]*models.ShippingProfileEntity, error)
	// UpdateProfile performs a full update. The caller is responsible for incrementing Version.
	UpdateProfile(ctx context.Context, profile *models.ShippingProfileEntity) error
	DeleteProfile(ctx context.Context, id string) error
	// SetDefaultProfile clears is_default on all profiles, then sets it on the given id.
	SetDefaultProfile(ctx context.Context, id string) error

	// --- Location CRUD ---

	CreateLocation(ctx context.Context, location *models.ShippingLocationEntity) error
	GetLocation(ctx context.Context, id string) (*models.ShippingLocationEntity, error)
	ListLocations(ctx context.Context) ([]*models.ShippingLocationEntity, error)
	UpdateLocation(ctx context.Context, location *models.ShippingLocationEntity) error
	DeleteLocation(ctx context.Context, id string) error

	// --- Listing-Profile references ---

	// UpsertListingRef creates or updates the listing ↔ profile association.
	UpsertListingRef(ctx context.Context, ref *models.ListingShippingRef) error
	// GetListingRef returns the ref for a listing slug, or nil if none exists.
	GetListingRef(ctx context.Context, listingSlug string) (*models.ListingShippingRef, error)
	// DeleteListingRef removes the ref for a listing slug (e.g. when listing changes to DIGITAL).
	DeleteListingRef(ctx context.Context, listingSlug string) error
	// ListRefsByProfile returns paginated refs associated with a profile.
	ListRefsByProfile(ctx context.Context, profileID string, page, pageSize int) ([]*models.ListingShippingRef, int, error)
	// MigrateRefs moves all refs from one profile to another (used during profile deletion
	// with migration). All migrated refs are marked stale. Returns count of migrated refs.
	MigrateRefs(ctx context.Context, fromProfileID, toProfileID string) (int, error)
	// MarkProfileStale sets is_stale=true on all refs associated with the given profile.
	MarkProfileStale(ctx context.Context, profileID string) error
	// ListStaleRefs returns paginated refs where is_stale=true.
	ListStaleRefs(ctx context.Context, page, pageSize int) ([]*models.ListingShippingRef, int, error)
	// CountListingsByProfile returns the number of listings associated with a profile.
	CountListingsByProfile(ctx context.Context, profileID string) (int, error)
}

// ListingPublisher is a narrow interface that ShippingAppService uses to trigger
// listing re-publication without depending on the full ListingAppService.
// This breaks the circular dependency: Shipping → ListingPublisher ← Listing.
type ListingPublisher interface {
	RepublishListing(ctx context.Context, slug string) error
}

// ShippingService is the business-level interface for shipping profile operations.
// Implemented by ShippingAppService in internal/core/.
type ShippingService interface {
	// Profile CRUD
	CreateProfile(ctx context.Context, profile *models.ShippingProfileEntity) error
	GetProfile(ctx context.Context, id string) (*models.ShippingProfileEntity, error)
	ListProfiles(ctx context.Context) ([]*models.ShippingProfileEntity, error)
	UpdateProfile(ctx context.Context, profile *models.ShippingProfileEntity, clientVersion int) error
	PatchProfile(ctx context.Context, id string, patch *models.ShippingProfilePatch) error
	DeleteProfile(ctx context.Context, id string, migrateTo string) error
	ResolveProfileForListing(ctx context.Context, profileID string) (*models.ShippingProfileEntity, error)

	// Location CRUD
	CreateLocation(ctx context.Context, loc *models.ShippingLocationEntity) error
	GetLocation(ctx context.Context, id string) (*models.ShippingLocationEntity, error)
	ListLocations(ctx context.Context) ([]*models.ShippingLocationEntity, error)
	UpdateLocation(ctx context.Context, loc *models.ShippingLocationEntity) error
	DeleteLocation(ctx context.Context, id string) error

	// Listing-Profile ref management
	ListRefsByProfile(ctx context.Context, profileID string, page, pageSize int) ([]*models.ListingShippingRef, int, error)
	ListStaleListings(ctx context.Context, page, pageSize int) ([]*models.ListingShippingRef, int, error)
	RefreshStaleListings(ctx context.Context) (int, []error)
}

// ShippingProvider exposes the per-node shipping subsystem.
// Handlers obtain this via type assertion on NodeService.
type ShippingProvider interface {
	Shipping() ShippingService
}
