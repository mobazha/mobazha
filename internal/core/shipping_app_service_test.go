package core

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock ShippingStore ---

type mockShippingStore struct {
	profiles  map[string]*models.ShippingProfileEntity
	locations map[string]*models.ShippingLocationEntity
	refs      map[string]*models.ListingShippingRef // keyed by listing_slug
}

func newMockShippingStore() *mockShippingStore {
	return &mockShippingStore{
		profiles:  make(map[string]*models.ShippingProfileEntity),
		locations: make(map[string]*models.ShippingLocationEntity),
		refs:      make(map[string]*models.ListingShippingRef),
	}
}

func (m *mockShippingStore) CreateProfile(_ context.Context, p *models.ShippingProfileEntity) error {
	cp := *p
	m.profiles[p.ID] = &cp
	return nil
}

func (m *mockShippingStore) GetProfile(_ context.Context, id string) (*models.ShippingProfileEntity, error) {
	p, ok := m.profiles[id]
	if !ok {
		return nil, errors.New("shipping profile not found")
	}
	cp := *p
	return &cp, nil
}

func (m *mockShippingStore) GetDefaultProfile(_ context.Context) (*models.ShippingProfileEntity, error) {
	for _, p := range m.profiles {
		if p.IsDefault {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errors.New("shipping profile not found")
}

func (m *mockShippingStore) ListProfiles(_ context.Context) ([]*models.ShippingProfileEntity, error) {
	var result []*models.ShippingProfileEntity
	for _, p := range m.profiles {
		cp := *p
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockShippingStore) UpdateProfile(_ context.Context, p *models.ShippingProfileEntity) error {
	if _, ok := m.profiles[p.ID]; !ok {
		return errors.New("shipping profile not found")
	}
	cp := *p
	m.profiles[p.ID] = &cp
	return nil
}

func (m *mockShippingStore) DeleteProfile(_ context.Context, id string) error {
	delete(m.profiles, id)
	return nil
}

func (m *mockShippingStore) SetDefaultProfile(_ context.Context, id string) error {
	found := false
	for k, p := range m.profiles {
		p.IsDefault = (k == id)
		if k == id {
			found = true
		}
	}
	if !found {
		return errors.New("shipping profile not found")
	}
	return nil
}

func (m *mockShippingStore) CreateLocation(_ context.Context, loc *models.ShippingLocationEntity) error {
	cp := *loc
	m.locations[loc.ID] = &cp
	return nil
}

func (m *mockShippingStore) GetLocation(_ context.Context, id string) (*models.ShippingLocationEntity, error) {
	loc, ok := m.locations[id]
	if !ok {
		return nil, errors.New("shipping location not found")
	}
	cp := *loc
	return &cp, nil
}

func (m *mockShippingStore) ListLocations(_ context.Context) ([]*models.ShippingLocationEntity, error) {
	var result []*models.ShippingLocationEntity
	for _, loc := range m.locations {
		cp := *loc
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockShippingStore) UpdateLocation(_ context.Context, loc *models.ShippingLocationEntity) error {
	if _, ok := m.locations[loc.ID]; !ok {
		return errors.New("shipping location not found")
	}
	cp := *loc
	m.locations[loc.ID] = &cp
	return nil
}

func (m *mockShippingStore) DeleteLocation(_ context.Context, id string) error {
	delete(m.locations, id)
	return nil
}

func (m *mockShippingStore) UpsertListingRef(_ context.Context, ref *models.ListingShippingRef) error {
	cp := *ref
	m.refs[ref.ListingSlug] = &cp
	return nil
}

func (m *mockShippingStore) GetListingRef(_ context.Context, slug string) (*models.ListingShippingRef, error) {
	ref, ok := m.refs[slug]
	if !ok {
		return nil, nil
	}
	cp := *ref
	return &cp, nil
}

func (m *mockShippingStore) DeleteListingRef(_ context.Context, slug string) error {
	delete(m.refs, slug)
	return nil
}

func (m *mockShippingStore) ListRefsByProfile(_ context.Context, profileID string, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	var result []*models.ListingShippingRef
	for _, ref := range m.refs {
		if ref.ShippingProfileID == profileID {
			cp := *ref
			result = append(result, &cp)
		}
	}
	total := len(result)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return result[start:end], total, nil
}

func (m *mockShippingStore) MigrateRefs(_ context.Context, from, to string) (int, error) {
	count := 0
	for _, ref := range m.refs {
		if ref.ShippingProfileID == from {
			ref.ShippingProfileID = to
			ref.IsStale = true
			count++
		}
	}
	return count, nil
}

func (m *mockShippingStore) MarkProfileStale(_ context.Context, profileID string) error {
	for _, ref := range m.refs {
		if ref.ShippingProfileID == profileID {
			ref.IsStale = true
		}
	}
	return nil
}

func (m *mockShippingStore) ListStaleRefs(_ context.Context, page, pageSize int) ([]*models.ListingShippingRef, int, error) {
	var result []*models.ListingShippingRef
	for _, ref := range m.refs {
		if ref.IsStale {
			cp := *ref
			result = append(result, &cp)
		}
	}
	total := len(result)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return result[start:end], total, nil
}

func (m *mockShippingStore) CountListingsByProfile(_ context.Context, profileID string) (int, error) {
	count := 0
	for _, ref := range m.refs {
		if ref.ShippingProfileID == profileID {
			count++
		}
	}
	return count, nil
}

// --- Mock ListingPublisher ---

type mockListingPublisher struct {
	published []string
	failSlugs map[string]bool
}

func newMockPublisher() *mockListingPublisher {
	return &mockListingPublisher{failSlugs: make(map[string]bool)}
}

func (p *mockListingPublisher) RepublishListing(_ context.Context, slug string) error {
	if p.failSlugs[slug] {
		return errors.New("publish failed")
	}
	p.published = append(p.published, slug)
	return nil
}

// --- Helper ---

func newTestShippingService() (*ShippingAppService, *mockShippingStore, *mockListingPublisher) {
	store := newMockShippingStore()
	pub := newMockPublisher()
	svc := NewShippingAppService(store, pub)
	return svc, store, pub
}

// ========== Profile Tests ==========

func TestShippingAppService_CreateProfile(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	p := &models.ShippingProfileEntity{Name: "Standard Shipping", LocationGroupsJSON: "[]"}
	require.NoError(t, svc.CreateProfile(ctx, p))

	assert.NotEmpty(t, p.ID)
	assert.Equal(t, 1, p.Version)
	assert.True(t, p.IsDefault, "first profile should be default")

	got := store.profiles[p.ID]
	assert.Equal(t, "Standard Shipping", got.Name)
}

func TestShippingAppService_CreateProfile_EmptyName(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	err := svc.CreateProfile(ctx, &models.ShippingProfileEntity{Name: "  "})
	assert.ErrorIs(t, err, ErrShippingProfileNameRequired)
}

func TestShippingAppService_CreateProfile_SecondNotDefault(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{Name: "First", LocationGroupsJSON: "[]"}))

	p2 := &models.ShippingProfileEntity{Name: "Second", LocationGroupsJSON: "[]"}
	require.NoError(t, svc.CreateProfile(ctx, p2))
	assert.False(t, p2.IsDefault, "second profile should not be default unless requested")
}

func TestShippingAppService_GetProfile_WithListingCount(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: "[]"}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1"}
	store.refs["prod-2"] = &models.ListingShippingRef{ListingSlug: "prod-2", ShippingProfileID: "p1"}

	got, err := svc.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, 2, got.ListingCount)
}

func TestShippingAppService_ListProfiles_WithListingCounts(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "A", LocationGroupsJSON: "[]"}))
	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p2", Name: "B", LocationGroupsJSON: "[]"}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1"}

	profiles, err := svc.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, profiles, 2)

	counts := map[string]int{}
	for _, p := range profiles {
		counts[p.ID] = p.ListingCount
	}
	assert.Equal(t, 1, counts["p1"])
	assert.Equal(t, 0, counts["p2"])
}

// ========== Optimistic Locking Tests ==========

func TestShippingAppService_UpdateProfile_VersionMatch(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Original", LocationGroupsJSON: "[]"}))

	p := store.profiles["p1"]
	p.Name = "Updated"
	require.NoError(t, svc.UpdateProfile(ctx, p, 1))

	got := store.profiles["p1"]
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, 2, got.Version)
}

func TestShippingAppService_UpdateProfile_VersionMismatch(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Original", LocationGroupsJSON: "[]"}))

	p := &models.ShippingProfileEntity{ID: "p1", Name: "Conflict"}
	err := svc.UpdateProfile(ctx, p, 99)
	assert.ErrorIs(t, err, ErrShippingVersionConflict)
}

func TestShippingAppService_UpdateProfile_LocationGroupsChange_MarkStale(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: `[{"id":"lg1"}]`}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: false}
	store.refs["prod-2"] = &models.ListingShippingRef{ListingSlug: "prod-2", ShippingProfileID: "p1", IsStale: false}

	p := &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: `[{"id":"lg2"}]`}
	require.NoError(t, svc.UpdateProfile(ctx, p, 1))

	assert.True(t, store.refs["prod-1"].IsStale)
	assert.True(t, store.refs["prod-2"].IsStale)
}

func TestShippingAppService_UpdateProfile_NameOnlyChange_NoStale(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Old", LocationGroupsJSON: `[{"id":"lg1"}]`}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: false}

	p := &models.ShippingProfileEntity{ID: "p1", Name: "New Name", LocationGroupsJSON: `[{"id":"lg1"}]`}
	require.NoError(t, svc.UpdateProfile(ctx, p, 1))

	assert.False(t, store.refs["prod-1"].IsStale, "name-only change should not mark stale")
}

// ========== Patch Tests ==========

func TestShippingAppService_PatchProfile_NameOnly_NoStale(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Old", LocationGroupsJSON: `[{"id":"lg1"}]`}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: false}

	newName := "Renamed"
	require.NoError(t, svc.PatchProfile(ctx, "p1", &models.ShippingProfilePatch{Name: &newName, Version: 1}))

	assert.Equal(t, "Renamed", store.profiles["p1"].Name)
	assert.Equal(t, 2, store.profiles["p1"].Version)
	assert.False(t, store.refs["prod-1"].IsStale, "name-only patch should not mark stale")
}

func TestShippingAppService_PatchProfile_LocationGroupsChange_MarkStale(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: `[{"id":"old"}]`}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: false}

	newGroups := `[{"id":"new"}]`
	require.NoError(t, svc.PatchProfile(ctx, "p1", &models.ShippingProfilePatch{LocationGroups: &newGroups, Version: 1}))

	assert.Equal(t, `[{"id":"new"}]`, store.profiles["p1"].LocationGroupsJSON)
	assert.True(t, store.refs["prod-1"].IsStale)
}

func TestShippingAppService_PatchProfile_VersionMismatch(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: "[]"}))

	newName := "Fail"
	err := svc.PatchProfile(ctx, "p1", &models.ShippingProfilePatch{Name: &newName, Version: 99})
	assert.ErrorIs(t, err, ErrShippingVersionConflict)
}

// ========== Delete Tests ==========

func TestShippingAppService_DeleteProfile_NoListings(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: "[]"}))
	require.NoError(t, svc.DeleteProfile(ctx, "p1", ""))

	_, ok := store.profiles["p1"]
	assert.False(t, ok)
}

func TestShippingAppService_DeleteProfile_HasListings_NoMigrateTo(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: "[]"}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1"}

	err := svc.DeleteProfile(ctx, "p1", "")
	assert.ErrorIs(t, err, ErrShippingProfileHasListings)
	assert.Contains(t, err.Error(), "count: 1")
}

func TestShippingAppService_DeleteProfile_HasListings_WithMigrateTo(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Source", LocationGroupsJSON: "[]"}))
	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p2", Name: "Target", LocationGroupsJSON: "[]"}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: false}
	store.refs["prod-2"] = &models.ListingShippingRef{ListingSlug: "prod-2", ShippingProfileID: "p1", IsStale: false}

	require.NoError(t, svc.DeleteProfile(ctx, "p1", "p2"))

	_, ok := store.profiles["p1"]
	assert.False(t, ok, "source profile should be deleted")

	assert.Equal(t, "p2", store.refs["prod-1"].ShippingProfileID)
	assert.Equal(t, "p2", store.refs["prod-2"].ShippingProfileID)
	assert.True(t, store.refs["prod-1"].IsStale, "migrated refs should be stale")
}

func TestShippingAppService_DeleteProfile_MigrateToSelf(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Test", LocationGroupsJSON: "[]"}))
	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1"}

	err := svc.DeleteProfile(ctx, "p1", "p1")
	assert.ErrorIs(t, err, ErrShippingMigrateToSelf)
}

// ========== Resolve Profile Tests ==========

func TestShippingAppService_ResolveProfile_ByID(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Specific", LocationGroupsJSON: "[]"}))
	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p2", Name: "Default", LocationGroupsJSON: "[]", IsDefault: true}))

	got, err := svc.ResolveProfileForListing(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ID)
}

func TestShippingAppService_ResolveProfile_Default(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateProfile(ctx, &models.ShippingProfileEntity{ID: "p1", Name: "Default", LocationGroupsJSON: "[]"}))

	got, err := svc.ResolveProfileForListing(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ID)
}

// ========== Refresh Stale Listings Tests ==========

func TestShippingAppService_RefreshStaleListings(t *testing.T) {
	svc, store, pub := newTestShippingService()
	ctx := context.Background()

	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: true}
	store.refs["prod-2"] = &models.ListingShippingRef{ListingSlug: "prod-2", ShippingProfileID: "p1", IsStale: true}
	store.refs["prod-3"] = &models.ListingShippingRef{ListingSlug: "prod-3", ShippingProfileID: "p1", IsStale: false}

	refreshed, errs := svc.RefreshStaleListings(ctx)
	assert.Empty(t, errs)
	assert.Equal(t, 2, refreshed)
	assert.Len(t, pub.published, 2)
}

func TestShippingAppService_RefreshStaleListings_PartialFailure(t *testing.T) {
	svc, store, pub := newTestShippingService()
	ctx := context.Background()

	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: true}
	store.refs["prod-2"] = &models.ListingShippingRef{ListingSlug: "prod-2", ShippingProfileID: "p1", IsStale: true}
	pub.failSlugs["prod-2"] = true

	refreshed, errs := svc.RefreshStaleListings(ctx)
	assert.Len(t, errs, 1)
	assert.Equal(t, 1, refreshed)
	assert.Contains(t, errs[0].Error(), "prod-2")
}

func TestShippingAppService_RefreshStaleListings_NilPublisher(t *testing.T) {
	store := newMockShippingStore()
	svc := NewShippingAppService(store, nil)
	ctx := context.Background()

	store.refs["prod-1"] = &models.ListingShippingRef{ListingSlug: "prod-1", ShippingProfileID: "p1", IsStale: true}

	refreshed, errs := svc.RefreshStaleListings(ctx)
	assert.Empty(t, errs)
	assert.Equal(t, 0, refreshed)
}

// ========== Location Tests ==========

func TestShippingAppService_CreateLocation(t *testing.T) {
	svc, store, _ := newTestShippingService()
	ctx := context.Background()

	loc := &models.ShippingLocationEntity{Name: "Beijing Warehouse", Address: "Haidian"}
	require.NoError(t, svc.CreateLocation(ctx, loc))

	assert.NotEmpty(t, loc.ID)
	assert.True(t, loc.IsDefault, "first location should be default")
	assert.NotNil(t, store.locations[loc.ID])
}

func TestShippingAppService_CreateLocation_EmptyName(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	err := svc.CreateLocation(ctx, &models.ShippingLocationEntity{Name: ""})
	assert.ErrorIs(t, err, ErrShippingLocationNameRequired)
}

func TestShippingAppService_UpdateLocation_EmptyName(t *testing.T) {
	svc, _, _ := newTestShippingService()
	ctx := context.Background()

	require.NoError(t, svc.CreateLocation(ctx, &models.ShippingLocationEntity{ID: "loc1", Name: "Test"}))

	err := svc.UpdateLocation(ctx, &models.ShippingLocationEntity{ID: "loc1", Name: "  "})
	assert.ErrorIs(t, err, ErrShippingLocationNameRequired)
}
