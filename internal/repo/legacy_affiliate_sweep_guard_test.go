// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package repo

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestAssertNoLegacyWalletReceivingState_FailsClosed(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.SweepTask{}); err != nil {
			return err
		}
		return tx.Create(&models.SweepTask{
			ID: 1, OrderToken: "legacy-affiliate", ChainKey: "BTC",
			KeySource: models.SweepKeySource("affiliate_escrow"), Status: models.SweepStatusPending,
		})
	}))

	err = db.Update(assertNoLegacyWalletReceivingState)
	require.ErrorContains(t, err, "legacy affiliate escrow sweep state exists")
}

func TestAssertNoLegacyWalletReceivingState_RejectsGuestBIP44Sweep(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.SweepTask{}); err != nil {
			return err
		}
		return tx.Create(&models.SweepTask{
			ID: 2, OrderToken: "legacy-guest", ChainKey: "BTC",
			KeySource: models.SweepKeySourceBIP44, Status: models.SweepStatusPending,
		})
	}))

	err = db.Update(assertNoLegacyWalletReceivingState)
	require.ErrorContains(t, err, "legacy guest sweep state exists")
}

func TestAssertNoLegacyWalletReceivingState_RejectsLegacyManagedEscrowOwner(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.GuestOrder{}); err != nil {
			return err
		}
		return tx.Create(&models.GuestOrder{
			ID: 1, OrderToken: "gst_legacy_safe", ManagedEscrowMetadata: []byte(`{"owner":"legacy"}`),
		})
	}))

	err = db.Update(assertNoLegacyWalletReceivingState)
	require.ErrorContains(t, err, "legacy guest managed escrow owner state exists")
}

func TestAssertNoLegacyWalletReceivingState_AcceptsSettlementDomainOwnerVersion(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.GuestOrder{}); err != nil {
			return err
		}
		return tx.Create(&models.GuestOrder{
			ID: 1, OrderToken: "gst_settlement_safe", ManagedEscrowMetadata: []byte(`{"owner":"settlement"}`),
			SettlementOwnerVersion: "settlement-domain-v1",
		})
	}))

	require.NoError(t, db.Update(assertNoLegacyWalletReceivingState))
}

func TestAssertNoLegacyWalletReceivingState_RejectsUnreservedGuestWalletAddress(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.GuestOrder{}); err != nil {
			return err
		}
		return tx.Create(&models.GuestOrder{
			ID: 1, OrderToken: "gst_legacy_address", PaymentCoin: string(rail), PaymentAddress: "legacy-address",
		})
	}))

	err = db.Update(assertNoLegacyWalletReceivingState)
	require.ErrorContains(t, err, "legacy guest payment address state exists")
}

func TestAssertNoLegacyWalletReceivingState_AcceptsReservedGuestWalletAddress(t *testing.T) {
	db, err := MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.GuestOrder{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.WalletAddressReservation{}); err != nil {
			return err
		}
		if err := tx.Create(&models.WalletAddressReservation{
			RailID: string(rail), AccountRole: "guest", ReferenceID: "gst_reserved_address",
			Address: "wallet-address", Version: 1,
		}); err != nil {
			return err
		}
		return tx.Create(&models.GuestOrder{
			ID: 1, OrderToken: "gst_reserved_address", PaymentCoin: string(rail), PaymentAddress: "wallet-address",
		})
	}))

	require.NoError(t, db.Update(assertNoLegacyWalletReceivingState))
}
