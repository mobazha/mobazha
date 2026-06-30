package guest

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/redact"
)

// ManagedEscrowGuestSettlementSource projects private guest-order state into
// immutable, chain-neutral settlement requests for a trusted distribution.
type ManagedEscrowGuestSettlementSource struct {
	db database.Database
}

// NewManagedEscrowGuestSettlementSource constructs a Core-owned source.
func NewManagedEscrowGuestSettlementSource(db database.Database) *ManagedEscrowGuestSettlementSource {
	return &ManagedEscrowGuestSettlementSource{db: db}
}

// ManagedEscrowGuestSettlement returns a request only when the order is
// eligible and no active deployment or settlement action already exists.
func (s *ManagedEscrowGuestSettlementSource) ManagedEscrowGuestSettlement(
	ctx context.Context,
	orderID string,
) (*distribution.ManagedEscrowGuestSettlementRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("managed escrow guest settlement: database not configured")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("managed escrow guest settlement: order ID is required")
	}

	var order models.GuestOrder
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderID).First(&order).Error
	}); err != nil {
		return nil, fmt.Errorf("managed escrow guest settlement: load order %s: %w", orderID, err)
	}
	if !order.HasEVMManagedEscrowFundingTarget() || !guestOrderEligibleForSettlement(order.State) {
		return nil, nil
	}
	hasActive, err := s.hasActiveAction(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, nil
	}

	meta, err := RecoverGuestEVMManagedEscrowMetadata(&order)
	if err != nil {
		return nil, fmt.Errorf("managed escrow guest settlement: recover metadata for %s: %w", orderID, err)
	}
	coinInfo, err := validateGuestEVMManagedEscrowMetadataCoin(meta)
	if err != nil {
		return nil, err
	}
	predicted, err := PredictGuestEVMManagedEscrowAddress(meta)
	if err != nil {
		return nil, fmt.Errorf("managed escrow guest settlement: predict address for %s: %w", orderID, err)
	}
	escrowAddress := common.HexToAddress(meta.ManagedEscrowAddress)
	if escrowAddress == (common.Address{}) || escrowAddress != predicted {
		return nil, fmt.Errorf("managed escrow guest settlement: invalid escrow address for %s", orderID)
	}
	owner := common.HexToAddress(meta.SellerOwnerAddress)
	recipient := common.HexToAddress(meta.SettlementRecipient)
	if owner == (common.Address{}) || recipient == (common.Address{}) {
		return nil, fmt.Errorf("managed escrow guest settlement: owner and recipient are required for %s", orderID)
	}
	salt, ok := new(big.Int).SetString(strings.TrimSpace(meta.SaltNonce), 10)
	if !ok || salt.Sign() < 0 {
		return nil, fmt.Errorf("managed escrow guest settlement: invalid salt nonce for %s", orderID)
	}
	amount := guestEVMManagedEscrowSettlementAmount(&order)
	parsedAmount, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || parsedAmount.Sign() <= 0 {
		return nil, fmt.Errorf("managed escrow guest settlement: invalid payment amount for %s", orderID)
	}

	return &distribution.ManagedEscrowGuestSettlementRequest{
		OrderID:       orderID,
		Chain:         coinInfo.Chain,
		ChainID:       meta.ChainID,
		PaymentCoin:   meta.Coin,
		PaymentAmount: parsedAmount.String(),
		EscrowAddress: escrowAddress.Hex(),
		OwnerAddress:  owner.Hex(),
		SaltNonce:     salt.String(),
		Recipient:     recipient.Hex(),
	}, nil
}

// ListPendingManagedEscrowGuestSettlements returns all currently eligible
// requests. Invalid persisted metadata fails closed instead of reaching a
// private chain implementation.
func (s *ManagedEscrowGuestSettlementSource) ListPendingManagedEscrowGuestSettlements(
	ctx context.Context,
) ([]distribution.ManagedEscrowGuestSettlementRequest, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("managed escrow guest settlement: database not configured")
	}
	var orders []models.GuestOrder
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state IN ?", guestSettlementEligibleStates).
			Where("evm_managed_escrow_metadata IS NOT NULL AND evm_managed_escrow_metadata <> ''").
			Find(&orders).Error
	}); err != nil {
		return nil, fmt.Errorf("managed escrow guest settlement: list pending orders: %w", err)
	}
	requests := make([]distribution.ManagedEscrowGuestSettlementRequest, 0, len(orders))
	for i := range orders {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request, err := s.ManagedEscrowGuestSettlement(ctx, orders[i].OrderToken)
		if err != nil {
			return nil, err
		}
		if request != nil {
			requests = append(requests, *request)
		}
	}
	return requests, nil
}

// ListConfirmedManagedEscrowGuestSettlements returns confirmed order IDs so
// Core can replay entitlement emission after a crash.
func (s *ManagedEscrowGuestSettlementSource) ListConfirmedManagedEscrowGuestSettlements(
	ctx context.Context,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("managed escrow guest settlement: database not configured")
	}
	var rows []models.SettlementAction
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("action_kind = ? AND state = ?", payment.ManagedEscrowGuestSettlementAction, "confirmed").
			Find(&rows).Error
	}); err != nil {
		return nil, fmt.Errorf("managed escrow guest settlement: list confirmed actions: %w", err)
	}
	seen := make(map[string]struct{}, len(rows))
	orderIDs := make([]string, 0, len(rows))
	for i := range rows {
		orderID := strings.TrimSpace(rows[i].OrderID)
		if orderID == "" {
			continue
		}
		if _, ok := seen[orderID]; ok {
			continue
		}
		seen[orderID] = struct{}{}
		orderIDs = append(orderIDs, orderID)
	}
	return orderIDs, nil
}

func (s *ManagedEscrowGuestSettlementSource) hasActiveAction(ctx context.Context, orderID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.SettlementAction{}).
			Where("order_id = ?", orderID).
			Where(
				"(action_kind = ? AND state IN ?) OR (action_kind = ? AND state IN ?)",
				payment.ManagedEscrowGuestSettlementAction, guestReleaseActiveStates,
				payment.ManagedEscrowGuestDeployAction, guestDeployActiveStates,
			).
			Count(&count).Error
	})
	if err != nil {
		return false, fmt.Errorf("managed escrow guest settlement: inspect actions for %s: %w", orderID, err)
	}
	return count > 0, nil
}

// DistributionManagedEscrowGuestSettlementService keeps Core orchestration
// separate from the private chain executor.
type DistributionManagedEscrowGuestSettlementService struct {
	source   distribution.ManagedEscrowGuestSettlementSource
	executor distribution.ManagedEscrowGuestSettlementExecutor
}

// NewDistributionManagedEscrowGuestSettlementService constructs the bridge.
func NewDistributionManagedEscrowGuestSettlementService(
	source distribution.ManagedEscrowGuestSettlementSource,
	executor distribution.ManagedEscrowGuestSettlementExecutor,
) *DistributionManagedEscrowGuestSettlementService {
	return &DistributionManagedEscrowGuestSettlementService{source: source, executor: executor}
}

// SubmitReleaseForOrder delegates only a validated immutable request.
func (s *DistributionManagedEscrowGuestSettlementService) SubmitReleaseForOrder(ctx context.Context, orderID string) error {
	if s == nil || s.source == nil || s.executor == nil {
		return fmt.Errorf("managed escrow guest settlement: source and executor are required")
	}
	request, err := s.source.ManagedEscrowGuestSettlement(ctx, orderID)
	if err != nil || request == nil {
		return err
	}
	return s.executor.SubmitManagedEscrowGuestSettlement(ctx, *request)
}

// RecoverPendingSettlements retries every currently eligible request.
func (s *DistributionManagedEscrowGuestSettlementService) RecoverPendingSettlements(ctx context.Context) {
	if s == nil || s.source == nil || s.executor == nil {
		return
	}
	requests, err := s.source.ListPendingManagedEscrowGuestSettlements(ctx)
	if err != nil {
		log.Warningf("managed escrow guest settlement recovery: %v", err)
		return
	}
	for i := range requests {
		if err := s.executor.SubmitManagedEscrowGuestSettlement(ctx, requests[i]); err != nil {
			log.Warningf("managed escrow guest settlement recovery for %s: %v",
				redact.Token(requests[i].OrderID), err)
		}
	}
}

var _ distribution.ManagedEscrowGuestSettlementSource = (*ManagedEscrowGuestSettlementSource)(nil)
