// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"time"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

// SellerAffiliateStore persists the minimal Phase 1 affiliate domain.
type SellerAffiliateStore interface {
	PutAffiliateProgram(ctx context.Context, program *models.AffiliateProgram) error
	GetAffiliateProgram(ctx context.Context) (*models.AffiliateProgram, error)
	CreateAffiliateLink(ctx context.Context, link *models.AffiliateLink) error
	UpdateAffiliateLink(ctx context.Context, link *models.AffiliateLink) error
	GetAffiliateLink(ctx context.Context, id string) (*models.AffiliateLink, error)
	GetAffiliateLinkByToken(ctx context.Context, token string) (*models.AffiliateLink, error)
	GetAffiliateLinkByPromoter(ctx context.Context, programID, promoterPeerID string) (*models.AffiliateLink, error)
	ListAffiliateLinks(ctx context.Context, programID string) ([]models.AffiliateLink, error)
	CreateAffiliateReferralSession(ctx context.Context, session *models.AffiliateReferralSession) error
	GetAffiliateReferralSession(ctx context.Context, id string) (*models.AffiliateReferralSession, error)
	RecordAffiliateOrder(ctx context.Context, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error)
	RecordAffiliateOrderTx(tx database.Tx, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error)
	GetAffiliateAttributionByOrder(ctx context.Context, orderID string) (*models.AffiliateAttribution, error)
	ListAffiliateCommissionLinesByOrder(ctx context.Context, orderID string) ([]models.AffiliateCommissionLine, error)
	ListAffiliateStatementLines(ctx context.Context, promoterPeerID string) ([]models.AffiliateStatementLine, error)
	ListPendingAffiliateCommissionOrderIDs(ctx context.Context) ([]string, error)
	TransitionAffiliateCommission(ctx context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
	TransitionAffiliateCommissionLines(ctx context.Context, orderID string, orderLineIDs []string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
	TransitionAffiliateCommissionTx(tx database.Tx, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
}

// SellerAffiliateSettlementPayoutProvider exposes the Core-owned payout plan
// needed by settlement execution without exposing affiliate write operations.
type SellerAffiliateSettlementPayoutProvider interface {
	SettlementPayout(ctx context.Context, orderID, settlementCoin string) (*models.AffiliateSettlementPayout, error)
}

// SellerAffiliateSettlementTermsProvider reports whether an order has frozen
// affiliate terms even when the commission rounds to zero atomic units.
type SellerAffiliateSettlementTermsProvider interface {
	HasSettlementTerms(ctx context.Context, orderID string) (bool, error)
}

// AffiliateSettlementActionReader provides the immutable settlement-action
// facts needed to project promoter outputs into affiliate statements.
type AffiliateSettlementActionReader interface {
	ListSettlementActions(ctx context.Context, orderIDs []string) ([]models.SettlementActionSnapshot, error)
}

// SellerAffiliateService defines the automation-first affiliate operations.
type SellerAffiliateService interface {
	SellerAffiliateSettlementPayoutProvider
	PutProgram(ctx context.Context, program *models.AffiliateProgram) (*models.AffiliateProgram, error)
	GetProgram(ctx context.Context) (*models.AffiliateProgram, error)
	CreateLink(ctx context.Context, promoterPeerID, publicToken string, payoutDestinations models.PayoutDestinationSet) (*models.AffiliateLink, error)
	ReissueLink(ctx context.Context, linkID, publicToken string, payoutDestinations models.PayoutDestinationSet) (*models.AffiliateLink, error)
	GetLink(ctx context.Context, linkID string) (*models.AffiliateLink, error)
	GetLinkByToken(ctx context.Context, token string) (*models.AffiliateLink, error)
	ListLinks(ctx context.Context) ([]models.AffiliateLink, error)
	RevokeLink(ctx context.Context, linkID string) (*models.AffiliateLink, error)
	CreateReferralSession(ctx context.Context, publicToken string, issuedAt time.Time) (*models.AffiliateReferralSession, error)
	AttributeOrder(ctx context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error)
	TransitionCommission(ctx context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
	TransitionCommissionLines(ctx context.Context, orderID string, orderLineIDs []string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
	GetAttributionByOrder(ctx context.Context, orderID string) (*models.AffiliateAttribution, error)
	ListCommissionLinesByOrder(ctx context.Context, orderID string) ([]models.AffiliateCommissionLine, error)
	ListSellerStatement(ctx context.Context) ([]models.AffiliateStatementLine, error)
	ListPromoterStatement(ctx context.Context, promoterPeerID string) ([]models.AffiliateStatementLine, error)
	ListPendingCommissionOrderIDs(ctx context.Context) ([]string, error)
}

// SellerAffiliateAttributionService exposes the two-phase attribution
// operations used to validate mutable referral resources before an order
// transaction and commit the resulting immutable snapshot with that order.
type SellerAffiliateAttributionService interface {
	PrepareOrderAttribution(ctx context.Context, facts models.AffiliateOrderFacts) (*models.AffiliateOrderResult, error)
	RecordPreparedOrderTx(tx database.Tx, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error)
	TransitionCommissionTx(tx database.Tx, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error)
}

// GuestSellerAffiliateService is retained as the checkout-facing name for the
// shared transactional attribution contract.
type GuestSellerAffiliateService = SellerAffiliateAttributionService

// SellerAffiliateProvider exposes the tenant-local affiliate subsystem.
type SellerAffiliateProvider interface {
	SellerAffiliate() SellerAffiliateService
}
