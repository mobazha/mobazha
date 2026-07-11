// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	PaymentAttemptFundingTargetVersion = 1
	PaymentAttemptFundingTargetAddress = "address"
)

// PaymentAttemptFundingTarget is the immutable, persistence-safe crypto
// funding target owned by one PaymentAttempt. Display-only instructions and
// provider secrets do not belong in this value.
type PaymentAttemptFundingTarget struct {
	Version      uint32 `json:"version"`
	AttemptID    string `json:"attemptID"`
	Type         string `json:"type"`
	AssetID      string `json:"assetID"`
	AmountAtomic string `json:"amountAtomic"`
	Address      string `json:"address"`
	MemoOrTag    string `json:"memoOrTag,omitempty"`
}

// CanonicalBytesAndHash validates and canonically encodes the target.
func (t PaymentAttemptFundingTarget) CanonicalBytesAndHash() ([]byte, string, error) {
	if err := t.Validate(); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(t)
	if err != nil {
		return nil, "", fmt.Errorf("encode payment attempt funding target: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

// Validate checks the immutable crypto address target contract.
func (t PaymentAttemptFundingTarget) Validate() error {
	if t.Version != PaymentAttemptFundingTargetVersion ||
		strings.TrimSpace(t.AttemptID) == "" ||
		strings.TrimSpace(t.Type) != PaymentAttemptFundingTargetAddress ||
		strings.TrimSpace(t.AssetID) == "" || strings.TrimSpace(t.Address) == "" {
		return fmt.Errorf("invalid payment attempt funding target identity")
	}
	if _, err := settlementAtomicAmount(t.AmountAtomic, true); err != nil {
		return fmt.Errorf("invalid funding target amount: %w", err)
	}
	return nil
}
