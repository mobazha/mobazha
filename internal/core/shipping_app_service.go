package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	maxShippingProfilesPerTenant  = 100
	maxShippingLocationsPerTenant = 50
	staleRefreshBatchSize         = 20
)

var (
	ErrShippingProfileNameRequired = errors.New("profile name is required")
	ErrShippingLocationNameRequired = errors.New("location name is required")
	ErrShippingProfileHasListings   = errors.New("profile has associated listings; specify migrateTo or remove listings first")
	ErrShippingVersionConflict      = errors.New("version conflict: profile was modified by another request")
	ErrShippingMigrateToSelf        = errors.New("cannot migrate refs to the same profile")
	ErrShippingProfileLimitReached  = errors.New("maximum shipping profiles reached")
	ErrShippingLocationLimitReached = errors.New("maximum shipping locations reached")
)

var _ contracts.ShippingService = (*ShippingAppService)(nil)

type ShippingAppService struct {
	store     contracts.ShippingStore
	publisher contracts.ListingPublisher
	bus       events.Bus
}

func NewShippingAppService(store contracts.ShippingStore, publisher contracts.ListingPublisher) *ShippingAppService {
	return &ShippingAppService{
		store:     store,
		publisher: publisher,
	}
}

// Store returns the underlying ShippingStore for cross-service injection
// (e.g., ListingAppService needs the store to manage listing-profile refs).
func (s *ShippingAppService) Store() contracts.ShippingStore {
	return s.store
}

// SetEventBus injects the event bus for async notifications.
// Called after node EventBus is available (separate from constructor to avoid init-order issues).
func (s *ShippingAppService) SetEventBus(bus events.Bus) {
	s.bus = bus
}

func (s *ShippingAppService) emitEvent(evt interface{}) {
	if s.bus != nil {
		s.bus.Emit(evt)
	}
}

// --- Profile CRUD ---

func (s *ShippingAppService) CreateProfile(ctx context.Context, profile *models.ShippingProfileEntity) error {
	if strings.TrimSpace(profile.Name) == "" {
		return ErrShippingProfileNameRequired
	}

	existing, err := s.store.ListProfiles(ctx)
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}
	if len(existing) >= maxShippingProfilesPerTenant {
		return ErrShippingProfileLimitReached
	}

	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	profile.Version = 1

	if len(existing) == 0 {
		profile.IsDefault = true
	}

	if err := s.store.CreateProfile(ctx, profile); err != nil {
		return err
	}

	if profile.IsDefault && len(existing) > 0 {
		return s.store.SetDefaultProfile(ctx, profile.ID)
	}
	return nil
}

func (s *ShippingAppService) GetProfile(ctx context.Context, id string) (*models.ShippingProfileEntity, error) {
	profile, err := s.store.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	count, err := s.store.CountListingsByProfile(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count listings: %w", err)
	}
	profile.ListingCount = count
	return profile, nil
}

func (s *ShippingAppService) ListProfiles(ctx context.Context) ([]*models.ShippingProfileEntity, error) {
	profiles, err := s.store.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		count, err := s.store.CountListingsByProfile(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("count listings for %s: %w", p.ID, err)
		}
		p.ListingCount = count
	}
	return profiles, nil
}

// UpdateProfile performs a full update with optimistic locking.
// clientVersion must match the current version; on success, version is incremented.
// If locationGroups changed, associated listings are marked stale.
func (s *ShippingAppService) UpdateProfile(ctx context.Context, profile *models.ShippingProfileEntity, clientVersion int) error {
	if strings.TrimSpace(profile.Name) == "" {
		return ErrShippingProfileNameRequired
	}

	existing, err := s.store.GetProfile(ctx, profile.ID)
	if err != nil {
		return err
	}
	if existing.Version != clientVersion {
		return ErrShippingVersionConflict
	}

	locationGroupsChanged := existing.LocationGroupsJSON != profile.LocationGroupsJSON
	profile.Version = existing.Version + 1

	if profile.IsDefault && !existing.IsDefault {
		if err := s.store.SetDefaultProfile(ctx, profile.ID); err != nil {
			return fmt.Errorf("set default: %w", err)
		}
	}

	if err := s.store.UpdateProfile(ctx, profile); err != nil {
		return err
	}

	if locationGroupsChanged {
		if err := s.store.MarkProfileStale(ctx, profile.ID); err != nil {
			return err
		}
	}

	s.emitEvent(&events.ShippingProfileUpdated{
		ProfileID:           profile.ID,
		LocationGroupsDirty: locationGroupsChanged,
	})
	return nil
}

// PatchProfile applies partial updates with optimistic locking.
// Only non-nil fields are applied. Stale marking only triggers when locationGroups changes.
func (s *ShippingAppService) PatchProfile(ctx context.Context, id string, patch *models.ShippingProfilePatch) error {
	existing, err := s.store.GetProfile(ctx, id)
	if err != nil {
		return err
	}
	if existing.Version != patch.Version {
		return ErrShippingVersionConflict
	}

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return ErrShippingProfileNameRequired
		}
		existing.Name = name
	}

	locationGroupsChanged := false
	if patch.LocationGroups != nil {
		locationGroupsChanged = existing.LocationGroupsJSON != *patch.LocationGroups
		existing.LocationGroupsJSON = *patch.LocationGroups
	}

	if patch.IsDefault != nil && *patch.IsDefault && !existing.IsDefault {
		if err := s.store.SetDefaultProfile(ctx, id); err != nil {
			return fmt.Errorf("set default: %w", err)
		}
		existing.IsDefault = true
	}

	existing.Version++
	if err := s.store.UpdateProfile(ctx, existing); err != nil {
		return err
	}

	if locationGroupsChanged {
		if err := s.store.MarkProfileStale(ctx, id); err != nil {
			return err
		}
	}

	s.emitEvent(&events.ShippingProfileUpdated{
		ProfileID:           id,
		LocationGroupsDirty: locationGroupsChanged,
	})
	return nil
}

// DeleteProfile removes a profile. If the profile has associated listings:
//   - migrateTo == "": returns ErrShippingProfileHasListings with listing count info
//   - migrateTo != "": migrates refs to the target profile, marks them stale, then deletes
func (s *ShippingAppService) DeleteProfile(ctx context.Context, id string, migrateTo string) error {
	if _, err := s.store.GetProfile(ctx, id); err != nil {
		return err
	}

	count, err := s.store.CountListingsByProfile(ctx, id)
	if err != nil {
		return fmt.Errorf("count listings: %w", err)
	}

	if count > 0 {
		if migrateTo == "" {
			return fmt.Errorf("%w (count: %d)", ErrShippingProfileHasListings, count)
		}
		if migrateTo == id {
			return ErrShippingMigrateToSelf
		}
		if _, err := s.store.GetProfile(ctx, migrateTo); err != nil {
			return fmt.Errorf("target profile: %w", err)
		}

		if _, err := s.store.MigrateRefs(ctx, id, migrateTo); err != nil {
			return fmt.Errorf("migrate refs: %w", err)
		}
		if err := s.store.MarkProfileStale(ctx, migrateTo); err != nil {
			return fmt.Errorf("mark migrated refs stale: %w", err)
		}
	}

	if err := s.store.DeleteProfile(ctx, id); err != nil {
		return err
	}

	s.emitEvent(&events.ShippingProfileDeleted{
		ProfileID:  id,
		MigratedTo: migrateTo,
	})
	return nil
}

// ResolveProfileForListing returns the profile for a given ID, or the default profile if ID is empty.
func (s *ShippingAppService) ResolveProfileForListing(ctx context.Context, profileID string) (*models.ShippingProfileEntity, error) {
	if profileID != "" {
		return s.store.GetProfile(ctx, profileID)
	}
	return s.store.GetDefaultProfile(ctx)
}

// RefreshStaleListings finds stale refs and republishes them via ListingPublisher.
// Returns the number of successfully refreshed listings and any errors encountered.
func (s *ShippingAppService) RefreshStaleListings(ctx context.Context) (int, []error) {
	refs, _, err := s.store.ListStaleRefs(ctx, 1, staleRefreshBatchSize)
	if err != nil {
		return 0, []error{fmt.Errorf("list stale refs: %w", err)}
	}

	if s.publisher == nil {
		return 0, nil
	}

	var refreshed int
	var errs []error
	for _, ref := range refs {
		if err := s.publisher.RepublishListing(ctx, ref.ListingSlug); err != nil {
			errs = append(errs, fmt.Errorf("republish %s: %w", ref.ListingSlug, err))
			continue
		}
		refreshed++
	}

	s.emitEvent(&events.ShippingSnapshotsRefreshed{
		Refreshed: refreshed,
		Errors:    len(errs),
	})
	return refreshed, errs
}

// ListStaleListings returns paginated stale refs for admin UI.
func (s *ShippingAppService) ListStaleListings(ctx context.Context, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	return s.store.ListStaleRefs(ctx, page, pageSize)
}

// ListRefsByProfile returns paginated refs for a profile.
func (s *ShippingAppService) ListRefsByProfile(ctx context.Context, profileID string, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	return s.store.ListRefsByProfile(ctx, profileID, page, pageSize)
}

// --- Location CRUD ---

func (s *ShippingAppService) CreateLocation(ctx context.Context, loc *models.ShippingLocationEntity) error {
	if strings.TrimSpace(loc.Name) == "" {
		return ErrShippingLocationNameRequired
	}

	existing, err := s.store.ListLocations(ctx)
	if err != nil {
		return fmt.Errorf("list locations: %w", err)
	}
	if len(existing) >= maxShippingLocationsPerTenant {
		return ErrShippingLocationLimitReached
	}

	if loc.ID == "" {
		loc.ID = uuid.New().String()
	}
	if len(existing) == 0 {
		loc.IsDefault = true
	}
	return s.store.CreateLocation(ctx, loc)
}

func (s *ShippingAppService) GetLocation(ctx context.Context, id string) (*models.ShippingLocationEntity, error) {
	return s.store.GetLocation(ctx, id)
}

func (s *ShippingAppService) ListLocations(ctx context.Context) ([]*models.ShippingLocationEntity, error) {
	return s.store.ListLocations(ctx)
}

func (s *ShippingAppService) UpdateLocation(ctx context.Context, loc *models.ShippingLocationEntity) error {
	if strings.TrimSpace(loc.Name) == "" {
		return ErrShippingLocationNameRequired
	}
	return s.store.UpdateLocation(ctx, loc)
}

func (s *ShippingAppService) DeleteLocation(ctx context.Context, id string) error {
	return s.store.DeleteLocation(ctx, id)
}
