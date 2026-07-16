// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

const affiliateMaxTextLength = 256

const (
	// AffiliatePayoutRailBitcoin identifies the native Bitcoin payout rail.
	AffiliatePayoutRailBitcoin = "BTC"
	// AffiliatePayoutRailBitcoinCash identifies the native Bitcoin Cash payout rail.
	AffiliatePayoutRailBitcoinCash = "BCH"
	// AffiliatePayoutRailLitecoin identifies the native Litecoin payout rail.
	AffiliatePayoutRailLitecoin = "LTC"
)

// AffiliateUTXOPayoutAddresses contains the promoter-controlled native payout
// address for each UTXO rail supported by affiliate settlement. The address
// set is frozen with the link, referral session, and order attribution.
type AffiliateUTXOPayoutAddresses map[string]string

// Clone returns an independent copy suitable for immutable snapshots.
func (a AffiliateUTXOPayoutAddresses) Clone() AffiliateUTXOPayoutAddresses {
	if a == nil {
		return nil
	}
	clone := make(AffiliateUTXOPayoutAddresses, len(a))
	for rail, address := range a {
		clone[rail] = address
	}
	return clone
}

// Equal reports whether two payout address sets have the same rails and values.
func (a AffiliateUTXOPayoutAddresses) Equal(other AffiliateUTXOPayoutAddresses) bool {
	if len(a) != len(other) {
		return false
	}
	for rail, address := range a {
		if other[rail] != address {
			return false
		}
	}
	return true
}

// AddressForRail returns the frozen destination for a native UTXO rail.
func (a AffiliateUTXOPayoutAddresses) AddressForRail(rail string) (string, bool) {
	address, ok := a[strings.ToUpper(strings.TrimSpace(rail))]
	return strings.TrimSpace(address), ok && strings.TrimSpace(address) != ""
}

// Valid reports whether the set covers exactly the UTXO rails enabled for
// affiliate settlement. Chain-specific address syntax is validated by the
// settlement wallet immediately before it signs an output.
func (a AffiliateUTXOPayoutAddresses) Valid() bool {
	addresses := a
	if len(addresses) != 3 {
		return false
	}
	for _, rail := range []string{AffiliatePayoutRailBitcoin, AffiliatePayoutRailBitcoinCash, AffiliatePayoutRailLitecoin} {
		address, ok := addresses.AddressForRail(rail)
		if !ok || len(address) > affiliateMaxTextLength {
			return false
		}
	}
	for rail := range addresses {
		switch rail {
		case AffiliatePayoutRailBitcoin, AffiliatePayoutRailBitcoinCash, AffiliatePayoutRailLitecoin:
		default:
			return false
		}
	}
	return true
}

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
	TenantID                    string                       `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default';uniqueIndex:idx_affiliate_link_promoter,priority:1;uniqueIndex:idx_affiliate_link_token,priority:1"`
	ID                          string                       `json:"id" gorm:"primaryKey;type:text"`
	ProgramID                   string                       `json:"programID" gorm:"column:program_id;type:text;not null;index;uniqueIndex:idx_affiliate_link_promoter,priority:2"`
	PromoterPeerID              string                       `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index;uniqueIndex:idx_affiliate_link_promoter,priority:3"`
	PromoterPayoutAddress       string                       `json:"promoterPayoutAddress" gorm:"column:promoter_payout_address;type:text;not null"`
	PromoterUTXOPayoutAddresses AffiliateUTXOPayoutAddresses `json:"promoterUTXOPayoutAddresses" gorm:"column:promoter_utxo_payout_addresses;serializer:json;type:text;not null"`
	PromoterPayoutDestinations  PayoutDestinationSet         `json:"promoterPayoutDestinations" gorm:"column:promoter_payout_destinations;serializer:json;type:text"`
	PublicToken                 string                       `json:"publicToken" gorm:"column:public_token;type:text;not null;uniqueIndex:idx_affiliate_link_token,priority:2"`
	Status                      AffiliateLinkStatus          `json:"status" gorm:"type:text;not null;index"`
	CreatedAt                   time.Time                    `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
	UpdatedAt                   time.Time                    `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (AffiliateLink) TableName() string { return "affiliate_links" }

// Validate checks the direct promoter-link contract.
func (l *AffiliateLink) Validate() error {
	if l == nil || !validAffiliateID(l.ID) || !validAffiliateID(l.ProgramID) ||
		!validAffiliatePeerID(l.PromoterPeerID) || !validAffiliateID(l.PublicToken) ||
		!validAffiliatePayoutSnapshot(l.PromoterPayoutDestinations, l.PromoterPayoutAddress, l.PromoterUTXOPayoutAddresses) ||
		(l.Status != AffiliateLinkStatusActive && l.Status != AffiliateLinkStatusRevoked) ||
		l.CreatedAt.IsZero() || l.UpdatedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// AffiliateReferralSession is a seller-scoped referral carried into checkout.
type AffiliateReferralSession struct {
	TenantID                    string                       `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default'"`
	ID                          string                       `json:"id" gorm:"primaryKey;type:text"`
	AffiliateLinkID             string                       `json:"affiliateLinkID" gorm:"column:affiliate_link_id;type:text;not null;index"`
	ProgramID                   string                       `json:"programID" gorm:"column:program_id;type:text;not null;index"`
	SellerPeerID                string                       `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;index"`
	PromoterPeerID              string                       `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index"`
	CommissionRateBPSSnapshot   uint32                       `json:"commissionRateBPSSnapshot" gorm:"column:commission_rate_bps_snapshot;not null"`
	PromoterPayoutAddress       string                       `json:"promoterPayoutAddress" gorm:"column:promoter_payout_address;type:text;not null"`
	PromoterUTXOPayoutAddresses AffiliateUTXOPayoutAddresses `json:"promoterUTXOPayoutAddresses" gorm:"column:promoter_utxo_payout_addresses;serializer:json;type:text;not null"`
	PromoterPayoutDestinations  PayoutDestinationSet         `json:"promoterPayoutDestinations" gorm:"column:promoter_payout_destinations;serializer:json;type:text"`
	IssuedAt                    time.Time                    `json:"issuedAt" gorm:"column:issued_at;not null;index"`
	ExpiresAt                   time.Time                    `json:"expiresAt" gorm:"column:expires_at;not null;index"`
	RevokedAt                   *time.Time                   `json:"revokedAt,omitempty" gorm:"column:revoked_at"`
	CreatedAt                   time.Time                    `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime:false"`
}

func (AffiliateReferralSession) TableName() string { return "affiliate_referral_sessions" }

// Validate checks the immutable referral-session contract.
func (s *AffiliateReferralSession) Validate() error {
	if s == nil || !validAffiliateID(s.ID) || !validAffiliateID(s.AffiliateLinkID) ||
		!validAffiliateID(s.ProgramID) || !validAffiliatePeerID(s.SellerPeerID) ||
		!validAffiliatePeerID(s.PromoterPeerID) || s.IssuedAt.IsZero() ||
		s.CommissionRateBPSSnapshot == 0 || s.CommissionRateBPSSnapshot > 10000 ||
		!validAffiliatePayoutSnapshot(s.PromoterPayoutDestinations, s.PromoterPayoutAddress, s.PromoterUTXOPayoutAddresses) ||
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
	TenantID                    string                       `json:"-" gorm:"column:tenant_id;primaryKey;default:'_default';uniqueIndex:idx_affiliate_attribution_order,priority:1"`
	ID                          string                       `json:"id" gorm:"primaryKey;type:text"`
	OrderID                     string                       `json:"orderID" gorm:"column:order_id;type:text;not null;index;uniqueIndex:idx_affiliate_attribution_order,priority:2"`
	ReferralSessionID           string                       `json:"referralSessionID" gorm:"column:referral_session_id;type:text;not null;index"`
	ProgramID                   string                       `json:"programID" gorm:"column:program_id;type:text;not null;index"`
	SellerPeerID                string                       `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;index"`
	BuyerKind                   AffiliateBuyerKind           `json:"buyerKind" gorm:"column:buyer_kind;type:text;not null;default:'peer';index"`
	BuyerPeerID                 string                       `json:"buyerPeerID,omitempty" gorm:"column:buyer_peer_id;type:text;not null;default:'';index"`
	GuestBuyerID                string                       `json:"-" gorm:"column:guest_buyer_id;type:text;not null;default:'';index"`
	PromoterPeerID              string                       `json:"promoterPeerID" gorm:"column:promoter_peer_id;type:text;not null;index"`
	CommissionRateBPSSnapshot   uint32                       `json:"commissionRateBPSSnapshot" gorm:"column:commission_rate_bps_snapshot;not null"`
	PromoterPayoutAddress       string                       `json:"promoterPayoutAddress" gorm:"column:promoter_payout_address;type:text;not null"`
	PromoterUTXOPayoutAddresses AffiliateUTXOPayoutAddresses `json:"promoterUTXOPayoutAddresses" gorm:"column:promoter_utxo_payout_addresses;serializer:json;type:text;not null"`
	PromoterPayoutDestinations  PayoutDestinationSet         `json:"promoterPayoutDestinations" gorm:"column:promoter_payout_destinations;serializer:json;type:text"`
	AttributedAt                time.Time                    `json:"attributedAt" gorm:"column:attributed_at;not null;index"`
}

func (AffiliateAttribution) TableName() string { return "affiliate_attributions" }

// Validate checks the immutable order-attribution contract.
func (a *AffiliateAttribution) Validate() error {
	if a == nil || !validAffiliateID(a.ID) || !validAffiliateID(a.OrderID) ||
		!validAffiliateID(a.ReferralSessionID) || !validAffiliateID(a.ProgramID) ||
		!validAffiliatePeerID(a.SellerPeerID) || !a.validBuyerIdentity() ||
		!validAffiliatePeerID(a.PromoterPeerID) || a.CommissionRateBPSSnapshot == 0 ||
		a.CommissionRateBPSSnapshot > 10000 ||
		!validAffiliatePayoutSnapshot(a.PromoterPayoutDestinations, a.PromoterPayoutAddress, a.PromoterUTXOPayoutAddresses) ||
		a.AttributedAt.IsZero() {
		return ErrInvalidSellerAffiliate
	}
	return nil
}

// validAffiliatePayoutSnapshot accepts the generic rail-keyed snapshot used by
// all new links. The legacy EVM/UTXO pair remains readable only so existing
// persisted referrals can settle while deployments migrate in place.
func validAffiliatePayoutSnapshot(destinations PayoutDestinationSet, legacyEVM string, legacyUTXO AffiliateUTXOPayoutAddresses) bool {
	if len(destinations.Destinations) > 0 {
		return destinations.Valid()
	}
	return validAffiliateEVMPayoutAddress(legacyEVM) && legacyUTXO.Valid()
}

// AffiliateBuyerKind identifies whether attribution belongs to a peer order or
// to one anonymous Guest Checkout order.
type AffiliateBuyerKind string

const (
	AffiliateBuyerKindPeer  AffiliateBuyerKind = "peer"
	AffiliateBuyerKindGuest AffiliateBuyerKind = "guest"
)

func (a *AffiliateAttribution) validBuyerIdentity() bool {
	switch a.BuyerKind {
	case "", AffiliateBuyerKindPeer:
		return validAffiliatePeerID(a.BuyerPeerID) && strings.TrimSpace(a.GuestBuyerID) == ""
	case AffiliateBuyerKindGuest:
		return strings.TrimSpace(a.BuyerPeerID) == "" && validAffiliateID(a.GuestBuyerID)
	default:
		return false
	}
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
	case AffiliateCommissionStatusPending:
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
	BuyerKind         AffiliateBuyerKind
	GuestBuyerID      string
	ReferralSessionID string
	AttributedAt      time.Time
	Lines             []AffiliateOrderLineFact
}

// AffiliateOrderResult is the immutable attribution and line commission snapshot.
type AffiliateOrderResult struct {
	Attribution AffiliateAttribution
	Lines       []AffiliateCommissionLine
}

// AffiliateSettlementPayout is the Core-validated seller-funded commission
// output for one order settlement. Amount uses the settlement asset's minimal
// unit and is never a separate payout workflow.
type AffiliateSettlementPayout struct {
	Address string
	Amount  string
}

// AffiliateStatementLine is a read-only projection of one immutable
// attribution and one commission line. It is not persisted separately.
type AffiliateStatementLine struct {
	Attribution    AffiliateAttribution       `json:"attribution"`
	CommissionLine AffiliateCommissionLine    `json:"commissionLine"`
	Settlement     *AffiliateSettlementOutput `json:"settlement,omitempty"`
}

// AffiliateSettlementOutput is a read-only projection of the promoter output
// in a backend settlement action. It never changes commission lifecycle state:
// the UI derives settling/paid/failed from State and uses CommissionLine.Status
// only for pending/reversed business facts.
type AffiliateSettlementOutput struct {
	ActionID      string     `json:"actionId"`
	Action        string     `json:"action"`
	State         string     `json:"state"` // planned | submitted | confirmed | failed | abandoned
	TxHash        string     `json:"txHash,omitempty"`
	Coin          string     `json:"coin"`
	Amount        string     `json:"amount"`
	Address       string     `json:"address"`
	Confirmations int        `json:"confirmations,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	ConfirmedAt   *time.Time `json:"confirmedAt,omitempty"`
}

var (
	// ErrInvalidSellerAffiliate indicates malformed or inconsistent affiliate facts.
	ErrInvalidSellerAffiliate = errors.New("invalid seller affiliate data")
	// ErrSellerAffiliateNotFound indicates a missing tenant-local affiliate resource.
	ErrSellerAffiliateNotFound = errors.New("seller affiliate resource not found")
	// ErrSellerAffiliateConflict indicates an immutable binding or lifecycle conflict.
	ErrSellerAffiliateConflict = errors.New("seller affiliate conflict")
	// ErrSellerAffiliatePaymentRailUnsupported rejects a payment rail before
	// provisioning when it cannot execute the frozen affiliate payout.
	ErrSellerAffiliatePaymentRailUnsupported = errors.New("seller affiliate payment rail is unsupported")
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

func validAffiliateEVMPayoutAddress(value string) bool {
	value = strings.TrimSpace(value)
	return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
}
