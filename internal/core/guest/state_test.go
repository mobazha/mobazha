package guest

import (
	"testing"
	"time"

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
		{"ETH native", iwallet.CoinType("crypto:eip155:1:native"), 0},
		{"SOL native", iwallet.CoinType("crypto:solana:mainnet:native"), 0},
		{"BSC native", iwallet.CoinType("crypto:eip155:56:native"), 0},
		{"TRON native", iwallet.CoinType("crypto:tron:mainnet:native"), 0},
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

	err := svc.HandlePaymentDetected("gst_test_zero_confs", "0xabc123")
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

	err := svc.HandlePaymentDetected("gst_test_ltc", "ltctxhash123")
	require.NoError(t, err)

	order := loadGuestOrder(t, db, "gst_test_ltc")
	assert.Equal(t, models.GuestOrderPaymentDetected, order.State)
	assert.Equal(t, "ltctxhash123", order.PaymentTxHash)
	assert.Nil(t, order.FundedAt)
}

func TestHandleConfirmationUpdate_ReachesThreshold_Funded(t *testing.T) {
	db := newGuestTestDB(t)
	svc := &GuestOrderAppService{db: db}

	seedGuestOrder(t, db, 1, models.GuestOrder{
		OrderToken:    "gst_test_confirm",
		State:         models.GuestOrderPaymentDetected,
		PaymentCoin:   "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native",
		PaymentTxHash: "ltctx456",
		RequiredConfs: 3,
		ExpiresAt:     time.Now().Add(time.Hour),
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

			err := svc.HandlePaymentDetected(token, "0xtx")
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

	err := svc.HandlePaymentDetected("gst_expired", "0xtx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state mismatch")
}

func TestValidateCoinAvailability(t *testing.T) {
	private_distributionSvc := &GuestOrderAppService{
		supportedUTXOChains:    toChainSet([]iwallet.ChainType{iwallet.ChainLitecoin}),
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

	t.Run("PrivateDistribution allows LTC", func(t *testing.T) {
		err := private_distributionSvc.validateCoinAvailability(ltcCoin, ltcInfo)
		assert.NoError(t, err)
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
		assert.Contains(t, err.Error(), "EVM/TRON balance monitor not configured")
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

	t.Run("Full build allows TRON", func(t *testing.T) {
		err := fullBuildSvc.validateCoinAvailability(tronCoin, tronInfo)
		assert.NoError(t, err)
	})
}
