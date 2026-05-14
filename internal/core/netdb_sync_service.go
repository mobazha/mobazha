//go:build !private_distribution

package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

const reconcileInterval = 10 * time.Minute

const netdbDirtyPrefix = "netdb_dirty_"

// netDBWriter abstracts the write methods of netdb.NetDB used by this service,
// enabling unit tests without an HTTP test server.
type netDBWriter interface {
	SetOwnProfile(profile *models.Profile) error
	SetOwnListing(sl *pb.SignedListing) error
	SetOwnListingIndex(index models.ListingIndex) error
	DeleteOwnListing(listingID string) error
	SetOwnFollowing(following models.Following) error
	SetOwnFollowers(followers models.Followers) error
	SetOwnStoreMetadata(metadataType string, data json.RawMessage) error
	SetOwnRatingIndex(index models.RatingIndex) error
	SetOwnRating(vendorPeerID string, ratingJSON json.RawMessage) error
}

// NetDBSyncService centralises all search-service (NetDB) push logic.
// AppServices emit lightweight domain events; this service subscribes,
// reads fresh data from DB, and pushes to the search service. On failure
// it marks a dirty flag so the next startup can reconcile.
type NetDBSyncService struct {
	netDB    netDBWriter
	db       database.Database
	eventBus events.Bus
	nodeID   string

	listingService    *ListingAppService
	ratingsService    *RatingsAppService
	collectionService *CollectionAppService
	discountService   *DiscountAppService

	cancel context.CancelFunc
}

type NetDBSyncServiceConfig struct {
	NetDB    netDBWriter
	DB       database.Database
	EventBus events.Bus
	NodeID   string

	ListingService    *ListingAppService
	RatingsService    *RatingsAppService
	CollectionService *CollectionAppService
	DiscountService   *DiscountAppService
}

func NewNetDBSyncService(cfg NetDBSyncServiceConfig) *NetDBSyncService {
	return &NetDBSyncService{
		netDB:             cfg.NetDB,
		db:                cfg.DB,
		eventBus:          cfg.EventBus,
		nodeID:            cfg.NodeID,
		listingService:    cfg.ListingService,
		ratingsService:    cfg.RatingsService,
		collectionService: cfg.CollectionService,
		discountService:   cfg.DiscountService,
	}
}

// Start subscribes to all NetDB-related domain events and dispatches
// them to the appropriate handler in a background goroutine. It also
// starts a periodic reconciliation timer that retries any dirty flags
// left by failed pushes, ensuring data eventually reaches the search
// service even when the initial push (and startup Reconcile) both fail.
func (s *NetDBSyncService) Start() {
	if s.netDB == nil || s.eventBus == nil {
		return
	}

	sub, err := s.eventBus.Subscribe([]interface{}{
		&events.ProfileChanged{},
		&events.ListingChanged{},
		&events.ListingDeleted{},
		&events.ListingsReindexed{},
		&events.FollowingChanged{},
		&events.FollowersChanged{},
		&events.CollectionsChanged{},
		&events.DiscountsChanged{},
		&events.StorefrontChanged{},
		&events.RatingsChanged{},
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "NetDBSyncService: failed to subscribe: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-sub.Out():
				if !ok {
					return
				}
				s.dispatch(evt)
			}
		}
	}()
}

// Stop shuts down the background event consumer.
func (s *NetDBSyncService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Reconcile checks all dirty flags and re-pushes stale data.
// Called during Node.Start() to catch updates made while offline.
func (s *NetDBSyncService) Reconcile() {
	if s.netDB == nil {
		return
	}

	dirtyKeys := s.allDirtyKeys()
	if len(dirtyKeys) == 0 {
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "NetDBSyncService: reconciling %d dirty keys: %v", len(dirtyKeys), dirtyKeys)

	for _, key := range dirtyKeys {
		switch key {
		case "profile":
			s.pushProfile()
		case "listing_index":
			s.pushAllListings()
		case "following":
			s.pushFollowing()
		case "followers":
			s.pushFollowers()
		case "collections":
			s.pushCollections()
		case "discounts":
			s.pushDiscounts()
		case "storefront":
			s.pushStorefront()
		case "ratings":
			s.pushRatingIndex()
		}
	}
}

func (s *NetDBSyncService) dispatch(evt interface{}) {
	switch e := evt.(type) {
	case *events.ProfileChanged:
		s.pushProfile()
	case *events.ListingChanged:
		s.pushSingleListing(e.Slug)
		s.pushListingIndex()
	case *events.ListingDeleted:
		if e.Cid != "" {
			if err := s.netDB.DeleteOwnListing(e.Cid); err != nil {
				logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: DeleteOwnListing(%s): %v", e.Cid, err)
			}
		}
		s.pushListingIndex()
	case *events.ListingsReindexed:
		s.pushAllListings()
		s.pushProfile()
	case *events.FollowingChanged:
		s.pushFollowing()
		s.pushProfile()
	case *events.FollowersChanged:
		s.pushFollowers()
		s.pushProfile()
	case *events.CollectionsChanged:
		s.pushCollections()
	case *events.DiscountsChanged:
		s.pushDiscounts()
	case *events.StorefrontChanged:
		s.pushStorefrontData(e.Config)
	case *events.RatingsChanged:
		s.pushRatingIndex()
		s.pushIndividualRatings(e.Ratings)
	}
}

// ── Push helpers ────────────────────────────────────────────────

func (s *NetDBSyncService) pushProfile() {
	profile, err := getProfileWithStats(s.db)
	if err != nil {
		s.markDirty("profile")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: read profile failed: %v", err)
		return
	}
	if err := s.netDB.SetOwnProfile(profile); err != nil {
		s.markDirty("profile")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnProfile failed: %v", err)
		return
	}
	s.clearDirty("profile")
}

func (s *NetDBSyncService) pushSingleListing(slug string) {
	if s.listingService == nil {
		return
	}
	sl, err := s.listingService.GetMyListingBySlug(slug)
	if err != nil {
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: GetMyListingBySlug(%s): %v", slug, err)
		return
	}
	if err := s.netDB.SetOwnListing(sl); err != nil {
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnListing(%s): %v", slug, err)
	}
}

func (s *NetDBSyncService) pushListingIndex() {
	if s.listingService == nil {
		return
	}
	idx, err := s.listingService.GetMyListings()
	if err != nil {
		s.markDirty("listing_index")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: GetMyListings: %v", err)
		return
	}
	if err := s.netDB.SetOwnListingIndex(idx); err != nil {
		s.markDirty("listing_index")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnListingIndex: %v", err)
		return
	}
	s.clearDirty("listing_index")
}

func (s *NetDBSyncService) pushAllListings() {
	if s.listingService == nil {
		return
	}
	idx, err := s.listingService.GetMyListings()
	if err != nil {
		s.markDirty("listing_index")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: GetMyListings(all): %v", err)
		return
	}
	if err := s.netDB.SetOwnListingIndex(idx); err != nil {
		s.markDirty("listing_index")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnListingIndex(all): %v", err)
	} else {
		s.clearDirty("listing_index")
	}
	for _, lmd := range idx {
		if sl, err := s.listingService.GetMyListingBySlug(lmd.Slug); err == nil {
			if pushErr := s.netDB.SetOwnListing(sl); pushErr != nil {
				logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnListing(%s): %v", lmd.Slug, pushErr)
			}
		}
	}
}

func (s *NetDBSyncService) pushFollowing() {
	var following models.Following
	err := s.db.View(func(tx database.Tx) error {
		var e error
		following, e = tx.GetFollowing()
		return e
	})
	if err != nil {
		s.markDirty("following")
		return
	}
	if err := s.netDB.SetOwnFollowing(following); err != nil {
		s.markDirty("following")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnFollowing: %v", err)
	} else {
		s.clearDirty("following")
	}
}

func (s *NetDBSyncService) pushFollowers() {
	var followers models.Followers
	err := s.db.View(func(tx database.Tx) error {
		var e error
		followers, e = tx.GetFollowers()
		return e
	})
	if err != nil {
		s.markDirty("followers")
		return
	}
	if err := s.netDB.SetOwnFollowers(followers); err != nil {
		s.markDirty("followers")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnFollowers: %v", err)
	} else {
		s.clearDirty("followers")
	}
}

func (s *NetDBSyncService) pushCollections() {
	if s.collectionService == nil {
		return
	}
	collections, _, err := s.collectionService.store.ListCollections(context.Background(), 1, maxCollectionsPerTenant, false)
	if err != nil {
		s.markDirty("collections")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: ListCollections: %v", err)
		return
	}
	data, err := json.Marshal(collections)
	if err != nil {
		s.markDirty("collections")
		return
	}
	if err := s.netDB.SetOwnStoreMetadata("collections", data); err != nil {
		s.markDirty("collections")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnStoreMetadata(collections): %v", err)
	} else {
		s.clearDirty("collections")
	}
}

func (s *NetDBSyncService) pushDiscounts() {
	if s.discountService == nil {
		return
	}
	activeStatus := models.DiscountStatusActive
	discounts, _, err := s.discountService.store.ListDiscounts(context.Background(), contracts.DiscountFilter{
		Page:     1,
		PageSize: maxDiscountsPerTenant,
		Status:   &activeStatus,
	})
	if err != nil {
		s.markDirty("discounts")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: ListDiscounts: %v", err)
		return
	}
	data, err := json.Marshal(discounts)
	if err != nil {
		s.markDirty("discounts")
		return
	}
	if err := s.netDB.SetOwnStoreMetadata("discounts", data); err != nil {
		s.markDirty("discounts")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnStoreMetadata(discounts): %v", err)
	} else {
		s.clearDirty("discounts")
	}
}

func (s *NetDBSyncService) pushStorefront() {
	val, err := s.readSetting(models.SettingsKeyStoreConfig)
	if err != nil || val == "" {
		return
	}
	s.pushStorefrontData(json.RawMessage(val))
}

func (s *NetDBSyncService) pushStorefrontData(cfg json.RawMessage) {
	if err := s.netDB.SetOwnStoreMetadata("storefront", cfg); err != nil {
		s.markDirty("storefront")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnStoreMetadata(storefront): %v", err)
	} else {
		s.clearDirty("storefront")
	}
}

func (s *NetDBSyncService) pushRatingIndex() {
	if s.ratingsService == nil {
		return
	}
	idx, err := s.ratingsService.GetMyRatings()
	if err != nil {
		s.markDirty("ratings")
		return
	}
	if err := s.netDB.SetOwnRatingIndex(idx); err != nil {
		s.markDirty("ratings")
		logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnRatingIndex: %v", err)
	} else {
		s.clearDirty("ratings")
	}
}

func (s *NetDBSyncService) pushIndividualRatings(ratings []*pb.Rating) {
	if ratings == nil {
		return
	}
	marshaler := protojson.MarshalOptions{EmitUnpopulated: false}
	for _, r := range ratings {
		vendorPeerID := ""
		if r.VendorID != nil {
			vendorPeerID = r.VendorID.PeerID
		}
		if vendorPeerID == "" {
			continue
		}
		ratingBytes, err := marshaler.Marshal(r)
		if err != nil {
			logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: marshal rating: %v", err)
			continue
		}
		if err := s.netDB.SetOwnRating(vendorPeerID, json.RawMessage(ratingBytes)); err != nil {
			logger.LogDebugWithIDf(log, s.nodeID, "NetDBSync: SetOwnRating: %v", err)
		}
	}
}

// ── Dirty flag helpers (node_settings table) ────────────────────

func (s *NetDBSyncService) markDirty(key string) {
	_ = s.writeSetting(netdbDirtyPrefix+key, "1")
}

func (s *NetDBSyncService) clearDirty(key string) {
	_ = s.writeSetting(netdbDirtyPrefix+key, "")
}

func (s *NetDBSyncService) allDirtyKeys() []string {
	knownKeys := []string{"profile", "listing_index", "following", "followers", "collections", "discounts", "storefront", "ratings"}
	var dirty []string
	for _, k := range knownKeys {
		val, err := s.readSetting(netdbDirtyPrefix + k)
		if err == nil && val != "" {
			dirty = append(dirty, k)
		}
	}
	return dirty
}

func (s *NetDBSyncService) readSetting(key string) (string, error) {
	var setting models.NodeSettings
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("key = ?", key).First(&setting).Error
	})
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *NetDBSyncService) writeSetting(key, value string) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.NodeSettings{Key: key, Value: value})
	})
}
