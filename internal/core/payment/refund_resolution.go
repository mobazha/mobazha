package payment

import (
	"errors"

	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// LoadLocalRefundReceivingAddresses reads buyer default refund destinations from
// the local node's UserPreferences singleton.
func LoadLocalRefundReceivingAddresses(db database.Database) (map[string]string, error) {
	if db == nil {
		return nil, nil
	}

	var prefs models.UserPreferences
	err := db.View(func(tx database.Tx) error {
		err := tx.Read().First(&prefs).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return prefs.RefundReceivingAddresses()
}

// ResolveBuyerRefundForLocalNode resolves a buyer refund address using local DB
// preferences when the order role is buyer. Seller nodes never read local prefs.
func ResolveBuyerRefundForLocalNode(
	db database.Database,
	order *models.Order,
	paymentSent *pb.PaymentSent,
	coin iwallet.CoinType,
	observations []models.PaymentObservation,
	payFromCustodial bool,
) paymentpkg.RefundResolveResult {
	var prefs map[string]string
	if order != nil && order.Role() == models.RoleBuyer {
		loaded, loadErr := LoadLocalRefundReceivingAddresses(db)
		if loadErr != nil {
			nodeID := order.ID.String()
			logger.LogWarningWithIDf(log, nodeID, "Failed to load local refund receiving addresses: %v", loadErr)
		} else {
			prefs = loaded
		}
	}

	return paymentpkg.RefundResolveRequest{
		Order:                  order,
		PaymentSent:            paymentSent,
		Coin:                   coin,
		Observations:           observations,
		PayFromCustodial:       payFromCustodial,
		LocalRefundPreferences: prefs,
	}.Resolve()
}

// loadLocalRefundReceivingPreferencesGorm reads buyer defaults inside an open
// GORM transaction (payment verifier path).
func loadLocalRefundReceivingPreferencesGorm(gdb *gorm.DB) (map[string]string, error) {
	if gdb == nil {
		return nil, nil
	}
	var prefs models.UserPreferences
	if err := gdb.First(&prefs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return prefs.RefundReceivingAddresses()
}
