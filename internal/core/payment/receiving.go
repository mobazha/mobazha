//go:build !private_distribution

package payment

import (
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// ── Receiving Account Management ────────────────────────────────

func (s *PaymentAppService) findExistingAccountByAddress(tx database.Tx, chainType iwallet.ChainType, address string) (*models.ReceivingAccount, error) {
	var existing models.ReceivingAccount
	err := tx.Read().Where("chain_type = ? AND address = ?", chainType, address).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func (s *PaymentAppService) checkNameExists(tx database.Tx, name string, excludeID int) error {
	var nameCheck models.ReceivingAccount
	query := tx.Read().Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.First(&nameCheck).Error; err == nil {
		return errors.New("name already used by another account")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func (s *PaymentAppService) deactivateOtherAccounts(tx database.Tx, chainType iwallet.ChainType, excludeID int) error {
	var otherRecords []models.ReceivingAccount
	query := tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Find(&otherRecords).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	for i := range otherRecords {
		otherRecords[i].IsActive = false
		if err := tx.Save(&otherRecords[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *PaymentAppService) saveReceivingAccount(tx database.Tx, account *models.ReceivingAccount, isUpdate bool) error {
	if err := account.NormalizeActiveTokens(); err != nil {
		return fmt.Errorf("normalize active tokens: %w", err)
	}
	if err := s.checkNameExists(tx, account.Name, account.ID); err != nil {
		return err
	}
	if account.IsActive {
		if err := s.deactivateOtherAccounts(tx, account.ChainType, account.ID); err != nil {
			return err
		}
	}
	if err := tx.Save(account); err != nil {
		return err
	}
	action := "创建"
	if isUpdate {
		action = "更新"
	}
	logger.LogInfoWithIDf(log, s.nodeID, "%s收款账户成功: ID=%d, 名称=%s, 链类型=%s, 地址=%s",
		action, account.ID, account.Name, account.ChainType, account.Address)
	return nil
}

func (s *PaymentAppService) AddReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	if err := account.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Update(func(tx database.Tx) error {
		existing, err := s.findExistingAccountByAddress(tx, account.ChainType, account.Address)
		if err != nil {
			return err
		}
		if existing != nil {
			account.ID = existing.ID
		}
		return s.saveReceivingAccount(tx, account, existing != nil)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *PaymentAppService) UpdateReceivingAccount(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	if err := account.ValidateForUpdate(); err != nil {
		return nil, err
	}
	err := s.db.Update(func(tx database.Tx) error {
		var existing models.ReceivingAccount
		if err := tx.Read().Where("id = ?", account.ID).First(&existing).Error; err != nil {
			return err
		}
		if account.Address != existing.Address {
			other, err := s.findExistingAccountByAddress(tx, account.ChainType, account.Address)
			if err != nil {
				return err
			}
			if other != nil && other.ID != account.ID {
				return errors.New("address already used by another account")
			}
		}
		return s.saveReceivingAccount(tx, account, true)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *PaymentAppService) GetReceivingAccounts() ([]models.ReceivingAccount, error) {
	var records []models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *PaymentAppService) GetAcceptedCurrencies() ([]string, error) {
	records, err := s.GetReceivingAccounts()
	if err != nil {
		return nil, err
	}
	var currencies []string
	for _, record := range records {
		currencies = append(currencies, record.AcceptedCurrencies()...)
	}
	return currencies, nil
}

func (s *PaymentAppService) GetReceivingAccountsByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error) {
	var records []models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ?", chainType).Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *PaymentAppService) GetActiveReceivingAccount(chainType iwallet.ChainType) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *PaymentAppService) DeleteReceivingAccount(id int) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("id", id, nil, &models.ReceivingAccount{})
	})
}

func (s *PaymentAppService) GetReceivingAccountByID(id int) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", id).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ── Wallet Metadata ─────────────────────────────────────────────

func (s *PaymentAppService) SaveTransactionMetadata(metadata *models.TransactionMetadata) error {
	return s.db.Update(func(tx database.Tx) error {
		var order models.Order
		err := tx.Read().Where("payment_address = ?", metadata.PaymentAddress).First(&order).Error
		if err == nil {
			metadata.OrderID = order.ID
			orderOpen, err := order.OrderOpenMessage()
			if err == nil {
				metadata.Thumbnail = orderOpen.Listings[0].Listing.Item.Images[0].Tiny
				if metadata.Memo == "" {
					metadata.Memo = orderOpen.Listings[0].Listing.Item.Title
				}
			}
		}
		return tx.Save(metadata)
	})
}

func (s *PaymentAppService) GetTransactionMetadata(txid iwallet.TransactionID) (models.TransactionMetadata, error) {
	var metadata models.TransactionMetadata
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("txid=?", txid.String()).First(&metadata).Error
	})
	return metadata, err
}

func (s *PaymentAppService) GetMnemonic() (string, error) {
	var dbSeed models.Key
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "mnemonic").First(&dbSeed).Error
	})
	return string(dbSeed.Value), err
}

// GetPayoutAddress delegates to SettlementService.
func (s *PaymentAppService) GetPayoutAddress(coinType string) (iwallet.Address, error) {
	if s.escrowOps == nil {
		return iwallet.Address{}, errors.New("settlement service not initialized")
	}
	return s.escrowOps.GetPayoutAddress(coinType)
}
