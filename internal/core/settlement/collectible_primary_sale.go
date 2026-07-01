package settlement

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ExecuteCollectiblePrimarySaleRelease releases the seller side of a verified
// collectible primary-sale escrow after hosting has confirmed Hub intake.
func (s *SettlementService) ExecuteCollectiblePrimarySaleRelease(
	ctx context.Context,
	orderID models.OrderID,
	payoutAddr string,
) (*payment.ActionResult, iwallet.CoinType, error) {
	if s == nil || s.db == nil {
		return nil, "", fmt.Errorf("database not initialized")
	}
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, "", err
	}

	if err := validateCollectiblePrimarySaleReleaseOrder(&order); err != nil {
		return nil, "", err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil, "", fmt.Errorf("%w: payment settlement spec is missing", coreiface.ErrBadRequest)
	}
	if spec.GetMethod() != pb.PaymentSent_CANCELABLE {
		return nil, "", fmt.Errorf("%w: collectible primary-sale release requires a cancelable escrow", coreiface.ErrBadRequest)
	}
	// The paid-order hook and the reconciliation worker can race. If another
	// execution already confirmed the order, the seller-side escrow release is
	// complete and the collectible release must remain idempotent.
	if order.SerializedOrderConfirmation != nil {
		coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
		if err != nil {
			return nil, "", err
		}
		return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
	}

	return s.executeSettlementActionForOrder(ctx, payment.SettlementActionConfirm, &order, payoutAddr)
}

func validateCollectiblePrimarySaleReleaseOrder(order *models.Order) error {
	if order == nil {
		return fmt.Errorf("%w: order is required", coreiface.ErrBadRequest)
	}
	if order.Role() != models.RoleVendor {
		return fmt.Errorf("%w: collectible primary-sale release requires the seller node", coreiface.ErrBadRequest)
	}
	if !order.IsPaymentVerified() {
		return fmt.Errorf("%w: collectible primary-sale release requires verified payment", coreiface.ErrBadRequest)
	}
	fiatMeta, err := order.GetFiatMetadata()
	if err != nil {
		return err
	}
	meta, ok := models.CollectibleOrderMetadataFromFiatMetadata(fiatMeta)
	if !ok || meta.HubSlotID == "" {
		return fmt.Errorf("%w: order is not a collectible primary sale", coreiface.ErrBadRequest)
	}
	return nil
}
