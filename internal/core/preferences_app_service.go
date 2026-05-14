package core

import (
	"fmt"
	"os"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PreferencesAppService encapsulates user preferences and block-list management.
type PreferencesAppService struct {
	db         database.Database
	banChecker contracts.BanChecker
}

type PreferencesAppServiceConfig struct {
	DB         database.Database
	BanChecker contracts.BanChecker
}

func NewPreferencesAppService(cfg PreferencesAppServiceConfig) *PreferencesAppService {
	return &PreferencesAppService{
		db:         cfg.DB,
		banChecker: cfg.BanChecker,
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

func (s *PreferencesAppService) SavePreferences(prefs *models.UserPreferences, done chan struct{}) error {
	if _, err := prefs.StoreModerators(); err != nil {
		return fmt.Errorf("%w: invalid moderator ID", coreiface.ErrBadRequest)
	}

	// DG-1.11: cap the per-store digital-good review window override.
	// 0 = use ContractType default (3d). Values >0 are honoured by
	// ResolvePolicyForOrder ONLY when they extend (>= default) the window.
	// Reject anything above the protocol-wide ceiling to prevent abusive
	// long holds that would erode buyer trust.
	if prefs.DigitalGoodReviewWindowDays > models.MaxDigitalGoodReviewWindowDays {
		return fmt.Errorf("%w: digitalGoodReviewWindowDays must be between 0 and %d",
			coreiface.ErrBadRequest, models.MaxDigitalGoodReviewWindowDays)
	}

	shippingProfiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return fmt.Errorf("%w: invalid shipping profiles", coreiface.ErrBadRequest)
	}

	if len(shippingProfiles) == 0 {
		currentPrefs, err := s.GetPreferences()
		if err == nil {
			currentShippingProfiles, _ := currentPrefs.GetShippingProfiles()
			if len(currentShippingProfiles) > 0 {
				physicalGoodsCount, err := s.countPhysicalGoods()
				if err != nil {
					maybeCloseDone(done)
					return fmt.Errorf("failed to check physical goods: %w", err)
				}
				if physicalGoodsCount > 0 {
					maybeCloseDone(done)
					return fmt.Errorf("%w: cannot remove all shipping profiles while %d physical goods exist. Please delete or convert those listings first", coreiface.ErrBadRequest, physicalGoodsCount)
				}
			}
		}
	}

	err = s.db.Update(func(tx database.Tx) error {
		_, err := prefs.BlockedNodes()
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
		return tx.Save(prefs)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	maybeCloseDone(done)
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

	if s.banChecker != nil {
		s.banChecker.AddBlockedID(pid)
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

	if s.banChecker != nil {
		s.banChecker.RemoveBlockedID(pid)
	}

	return removeFromBlock, err
}

func (s *PreferencesAppService) countPhysicalGoods() (int, error) {
	var count int
	err := s.db.View(func(tx database.Tx) error {
		index, err := tx.GetListingIndex()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, listing := range index {
			if listing.ContractType == "PHYSICAL_GOOD" {
				count++
			}
		}
		return nil
	})
	return count, err
}
