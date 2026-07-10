package core

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// SellerAffiliateAppService implements the automation-first Phase 1 domain.
type SellerAffiliateAppService struct {
	store contracts.SellerAffiliateStore
}

var _ contracts.SellerAffiliateService = (*SellerAffiliateAppService)(nil)

// NewSellerAffiliateAppService constructs the minimal affiliate application service.
func NewSellerAffiliateAppService(store contracts.SellerAffiliateStore) *SellerAffiliateAppService {
	return &SellerAffiliateAppService{store: store}
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
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	if facts.AttributedAt.IsZero() || len(facts.Lines) == 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	facts.OrderID = strings.TrimSpace(facts.OrderID)
	facts.SellerPeerID = strings.TrimSpace(facts.SellerPeerID)
	facts.BuyerPeerID = strings.TrimSpace(facts.BuyerPeerID)
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
	if facts.BuyerPeerID == facts.SellerPeerID || facts.BuyerPeerID == session.PromoterPeerID || facts.SellerPeerID == session.PromoterPeerID {
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
			BuyerPeerID:                 facts.BuyerPeerID,
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
	return s.store.RecordAffiliateOrder(ctx, result)
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
	if at.IsZero() || (status != models.AffiliateCommissionStatusEarned && status != models.AffiliateCommissionStatusReversed) ||
		(status == models.AffiliateCommissionStatusEarned && reason != "") ||
		(status == models.AffiliateCommissionStatusReversed && !reason.Valid()) {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return s.store.TransitionAffiliateCommission(ctx, strings.TrimSpace(orderID), status, reason, at.UTC())
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
	return s.store.ListAffiliateStatementLines(ctx, "")
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
	return s.store.ListAffiliateStatementLines(ctx, promoterPeerID)
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
		if (line.Status != models.AffiliateCommissionStatusPending && line.Status != models.AffiliateCommissionStatusEarned) ||
			!strings.EqualFold(strings.TrimSpace(line.Currency), settlementCoin) {
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
