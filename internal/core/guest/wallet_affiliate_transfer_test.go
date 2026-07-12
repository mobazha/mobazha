// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

type guestAffiliateWalletStub struct {
	receipt contracts.WalletTransfer
	err     error
	request contracts.WalletTransferRequest
}

func (*guestAffiliateWalletStub) Capabilities(context.Context, string) (contracts.WalletCapabilities, error) {
	return contracts.WalletCapabilities{}, nil
}
func (*guestAffiliateWalletStub) ReserveAddress(context.Context, string, contracts.WalletAccountRole, string) (contracts.ReservedDestination, error) {
	return contracts.ReservedDestination{}, nil
}
func (s *guestAffiliateWalletStub) Transfer(_ context.Context, request contracts.WalletTransferRequest) (contracts.WalletTransfer, error) {
	s.request = request
	return s.receipt, s.err
}
func (*guestAffiliateWalletStub) ReconcileTransfers(context.Context) error { return nil }

func TestGuestWalletAffiliateTransfer_ProjectsSettlementActionAndReorg(t *testing.T) {
	db := newGuestTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.SettlementAction{}))
	wallet := &guestAffiliateWalletStub{receipt: contracts.WalletTransfer{
		IdempotencyKey: "guest-wallet-affiliate:gst_affiliate_wallet",
		State:          contracts.WalletTransferConfirmed, TxHash: "wallet_tx_1", Confirmations: 2,
	}}
	service := &GuestOrderAppService{db: db, walletAccounts: wallet}
	order := models.GuestOrder{
		OrderToken: "gst_affiliate_wallet", PaymentCoin: "crypto:bip122:000000000019d6689c085ae165831e93:native",
		PaymentAddress: "source", PaymentAmount: "500000", AffiliatePayoutAddress: "affiliate",
		AffiliatePayoutAmount: "100000",
	}

	confirmed, err := service.settleGuestWalletAffiliate(t.Context(), order)
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Equal(t, contracts.AccountGuest, wallet.request.Role)
	assert.Equal(t, order.OrderToken, wallet.request.ReferenceID)
	assert.Equal(t, uint64(100000), wallet.request.Amount)
	assert.Equal(t, order.AffiliatePayoutAddress, wallet.request.Destination)

	var action models.SettlementAction
	require.NoError(t, db.gormDB.Where("action_id = ?", guestWalletAffiliateActionID(order.OrderToken)).First(&action).Error)
	assert.Equal(t, "confirmed", action.State)
	assert.Equal(t, "wallet_tx_1", action.TxHash)
	assert.NotNil(t, action.ConfirmedAt)
	require.Len(t, models.DecodeSettlementPayoutLines(action.ObservedLines), 1)
	assert.Equal(t, "affiliate", models.DecodeSettlementPayoutLines(action.ObservedLines)[0].Type)

	wallet.receipt.State = contracts.WalletTransferReorged
	wallet.receipt.Confirmations = 0
	confirmed, err = service.settleGuestWalletAffiliate(t.Context(), order)
	require.NoError(t, err)
	assert.False(t, confirmed)
	action = models.SettlementAction{}
	require.NoError(t, db.gormDB.Where("action_id = ?", guestWalletAffiliateActionID(order.OrderToken)).First(&action).Error)
	assert.Equal(t, "reorged", action.State)
	assert.Nil(t, action.ConfirmedAt)
}

func TestIsGuestWalletAffiliateOrder_ExcludesLegacySweepAndManagedEscrow(t *testing.T) {
	order := &models.GuestOrder{AffiliatePayoutAddress: "affiliate", AffiliatePayoutAmount: "1"}
	assert.True(t, isGuestWalletAffiliateOrder(order))
	order.SweepToAddress = "legacy-seller"
	assert.False(t, isGuestWalletAffiliateOrder(order))
	order.SweepToAddress = ""
	require.NoError(t, order.SetManagedEscrowGuestMetadata([]byte(`{"safe":true}`)))
	assert.False(t, isGuestWalletAffiliateOrder(order))
}
