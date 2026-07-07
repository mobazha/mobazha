// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGuestCheckoutConfigAutoMigrateAddsAddressEncryptionDefault(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE guest_checkout_configs (
			tenant_id text DEFAULT '',
			id integer,
			enabled numeric,
			accepted_coins text,
			max_order_amount text,
			payment_timeout integer,
			pgp_public_key text,
			pgp_key_fingerprint text,
			pgp_key_version integer,
			pgp_encrypted_private_key text,
			PRIMARY KEY (tenant_id, id)
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO guest_checkout_configs
			(tenant_id, id, enabled, accepted_coins, max_order_amount, payment_timeout)
		VALUES ('', 1, 1, 'XMR', '0', 60)
	`).Error)

	require.NoError(t, db.AutoMigrate(&GuestCheckoutConfig{}))

	var config GuestCheckoutConfig
	require.NoError(t, db.First(&config, "tenant_id = ? AND id = ?", "", 1).Error)
	require.False(t, config.AddressEncryptionRequired)
}
