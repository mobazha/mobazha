package guest

import (
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/pkg/models"
	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// --- mock wallet for UTXOAddressUtilities ---

type mockUTXOWallet struct {
	iwallet.Wallet
	scriptPubKey []byte
}

func (m *mockUTXOWallet) AddressToScriptPubKey(_ string) ([]byte, error) {
	return m.scriptPubKey, nil
}

func (m *mockUTXOWallet) DerivePaymentAddressFromPubKey(_ *btcec.PublicKey) (string, []byte, error) {
	return "", nil, nil
}

func (m *mockUTXOWallet) BuildSweepTx(_ []iwallet.SweepInput, _ btcec.PrivateKey, _ string, _ int64) ([]byte, string, error) {
	return []byte{0x01}, "mock-sweep", nil
}

type mockWalletOperator struct {
	wallet *mockUTXOWallet
}

func (m *mockWalletOperator) WalletForCurrencyCode(_ string) (iwallet.Wallet, error) {
	return m.wallet, nil
}

func (m *mockWalletOperator) SupportedChains() []iwallet.ChainType {
	return []iwallet.ChainType{iwallet.ChainLitecoin}
}

func (m *mockWalletOperator) WalletForChain(_ iwallet.ChainType) (iwallet.Wallet, bool) {
	return m.wallet, true
}

func (m *mockWalletOperator) Start() error { return nil }
func (m *mockWalletOperator) Close() error { return nil }

const ltcCoinTypeStr = "crypto:bip122:12a765e31ffd4059bada1e25190f6e98:native"

// TestLTCGuestOrder_PaymentDetection verifies the Monitor→Service integration:
// WatchOrder sets up UTXO monitoring, SimulatePayment triggers the subscription
// callback, and the order transitions to PaymentDetected.
func TestLTCGuestOrder_PaymentDetection(t *testing.T) {
	db := newGuestTestDB(t)

	mockSrc := newMockPaymentSource(iwallet.ChainLitecoin)
	mon := pkgutxo.NewMonitor(nil)
	mon.AddSource(iwallet.ChainLitecoin, mockSrc)

	svc := &GuestOrderAppService{db: db}
	payMon := NewGuestPaymentMonitor(db, svc, nil, nil)
	payMon.SetUTXOMonitor(mon)
	payMon.SetMultiwallet(&mockWalletOperator{
		wallet: &mockUTXOWallet{scriptPubKey: []byte{0x76, 0xa9, 0x14}},
	})
	defer payMon.StopAll()

	token := "gst_ltc_detect_test1"
	seedGuestOrder(t, db, 500, models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    ltcCoinTypeStr,
		PaymentAddress: "ltc1q_test_addr",
		PaymentAmount:  "500000",
		SweepToAddress: "ltc1q_seller",
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(time.Hour),
	})

	order := loadGuestOrder(t, db, token)
	payMon.WatchOrder(&order)

	time.Sleep(200 * time.Millisecond)

	tx := &iwallet.Transaction{
		ID:    "ltc_tx_001",
		Value: iwallet.NewAmount(uint64(500000)),
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress("ltc1q_test_addr", iwallet.CoinType(ltcCoinTypeStr)),
			Amount:  iwallet.NewAmount(uint64(500000)),
		}},
	}
	mockSrc.SimulatePayment("ltc1q_test_addr", tx)

	require.Eventually(t, func() bool {
		o := loadGuestOrder(t, db, token)
		return o.State == models.GuestOrderPaymentDetected
	}, 5*time.Second, 100*time.Millisecond,
		"order should transition to PaymentDetected after simulated payment")

	o := loadGuestOrder(t, db, token)
	assert.Equal(t, "ltc_tx_001", o.PaymentTxHash)
}

// TestLTCGuestOrder_InsufficientPayment verifies that a partial payment
// does NOT transition the order to PaymentDetected.
func TestLTCGuestOrder_InsufficientPayment(t *testing.T) {
	db := newGuestTestDB(t)

	mockSrc := newMockPaymentSource(iwallet.ChainLitecoin)
	mon := pkgutxo.NewMonitor(nil)
	mon.AddSource(iwallet.ChainLitecoin, mockSrc)

	svc := &GuestOrderAppService{db: db}
	payMon := NewGuestPaymentMonitor(db, svc, nil, nil)
	payMon.SetUTXOMonitor(mon)
	payMon.SetMultiwallet(&mockWalletOperator{
		wallet: &mockUTXOWallet{scriptPubKey: []byte{0x76, 0xa9, 0x14}},
	})
	defer payMon.StopAll()

	token := "gst_ltc_partial_test"
	seedGuestOrder(t, db, 501, models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderAwaitingPayment,
		PaymentCoin:    ltcCoinTypeStr,
		PaymentAddress: "ltc1q_partial_addr",
		PaymentAmount:  "1000000",
		SweepToAddress: "ltc1q_seller2",
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(time.Hour),
	})

	order := loadGuestOrder(t, db, token)
	payMon.WatchOrder(&order)

	time.Sleep(200 * time.Millisecond)

	tx := &iwallet.Transaction{
		ID:    "ltc_tx_partial",
		Value: iwallet.NewAmount(uint64(300000)),
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress("ltc1q_partial_addr", iwallet.CoinType(ltcCoinTypeStr)),
			Amount:  iwallet.NewAmount(uint64(300000)),
		}},
	}
	mockSrc.SimulatePayment("ltc1q_partial_addr", tx)

	time.Sleep(500 * time.Millisecond)

	o := loadGuestOrder(t, db, token)
	assert.Equal(t, models.GuestOrderAwaitingPayment, o.State,
		"order should remain AwaitingPayment after insufficient payment")
}

// TestLTCGuestOrder_ConfirmToFunded_CreatesSweepTask verifies that
// HandleConfirmationUpdate reaching the threshold creates a SweepTask
// with correct LTC chain key derivation.
func TestLTCGuestOrder_ConfirmToFunded_CreatesSweepTask(t *testing.T) {
	db := newGuestTestDB(t)
	sweepSvc := &AutoSweepService{db: db}
	svc := &GuestOrderAppService{db: db, sweepService: sweepSvc}

	token := "gst_ltc_funded_sweep"
	seedGuestOrder(t, db, 502, models.GuestOrder{
		OrderToken:     token,
		State:          models.GuestOrderPaymentDetected,
		PaymentCoin:    ltcCoinTypeStr,
		PaymentTxHash:  "ltc_tx_conf_001",
		PaymentAddress: "ltc1q_pay_addr_conf",
		PaymentAmount:  "750000",
		SweepToAddress: "ltc1q_seller_conf",
		AddressIndex:   12,
		RequiredConfs:  3,
		ExpiresAt:      time.Now().Add(time.Hour),
	})

	err := svc.HandleConfirmationUpdate(token, 3)
	require.NoError(t, err)

	o := loadGuestOrder(t, db, token)
	assert.Equal(t, models.GuestOrderFunded, o.State)
	assert.NotNil(t, o.FundedAt)

	var task models.SweepTask
	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&task).Error)
	assert.Equal(t, "ltc1q_pay_addr_conf", task.FromAddress)
	assert.Equal(t, "ltc1q_seller_conf", task.ToAddress)
	assert.Equal(t, "750000", task.Amount)
	assert.Equal(t, uint32(12), task.AddressIndex)
	assert.Equal(t, models.SweepStatusPending, task.Status)
	assert.Equal(t, "LTC", task.ChainKey)
}
