//go:build !private_distribution

package payment

import (
	"errors"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

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
