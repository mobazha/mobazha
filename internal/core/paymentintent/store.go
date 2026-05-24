package paymentintent

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertSharedPaymentIntent persists the canonical shared funding route for a
// business order. Empty fields are treated as "preserve existing" for partial
// updates such as refund-address writes.
func UpsertSharedPaymentIntent(gdb *gorm.DB, orderID string, paymentAddress string, refundAddress string, info *models.PendingManagedEscrowPaymentInfo) error {
	if gdb == nil {
		return fmt.Errorf("shared payment intent: db is nil")
	}
	gdb = gdb.Session(&gorm.Session{NewDB: true})
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fmt.Errorf("shared payment intent: orderID is required")
	}

	intent := &models.SharedPaymentIntent{
		OrderID:        models.OrderID(orderID),
		PaymentAddress: strings.TrimSpace(paymentAddress),
		RefundAddress:  strings.TrimSpace(refundAddress),
	}
	if info != nil {
		if err := intent.SetPendingManagedEscrowPaymentInfo(info); err != nil {
			return err
		}
	}

	updates := map[string]interface{}{}
	if intent.PaymentAddress != "" {
		updates["payment_address"] = intent.PaymentAddress
	}
	if intent.RefundAddress != "" {
		updates["refund_address"] = intent.RefundAddress
	}
	if len(intent.PendingPaymentInfo) > 0 {
		updates["pending_payment_info"] = intent.PendingPaymentInfo
	}
	if len(updates) == 0 {
		return nil
	}

	return gdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(intent).Error
}

// UpsertSharedPaymentPolicySnapshot persists the StorePolicy decision used to
// provision the shared payment intent. Empty moderator values are allowed for
// cancelable orders; revision zero means no StorePolicy-backed moderator was
// selected.
func UpsertSharedPaymentPolicySnapshot(gdb *gorm.DB, orderID string, moderatorPeerID string, storePolicyRevision uint64) error {
	if gdb == nil {
		return fmt.Errorf("shared payment intent policy snapshot: db is nil")
	}
	gdb = gdb.Session(&gorm.Session{NewDB: true})
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fmt.Errorf("shared payment intent policy snapshot: orderID is required")
	}
	if strings.TrimSpace(moderatorPeerID) == "" && storePolicyRevision == 0 {
		return nil
	}

	intent := &models.SharedPaymentIntent{
		OrderID:             models.OrderID(orderID),
		ModeratorPeerID:     strings.TrimSpace(moderatorPeerID),
		StorePolicyRevision: storePolicyRevision,
	}
	return gdb.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"moderator_peer_id":     intent.ModeratorPeerID,
			"store_policy_revision": intent.StorePolicyRevision,
		}),
	}).Create(intent).Error
}

// LoadSharedPaymentIntent returns the tenant-less shared payment intent for an
// order. A missing row is not an error.
func LoadSharedPaymentIntent(gdb *gorm.DB, orderID string) (*models.SharedPaymentIntent, error) {
	if gdb == nil {
		return nil, fmt.Errorf("shared payment intent: db is nil")
	}
	gdb = gdb.Session(&gorm.Session{NewDB: true})
	var intent models.SharedPaymentIntent
	if err := gdb.Where("order_id = ?", strings.TrimSpace(orderID)).First(&intent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &intent, nil
}

// HydrateOrderFromSharedIntent fills missing ManagedEscrow route fields from the
// canonical shared intent row when present.
func HydrateOrderFromSharedIntent(gdb *gorm.DB, order *models.Order) error {
	if gdb == nil || order == nil {
		return nil
	}
	intent, err := LoadSharedPaymentIntent(gdb, order.ID.String())
	if err != nil || intent == nil {
		return err
	}
	return intent.HydrateOrder(order)
}
