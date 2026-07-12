// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type walletAccountServiceStub struct {
	gotRail      string
	gotRole      contracts.WalletAccountRole
	gotReference string
	result       contracts.ReservedDestination
}

func (*walletAccountServiceStub) Capabilities(context.Context, string) (contracts.WalletCapabilities, error) {
	return contracts.WalletCapabilities{Receive: true}, nil
}

func (*walletAccountServiceStub) Transfer(context.Context, contracts.WalletTransferRequest) (contracts.WalletTransfer, error) {
	return contracts.WalletTransfer{}, nil
}

func (*walletAccountServiceStub) ReconcileTransfers(context.Context) error { return nil }

func (s *walletAccountServiceStub) ReserveAddress(
	_ context.Context,
	railID string,
	role contracts.WalletAccountRole,
	referenceID string,
) (contracts.ReservedDestination, error) {
	s.gotRail = railID
	s.gotRole = role
	s.gotReference = referenceID
	return s.result, nil
}

func TestDirectPaymentService_GuestUTXOUsesWalletAccountReservation(t *testing.T) {
	db := newGuestTestDB(t)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	accounts := &walletAccountServiceStub{result: contracts.ReservedDestination{
		Destination: contracts.Destination{RailID: string(rail), Address: "bc1qguestwallet", Version: 1},
		Index:       7,
	}}
	service := NewDirectPaymentService(db)
	service.SetWalletAccountService(accounts)

	result, err := service.GeneratePaymentAddress(t.Context(), PaymentAddressRequest{
		CoinType: rail, Amount: "1000", OrderToken: "guest-wallet-order", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "bc1qguestwallet", result.Address)
	assert.Equal(t, uint32(7), result.AddressIndex)
	assert.Empty(t, result.SweepTo)
	assert.Equal(t, string(rail), accounts.gotRail)
	assert.Equal(t, contracts.AccountGuest, accounts.gotRole)
	assert.Equal(t, "guest-wallet-order", accounts.gotReference)
}
