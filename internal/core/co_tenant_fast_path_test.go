//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPublicData implements pkgdb.PublicData for testing the co-tenant fast path.
type mockPublicData struct {
	listingIndex models.ListingIndex
	listings     map[string]*pb.SignedListing
	profile      *models.Profile
	ratingIndex  models.RatingIndex
	followers    models.Followers
	following    models.Following
	err          error
}

var _ pkgdb.PublicData = (*mockPublicData)(nil)

func (m *mockPublicData) GetProfile() (*models.Profile, error)          { return m.profile, m.err }
func (m *mockPublicData) SetProfile(*models.Profile) error              { return nil }
func (m *mockPublicData) GetFollowers() (models.Followers, error)       { return m.followers, m.err }
func (m *mockPublicData) SetFollowers(models.Followers) error           { return nil }
func (m *mockPublicData) GetFollowing() (models.Following, error)       { return m.following, m.err }
func (m *mockPublicData) SetFollowing(models.Following) error           { return nil }
func (m *mockPublicData) GetListingIndex() (models.ListingIndex, error) { return m.listingIndex, m.err }
func (m *mockPublicData) SetListingIndex(models.ListingIndex) error     { return nil }
func (m *mockPublicData) GetRatingIndex() (models.RatingIndex, error)   { return m.ratingIndex, m.err }
func (m *mockPublicData) SetRatingIndex(models.RatingIndex) error       { return nil }
func (m *mockPublicData) SetRating(*pb.Rating) error                    { return nil }
func (m *mockPublicData) GetPostIndex() ([]models.PostData, error)      { return nil, nil }
func (m *mockPublicData) SetPostIndex([]models.PostData) error          { return nil }
func (m *mockPublicData) AddPost(*postsPb.SignedPost) error             { return nil }
func (m *mockPublicData) DeletePost(string) error                       { return nil }
func (m *mockPublicData) PostExist(string) bool                         { return false }
func (m *mockPublicData) GetPost(string) (*postsPb.SignedPost, error)   { return nil, nil }
func (m *mockPublicData) SetImage(models.Image) error                   { return nil }
func (m *mockPublicData) GetImageByName(models.ImageSize, string) ([]byte, error) {
	return nil, nil
}
func (m *mockPublicData) GetMediaByCID(string) ([]byte, string, error)               { return nil, "", nil }
func (m *mockPublicData) IndexMediaCID(string, string, string, string, string) error { return nil }
func (m *mockPublicData) SetUploadedFile(models.UploadedFile) error                  { return nil }
func (m *mockPublicData) SetIntroVideo(models.IntroVideo) error                      { return nil }
func (m *mockPublicData) SetListing(*pb.SignedListing) error                         { return nil }
func (m *mockPublicData) GetEncryptedListing(string) ([]byte, error)                 { return nil, nil }
func (m *mockPublicData) SetEncryptedListing(string, []byte) error                   { return nil }
func (m *mockPublicData) DeleteListing(string) error                                 { return nil }

func (m *mockPublicData) GetListing(slug string) (*pb.SignedListing, error) {
	if m.err != nil {
		return nil, m.err
	}
	if sl, ok := m.listings[slug]; ok {
		return sl, nil
	}
	return nil, fmt.Errorf("listing not found: %s", slug)
}

func coTenantHit(pd pkgdb.PublicData) contracts.CoTenantPublicDataFn {
	return func(_ peer.ID) (pkgdb.PublicData, error) {
		return pd, nil
	}
}

func coTenantMiss() contracts.CoTenantPublicDataFn {
	return func(_ peer.ID) (pkgdb.PublicData, error) {
		return nil, fmt.Errorf("not a co-tenant")
	}
}

// testLocalNodePeerID is a different peer ID from testVendorPeerID, used as the
// local node identity so co-tenant lookups for testVendorPeerID don't
// short-circuit to the local DB.
const testLocalNodePeerID = "QmT5NvUtoM5nWFfrQdVrFtvGfKFmG7AHE8P34isapyhCxX"

// --- ListingAppService tests ---

func TestListingGetListings_CoTenantHit(t *testing.T) {
	wantIndex := models.ListingIndex{{Slug: "test-product", Title: "Test"}}
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		NodeID:             mustPeerID(t, testLocalNodePeerID),
		CoTenantPublicData: coTenantHit(&mockPublicData{listingIndex: wantIndex}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetListings(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, wantIndex, got)
}

func TestListingGetListings_NilCoTenant_NoFastPath(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		CoTenantPublicData: nil,
	})
	require.Nil(t, svc.coTenantPublicData, "standalone mode: no co-tenant resolver")
}

func TestListingGetListings_CoTenantMissReturnsEmptyIndex(t *testing.T) {
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		NodeID:             mustPeerID(t, testLocalNodePeerID),
		CoTenantPublicData: coTenantHit(&mockPublicData{listingIndex: nil}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetListings(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Empty(t, got, "empty ListingIndex is still a valid co-tenant hit (nil == empty)")
}

func TestListingGetListingBySlug_CoTenantHit(t *testing.T) {
	sl := &pb.SignedListing{Listing: &pb.Listing{Slug: "cool-shirt"}}
	svc := newTestListingAppService(t, ListingAppServiceConfig{
		NodeID: mustPeerID(t, testLocalNodePeerID),
		CoTenantPublicData: coTenantHit(&mockPublicData{
			listings: map[string]*pb.SignedListing{"cool-shirt": sl},
		}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetListingBySlug(context.Background(), pid, "cool-shirt", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "cool-shirt", got.Listing.Slug)
}

// --- ProfileAppService tests ---

func TestProfileGetProfile_CoTenantHit(t *testing.T) {
	wantProfile := &models.Profile{Name: "Alice"}
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewProfileAppService(ProfileAppServiceConfig{
		DB:                 db,
		PeerID:             mustPeerID(t, testVendorPeerID),
		CoTenantPublicData: coTenantHit(&mockPublicData{profile: wantProfile}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetProfile(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
}

func TestProfileGetProfile_CoTenantErrorFallthrough(t *testing.T) {
	wantProfile := &models.Profile{Name: "Carol"}
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	callCount := 0
	svc := NewProfileAppService(ProfileAppServiceConfig{
		DB:     db,
		PeerID: mustPeerID(t, testVendorPeerID),
		CoTenantPublicData: func(_ peer.ID) (pkgdb.PublicData, error) {
			callCount++
			return &mockPublicData{profile: wantProfile}, nil
		},
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetProfile(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "co-tenant resolver should be called exactly once")
	assert.Equal(t, "Carol", got.Name)
}

// --- RatingsAppService tests ---

func TestRatingsGetRatings_CoTenantHit(t *testing.T) {
	wantIndex := models.RatingIndex{{Slug: "rating-1"}}
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewRatingsAppService(RatingsAppServiceConfig{
		DB:                 db,
		CoTenantPublicData: coTenantHit(&mockPublicData{ratingIndex: wantIndex}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetRatings(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, wantIndex, got)
}

// --- FollowAppService tests ---

func TestFollowGetFollowers_CoTenantHit(t *testing.T) {
	wantFollowers := models.Followers{testVendorPeerID}
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewFollowAppService(FollowAppServiceConfig{
		DB:                 db,
		CoTenantPublicData: coTenantHit(&mockPublicData{followers: wantFollowers}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetFollowers(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, wantFollowers, got)
}

func TestFollowGetFollowing_CoTenantHit(t *testing.T) {
	wantFollowing := models.Following{testVendorPeerID}
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewFollowAppService(FollowAppServiceConfig{
		DB:                 db,
		CoTenantPublicData: coTenantHit(&mockPublicData{following: wantFollowing}),
	})

	pid := mustPeerID(t, testVendorPeerID)
	got, err := svc.GetFollowing(context.Background(), pid, nil, false)
	require.NoError(t, err)
	assert.Equal(t, wantFollowing, got)
}

// --- coTenantPublicDataDeferred tests ---

func TestCoTenantPublicDataDeferred_NilFn(t *testing.T) {
	node := &MobazhaNode{}
	deferred := node.coTenantPublicDataDeferred()
	require.NotNil(t, deferred, "deferred wrapper should never be nil")

	pid := mustPeerID(t, testVendorPeerID)
	_, err := deferred(pid)
	assert.Error(t, err, "should return error when coTenantPublicData is nil")
	assert.Contains(t, err.Error(), "not configured")
}

func TestCoTenantPublicDataDeferred_SetAfterCreation(t *testing.T) {
	node := &MobazhaNode{}
	deferred := node.coTenantPublicDataDeferred()

	wantProfile := &models.Profile{Name: "Bob"}
	node.SetCoTenantPublicData(coTenantHit(&mockPublicData{profile: wantProfile}))

	pid := mustPeerID(t, testVendorPeerID)
	pd, err := deferred(pid)
	require.NoError(t, err)

	profile, err := pd.GetProfile()
	require.NoError(t, err)
	assert.Equal(t, "Bob", profile.Name)
}
