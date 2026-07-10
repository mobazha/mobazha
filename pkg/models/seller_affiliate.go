package models

import (
	"errors"
	"math/big"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

const affiliateMaxTextLength = 256

// AffiliateProgramStatus is the minimal seller-controlled program state.
type AffiliateProgramStatus string

const (
	AffiliateProgramStatusActive AffiliateProgramStatus = "active"
	AffiliateProgramStatusPaused AffiliateProgramStatus = "paused"
)

// AffiliateLinkStatus is the direct promoter-link state.
type AffiliateLinkStatus string

const (
	AffiliateLinkStatusActive  AffiliateLinkStatus = "active"
	AffiliateLinkStatusRevoked AffiliateLinkStatus = "revoked"
)

// AffiliateCommissionStatus is the complete Phase 1 commission lifecycle.
type AffiliateCommissionStatus string

const (
	AffiliateCommissionStatusPending  AffiliateCommissionStatus = "pending"
	AffiliateCommissionStatusEarned   AffiliateCommissionStatus = "earned"
	AffiliateCommissionStatusReversed AffiliateCommissionStatus = "reversed"
)

// AffiliateCommissionReversalReason identifies an objective order/payment fact.
type AffiliateCommissionReversalReason string

const (
	AffiliateReversalRefund       AffiliateCommissionReversalReason = "refund"
	AffiliateReversalChargeback   AffiliateCommissionReversalReason = "chargeback"
	AffiliateReversalDisputeLost  AffiliateCommissionReversalReason = "dispute_lost"
	AffiliateReversalOrderInvalid AffiliateCommissionReversalReason = "order_invalid"
)

// AffiliateProgram is the single storefront-wide seller program used in Phase 1.
type AffiliateProgram struct {
	TenantID                 string                 `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default';uniqueIndex:idx_affiliate_program_seller,priority:1"`
	ID                       string                 `json:"id" gorm:"primaryKey;type:text"`
	SellerPeerID             string                 `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;uniqueIndex:idx_affiliate_program_seller,priority:2"`
	Status                   AffiliateProgramStatus `json:"status" gorm:"type:text;not null;index"`
	CommissionRateBPS        uint32                 `json:"commissionRateBPS" gorm:"column:commission_rate_bps;not null"`
	AttributionWindowSeconds uint64                 `json:"attributionWindowSeconds" gorm:"column:attribution_window_seconds;not null"`
	CreatedAt                time.Time              `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
	UpdatedAt                time.Time              `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (AffiliateProgram) TableName() string { return "affiliate_programs" }

// Validate checks the Phase 1 seller program contract.
func (p *AffiliateProgram) Validate() error {
	if p == nil || !validAffiliateID(p.ID) || !validAffiliatePeerID(p.SellerPeerID) ||
		(p.Status != AffiliateProgramStatusActive && p.Status != AffiliateProgramStatusPaused) ||
		p.CommissionRateBPS == 0 || p.CommissionRateBPS > 10000 || p.AttributionWindowSeconds == 0 ||
		p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// AffiliateLink is one seller program's direct link for one promoter.
type AffiliateLink struct {
	TenantID       string              `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default';uniqueIndex:idx_affiliate_link_promoter,priority:1;uniqueIndex:idx_affiliate_link_token,priority:1"`
	ID             string              `json:"id" gorm:"primaryKey;type:text"`
	ProgramID      string              `json:"programID" gorm:"column:program_id;type:text;not null;index;uniqueIndex:idx_affiliate_link_promoter,priority:2"`
	PromoterPeerID string              `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index;uniqueIndex:idx_affiliate_link_promoter,priority:3"`
	PublicToken    string              `json:"publicToken" gorm:"column:public_token;type:text;not null;uniqueIndex:idx_affiliate_link_token,priority:2"`
	Status         AffiliateLinkStatus `json:"status" gorm:"type:text;not null;index"`
	CreatedAt      time.Time           `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
	UpdatedAt      time.Time           `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (AffiliateLink) TableName() string { return "affiliate_links" }

// Validate checks the direct promoter-link contract.
func (l *AffiliateLink) Validate() error {
	if l == nil || !validAffiliateID(l.ID) || !validAffiliateID(l.ProgramID) ||
		!validAffiliatePeerID(l.PromoterPeerID) || !validAffiliateID(l.PublicToken) ||
		(l.Status != AffiliateLinkStatusActive && l.Status != AffiliateLinkStatusRevoked) ||
		l.CreatedAt.IsZero() || l.UpdatedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// AffiliateReferralSession is a seller-scoped referral carried into checkout.
type AffiliateReferralSession struct {
	TenantID        string     `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default'"`
	ID              string     `json:"id" gorm:"primaryKey;type:text"`
	AffiliateLinkID string     `json:"affiliateLinkID" gorm:"column:affiliate_link_id;type:text;not null;index"`
	ProgramID       string     `json:"programID" gorm:"column:program_id;type:text;not null;index"`
	SellerPeerID    string     `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;index"`
	PromoterPeerID  string     `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index"`
	IssuedAt        time.Time  `json:"issuedAt" gorm:"column:issued_at;not null;index"`
	ExpiresAt       time.Time  `json:"expiresAt" gorm:"column:expires_at;not null;index"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty" gorm:"column:revoked_at"`
	CreatedAt       time.Time  `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
}

func (AffiliateReferralSession) TableName() string { return "affiliate_referral_sessions" }

// Validate checks the immutable referral-session contract.
func (s *AffiliateReferralSession) Validate() error {
	if s == nil || !validAffiliateID(s.ID) || !validAffiliateID(s.AffiliateLinkID) ||
		!validAffiliateID(s.ProgramID) || !validAffiliatePeerID(s.SellerPeerID) ||
		!validAffiliatePeerID(s.PromoterPeerID) || s.IssuedAt.IsZero() ||
		s.ExpiresAt.IsZero() || !s.ExpiresAt.After(s.IssuedAt) || s.CreatedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// UsableAt reports whether the referral can attribute an order at the given time.
func (s AffiliateReferralSession) UsableAt(at time.Time) bool {
	return s.RevokedAt == nil && !at.Before(s.IssuedAt) && at.Before(s.ExpiresAt)
}

// AffiliateAttribution is the immutable order-level referral result.
type AffiliateAttribution struct {
	TenantID                  string    `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default';uniqueIndex:idx_affiliate_attribution_order,priority:1"`
	ID                        string    `json:"id" gorm:"primaryKey;type:text"`
	OrderID                   string    `json:"orderID" gorm:"column:order_id;type:text;not null;index;uniqueIndex:idx_affiliate_attribution_order,priority:2"`
	ReferralSessionID         string    `json:"referralSessionID" gorm:"column:referral_session_id;type:text;not null;index"`
	ProgramID                 string    `json:"programID" gorm:"column:program_id;type:text;not null;index"`
	SellerPeerID              string    `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;index"`
	BuyerPeerID               string    `json:"buyerPeerID" gorm:"column:buyer_peer_id;type:text;not null;index"`
	PromoterPeerID            string    `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index"`
	CommissionRateBPSSnapshot uint32    `json:"commissionRateBPSSnapshot" gorm:"column:commission_rate_bps_snapshot;not null"`
	AttributedAt              time.Time `json:"attributedAt" gorm:"column:attributed_at;not null;index"`
}

func (AffiliateAttribution) TableName() string { return "affiliate_attributions" }

// Validate checks the immutable order-attribution contract.
func (a *AffiliateAttribution) Validate() error {
	if a == nil || !validAffiliateID(a.ID) || !validAffiliateID(a.OrderID) ||
		!validAffiliateID(a.ReferralSessionID) || !validAffiliateID(a.ProgramID) ||
		!validAffiliatePeerID(a.SellerPeerID) || !validAffiliatePeerID(a.BuyerPeerID) ||
		!validAffiliatePeerID(a.PromoterPeerID) || a.CommissionRateBPSSnapshot == 0 ||
		a.CommissionRateBPSSnapshot > 10000 || a.AttributedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// AffiliateCommissionLine is one order line's complete Phase 1 commission record.
type AffiliateCommissionLine struct {
	TenantID                  string                            `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default'"`
	AttributionID             string                            `json:"attributionID" gorm:"column:attribution_id;type:text;primaryKey"`
	OrderID                   string                            `json:"orderID" gorm:"column:order_id;type:text;not null;index"`
	OrderLineID               string                            `json:"orderLineID" gorm:"column:order_line_id;type:text;primaryKey"`
	NetMerchandiseAtomic      string                            `json:"netMerchandiseAtomic" gorm:"column:net_merchandise_atomic;type:text;not null"`
	Currency                  string                            `json:"currency" gorm:"type:text;not null"`
	CommissionRateBPSSnapshot uint32                            `json:"commissionRateBPSSnapshot" gorm:"column:commission_rate_bps_snapshot;not null"`
	CommissionAtomic          string                            `json:"commissionAtomic" gorm:"column:commission_atomic;type:text;not null"`
	Status                    AffiliateCommissionStatus         `json:"status" gorm:"type:text;not null;index"`
	ReversalReason            AffiliateCommissionReversalReason `json:"reversalReason,omitempty" gorm:"column:reversal_reason;type:text"`
	ReversedAt                *time.Time                        `json:"reversedAt,omitempty" gorm:"column:reversed_at"`
	CreatedAt                 time.Time                         `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
	UpdatedAt                 time.Time                         `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (AffiliateCommissionLine) TableName() string { return "affiliate_commission_lines" }

// Validate checks the line amount and three-state lifecycle.
func (l *AffiliateCommissionLine) Validate() error {
	if l == nil || !validAffiliateID(l.AttributionID) || !validAffiliateID(l.OrderID) ||
		!validAffiliateID(l.OrderLineID) || !validAffiliateAtomic(l.NetMerchandiseAtomic, true) ||
		!validAffiliateAtomic(l.CommissionAtomic, false) || strings.TrimSpace(l.Currency) == "" ||
		len(l.Currency) > 64 || l.CommissionRateBPSSnapshot == 0 || l.CommissionRateBPSSnapshot > 10000 ||
		l.CreatedAt.IsZero() || l.UpdatedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	switch l.Status {
	case AffiliateCommissionStatusPending, AffiliateCommissionStatusEarned:
		if l.ReversalReason != "" || l.ReversedAt != nil {
			return ErrInvalidSellerAffiliate
		}
	case AffiliateCommissionStatusReversed:
		if !l.ReversalReason.Valid() || l.ReversedAt == nil {
			return ErrInvalidSellerAffiliate
		}
	default:
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// Valid reports whether the reversal reason is an objective order/payment fact.
func (r AffiliateCommissionReversalReason) Valid() bool {
	switch r {
	case AffiliateReversalRefund, AffiliateReversalChargeback, AffiliateReversalDisputeLost, AffiliateReversalOrderInvalid:
		return true
	default:
		return false
	}
}

// AffiliateOrderLineFact is the verified Node amount for one order line.
type AffiliateOrderLineFact struct {
	OrderLineID          string
	NetMerchandiseAtomic string
	Currency             string
}

// AffiliateOrderFacts are the verified Node facts accepted for automatic attribution.
type AffiliateOrderFacts struct {
	OrderID           string
	SellerPeerID      string
	BuyerPeerID       string
	ReferralSessionID string
	AttributedAt      time.Time
	Lines             []AffiliateOrderLineFact
}

// AffiliateOrderResult is the immutable attribution and line commission snapshot.
type AffiliateOrderResult struct {
	Attribution AffiliateAttribution
	Lines       []AffiliateCommissionLine
}

var (
	// ErrInvalidSellerAffiliate indicates malformed or inconsistent affiliate facts.
	ErrInvalidSellerAffiliate = errors.New("invalid seller affiliate data")
	// ErrSellerAffiliateNotFound indicates a missing tenant-local affiliate resource.
	ErrSellerAffiliateNotFound = errors.New("seller affiliate resource not found")
	// ErrSellerAffiliateConflict indicates an immutable binding or lifecycle conflict.
	ErrSellerAffiliateConflict = errors.New("seller affiliate conflict")
)

func validAffiliateID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= affiliateMaxTextLength
}

func validAffiliatePeerID(value string) bool {
	_, err := peer.Decode(strings.TrimSpace(value))
	return err == nil
}

func validAffiliateAtomic(value string, positive bool) bool {
	n, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return false
	}
	if positive {
		return n.Sign() > 0
	}
	return n.Sign() >= 0
}
