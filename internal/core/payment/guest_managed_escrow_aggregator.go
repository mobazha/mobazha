// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// GuestOrderTokenPrefix identifies guest checkout orders in managed escrow watch and
// payment_observations.order_id. Must match internal/core/guest.OrderTokenPrefix.
const GuestOrderTokenPrefix = "gst_"

// GuestManagedEscrowPaymentAggregator turns confirmed PaymentObservation rows into
// guest order state transitions (Phase 3B). P2P orders use AggregatingVerifier.
type GuestManagedEscrowPaymentAggregator struct {
	db    database.Database
	guest contracts.GuestOrderService
	repo  contracts.PaymentObservationRepo
}

// NewGuestManagedEscrowPaymentAggregator wires the guest managed escrow observation bridge.
func NewGuestManagedEscrowPaymentAggregator(
	db database.Database,
	guest contracts.GuestOrderService,
	repo contracts.PaymentObservationRepo,
) *GuestManagedEscrowPaymentAggregator {
	return &GuestManagedEscrowPaymentAggregator{db: db, guest: guest, repo: repo}
}

func (a *GuestManagedEscrowPaymentAggregator) AggregateAndEmit(ctx context.Context, tenantID, orderID string) error {
	if a == nil || a.guest == nil || a.repo == nil || a.db == nil {
		return fmt.Errorf("guest managed-escrow aggregator: not configured")
	}
	if !strings.HasPrefix(orderID, GuestOrderTokenPrefix) {
		return nil
	}

	rows, err := a.repo.ListDeduplicatedConfirmed(ctx, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("guest managed-escrow aggregator: list observations for %s: %w", orderID, err)
	}
	total, err := sumObservations(rows)
	if err != nil {
		return fmt.Errorf("guest managed-escrow aggregator: sum observations for %s: %w", orderID, err)
	}

	var order models.GuestOrder
	if err := a.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderID).First(&order).Error
	}); err != nil {
		return fmt.Errorf("guest managed-escrow aggregator: load order %s: %w", orderID, err)
	}
	if !order.HasManagedEscrowGuestFundingTarget() {
		return nil
	}

	expected, err := parseGuestPaymentAmount(order.PaymentAmount)
	if err != nil {
		return fmt.Errorf("guest managed-escrow aggregator: order %s: %w", orderID, err)
	}
	if err := a.recordGuestObservedTotals(ctx, &order, total, expected); err != nil {
		return err
	}
	if total.Cmp(expected) < 0 {
		return nil
	}

	txHash := latestObservationTxHash(rows)
	if err := a.guest.HandlePaymentDetected(orderID, txHash, nil); err != nil {
		return fmt.Errorf("guest managed-escrow aggregator: payment detected for %s: %w", orderID, err)
	}
	return nil
}

func (a *GuestManagedEscrowPaymentAggregator) recordGuestObservedTotals(ctx context.Context, order *models.GuestOrder, total, expected *big.Int) error {
	if order == nil || total == nil || expected == nil {
		return nil
	}
	received := total.String()
	overpaid := ""
	if total.Cmp(expected) > 0 {
		overpaid = new(big.Int).Sub(total, expected).String()
	}
	if order.TotalReceived == received && order.OverpaidAmount == overpaid {
		return nil
	}
	order.TotalReceived = received
	order.OverpaidAmount = overpaid
	return a.db.Update(func(tx database.Tx) error {
		rows, err := tx.UpdateColumns(
			map[string]interface{}{
				"total_received":  received,
				"overpaid_amount": overpaid,
			},
			map[string]interface{}{
				"tenant_id = ?":   order.TenantID,
				"order_token = ?": order.OrderToken,
			},
			&models.GuestOrder{},
		)
		if err != nil {
			return fmt.Errorf("guest managed-escrow aggregator: save observed totals for %s: %w", order.OrderToken, err)
		}
		if rows == 0 {
			return fmt.Errorf("guest managed-escrow aggregator: save observed totals for %s: no rows updated", order.OrderToken)
		}
		return nil
	})
}

func parseGuestPaymentAmount(amount string) (*big.Int, error) {
	v, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || v.Sign() < 0 {
		return nil, fmt.Errorf("invalid payment amount %q", amount)
	}
	return v, nil
}

func latestObservationTxHash(rows []models.PaymentObservation) string {
	row, ok := latestChainTxObservation(rows)
	if !ok {
		return ""
	}
	return row.TxHash
}

// RoutingPaymentAggregator dispatches to guest or standard order aggregators by order id.
type RoutingPaymentAggregator struct {
	guest PaymentAggregator
	order PaymentAggregator
}

// NewRoutingPaymentAggregator constructs a composite PaymentAggregator.
func NewRoutingPaymentAggregator(guest, order PaymentAggregator) *RoutingPaymentAggregator {
	return &RoutingPaymentAggregator{guest: guest, order: order}
}

func (r *RoutingPaymentAggregator) AggregateAndEmit(ctx context.Context, tenantID, orderID string) error {
	if strings.HasPrefix(orderID, GuestOrderTokenPrefix) {
		return r.guest.AggregateAndEmit(ctx, tenantID, orderID)
	}
	return r.order.AggregateAndEmit(ctx, tenantID, orderID)
}
