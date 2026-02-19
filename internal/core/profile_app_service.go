package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ipfs/boxo/path"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
	"github.com/mobazha/mobazha3.0/internal/logger"
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
