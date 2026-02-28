package core

import (
	"errors"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

const maxWishlistItems = 200


// WishlistAppService encapsulates wishlist business logic.
type WishlistAppService struct {
	db     database.Database
	nodeID string
}

// WishlistAppServiceConfig holds dependencies for WishlistAppService.
type WishlistAppServiceConfig struct {
	DB     database.Database
	NodeID string
}

// NewWishlistAppService creates a new WishlistAppService.
func NewWishlistAppService(cfg WishlistAppServiceConfig) *WishlistAppService {
	return &WishlistAppService{
		db:     cfg.DB,
		nodeID: cfg.NodeID,
	}
}

// GetWishlist returns all wishlist items for the current tenant, ordered by creation time (newest first).
func (s *WishlistAppService) GetWishlist() ([]models.WishlistItem, error) {
	var items []models.WishlistItem
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Order("created_at DESC").Find(&items).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to get wishlist: %v", err)
		return nil, err
	}
	return items, nil
}

// AddToWishlist adds a product to the wishlist with snapshot info for display.
// Idempotent: re-adding an existing item is a no-op (snapshot is NOT updated).
func (s *WishlistAppService) AddToWishlist(item models.WishlistItem) (*models.WishlistItem, error) {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.WishlistItem{}).Count(&count).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Failed to count wishlist items: %v", err)
		return nil, err
	}
	if count >= maxWishlistItems {
		return nil, contracts.ErrWishlistFull
	}

	var result models.WishlistItem
	err = s.db.Update(func(tx database.Tx) error {
		var existing models.WishlistItem
		findErr := tx.Read().
			Where("vendor_peer_id = ? AND slug = ?", item.VendorPeerID, item.Slug).
			First(&existing).Error
		if findErr == nil {
			result = existing
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if saveErr := tx.Save(&item); saveErr != nil {
			return saveErr
		}
		result = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// RemoveFromWishlist removes a product from the wishlist. No-op if item doesn't exist.
func (s *WishlistAppService) RemoveFromWishlist(vendorPeerID, slug string) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("vendor_peer_id", vendorPeerID, map[string]interface{}{"slug = ?": slug}, &models.WishlistItem{})
	})
}

// WishlistCount returns the number of items in the wishlist.
func (s *WishlistAppService) WishlistCount() (int, error) {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.WishlistItem{}).Count(&count).Error
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
