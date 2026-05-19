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
	"gorm.io/gorm"
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

func TestPaymentAppService_PersistManagedEscrowPaymentAddress_UpdatesAllTenantRows(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rawProvider, ok := db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	raw := rawProvider.RawDB()
	require.NotNil(t, raw)

	require.NoError(t, raw.Create(&models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-buyer"},
		ID:          models.OrderID("order-safe"),
	}).Error)
	require.NoError(t, raw.Create(&models.Order{
		TenantMixin: models.TenantMixin{TenantID: "tenant-vendor"},
		ID:          models.OrderID("order-safe"),
	}).Error)

	svc := newTestPaymentAppService(t, PaymentAppServiceConfig{DB: db})
	require.NoError(t, svc.persistManagedEscrowPaymentAddress(
		"order-safe",
		"crypto:eip155:11155111:native",
		"0xmanagedescrow",
		1000,
		false,
	))

	var orders []models.Order
	require.NoError(t, raw.
		Where("id = ?", "order-safe").
		Order("tenant_id ASC").
		Find(&orders).Error)
	require.Len(t, orders, 2)
	for i := range orders {
		require.Equal(t, "0xmanagedescrow", orders[i].PaymentAddress)
		info, err := orders[i].GetPendingManagedEscrowPaymentInfo()
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, uint64(1000), info.Amount)
		require.Equal(t, "crypto:eip155:11155111:native", info.Coin)
		require.Equal(t, "0xmanagedescrow", info.Address)
	}
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

// ReceivingAccount CRUD tests have been migrated to
// internal/core/receiving_account_service_test.go (OP-1.3)

func TestPaymentAppService_ReceivingAccount_MigratedPlaceholder(t *testing.T) {
	t.Skip("ReceivingAccount + GetAcceptedCurrencies tests migrated to receiving_account_service_test.go")
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
