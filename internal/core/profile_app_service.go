package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	solana "github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/netdb"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/request"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type ProfileAppService struct {
	db       database.Database
	publish  PublishFunc
	eventBus events.Bus
	netDB    *netdb.NetDB
	nodeID   string
	peerID   peer.ID

	escrowPubKeyHex        string
	ethPubKeyHex           string
	solanaPubKeyStr        string
	stripeAccountID        string
	storeAndForwardServers []string
	walletAccounts         contracts.WalletAccountService

	coTenantPublicData contracts.CoTenantPublicDataFn
}

type ProfileAppServiceConfig struct {
	DB       database.Database
	Publish  PublishFunc
	EventBus events.Bus
	NetDB    *netdb.NetDB
	NodeID   string
	PeerID   peer.ID

	EscrowPubKeyHex        string
	ETHPubKeyHex           string
	SolanaPubKeyStr        string
	StripeAccountID        string
	StoreAndForwardServers []string
	WalletAccounts         contracts.WalletAccountService

	CoTenantPublicData contracts.CoTenantPublicDataFn
}

func NewProfileAppService(cfg ProfileAppServiceConfig) *ProfileAppService {
	return &ProfileAppService{
		db:                     cfg.DB,
		publish:                cfg.Publish,
		eventBus:               cfg.EventBus,
		netDB:                  cfg.NetDB,
		nodeID:                 cfg.NodeID,
		peerID:                 cfg.PeerID,
		escrowPubKeyHex:        cfg.EscrowPubKeyHex,
		ethPubKeyHex:           cfg.ETHPubKeyHex,
		solanaPubKeyStr:        cfg.SolanaPubKeyStr,
		stripeAccountID:        cfg.StripeAccountID,
		storeAndForwardServers: cfg.StoreAndForwardServers,
		walletAccounts:         cfg.WalletAccounts,
		coTenantPublicData:     cfg.CoTenantPublicData,
	}
}

func (s *ProfileAppService) SetProfile(profile *models.Profile, done chan<- struct{}) error {
	profile.EscrowPublicKey = s.escrowPubKeyHex
	profile.ETHPublicKey = s.ethPubKeyHex
	profile.SolanaPublicKey = s.solanaPubKeyStr
	profile.StripeAccountID = s.stripeAccountID
	profile.PeerID = s.peerID.String()
	profile.LastModified = time.Now()
	profile.StoreAndForwardServers = s.storeAndForwardServers
	profile.PayoutDestinationSet = models.PayoutDestinationSet{}
	if s.walletAccounts != nil {
		// A single unavailable rail (e.g. a wallet adapter still syncing)
		// must not block unrelated profile edits such as name or bio.
		// reserveAffiliateDestinations publishes whatever rails currently
		// succeed; a rail that fails is simply absent from the set until a
		// later save succeeds. Affiliate checkout remains fail-closed for a rail
		// whose destination is absent, without blocking unrelated profile edits.
		profile.PayoutDestinationSet = s.reserveAffiliateDestinations()
	}

	if err := validateProfile(profile); err != nil {
		if done != nil {
			close(done)
		}
		return fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
	}

	err := s.db.Update(func(tx database.Tx) error {
		return tx.SetProfile(profile)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	if s.eventBus != nil {
		s.eventBus.Emit(&events.ProfileChanged{})
	}

	if s.publish != nil {
		s.publish(done)
	} else {
		maybeCloseDone(done)
	}
	return nil
}

// reserveAffiliateDestinations publishes the best-effort set of Affiliate
// payout destinations. A rail that cannot currently reserve an address (for
// example a wallet adapter that has not finished loading) is skipped rather
// than failing the whole profile save; Hosting already treats a missing rail
// destination as "not yet available" for that specific rail, so partial
// publication keeps new Affiliate links unavailable without blocking profile edits.
func (s *ProfileAppService) reserveAffiliateDestinations() models.PayoutDestinationSet {
	chains := []iwallet.ChainType{
		iwallet.ChainBitcoin,
		iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin,
		iwallet.ChainEthereum,
		iwallet.ChainBSC,
		iwallet.ChainPolygon,
		iwallet.ChainBase,
		iwallet.ChainSolana,
	}
	destinations := make([]models.PayoutDestination, 0, len(chains))
	for _, chain := range chains {
		railID, ok := iwallet.CanonicalNativeCoinType(chain)
		if !ok {
			log.Warningf("affiliate payout destination: no canonical rail for %s", chain)
			continue
		}
		// Reservations intentionally have a tenant-wide unique reference. The
		// Affiliate payout set owns one deterministic address per rail, so the
		// reference must include that rail instead of making the first successful
		// reservation block every subsequent payout destination.
		referenceID := "affiliate:" + string(railID)
		reserved, err := s.walletAccounts.ReserveAddress(context.Background(), string(railID), contracts.AccountAffiliate, referenceID)
		if err != nil {
			// Hosted runtimes may deliberately keep payout keys outside Core.
			// Freeze the tenant's signed public key or explicitly configured active
			// receiving account instead of requiring private-key access merely to
			// publish a commission destination.
			if fallback, ok := s.configuredAffiliateDestination(chain, string(railID)); ok {
				destinations = append(destinations, fallback)
				continue
			}
			log.Warningf("affiliate payout destination: reserve %s address failed (will retry on next profile save): %v", railID, err)
			continue
		}
		destinations = append(destinations, models.PayoutDestination{
			RailID: reserved.RailID, Address: reserved.Address, Tag: reserved.Tag, Version: reserved.Version,
		})
	}
	return models.PayoutDestinationSet{Destinations: destinations}
}

func (s *ProfileAppService) configuredAffiliateDestination(chain iwallet.ChainType, railID string) (models.PayoutDestination, bool) {
	if chain == iwallet.ChainSolana {
		key, err := solana.PublicKeyFromBase58(strings.TrimSpace(s.solanaPubKeyStr))
		if err == nil && !key.IsZero() {
			return models.PayoutDestination{RailID: railID, Address: key.String(), Version: 1}, true
		}
	}
	if s == nil || s.db == nil {
		return models.PayoutDestination{}, false
	}
	var account models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", chain, true).
			Order("updated_at DESC").First(&account).Error
	})
	if err != nil || strings.TrimSpace(account.Address) == "" {
		return models.PayoutDestination{}, false
	}
	return models.PayoutDestination{RailID: railID, Address: strings.TrimSpace(account.Address), Version: 1}, true
}

func (s *ProfileAppService) GetMyProfile() (*models.Profile, error) {
	var (
		profile *models.Profile
		err     error
	)
	err = s.db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		if err != nil {
			return fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
		}
		return nil
	})
	return profile, err
}

func (s *ProfileAppService) GetProfileStats() (*models.ProfileStats, error) {
	var stats *models.ProfileStats
	err := s.db.View(func(tx database.Tx) error {
		var p models.Profile
		if err := computeProfileStats(tx, &p); err != nil {
			return err
		}
		stats = p.Stats
		return nil
	})
	return stats, err
}

func (s *ProfileAppService) GetProfile(_ context.Context, peerID peer.ID, reqCtx *request.Context, _ bool) (*models.Profile, error) {
	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if profile, err := pd.GetProfile(); err == nil {
				return profile, nil
			}
		}
	}

	if s.netDB != nil {
		return s.netDB.GetProfile(peerID.String(), reqCtx)
	}

	return nil, fmt.Errorf("profile data not available for remote peer %s", peerID)
}

func getProfileWithStats(db database.Database) (*models.Profile, error) {
	var profile *models.Profile
	err := db.View(func(tx database.Tx) error {
		var err error
		profile, err = tx.GetProfile()
		if err != nil {
			return err
		}
		_ = computeProfileStats(tx, profile)
		return nil
	})
	return profile, err
}

func computeProfileStats(tx database.Tx, profile *models.Profile) error {
	followers, err := tx.GetFollowers()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	following, err := tx.GetFollowing()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	listings, err := tx.GetListingIndex()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	physicalListingCount := 0
	digitalListingCount := 0
	serviceListingCount := 0
	cryptocurrencyListingCount := 0
	rwaTokenListingCount := 0
	for _, listing := range listings {
		switch listing.ContractType {
		case pb.Listing_Metadata_PHYSICAL_GOOD.String():
			physicalListingCount += 1
		case pb.Listing_Metadata_DIGITAL_GOOD.String():
			digitalListingCount += 1
		case pb.Listing_Metadata_SERVICE.String():
			serviceListingCount += 1
		case pb.Listing_Metadata_CRYPTOCURRENCY.String():
			cryptocurrencyListingCount += 1
		case pb.Listing_Metadata_RWA_TOKEN.String():
			rwaTokenListingCount += 1
		}
	}

	averageRating := float32(0)
	ratingTotal := float32(0)
	ratingCount := 0
	if ratings, err := tx.GetRatingIndex(); err == nil {
		for _, rating := range ratings {
			ratingTotal += float32(rating.Average * float64(rating.Count))
			ratingCount += int(rating.Count)
		}
		if ratingCount != 0 {
			averageRating = ratingTotal / float32(ratingCount)
		}
	}

	posts, _ := tx.GetPostIndex()

	profile.Stats = &models.ProfileStats{
		FollowerCount:              uint32(followers.Count()),
		FollowingCount:             uint32(following.Count()),
		ListingCount:               uint32(listings.Count()),
		PhysicalListingCount:       uint32(physicalListingCount),
		DigitalListingCount:        uint32(digitalListingCount),
		ServiceListingCount:        uint32(serviceListingCount),
		CryptocurrencyListingCount: uint32(cryptocurrencyListingCount),
		RwaTokenListingCount:       uint32(rwaTokenListingCount),
		PostCount:                  uint32(len(posts)),
		RatingCount:                uint32(ratingCount),
		AverageRating:              averageRating,
	}

	return nil
}

func (s *ProfileAppService) UpdateSNFServers() error {
	equal := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if v != b[i] {
				return false
			}
		}
		return true
	}
	updated := false
	err := s.db.Update(func(tx database.Tx) error {
		profile, err := tx.GetProfile()
		if err != nil {
			return err
		}
		if !equal(profile.StoreAndForwardServers, s.storeAndForwardServers) {
			profile.StoreAndForwardServers = s.storeAndForwardServers

			if err := tx.SetProfile(profile); err != nil {
				return err
			}

			updated = true
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && updated && s.publish != nil {
		s.publish(nil)
	}
	return nil
}

// validateProfile checks each field to make sure they're formatted properly and/or
// within the desired limits.
func validateProfile(profile *models.Profile) error {
	switch profile.Visibility {
	case models.VisibilityPublic, models.VisibilityUnlisted, models.VisibilityPrivate:
	case "":
		profile.Visibility = models.VisibilityPublic
	default:
		return fmt.Errorf("%w: visibility must be one of: public, unlisted, private", coreiface.ErrBadRequest)
	}

	if len(profile.Name) == 0 {
		return coreiface.ErrMissingField("name")
	}
	if len(profile.Name) > WordMaxCharacters {
		return coreiface.ErrTooManyCharacters{"name", strconv.Itoa(WordMaxCharacters)}
	}
	if len(profile.Location) > WordMaxCharacters {
		return coreiface.ErrTooManyCharacters{"location", strconv.Itoa(WordMaxCharacters)}
	}
	if len(profile.About) > AboutMaxCharacters {
		return coreiface.ErrTooManyCharacters{"about", strconv.Itoa(AboutMaxCharacters)}
	}
	if len(profile.ShortDescription) > models.ShortDescriptionLength {
		return coreiface.ErrTooManyCharacters{"shortdescription", strconv.Itoa(models.ShortDescriptionLength)}
	}
	if profile.ContactInfo != nil {
		if len(profile.ContactInfo.Website) > URLMaxCharacters {
			return coreiface.ErrTooManyCharacters{"contactinfo.website", strconv.Itoa(URLMaxCharacters)}
		}
		if len(profile.ContactInfo.Email) > SentenceMaxCharacters {
			return coreiface.ErrTooManyCharacters{"contactinfo.email", strconv.Itoa(SentenceMaxCharacters)}
		}
		if len(profile.ContactInfo.PhoneNumber) > WordMaxCharacters {
			return coreiface.ErrTooManyCharacters{"contactinfo.phonenumber", strconv.Itoa(SentenceMaxCharacters)}
		}
		if len(profile.ContactInfo.Social) > MaxListItems {
			return coreiface.ErrTooManyItems{"contactinfo.social", strconv.Itoa(MaxListItems)}
		}
		for _, s := range profile.ContactInfo.Social {
			if len(s.Username) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"contactinfo.social.username", strconv.Itoa(WordMaxCharacters)}
			}
			if len(s.Type) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"contactinfo.social.type", strconv.Itoa(WordMaxCharacters)}
			}
			if len(s.Proof) > URLMaxCharacters {
				return coreiface.ErrTooManyCharacters{"contactinfo.social.proof", strconv.Itoa(URLMaxCharacters)}
			}
		}
	}
	if profile.Moderator && profile.ModeratorInfo == nil {
		return errors.New("moderatorinfo must be included if moderator boolean is set")
	}
	if profile.ModeratorInfo != nil {
		if len(profile.ModeratorInfo.Description) > AboutMaxCharacters {
			return coreiface.ErrTooManyCharacters{"moderatorinfo.description", strconv.Itoa(AboutMaxCharacters)}
		}
		if len(profile.ModeratorInfo.TermsAndConditions) > PolicyMaxCharacters {
			return coreiface.ErrTooManyCharacters{"moderatorinfo.termsandconditions", strconv.Itoa(PolicyMaxCharacters)}
		}
		if len(profile.ModeratorInfo.Languages) > MaxListItems {
			return coreiface.ErrTooManyItems{"moderatorinfo.languages", strconv.Itoa(MaxListItems)}
		}
		for _, lang := range profile.ModeratorInfo.Languages {
			if len(lang) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.language", strconv.Itoa(WordMaxCharacters)}
			}
		}
		if profile.ModeratorInfo.Fee.FixedFee != nil && profile.ModeratorInfo.Fee.FixedFee.Currency != nil {
			cur := profile.ModeratorInfo.Fee.FixedFee.Currency
			if len(cur.Name) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.name", strconv.Itoa(WordMaxCharacters)}
			}
			if len(string(cur.Code)) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.code", strconv.Itoa(WordMaxCharacters)}
			}
			if len(string(cur.CurrencyType)) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.type", strconv.Itoa(WordMaxCharacters)}
			}
		}
	}
	if profile.AvatarHashes.Large != "" || profile.AvatarHashes.Medium != "" ||
		profile.AvatarHashes.Small != "" || profile.AvatarHashes.Tiny != "" || profile.AvatarHashes.Original != "" {
		_, err := cid.Decode(profile.AvatarHashes.Tiny)
		if err != nil {
			return errors.New("tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Small)
		if err != nil {
			return errors.New("small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Medium)
		if err != nil {
			return errors.New("medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Large)
		if err != nil {
			return errors.New("large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Original)
		if err != nil {
			return errors.New("original image hashes must be properly formatted CID")
		}
	}
	if profile.HeaderHashes.Large != "" || profile.HeaderHashes.Medium != "" ||
		profile.HeaderHashes.Small != "" || profile.HeaderHashes.Tiny != "" || profile.HeaderHashes.Original != "" {
		_, err := cid.Decode(profile.HeaderHashes.Tiny)
		if err != nil {
			return errors.New("tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Small)
		if err != nil {
			return errors.New("small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Medium)
		if err != nil {
			return errors.New("medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Large)
		if err != nil {
			return errors.New("large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Original)
		if err != nil {
			return errors.New("original image hashes must be properly formatted CID")
		}
	}
	if profile.Stats != nil && (profile.Stats.AverageRating < 0 || profile.Stats.AverageRating > 5) {
		return errors.New("average rating must be between 0 and 5")
	}
	if len(profile.StoreAndForwardServers) > MaxListItems {
		return coreiface.ErrTooManyItems{"storeAndForwardServers", strconv.Itoa(MaxListItems)}
	}
	for _, pid := range profile.StoreAndForwardServers {
		_, err := peer.Decode(pid)
		if err != nil {
			return errors.New("invalid snf server peerID")
		}
	}
	if profile.EscrowPublicKey != "" && len(profile.EscrowPublicKey) != 66 {
		return fmt.Errorf("bad request: secp256k1 public key must be exactly %d hex characters, got %d", 66, len(profile.EscrowPublicKey))
	}
	return nil
}
