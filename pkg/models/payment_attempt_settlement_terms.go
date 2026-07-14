// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const (
	PaymentAttemptSettlementTermsVersion           = 1
	PaymentAttemptSettlementTermsQuoteBoundVersion = 2
	DisputeScalingSellerAwardProRataFloor          = "seller_award_pro_rata_floor"
	settlementTermsSigningDomainV1                 = "mobazha/payment-attempt-settlement-terms/v1\x00"
	settlementTermsSigningDomainV2                 = "mobazha/payment-attempt-settlement-terms/v2\x00"
)

var ErrPaymentAttemptSettlementTermsConflict = errors.New("payment attempt settlement terms conflict")

// PaymentAttemptSettlementTerms is the immutable economic allocation for one
// payment attempt. Runtime outpoints, gas, transaction hashes and observed
// outputs deliberately do not belong here.
type PaymentAttemptSettlementTerms struct {
	Version              uint32                       `json:"version"`
	OrderID              string                       `json:"orderID"`
	AttemptID            string                       `json:"attemptID"`
	AssetID              string                       `json:"assetID"`
	FundingAmount        string                       `json:"fundingAmount"`
	FundingBasisHash     string                       `json:"fundingBasisHash,omitempty"`
	FundingTargetAddress string                       `json:"fundingTargetAddress"`
	RouteBindingID       string                       `json:"routeBindingID"`
	BuyerPeerID          string                       `json:"buyerPeerID"`
	BuyerRefundAddress   string                       `json:"buyerRefundAddress,omitempty"`
	SellerPeerID         string                       `json:"sellerPeerID"`
	ModeratorPeerID      string                       `json:"moderatorPeerID,omitempty"`
	ModeratorFee         *PaymentAttemptSettlementFee `json:"moderatorFee,omitempty"`
	EscrowTimeoutHours   uint32                       `json:"escrowTimeoutHours,omitempty"`
	EscrowUnlockUnix     int64                        `json:"escrowUnlockUnix,omitempty"`
	SellerAddress        string                       `json:"sellerAddress"`
	SellerGrossBasis     string                       `json:"sellerGrossBasis"`
	PlatformReleaseFee   PaymentAttemptSettlementFee  `json:"platformReleaseFee"`
	BuyerCancellationFee PaymentAttemptSettlementFee  `json:"buyerCancellationFee"`
	Affiliate            *PaymentAttemptAffiliateTerm `json:"affiliate,omitempty"`
	DisputePolicy        string                       `json:"disputePolicy"`
}

type PaymentAttemptSettlementFee struct {
	Address string `json:"address,omitempty"`
	Amount  string `json:"amount"`
}

type PaymentAttemptAffiliateTerm struct {
	ReferralSessionID string                            `json:"referralSessionID"`
	ProgramID         string                            `json:"programID"`
	PromoterPeerID    string                            `json:"promoterPeerID"`
	BuyerPeerID       string                            `json:"buyerPeerID"`
	CommissionRateBPS uint32                            `json:"commissionRateBPS"`
	Address           string                            `json:"address"`
	Amount            string                            `json:"amount"`
	SellerGrossBasis  string                            `json:"sellerGrossBasis"`
	Lines             []PaymentAttemptAffiliateLineTerm `json:"lines"`
}

type PaymentAttemptAffiliateLineTerm struct {
	OrderLineID          string `json:"orderLineID"`
	NetMerchandiseAtomic string `json:"netMerchandiseAtomic"`
	CommissionAtomic     string `json:"commissionAtomic"`
}

func (t PaymentAttemptSettlementTerms) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := t.Validate(); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(t)
	if err != nil {
		return nil, "", fmt.Errorf("encode payment attempt settlement terms: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

func (t PaymentAttemptSettlementTerms) Validate() error {
	if (t.Version != PaymentAttemptSettlementTermsVersion &&
		t.Version != PaymentAttemptSettlementTermsQuoteBoundVersion) ||
		strings.TrimSpace(t.OrderID) == "" || strings.TrimSpace(t.AttemptID) == "" ||
		strings.TrimSpace(t.AssetID) == "" || strings.TrimSpace(t.FundingTargetAddress) == "" ||
		strings.TrimSpace(t.RouteBindingID) == "" ||
		strings.TrimSpace(t.SellerAddress) == "" ||
		strings.TrimSpace(t.DisputePolicy) != DisputeScalingSellerAwardProRataFloor {
		return fmt.Errorf("invalid payment attempt settlement terms identity")
	}
	if (t.Version == PaymentAttemptSettlementTermsVersion && strings.TrimSpace(t.FundingBasisHash) != "") ||
		(t.Version == PaymentAttemptSettlementTermsQuoteBoundVersion && !validCanonicalSHA256Hex(strings.TrimSpace(t.FundingBasisHash))) {
		return fmt.Errorf("invalid payment attempt funding-basis commitment")
	}
	buyerPeerID := strings.TrimSpace(t.BuyerPeerID)
	sellerPeerID := strings.TrimSpace(t.SellerPeerID)
	for role, peerID := range map[string]string{"buyer": buyerPeerID, "seller": sellerPeerID} {
		if _, err := peer.Decode(strings.TrimSpace(peerID)); err != nil {
			return fmt.Errorf("invalid %s peer ID", role)
		}
	}
	if buyerPeerID == sellerPeerID {
		return fmt.Errorf("buyer and seller settlement participants must differ")
	}
	moderatorPeerID := strings.TrimSpace(t.ModeratorPeerID)
	solanaRail := strings.HasPrefix(strings.TrimSpace(t.AssetID), "crypto:solana:")
	if solanaRail || strings.TrimSpace(t.BuyerRefundAddress) != "" {
		if err := ValidateRefundAddress(iwallet.CoinType(t.AssetID), strings.TrimSpace(t.BuyerRefundAddress)); err != nil {
			return fmt.Errorf("invalid buyer refund address: %w", err)
		}
	}
	if t.EscrowUnlockUnix < 0 || (solanaRail && (t.EscrowUnlockUnix == 0 || t.EscrowTimeoutHours == 0)) ||
		(!solanaRail && t.EscrowUnlockUnix != 0) {
		return fmt.Errorf("invalid escrow unlock time")
	}
	if moderatorPeerID != "" {
		if _, err := peer.Decode(moderatorPeerID); err != nil {
			return fmt.Errorf("invalid moderator peer ID")
		}
		if moderatorPeerID == buyerPeerID || moderatorPeerID == sellerPeerID {
			return fmt.Errorf("moderator settlement participant must differ from buyer and seller")
		}
		if t.ModeratorFee != nil && strings.TrimSpace(t.ModeratorFee.Address) == "" {
			return fmt.Errorf("moderator payout address is required when moderator fee terms are present")
		}
	} else if t.ModeratorFee != nil {
		return fmt.Errorf("unmoderated settlement terms cannot include moderator payout terms")
	}
	funding, err := settlementAtomicAmount(t.FundingAmount, true)
	if err != nil {
		return fmt.Errorf("invalid funding amount: %w", err)
	}
	sellerGross, err := settlementAtomicAmount(t.SellerGrossBasis, true)
	if err != nil || sellerGross.Cmp(funding) > 0 {
		return fmt.Errorf("invalid seller gross basis")
	}
	platform, err := validateSettlementFee(t.PlatformReleaseFee)
	if err != nil {
		return fmt.Errorf("invalid platform release fee: %w", err)
	}
	cancel, err := validateSettlementFee(t.BuyerCancellationFee)
	if err != nil {
		return fmt.Errorf("invalid buyer cancel fee: %w", err)
	}
	if cancel.Cmp(funding) >= 0 {
		return fmt.Errorf("buyer cancel fee must be less than funding amount")
	}
	if t.ModeratorFee != nil {
		moderatorFee, err := validateSettlementFee(*t.ModeratorFee)
		if err != nil || moderatorFee.Cmp(funding) >= 0 {
			return fmt.Errorf("invalid moderator fee")
		}
	}
	deductions := new(big.Int).Set(platform)
	if t.Affiliate != nil {
		if strings.TrimSpace(t.Affiliate.BuyerPeerID) != buyerPeerID {
			return fmt.Errorf("affiliate buyer does not match settlement buyer")
		}
		affiliate, _, err := validatePaymentAttemptAffiliateTerm(t.Affiliate, sellerGross)
		if err != nil {
			return err
		}
		deductions.Add(deductions, affiliate)
	}
	if deductions.Cmp(sellerGross) >= 0 {
		return fmt.Errorf("seller-funded deductions must be less than seller gross basis")
	}
	return nil
}

func validatePaymentAttemptAffiliateTerm(term *PaymentAttemptAffiliateTerm, sellerGross *big.Int) (*big.Int, *big.Int, error) {
	if term == nil || strings.TrimSpace(term.ReferralSessionID) == "" || strings.TrimSpace(term.ProgramID) == "" ||
		strings.TrimSpace(term.Address) == "" || term.CommissionRateBPS == 0 || term.CommissionRateBPS > 10000 ||
		len(term.Lines) == 0 {
		return nil, nil, fmt.Errorf("invalid affiliate identity")
	}
	for _, rawPeerID := range []string{term.PromoterPeerID, term.BuyerPeerID} {
		if _, err := peer.Decode(strings.TrimSpace(rawPeerID)); err != nil {
			return nil, nil, fmt.Errorf("invalid affiliate peer ID")
		}
	}
	affiliate, err := settlementAtomicAmount(term.Amount, false)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid affiliate amount: %w", err)
	}
	affiliateBasis, err := settlementAtomicAmount(term.SellerGrossBasis, true)
	if err != nil || affiliateBasis.Cmp(sellerGross) > 0 {
		return nil, nil, fmt.Errorf("invalid affiliate seller gross basis")
	}

	seen := make(map[string]struct{}, len(term.Lines))
	netTotal := new(big.Int)
	commissionTotal := new(big.Int)
	for _, line := range term.Lines {
		lineID := strings.TrimSpace(line.OrderLineID)
		if lineID == "" {
			return nil, nil, fmt.Errorf("invalid affiliate order line ID")
		}
		if _, exists := seen[lineID]; exists {
			return nil, nil, fmt.Errorf("duplicate affiliate order line ID")
		}
		seen[lineID] = struct{}{}
		netAmount, err := settlementAtomicAmount(line.NetMerchandiseAtomic, true)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid affiliate line basis: %w", err)
		}
		commission, err := settlementAtomicAmount(line.CommissionAtomic, false)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid affiliate line commission: %w", err)
		}
		expected := new(big.Int).Mul(netAmount, new(big.Int).SetUint64(uint64(term.CommissionRateBPS)))
		expected.Div(expected, big.NewInt(10000))
		if commission.Cmp(expected) != 0 {
			return nil, nil, fmt.Errorf("affiliate line commission does not match frozen rate")
		}
		netTotal.Add(netTotal, netAmount)
		commissionTotal.Add(commissionTotal, commission)
	}
	if netTotal.Cmp(affiliateBasis) != 0 || commissionTotal.Cmp(affiliate) != 0 {
		return nil, nil, fmt.Errorf("affiliate line totals do not match frozen terms")
	}
	return affiliate, affiliateBasis, nil
}

// SellerSigningPayload returns the domain-separated canonical bytes that the
// seller identity key authorizes before a funding target can be exposed.
func (t PaymentAttemptSettlementTerms) SellerSigningPayload() ([]byte, error) {
	canonical, _, err := t.CanonicalBytesAndHash()
	if err != nil {
		return nil, err
	}
	domain := settlementTermsSigningDomainV1
	if t.Version == PaymentAttemptSettlementTermsQuoteBoundVersion {
		domain = settlementTermsSigningDomainV2
	}
	payload := make([]byte, 0, len(domain)+len(canonical))
	payload = append(payload, domain...)
	payload = append(payload, canonical...)
	return payload, nil
}

// VerifySellerAuthorization verifies that signature was produced by the
// seller PeerID bound into these exact settlement terms.
func (t PaymentAttemptSettlementTerms) VerifySellerAuthorization(signerPeerID string, signature []byte) error {
	signerPeerID = strings.TrimSpace(signerPeerID)
	if signerPeerID == "" || signerPeerID != strings.TrimSpace(t.SellerPeerID) || len(signature) == 0 {
		return fmt.Errorf("invalid seller settlement terms authorization")
	}
	pid, err := peer.Decode(signerPeerID)
	if err != nil {
		return fmt.Errorf("decode seller settlement terms signer: %w", err)
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extract seller settlement terms public key: %w", err)
	}
	payload, err := t.SellerSigningPayload()
	if err != nil {
		return err
	}
	valid, err := pubKey.Verify(payload, signature)
	if err != nil {
		return fmt.Errorf("verify seller settlement terms signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid seller settlement terms signature")
	}
	return nil
}

func validateSettlementFee(fee PaymentAttemptSettlementFee) (*big.Int, error) {
	amount, err := settlementAtomicAmount(fee.Amount, false)
	if err != nil {
		return nil, err
	}
	if amount.Sign() > 0 && strings.TrimSpace(fee.Address) == "" {
		return nil, fmt.Errorf("fee address is required")
	}
	if amount.Sign() == 0 && strings.TrimSpace(fee.Address) != "" {
		return nil, fmt.Errorf("zero fee must not declare an address")
	}
	return amount, nil
}

func settlementAtomicAmount(raw string, positive bool) (*big.Int, error) {
	raw = strings.TrimSpace(raw)
	amount, ok := new(big.Int).SetString(raw, 10)
	if !ok || amount.Sign() < 0 || (positive && amount.Sign() == 0) || amount.String() != raw {
		return nil, fmt.Errorf("amount must be canonical base-10 atomic units")
	}
	return amount, nil
}
