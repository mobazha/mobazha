//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── test helpers ────────────────────────────────────────────────────────

// testChainEscrow implements payment.ChainEscrow for unit testing.
type testChainEscrow struct {
	model        payment.PaymentModel
	genResult    *payment.PaymentSetupResult
	genErr       error
	genCallCount int
}

func (s *testChainEscrow) Model() payment.PaymentModel { return s.model }
func (s *testChainEscrow) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (s *testChainEscrow) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	return nil
}
func (s *testChainEscrow) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (s *testChainEscrow) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *testChainEscrow) GeneratePaymentInstructions(_ context.Context, _ payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	s.genCallCount++
	return s.genResult, s.genErr
}
func (s *testChainEscrow) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (s *testChainEscrow) ValidatePaymentMessage(_ payment.PaymentMessageParams) error {
	return nil
}
func (s *testChainEscrow) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}
func (s *testChainEscrow) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}
func (s *testChainEscrow) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{}, nil
}

// newTestPaymentAppService creates a PaymentAppService with an in-memory DB
// suitable for unit testing. Only the DB and fields explicitly set via opts
// are populated; optional callbacks default to nil.
func newTestPaymentAppService(t *testing.T, cfg PaymentAppServiceConfig) *PaymentAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.EventBus == nil {
		cfg.EventBus = events.NewBus()
	}
	if cfg.Shutdown == nil {
		ch := make(chan struct{})
		cfg.Shutdown = ch
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-payment-svc"
	}
	return NewPaymentAppService(cfg)
}

// ── Constructor & Registry ──────────────────────────────────────────────

func TestPaymentAppService_NewPaymentAppService(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})
	assert.NotNil(t, svc)
	assert.Equal(t, "test-payment-svc", svc.nodeID)
}

func TestPaymentAppService_Registry_GetSet(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	assert.Nil(t, svc.Registry())

	reg := payment.NewRegistry()
	svc.SetRegistry(reg)
	assert.Same(t, reg, svc.Registry())
}

type stubFiatPaymentQuery struct{}

func (*stubFiatPaymentQuery) GetPayment(_ context.Context, _ string, _ string) (*contracts.PaymentDetail, error) {
	return nil, nil
}

func TestPaymentAppService_SetFiatPaymentQuery_PropagatesToExistingVerificationService(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})
	pvs := NewPaymentVerificationService(nil, nil, nil)

	svc.SetVerificationService(pvs)

	fq := &stubFiatPaymentQuery{}
	svc.SetFiatPaymentQuery(fq)

	assert.Same(t, fq, svc.fiatPaymentQuery)
	assert.Same(t, fq, pvs.fiatPayment)
}

func TestPaymentAppService_SetVerificationService_BackfillsStoredFiatPaymentQuery(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	fq := &stubFiatPaymentQuery{}
	svc.SetFiatPaymentQuery(fq)

	pvs := NewPaymentVerificationService(nil, nil, nil)
	svc.SetVerificationService(pvs)

	assert.Same(t, fq, svc.fiatPaymentQuery)
	assert.Same(t, fq, pvs.fiatPayment)
}

// ── FetchOrderByID ──────────────────────────────────────────────────────

func TestPaymentAppService_FetchOrderByID_NotFound(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	_, err := svc.FetchOrderByID("nonexistent-order")
	assert.Error(t, err, "should return error for nonexistent order")
}

func TestPaymentAppService_FetchOrderByID_Found(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	order := &models.Order{ID: models.OrderID("test-order-123")}
	err := svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	})
	require.NoError(t, err)

	got, err := svc.FetchOrderByID("test-order-123")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("test-order-123"), got.ID)
}

// ── GeneratePaymentInstructions ─────────────────────────────────────────

func TestPaymentAppService_GeneratePaymentInstructions_Success(t *testing.T) {
	reg := payment.NewRegistry()
	expectedResult := &payment.PaymentSetupResult{
		PaymentModel: payment.PaymentModelClientSigned,
		EscrowAddr:   "0xabc",
	}
	strategy := &testChainEscrow{
		model:     payment.PaymentModelClientSigned,
		genResult: expectedResult,
	}
	reg.Register(iwallet.ChainEthereum, strategy)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	result, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		OrderID:  "order-1",
		CoinType: iwallet.CoinType("crypto:eip155:1:native"),
		Amount:   1000000,
	})
	require.NoError(t, err)
	assert.Equal(t, payment.PaymentModelClientSigned, result.PaymentModel)
	assert.Equal(t, "0xabc", result.EscrowAddr)
	assert.Equal(t, 1, strategy.genCallCount)
}

func TestPaymentAppService_GeneratePaymentInstructions_NoCoinStrategy(t *testing.T) {
	reg := payment.NewRegistry()
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		CoinType: iwallet.CoinType("NONEXISTENT"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no chain escrow")
}

func TestPaymentAppService_GeneratePaymentInstructions_StrategyError(t *testing.T) {
	reg := payment.NewRegistry()
	strategy := &testChainEscrow{
		model:  payment.PaymentModelMonitored,
		genErr: errors.New("escrow generation failed"),
	}
	reg.Register(iwallet.ChainBitcoin, strategy)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	_, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
		CoinType: iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "escrow generation failed")
}

func TestPaymentAppService_GeneratePaymentInstructions_MultipleChains(t *testing.T) {
	reg := payment.NewRegistry()

	utxoStrategy := &testChainEscrow{
		model:     payment.PaymentModelMonitored,
		genResult: &payment.PaymentSetupResult{PaymentModel: payment.PaymentModelMonitored},
	}
	evmStrategy := &testChainEscrow{
		model:     payment.PaymentModelClientSigned,
		genResult: &payment.PaymentSetupResult{PaymentModel: payment.PaymentModelClientSigned},
	}

	reg.Register(iwallet.ChainBitcoin, utxoStrategy)
	reg.Register(iwallet.ChainEthereum, evmStrategy)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{
		PaymentRegistry: reg,
	})

	tests := []struct {
		name     string
		coin     iwallet.CoinType
		expected payment.PaymentModel
	}{
		{"BTC dispatches to UTXO strategy", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"), payment.PaymentModelMonitored},
		{"ETH dispatches to EVM strategy", iwallet.CoinType("crypto:eip155:1:native"), payment.PaymentModelClientSigned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.GeneratePaymentInstructions(context.Background(), models.InitializeEscrowData{
				CoinType: tt.coin,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.PaymentModel)
		})
	}
}

// IsEVMRelayAvailable and TryLockAutoConfirm tests live in
// internal/core/settlement/ as they test SettlementService methods.

// ── ReceivingAccount CRUD ───────────────────────────────────────────────

func TestPaymentAppService_AddReceivingAccount_ValidationError(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	_, err := svc.AddReceivingAccount(&models.ReceivingAccount{})
	assert.Error(t, err, "empty account should fail validation")
}

func TestPaymentAppService_AddReceivingAccount_Success(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	account := &models.ReceivingAccount{
		Name:      "My ETH Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0x1234567890abcdef1234567890abcdef12345678",
		IsActive:  true,
	}
	_ = account.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	saved, err := svc.AddReceivingAccount(account)
	require.NoError(t, err)
	assert.NotZero(t, saved.ID)
	assert.Equal(t, "My ETH Wallet", saved.Name)
	assert.True(t, saved.IsActive)
}

func TestPaymentAppService_GetReceivingAccounts_Empty(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	accounts, err := svc.GetReceivingAccounts()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

func TestPaymentAppService_ReceivingAccount_AddAndGet(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	acc1 := &models.ReceivingAccount{
		Name:      "ETH Account",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xaaa",
		IsActive:  true,
	}
	_ = acc1.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	acc2 := &models.ReceivingAccount{
		Name:      "BTC Account",
		ChainType: iwallet.ChainBitcoin,
		Address:   "bc1qxyz",
		IsActive:  true,
	}
	_ = acc2.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	_, err := svc.AddReceivingAccount(acc1)
	require.NoError(t, err)
	_, err = svc.AddReceivingAccount(acc2)
	require.NoError(t, err)

	accounts, err := svc.GetReceivingAccounts()
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
}

func TestPaymentAppService_GetReceivingAccountsByChain(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	for _, addr := range []string{"0xaaa", "0xbbb"} {
		acc := &models.ReceivingAccount{
			Name:      "ETH " + addr,
			ChainType: iwallet.ChainEthereum,
			Address:   addr,
			IsActive:  true,
		}
		_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
		_, err := svc.AddReceivingAccount(acc)
		require.NoError(t, err)
	}
	btcAcc := &models.ReceivingAccount{
		Name:      "BTC",
		ChainType: iwallet.ChainBitcoin,
		Address:   "bc1q",
		IsActive:  true,
	}
	_ = btcAcc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err := svc.AddReceivingAccount(btcAcc)
	require.NoError(t, err)

	ethAccounts, err := svc.GetReceivingAccountsByChain(iwallet.ChainEthereum)
	require.NoError(t, err)
	assert.Len(t, ethAccounts, 2)

	btcAccounts, err := svc.GetReceivingAccountsByChain(iwallet.ChainBitcoin)
	require.NoError(t, err)
	assert.Len(t, btcAccounts, 1)
}

func TestPaymentAppService_DeleteReceivingAccount(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	acc := &models.ReceivingAccount{
		Name:      "To Delete",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdead",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	saved, err := svc.AddReceivingAccount(acc)
	require.NoError(t, err)

	err = svc.DeleteReceivingAccount(saved.ID)
	require.NoError(t, err)

	accounts, err := svc.GetReceivingAccounts()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

func TestPaymentAppService_AddReceivingAccount_DuplicateAddress(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	acc := &models.ReceivingAccount{
		Name:      "First",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdup",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	first, err := svc.AddReceivingAccount(acc)
	require.NoError(t, err)

	acc2 := &models.ReceivingAccount{
		Name:      "Second (same addr)",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdup",
		IsActive:  false,
	}
	_ = acc2.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	second, err := svc.AddReceivingAccount(acc2)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "duplicate address should reuse existing record")

	accounts, err := svc.GetReceivingAccounts()
	require.NoError(t, err)
	assert.Len(t, accounts, 1, "should still be 1 record after duplicate add")
}

// ── GetAcceptedCurrencies ───────────────────────────────────────────────

func TestPaymentAppService_GetAcceptedCurrencies_Empty(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	currencies, err := svc.GetAcceptedCurrencies()
	require.NoError(t, err)
	assert.Empty(t, currencies)
}

func TestPaymentAppService_GetAcceptedCurrencies_WithAccounts(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	acc := &models.ReceivingAccount{
		Name:      "ETH Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xtest",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err := svc.AddReceivingAccount(acc)
	require.NoError(t, err)

	currencies, err := svc.GetAcceptedCurrencies()
	require.NoError(t, err)
	assert.NotEmpty(t, currencies)
	assert.Contains(t, currencies, string(iwallet.ChainEthereum))
}

// ── TransactionMetadata ─────────────────────────────────────────────────

func TestPaymentAppService_TransactionMetadata_SaveAndGet(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	meta := &models.TransactionMetadata{
		Txid:    "txid-abc-123",
		OrderID: models.OrderID("order-456"),
	}
	err := svc.SaveTransactionMetadata(meta)
	require.NoError(t, err)

	got, err := svc.GetTransactionMetadata("txid-abc-123")
	require.NoError(t, err)
	assert.Equal(t, models.OrderID("order-456"), got.OrderID)
}

func TestPaymentAppService_TransactionMetadata_NotFound(t *testing.T) {
	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{})

	_, err := svc.GetTransactionMetadata("nonexistent-tx")
	assert.Error(t, err)
}
