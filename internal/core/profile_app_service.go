package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"gorm.io/gorm"
)

type GetAcceptedCurrenciesFunc func() ([]string, error)

type ProfileAppService struct {
	db              database.Database
	contentStore    ContentStorePort
	fetchIPNSRecord FetchIPNSRecordFunc
	publish         PublishFunc
	netDB           *netdb.NetDB
	nodeID          string
	peerID          peer.ID

	escrowPubKeyHex        string
	ethPubKeyHex           string
	solanaPubKeyStr        string
	stripeAccountID        string
	storeAndForwardServers []string

	coTenantPublicData    contracts.CoTenantPublicDataFn
	getAcceptedCurrencies GetAcceptedCurrenciesFunc
}

type ProfileAppServiceConfig struct {
	DB              database.Database
	ContentStore    ContentStorePort
	FetchIPNSRecord FetchIPNSRecordFunc
	Publish         PublishFunc
	NetDB           *netdb.NetDB
	NodeID          string
	PeerID          peer.ID

	EscrowPubKeyHex        string
	ETHPubKeyHex           string
	SolanaPubKeyStr        string
	StripeAccountID        string
	StoreAndForwardServers []string

	CoTenantPublicData    contracts.CoTenantPublicDataFn
	GetAcceptedCurrencies GetAcceptedCurrenciesFunc
}

func NewProfileAppService(cfg ProfileAppServiceConfig) *ProfileAppService {
	return &ProfileAppService{
		db:                     cfg.DB,
		contentStore:           cfg.ContentStore,
		fetchIPNSRecord:        cfg.FetchIPNSRecord,
		publish:                cfg.Publish,
		netDB:                  cfg.NetDB,
		nodeID:                 cfg.NodeID,
		peerID:                 cfg.PeerID,
		escrowPubKeyHex:        cfg.EscrowPubKeyHex,
		ethPubKeyHex:           cfg.ETHPubKeyHex,
		solanaPubKeyStr:        cfg.SolanaPubKeyStr,
		stripeAccountID:        cfg.StripeAccountID,
		storeAndForwardServers: cfg.StoreAndForwardServers,
		coTenantPublicData:     cfg.CoTenantPublicData,
		getAcceptedCurrencies:  cfg.GetAcceptedCurrencies,
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

	if err := validateProfile(profile); err != nil {
		if done != nil {
			close(done)
		}
		return fmt.Errorf("%w: %s", coreiface.ErrBadRequest, err)
	}

	var enabledCoins []string
	if s.getAcceptedCurrencies != nil {
		enabledCoins, _ = s.getAcceptedCurrencies()
	}

	err := s.db.Update(func(tx database.Tx) error {
		var prefs models.UserPreferences
		if err := tx.Read().First(&prefs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		currencies, err := prefs.PreferredCurrencies()
		if err != nil {
			return err
		}
		if len(currencies) == 0 {
			currencies = append(currencies, enabledCoins...)
		}

		if len(profile.Currencies) == 0 {
			profile.Currencies = currencies
		}

		if profile.Moderator && profile.ModeratorInfo != nil {
			profile.ModeratorInfo.AcceptedCurrencies = profile.Currencies
		}

		if err := s.updateProfileStats(tx, profile); err != nil {
			return err
		}
		if err := tx.SetProfile(profile); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	go func() {
		if s.netDB != nil {
			if err = s.netDB.SetOwnProfile(profile); err != nil {
				logger.LogDebugWithIDf(log, s.nodeID, "Failed to set profile to netdb, err: %s", err)
			}
		}
	}()

	if s.publish != nil {
		s.publish(done)
	} else {
		maybeCloseDone(done)
	}
	return nil
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

func (s *ProfileAppService) GetProfile(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error) {
	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if profile, err := pd.GetProfile(); err == nil {
				return profile, nil
			}
		}
	}

	getDatafromIPNS := func() (*models.Profile, error) {
		if s.fetchIPNSRecord == nil {
			return nil, fmt.Errorf("IPNS resolver not available")
		}
		record, err := s.fetchIPNSRecord(ctx, peerID, useCache)
		if err != nil {
			return nil, err
		}
		pth, err := record.Value()
		if err != nil {
			return nil, err
		}
		pth1, err := path.Join(pth, ffsqlite.ProfileFile)
		if err != nil {
			return nil, err
		}
		profileBytes, err := s.contentStore.Cat(ctx, pth1.String())
		if err != nil {
			return nil, err
		}
		profile := new(models.Profile)
		if err := json.Unmarshal(profileBytes, profile); err != nil {
			return nil, err
		}
		if err := validateProfile(profile); err != nil {
			return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
		}
		if len(profile.StoreAndForwardServers) > 0 {
			err := s.db.Update(func(tx database.Tx) error {
				pi := models.StoreAndForwardServers{
					PeerID:      peerID.String(),
					LastUpdated: time.Now(),
				}
				if err := pi.PutServers(profile.StoreAndForwardServers); err != nil {
					return err
				}
				return tx.Save(&pi)
			})
			if err != nil {
				return nil, err
			}
		}
		return profile, nil
	}

	if preferIPNS || s.netDB == nil {
		profile, err := getDatafromIPNS()
		if err != nil && s.netDB != nil {
			logger.LogDebugWithIDf(log, s.nodeID, "Failed to get profile from p2p network: %s, error: %s", peerID, err)
			return s.netDB.GetProfile(peerID.String(), reqCtx)
		}
		return profile, err
	}

	profile, err := s.netDB.GetProfile(peerID.String(), reqCtx)
	if err != nil {
		return getDatafromIPNS()
	}

	return profile, err
}

// UpdateAndSaveProfile loads the profile from disk, updates stats, and saves.
// Exposed for cross-domain callers (FollowAppService, PostsAppService).
func (s *ProfileAppService) UpdateAndSaveProfile(tx database.Tx) error {
	profile, err := tx.GetProfile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if profile == nil {
		return nil
	}
	if err := s.updateProfileStats(tx, profile); err != nil {
		return err
	}
	return tx.SetProfile(profile)
}

func (s *ProfileAppService) updateProfileStats(tx database.Tx, profile *models.Profile) error {
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

			if err := s.updateProfileStats(tx, profile); err != nil {
				return err
			}
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

// ContentStorePort is the subset of contracts.ContentStore needed by profile/follow services.
type ContentStorePort interface {
	Cat(ctx context.Context, contentPath string) ([]byte, error)
}

// validateProfile checks each field to make sure they're formatted properly and/or
// within the desired limits.
func validateProfile(profile *models.Profile) error {
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
		if (profile.ModeratorInfo.Fee.FeeType == models.FixedFee || profile.ModeratorInfo.Fee.FeeType == models.FixedPlusPercentageFee) && profile.ModeratorInfo.Fee.FixedFee == nil {
			return errors.New("moderator fee type must be set if using fixed fee or fixed plus percentage")
		}
		if len(profile.ModeratorInfo.Description) > AboutMaxCharacters {
			return coreiface.ErrTooManyCharacters{"moderatorinfo.description", strconv.Itoa(AboutMaxCharacters)}
		}
		if len(profile.ModeratorInfo.TermsAndConditions) > PolicyMaxCharacters {
			return coreiface.ErrTooManyCharacters{"moderatorinfo.termsandconditions", strconv.Itoa(PolicyMaxCharacters)}
		}
		if len(profile.ModeratorInfo.Languages) > MaxListItems {
			return coreiface.ErrTooManyItems{"moderatorinfo.languages", strconv.Itoa(MaxListItems)}
		}
		for _, l := range profile.ModeratorInfo.Languages {
			if len(l) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.languages", strconv.Itoa(WordMaxCharacters)}
			}
		}
		for _, l := range profile.ModeratorInfo.AcceptedCurrencies {
			if len(l) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.acceptedCurrencies", strconv.Itoa(WordMaxCharacters)}
			}
		}
		if len(profile.ModeratorInfo.AcceptedCurrencies) > MaxListItems {
			return coreiface.ErrTooManyItems{"moderatorinfo.acceptedCurrencies"}
		}
		if profile.ModeratorInfo.Fee.FixedFee != nil {
			if len(profile.ModeratorInfo.Fee.FixedFee.Currency.Name) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.name", strconv.Itoa(WordMaxCharacters)}
			}
			if len(string(profile.ModeratorInfo.Fee.FixedFee.Currency.CurrencyType)) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.currencytype", strconv.Itoa(WordMaxCharacters)}
			}
			if len(profile.ModeratorInfo.Fee.FixedFee.Currency.Code.String()) > WordMaxCharacters {
				return coreiface.ErrTooManyCharacters{"moderatorinfo.fee.fixedfee.currency.code", strconv.Itoa(WordMaxCharacters)}
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
	if len(profile.StoreAndForwardServers) > MaxListItems {
		return coreiface.ErrTooManyItems{"storeAndForwardServers"}
	}
	for _, pid := range profile.StoreAndForwardServers {
		_, err := peer.Decode(pid)
		if err != nil {
			return errors.New("invalid snf server peerID")
		}
	}
	if len(profile.EscrowPublicKey) != 66 {
		return fmt.Errorf("bad request: secp256k1 public key must be exactly %d hex characters, got %d", 66, len(profile.EscrowPublicKey))
	}
	if profile.Stats != nil {
		if profile.Stats.AverageRating > 5 {
			return fmt.Errorf("average rating cannot be greater than %d", 5)
		}
	}
	return nil
}
