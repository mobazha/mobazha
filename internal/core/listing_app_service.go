package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/gosimple/slug"
	"github.com/ipfs/go-cid"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/microcosm-cc/bluemonday"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

var _ contracts.ListingPublisher = (*ListingAppService)(nil)

// ListingAppService encapsulates listing CRUD, validation, signing, and network sync.
type ListingAppService struct {
	db                 database.Database
	signer             contracts.Signer
	contentStore       contracts.ContentStore
	netDB              *netdb.NetDB
	eventBus           events.Bus
	banChecker         contracts.BanChecker
	keys               contracts.KeyProvider
	featureManager     *pkgconfig.FeatureManager
	localListingCrypto *encryption.LocalListingCrypto
	nodeID             peer.ID
	testnet            bool

	publish         PublishFunc
	onDeleteCleanup func(slug string)

	coTenantPublicData contracts.CoTenantPublicDataFn
	coTenantAllPeers   func() []peer.ID

	shippingStore contracts.ShippingStore
}

type ListingAppServiceConfig struct {
	DB                 database.Database
	Signer             contracts.Signer
	ContentStore       contracts.ContentStore
	NetDB              *netdb.NetDB
	EventBus           events.Bus
	BanChecker         contracts.BanChecker
	Keys               contracts.KeyProvider
	FeatureManager     *pkgconfig.FeatureManager
	LocalListingCrypto *encryption.LocalListingCrypto
	NodeID             peer.ID
	Testnet            bool

	Publish PublishFunc

	CoTenantPublicData contracts.CoTenantPublicDataFn
	CoTenantAllPeers   func() []peer.ID

	ShippingStore contracts.ShippingStore
}

func NewListingAppService(cfg ListingAppServiceConfig) *ListingAppService {
	return &ListingAppService{
		db:                 cfg.DB,
		signer:             cfg.Signer,
		contentStore:       cfg.ContentStore,
		netDB:              cfg.NetDB,
		eventBus:           cfg.EventBus,
		banChecker:         cfg.BanChecker,
		keys:               cfg.Keys,
		featureManager:     cfg.FeatureManager,
		localListingCrypto: cfg.LocalListingCrypto,
		nodeID:             cfg.NodeID,
		testnet:            cfg.Testnet,
		publish:            cfg.Publish,
		coTenantPublicData: cfg.CoTenantPublicData,
		coTenantAllPeers:   cfg.CoTenantAllPeers,
		shippingStore:      cfg.ShippingStore,
	}
}

// SetShippingStore allows late injection of the ShippingStore, useful when the
// shipping subsystem initializes after the listing service.
func (s *ListingAppService) SetShippingStore(store contracts.ShippingStore) {
	s.shippingStore = store
}

// SetCoTenantAllPeers allows late injection of a function that returns all
// co-tenant peer IDs. Used by mocknet for CID-based listing lookup across nodes.
func (s *ListingAppService) SetCoTenantAllPeers(fn func() []peer.ID) {
	s.coTenantAllPeers = fn
}

func (s *ListingAppService) IsGlobalBanned(peerID peer.ID) bool {
	if s.banChecker == nil {
		return false
	}
	return s.banChecker.IsGlobalBanned(peerID)
}

// resolveShippingProfile resolves the shipping profile entity for a physical listing.
// If the listing specifies a profileID, that profile is fetched; otherwise the default
// profile is used. Returns an error if no profile can be resolved.
func (s *ListingAppService) resolveShippingProfile(listing *pb.Listing) (*models.ShippingProfileEntity, error) {
	if s.shippingStore == nil {
		return nil, fmt.Errorf("%w: shipping subsystem not initialized", coreiface.ErrBadRequest)
	}
	ctx := context.Background()

	profileID := listing.GetShippingProfileId()
	if profileID == "" && listing.ShippingProfile != nil {
		profileID = listing.ShippingProfile.ProfileID
	}

	if profileID != "" {
		entity, err := s.shippingStore.GetProfile(ctx, profileID)
		if err != nil {
			return nil, err
		}
		if entity == nil {
			return nil, fmt.Errorf("%w: shipping profile not found: %s", coreiface.ErrNotFound, profileID)
		}
		return entity, nil
	}

	entity, err := s.shippingStore.GetDefaultProfile(ctx)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, fmt.Errorf("%w: no default shipping profile configured", coreiface.ErrBadRequest)
	}
	return entity, nil
}

func (s *ListingAppService) SaveListing(listing *pb.Listing, done chan<- struct{}) error {
	// Resolve shipping profile BEFORE the main transaction to avoid deadlock.
	// GormShippingStore opens its own db.View() internally; calling it inside
	// s.db.Update() would re-enter the same database.Database and deadlock.
	isDraft := listing.Status == models.ListingStatusDraft
	var resolvedProfileID string
	var resolvedProfileVersion int
	if listing.Metadata != nil && listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
		entity, resolveErr := s.resolveShippingProfile(listing)
		if resolveErr != nil {
			if !isDraft {
				maybeCloseDone(done)
				return resolveErr
			}
			// Drafts: shipping profile resolution failure is non-fatal;
			// keep whatever inline profile was submitted (or nil).
		} else {
			listing.ShippingProfile = models.ConvertShippingEntityToProto(entity)
			resolvedProfileID = entity.ID
			resolvedProfileVersion = entity.Version
		}
	}

	err := s.db.Update(func(tx database.Tx) error {
		var currentPrefs models.UserPreferences
		err := tx.Read().First(&currentPrefs).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if len(listing.Moderators) == 0 {
			mods, err := currentPrefs.StoreModerators()
			if err != nil {
				return err
			}
			modStrs := make([]string, 0, len(mods))
			for _, mod := range mods {
				modStrs = append(modStrs, mod.String())
			}
			listing.Moderators = modStrs
		}

		cid, err := s.saveListingToDB(tx, listing)
		if err != nil {
			return err
		}

		lmd, err := models.NewListingMetadataFromListing(listing, cid)
		if err != nil {
			return err
		}

		index, err := tx.GetListingIndex()
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		index.UpdateListing(*lmd)

		return tx.SetListingIndex(index)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	// Upsert/delete shipping ref AFTER the main transaction for the same reason:
	// shippingStore methods open their own transactions internally.
	if s.shippingStore != nil && listing.Slug != "" {
		if resolvedProfileID != "" {
			ref := &models.ListingShippingRef{
				ListingSlug:       listing.Slug,
				ShippingProfileID: resolvedProfileID,
				SnapshotVersion:   resolvedProfileVersion,
				IsStale:           false,
			}
			if upsertErr := s.shippingStore.UpsertListingRef(context.Background(), ref); upsertErr != nil {
				log.Errorf("failed to upsert shipping ref for listing %s: %v", listing.Slug, upsertErr)
			}
		} else {
			_ = s.shippingStore.DeleteListingRef(context.Background(), listing.Slug)
		}
	}

	if listing.Status == models.ListingStatusDraft {
		maybeCloseDone(done)
		return nil
	}

	if s.eventBus != nil {
		s.eventBus.Emit(&events.ListingChanged{Slug: listing.Slug})
	}

	s.publish(done)
	return nil
}

// RepublishListing implements contracts.ListingPublisher.
// It re-saves an existing listing, which triggers shipping profile snapshot refresh,
// re-signing, and network re-publication.
func (s *ListingAppService) RepublishListing(ctx context.Context, slug string) error {
	sl, err := s.GetMyListingBySlug(slug)
	if err != nil {
		return fmt.Errorf("get listing %s: %w", slug, err)
	}
	if sl.Listing == nil {
		return fmt.Errorf("listing %s has no content", slug)
	}
	return s.SaveListing(sl.Listing, nil)
}

// GetListingStatus returns the current status of a listing by slug.
func (s *ListingAppService) GetListingStatus(slug string) (string, error) {
	sl, err := s.GetMyListingBySlug(slug)
	if err != nil {
		return "", err
	}
	if sl.Listing == nil {
		return "", fmt.Errorf("listing %s has no content", slug)
	}
	return sl.Listing.Status, nil
}

// SetListingStatus changes a listing's status (e.g. "published" ↔ "draft")
// and re-saves it. Used by supply chain monitoring to hide/show listings
// based on supplier stock availability.
func (s *ListingAppService) SetListingStatus(slug string, status string) error {
	sl, err := s.GetMyListingBySlug(slug)
	if err != nil {
		return fmt.Errorf("get listing %s: %w", slug, err)
	}
	if sl.Listing == nil {
		return fmt.Errorf("listing %s has no content", slug)
	}
	oldStatus := sl.Listing.Status
	if oldStatus == status {
		return nil
	}
	sl.Listing.Status = status
	if err := s.SaveListing(sl.Listing, nil); err != nil {
		return err
	}

	// published/private → draft: remove from public listing index + notify search
	if oldStatus != models.ListingStatusDraft && status == models.ListingStatusDraft {
		if err := s.removeFromPublicIndex(slug); err != nil {
			return fmt.Errorf("remove %s from listing index: %w", slug, err)
		}
		if s.eventBus != nil && sl.Cid != "" {
			s.eventBus.Emit(&events.ListingDeleted{Cid: sl.Cid})
		}
	}
	return nil
}

// removeFromPublicIndex removes a slug from the local listing index without
// deleting the listing blob. On failure the caller must NOT emit events.
func (s *ListingAppService) removeFromPublicIndex(slugStr string) error {
	return s.db.Update(func(tx database.Tx) error {
		index, err := tx.GetListingIndex()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		index.DeleteListing(slugStr)
		return tx.SetListingIndex(index)
	})
}

func (s *ListingAppService) UpdateAllListings(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error {
	var (
		listingsUpdated = false
		err             error
	)
	err = s.db.Update(func(tx database.Tx) error {
		listingsUpdated, err = s.updateAllListings(tx, updateFunc)
		return err
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}
	if !listingsUpdated {
		maybeCloseDone(done)
		return nil
	}

	if s.eventBus != nil {
		s.eventBus.Emit(&events.ListingsReindexed{})
	}

	s.publish(done)
	return nil
}

func (s *ListingAppService) DeleteListing(slugStr string, done chan<- struct{}) error {
	listing, _ := s.GetMyListingBySlug(slugStr)

	err := s.db.Update(func(tx database.Tx) error {
		index, err := tx.GetListingIndex()
		if err != nil {
			return fmt.Errorf("%w: listing index not found", coreiface.ErrNotFound)
		}
		index.DeleteListing(slugStr)
		if err := tx.SetListingIndex(index); err != nil {
			return err
		}

		if err := tx.DeleteListing(slugStr); err != nil {
			return fmt.Errorf("%w: listing not found", coreiface.ErrNotFound)
		}

		return nil
	})

	if err != nil {
		maybeCloseDone(done)
		return err
	}

	// Delete shipping ref AFTER the main transaction: shippingStore methods
	// open their own transactions internally, and TenantDB mutex is not reentrant.
	if s.shippingStore != nil {
		_ = s.shippingStore.DeleteListingRef(context.Background(), slugStr)
	}

	if s.onDeleteCleanup != nil {
		s.onDeleteCleanup(slugStr)
	}

	if listing != nil && s.eventBus != nil {
		s.eventBus.Emit(&events.ListingDeleted{Cid: listing.Cid})
	}

	s.publish(done)
	return nil
}

func (s *ListingAppService) GetMyListings() (models.ListingIndex, error) {
	var index models.ListingIndex
	err := s.db.View(func(tx database.Tx) error {
		var txErr error
		index, txErr = tx.GetListingIndex()
		if txErr != nil {
			if os.IsNotExist(txErr) {
				index = models.ListingIndex{}
				return nil
			}
			return txErr
		}
		return nil
	})
	return index, err
}

func (s *ListingAppService) GetListings(_ context.Context, peerID peer.ID, reqCtx *request.Context, _ bool) (models.ListingIndex, error) {
	if peerID == s.nodeID {
		return s.GetMyListings()
	}

	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if index, err := pd.GetListingIndex(); err == nil {
				return index, nil
			}
		}
	}

	if s.netDB != nil {
		return s.netDB.GetListingIndex(peerID.String(), reqCtx)
	}

	return nil, fmt.Errorf("listing data not available for remote peer %s", peerID)
}

func (s *ListingAppService) GetMyListingBySlug(slugStr string) (*pb.SignedListing, error) {
	var (
		listing *pb.SignedListing
		err     error
	)
	err = s.db.View(func(tx database.Tx) error {
		index, err := tx.GetListingIndex()
		if err != nil {
			return fmt.Errorf("%w: listing index not found", coreiface.ErrNotFound)
		}

		id, err := index.GetListingCID(slugStr)
		if err != nil {
			return fmt.Errorf("%w: listing not found", coreiface.ErrNotFound)
		}

		readPlaintext := func() error {
			var err error
			listing, err = tx.GetListing(slugStr)
			if err != nil {
				return fmt.Errorf("%w: listing not found", coreiface.ErrNotFound)
			}
			listing.Cid = id.String()
			return nil
		}

		useEncryption := s.featureManager != nil && s.featureManager.IsEnabled(pkgconfig.FeaturePrivacyLocalEncryptedStorageEnabled)
		if !useEncryption {
			return readPlaintext()
		}

		encryptedData, err := tx.GetEncryptedListing(slugStr)
		if err != nil {
			return readPlaintext()
		}

		decryptedData, err := s.localListingCrypto.TryDecryptListingData(encryptedData, slugStr)
		if err != nil {
			log.Warningf("Failed to decrypt listing %s, falling back to plaintext: %v", slugStr, err)
			return readPlaintext()
		}

		var sl pb.SignedListing
		if err := (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(decryptedData, &sl); err != nil {
			return fmt.Errorf("failed to unmarshal decrypted listing: %w", err)
		}
		listing = &sl

		listing.Cid = id.String()
		return nil
	})
	if err != nil {
		return nil, err
	}

	return listing, nil
}

func (s *ListingAppService) GetMyListingByCID(c cid.Cid) (*pb.SignedListing, error) {
	var (
		listing *pb.SignedListing
		err     error
	)
	err = s.db.View(func(tx database.Tx) error {
		index, err := tx.GetListingIndex()
		if err != nil {
			return fmt.Errorf("%w: listing index not found", coreiface.ErrNotFound)
		}
		slugStr, err := index.GetListingSlug(c)
		if err != nil {
			return fmt.Errorf("%w: listing not found in index", coreiface.ErrNotFound)
		}
		listing, err = tx.GetListing(slugStr)
		if err != nil {
			return fmt.Errorf("%w: listing not found", coreiface.ErrNotFound)
		}
		listing.Cid = c.String()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return listing, nil
}

func (s *ListingAppService) GetListingBySlug(_ context.Context, peerID peer.ID, slugStr string, reqCtx *request.Context, _ bool) (*pb.SignedListing, error) {
	if peerID == s.nodeID {
		return s.GetMyListingBySlug(slugStr)
	}

	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if sl, err := pd.GetListing(slugStr); err == nil {
				return sl, nil
			}
		}
	}

	if s.netDB != nil {
		return s.netDB.GetListingBySlug(peerID.String(), slugStr, reqCtx)
	}

	return nil, fmt.Errorf("listing data not available for remote peer %s", peerID)
}

func (s *ListingAppService) GetListingByCID(_ context.Context, c cid.Cid, reqCtx *request.Context) (*pb.SignedListing, error) {
	if s.netDB != nil {
		return s.netDB.GetListingByCID(c.String(), reqCtx)
	}

	cidStr := c.String()

	// Local DB lookup: find the listing by CID in our own index, then load by slug.
	if slug, found := s.findSlugByCIDLocal(cidStr); found {
		var sl *pb.SignedListing
		err := s.db.View(func(tx database.Tx) error {
			var e error
			sl, e = tx.GetListing(slug)
			return e
		})
		if err == nil {
			return sl, nil
		}
	}

	// Co-tenant search: iterate over all known peers to find the CID.
	if s.coTenantPublicData != nil && s.coTenantAllPeers != nil {
		for _, pid := range s.coTenantAllPeers() {
			pd, err := s.coTenantPublicData(pid)
			if err != nil {
				continue
			}
			idx, err := pd.GetListingIndex()
			if err != nil {
				continue
			}
			for _, lm := range idx {
				if lm.CID == cidStr {
					return pd.GetListing(lm.Slug)
				}
			}
		}
	}

	return nil, fmt.Errorf("listing data not available for CID %s", c)
}

// findSlugByCIDLocal looks up the local listing index for a matching CID.
func (s *ListingAppService) findSlugByCIDLocal(cidStr string) (string, bool) {
	var slug string
	err := s.db.View(func(tx database.Tx) error {
		idx, err := tx.GetListingIndex()
		if err != nil {
			return err
		}
		for _, lm := range idx {
			if lm.CID == cidStr {
				slug = lm.Slug
				return nil
			}
		}
		return fmt.Errorf("not found")
	})
	return slug, err == nil
}

// --- Private methods ---

func (s *ListingAppService) generateListingSlug(dbtx database.Tx, title string) (string, error) {
	title = strings.Replace(title, "/", "", -1)
	counter := 1

	l := SentenceMaxCharacters - SlugBuffer

	var rx = regexp.MustCompile(EmojiPattern)
	title = rx.ReplaceAllStringFunc(title, func(str string) string {
		r, _ := utf8.DecodeRuneInString(str)
		html := fmt.Sprintf(`&#x%X;`, r)
		return html
	})

	slugBase := slug.Make(title)
	if len(slugBase) < SentenceMaxCharacters-SlugBuffer {
		l = len(slugBase)
	}
	slugBase = slugBase[:l]

	slugToTry := slugBase
	for {
		index, err := dbtx.GetListingIndex()
		if os.IsNotExist(err) {
			return slugToTry, nil
		} else if err != nil {
			return "", err
		}

		_, err = index.GetListingCID(slugToTry)
		if err != nil {
			return slugToTry, nil
		}
		slugToTry = slugBase + strconv.Itoa(counter)
		counter++
	}
}

func (s *ListingAppService) updateAllListings(tx database.Tx, updateFunc func(l *pb.Listing) (bool, error)) (listingsUpdated bool, _ error) {
	index, err := tx.GetListingIndex()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	var updatedMetadata []models.ListingMetadata
	for _, lmd := range index {
		signedListing, err := tx.GetListing(lmd.Slug)
		if err != nil {
			return false, err
		}
		listing := signedListing.Listing

		updated, err := updateFunc(listing)
		if err != nil {
			return false, err
		}

		if updated {
			c, err := s.saveListingToDB(tx, listing)
			if err != nil {
				return false, err
			}

			newLmd, err := models.NewListingMetadataFromListing(listing, c)
			if err != nil {
				return false, err
			}

			updatedMetadata = append(updatedMetadata, *newLmd)
			listingsUpdated = true
		}
	}
	if !listingsUpdated {
		return false, nil
	}

	for _, lmd := range updatedMetadata {
		index.UpdateListing(lmd)
	}

	if err := tx.SetListingIndex(index); err != nil {
		return true, err
	}

	return true, nil
}

func (s *ListingAppService) saveListingToDB(dbtx database.Tx, listing *pb.Listing) (cid.Cid, error) {
	if listing.Item == nil {
		return cid.Cid{}, fmt.Errorf("%w: no item in listing", coreiface.ErrBadRequest)
	}

	if s.testnet {
		if listing.Metadata.EscrowTimeoutHours == 0 {
			listing.Metadata.EscrowTimeoutHours = 1
		}
	} else {
		listing.Metadata.EscrowTimeoutHours = EscrowTimeout
	}

	if listing.Slug == "" {
		var err error
		listing.Slug, err = s.generateListingSlug(dbtx, listing.Item.Title)
		if err != nil {
			return cid.Cid{}, err
		}
	}

	sanitizer := bluemonday.UGCPolicy()
	for _, opt := range listing.Item.Options {
		opt.Name = sanitizer.Sanitize(opt.Name)
		for _, v := range opt.Variants {
			v.Name = sanitizer.Sanitize(v.Name)
		}
	}
	if listing.ShippingProfile != nil {
		for _, lg := range listing.ShippingProfile.LocationGroups {
			if lg == nil {
				continue
			}
			for _, zone := range lg.Zones {
				if zone == nil {
					continue
				}
				zone.Name = sanitizer.Sanitize(zone.Name)
				for _, rate := range zone.Rates {
					if rate != nil {
						rate.Name = sanitizer.Sanitize(rate.Name)
					}
				}
			}
		}
	}

	if listing.Metadata.Version <= 0 {
		listing.Metadata.Version = ListingVersion
	}

	profile, err := dbtx.GetProfile()
	if err != nil && !os.IsNotExist(err) {
		return cid.Cid{}, err
	}
	rawPubKey, err := s.signer.PublicKey()
	if err != nil {
		return cid.Cid{}, err
	}
	pubkey, err := identity.MarshalPublicKeyFromEd25519(rawPubKey)
	if err != nil {
		return cid.Cid{}, err
	}

	if s.keys == nil {
		return cid.Cid{}, fmt.Errorf("key provider not available")
	}
	escrowKey, err := s.keys.EscrowMasterKey()
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to get escrow master key: %w", err)
	}
	ethKey, err := s.keys.EVMMasterKey()
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to get EVM master key: %w", err)
	}
	solKey, err := s.keys.SolanaMasterKey()
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to get Solana master key: %w", err)
	}

	idHash := sha256.Sum256([]byte(s.nodeID.String()))
	sig := ecdsa.Sign(escrowKey, idHash[:])

	listing.VendorID = &pb.ID{
		PeerID: s.nodeID.String(),
		Pubkeys: &pb.ID_Pubkeys{
			Identity: pubkey,
			Escrow:   escrowKey.PubKey().SerializeCompressed(),
			Eth:      ethKey.PubKey().SerializeCompressed(),
			Solana:   solKey.PublicKey().Bytes(),
		},
		Sig: sig.Serialize(),
	}
	if profile != nil {
		listing.VendorID.Handle = profile.Handle
	}

	sl, err := s.signListing(listing)
	if err != nil {
		return cid.Cid{}, err
	}

	if listing.Status == models.ListingStatusDraft {
		if err := s.validateListingDraft(sl); err != nil {
			return cid.Cid{}, fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
		}
	} else {
		if err := s.ValidateListing(sl); err != nil {
			if errors.Is(err, coreiface.ErrInternalServer) {
				return cid.Cid{}, err
			}
			return cid.Cid{}, fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
		}
	}

	m := protojson.MarshalOptions{
		EmitUnpopulated: false,
	}
	ser := m.Format(sl)
	var out bytes.Buffer
	json.Indent(&out, []byte(ser), "", "    ")
	plaintextCID, err := s.contentStore.ComputeCID(out.Bytes())
	if err != nil {
		return cid.Cid{}, err
	}

	savePlaintext := func() error {
		return dbtx.SetListing(sl)
	}

	useEncryption := s.featureManager != nil && s.featureManager.IsEnabled(pkgconfig.FeaturePrivacyLocalEncryptedStorageEnabled)
	if !useEncryption {
		if err := savePlaintext(); err != nil {
			return cid.Cid{}, err
		}
		return plaintextCID, nil
	}

	profile, err = dbtx.GetProfile()
	if err != nil {
		return cid.Cid{}, err
	}

	_, encryptedData, err := s.localListingCrypto.EncryptListing(sl)
	if err != nil {
		log.Warningf("Failed to encrypt listing %s, saving as plaintext: %v", listing.Slug, err)
		if err := savePlaintext(); err != nil {
			return cid.Cid{}, err
		}
		return plaintextCID, nil
	}

	if err := dbtx.SetEncryptedListing(listing.Slug, encryptedData); err != nil {
		return cid.Cid{}, err
	}

	return plaintextCID, nil
}

func (s *ListingAppService) signListing(listing *pb.Listing) (*pb.SignedListing, error) {
	m := protojson.MarshalOptions{
		EmitUnpopulated: false,
	}
	ser := m.Format(listing)

	var out bytes.Buffer
	json.Indent(&out, []byte(ser), "", "")
	sig, err := s.signer.Sign(out.Bytes())
	if err != nil {
		return nil, err
	}
	return &pb.SignedListing{Listing: listing, Signature: sig}, nil
}

func validateImageHashes(img *pb.Image) error {
	if img == nil {
		return nil
	}
	_, err := cid.Decode(img.Tiny)
	if err != nil {
		return errors.New("tiny image hashes must be properly formatted CID")
	}
	_, err = cid.Decode(img.Small)
	if err != nil {
		return errors.New("small image hashes must be properly formatted CID")
	}
	_, err = cid.Decode(img.Medium)
	if err != nil {
		return errors.New("medium image hashes must be properly formatted CID")
	}
	_, err = cid.Decode(img.Large)
	if err != nil {
		return errors.New("large image hashes must be properly formatted CID")
	}
	_, err = cid.Decode(img.Original)
	if err != nil {
		return errors.New("original image hashes must be properly formatted CID")
	}
	return nil
}

func hasAnyImageRef(img *pb.Image) bool {
	if img == nil {
		return false
	}
	return img.Filename != "" || img.Large != "" || img.Medium != "" ||
		img.Small != "" || img.Tiny != "" || img.Original != ""
}

func (s *ListingAppService) ValidateListing(sl *pb.SignedListing) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("unknown panic")
			}
		}
	}()

	if sl.Listing.Slug == "" {
		return coreiface.ErrMissingField("slug")
	}
	if len(sl.Listing.Slug) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"slug", strconv.Itoa(SentenceMaxCharacters)}
	}
	if strings.Contains(sl.Listing.Slug, " ") {
		return errors.New("slugs cannot contain spaces")
	}
	if strings.Contains(sl.Listing.Slug, "/") {
		return errors.New("slugs cannot contain file separators")
	}

	if sl.Listing.Status != "" {
		if !models.ValidListingStatuses[sl.Listing.Status] {
			return fmt.Errorf("invalid listing status: %s (must be draft, published, or private)", sl.Listing.Status)
		}
	}

	if sl.Listing.Item.WeightUnit != "" {
		if !models.ValidWeightUnits[sl.Listing.Item.WeightUnit] {
			return fmt.Errorf("invalid weight unit: %s (must be g, kg, lb, or oz)", sl.Listing.Item.WeightUnit)
		}
	}

	if sl.Listing.Item.InventoryPolicy != "" {
		if !models.ValidInventoryPolicies[sl.Listing.Item.InventoryPolicy] {
			return fmt.Errorf("invalid inventory policy: %s (must be deny or continue)", sl.Listing.Item.InventoryPolicy)
		}
	}

	if sl.Listing.Item.DimensionUnit != "" {
		if !models.ValidDimensionUnits[sl.Listing.Item.DimensionUnit] {
			return fmt.Errorf("invalid dimension unit: %s (must be cm or in)", sl.Listing.Item.DimensionUnit)
		}
	}

	if len(sl.Listing.Item.Brand) > WordMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.brand", strconv.Itoa(WordMaxCharacters)}
	}

	if sl.Listing.Item.PackageLength < 0 || sl.Listing.Item.PackageWidth < 0 || sl.Listing.Item.PackageHeight < 0 {
		return fmt.Errorf("package dimensions must be non-negative")
	}

	if sl.Listing.Metadata == nil {
		return coreiface.ErrMissingField("metadata")
	}
	if sl.Listing.Metadata.ContractType > pb.Listing_Metadata_RWA_TOKEN {
		return errors.New("invalid contract type")
	}
	if sl.Listing.Metadata.Format > pb.Listing_Metadata_MARKET_PRICE {
		return errors.New("invalid listing format")
	}
	// PrivateDistribution build rejects MARKET_PRICE (needs exchange-rate oracle) and
	// RWA_TOKEN / CRYPTOCURRENCY contract types (need on-chain monitors).
	// No-op on full builds. Runs before the PricingCurrency block so we
	// catch MARKET_PRICE even when PricingCurrency is nil.
	if err := validatePrivateDistributionListingFormat(sl.Listing.Metadata.Format, sl.Listing.Metadata.ContractType); err != nil {
		return err
	}
	if sl.Listing.Metadata.Expiry == nil {
		return coreiface.ErrMissingField("metadata.expiry")
	}
	if time.Unix(sl.Listing.Metadata.Expiry.Seconds, 0).Before(time.Now()) {
		return errors.New("listing expiration must be in the future")
	}
	if len(sl.Listing.Metadata.Language) > WordMaxCharacters {
		return coreiface.ErrTooManyCharacters{"metadata.language", strconv.Itoa(WordMaxCharacters)}
	}
	if !s.testnet && sl.Listing.Metadata.EscrowTimeoutHours > EscrowTimeout {
		return fmt.Errorf("escrow timeout must be less than or equal to %d hours", EscrowTimeout)
	}
	if sl.Listing.Metadata.Format != pb.Listing_Metadata_MARKET_PRICE && sl.Listing.Metadata.PricingCurrency == nil {
		return coreiface.ErrMissingField("metadata.pricingcurrency")
	}
	if sl.Listing.Metadata.PricingCurrency != nil {
		if sl.Listing.Metadata.PricingCurrency.Code == "" {
			return coreiface.ErrMissingField("metadata.pricingcurrency.code")
		}
		if len(sl.Listing.Metadata.PricingCurrency.Code) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"metadata.pricingcurrency.code", strconv.Itoa(WordMaxCharacters)}
		}
		def, err := models.CurrencyDefinitions.Lookup(sl.Listing.Metadata.PricingCurrency.Code)
		if err != nil {
			return errors.New("unknown pricing currency")
		}
		if sl.Listing.Metadata.PricingCurrency.Divisibility != uint32(def.Divisibility) {
			return errors.New("divisibility differs from expected value")
		}
		// Crypto-native pricing guard: private_distribution builds reject any pricing
		// currency outside the supported set (see private_distribution_supported_coins.go).
		// This is a server-side defense against API bypass of the UI restriction
		// in BasicInfoSection. No-op on full builds.
		if err := validatePrivateDistributionPricingCurrency(sl.Listing.Metadata.PricingCurrency.Code); err != nil {
			return err
		}
	}

	if sl.Listing.Item.Title == "" {
		return coreiface.ErrMissingField("item.title")
	}
	price, _ := new(big.Int).SetString(sl.Listing.Item.Price, 10)
	if (sl.Listing.Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY &&
		sl.Listing.Metadata.ContractType != pb.Listing_Metadata_CLASSIFIED) &&
		price.Cmp(big.NewInt(0)) == 0 {
		return errors.New("zero price listings are not allowed")
	}
	if sl.Listing.Metadata.ContractType == pb.Listing_Metadata_CLASSIFIED && sl.Listing.ShippingProfile != nil {
		return errors.New("classified listings can not have shipping")
	}
	if len(sl.Listing.Item.Title) > TitleMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.title", strconv.Itoa(TitleMaxCharacters)}
	}
	if len(sl.Listing.Item.Description) > DescriptionMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.description", strconv.Itoa(DescriptionMaxCharacters)}
	}
	if len(sl.Listing.Item.ProcessingTime) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.processingtime", strconv.Itoa(SentenceMaxCharacters)}
	}
	if len(sl.Listing.Item.Tags) > MaxTags {
		return fmt.Errorf("number of tags exceeds the max of %d", MaxTags)
	}
	for _, tag := range sl.Listing.Item.Tags {
		if tag == "" {
			return errors.New("tags must not be empty")
		}
		if len(tag) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.tags", strconv.Itoa(WordMaxCharacters)}
		}
	}
	if len(sl.Listing.Item.Images) == 0 {
		return coreiface.ErrMissingField("item.images")
	}
	if len(sl.Listing.Item.Images) > MaxListItems {
		return coreiface.ErrTooManyItems{"item.images", strconv.Itoa(MaxListItems)}
	}
	for _, img := range sl.Listing.Item.Images {
		if err := validateImageHashes(img); err != nil {
			return err
		}
		if img.Filename == "" {
			return errors.New("image file names must not be nil")
		}
		if len(img.Filename) > FilenameMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.images.filename", strconv.Itoa(FilenameMaxCharacters)}
		}
		if len(img.Alt) > SentenceMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.images.alt", strconv.Itoa(SentenceMaxCharacters)}
		}
	}
	if len(sl.Listing.Item.ProductType) > WordMaxCharacters*2 {
		return coreiface.ErrTooManyCharacters{"item.productType", strconv.Itoa(WordMaxCharacters * 2)}
	}

	maxCombos := 1
	optionMap := make(map[string]map[string]struct{})
	for _, option := range sl.Listing.Item.Options {
		if _, ok := optionMap[option.Name]; ok {
			return errors.New("option names must be unique")
		}
		if option.Name == "" {
			return coreiface.ErrMissingField("item.options.name")
		}
		if len(option.Variants) < 2 {
			return errors.New("options must have more than one variants")
		}
		if len(option.Name) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.options.name", strconv.Itoa(WordMaxCharacters)}
		}
		if len(option.Description) > SentenceMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.options.description", strconv.Itoa(SentenceMaxCharacters)}
		}
		if len(option.Variants) > MaxListItems {
			return coreiface.ErrTooManyItems{"item.options.variants", strconv.Itoa(MaxListItems)}
		}
		varMap := make(map[string]struct{})
		for _, variant := range option.Variants {
			if _, ok := varMap[variant.Name]; ok {
				return errors.New("variant names must be unique")
			}
			if len(variant.Name) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"item.options.variants.name", strconv.Itoa(WordMaxCharacters)}
			}
			if hasAnyImageRef(variant.Image) {
				if err := validateImageHashes(variant.Image); err != nil {
					return err
				}
				if variant.Image.Filename == "" {
					return coreiface.ErrMissingField("items.options.variants.image.file")
				}
				if len(variant.Image.Filename) > FilenameMaxCharacters {
					return coreiface.ErrTooManyCharacters{"item.options.variants.image.filename", strconv.Itoa(FilenameMaxCharacters)}
				}
			}
			varMap[variant.Name] = struct{}{}
		}
		maxCombos *= len(option.Variants)
		optionMap[option.Name] = varMap
	}

	if len(sl.Listing.Item.Skus) > maxCombos {
		return errors.New("more skus than variant combinations")
	}
	comboMap := make(map[string]bool)
	for _, sku := range sl.Listing.Item.Skus {
		if maxCombos > 1 && len(sku.Selections) == 0 {
			return errors.New("skus must specify a variant combo when options are used")
		}
		if len(sku.ProductID) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"item.sku.productID", strconv.Itoa(WordMaxCharacters)}
		}
		formatted, err := json.Marshal(sku.Selections)
		if err != nil {
			return err
		}
		_, ok := comboMap[string(formatted)]
		if !ok {
			comboMap[string(formatted)] = true
		} else {
			return errors.New("duplicate sku")
		}
		expectedSelectionCount := 0
		for _, option := range sl.Listing.Item.Options {
			if len(option.Variants) > 0 {
				expectedSelectionCount++
			}
		}
		if len(sku.Selections) != expectedSelectionCount {
			return errors.New("incorrect number of variants in sku combination")
		}
		for _, selection := range sku.Selections {
			variantMap, ok := optionMap[selection.Option]
			if !ok {
				return errors.New("sku option not listed in listing")
			}
			if _, ok := variantMap[selection.Variant]; !ok {
				return errors.New("sku variant not listed in option")
			}
		}
	}
	if len(sl.Listing.Item.Price) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.price", strconv.Itoa(SentenceMaxCharacters)}
	}
	if sl.Listing.Metadata.Format != pb.Listing_Metadata_MARKET_PRICE {
		_, ok := new(big.Int).SetString(sl.Listing.Item.Price, 10)
		if !ok {
			return errors.New("invalid item price")
		}
	}

	if len(sl.Listing.Taxes) > MaxListItems {
		return coreiface.ErrTooManyItems{"taxes", strconv.Itoa(MaxListItems)}
	}
	for _, tax := range sl.Listing.Taxes {
		if tax.TaxType == "" {
			return coreiface.ErrMissingField("taxes.taxtype")
		}
		if len(tax.TaxType) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"taxes.taxtype", strconv.Itoa(WordMaxCharacters)}
		}
		if len(tax.TaxRegions) == 0 {
			return errors.New("tax must specify at least one region")
		}
		if len(tax.TaxRegions) > MaxCountryCodes {
			return fmt.Errorf("number of tax regions is greater than the max of %d", MaxCountryCodes)
		}
		if tax.Percentage == 0 || tax.Percentage > 100 {
			return errors.New("tax percentage must be between 0 and 100")
		}
	}

	if len(sl.Listing.Moderators) > MaxListItems {
		return coreiface.ErrTooManyItems{"moderators", strconv.Itoa(MaxListItems)}
	}
	for _, moderator := range sl.Listing.Moderators {
		_, err := peer.Decode(moderator)
		if err != nil {
			return errors.New("moderator IDs must be valid")
		}
	}

	if sl.Listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
		err := validatePhysicalListing(sl.Listing)
		if err != nil {
			return err
		}
	} else if sl.Listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		err := s.validateCryptocurrencyListing(sl.Listing)
		if err != nil {
			return err
		}
	}

	if sl.Listing.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
		err := validateMarketPriceListing(sl.Listing)
		if err != nil {
			return err
		}
	}

	if sl.Listing.VendorID == nil {
		return coreiface.ErrMissingField("vendorID")
	}
	if len(sl.Listing.VendorID.Handle) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"vendorID.handle", strconv.Itoa(SentenceMaxCharacters)}
	}
	if sl.Listing.VendorID.Pubkeys == nil {
		return coreiface.ErrMissingField("vendorID.pubkeys")
	}
	identityPubkey, err := crypto.UnmarshalPublicKey(sl.Listing.VendorID.Pubkeys.Identity)
	if err != nil {
		return errors.New("invalid vendor identity public key")
	}
	peerID, err := peer.IDFromPublicKey(identityPubkey)
	if err != nil {
		return fmt.Errorf("%w: %s", coreiface.ErrInternalServer, err)
	}
	if peerID.String() != sl.Listing.VendorID.PeerID {
		return errors.New("vendor peerID does not match public key")
	}
	if len(sl.Listing.VendorID.Pubkeys.Escrow) != 33 {
		return errors.New("vendor escrow pubkey invalid length")
	}
	ecPubkey, err := btcec.ParsePubKey(sl.Listing.VendorID.Pubkeys.Escrow)
	if err != nil {
		return errors.New("invalid vendor escrow public key")
	}
	sigParsed, err := ecdsa.ParseSignature(sl.Listing.VendorID.Sig)
	if err != nil {
		return errors.New("invalid vendor identity signature")
	}
	idHash := sha256.Sum256([]byte(sl.Listing.VendorID.PeerID))
	valid := sigParsed.Verify(idHash[:], ecPubkey)
	if !valid {
		return errors.New("invalid secp256k1 signature on vendor identity key")
	}

	m := protojson.MarshalOptions{
		EmitUnpopulated: false,
	}
	ser := m.Format(sl.Listing)

	var out bytes.Buffer
	err = json.Indent(&out, []byte(ser), "", "")
	if err != nil {
		return fmt.Errorf("%w: %s", coreiface.ErrInternalServer, err)
	}
	valid, err = identityPubkey.Verify(out.Bytes(), sl.Signature)
	if err != nil {
		return fmt.Errorf("%w: %s", coreiface.ErrInternalServer, err)
	}
	if !valid {
		return errors.New("invalid signature on listing")
	}

	return nil
}

// validateListingDraft applies minimal validation for draft listings:
// structural checks only (slug, title, metadata existence), skipping
// business-required fields like images, price, and shipping profile.
func (s *ListingAppService) validateListingDraft(sl *pb.SignedListing) error {
	if sl.Listing.Slug == "" {
		return coreiface.ErrMissingField("slug")
	}
	if len(sl.Listing.Slug) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"slug", strconv.Itoa(SentenceMaxCharacters)}
	}
	if strings.Contains(sl.Listing.Slug, " ") {
		return errors.New("slugs cannot contain spaces")
	}
	if strings.Contains(sl.Listing.Slug, "/") {
		return errors.New("slugs cannot contain file separators")
	}
	if sl.Listing.Item == nil {
		return coreiface.ErrMissingField("item")
	}
	if sl.Listing.Item.Title == "" {
		return coreiface.ErrMissingField("item.title")
	}
	if sl.Listing.Metadata == nil {
		return coreiface.ErrMissingField("metadata")
	}
	if sl.Listing.VendorID == nil {
		return coreiface.ErrMissingField("vendorID")
	}
	for _, img := range sl.Listing.Item.Images {
		if err := validateImageHashes(img); err != nil {
			return err
		}
	}
	for _, option := range sl.Listing.Item.Options {
		for _, variant := range option.Variants {
			if hasAnyImageRef(variant.Image) {
				if err := validateImageHashes(variant.Image); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *ListingAppService) deserializeAndValidateListing(listingBytes []byte, c cid.Cid) (*pb.SignedListing, error) {
	signedListing := new(pb.SignedListing)
	if err := (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(listingBytes, signedListing); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	if err := s.ValidateListing(signedListing); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	signedListing.Cid = c.String()
	return signedListing, nil
}

func (s *ListingAppService) validateCryptocurrencyListing(listing *pb.Listing) error {
	switch {
	case len(listing.Item.Options) > 0:
		return coreiface.ErrCryptocurrencyListingIllegalField("item.options")
	case listing.ShippingProfile != nil:
		return coreiface.ErrCryptocurrencyListingIllegalField("shippingProfile")
	case len(listing.Item.Condition) > 0:
		return coreiface.ErrCryptocurrencyListingIllegalField("item.condition")
	}
	return nil
}

func validatePhysicalListing(listing *pb.Listing) error {
	if len(listing.Item.Condition) > SentenceMaxCharacters {
		return coreiface.ErrTooManyCharacters{"item.condition", strconv.Itoa(SentenceMaxCharacters)}
	}
	if len(listing.Item.Options) > MaxListItems {
		return fmt.Errorf("number of options is greater than the max of %d", MaxListItems)
	}

	if listing.ShippingProfile == nil || listing.ShippingProfile.ProfileID == "" {
		return coreiface.ErrMissingField("shippingProfile")
	}
	return validateShippingProfile(listing.ShippingProfile)
}

func validateMarketPriceListing(listing *pb.Listing) error {
	if listing.Item.Price != "" {
		n, _ := new(big.Int).SetString(listing.Item.Price, 10)
		if n.Cmp(big.NewInt(0)) > 0 {
			return coreiface.ErrMarketPriceListingIllegalField("item.price")
		}
	}

	if listing.Item.CryptoListingPriceModifier != 0 {
		listing.Item.CryptoListingPriceModifier = float32(int(listing.Item.CryptoListingPriceModifier*100.0)) / 100.0
	}

	if listing.Item.CryptoListingPriceModifier < PriceModifierMin ||
		listing.Item.CryptoListingPriceModifier > PriceModifierMax {
		return coreiface.ErrPriceModifierOutOfRange{
			Min: PriceModifierMin,
			Max: PriceModifierMax,
		}
	}

	return nil
}

func validateShippingProfile(profile *pb.ShippingProfile) error {
	if profile == nil {
		return coreiface.ErrMissingField("shippingprofile")
	}
	if profile.ProfileID == "" {
		return coreiface.ErrMissingField("shippingprofile.profileid")
	}
	if len(profile.LocationGroups) == 0 {
		return coreiface.ErrMissingField("shippingprofile.locationgroups")
	}
	for _, lg := range profile.LocationGroups {
		if lg == nil {
			continue
		}
		if len(lg.Zones) == 0 {
			return coreiface.ErrMissingField("shippingprofile.locationgroup.zones")
		}
		for _, zone := range lg.Zones {
			if err := validateShippingZone(zone); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateShippingZone(zone *pb.ShippingZone) error {
	if zone.Id == "" {
		return coreiface.ErrMissingField("shippingprofile.zone.id")
	}
	if len(zone.Regions) == 0 {
		return coreiface.ErrMissingField("shippingprofile.zone.regions")
	}
	if len(zone.Rates) == 0 {
		return coreiface.ErrMissingField("shippingprofile.zone.rates")
	}
	return nil
}
