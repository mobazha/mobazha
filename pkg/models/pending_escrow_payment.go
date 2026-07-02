package models

import (
	"encoding/json"
	"fmt"
)

// PendingEscrowPaymentInfo stores escrow setup intent before PaymentSent is
// written. It carries settlement metadata for escrow rails such as Solana
// Anchor and legacy contract escrow.
type PendingEscrowPaymentInfo struct {
	Type                 string                 `json:"type"` // always "escrow"
	Coin                 string                 `json:"coin,omitempty"`
	Amount               uint64                 `json:"amount,omitempty"`
	ContractAddress      string                 `json:"contractAddress,omitempty"`
	EscrowAddress        string                 `json:"escrowAddress,omitempty"`
	Moderator            string                 `json:"moderator,omitempty"`
	ModeratorAddress     string                 `json:"moderatorAddress,omitempty"`
	PlatformFeeCollector string                 `json:"platformFeeCollector,omitempty"`
	RentCollector        string                 `json:"rentCollector,omitempty"`
	UnlockTime           int64                  `json:"unlockTime,omitempty"`
	FundingDeadline      int64                  `json:"fundingDeadline,omitempty"`
	EscrowServiceFee     uint64                 `json:"escrowServiceFee,omitempty"`
	SettlementSpec       *PendingSettlementSpec `json:"settlementSpec,omitempty"`
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
