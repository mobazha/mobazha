package guest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	"gorm.io/gorm"
)

const (
	guestSettlementIntentVersion = "v1"
	guestSettlementLeaseDuration = 10 * time.Minute
	guestSettlementRecoveryEvery = time.Minute
	guestSettlementWorkers       = 4
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

// ClaimManagedEscrowGuestSettlement returns a request only after atomically
// acquiring the durable execution lease for the order's deterministic intent.
func (s *ManagedEscrowGuestSettlementSource) ClaimManagedEscrowGuestSettlement(
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

	request := &distribution.ManagedEscrowGuestSettlementRequest{
		IntentID:      managedEscrowGuestSettlementIntentID(orderID),
		OrderID:       orderID,
		Chain:         coinInfo.Chain,
		ChainID:       meta.ChainID,
		PaymentCoin:   meta.Coin,
		PaymentAmount: parsedAmount.String(),
		EscrowAddress: escrowAddress.Hex(),
		OwnerAddress:  owner.Hex(),
		SaltNonce:     salt.String(),
		Recipient:     recipient.Hex(),
	}
	claimed, err := s.claimSettlementIntent(ctx, request)
	if err != nil || !claimed {
		return nil, err
	}
	return request, nil
}

// ListPendingManagedEscrowGuestSettlementOrderIDs returns candidates only.
// Each candidate is validated and atomically claimed by workers separately so
// one corrupt row cannot poison the entire recovery batch.
func (s *ManagedEscrowGuestSettlementSource) ListPendingManagedEscrowGuestSettlementOrderIDs(
	ctx context.Context,
) ([]string, error) {
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
	orderIDs := make([]string, 0, len(orders))
	for i := range orders {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		orderID := strings.TrimSpace(orders[i].OrderToken)
		if orderID != "" {
			orderIDs = append(orderIDs, orderID)
		}
	}
	return orderIDs, nil
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

func managedEscrowGuestSettlementIntentID(orderID string) string {
	digest := sha256.Sum256([]byte(guestSettlementIntentVersion + "\x00" + payment.ManagedEscrowGuestSettlementAction + "\x00" + strings.TrimSpace(orderID)))
	return "guest-release-" + hex.EncodeToString(digest[:])
}

func newGuestSettlementClaimToken() (string, error) {
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate claim token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func (s *ManagedEscrowGuestSettlementSource) claimSettlementIntent(
	ctx context.Context,
	request *distribution.ManagedEscrowGuestSettlementRequest,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	token, err := newGuestSettlementClaimToken()
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	leaseUntil := now.Add(guestSettlementLeaseDuration)
	immutable := *request
	immutable.ClaimToken = ""
	raw, err := json.Marshal(immutable)
	if err != nil {
		return false, fmt.Errorf("managed escrow guest settlement: encode intent: %w", err)
	}
	row := models.SettlementAction{
		ActionID: request.IntentID, IntentKey: request.IntentID, OrderID: request.OrderID,
		ActionKind: payment.ManagedEscrowGuestSettlementAction, ChainID: request.ChainID,
		IntentPayload: string(raw), State: "claimed", Attempts: 1, LeaseToken: token,
		LeaseExpiresAt: &leaseUntil, SettlementCoin: request.PaymentCoin,
		GrossAmount: request.PaymentAmount, CreatedAt: now, UpdatedAt: now,
	}
	err = s.db.Update(func(tx database.Tx) error { return tx.Create(&row) })
	if err == nil {
		request.ClaimToken = token
		return true, nil
	}

	var existing models.SettlementAction
	if loadErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", request.IntentID).First(&existing).Error
	}); loadErr != nil {
		if errors.Is(loadErr, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("managed escrow guest settlement: create intent: %w", err)
		}
		return false, fmt.Errorf("managed escrow guest settlement: load intent: %w", loadErr)
	}
	if existing.IntentKey != request.IntentID || existing.OrderID != request.OrderID ||
		existing.ActionKind != payment.ManagedEscrowGuestSettlementAction || existing.IntentPayload != string(raw) {
		return false, fmt.Errorf("managed escrow guest settlement: intent identity conflict")
	}
	if existing.State != "claimed" && existing.State != "failed" {
		return false, nil
	}
	if existing.LeaseExpiresAt != nil && existing.LeaseExpiresAt.After(now) {
		return false, nil
	}
	var affected int64
	err = s.db.Update(func(tx database.Tx) error {
		where := map[string]interface{}{
			"action_id = ?": request.IntentID, "state = ?": existing.State,
			"lease_token = ?": existing.LeaseToken,
		}
		rows, updateErr := tx.UpdateColumns(map[string]interface{}{
			"state": "claimed", "lease_token": token, "lease_expires_at": leaseUntil,
			"attempts": existing.Attempts + 1, "last_error": "", "updated_at": now,
		}, where, &models.SettlementAction{})
		affected = rows
		return updateErr
	})
	if err != nil {
		return false, fmt.Errorf("managed escrow guest settlement: reclaim intent: %w", err)
	}
	if affected != 1 {
		return false, nil
	}
	request.ClaimToken = token
	return true, nil
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
	request, err := s.source.ClaimManagedEscrowGuestSettlement(ctx, orderID)
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
	orderIDs, err := s.source.ListPendingManagedEscrowGuestSettlementOrderIDs(ctx)
	if err != nil {
		log.Warningf("managed escrow guest settlement recovery: %v", err)
		return
	}
	jobs := make(chan string)
	var workers sync.WaitGroup
	workerCount := guestSettlementWorkers
	if len(orderIDs) < workerCount {
		workerCount = len(orderIDs)
	}
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for orderID := range jobs {
				request, claimErr := s.source.ClaimManagedEscrowGuestSettlement(ctx, orderID)
				if claimErr != nil {
					log.Warningf("managed escrow guest settlement recovery claim for %s: %v", redact.Token(orderID), claimErr)
					continue
				}
				if request == nil {
					continue
				}
				if submitErr := s.executor.SubmitManagedEscrowGuestSettlement(ctx, *request); submitErr != nil {
					log.Warningf("managed escrow guest settlement recovery for %s: %v", redact.Token(orderID), submitErr)
				}
			}
		}()
	}
	for _, orderID := range orderIDs {
		select {
		case jobs <- orderID:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		}
	}
	close(jobs)
	workers.Wait()
}

// RunPendingSettlementRecovery continuously reconciles eligible guest
// settlements until shutdown. A single startup scan is insufficient: an RPC
// outage can leave a durable failed/expired claim after the process is already
// running, with no second order event available to trigger it again.
func (s *DistributionManagedEscrowGuestSettlementService) RunPendingSettlementRecovery(ctx context.Context) {
	s.runPendingSettlementRecovery(ctx, guestSettlementRecoveryEvery)
}

func (s *DistributionManagedEscrowGuestSettlementService) runPendingSettlementRecovery(
	ctx context.Context,
	interval time.Duration,
) {
	if interval <= 0 {
		interval = guestSettlementRecoveryEvery
	}
	if ctx.Err() != nil {
		return
	}
	s.RecoverPendingSettlements(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RecoverPendingSettlements(ctx)
		}
	}
}

var _ distribution.ManagedEscrowGuestSettlementSource = (*ManagedEscrowGuestSettlementSource)(nil)
