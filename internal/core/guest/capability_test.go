package guest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func newUTXOCapableService(t *testing.T, monitorHealthy, withReceiving bool) *GuestOrderAppService {
	t.Helper()
	db := newGuestTestDB(t)
	if withReceiving {
		require.NoError(t, db.gormDB.AutoMigrate(&models.ReceivingAccount{}))
		require.NoError(t, db.gormDB.Create(&models.ReceivingAccount{
			TenantMixin: models.TenantMixin{TenantID: testTenantID},
			ID:          1,
			ChainType:   iwallet.ChainLitecoin,
			IsActive:    true,
			Address:     "ltc1q_seller_cap",
		}).Error)
	}
	svc := &GuestOrderAppService{
		db: db,
		supportedUTXOChains: map[iwallet.ChainType]struct{}{
			iwallet.ChainLitecoin: {},
		},
		sweepService: NewAutoSweepService(db, nil, nil),
		multiwallet: &mockWalletOperator{
			wallet: &mockUTXOWallet{scriptPubKey: []byte{0x76, 0xa9, 0x14}},
		},
	}
	svc.SetUTXOMonitor(&stubUTXOMonitor{healthy: monitorHealthy, sources: 1})
	return svc
}

func TestEvaluateGuestPaymentCapability_UTXOVisibleWhenClosureReady(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	ltc := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	info, _ := iwallet.CoinInfoFromCoinType(ltc)

	cap := svc.evaluateGuestPaymentCapability(ltc, info)
	if !cap.BuyerVisible {
		t.Fatalf("BuyerVisible = false, reason=%q", cap.Reason)
	}
	if !cap.CanSettleFunds {
		t.Fatal("expected CanSettleFunds for sweepable UTXO")
	}
}

func TestEvaluateGuestPaymentCapability_UTXOHiddenWhenMonitorUnhealthy(t *testing.T) {
	svc := newUTXOCapableService(t, false, true)
	ltc := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	info, _ := iwallet.CoinInfoFromCoinType(ltc)

	cap := svc.evaluateGuestPaymentCapability(ltc, info)
	if cap.BuyerVisible {
		t.Fatal("UTXO must be hidden when monitor has no healthy sources")
	}
}

func TestEvaluateGuestPaymentCapability_UTXOHiddenWithoutMonitor(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	svc.SetUTXOMonitor(nil)
	ltc := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	info, _ := iwallet.CoinInfoFromCoinType(ltc)

	cap := svc.evaluateGuestPaymentCapability(ltc, info)
	if cap.BuyerVisible {
		t.Fatal("UTXO must be hidden when monitor is not configured")
	}
	if !errors.Is(cap.Err, contracts.ErrCoinUnavailable) {
		t.Fatalf("cap.Err = %v, want ErrCoinUnavailable", cap.Err)
	}
}

func TestEvaluateGuestPaymentCapability_UTXOHiddenWithoutReceivingAccount(t *testing.T) {
	svc := newUTXOCapableService(t, true, false)
	ltc := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	info, _ := iwallet.CoinInfoFromCoinType(ltc)

	cap := svc.evaluateGuestPaymentCapability(ltc, info)
	if cap.BuyerVisible {
		t.Fatal("UTXO must be hidden without active receiving account")
	}
}

func TestFilterAvailableCoins_HidesEVMAndRequiresUTXOClosure(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)

	got := svc.filterAvailableCoins("ETH,LTC,TRX,XMR,NOTREAL")
	if got != "LTC" {
		t.Fatalf("filterAvailableCoins() = %q, want LTC", got)
	}
}

func TestFilterAvailableCoins_HidesLTCWhenMonitorUnhealthy(t *testing.T) {
	svc := newUTXOCapableService(t, false, true)

	got := svc.filterAvailableCoins("ETH,LTC")
	if got != "" {
		t.Fatalf("filterAvailableCoins() = %q, want empty when monitor unhealthy", got)
	}
}

func newEVMClosureTestService(t *testing.T, runtime ManagedEscrowClosureRuntime) *GuestOrderAppService {
	t.Helper()
	db := newGuestTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(&models.ReceivingAccount{}))
	require.NoError(t, db.gormDB.Create(&models.ReceivingAccount{
		TenantMixin: models.TenantMixin{TenantID: testTenantID},
		ID:          1,
		ChainType:   iwallet.ChainEthereum,
		IsActive:    true,
		Address:     "0x1111111111111111111111111111111111111111",
	}).Error)
	dp := NewDirectPaymentService(db, nil)
	dp.SetManagedEscrowFunding(testManagedEscrowProjector{}, testGuestOwnerResolver{})
	svc := &GuestOrderAppService{
		db:                      db,
		directPayment:           dp,
		evmObservationAvailable: true,
	}
	svc.SetManagedEscrowClosureRuntime(runtime)
	return svc
}

func TestEvaluateGuestPaymentCapability_EVMHiddenWithoutClosureRuntime(t *testing.T) {
	svc := newEVMClosureTestService(t, ManagedEscrowClosureRuntime{})
	eth := iwallet.CoinType("crypto:eip155:1:native")
	info, _ := iwallet.CoinInfoFromCoinType(eth)

	cap := svc.evaluateGuestPaymentCapability(eth, info)
	if cap.BuyerVisible {
		t.Fatal("EVM must stay hidden until managed-escrow runtime readiness is ready")
	}
	if !errors.Is(cap.Err, contracts.ErrCoinUnavailable) {
		t.Fatalf("cap.Err = %v, want ErrCoinUnavailable", cap.Err)
	}
}

func TestEvaluateGuestPaymentCapability_EVMVisibleWhenClosureReady(t *testing.T) {
	svc := newEVMClosureReadyTestService(t)
	eth := iwallet.CoinType("crypto:eip155:1:native")
	info, _ := iwallet.CoinInfoFromCoinType(eth)

	cap := svc.evaluateGuestPaymentCapability(eth, info)
	require.True(t, cap.BuyerVisible, "reason=%q err=%v", cap.Reason, cap.Err)
	require.True(t, cap.CanSettleFunds)
}

func TestValidateBuyerVisibleCoin_ETHRequiresClosureRuntime(t *testing.T) {
	svc := newEVMClosureTestService(t, ManagedEscrowClosureRuntime{})
	eth := iwallet.CoinType("crypto:eip155:1:native")
	info, _ := iwallet.CoinInfoFromCoinType(eth)

	err := svc.validateBuyerVisibleCoin(eth, info, "ETH")
	require.Error(t, err)
	require.True(t, errors.Is(err, contracts.ErrCoinUnavailable) || errors.Is(err, contracts.ErrInvalidGuestRequest))
}

func newEVMClosureReadyTestService(t *testing.T) *GuestOrderAppService {
	t.Helper()
	svc := newEVMClosureTestService(t, ManagedEscrowClosureRuntime{
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
	svc.SetManagedEscrowSettlement(testManagedEscrowSettlementService{})
	return svc
}

func TestFilterAvailableCoins_IncludesETHWhenClosureReady(t *testing.T) {
	svc := newEVMClosureReadyTestService(t)
	got := svc.filterAvailableCoins("ETH,LTC")
	require.Equal(t, "ETH", got)
}

func TestGetGuestCheckoutReadiness_ReportsChainHealth(t *testing.T) {
	svc := newUTXOCapableService(t, true, true)
	svc.SetUTXOMonitor(&stubUTXOMonitor{healthy: true, sources: 2, watched: 4})

	report, err := svc.GetGuestCheckoutReadiness(context.Background())
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.Chains, 1)
	require.Equal(t, "LTC", report.Chains[0].Chain)
	require.Equal(t, 2, report.Chains[0].HealthySourceCount)
	require.True(t, report.Chains[0].WalletLoaded)
	require.True(t, report.Chains[0].ReceivingAccountActive)
	require.True(t, report.Chains[0].BuyerVisible)
	require.Equal(t, 4, report.WatchedAddressCount)
}

func TestGetGuestCheckoutReadiness_ReportsEVMClosure(t *testing.T) {
	svc := newEVMClosureReadyTestService(t)

	report, err := svc.GetGuestCheckoutReadiness(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, report.EVMChains)
	found := false
	for _, ch := range report.EVMChains {
		if ch.Chain == string(iwallet.ChainEthereum) {
			found = true
			require.True(t, ch.FundingReady)
			require.True(t, ch.ObservationReady)
			require.True(t, ch.SettlementReady)
			require.True(t, ch.RelayReady)
			require.True(t, ch.RelayGasHealthy)
			require.True(t, ch.ManagedEscrowMonitorActive)
			require.True(t, ch.ReceivingAccountActive)
			require.True(t, ch.BuyerVisible)
		}
	}
	require.True(t, found, "expected ETH chain readiness entry")
}
