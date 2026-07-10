package core

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
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

// CreateLink creates the tenant's direct link for one promoter.
func (s *SellerAffiliateAppService) CreateLink(ctx context.Context, promoterPeerID, publicToken string) (*models.AffiliateLink, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	program, err := s.store.GetAffiliateProgram(ctx)
	if err != nil {
		return nil, err
	}
	if program.Status != models.AffiliateProgramStatusActive || promoterPeerID == program.SellerPeerID {
		return nil, models.ErrInvalidSellerAffiliate
	}
	now := time.Now().UTC()
	link := &models.AffiliateLink{
		ID:             uuid.NewString(),
		ProgramID:      program.ID,
		PromoterPeerID: strings.TrimSpace(promoterPeerID),
		PublicToken:    strings.TrimSpace(publicToken),
		Status:         models.AffiliateLinkStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
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
		ID:              uuid.NewString(),
		AffiliateLinkID: link.ID,
		ProgramID:       program.ID,
		SellerPeerID:    program.SellerPeerID,
		PromoterPeerID:  link.PromoterPeerID,
		IssuedAt:        issuedAt,
		ExpiresAt:       issuedAt.Add(time.Duration(program.AttributionWindowSeconds) * time.Second),
		CreatedAt:       issuedAt,
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
			ID:                        attributionID,
			OrderID:                   facts.OrderID,
			ReferralSessionID:         session.ID,
			ProgramID:                 program.ID,
			SellerPeerID:              facts.SellerPeerID,
			BuyerPeerID:               facts.BuyerPeerID,
			PromoterPeerID:            session.PromoterPeerID,
			CommissionRateBPSSnapshot: program.CommissionRateBPS,
			AttributedAt:              facts.AttributedAt,
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
		commissionAtomic, err := affiliateCommissionAtomic(fact.NetMerchandiseAtomic, program.CommissionRateBPS)
		if err != nil {
			return nil, err
		}
		line := models.AffiliateCommissionLine{
			AttributionID:             attributionID,
			OrderID:                   result.Attribution.OrderID,
			OrderLineID:               fact.OrderLineID,
			NetMerchandiseAtomic:      fact.NetMerchandiseAtomic,
			Currency:                  strings.TrimSpace(fact.Currency),
			CommissionRateBPSSnapshot: program.CommissionRateBPS,
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

// ListPendingCommissionOrderIDs returns orders waiting on existing protection facts.
func (s *SellerAffiliateAppService) ListPendingCommissionOrderIDs(ctx context.Context) ([]string, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("seller affiliate store not configured")
	}
	return s.store.ListPendingAffiliateCommissionOrderIDs(ctx)
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
