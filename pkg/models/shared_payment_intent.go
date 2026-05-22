package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SharedPaymentIntent stores the peer-shared payment intent for a business
// order exactly once, outside any tenant-scoped mirror row.
//
// In multi-tenant hosting, buyer/vendor order rows are projections of the same
// commercial order. Fields that represent the shared payment route itself
// (predicted ManagedEscrow address, buyer refund target, pending monitored-payment
// metadata) belong to the business order, not to any single tenant mirror.
//
// The row is intentionally tenant-less and keyed only by OrderID so both
// mirrors can converge on the same funding contract without copying state from
// sibling order rows.
type SharedPaymentIntent struct {
	OrderID OrderID `gorm:"primaryKey;column:order_id"`

	PaymentAddress string `gorm:"column:payment_address;type:text"`
	RefundAddress  string `gorm:"column:refund_address;type:text"`

	// PendingPaymentInfo currently stores ManagedEscrow monitored-payment metadata using
	// the same discriminator-based JSON format as Order.PendingPaymentInfo.
	// Keeping the wire format aligned lets the projection be future-extended to
	// other payment rails without another schema hop.
	PendingPaymentInfo []byte `gorm:"column:pending_payment_info"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SetPendingManagedEscrowPaymentInfo stores ManagedEscrow payment info in PendingPaymentInfo.
func (s *SharedPaymentIntent) SetPendingManagedEscrowPaymentInfo(info *PendingManagedEscrowPaymentInfo) error {
	if s == nil {
		return fmt.Errorf("shared payment intent is nil")
	}
	if info == nil {
		s.PendingPaymentInfo = nil
		return nil
	}
	info.Type = "managed_escrow"
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal shared pending managed escrow payment info: %w", err)
	}
	s.PendingPaymentInfo = data
	return nil
}

// GetPendingManagedEscrowPaymentInfo loads ManagedEscrow payment info from PendingPaymentInfo.
func (s *SharedPaymentIntent) GetPendingManagedEscrowPaymentInfo() (*PendingManagedEscrowPaymentInfo, error) {
	if s == nil || len(s.PendingPaymentInfo) == 0 {
		return nil, nil
	}
	var hint struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(s.PendingPaymentInfo, &hint); err != nil {
		return nil, fmt.Errorf("unmarshal shared pending payment info type: %w", err)
	}
	if hint.Type != "managed_escrow" {
		return nil, nil
	}
	var info PendingManagedEscrowPaymentInfo
	if err := json.Unmarshal(s.PendingPaymentInfo, &info); err != nil {
		return nil, fmt.Errorf("unmarshal shared pending managed escrow payment info: %w", err)
	}
	return &info, nil
}

// HydrateOrder fills missing shared payment-route fields onto the order copy.
// Existing order fields always win.
func (s *SharedPaymentIntent) HydrateOrder(order *Order) error {
	if s == nil || order == nil {
		return nil
	}
	if strings.TrimSpace(order.PaymentAddress) == "" && strings.TrimSpace(s.PaymentAddress) != "" {
		order.PaymentAddress = strings.TrimSpace(s.PaymentAddress)
	}
	if strings.TrimSpace(order.RefundAddress) == "" && strings.TrimSpace(s.RefundAddress) != "" {
		order.RefundAddress = strings.TrimSpace(s.RefundAddress)
	}
	if info, err := s.GetPendingManagedEscrowPaymentInfo(); err != nil {
		return err
	} else if info != nil {
		existing, err := order.GetPendingManagedEscrowPaymentInfo()
		if err != nil {
			return fmt.Errorf("read order pending managed escrow payment info: %w", err)
		}
		if existing == nil {
			if err := order.SetPendingManagedEscrowPaymentInfo(info); err != nil {
				return err
			}
		}
	}
	return nil
}
