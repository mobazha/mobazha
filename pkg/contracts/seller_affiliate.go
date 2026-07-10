package contracts

import (
	"context"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

// SellerAffiliateStore persists the minimal Phase 1 affiliate domain.
type SellerAffiliateStore interface {
	PutAffiliateProgram(ctx context.Context, program *models.AffiliateProgram) error
	GetAffiliateProgram(ctx context.Context) (*models.AffiliateProgram, error)
	CreateAffiliateLink(ctx context.Context, link *models.AffiliateLink) error
	GetAffiliateLink(ctx context.Context, id string) (*models.AffiliateLink, error)
	GetAffiliateLinkByToken(ctx context.Context, token string) (*models.AffiliateLink, error)
	CreateAffiliateReferralSession(ctx context.Context, session *models.AffiliateReferralSession) error
	GetAffiliateReferralSession(ctx context.Context, id string) (*models.AffiliateReferralSession, error)
	RecordAffiliateOrder(ctx context.Context, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error)
	GetAffiliateAttributionByOrder(ctx context.Context, orderID string) (*models.AffiliateAttribution, error)
	ListAffiliateCommissionLinesByOrder(ctx context.Context, orderID string) ([]models.AffiliateCommissionLine, error)
	TransitionAffiliateCommission(ctx context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
}

// SellerAffiliateService defines the automation-first Phase 1 operations.
type SellerAffiliateService interface {
	PutProgram(ctx context.Context, program *models.AffiliateProgram) (*models.AffiliateProgram, error)
	GetProgram(ctx context.Context) (*models.AffiliateProgram, error)
	CreateLink(ctx context.Context, promoterPeerID, publicToken string) (*models.AffiliateLink, error)
	GetLinkByToken(ctx context.Context, token string) (*models.AffiliateLink, error)
	CreateReferralSession(ctx context.Context, publicToken string, issuedAt time.Time) (*models.AffiliateReferralSession, error)
	AttributeOrder(ctx context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error)
	TransitionCommission(ctx context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
	GetAttributionByOrder(ctx context.Context, orderID string) (*models.AffiliateAttribution, error)
	ListCommissionLinesByOrder(ctx context.Context, orderID string) ([]models.AffiliateCommissionLine, error)
}

// SellerAffiliateProvider exposes the tenant-local affiliate subsystem.
type SellerAffiliateProvider interface {
	SellerAffiliate() SellerAffiliateService
}
