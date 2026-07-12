// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import "time"

// WalletAddressCursor tracks the next derivation index for one tenant, rail,
// and account role. It is wallet infrastructure, not Guest or Affiliate
// business state.
type WalletAddressCursor struct {
	TenantID    string `gorm:"column:tenant_id;type:varchar(255);primaryKey;default:''" json:"-"`
	RailID      string `gorm:"column:rail_id;type:varchar(255);primaryKey"`
	AccountRole string `gorm:"column:account_role;type:varchar(32);primaryKey"`
	NextIndex   uint32 `gorm:"column:next_index;not null"`
}

// TableName overrides the default GORM table name.
func (WalletAddressCursor) TableName() string { return "wallet_address_cursors" }

// WalletAddressReservation persists a deterministic receiving destination and
// its caller reference. The composite primary key makes ReserveAddress
// idempotent; the index constraint prevents accidental address reuse.
type WalletAddressReservation struct {
	TenantID     string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;default:'';uniqueIndex:uidx_wallet_address_reservation_index,priority:1;uniqueIndex:uidx_wallet_address_reservation_reference,priority:1" json:"-"`
	RailID       string    `gorm:"column:rail_id;type:varchar(255);primaryKey;uniqueIndex:uidx_wallet_address_reservation_index,priority:2"`
	AccountRole  string    `gorm:"column:account_role;type:varchar(32);primaryKey;uniqueIndex:uidx_wallet_address_reservation_index,priority:3"`
	ReferenceID  string    `gorm:"column:reference_id;type:varchar(255);primaryKey;uniqueIndex:uidx_wallet_address_reservation_reference,priority:2"`
	AddressIndex uint32    `gorm:"column:address_index;not null;uniqueIndex:uidx_wallet_address_reservation_index,priority:4"`
	Address      string    `gorm:"column:address;type:text;not null"`
	Tag          string    `gorm:"column:tag;type:text"`
	Version      uint32    `gorm:"column:version;not null;default:1"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName overrides the default GORM table name.
func (WalletAddressReservation) TableName() string { return "wallet_address_reservations" }
