package core

import (
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// receivingAccountService is the concrete ReceivingAccountService implementation.
// Pure DB CRUD — no multiwallet, no escrow, no heavy dependencies.
type receivingAccountService struct {
	db     database.Database
	nodeID string
}

// NewReceivingAccountService constructs a ReceivingAccountService.
func NewReceivingAccountService(db database.Database, nodeID string) *receivingAccountService {
	return &receivingAccountService{db: db, nodeID: nodeID}
}

func (n *MobazhaNode) initReceivingAccountService() {
	n.receivingAccountService = NewReceivingAccountService(n.db, n.nodeID)
}

func (s *receivingAccountService) findExistingByAddress(tx database.Tx, chainType iwallet.ChainType, address string) (*models.ReceivingAccount, error) {
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

func (s *receivingAccountService) checkNameExists(tx database.Tx, name string, excludeID int) error {
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

func (s *receivingAccountService) deactivateOtherAccounts(tx database.Tx, chainType iwallet.ChainType, excludeID int) error {
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

func (s *receivingAccountService) saveAccount(tx database.Tx, account *models.ReceivingAccount, isUpdate bool) error {
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
	action := "created"
	if isUpdate {
		action = "updated"
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Receiving account %s: ID=%d, name=%s, chain=%s, address=%s",
		action, account.ID, account.Name, account.ChainType, account.Address)
	return nil
}

func (s *receivingAccountService) Add(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	if err := account.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Update(func(tx database.Tx) error {
		existing, err := s.findExistingByAddress(tx, account.ChainType, account.Address)
		if err != nil {
			return err
		}
		if existing != nil {
			account.ID = existing.ID
		}
		return s.saveAccount(tx, account, existing != nil)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *receivingAccountService) Update(account *models.ReceivingAccount) (*models.ReceivingAccount, error) {
	if err := account.ValidateForUpdate(); err != nil {
		return nil, err
	}
	err := s.db.Update(func(tx database.Tx) error {
		var existing models.ReceivingAccount
		if err := tx.Read().Where("id = ?", account.ID).First(&existing).Error; err != nil {
			return err
		}
		if account.Address != existing.Address {
			other, err := s.findExistingByAddress(tx, account.ChainType, account.Address)
			if err != nil {
				return err
			}
			if other != nil && other.ID != account.ID {
				return errors.New("address already used by another account")
			}
		}
		return s.saveAccount(tx, account, true)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *receivingAccountService) Delete(id int) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("id", id, nil, &models.ReceivingAccount{})
	})
}

func (s *receivingAccountService) List() ([]models.ReceivingAccount, error) {
	var records []models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *receivingAccountService) GetByID(id int) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", id).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *receivingAccountService) GetActive(chainType iwallet.ChainType) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *receivingAccountService) GetByChain(chainType iwallet.ChainType) ([]models.ReceivingAccount, error) {
	var records []models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ?", chainType).Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *receivingAccountService) GetAcceptedCurrencies() ([]string, error) {
	records, err := s.List()
	if err != nil {
		return nil, err
	}
	var currencies []string
	for _, record := range records {
		currencies = append(currencies, record.AcceptedCurrencies()...)
	}
	return currencies, nil
}
