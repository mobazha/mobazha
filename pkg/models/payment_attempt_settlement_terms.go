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
)

const (
	PaymentAttemptSettlementTermsVersion  = 1
	DisputeScalingSellerAwardProRataFloor = "seller_award_pro_rata_floor"
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
	RouteBindingID       string                       `json:"routeBindingID"`
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
	Address          string `json:"address"`
	Amount           string `json:"amount"`
	SellerGrossBasis string `json:"sellerGrossBasis"`
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
	if t.Version != PaymentAttemptSettlementTermsVersion ||
		strings.TrimSpace(t.OrderID) == "" || strings.TrimSpace(t.AttemptID) == "" ||
		strings.TrimSpace(t.AssetID) == "" || strings.TrimSpace(t.RouteBindingID) == "" ||
		strings.TrimSpace(t.SellerAddress) == "" ||
		strings.TrimSpace(t.DisputePolicy) != DisputeScalingSellerAwardProRataFloor {
		return fmt.Errorf("invalid payment attempt settlement terms identity")
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
	deductions := new(big.Int).Set(platform)
	if t.Affiliate != nil {
		if strings.TrimSpace(t.Affiliate.Address) == "" {
			return fmt.Errorf("invalid affiliate address")
		}
		affiliate, err := settlementAtomicAmount(t.Affiliate.Amount, true)
		if err != nil {
			return fmt.Errorf("invalid affiliate amount: %w", err)
		}
		affiliateBasis, err := settlementAtomicAmount(t.Affiliate.SellerGrossBasis, true)
		if err != nil || affiliateBasis.Cmp(sellerGross) > 0 {
			return fmt.Errorf("invalid affiliate seller gross basis")
		}
		deductions.Add(deductions, affiliate)
	}
	if deductions.Cmp(sellerGross) >= 0 {
		return fmt.Errorf("seller-funded deductions must be less than seller gross basis")
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
