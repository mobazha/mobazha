// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	settlementpayment "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// SellerAffiliateAppService implements the automation-first Phase 1 domain.
type SellerAffiliateAppService struct {
	store   contracts.SellerAffiliateStore
	actions contracts.AffiliateSettlementActionReader
}

var _ contracts.SellerAffiliateService = (*SellerAffiliateAppService)(nil)

// NewSellerAffiliateAppService constructs the minimal affiliate application service.
func NewSellerAffiliateAppService(store contracts.SellerAffiliateStore, readers ...contracts.AffiliateSettlementActionReader) *SellerAffiliateAppService {
	service := &SellerAffiliateAppService{store: store}
	if len(readers) > 0 {
		service.actions = readers[0]
	}
	return service
}

// PutProgram creates or updates the tenant's single storefront-wide program.
func (s *SellerAffiliateAppService) PutProgram(ctx context.Context, program *models.AffiliateProgram) (*models.AffiliateProgram, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if program == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}
	next := *program
	now := time.Now().UTC()
	if next.ID == "" {
		next.ID = uuid.NewString()
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	next.UpdatedAt = now
	if err := next.Validate(); err != nil {
		return nil, err
	}
	if err := s.store.PutAffiliateProgram(ctx, &next); err != nil {
		return nil, err
	}
	return &next, nil
}

// GetProgram returns the tenant's seller program.
func (s *SellerAffiliateAppService) GetProgram(ctx context.Context) (*models.AffiliateProgram, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.GetAffiliateProgram(ctx)
}

// CreateLink creates a link with the promoter's immutable settlement destinations.
// Every enabled settlement rail must have a destination before a referral exists.
func (s *SellerAffiliateAppService) CreateLink(ctx context.Context, promoterPeerID, publicToken, payoutAddress string, utxoPayoutAddresses models.AffiliateUTXOPayoutAddresses) (*models.AffiliateLink, error) {
	payoutAddress, err := normalizeAffiliateEVMPayoutAddress(payoutAddress)
	if err != nil {
		return nil, err
	}
	if !utxoPayoutAddresses.Valid() {
		return nil, models.ErrInvalidSellerAffiliate
	}
	utxoPayoutAddresses = utxoPayoutAddresses.Clone()
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	program, err := s.store.GetAffiliateProgram(ctx)
	if err != nil {
		return nil, err
	}
	promoterPeerID = strings.TrimSpace(promoterPeerID)
	if program.Status != models.AffiliateProgramStatusActive || promoterPeerID == program.SellerPeerID {
		return nil, models.ErrInvalidSellerAffiliate
	}
	existing, err := s.store.GetAffiliateLinkByPromoter(ctx, program.ID, promoterPeerID)
	if err == nil {
		existingAddress, normalizeErr := normalizeAffiliateEVMPayoutAddress(existing.PromoterPayoutAddress)
		if normalizeErr != nil || existingAddress != payoutAddress || !existing.PromoterUTXOPayoutAddresses.Equal(utxoPayoutAddresses) {
			return nil, models.ErrSellerAffiliateConflict
		}
		return existing, nil
	}
	if !errors.Is(err, models.ErrSellerAffiliateNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	link := &models.AffiliateLink{
		ID:                          uuid.NewString(),
		ProgramID:                   program.ID,
		PromoterPeerID:              promoterPeerID,
		PromoterPayoutAddress:       payoutAddress,
		PromoterUTXOPayoutAddresses: utxoPayoutAddresses,
		PublicToken:                 strings.TrimSpace(publicToken),
		Status:                      models.AffiliateLinkStatusActive,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	if err := link.Validate(); err != nil {
		return nil, err
	}
	if err := s.store.CreateAffiliateLink(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// GetLinkByToken resolves a direct promoter link inside the current tenant.
func (s *SellerAffiliateAppService) GetLinkByToken(ctx context.Context, token string) (*models.AffiliateLink, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.GetAffiliateLinkByToken(ctx, strings.TrimSpace(token))
}

// CreateReferralSession creates an expiring seller-scoped checkout reference.
func (s *SellerAffiliateAppService) CreateReferralSession(ctx context.Context, publicToken string, issuedAt time.Time) (*models.AffiliateReferralSession, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	} else {
		issuedAt = issuedAt.UTC()
	}
	link, err := s.store.GetAffiliateLinkByToken(ctx, strings.TrimSpace(publicToken))
	if err != nil {
		return nil, err
	}
	program, err := s.store.GetAffiliateProgram(ctx)
	if err != nil {
		return nil, err
	}
	if link.Status != models.AffiliateLinkStatusActive || program.Status != models.AffiliateProgramStatusActive || link.ProgramID != program.ID {
		return nil, models.ErrInvalidSellerAffiliate
	}
	session := &models.AffiliateReferralSession{
		ID:                          uuid.NewString(),
		AffiliateLinkID:             link.ID,
		ProgramID:                   program.ID,
		SellerPeerID:                program.SellerPeerID,
		PromoterPeerID:              link.PromoterPeerID,
		CommissionRateBPSSnapshot:   program.CommissionRateBPS,
		PromoterPayoutAddress:       link.PromoterPayoutAddress,
		PromoterUTXOPayoutAddresses: link.PromoterUTXOPayoutAddresses.Clone(),
		IssuedAt:                    issuedAt,
		ExpiresAt:                   issuedAt.Add(time.Duration(program.AttributionWindowSeconds) * time.Second),
		CreatedAt:                   issuedAt,
	}
	if err := session.Validate(); err != nil {
		return nil, err
	}
	if err := s.store.CreateAffiliateReferralSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// AttributeOrder records one automatic order attribution and its line commissions.
func (s *SellerAffiliateAppService) AttributeOrder(ctx context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error) {
	result, err := s.PrepareOrderAttribution(ctx, facts)
	if err != nil || result == nil {
		return result, err
	}
	return s.store.RecordAffiliateOrder(ctx, result)
}

// PrepareOrderAttribution validates mutable referral resources and returns the
// immutable snapshot that must be committed with its order.
func (s *SellerAffiliateAppService) PrepareOrderAttribution(ctx context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if facts.AttributedAt.IsZero() || len(facts.Lines) == 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	facts.OrderID = strings.TrimSpace(facts.OrderID)
	facts.SellerPeerID = strings.TrimSpace(facts.SellerPeerID)
	facts.BuyerPeerID = strings.TrimSpace(facts.BuyerPeerID)
	facts.GuestBuyerID = strings.TrimSpace(facts.GuestBuyerID)
	if facts.BuyerKind == "" {
		facts.BuyerKind = models.AffiliateBuyerKindPeer
	}
	facts.ReferralSessionID = strings.TrimSpace(facts.ReferralSessionID)
	facts.AttributedAt = facts.AttributedAt.UTC()
	session, err := s.store.GetAffiliateReferralSession(ctx, facts.ReferralSessionID)
	if err != nil {
		return nil, err
	}
	link, err := s.store.GetAffiliateLink(ctx, session.AffiliateLinkID)
	if err != nil {
		return nil, err
	}
	program, err := s.store.GetAffiliateProgram(ctx)
	if err != nil {
		return nil, err
	}
	if program.Status != models.AffiliateProgramStatusActive || link.Status != models.AffiliateLinkStatusActive ||
		program.ID != session.ProgramID || link.ID != session.AffiliateLinkID || link.ProgramID != program.ID ||
		session.SellerPeerID != facts.SellerPeerID || session.SellerPeerID != program.SellerPeerID ||
		session.PromoterPeerID != link.PromoterPeerID || !session.UsableAt(facts.AttributedAt) {
		return nil, models.ErrInvalidSellerAffiliate
	}
	if facts.SellerPeerID == session.PromoterPeerID ||
		(facts.BuyerKind == models.AffiliateBuyerKindPeer &&
			(facts.BuyerPeerID == facts.SellerPeerID || facts.BuyerPeerID == session.PromoterPeerID)) {
		return nil, nil
	}
	attributionID := uuid.NewString()
	result := &models.AffiliateOrderResult{
		Attribution: models.AffiliateAttribution{
			ID:                          attributionID,
			OrderID:                     facts.OrderID,
			ReferralSessionID:           session.ID,
			ProgramID:                   session.ProgramID,
			SellerPeerID:                facts.SellerPeerID,
			BuyerKind:                   facts.BuyerKind,
			BuyerPeerID:                 facts.BuyerPeerID,
			GuestBuyerID:                facts.GuestBuyerID,
			PromoterPeerID:              session.PromoterPeerID,
			CommissionRateBPSSnapshot:   session.CommissionRateBPSSnapshot,
			PromoterPayoutAddress:       session.PromoterPayoutAddress,
			PromoterUTXOPayoutAddresses: session.PromoterUTXOPayoutAddresses.Clone(),
			AttributedAt:                facts.AttributedAt,
		},
		Lines: make([]models.AffiliateCommissionLine, 0, len(facts.Lines)),
	}
	if err := result.Attribution.Validate(); err != nil {
		return nil, err
	}
	seenLines := make(map[string]struct{}, len(facts.Lines))
	for _, fact := range facts.Lines {
		fact.OrderLineID = strings.TrimSpace(fact.OrderLineID)
		if _, exists := seenLines[fact.OrderLineID]; exists {
			return nil, models.ErrInvalidSellerAffiliate
		}
		seenLines[fact.OrderLineID] = struct{}{}
		commissionAtomic, err := affiliateCommissionAtomic(fact.NetMerchandiseAtomic, session.CommissionRateBPSSnapshot)
		if err != nil {
			return nil, err
		}
		line := models.AffiliateCommissionLine{
			AttributionID:             attributionID,
			OrderID:                   result.Attribution.OrderID,
			OrderLineID:               fact.OrderLineID,
			NetMerchandiseAtomic:      fact.NetMerchandiseAtomic,
			Currency:                  strings.TrimSpace(fact.Currency),
			CommissionRateBPSSnapshot: session.CommissionRateBPSSnapshot,
			CommissionAtomic:          commissionAtomic,
			Status:                    models.AffiliateCommissionStatusPending,
			CreatedAt:                 facts.AttributedAt,
			UpdatedAt:                 facts.AttributedAt,
		}
		if err := line.Validate(); err != nil {
			return nil, err
		}
		result.Lines = append(result.Lines, line)
	}
	return result, nil
}

// RecordPreparedOrderTx persists a prepared attribution in the caller's
// tenant-scoped transaction.
func (s *SellerAffiliateAppService) RecordPreparedOrderTx(tx database.Tx, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.RecordAffiliateOrderTx(tx, result)
}

func normalizeAffiliateEVMPayoutAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !common.IsHexAddress(value) || common.HexToAddress(value) == (common.Address{}) {
		return "", models.ErrInvalidSellerAffiliate
	}
	return common.HexToAddress(value).Hex(), nil
}

// TransitionCommission advances all order lines using an objective order fact.
func (s *SellerAffiliateAppService) TransitionCommission(ctx context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if at.IsZero() || status != models.AffiliateCommissionStatusReversed || !reason.Valid() {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return s.store.TransitionAffiliateCommission(ctx, strings.TrimSpace(orderID), status, reason, at.UTC())
}

// TransitionCommissionTx applies an objective lifecycle fact in the caller's
// tenant-scoped order transaction.
func (s *SellerAffiliateAppService) TransitionCommissionTx(tx database.Tx, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if at.IsZero() || status != models.AffiliateCommissionStatusReversed || !reason.Valid() {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return s.store.TransitionAffiliateCommissionTx(tx, strings.TrimSpace(orderID), status, reason, at.UTC())
}

// GetAttributionByOrder returns the immutable attribution for one seller order.
func (s *SellerAffiliateAppService) GetAttributionByOrder(ctx context.Context, orderID string) (*models.AffiliateAttribution, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.GetAffiliateAttributionByOrder(ctx, strings.TrimSpace(orderID))
}

// ListCommissionLinesByOrder returns the order's line-level commission snapshot.
func (s *SellerAffiliateAppService) ListCommissionLinesByOrder(ctx context.Context, orderID string) ([]models.AffiliateCommissionLine, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.ListAffiliateCommissionLinesByOrder(ctx, strings.TrimSpace(orderID))
}

// ListSellerStatement returns the tenant seller's complete line-level statement.
func (s *SellerAffiliateAppService) ListSellerStatement(ctx context.Context) ([]models.AffiliateStatementLine, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	statement, err := s.store.ListAffiliateStatementLines(ctx, "")
	if err != nil {
		return nil, err
	}
	return s.projectStatementSettlement(ctx, statement)
}

// ListPromoterStatement returns only lines attributed to one promoter PeerID.
func (s *SellerAffiliateAppService) ListPromoterStatement(ctx context.Context, promoterPeerID string) ([]models.AffiliateStatementLine, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	promoterPeerID = strings.TrimSpace(promoterPeerID)
	if promoterPeerID == "" {
		return nil, models.ErrInvalidSellerAffiliate
	}
	statement, err := s.store.ListAffiliateStatementLines(ctx, promoterPeerID)
	if err != nil {
		return nil, err
	}
	return s.projectStatementSettlement(ctx, statement)
}

func (s *SellerAffiliateAppService) projectStatementSettlement(ctx context.Context, statement []models.AffiliateStatementLine) ([]models.AffiliateStatementLine, error) {
	if len(statement) == 0 || s.actions == nil {
		return statement, nil
	}
	orderIDs := make([]string, 0, len(statement))
	seenOrders := make(map[string]struct{}, len(statement))
	for _, line := range statement {
		orderID := strings.TrimSpace(line.Attribution.OrderID)
		if orderID == "" {
			continue
		}
		if _, seen := seenOrders[orderID]; !seen {
			seenOrders[orderID] = struct{}{}
			orderIDs = append(orderIDs, orderID)
		}
	}
	actions, err := s.actions.ListSettlementActions(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	actionsByOrder := make(map[string][]models.SettlementActionSnapshot, len(orderIDs))
	for _, action := range actions {
		if action.OrderID != "" {
			actionsByOrder[action.OrderID] = append(actionsByOrder[action.OrderID], action)
		}
	}
	for i := range statement {
		if !affiliateCommissionPositive(statement[i].CommissionLine.CommissionAtomic) {
			continue
		}
		statement[i].Settlement = affiliateStatementSettlement(&statement[i].Attribution, actionsByOrder[statement[i].Attribution.OrderID])
	}
	return statement, nil
}

func affiliateCommissionPositive(raw string) bool {
	amount, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	return ok && amount.Sign() > 0
}

func affiliateStatementSettlement(attribution *models.AffiliateAttribution, actions []models.SettlementActionSnapshot) *models.AffiliateSettlementOutput {
	var best *models.AffiliateSettlementOutput
	for _, action := range actions {
		candidate := affiliateSettlementFromAction(attribution, action)
		if candidate == nil || affiliateSettlementRank(candidate.State) < affiliateSettlementRank("planned") {
			continue
		}
		if best == nil || affiliateSettlementRank(candidate.State) > affiliateSettlementRank(best.State) ||
			(affiliateSettlementRank(candidate.State) == affiliateSettlementRank(best.State) && candidate.UpdatedAt.After(best.UpdatedAt)) {
			best = candidate
		}
	}
	return best
}

func affiliateSettlementFromAction(attribution *models.AffiliateAttribution, action models.SettlementActionSnapshot) *models.AffiliateSettlementOutput {
	if attribution == nil || strings.TrimSpace(action.SettlementCoin) == "" {
		return nil
	}
	expectedAddress, err := affiliatePayoutAddressForSettlementCoin(attribution, action.SettlementCoin)
	if err != nil {
		return nil
	}
	planned, ok := affiliatePayoutLine(action.PlannedLines, expectedAddress, "")
	if !ok {
		return nil
	}
	output := &models.AffiliateSettlementOutput{
		ActionID:      action.ActionID,
		Action:        action.Action,
		State:         "planned",
		TxHash:        action.TxHash,
		Coin:          firstNonEmpty(planned.Coin, action.SettlementCoin),
		Amount:        planned.Amount,
		Address:       planned.Address,
		Confirmations: action.Confirmations,
		UpdatedAt:     action.UpdatedAt,
		ConfirmedAt:   action.ConfirmedAt,
	}
	switch action.State {
	case "submitting":
		return output
	case "submitted":
		output.State = "submitted"
		return output
	case "reorged":
		// A previously confirmed output that left the canonical chain is no
		// longer paid, but the persisted transaction remains the active
		// submission while WalletTransfer waits for reconfirmation.
		output.State = "submitted"
		output.ConfirmedAt = nil
		return output
	case "confirmed":
		observed, observedOK := affiliatePayoutLine(action.ObservedLines, expectedAddress, planned.Amount)
		if !observedOK {
			return nil
		}
		output.State = "confirmed"
		output.Amount = observed.Amount
		output.Address = observed.Address
		output.Coin = firstNonEmpty(observed.Coin, output.Coin)
		output.TxHash = firstNonEmpty(observed.TxHash, action.TxHash)
		return output
	default:
		return nil
	}
}

func affiliatePayoutLine(lines []models.SettlementPayoutLine, expectedAddress, expectedAmount string) (models.SettlementPayoutLine, bool) {
	for _, line := range lines {
		if strings.TrimSpace(line.Type) != "affiliate" || !sameAffiliatePayoutAddress(line.Address, expectedAddress) || strings.TrimSpace(line.Amount) == "" || strings.TrimSpace(line.Amount) == "0" {
			continue
		}
		if expectedAmount != "" && strings.TrimSpace(line.Amount) != strings.TrimSpace(expectedAmount) {
			continue
		}
		return line, true
	}
	return models.SettlementPayoutLine{}, false
}

func sameAffiliatePayoutAddress(actual, expected string) bool {
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if common.IsHexAddress(actual) && common.IsHexAddress(expected) {
		return common.HexToAddress(actual) == common.HexToAddress(expected)
	}
	return actual == expected
}

func affiliateSettlementRank(state string) int {
	switch state {
	case "confirmed":
		return 3
	case "submitted":
		return 2
	case "planned":
		return 1
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// ListPendingCommissionOrderIDs returns orders waiting on existing protection facts.
func (s *SellerAffiliateAppService) ListPendingCommissionOrderIDs(ctx context.Context) ([]string, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.ListPendingAffiliateCommissionOrderIDs(ctx)
}

// SettlementPayout returns the immutable commission split that an order
// settlement adapter must execute. It never recomputes rate or destination
// from mutable program/profile state.
func (s *SellerAffiliateAppService) SettlementPayout(ctx context.Context, orderID, settlementCoin string) (*models.AffiliateSettlementPayout, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	orderID = strings.TrimSpace(orderID)
	settlementCoin = strings.TrimSpace(settlementCoin)
	if orderID == "" || settlementCoin == "" {
		return nil, models.ErrInvalidSellerAffiliate
	}
	attribution, err := s.store.GetAffiliateAttributionByOrder(ctx, orderID)
	if errors.Is(err, models.ErrSellerAffiliateNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if attribution == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}
	address, err := affiliatePayoutAddressForSettlementCoin(attribution, settlementCoin)
	if err != nil {
		return nil, err
	}
	lines, err := s.store.ListAffiliateCommissionLinesByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	amount := new(big.Int)
	for _, line := range lines {
		if line.Status == models.AffiliateCommissionStatusReversed {
			continue
		}
		if line.Status != models.AffiliateCommissionStatusPending ||
			!sameAffiliateSettlementCoin(line.Currency, settlementCoin) {
			return nil, models.ErrInvalidSellerAffiliate
		}
		lineAmount, ok := new(big.Int).SetString(line.CommissionAtomic, 10)
		if !ok || lineAmount.Sign() < 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		amount.Add(amount, lineAmount)
	}
	if amount.Sign() == 0 {
		return nil, nil
	}
	return &models.AffiliateSettlementPayout{Address: address, Amount: amount.String()}, nil
}

// SettlementAttemptTerm returns the complete immutable affiliate allocation
// that a seller must bind into one attempt-scoped settlement authorization.
// Unlike SettlementPayout, an allocation that rounds to zero is retained so
// the signed terms still prove that the referred order was not silently
// stripped of its attribution.
func (s *SellerAffiliateAppService) SettlementAttemptTerm(ctx context.Context, orderID, settlementCoin string) (*models.PaymentAttemptAffiliateTerm, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	orderID = strings.TrimSpace(orderID)
	settlementCoin = strings.TrimSpace(settlementCoin)
	if orderID == "" || settlementCoin == "" {
		return nil, models.ErrInvalidSellerAffiliate
	}
	attribution, err := s.store.GetAffiliateAttributionByOrder(ctx, orderID)
	if errors.Is(err, models.ErrSellerAffiliateNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if attribution == nil || attribution.Validate() != nil ||
		(attribution.BuyerKind != "" && attribution.BuyerKind != models.AffiliateBuyerKindPeer) {
		return nil, models.ErrInvalidSellerAffiliate
	}
	address, err := affiliatePayoutAddressForSettlementCoin(attribution, settlementCoin)
	if err != nil {
		return nil, err
	}
	lines, err := s.store.ListAffiliateCommissionLinesByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	termLines := make([]models.PaymentAttemptAffiliateLineTerm, 0, len(lines))
	netTotal := new(big.Int)
	commissionTotal := new(big.Int)
	for i := range lines {
		line := &lines[i]
		if line.Validate() != nil || line.Status != models.AffiliateCommissionStatusPending ||
			line.OrderID != orderID || line.AttributionID != attribution.ID ||
			line.CommissionRateBPSSnapshot != attribution.CommissionRateBPSSnapshot ||
			!sameAffiliateSettlementCoin(line.Currency, settlementCoin) {
			return nil, models.ErrInvalidSellerAffiliate
		}
		netAmount, netOK := new(big.Int).SetString(strings.TrimSpace(line.NetMerchandiseAtomic), 10)
		commission, commissionOK := new(big.Int).SetString(strings.TrimSpace(line.CommissionAtomic), 10)
		if !netOK || !commissionOK || netAmount.Sign() <= 0 || commission.Sign() < 0 ||
			netAmount.String() != line.NetMerchandiseAtomic || commission.String() != line.CommissionAtomic {
			return nil, models.ErrInvalidSellerAffiliate
		}
		netTotal.Add(netTotal, netAmount)
		commissionTotal.Add(commissionTotal, commission)
		termLines = append(termLines, models.PaymentAttemptAffiliateLineTerm{
			OrderLineID: line.OrderLineID, NetMerchandiseAtomic: line.NetMerchandiseAtomic,
			CommissionAtomic: line.CommissionAtomic,
		})
	}
	sort.Slice(termLines, func(i, j int) bool { return termLines[i].OrderLineID < termLines[j].OrderLineID })
	return &models.PaymentAttemptAffiliateTerm{
		ReferralSessionID: attribution.ReferralSessionID,
		ProgramID:         attribution.ProgramID, PromoterPeerID: attribution.PromoterPeerID,
		BuyerPeerID: attribution.BuyerPeerID, CommissionRateBPS: attribution.CommissionRateBPSSnapshot,
		Address: address, Amount: commissionTotal.String(), SellerGrossBasis: netTotal.String(),
		Lines: termLines,
	}, nil
}

// HasSettlementTerms reports whether immutable affiliate attribution exists
// for an order. A true result with a nil SettlementPayout means the frozen
// commission legitimately rounds to zero atomic units.
func (s *SellerAffiliateAppService) HasSettlementTerms(ctx context.Context, orderID string) (bool, error) {
	if s == nil || s.store == nil {
		return false, errors.New("seller affiliate store not configured")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return false, models.ErrInvalidSellerAffiliate
	}
	attribution, err := s.store.GetAffiliateAttributionByOrder(ctx, orderID)
	if errors.Is(err, models.ErrSellerAffiliateNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if attribution == nil {
		return false, models.ErrInvalidSellerAffiliate
	}
	return true, nil
}

func sameAffiliateSettlementCoin(lineCurrency, settlementCoin string) bool {
	lineCurrency = strings.TrimSpace(lineCurrency)
	settlementCoin = strings.TrimSpace(settlementCoin)
	if strings.EqualFold(lineCurrency, settlementCoin) {
		return true
	}
	lineCanonical, lineOK := settlementpayment.NormalizeSettlementPaymentCoin(lineCurrency)
	settlementCanonical, settlementOK := settlementpayment.NormalizeSettlementPaymentCoin(settlementCoin)
	return lineOK && settlementOK && lineCanonical == settlementCanonical
}

func affiliatePayoutAddressForSettlementCoin(attribution *models.AffiliateAttribution, settlementCoin string) (string, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(settlementCoin))
	if err == nil {
		switch coinInfo.Chain {
		case iwallet.ChainBitcoin:
			return affiliateUTXOPayoutAddress(attribution.PromoterUTXOPayoutAddresses, models.AffiliatePayoutRailBitcoin)
		case iwallet.ChainBitcoinCash:
			return affiliateUTXOPayoutAddress(attribution.PromoterUTXOPayoutAddresses, models.AffiliatePayoutRailBitcoinCash)
		case iwallet.ChainLitecoin:
			return affiliateUTXOPayoutAddress(attribution.PromoterUTXOPayoutAddresses, models.AffiliatePayoutRailLitecoin)
		}
	}
	return normalizeAffiliateEVMPayoutAddress(attribution.PromoterPayoutAddress)
}

func affiliateUTXOPayoutAddress(addresses models.AffiliateUTXOPayoutAddresses, rail string) (string, error) {
	address, ok := addresses.AddressForRail(rail)
	if !ok {
		return "", models.ErrInvalidSellerAffiliate
	}
	return address, nil
}

func affiliateCommissionAtomic(baseAtomic string, rateBPS uint32) (string, error) {
	base, ok := new(big.Int).SetString(baseAtomic, 10)
	if !ok || base.Sign() <= 0 || rateBPS == 0 || rateBPS > 10000 {
		return "", models.ErrInvalidSellerAffiliate
	}
	commission := new(big.Int).Mul(base, big.NewInt(int64(rateBPS)))
	commission.Quo(commission, big.NewInt(10000))
	return commission.String(), nil
}
