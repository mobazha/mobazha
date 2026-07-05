// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestCredentialRefreshStateClaimsObservesAndSelectsDueWork(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.CollateralCredentialRefreshRecord{}); err != nil {
			return err
		}
		return tx.Migrate(&models.CollateralAllocationCredentialRecord{})
	}))
	now := time.Now().UTC().Truncate(time.Second)
	const (
		orderID     = "order-refresh"
		extensionID = "extension-refresh"
		audience    = "buyer-peer"
	)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		claimed, err := ClaimCredentialRequestTx(tx, orderID, extensionID, 1, audience, "request-1", now, 2*time.Minute)
		require.True(t, claimed)
		return err
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		claimed, err := ClaimCredentialRequestTx(tx, orderID, extensionID, 1, audience, "request-2", now.Add(time.Minute), 2*time.Minute)
		require.False(t, claimed)
		return err
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		candidates, err := DueCredentialRefreshesTx(tx, audience, now, now.Add(8*time.Minute), now.Add(time.Minute), 10)
		require.Len(t, candidates, 1)
		return err
	}))

	credential := pkgcollateral.AllocationCredential{
		CredentialID: "credential-1", AudiencePeerID: audience,
		ExtensionRevision: 1, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(10 * time.Minute).Unix(),
		AccountExpiresAtUnix: now.Add(time.Hour).Unix(),
		Allocation:           pkgcollateral.AllocationReference{OrderID: orderID, ExtensionID: extensionID},
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return ObserveImportedCredentialTx(tx, credential, now)
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		candidates, err := DueCredentialRefreshesTx(tx, audience, now, now.Add(5*time.Minute), now.Add(time.Minute), 10)
		require.Empty(t, candidates)
		return err
	}))

	longer := credential
	longer.CredentialID = "credential-2"
	longer.ExpiresAtUnix = now.Add(12 * time.Minute).Unix()
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := PersistAllocationCredentialTx(tx, CredentialDirectionImported, credential, now); err != nil {
			return err
		}
		if err := PersistAllocationCredentialTx(tx, CredentialDirectionImported, longer, now.Add(time.Second)); err != nil {
			return err
		}
		return ObserveImportedCredentialTx(tx, longer, now.Add(time.Second))
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		latest, err := ImportedAllocationCredentialTx(tx, orderID, extensionID, 1, audience)
		require.Equal(t, longer.CredentialID, latest.CredentialID)
		return err
	}))
	var due CredentialRefreshCandidate
	require.NoError(t, db.View(func(tx database.Tx) error {
		candidates, err := DueCredentialRefreshesTx(tx, audience, now, now.Add(13*time.Minute), now.Add(3*time.Minute), 10)
		require.Len(t, candidates, 1)
		require.Equal(t, longer.CredentialID, candidates[0].CredentialID)
		due = candidates[0]
		return err
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		candidates, err := DueCredentialRefreshesTx(tx, audience, now.Add(2*time.Hour), now.Add(3*time.Hour), now.Add(2*time.Hour), 10)
		require.Empty(t, candidates, "expired collateral accounts do not occupy refresh batches")
		return err
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return DismissCredentialRefreshTx(tx, due)
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		candidates, err := DueCredentialRefreshesTx(tx, audience, now, now.Add(13*time.Minute), now.Add(3*time.Minute), 10)
		require.Empty(t, candidates)
		return err
	}))
}

func TestClaimCredentialRequestTxRejectsStaleCrossInstanceClaim(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.CollateralCredentialRefreshRecord{})
	}))
	now := time.Now().UTC().Truncate(time.Second)
	const (
		orderID     = "order-refresh-cas"
		extensionID = "extension-refresh-cas"
		audience    = "buyer-peer-cas"
	)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		claimed, err := ClaimCredentialRequestTx(tx, orderID, extensionID, 1, audience, "request-original", now, 2*time.Minute)
		require.True(t, claimed)
		return err
	}))

	var stale models.CollateralCredentialRefreshRecord
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"order_id = ? AND extension_id = ? AND extension_revision = ? AND audience_peer_id = ?",
			orderID, extensionID, 1, audience,
		).First(&stale).Error
	}))
	competingAt := now.Add(3 * time.Minute)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		rows, err := tx.UpdateColumns(map[string]interface{}{
			"last_request_message_id": "request-competing",
			"last_requested_at":       competingAt,
			"updated_at":              competingAt,
		}, map[string]interface{}{
			"order_id = ?":           orderID,
			"extension_id = ?":       extensionID,
			"extension_revision = ?": 1,
			"audience_peer_id = ?":   audience,
		}, &models.CollateralCredentialRefreshRecord{})
		require.Equal(t, int64(1), rows)
		return err
	}))

	require.NoError(t, db.Update(func(tx database.Tx) error {
		claimed, err := claimExistingCredentialRequestTx(tx, stale, "request-stale", competingAt)
		require.False(t, claimed)
		return err
	}))
	require.NoError(t, db.View(func(tx database.Tx) error {
		var current models.CollateralCredentialRefreshRecord
		if err := tx.Read().Where(
			"order_id = ? AND extension_id = ? AND extension_revision = ? AND audience_peer_id = ?",
			orderID, extensionID, 1, audience,
		).First(&current).Error; err != nil {
			return err
		}
		require.Equal(t, "request-competing", current.LastRequestMessageID)
		require.Equal(t, competingAt, current.LastRequestedAt)
		return nil
	}))
}
