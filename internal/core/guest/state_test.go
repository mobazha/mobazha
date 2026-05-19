package guest

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestRequiredConfsForCoin(t *testing.T) {
	tests := []struct {
		name     string
		coinType iwallet.CoinType
		expected int
	}{
		{"LTC native", iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native"), 3},
		{"BTC native", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"), 1},
		// EVM/Solana/TRON: 0 confs — see godoc on requiredConfsForCoin
		// (no confirmation polling implemented for these chains yet).
		{"ETH native", iwallet.CoinType("crypto:eip155:1:native"), 0},
		{"SOL native", iwallet.CoinType("crypto:solana:mainnet:native"), 0},
		{"BSC native", iwallet.CoinType("crypto:eip155:56:native"), 0},
		{"TRON native", iwallet.CoinType("crypto:tron:mainnet:native"), 0},
		{"EXTERNAL_PAYMENT native", iwallet.CoinType("crypto:external_payment:mainnet:native"), 10},
		{"unknown fallback", iwallet.CoinType("INVALID"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := requiredConfsForCoin(tt.coinType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHandlePaymentDetected_ZeroConfs_AtomicFunded(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_test_zero_confs",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:eip155:1:native",
		RequiredConfs: 0,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	err := svc.HandlePaymentDetected("gst_test_zero_confs", "0xabc123", nil)
	require.NoError(t, err)

	order := loadGuestOrder(t, db, "gst_test_zero_confs")
	assert.Equal(t, models.GuestOrderFunded, order.State)
	assert.Equal(t, "0xabc123", order.PaymentTxHash)
	assert.NotNil(t, order.FundedAt)
}

func TestHandlePaymentDetected_NonZeroConfs_StaysDetected(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_test_ltc",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native",
		RequiredConfs: 3,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	err := svc.HandlePaymentDetected("gst_test_ltc", "ltctxhash123", nil)
	require.NoError(t, err)

	order := loadGuestOrder(t, db, "gst_test_ltc")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State)
	assert.Equal(t, "ltctxhash123", order.PaymentTxHash)
	assert.Nil(t, order.FundedAt)
}

func TestHandleConfirmationUpdate_ReachesThreshold_Funded(t *testing.T) {
	db := newGuestTestDB(t)
	sweepSvc := &AutoSweepService{db: db}
	svc := &GuestOrderAppService{db: db, sweepService: sweepSvc}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:     "gst_test_confirm",
		State:          models.GuestOrderPaymentDetected,
		PaymentCoin:    "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native",
		PaymentTxHash:  "ltctx456",
		PaymentAddress: "ltc1q_payment_addr",
		PaymentAmount:  "500000",
		SweepToAddress: "ltc1q_seller_addr",
		AddressIndex:   7,
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(time.Hour),
	})

	err := svc.HandleConfirmationUpdate("gst_test_confirm", 2)
	require.NoError(t, err)
	order := loadGuestOrder(t, db, "gst_test_confirm")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State)
	assert.Equal(t, 2, order.Confirmations)

	err = svc.HandleConfirmationUpdate("gst_test_confirm", 3)
	require.NoError(t, err)
	order = loadGuestOrder(t, db, "gst_test_confirm")
	assert.Equal(t, models.GuestOrderFunded, order.State)
	assert.Equal(t, 3, order.Confirmations)
	assert.NotNil(t, order.FundedAt)

	var task models.SweepTask
	require.NoError(t, db.gormDB.Where("order_token = ?", "gst_test_confirm").First(&task).Error,
		"SweepTask should be created when order transitions to Funded")
	assert.Equal(t, "ltc1q_payment_addr", task.FromAddress)
	assert.Equal(t, "ltc1q_seller_addr", task.ToAddress)
	assert.Equal(t, "500000", task.Amount)
	assert.Equal(t, uint32(7), task.AddressIndex)
	assert.Equal(t, models.SweepStatusPending, task.Status)
}

func TestHandlePaymentDetected_IdempotentForLaterStates(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	cases := []struct {
		name  string
		id    int
		state models.GuestOrderState
	}{
		{"funded", 100, models.GuestOrderFunded},
		{"payment_detected", 101, models.GuestOrderPaymentDetected},
		{"shipped", 102, models.GuestOrderShipped},
		{"completed", 103, models.GuestOrderCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := "gst_idempotent_" + tc.name
			seedGuestOrder(t, db, tc.id, models.GuestOrder{
				OrderToken:    token,
				State:         tc.state,
				PaymentCoin:   "crypto:eip155:1:native",
				RequiredConfs: 0,
				ExpiresAt:     time.Now().Add(time.Hour),
			})

			err := svc.HandlePaymentDetected(token, "0xtx", nil)
			require.NoError(t, err, "should be idempotent for state %s", tc.state)
		})
	}
}

func TestHandlePaymentDetected_WrongState_Error(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 200, models.GuestOrder{
		OrderToken:    "gst_expired",
		State:         models.GuestOrderExpired,
		PaymentCoin:   "crypto:eip155:1:native",
		RequiredConfs: 0,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	err := svc.HandlePaymentDetected("gst_expired", "0xtx", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state mismatch")
}

// TestHandlePoolPayment_KeepsAwaitingState exercises the EXTERNAL_PAYMENT pool-stage UX
// hint contract: HandlePoolPayment must NOT transition state out of
// AWAITING_PAYMENT, must populate the pool-stage fields for the buyer-facing
// status response, and must be idempotent across repeated polls.
func TestHandlePoolPayment_KeepsAwaitingState(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 300, models.GuestOrder{
		OrderToken:    "gst_external_payment_pool",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:external_payment:mainnet:native",
		RequiredConfs: 10,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	// Phase 1: pool detection — state must stay AWAITING_PAYMENT,
	// PoolTxHash + PoolAmount + PoolDetectedAt populated.
	err := svc.HandlePoolPayment("gst_external_payment_pool", "external_paymenttxhash001", 50_000_000_000)
	require.NoError(t, err)
	order := loadGuestOrder(t, db, "gst_external_payment_pool")
	assert.Equal(t, models.GuestOrderAwaitingPayment, order.State,
		"pool observation must NOT transition state — preserves CleanupExpiredOrders sweep semantics")
	assert.Equal(t, "external_paymenttxhash001", order.PoolTxHash)
	assert.Equal(t, uint64(50_000_000_000), order.PoolAmount)
	require.NotNil(t, order.PoolDetectedAt)
	firstDetectedAt := *order.PoolDetectedAt

	// Phase 1 idempotent: same (txHash, amount) is a no-op; PoolDetectedAt
	// must NOT churn to keep the buyer-facing timestamp stable across polls.
	time.Sleep(2 * time.Millisecond)
	err = svc.HandlePoolPayment("gst_external_payment_pool", "external_paymenttxhash001", 50_000_000_000)
	require.NoError(t, err)
	order = loadGuestOrder(t, db, "gst_external_payment_pool")
	require.NotNil(t, order.PoolDetectedAt)
	assert.True(t, firstDetectedAt.Equal(*order.PoolDetectedAt),
		"identical pool observations must be no-ops to keep PoolDetectedAt stable")

	// Phase 2: confirmed detection upgrades state and persists block height.
	// PoolDetectedAt is preserved (it's a UX hint about when we first saw the tx).
	opts := &contracts.PaymentDetectedOpts{TxBlockHeight: 12345}
	err = svc.HandlePaymentDetected("gst_external_payment_pool", "external_paymenttxhash001", opts)
	require.NoError(t, err)
	order = loadGuestOrder(t, db, "gst_external_payment_pool")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State,
		"state advances on confirmed detection")
	assert.Equal(t, uint64(12345), order.ExternalPaymentTxHeight)
	assert.Equal(t, "external_paymenttxhash001", order.PaymentTxHash)
	require.NotNil(t, order.PoolDetectedAt, "PoolDetectedAt is preserved post-confirmation")

	// Phase 3: pool poll fires AFTER confirmed (race during poll cadence).
	// HandlePoolPayment must be a no-op on non-AWAITING orders to avoid
	// stomping on the on-chain state machine.
	err = svc.HandlePoolPayment("gst_external_payment_pool", "external_paymenttxhash001", 60_000_000_000)
	require.NoError(t, err)
	order = loadGuestOrder(t, db, "gst_external_payment_pool")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State, "state unchanged")
	assert.Equal(t, uint64(50_000_000_000), order.PoolAmount,
		"PoolAmount frozen post-confirmation — on-chain state owns truth")
}

func TestHandlePaymentDetected_EXTERNAL_PAYMENT_DirectConfirmed(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 301, models.GuestOrder{
		OrderToken:    "gst_external_payment_direct",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:external_payment:mainnet:native",
		RequiredConfs: 10,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	// Skip pool, go directly to confirmed detection
	opts := &contracts.PaymentDetectedOpts{TxBlockHeight: 99000}
	err := svc.HandlePaymentDetected("gst_external_payment_direct", "external_paymenttxhash002", opts)
	require.NoError(t, err)
	order := loadGuestOrder(t, db, "gst_external_payment_direct")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State)
	assert.Equal(t, uint64(99000), order.ExternalPaymentTxHeight)
	assert.Equal(t, "external_paymenttxhash002", order.PaymentTxHash)
}

func TestValidateCoinAvailability(t *testing.T) {
	private_distributionSvc := &GuestOrderAppService{
		supportedUTXOChains:    toChainSet(nil),
		evmMonitorAvailable:    false,
		solanaMonitorAvailable: false,
	}

	fullBuildSvc := &GuestOrderAppService{
		supportedUTXOChains:    toChainSet([]iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainLitecoin, iwallet.ChainBitcoinCash, iwallet.ChainZCash}),
		evmMonitorAvailable:    true,
		solanaMonitorAvailable: true,
	}

	ltcCoin := iwallet.CoinType("crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native")
	btcCoin := iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	ethCoin := iwallet.CoinType("crypto:eip155:1:native")
	solCoin := iwallet.CoinType("crypto:solana:mainnet:native")
	tronCoin := iwallet.CoinType("crypto:tron:mainnet:native")

	ltcInfo, _ := iwallet.CoinInfoFromCoinType(ltcCoin)
	btcInfo, _ := iwallet.CoinInfoFromCoinType(btcCoin)
	ethInfo, _ := iwallet.CoinInfoFromCoinType(ethCoin)
	solInfo, _ := iwallet.CoinInfoFromCoinType(solCoin)
	tronInfo, _ := iwallet.CoinInfoFromCoinType(tronCoin)

	t.Run("PrivateDistribution rejects LTC (EXTERNAL_PAYMENT-only)", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(ltcCoin, ltcInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("PrivateDistribution rejects BTC", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(btcCoin, btcInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("PrivateDistribution rejects ETH", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(ethCoin, ethInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "EVM/TRON balance monitor not configured")
	})

	t.Run("PrivateDistribution rejects SOL", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(solCoin, solInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Solana reference checker not configured")
	})

	t.Run("PrivateDistribution rejects TRON", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(tronCoin, tronInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TRON balance monitor not configured")
	})

	t.Run("Full build allows BTC", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(btcCoin, btcInfo)
		assert.NoError(t, err)
	})

	t.Run("Full build allows LTC", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(ltcCoin, ltcInfo)
		assert.NoError(t, err)
	})

	t.Run("Full build allows ETH", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(ethCoin, ethInfo)
		assert.NoError(t, err)
	})

	t.Run("Full build allows SOL", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(solCoin, solInfo)
		assert.NoError(t, err)
	})

	t.Run("Full build rejects TRON until implemented", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(tronCoin, tronInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TRON balance monitor not configured")
	})

	external_paymentCoin := iwallet.CoinType("crypto:external_payment:mainnet:native")
	external_paymentInfo, _ := iwallet.CoinInfoFromCoinType(external_paymentCoin)

	t.Run("PrivateDistribution rejects EXTERNAL_PAYMENT without client", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(external_paymentCoin, external_paymentInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ExternalPayment wallet-rpc not configured")
	})

	external_paymentSvc := &GuestOrderAppService{
		supportedUTXOChains: toChainSet([]iwallet.ChainType{iwallet.ChainLitecoin}),
		external_paymentAvailable:     func() bool { return true },
	}
	t.Run("EXTERNAL_PAYMENT allowed when client available and healthy", func(t *testing.T) {
		err := external_paymentSvc.validateCoinAvailability(external_paymentCoin, external_paymentInfo)
		assert.NoError(t, err)
	})

	external_paymentUnhealthy := &GuestOrderAppService{
		supportedUTXOChains: toChainSet([]iwallet.ChainType{iwallet.ChainLitecoin}),
		external_paymentAvailable:     func() bool { return false },
	}
	t.Run("EXTERNAL_PAYMENT rejected when client unhealthy", func(t *testing.T) {
		err := external_paymentUnhealthy.validateCoinAvailability(external_paymentCoin, external_paymentInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ExternalPayment wallet-rpc unreachable")
	})
}

// TestHandlePaymentDetected_ZeroConfs_EmitsOrderConfirmation verifies the
// digital-goods bridge: when a guest order transitions into FUNDED via the
// 0-conf path, GuestOrderAppService emits events.OrderConfirmation so
// DigitalEntitlementAppService (the only non-test subscriber) can create
// download grants for digital purchases.
func TestHandlePaymentDetected_ZeroConfs_EmitsOrderConfirmation(t *testing.T) {
	db := newGuestTestDB(t)
	bus := events.NewBus()
	sub, err := bus.Subscribe(&events.OrderConfirmation{}, events.BufSize(4))
	require.NoError(t, err)
	defer sub.Close()

	svc := &GuestOrderAppService{db: db, eventBus: bus, nodeID: "test-node-1"}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_emit_zero_confs",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:eip155:1:native",
		RequiredConfs: 0,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	require.NoError(t, svc.HandlePaymentDetected("gst_emit_zero_confs", "0xemit", nil))

	select {
	case evt := <-sub.Out():
		oc, ok := evt.(*events.OrderConfirmation)
		require.True(t, ok)
		assert.Equal(t, "gst_emit_zero_confs", oc.OrderID, "OrderID must be the orderToken")
		assert.Equal(t, "test-node-1", oc.VendorID)
	case <-time.After(2 * time.Second):
		t.Fatal("expected OrderConfirmation event after FUNDED transition, got none")
	}
}

// TestHandleConfirmationUpdate_ReachesThreshold_EmitsOrderConfirmation
// verifies the same bridge for the multi-confirmation path (UTXO/EXTERNAL_PAYMENT).
func TestHandleConfirmationUpdate_ReachesThreshold_EmitsOrderConfirmation(t *testing.T) {
	db := newGuestTestDB(t)
	bus := events.NewBus()
	sub, err := bus.Subscribe(&events.OrderConfirmation{}, events.BufSize(4))
	require.NoError(t, err)
	defer sub.Close()

	svc := &GuestOrderAppService{db: db, eventBus: bus, nodeID: "test-node-2"}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_emit_confirm",
		State:         models.GuestOrderPaymentDetected,
		PaymentCoin:   "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native",
		PaymentTxHash: "ltctx-emit",
		RequiredConfs: 3,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	// Below threshold: no event.
	require.NoError(t, svc.HandleConfirmationUpdate("gst_emit_confirm", 2))
	select {
	case <-sub.Out():
		t.Fatal("must NOT emit before reaching confirmation threshold")
	case <-time.After(50 * time.Millisecond):
	}

	// Reaches threshold: event must fire.
	require.NoError(t, svc.HandleConfirmationUpdate("gst_emit_confirm", 3))
	select {
	case evt := <-sub.Out():
		oc := evt.(*events.OrderConfirmation)
		assert.Equal(t, "gst_emit_confirm", oc.OrderID)
	case <-time.After(2 * time.Second):
		t.Fatal("expected OrderConfirmation event after threshold reached")
	}
}

// TestHandlePaymentDetected_NilEventBus_NoCrash guards against accidental
// regressions: the helper must tolerate a nil eventBus (e.g. tests / private_distribution
// init order). If this regresses, every guest payment crashes the node.
func TestHandlePaymentDetected_NilEventBus_NoCrash(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db, eventBus: nil}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_no_bus",
		State:         models.GuestOrderAwaitingPayment,
		PaymentCoin:   "crypto:eip155:1:native",
		RequiredConfs: 0,
		ExpiresAt:     time.Now().Add(time.Hour),
	})

	require.NotPanics(t, func() {
		_ = svc.HandlePaymentDetected("gst_no_bus", "0xnobus", nil)
	})
	order := loadGuestOrder(t, db, "gst_no_bus")
	assert.Equal(t, models.GuestOrderFunded, order.State, "FUNDED transition still happens without bus")
}
