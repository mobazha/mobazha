package models

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// PendingEscrowPaymentInfo stores escrow setup intent before PaymentSent is
// written. It carries settlement metadata for escrow rails such as Solana
// Anchor and legacy contract escrow.
type PendingEscrowPaymentInfo struct {
	Type                   string                 `json:"type"` // always "escrow"
	Coin                   string                 `json:"coin,omitempty"`
	Amount                 uint64                 `json:"amount,omitempty"`
	ContractAddress        string                 `json:"contractAddress,omitempty"`
	EscrowAddress          string                 `json:"escrowAddress,omitempty"`
	EscrowSeed             string                 `json:"escrowSeed,omitempty"`
	Moderator              string                 `json:"moderator,omitempty"`
	ModeratorAddress       string                 `json:"moderatorAddress,omitempty"`
	ModeratorPayoutAddress string                 `json:"moderatorPayoutAddress,omitempty"`
	ModeratorPayoutAmount  uint64                 `json:"moderatorPayoutAmount,omitempty"`
	PlatformFeeAddress     string                 `json:"platformFeeAddress,omitempty"`
	RentCollector          string                 `json:"rentCollector,omitempty"`
	UnlockTime             int64                  `json:"unlockTime,omitempty"`
	FundingDeadline        int64                  `json:"fundingDeadline,omitempty"`
	PlatformFeeAmount      uint64                 `json:"platformFeeAmount,omitempty"`
	AffiliatePayoutAddress string                 `json:"affiliatePayoutAddress,omitempty"`
	AffiliatePayoutAmount  uint64                 `json:"affiliatePayoutAmount,omitempty"`
	SettlementSpec         *PendingSettlementSpec `json:"settlementSpec,omitempty"`
}

// EncodeHexScript serializes the snapshot into the canonical hex-encoded JSON
// form carried by PaymentSent.Script and the persisted setup metadata. Every
// producer must use this single marshal site: the binding is validated by the
// counterparty and must stay byte-identical across order mirrors.
func (info *PendingEscrowPaymentInfo) EncodeHexScript() (string, error) {
	if info == nil {
		return "", fmt.Errorf("pending escrow payment info is nil")
	}
	info.Type = "escrow"
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// SetPendingEscrowPaymentInfo stores escrow payment intent in PendingPaymentInfo.
func (o *Order) SetPendingEscrowPaymentInfo(info *PendingEscrowPaymentInfo) error {
	if info == nil {
		o.PendingPaymentInfo = nil
		return nil
	}
	info.Type = "escrow"
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal pending escrow payment info: %w", err)
	}
	o.PendingPaymentInfo = data
	return nil
}

// GetPendingEscrowPaymentInfo retrieves escrow pending info.
// Returns (nil, nil) when absent or when PendingPaymentInfo belongs to another type.
func (o *Order) GetPendingEscrowPaymentInfo() (*PendingEscrowPaymentInfo, error) {
	if len(o.PendingPaymentInfo) == 0 {
		return nil, nil
	}
	var hint struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(o.PendingPaymentInfo, &hint); err != nil {
		return nil, nil
	}
	if hint.Type != "escrow" {
		return nil, nil
	}
	var info PendingEscrowPaymentInfo
	if err := json.Unmarshal(o.PendingPaymentInfo, &info); err != nil {
		return nil, fmt.Errorf("unmarshal pending escrow payment info: %w", err)
	}
	return &info, nil
}
