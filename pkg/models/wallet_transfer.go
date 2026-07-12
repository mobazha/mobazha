// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import "time"

const MaxWalletTransferRetries = 5

// WalletTransfer is durable wallet infrastructure state. It deliberately does
// not encode Guest, Affiliate, order, commission, or payout business states.
type WalletTransfer struct {
	TenantID          string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;default:'';uniqueIndex:uidx_wallet_transfer_idempotency,priority:1" json:"-"`
	ID                string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	IdempotencyKey    string    `gorm:"column:idempotency_key;type:varchar(255);not null;uniqueIndex:uidx_wallet_transfer_idempotency,priority:2" json:"idempotencyKey"`
	RailID            string    `gorm:"column:rail_id;type:varchar(255);not null;index" json:"railId"`
	AccountRole       string    `gorm:"column:account_role;type:varchar(32);not null" json:"accountRole"`
	ReferenceID       string    `gorm:"column:reference_id;type:varchar(255);not null" json:"referenceId"`
	SourceAddress     string    `gorm:"column:source_address;type:text;not null" json:"sourceAddress"`
	AddressIndex      uint32    `gorm:"column:address_index;not null" json:"-"`
	Destination       string    `gorm:"column:destination;type:text;not null" json:"destination"`
	Amount            uint64    `gorm:"column:amount;not null" json:"amount"`
	SweepAll          bool      `gorm:"column:sweep_all;not null" json:"sweepAll"`
	FeeTargetBlocks   int       `gorm:"column:fee_target_blocks;not null" json:"feeTargetBlocks"`
	FeePerByte        uint64    `gorm:"column:fee_per_byte;not null" json:"feePerByte"`
	State             string    `gorm:"column:state;type:varchar(32);not null;index" json:"state"`
	RawTxHex          string    `gorm:"column:raw_tx_hex;type:text" json:"-"`
	TxHash            string    `gorm:"column:tx_hash;type:varchar(255)" json:"txHash,omitempty"`
	AttemptTxHashes   []string  `gorm:"column:attempt_tx_hashes;serializer:json;type:text" json:"attemptTxHashes,omitempty"`
	Confirmations     int       `gorm:"column:confirmations;not null" json:"confirmations"`
	RetryCount        int       `gorm:"column:retry_count;not null" json:"retryCount"`
	BuildAttempts     int       `gorm:"column:build_attempts;not null" json:"buildAttempts"`
	BroadcastAttempts int       `gorm:"column:broadcast_attempts;not null" json:"broadcastAttempts"`
	LastError         string    `gorm:"column:last_error;type:text" json:"lastError,omitempty"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (WalletTransfer) TableName() string { return "wallet_transfers" }

// WalletTransferInput reserves an outpoint before broadcast. The unique index
// prevents two concurrent transfers from signing the same input.
type WalletTransferInput struct {
	TenantID    string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;default:'';uniqueIndex:uidx_wallet_transfer_outpoint,priority:1" json:"-"`
	TransferID  string    `gorm:"column:transfer_id;type:varchar(64);primaryKey" json:"transferId"`
	RailID      string    `gorm:"column:rail_id;type:varchar(255);not null;uniqueIndex:uidx_wallet_transfer_outpoint,priority:2" json:"railId"`
	TxHash      string    `gorm:"column:tx_hash;type:varchar(255);primaryKey;uniqueIndex:uidx_wallet_transfer_outpoint,priority:3" json:"txHash"`
	OutputIndex uint32    `gorm:"column:output_index;primaryKey;uniqueIndex:uidx_wallet_transfer_outpoint,priority:4" json:"outputIndex"`
	Value       uint64    `gorm:"column:value;not null" json:"value"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (WalletTransferInput) TableName() string { return "wallet_transfer_inputs" }
