package core

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/google/uuid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// PreferencesAppService encapsulates user preferences and block-list management.
type PreferencesAppService struct {
	db         database.Database
	banManager *obnet.BanManager

	// Cross-domain callbacks injected at construction time.
	UpdateAllListingsFunc func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	GetMyListingsFunc     func() (models.ListingIndex, error)
}

type PreferencesAppServiceConfig struct {
	DB                    database.Database
	BanManager            *obnet.BanManager
	UpdateAllListingsFunc func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
	GetMyListingsFunc     func() (models.ListingIndex, error)
}

func NewPreferencesAppService(cfg PreferencesAppServiceConfig) *PreferencesAppService {
	return &PreferencesAppService{
		db:                    cfg.DB,
		banManager:            cfg.BanManager,
		UpdateAllListingsFunc: cfg.UpdateAllListingsFunc,
		GetMyListingsFunc:     cfg.GetMyListingsFunc,
	}
}

func (s *PreferencesAppService) GetPreferences() (*models.UserPreferences, error) {
	var prefs models.UserPreferences
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().First(&prefs).Error
	})
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (s *PreferencesAppService) MigrateShippingOptionsToProfiles(done chan<- struct{}) error {
	prefs, err := s.GetPreferences()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			maybeCloseDone(done)
			return nil
		}
		maybeCloseDone(done)
		return err
	}

	if !prefs.NeedsMigrationFromLegacy() {
		maybeCloseDone(done)
		return nil
	}

	profileID := uuid.New().String()
	profileName := "Default Shipping"
	locationID := uuid.New().String()
	locationName := "Default Location"

	err = prefs.MigrateFromLegacyShippingOptions(profileID, profileName, locationID, locationName)
	if err != nil {
		maybeCloseDone(done)
		return fmt.Errorf("failed to migrate shipping options to profiles: %w", err)
	}

	err = s.db.Update(func(tx database.Tx) error {
		prefs.ID = 1
		return tx.Save(prefs)
	})
	if err != nil {
		maybeCloseDone(done)
		return fmt.Errorf("failed to save migrated preferences: %w", err)
	}

	defaultProfile, err := prefs.GetDefaultShippingProfile()
	if err != nil || defaultProfile == nil {
		maybeCloseDone(done)
		return nil
	}

	pbShippingProfile := models.ConvertShippingProfileToProto(defaultProfile)

	return s.UpdateAllListingsFunc(func(l *pb.Listing) (bool, error) {
		if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			l.ShippingProfile = pbShippingProfile
			return true, nil
		}
		return false, nil
	}, done)
}

func (s *PreferencesAppService) CheckAndMigrateShippingProfiles() error {
	prefs, err := s.GetPreferences()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if prefs.NeedsMigrationFromLegacy() {
		log.Info("Migrating shipping options to shipping profiles...")
		err := s.MigrateShippingOptionsToProfiles(nil)
		if err != nil {
			log.Errorf("Failed to migrate shipping profiles: %v", err)
			return err
		}
		log.Info("Shipping profiles migration completed (publishing in background)")
	}
	return nil
}

func (s *PreferencesAppService) SyncShippingProfilesToListings() error {
	prefs, err := s.GetPreferences()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	shippingProfiles, err := prefs.GetShippingProfiles()
	if err != nil || len(shippingProfiles) == 0 {
		return nil
	}

	var defaultProfile *models.ShippingProfile
	for _, profile := range shippingProfiles {
		if profile.IsDefault {
			defaultProfile = profile
			break
		}
	}
	if defaultProfile == nil {
		defaultProfile = shippingProfiles[0]
	}

	allZones := defaultProfile.GetAllZones()
	if len(allZones) == 0 {
		return nil
	}

	pbDefaultProfile := models.ConvertShippingProfileToProto(defaultProfile)

	done := make(chan struct{})
	go func() {
		<-done
	}()

	var updated int
	err = s.UpdateAllListingsFunc(func(l *pb.Listing) (bool, error) {
		if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			if l.ShippingProfile == nil || l.ShippingProfile.ProfileID == "" {
				l.ShippingProfile = pbDefaultProfile
				l.ShippingOptions = nil
				updated++
				return true, nil
			}
		}
		return false, nil
	}, done)

	if err != nil {
		return err
	}

	if updated > 0 {
		log.Infof("Synced shipping profile to %d physical goods", updated)
	}

	return nil
}

func (s *PreferencesAppService) SavePreferences(prefs *models.UserPreferences, done chan struct{}) error {
	var modsChanged bool
	mods, err := prefs.StoreModerators()
	if err != nil {
		return fmt.Errorf("%w: invalid moderator ID", coreiface.ErrBadRequest)
	}

	var shippingOptionsChanged bool
	shippingOptions, err := prefs.GetShippingOptions()
	if err != nil {
		return fmt.Errorf("%w: invalid shipping options", coreiface.ErrBadRequest)
	}

	var shippingProfilesChanged bool
	shippingProfiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return fmt.Errorf("%w: invalid shipping profiles", coreiface.ErrBadRequest)
	}

	profileMap := make(map[string]*models.ShippingProfile)
	for _, profile := range shippingProfiles {
		profileMap[profile.ProfileID] = profile
	}

	if len(shippingOptions) == 0 && len(shippingProfiles) == 0 {
		currentPrefs, err := s.GetPreferences()
		if err == nil {
			currentShippingOptions, _ := currentPrefs.GetShippingOptions()
			currentShippingProfiles, _ := currentPrefs.GetShippingProfiles()
			if len(currentShippingOptions) > 0 || len(currentShippingProfiles) > 0 {
				physicalGoodsCount, err := s.countPhysicalGoods()
				if err != nil {
					maybeCloseDone(done)
					return fmt.Errorf("failed to check physical goods: %w", err)
				}
				if physicalGoodsCount > 0 {
					maybeCloseDone(done)
					return fmt.Errorf("%w: cannot remove all shipping options/profiles while %d physical goods exist. Please delete or convert those listings first", coreiface.ErrBadRequest, physicalGoodsCount)
				}
			}
		}
	}

	err = s.db.Update(func(tx database.Tx) error {
		var (
			currentPrefs  models.UserPreferences
			currentModMap = make(map[peer.ID]bool)
			newModMap     = make(map[peer.ID]bool)
		)
		err := tx.Read().First(&currentPrefs).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		currentMods, err := currentPrefs.StoreModerators()
		if err != nil {
			return err
		}
		for _, mod := range currentMods {
			currentModMap[mod] = true
		}

		for _, mod := range mods {
			newModMap[mod] = true
			if !currentModMap[mod] {
				modsChanged = true
			}
		}
		for _, mod := range currentMods {
			if !newModMap[mod] {
				modsChanged = true
			}
		}

		shippingOptionsChanged = !bytes.Equal(prefs.ShippingOptions, currentPrefs.ShippingOptions)
		if shippingOptionsChanged {
			latestID := 0
			currentOptions, _ := currentPrefs.GetShippingOptions()
			for _, option := range currentOptions {
				if latestID < option.ID {
					latestID = option.ID
				}
			}

			for i, option := range shippingOptions {
				if option.ID <= 0 {
					latestID += 1
					option.ID = latestID
					shippingOptions[i] = option
				}
			}
		}

		shippingProfilesChanged = !bytes.Equal(prefs.ShippingProfiles, currentPrefs.ShippingProfiles)

		_, err = prefs.BlockedNodes()
		if err != nil {
			return fmt.Errorf("%w: invalid block node ID", coreiface.ErrBadRequest)
		}

		currencies, err := prefs.PreferredCurrencies()
		if err != nil {
			return err
		}
		for _, cur := range currencies {
			if !iwallet.IsValidCoinType(iwallet.CoinType(cur)) {
				return fmt.Errorf("%w: currency %s is not valid", coreiface.ErrBadRequest, cur)
			}
		}
		prefs.ID = 1
		if err := tx.Save(prefs); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	if modsChanged || shippingOptionsChanged || shippingProfilesChanged {
		modStrs := make([]string, 0, len(mods))
		for _, mod := range mods {
			modStrs = append(modStrs, mod.String())
		}
		pbShippingOptions := models.ConvertShippingOptions(shippingOptions)

		var defaultProfile *models.ShippingProfile
		for _, profile := range shippingProfiles {
			if profile.IsDefault {
				defaultProfile = profile
				break
			}
		}
		if defaultProfile == nil && len(shippingProfiles) > 0 {
			defaultProfile = shippingProfiles[0]
		}

		return s.UpdateAllListingsFunc(func(l *pb.Listing) (bool, error) {
			if modsChanged {
				l.Moderators = modStrs
			}
			if l.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
				if shippingProfilesChanged {
					if l.ShippingProfile != nil && l.ShippingProfile.ProfileID != "" {
						if profile, ok := profileMap[l.ShippingProfile.ProfileID]; ok {
							l.ShippingProfile = models.ConvertShippingProfileToProto(profile)
						}
					} else if defaultProfile != nil {
						l.ShippingProfile = models.ConvertShippingProfileToProto(defaultProfile)
						l.ShippingOptions = nil
					}
				} else if shippingOptionsChanged && l.ShippingProfile == nil {
					l.ShippingOptions = pbShippingOptions
				}
			}
			return true, nil
		}, done)
	}

	return nil
}

func (s *PreferencesAppService) BlockNode(peerID string) (bool, error) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return false, err
	}

	prefs, err := s.GetPreferences()
	if err != nil {
		return false, err
	}

	addedToBlock := false
	err = s.db.Update(func(tx database.Tx) error {
		addedToBlock, err = prefs.AddBlockedNode(peerID)
		if err != nil {
			return err
		}
		return tx.Save(prefs)
	})
	if err != nil {
		return addedToBlock, err
	}

	if s.banManager != nil {
		s.banManager.AddBlockedID(pid)
	}

	return addedToBlock, err
}

func (s *PreferencesAppService) UnblockNode(peerID string) (bool, error) {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return false, err
	}

	prefs, err := s.GetPreferences()
	if err != nil {
		return false, err
	}

	removeFromBlock := false
	err = s.db.Update(func(tx database.Tx) error {
		removeFromBlock, err = prefs.RemoveBlockedNode(peerID)
		if err != nil {
			return err
		}
		return tx.Save(prefs)
	})
	if err != nil {
		return removeFromBlock, err
	}

	if s.banManager != nil {
		s.banManager.RemoveBlockedID(pid)
	}

	return removeFromBlock, err
}

func (s *PreferencesAppService) countPhysicalGoods() (int, error) {
	if s.GetMyListingsFunc == nil {
		return 0, fmt.Errorf("GetMyListingsFunc not configured")
	}
	index, err := s.GetMyListingsFunc()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, listing := range index {
		if listing.ContractType == "PHYSICAL_GOOD" {
			count++
		}
	}
	return count, nil
}
