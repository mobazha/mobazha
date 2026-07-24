// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// PaymentAttemptSettlementAuthorizationQuoteBoundVersion adds the complete
// funding basis while retaining the v1 participant authorization bundle.
const PaymentAttemptSettlementAuthorizationQuoteBoundVersion = 2

// PaymentAttemptSettlementAuthorization is the complete public snapshot a
// seller sends to the buyer after freezing one payment attempt. Both peers
// validate and persist the same canonical value before the target is exposed.
type PaymentAttemptSettlementAuthorization struct {
	Version       uint32                            `json:"version"`
	FundingBasis  *PaymentAttemptFundingBasis       `json:"fundingBasis,omitempty"`
	Terms         PaymentAttemptSettlementTerms     `json:"terms"`
	Target        PaymentAttemptFundingTarget       `json:"target"`
	Authorization PaymentAttemptAuthorizationBundle `json:"authorization"`
}

// NewPaymentAttemptSettlementAuthorization selects the v1 or quote-bound v2
// authorization envelope from whether an immutable funding basis is present.
func NewPaymentAttemptSettlementAuthorization(
	terms PaymentAttemptSettlementTerms,
	target PaymentAttemptFundingTarget,
	bundle PaymentAttemptAuthorizationBundle,
	fundingBasis *PaymentAttemptFundingBasis,
) PaymentAttemptSettlementAuthorization {
	version := uint32(SettlementAuthorizationVersion)
	if fundingBasis != nil {
		version = PaymentAttemptSettlementAuthorizationQuoteBoundVersion
	}
	return PaymentAttemptSettlementAuthorization{
		Version: version, FundingBasis: fundingBasis, Terms: terms, Target: target, Authorization: bundle,
	}
}

// CanonicalBytesAndHash validates and canonically encodes the complete public
// settlement authorization snapshot.
func (a PaymentAttemptSettlementAuthorization) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := a.Validate(); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(a)
	if err != nil {
		return nil, "", fmt.Errorf("encode payment attempt settlement authorization: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

// Validate verifies the target, seller terms authorization, and final bundle
// all describe the same order, attempt, rail and canonical hashes.
func (a PaymentAttemptSettlementAuthorization) Validate() error {
	if a.Version != SettlementAuthorizationVersion && a.Version != PaymentAttemptSettlementAuthorizationQuoteBoundVersion {
		return fmt.Errorf("invalid payment attempt settlement authorization version")
	}
	if (a.Version == SettlementAuthorizationVersion &&
		(a.FundingBasis != nil || a.Terms.Version != PaymentAttemptSettlementTermsVersion)) ||
		(a.Version == PaymentAttemptSettlementAuthorizationQuoteBoundVersion &&
			(a.FundingBasis == nil || a.Terms.Version != PaymentAttemptSettlementTermsQuoteBoundVersion)) {
		return ErrPaymentAttemptSettlementTermsConflict
	}
	var fundingBasisHash string
	if a.FundingBasis != nil {
		_, hash, err := a.FundingBasis.CanonicalBytesAndHash()
		if err != nil {
			return err
		}
		fundingBasisHash = hash
	}
	_, termsHash, err := a.Terms.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	_, targetHash, err := a.Target.CanonicalBytesAndHash()
	if err != nil {
		return err
	}
	if err := a.Authorization.Validate(); err != nil {
		return err
	}
	if err := ValidateSettlementTermsOfferBindings(a.Terms, a.Authorization.Offers); err != nil {
		return err
	}
	if a.Terms.OrderID != a.Authorization.OrderID || a.Terms.AttemptID != a.Authorization.AttemptID ||
		a.Terms.AssetID != a.Authorization.RailID || a.Target.AttemptID != a.Authorization.AttemptID ||
		a.Target.AssetID != a.Authorization.RailID || a.Target.AmountAtomic != a.Terms.FundingAmount ||
		a.Target.Address != a.Terms.FundingTargetAddress || a.Authorization.SettlementTermsHash != termsHash ||
		a.Authorization.FundingTargetHash != targetHash {
		return ErrPaymentAttemptSettlementTermsConflict
	}
	if a.FundingBasis != nil &&
		(a.FundingBasis.OrderID != a.Terms.OrderID || a.FundingBasis.AttemptID != a.Terms.AttemptID ||
			a.FundingBasis.AuthorizationContextID != a.Authorization.AuthorizationContextID ||
			a.FundingBasis.PaymentAssetID != a.Terms.AssetID ||
			a.FundingBasis.BuyerPaymentTotal != a.Terms.FundingAmount ||
			a.Terms.FundingBasisHash != fundingBasisHash) {
		return ErrPaymentAttemptSettlementTermsConflict
	}
	return a.Terms.VerifySellerAuthorization(
		a.Authorization.SellerTermsSigner, a.Authorization.SellerTermsSignature,
	)
}
