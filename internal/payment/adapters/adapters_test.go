package adapters_test

import (
	"context"
	"errors"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock: KeyProvider ───────────────────────────────────────────

type stubKeyProvider struct {
	ethKey    *btcec.PrivateKey
	solKey    *solana.PrivateKey
	escrowKey *btcec.PrivateKey
	ratingKey *btcec.PrivateKey
	tronKey   *btcec.PrivateKey
	err       error
}

func (s *stubKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)     { return s.ethKey, s.err }
func (s *stubKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) { return s.solKey, s.err }
func (s *stubKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)  { return s.escrowKey, s.err }
func (s *stubKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)  { return s.ratingKey, s.err }
func (s *stubKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)    { return s.tronKey, s.err }
func (s *stubKeyProvider) DigitalContentMasterKey(_ int) ([]byte, error) {
	return make([]byte, 32), s.err
}

func newTestKeyProvider() *stubKeyProvider {
	ethKey, _ := btcec.NewPrivateKey()
	escrowKey, _ := btcec.NewPrivateKey()
	ratingKey, _ := btcec.NewPrivateKey()
	tronKey, _ := btcec.NewPrivateKey()
	solKey := solana.NewWallet().PrivateKey
	return &stubKeyProvider{
		ethKey:    ethKey,
		solKey:    &solKey,
		escrowKey: escrowKey,
		ratingKey: ratingKey,
		tronKey:   tronKey,
	}
}

// ── Mock: WalletOperator ────────────────────────────────────────

type stubWalletOperator struct {
	wallet iwallet.Wallet
	err    error
}

func (s *stubWalletOperator) WalletForCurrencyCode(_ string) (iwallet.Wallet, error) {
	return s.wallet, s.err
}
func (s *stubWalletOperator) SupportedChains() []iwallet.ChainType { return nil }
func (s *stubWalletOperator) WalletForChain(_ iwallet.ChainType) (iwallet.Wallet, bool) {
	return s.wallet, s.wallet != nil
}
func (s *stubWalletOperator) Start() error { return nil }
func (s *stubWalletOperator) Close() error { return nil }

// ── Mock: UTXOEscrow wallet ─────────────────────────────────────

type stubUTXOEscrow struct {
	iwallet.Wallet
	estimateFee iwallet.Amount
	estimateErr error
	signSigs    []iwallet.EscrowSignature
	signErr     error
}

func (s *stubUTXOEscrow) EstimateEscrowFee(_, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return s.estimateFee, s.estimateErr
}
func (s *stubUTXOEscrow) SignMultisigTransaction(_ iwallet.Transaction, _ btcec.PrivateKey, _ []byte) ([]iwallet.EscrowSignature, error) {
	return s.signSigs, s.signErr
}
func (s *stubUTXOEscrow) CreateMultisigAddress(_ []btcec.PublicKey, _ []byte, _ int) (iwallet.Address, []byte, error) {
	return iwallet.NewAddress("", iwallet.CoinType("")), nil, nil
}
func (s *stubUTXOEscrow) BuildAndSend(_ iwallet.Tx, _ iwallet.Transaction, _ [][]iwallet.EscrowSignature, _ []byte, _ iwallet.OrderFinishType) (iwallet.TransactionID, error) {
	return "", nil
}

type stubNonEscrowWallet struct {
	iwallet.Wallet
}

// ── Mock: ChainOps ──────────────────────────────────────────────

type stubChainOps struct {
	autoConfirmErr       error
	autoConfirmCalled    bool
	signResult           []iwallet.EscrowSignature
	signErr              error
	cancelReleaseResult  any
	cancelReleaseErr     error
	completeEscrowResult any
	completeEscrowErr    error
	disputeReleaseResult any
	disputeReleaseErr    error
}

func (s *stubChainOps) AutoConfirm(_ *events.CancelablePaymentReady) error {
	s.autoConfirmCalled = true
	return s.autoConfirmErr
}
func (s *stubChainOps) SignEscrowRelease(_ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return s.signResult, s.signErr
}
func (s *stubChainOps) BuildCancelableRelease(_ *models.Order, _, _ string) (any, error) {
	return s.cancelReleaseResult, s.cancelReleaseErr
}
func (s *stubChainOps) BuildCompleteEscrow(_ *models.Order, _ string, _ *pb.EscrowRelease) (any, error) {
	return s.completeEscrowResult, s.completeEscrowErr
}
func (s *stubChainOps) BuildDisputeRelease(_ *models.Order, _ string) (any, error) {
	return s.disputeReleaseResult, s.disputeReleaseErr
}
func (s *stubChainOps) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (s *stubChainOps) ValidatePaymentMessage(_ payment.PaymentMessageParams) error {
	return nil
}
func (s *stubChainOps) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}

// ═══════════════════════════════════════════════════════════════
// ClientSignedAdapter Tests
// ═══════════════════════════════════════════════════════════════

func TestClientSignedAdapter_Model(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)
	assert.Equal(t, payment.PaymentModelClientSigned, adapter.Model())
}

func TestClientSignedAdapter_EstimateEscrowFee_AlwaysZero(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)
	fee, err := adapter.EstimateEscrowFee("ETH", 1, 2, iwallet.FlNormal)
	require.NoError(t, err)
	assert.Equal(t, iwallet.NewAmount(0), fee)
}

func TestClientSignedAdapter_AutoConfirm_DelegatesToChainOps(t *testing.T) {
	ops := &stubChainOps{}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	event := &events.CancelablePaymentReady{OrderID: "order-1"}
	err := adapter.AutoConfirm(context.Background(), event)

	require.NoError(t, err)
	assert.True(t, ops.autoConfirmCalled)
}

func TestClientSignedAdapter_AutoConfirm_PropagatesError(t *testing.T) {
	ops := &stubChainOps{autoConfirmErr: errors.New("chain error")}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	err := adapter.AutoConfirm(context.Background(), &events.CancelablePaymentReady{})
	assert.EqualError(t, err, "chain error")
}

func TestClientSignedAdapter_SignEscrowRelease_DelegatesToChainOps(t *testing.T) {
	expected := []iwallet.EscrowSignature{{Index: 0, Signature: []byte("sig")}}
	ops := &stubChainOps{signResult: expected}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	sigs, err := adapter.SignEscrowRelease(context.Background(), payment.SignEscrowParams{})
	require.NoError(t, err)
	assert.Equal(t, expected, sigs)
}

func TestClientSignedAdapter_GeneratePaymentInstructions(t *testing.T) {
	paymentData := &models.PaymentData{}
	escrowAddr := iwallet.NewAddress("0xESCROW", iwallet.CoinType("ETH"))

	buildFn := func(_ context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
		assert.Equal(t, "order-123", params.OrderID)
		return paymentData, escrowAddr, "tx-instructions", nil
	}
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, buildFn, nil)

	result, err := adapter.GeneratePaymentInstructions(context.Background(), payment.PaymentSetupParams{
		OrderID: "order-123",
	})
	require.NoError(t, err)
	assert.Equal(t, payment.PaymentModelClientSigned, result.PaymentModel)
	assert.Equal(t, paymentData, result.PaymentData)
	assert.Equal(t, escrowAddr.String(), result.EscrowAddr)
	assert.Equal(t, "tx-instructions", result.Instructions)
}

func TestClientSignedAdapter_GeneratePaymentInstructions_Error(t *testing.T) {
	buildFn := func(_ context.Context, _ models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
		return nil, iwallet.NewAddress("", iwallet.CoinType("")), nil, errors.New("build failed")
	}
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, buildFn, nil)

	_, err := adapter.GeneratePaymentInstructions(context.Background(), payment.PaymentSetupParams{})
	assert.EqualError(t, err, "build failed")
}

func TestClientSignedAdapter_GetConfirmInstructions(t *testing.T) {
	releaseFn := func(orderID models.OrderID, initiator, payout string) (iwallet.CoinType, any, error) {
		assert.Equal(t, models.OrderID("order-1"), orderID)
		assert.Equal(t, "0xINITIATOR", initiator)
		assert.Equal(t, "0xPAYOUT", payout)
		return iwallet.CoinType("ETH"), "release-instructions", nil
	}
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, releaseFn)

	result, err := adapter.GetConfirmInstructions(context.Background(), payment.InstructionParams{
		OrderID:       "order-1",
		InitiatorAddr: "0xINITIATOR",
		PayoutAddr:    "0xPAYOUT",
	})
	require.NoError(t, err)
	assert.Equal(t, "release-instructions", result.Instructions)
}

func TestClientSignedAdapter_GetCancelInstructions_NilOrderData(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)

	_, err := adapter.GetCancelInstructions(context.Background(), payment.InstructionParams{
		OrderData: nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OrderData")
}

func TestClientSignedAdapter_GetCancelInstructions_Success(t *testing.T) {
	ops := &stubChainOps{cancelReleaseResult: "cancel-instructions"}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	result, err := adapter.GetCancelInstructions(context.Background(), payment.InstructionParams{
		OrderData:     &models.Order{},
		InitiatorAddr: "0xBUYER",
		PayoutAddr:    "0xREFUND",
	})
	require.NoError(t, err)
	assert.Equal(t, "cancel-instructions", result.Instructions)
}

func TestClientSignedAdapter_GetCompleteInstructions_NilOrderData(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)

	_, err := adapter.GetCompleteInstructions(context.Background(), payment.InstructionParams{
		OrderData:   nil,
		ReleaseInfo: &pb.EscrowRelease{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OrderData")
}

func TestClientSignedAdapter_GetCompleteInstructions_NilReleaseInfo(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)

	_, err := adapter.GetCompleteInstructions(context.Background(), payment.InstructionParams{
		OrderData:   &models.Order{},
		ReleaseInfo: nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ReleaseInfo")
}

func TestClientSignedAdapter_GetCompleteInstructions_Success(t *testing.T) {
	ops := &stubChainOps{completeEscrowResult: "complete-instructions"}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	result, err := adapter.GetCompleteInstructions(context.Background(), payment.InstructionParams{
		OrderData:   &models.Order{},
		ReleaseInfo: &pb.EscrowRelease{},
	})
	require.NoError(t, err)
	assert.Equal(t, "complete-instructions", result.Instructions)
}

func TestClientSignedAdapter_GetDisputeReleaseInstructions_NilOrderData(t *testing.T) {
	adapter := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)

	_, err := adapter.GetDisputeReleaseInstructions(context.Background(), payment.InstructionParams{
		OrderData: nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OrderData")
}

func TestClientSignedAdapter_GetDisputeReleaseInstructions_Success(t *testing.T) {
	ops := &stubChainOps{disputeReleaseResult: "dispute-instructions"}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	result, err := adapter.GetDisputeReleaseInstructions(context.Background(), payment.InstructionParams{
		OrderData: &models.Order{},
	})
	require.NoError(t, err)
	assert.Equal(t, "dispute-instructions", result.Instructions)
}

func TestClientSignedAdapter_GetDisputeReleaseInstructions_ChainOpsError(t *testing.T) {
	ops := &stubChainOps{disputeReleaseErr: errors.New("dispute build failed")}
	adapter := adapters.NewClientSignedAdapter(ops, nil, nil)

	_, err := adapter.GetDisputeReleaseInstructions(context.Background(), payment.InstructionParams{
		OrderData: &models.Order{},
	})
	assert.EqualError(t, err, "dispute build failed")
}

// ═══════════════════════════════════════════════════════════════
// UTXOAutoConfirmAdapter Tests
// ═══════════════════════════════════════════════════════════════

func TestUTXOAdapter_Model(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{}
	assert.Equal(t, payment.PaymentModelMonitored, adapter.Model())
}

func TestUTXOAdapter_AutoConfirm_CallsCallback(t *testing.T) {
	var called bool
	adapter := &adapters.UTXOAutoConfirmAdapter{
		OnAutoConfirm: func(_ *events.CancelablePaymentReady) { called = true },
	}

	err := adapter.AutoConfirm(context.Background(), &events.CancelablePaymentReady{OrderID: "o1"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestUTXOAdapter_EstimateEscrowFee_WalletError(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{err: errors.New("no wallet")},
	}
	_, err := adapter.EstimateEscrowFee("BTC", 1, 2, iwallet.FlNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get wallet")
}

func TestUTXOAdapter_EstimateEscrowFee_NotUTXOEscrow(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{wallet: &stubNonEscrowWallet{}},
	}
	_, err := adapter.EstimateEscrowFee("BTC", 1, 2, iwallet.FlNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support escrow")
}

func TestUTXOAdapter_EstimateEscrowFee_Success(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{wallet: &stubUTXOEscrow{
			estimateFee: iwallet.NewAmount(1000),
		}},
	}
	fee, err := adapter.EstimateEscrowFee("BTC", 1, 2, iwallet.FlNormal)
	require.NoError(t, err)
	assert.Equal(t, iwallet.NewAmount(1000), fee)
}

func TestUTXOAdapter_SignEscrowRelease_WalletError(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{err: errors.New("no wallet")},
		Keys:        newTestKeyProvider(),
	}
	_, err := adapter.SignEscrowRelease(context.Background(), payment.SignEscrowParams{CoinCode: "BTC"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get wallet")
}

func TestUTXOAdapter_SignEscrowRelease_NotUTXOEscrow(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{wallet: &stubNonEscrowWallet{}},
		Keys:        newTestKeyProvider(),
	}
	_, err := adapter.SignEscrowRelease(context.Background(), payment.SignEscrowParams{CoinCode: "BTC"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support escrow")
}

func TestUTXOAdapter_SignEscrowRelease_KeyProviderError(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{wallet: &stubUTXOEscrow{}},
		Keys:        &stubKeyProvider{err: errors.New("vault unavailable")},
	}
	_, err := adapter.SignEscrowRelease(context.Background(), payment.SignEscrowParams{CoinCode: "BTC"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get escrow master key")
}

func TestUTXOAdapter_SignEscrowRelease_Success(t *testing.T) {
	expected := []iwallet.EscrowSignature{{Index: 0, Signature: []byte("utxo-sig")}}
	keys := newTestKeyProvider()
	adapter := &adapters.UTXOAutoConfirmAdapter{
		Multiwallet: &stubWalletOperator{wallet: &stubUTXOEscrow{signSigs: expected}},
		Keys:        keys,
	}
	sigs, err := adapter.SignEscrowRelease(context.Background(), payment.SignEscrowParams{
		CoinCode:  "BTC",
		ChainCode: keys.escrowKey.Serialize()[:32],
	})
	require.NoError(t, err)
	assert.Equal(t, expected, sigs)
}

func TestUTXOAdapter_GeneratePaymentInstructions_Success(t *testing.T) {
	paymentData := &models.PaymentData{}
	adapter := &adapters.UTXOAutoConfirmAdapter{
		GetPaymentInfo: func(_ context.Context, orderID, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error) {
			assert.Equal(t, "order-1", orderID)
			return paymentData, nil
		},
	}

	result, err := adapter.GeneratePaymentInstructions(context.Background(), payment.PaymentSetupParams{
		OrderID: "order-1",
	})
	require.NoError(t, err)
	assert.Equal(t, payment.PaymentModelMonitored, result.PaymentModel)
	assert.Equal(t, paymentData, result.PaymentData)
}

func TestUTXOAdapter_GeneratePaymentInstructions_Error(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{
		GetPaymentInfo: func(_ context.Context, _, _ string, _ iwallet.CoinType) (*models.PaymentData, error) {
			return nil, errors.New("no payment info")
		},
	}

	result, err := adapter.GeneratePaymentInstructions(context.Background(), payment.PaymentSetupParams{})
	assert.Error(t, err)
	assert.Equal(t, payment.PaymentModelMonitored, result.PaymentModel)
}

func TestUTXOAdapter_InstructionMethods_ReturnNilInstructions(t *testing.T) {
	adapter := &adapters.UTXOAutoConfirmAdapter{}
	ctx := context.Background()
	params := payment.InstructionParams{}

	tests := []struct {
		name string
		fn   func(context.Context, payment.InstructionParams) (*payment.InstructionResult, error)
	}{
		{"GetConfirmInstructions", adapter.GetConfirmInstructions},
		{"GetCancelInstructions", adapter.GetCancelInstructions},
		{"GetCompleteInstructions", adapter.GetCompleteInstructions},
		{"GetDisputeReleaseInstructions", adapter.GetDisputeReleaseInstructions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fn(ctx, params)
			require.NoError(t, err)
			assert.Nil(t, result.Instructions)
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// EVMChainOps Tests
// ═══════════════════════════════════════════════════════════════

func TestEVMChainOps_AutoConfirm_ResolvesChainAndCallsCallback(t *testing.T) {
	var capturedChainType string
	ops := &adapters.EVMChainOps{
		OnAutoConfirm: func(_ *events.CancelablePaymentReady, chainType string) {
			capturedChainType = chainType
		},
	}

	err := ops.AutoConfirm(&events.CancelablePaymentReady{Coin: "MCK"})
	require.NoError(t, err)
	assert.NotEmpty(t, capturedChainType)
}

func TestEVMChainOps_AutoConfirm_UnknownCoin(t *testing.T) {
	ops := &adapters.EVMChainOps{
		OnAutoConfirm: func(_ *events.CancelablePaymentReady, _ string) {},
	}

	err := ops.AutoConfirm(&events.CancelablePaymentReady{Coin: "UNKNOWN_COIN_XYZ"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown coin")
}

func TestEVMChainOps_SignEscrowRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.EVMChainOps{
		Keys: &stubKeyProvider{err: errors.New("key vault down")},
	}

	_, err := ops.SignEscrowRelease(payment.SignEscrowParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get EVM master key")
}

func TestEVMChainOps_BuildCancelableRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.EVMChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildCancelableRelease(&models.Order{}, "0xA", "0xB")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get EVM master key")
}

func TestEVMChainOps_BuildCompleteEscrow_KeyProviderError(t *testing.T) {
	ops := &adapters.EVMChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildCompleteEscrow(&models.Order{}, "0xA", &pb.EscrowRelease{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get EVM master key")
}

func TestEVMChainOps_BuildDisputeRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.EVMChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildDisputeRelease(&models.Order{}, "0xA")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get EVM master key")
}

// ═══════════════════════════════════════════════════════════════
// SolanaChainOps Tests
// ═══════════════════════════════════════════════════════════════

func TestSolanaChainOps_AutoConfirm_LogsAndReturnsNil(t *testing.T) {
	ops := &adapters.SolanaChainOps{NodeID: "test-node"}

	err := ops.AutoConfirm(&events.CancelablePaymentReady{OrderID: "sol-order-1"})
	require.NoError(t, err)
}

func TestSolanaChainOps_SignEscrowRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.SolanaChainOps{
		Keys: &stubKeyProvider{err: errors.New("key vault down")},
	}

	_, err := ops.SignEscrowRelease(payment.SignEscrowParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Solana master key")
}

func TestSolanaChainOps_BuildCancelableRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.SolanaChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildCancelableRelease(&models.Order{}, "addr1", "addr2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Solana master key")
}

func TestSolanaChainOps_BuildCompleteEscrow_KeyProviderError(t *testing.T) {
	ops := &adapters.SolanaChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildCompleteEscrow(&models.Order{}, "addr1", &pb.EscrowRelease{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Solana master key")
}

func TestSolanaChainOps_BuildDisputeRelease_KeyProviderError(t *testing.T) {
	ops := &adapters.SolanaChainOps{
		Keys: &stubKeyProvider{err: errors.New("key error")},
	}

	_, err := ops.BuildDisputeRelease(&models.Order{}, "addr1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Solana master key")
}

// ═══════════════════════════════════════════════════════════════
// Integration: adapter ↔ chainOps wiring
// ═══════════════════════════════════════════════════════════════

func TestClientSignedAdapter_FullPipeline_WithMockChainOps(t *testing.T) {
	ops := &stubChainOps{
		signResult:           []iwallet.EscrowSignature{{Index: 0, Signature: []byte("sig")}},
		cancelReleaseResult:  "cancel-data",
		completeEscrowResult: "complete-data",
		disputeReleaseResult: "dispute-data",
	}
	buildFn := func(_ context.Context, _ models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
		return &models.PaymentData{}, iwallet.NewAddress("0xESCROW", iwallet.CoinType("ETH")), "init-data", nil
	}
	releaseFn := func(_ models.OrderID, _, _ string) (iwallet.CoinType, any, error) {
		return iwallet.CoinType("ETH"), "confirm-data", nil
	}

	adapter := adapters.NewClientSignedAdapter(ops, buildFn, releaseFn)
	ctx := context.Background()

	t.Run("Model", func(t *testing.T) {
		assert.Equal(t, payment.PaymentModelClientSigned, adapter.Model())
	})

	t.Run("GeneratePaymentInstructions", func(t *testing.T) {
		result, err := adapter.GeneratePaymentInstructions(ctx, payment.PaymentSetupParams{OrderID: "o1"})
		require.NoError(t, err)
		assert.Equal(t, "init-data", result.Instructions)
	})

	t.Run("GetConfirmInstructions", func(t *testing.T) {
		result, err := adapter.GetConfirmInstructions(ctx, payment.InstructionParams{OrderID: "o1"})
		require.NoError(t, err)
		assert.Equal(t, "confirm-data", result.Instructions)
	})

	t.Run("SignEscrowRelease", func(t *testing.T) {
		sigs, err := adapter.SignEscrowRelease(ctx, payment.SignEscrowParams{})
		require.NoError(t, err)
		assert.Len(t, sigs, 1)
	})

	t.Run("GetCancelInstructions", func(t *testing.T) {
		result, err := adapter.GetCancelInstructions(ctx, payment.InstructionParams{OrderData: &models.Order{}})
		require.NoError(t, err)
		assert.Equal(t, "cancel-data", result.Instructions)
	})

	t.Run("GetCompleteInstructions", func(t *testing.T) {
		result, err := adapter.GetCompleteInstructions(ctx, payment.InstructionParams{
			OrderData:   &models.Order{},
			ReleaseInfo: &pb.EscrowRelease{},
		})
		require.NoError(t, err)
		assert.Equal(t, "complete-data", result.Instructions)
	})

	t.Run("GetDisputeReleaseInstructions", func(t *testing.T) {
		result, err := adapter.GetDisputeReleaseInstructions(ctx, payment.InstructionParams{OrderData: &models.Order{}})
		require.NoError(t, err)
		assert.Equal(t, "dispute-data", result.Instructions)
	})
}

func TestSolanaAnchorAdapter_SetupPaymentRelaysWithoutReturningInstructions(t *testing.T) {
	legacy := adapters.NewClientSignedAdapter(&stubChainOps{}, func(_ context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
		return &models.PaymentData{OrderID: params.OrderID, Coin: params.CoinType}, iwallet.NewAddress("escrow111", params.CoinType), "anchor-create-ix", nil
	}, nil)
	store := adapters.NewMemoryActionStore()
	var relayedOrder string
	var relayedInstructions any
	adapter := adapters.NewSolanaAnchorAdapter(adapters.SolanaAnchorAdapterDeps{
		Legacy: legacy,
		Relayer: adapters.SolanaInstructionRelayerFunc(func(_ context.Context, orderID string, instructions any) (string, error) {
			relayedOrder = orderID
			relayedInstructions = instructions
			return "solana-tx-sig", nil
		}),
		Store:    store,
		Recorder: store,
	})

	result, err := adapter.SetupPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-1",
		CoinType: iwallet.CoinType("crypto:solana:mainnet:native"),
	})
	require.NoError(t, err)
	require.Equal(t, payment.ActionModeSubmitted, result.Mode)
	require.Equal(t, "solana-tx-sig", result.ActionID)
	require.Equal(t, "solana-tx-sig", result.SubmittedTxHash)
	require.Nil(t, result.Instructions, "V2 setup must not leak frontend-signable instructions")
	require.Equal(t, "order-1", relayedOrder)
	require.Equal(t, "anchor-create-ix", relayedInstructions)

	status, err := adapter.GetActionStatus(context.Background(), "solana-tx-sig")
	require.NoError(t, err)
	require.Equal(t, "submitted", status.State)
	require.Equal(t, "setup", status.SettlementAction)
	require.Equal(t, "solana-tx-sig", status.TxHash)
}

func TestSolanaAnchorAdapter_ActionsFailClosedInsteadOfReturningLegacyInstructions(t *testing.T) {
	adapter := adapters.NewSolanaAnchorAdapter(adapters.SolanaAnchorAdapterDeps{
		Legacy: adapters.NewClientSignedAdapter(&stubChainOps{cancelReleaseResult: "legacy-cancel"}, nil, func(models.OrderID, string, string) (iwallet.CoinType, any, error) {
			return iwallet.CoinType("crypto:solana:mainnet:native"), "legacy-confirm", nil
		}),
	})

	confirm, err := adapter.Confirm(context.Background(), payment.ActionParams{OrderID: "order-1"})
	require.Nil(t, confirm)
	require.ErrorIs(t, err, payment.ErrUnsupportedAction)

	cancel, err := adapter.Cancel(context.Background(), payment.ActionParams{OrderID: "order-1"})
	require.Nil(t, cancel)
	require.ErrorIs(t, err, payment.ErrUnsupportedAction)
}
