//go:build !private_distribution

package guest

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// TestLTCGuestOrder_FullClosurePath exercises detect → FUNDED → sweep submitted → confirmed
// using the same code paths as production with mocked chain ops and wallet sweep signing.
func TestLTCGuestOrder_FullClosurePath(t *testing.T) {
	db := newSweepTestDB(t)
	require.NoError(t, db.gormDB.AutoMigrate(
		&models.GuestOrder{},
		&models.GuestOrderItem{},
		&models.InventoryReservation{},
		&models.ReceivingAccount{},
	))
	require.NoError(t, db.gormDB.Create(&models.ReceivingAccount{
		TenantMixin: models.TenantMixin{TenantID: testTenantID},
		ID:          1,
		ChainType:   iwallet.ChainLitecoin,
		IsActive:    true,
		Address:     "ltc1q_seller_closure",
	}).Error)

	keyDeriver := &mockSweepKeyDeriver{t: t}
	sweepSvc := NewAutoSweepService(db, keyDeriver, nil)
	sweepSvc.SetChainOps(&mockSweepChainOps{healthy: true})
	sweepSvc.SetMultiwallet(&mockSweepWalletOperator{t: t})

	svc := &GuestOrderAppService{
		db:                  db,
		sweepService:        sweepSvc,
		supportedUTXOChains: map[iwallet.ChainType]struct{}{iwallet.ChainLitecoin: {}},
	}
	svc.SetMultiwallet(&mockSweepWalletOperator{t: t})
	svc.SetUTXOMonitor(&stubUTXOMonitor{healthy: true, sources: 1})

	mockSrc := newMockPaymentSource(iwallet.ChainLitecoin)
	mon := pkgutxo.NewMonitor(nil)
	mon.AddSource(iwallet.ChainLitecoin, mockSrc)

	payMon := NewGuestPaymentMonitor(db, svc, nil, nil)
	payMon.SetUTXOMonitor(mon)
	payMon.SetMultiwallet(&mockWalletOperator{
		wallet: &mockUTXOWallet{scriptPubKey: []byte{0x76, 0xa9, 0x14}},
	})
	defer payMon.StopAll()

	token := "gst_ltc_closure_full"
	seedSweepGuestOrder(t, db, 510, models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    ltcCoinTypeStr,
		PaymentAddress: "ltc1q_closure_addr",
		PaymentAmount:  "750000",
		SweepToAddress: "ltc1q_seller_closure",
		AddressIndex:   3,
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(time.Hour),
	})

	order := loadSweepGuestOrder(t, db, token)
	payMon.WatchOrder(&order)
	time.Sleep(200 * time.Millisecond)

	mockSrc.SimulatePayment("ltc1q_closure_addr", &iwallet.Transaction{
		ID:    "ltc_tx_closure_pay",
		Value: iwallet.NewAmount(750000),
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress("ltc1q_closure_addr", iwallet.CoinType(ltcCoinTypeStr)),
			Amount:  iwallet.NewAmount(750000),
		}},
	})

	require.Eventually(t, func() bool {
		o := loadSweepGuestOrder(t, db, token)
		return o.State == models.GuestOrderPaymentDetected
	}, 5*time.Second, 100*time.Millisecond)

	require.NoError(t, svc.HandleConfirmationUpdate(token, 3))
	o := loadSweepGuestOrder(t, db, token)
	assert.Equal(t, models.GuestOrderFunded, o.State)

	var task models.SweepTask
	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&task).Error)
	assert.Equal(t, models.SweepStatusPending, task.Status)

	sweepSvc.ProcessPendingSweeps(context.Background())

	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&task).Error)
	assert.Equal(t, models.SweepStatusSubmitted, task.Status)
	assert.Equal(t, "ltc_sweep_tx_closure", task.TxHash)

	sweepSvc.ProcessPendingSweeps(context.Background())

	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&task).Error)
	assert.Equal(t, models.SweepStatusConfirmed, task.Status)
}

type stubUTXOMonitor struct {
	healthy bool
	sources int
	watched int
}

func (s *stubUTXOMonitor) IsHealthy(chain iwallet.ChainType) bool {
	if s == nil {
		return false
	}
	return s.healthy
}

func (s *stubUTXOMonitor) GetHealthySourceCount(chain iwallet.ChainType) int {
	if s == nil || !s.healthy {
		return 0
	}
	if s.sources > 0 {
		return s.sources
	}
	return 1
}

func (s *stubUTXOMonitor) GetWatchedAddressCount() int {
	if s == nil {
		return 0
	}
	return s.watched
}

type mockSweepKeyDeriver struct {
	t   *testing.T
	key *btcec.PrivateKey
}

func (m *mockSweepKeyDeriver) DeriveAddress(chainType iwallet.ChainType, index uint32) (string, error) {
	return "ltc1q_derived", nil
}

func (m *mockSweepKeyDeriver) DerivePrivateKey(chainType iwallet.ChainType, index uint32) ([]byte, error) {
	if m.key == nil {
		var err error
		m.key, err = btcec.NewPrivateKey()
		require.NoError(m.t, err)
	}
	return m.key.Serialize(), nil
}

type mockSweepWalletOperator struct {
	t *testing.T
}

func (m *mockSweepWalletOperator) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return m.wallet(), nil
}

func (m *mockSweepWalletOperator) SupportedChains() []iwallet.ChainType {
	return []iwallet.ChainType{iwallet.ChainLitecoin}
}

func (m *mockSweepWalletOperator) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	if chain == iwallet.ChainLitecoin {
		return m.wallet(), true
	}
	return nil, false
}

func (m *mockSweepWalletOperator) Start() error { return nil }
func (m *mockSweepWalletOperator) Close() error { return nil }

func (m *mockSweepWalletOperator) wallet() iwallet.Wallet {
	return &mockSweepWallet{mockUTXOWallet: mockUTXOWallet{scriptPubKey: []byte{0x76, 0xa9, 0x14}}}
}

type mockSweepWallet struct {
	mockUTXOWallet
}

func (m *mockSweepWallet) BuildSweepTx(inputs []iwallet.SweepInput, signingKey btcec.PrivateKey, destAddress string, feePerByte int64) ([]byte, string, error) {
	return []byte{0x01, 0x02}, "internal_sweep_hash", nil
}

type mockSweepChainOps struct {
	healthy bool
}

func (m *mockSweepChainOps) GetTransaction(_ iwallet.ChainType, _ string) (*iwallet.Transaction, error) {
	return nil, nil
}

func (m *mockSweepChainOps) GetFeeEstimate(_ iwallet.ChainType, _ int) uint64 { return 2 }

func (m *mockSweepChainOps) BroadcastTransaction(_ iwallet.ChainType, txHex string) (string, error) {
	if _, err := hex.DecodeString(txHex); err != nil {
		return "", err
	}
	return "ltc_sweep_tx_closure", nil
}

func (m *mockSweepChainOps) GetAddressTransactions(_ iwallet.ChainType, _ string, _ []byte) ([]iwallet.Transaction, error) {
	return nil, nil
}

func (m *mockSweepChainOps) IsHealthy(_ iwallet.ChainType) bool { return m.healthy }

func (m *mockSweepChainOps) ListUnspent(_ iwallet.ChainType, _ []byte) ([]pkgutxo.UnspentOutput, error) {
	return []pkgutxo.UnspentOutput{{
		TxHash: "ltc_tx_closure_pay", OutputIndex: 0, Value: 750000,
	}}, nil
}

func (m *mockSweepChainOps) GetTxConfirmations(_ iwallet.ChainType, txHash string) (int, error) {
	if txHash == "ltc_sweep_tx_closure" {
		return 3, nil
	}
	return 0, nil
}
