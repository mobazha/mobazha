package core

import (
	"encoding/json"
	"errors"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// ShoppingCartAppService encapsulates shopping cart business logic.
type ShoppingCartAppService struct {
	db       database.Database
	eventBus events.Bus
	nodeID   string
}

type ShoppingCartAppServiceConfig struct {
	DB       database.Database
	EventBus events.Bus
	NodeID   string
}

func NewShoppingCartAppService(cfg ShoppingCartAppServiceConfig) *ShoppingCartAppService {
	return &ShoppingCartAppService{
		db:       cfg.DB,
		eventBus: cfg.EventBus,
		nodeID:   cfg.NodeID,
	}
}

func (s *ShoppingCartAppService) GetCarts() ([]models.StoreCart, error) {
	var cartRecords []models.StoreCartRecord
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&cartRecords).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.LogErrorWithIDf(log, s.nodeID, "Find shopping cart items failed, %v", err)
		return []models.StoreCart{}, err
	}

	carts := []models.StoreCart{}
	for _, cartRecord := range cartRecords {
		var cartItems []models.ShoppingCartItem
		if err := json.Unmarshal(cartRecord.Items, &cartItems); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "unmarshal purchase items failed, peerID: %s, %v", cartRecord.VendorID, err)
			continue
		}
		carts = append(carts, models.StoreCart{
			VendorID: cartRecord.VendorID,
			Items:    cartItems,
		})
	}

	return carts, nil
}

func (s *ShoppingCartAppService) GetCartsTotalItemsCount() (int, error) {
	carts, err := s.GetCarts()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, cart := range carts {
		total += len(cart.Items)
	}
	return total, nil
}

func (s *ShoppingCartAppService) AddToCart(vendorID peer.ID, inputItem models.ShoppingCartItem) error {
	cartRecord := models.StoreCartRecord{
		VendorID: vendorID.String(),
	}
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().First(&cartRecord).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.LogErrorWithIDf(log, s.nodeID, "Find shopping cart by vendorID %s failed, %v", vendorID, err)
		return err
	}

	var cartItems []models.ShoppingCartItem
	if len(cartRecord.Items) > 0 {
		if err := json.Unmarshal(cartRecord.Items, &cartItems); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "unmarshal cartItems failed, peerID: %s, %v", vendorID, err)
			return err
		}
	}

	found := false
	resultItems := []models.ShoppingCartItem{}
	for _, item := range cartItems {
		if item.IsSamePurchaseItem(&inputItem) {
			found = true
			resultItems = append(resultItems, inputItem)
		} else {
			resultItems = append(resultItems, item)
		}
	}
	if !found {
		resultItems = append(resultItems, inputItem)
	}

	err = s.db.Update(func(tx database.Tx) error {
		itemsByte, err := json.MarshalIndent(resultItems, "", "    ")
		if err != nil {
			return fmt.Errorf("marshal purchase items failed, %v", err)
		}
		cartRecord.Items = itemsByte
		return tx.Save(&cartRecord)
	})

	total, _ := s.GetCartsTotalItemsCount()
	s.eventBus.Emit(&events.ShoppingCartUpdate{
		ItemsCount: total,
	})

	return err
}

func (s *ShoppingCartAppService) RemoveCartItem(vendorID peer.ID, inputItem models.ShoppingCartItem) error {
	cartRecord := models.StoreCartRecord{
		VendorID: vendorID.String(),
	}
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().First(&cartRecord).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Find shopping cart by vendorID %s failed, %v", vendorID, err)
		return err
	}

	var cartItems []models.ShoppingCartItem
	if len(cartRecord.Items) > 0 {
		if err := json.Unmarshal(cartRecord.Items, &cartItems); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "unmarshal cartItems failed, peerID: %s, %v", vendorID, err)
			return err
		}
	}

	found := false
	resultItems := []models.ShoppingCartItem{}
	for _, item := range cartItems {
		if item.IsSamePurchaseItem(&inputItem) {
			found = true
		} else {
			resultItems = append(resultItems, item)
		}
	}
	if !found {
		logger.LogWarningWithIDf(log, s.nodeID, "the purchase item doesn't exist in shopping cart")
		return nil
	}

	err = s.db.Update(func(tx database.Tx) error {
		if len(resultItems) > 0 {
			itemsByte, err := json.MarshalIndent(resultItems, "", "    ")
			if err != nil {
				return fmt.Errorf("marshal purchase items failed, %v", err)
			}
			cartRecord.Items = itemsByte
			return tx.Save(&cartRecord)
		}
		return tx.Delete("vendor_id", cartRecord.VendorID, nil, models.StoreCartRecord{})
	})

	total, _ := s.GetCartsTotalItemsCount()
	s.eventBus.Emit(&events.ShoppingCartUpdate{
		ItemsCount: total,
	})

	return err
}

func (s *ShoppingCartAppService) ClearCarts(vendorID peer.ID) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("vendor_id", vendorID.String(), nil, &models.StoreCartRecord{})
	})
}

func (s *ShoppingCartAppService) ClearAllCarts() error {
	carts, err := s.GetCarts()
	if err != nil {
		return err
	}
	return s.db.Update(func(tx database.Tx) error {
		for _, cart := range carts {
			tx.Delete("vendor_id", cart.VendorID, nil, &models.StoreCartRecord{})
		}
		return nil
	})
}
