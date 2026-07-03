// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// TestManagedEscrowClosure_CapabilityAndReadinessE2E verifies Phase 3D gates:
// buyer-visible EVM only when funding, observation, settlement, relay, and receiving account are ready.
func TestManagedEscrowClosure_CapabilityAndReadinessE2E(t *testing.T) {
	db := newGuestTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.ReceivingAccount{}))
	require.NoError(t, db.gormDB.Create(&models.ReceivingAccount{
		TenantMixin: models.TenantMixin{TenantID: testTenantID},
		ID:          1,
		ChainType:   iwallet.ChainEthereum,
		IsActive:    true,
		Address:     "0x2222222222222222222222222222222222222222",
	}).Error)

	dp := NewDirectPaymentService(db, nil)
	dp.SetManagedEscrowFunding(testManagedEscrowProjector{}, testGuestOwnerResolver{})

	svc := &GuestOrderAppService{
		db:                      db,
		directPayment:           dp,
		evmObservationAvailable: true,
	}
	eth := iwallet.CoinType("crypto:eip155:1:native")
	info, err := iwallet.CoinInfoFromCoinType(eth)
	require.NoError(t, err)

	// Before runtime wiring: hidden from buyers and readiness.
	cap := svc.evaluateGuestPaymentCapability(eth, info)
	require.False(t, cap.BuyerVisible)
	report, err := svc.GetGuestCheckoutReadiness(context.Background())
	require.NoError(t, err)
	for _, ch := range report.EVMChains {
		if ch.Chain == string(iwallet.ChainEthereum) {
			require.False(t, ch.BuyerVisible)
		}
	}

	svc.SetManagedEscrowSettlement(testManagedEscrowSettlementService{})
	svc.SetManagedEscrowClosureRuntime(ManagedEscrowClosureRuntime{
		FundingReady:     true,
		ObservationReady: true,
		SettlementReady:  true,
		RelayReady:       true,
		ManagedEscrowMonitorChains: map[iwallet.ChainType]struct{}{
			iwallet.ChainEthereum: {},
		},
		RelayGasHealthyChains: map[iwallet.ChainType]struct{}{
			iwallet.ChainEthereum: {},
		},
	})

	cap = svc.evaluateGuestPaymentCapability(eth, info)
	require.True(t, cap.BuyerVisible, "reason=%q", cap.Reason)
	require.True(t, cap.CanSettleFunds)
	require.NoError(t, svc.validateBuyerVisibleCoin(eth, info, "ETH"))
	require.Equal(t, "ETH", svc.filterAvailableCoins("ETH,LTC,TRX"))

	report, err = svc.GetGuestCheckoutReadiness(context.Background())
	require.NoError(t, err)
	found := false
	for _, ch := range report.EVMChains {
		if ch.Chain != string(iwallet.ChainEthereum) {
			continue
		}
		found = true
		require.True(t, ch.BuyerVisible)
		require.True(t, ch.FundingReady)
		require.True(t, ch.SettlementReady)
		require.True(t, ch.RelayGasHealthy)
	}
	require.True(t, found)
}

func TestEvaluateGuestPaymentCapability_EVMHiddenWhenRelayGasUnhealthy(t *testing.T) {
	svc := newEVMClosureTestService(t, ManagedEscrowClosureRuntime{
		FundingReady:     true,
		ObservationReady: true,
		SettlementReady:  true,
		RelayReady:       true,
		ManagedEscrowMonitorChains: map[iwallet.ChainType]struct{}{
			iwallet.ChainEthereum: {},
		},
		RelayGasHealthyChains: nil,
	})
	svc.SetManagedEscrowSettlement(testManagedEscrowSettlementService{})
	eth := iwallet.CoinType("crypto:eip155:1:native")
	info, _ := iwallet.CoinInfoFromCoinType(eth)
	cap := svc.evaluateGuestPaymentCapability(eth, info)
	require.False(t, cap.BuyerVisible)
}
