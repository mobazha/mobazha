package payment_test

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// mockStrategy implements ChainEscrow for testing.
type mockStrategy struct {
	model payment.PaymentModel
}

func (m *mockStrategy) Model() payment.PaymentModel { return m.model }
func (m *mockStrategy) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (m *mockStrategy) AutoConfirm(_ context.Context, _ *events.CancelablePaymentReady) error {
	return nil
}
func (m *mockStrategy) SignEscrowRelease(_ context.Context, _ payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (m *mockStrategy) EstimateEscrowFee(_ string, _, _ int, _ iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (m *mockStrategy) GeneratePaymentInstructions(_ context.Context, _ payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	return &payment.PaymentSetupResult{PaymentModel: m.model}, nil
}
func (m *mockStrategy) GetConfirmInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}
func (m *mockStrategy) GetCancelInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}
func (m *mockStrategy) GetCompleteInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}
func (m *mockStrategy) GetDisputeReleaseInstructions(_ context.Context, _ payment.InstructionParams) (*payment.InstructionResult, error) {
	return &payment.InstructionResult{Instructions: nil}, nil
}
func (m *mockStrategy) VerifyDeposit(_ context.Context, _ payment.DepositVerifyParams) error {
	return nil
}
func (m *mockStrategy) ValidatePaymentMessage(_ payment.PaymentMessageParams) error { return nil }
func (m *mockStrategy) VerifyPreRelease(_ context.Context, _ payment.PreReleaseParams) error {
	return nil
}

func TestRegistry_Register_And_Lookup(t *testing.T) {
	r := payment.NewRegistry()
	s := &mockStrategy{model: payment.PaymentModelMonitored}

	r.Register(iwallet.ChainBitcoin, s)

	got, err := r.ForChain(iwallet.ChainBitcoin)
	if err != nil {
		t.Fatalf("ForChain(BTC): unexpected error: %v", err)
	}
	if got.Model() != payment.PaymentModelMonitored {
		t.Errorf("Model() = %s, want %s", got.Model(), payment.PaymentModelMonitored)
	}
}

func TestRegistry_ForCoin_DelegatesToForChain(t *testing.T) {
	r := payment.NewRegistry()
	s := &mockStrategy{model: payment.PaymentModelClientSigned}

	// BSC native canonical coin resolves to BSC chain
	r.Register(iwallet.ChainBSC, s)

	got, err := r.ForCoin(iwallet.CoinType("crypto:eip155:56:native"))
	if err != nil {
		t.Fatalf("ForCoin(BSC native): unexpected error: %v", err)
	}
	if got.Model() != payment.PaymentModelClientSigned {
		t.Errorf("Model() = %s, want %s", got.Model(), payment.PaymentModelClientSigned)
	}
}

func TestRegistry_ForCoin_TokensResolveToPlatformChain(t *testing.T) {
	r := payment.NewRegistry()
	bscStrategy := &mockStrategy{model: payment.PaymentModelClientSigned}

	r.Register(iwallet.ChainBSC, bscStrategy)

	// Canonical BSC assets (native + ERC20) should resolve to BSC chain
	for _, coin := range []iwallet.CoinType{
		iwallet.CoinType("crypto:eip155:56:native"),
		iwallet.CoinType("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955"),
		iwallet.CoinType("crypto:eip155:56:erc20:0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d"),
	} {
		got, err := r.ForCoin(coin)
		if err != nil {
			t.Errorf("ForCoin(%s): unexpected error: %v", coin, err)
			continue
		}
		if got != bscStrategy {
			t.Errorf("ForCoin(%s) returned different strategy than registered for BSC", coin)
		}
	}
}

func TestRegistry_ForCoin_RuntimeManagedEscrowERC20ResolvesToPlatformChain(t *testing.T) {
	r := payment.NewRegistry()
	ethStrategy := &mockStrategy{model: payment.PaymentModelClientSigned}

	r.Register(iwallet.ChainEthereum, ethStrategy)

	got, err := r.ForCoin(iwallet.CoinType("crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0"))
	if err != nil {
		t.Fatalf("ForCoin(runtime ManagedEscrow ERC20): unexpected error: %v", err)
	}
	if got != ethStrategy {
		t.Fatalf("ForCoin(runtime ManagedEscrow ERC20) returned wrong strategy")
	}
}

func TestRegistry_ForCoin_CanonicalAssetID_ResolvesChain(t *testing.T) {
	r := payment.NewRegistry()
	bscStrategy := &mockStrategy{model: payment.PaymentModelClientSigned}
	r.Register(iwallet.ChainBSC, bscStrategy)

	coin := iwallet.CoinType("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955")
	got, err := r.ForCoin(coin)
	if err != nil {
		t.Fatalf("ForCoin(%s): unexpected error: %v", coin, err)
	}
	if got != bscStrategy {
		t.Fatalf("ForCoin(%s) returned wrong strategy", coin)
	}
}

func TestRegistry_ForChain_UnknownReturnsError(t *testing.T) {
	r := payment.NewRegistry()

	_, err := r.ForChain(iwallet.ChainType("UNKNOWN"))
	if err == nil {
		t.Fatal("ForChain for unregistered chain should return error")
	}
}

func TestRegistry_ForCoin_UnknownCoinReturnsError(t *testing.T) {
	r := payment.NewRegistry()

	_, err := r.ForCoin(iwallet.CoinType("NONEXISTENT"))
	if err == nil {
		t.Fatal("ForCoin for unknown coin should return error")
	}
}

func TestIsRetiredPaymentChain_TRON(t *testing.T) {
	if !payment.IsRetiredPaymentChain(iwallet.ChainTRON) {
		t.Fatal("TRON should be retired")
	}
}

func TestRegistry_Register_Overwrites(t *testing.T) {
	r := payment.NewRegistry()
	s1 := &mockStrategy{model: payment.PaymentModelMonitored}
	s2 := &mockStrategy{model: payment.PaymentModelClientSigned}

	r.Register(iwallet.ChainBitcoin, s1)
	r.Register(iwallet.ChainBitcoin, s2) // overwrite

	got, err := r.ForChain(iwallet.ChainBitcoin)
	if err != nil {
		t.Fatalf("ForChain: unexpected error: %v", err)
	}
	if got.Model() != payment.PaymentModelClientSigned {
		t.Errorf("overwrite failed: Model() = %s, want %s", got.Model(), payment.PaymentModelClientSigned)
	}
}

func TestRegistry_Chains_ReturnsAllRegistered(t *testing.T) {
	r := payment.NewRegistry()
	s := &mockStrategy{model: payment.PaymentModelMonitored}

	r.Register(iwallet.ChainBitcoin, s)
	r.Register(iwallet.ChainEthereum, s)
	r.Register(iwallet.ChainSolana, s)

	chains := r.Chains()
	if len(chains) != 3 {
		t.Errorf("Chains() returned %d chains, want 3", len(chains))
	}

	// Verify all registered chains are present
	chainSet := make(map[iwallet.ChainType]bool)
	for _, c := range chains {
		chainSet[c] = true
	}
	for _, expected := range []iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainEthereum, iwallet.ChainSolana} {
		if !chainSet[expected] {
			t.Errorf("Chains() missing %s", expected)
		}
	}
}

func TestRegistry_Chains_EmptyRegistry(t *testing.T) {
	r := payment.NewRegistry()

	chains := r.Chains()
	if len(chains) != 0 {
		t.Errorf("Chains() on empty registry returned %d, want 0", len(chains))
	}
}

func TestRegistry_ForCoinV2_V1RegistrationWrapsUnderlying(t *testing.T) {
	r := payment.NewRegistry()
	v1 := &mockStrategy{model: payment.PaymentModelClientSigned}
	r.Register(iwallet.ChainTRON, v1)
	tronCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainTRON)
	if err != nil {
		t.Fatalf("RequireCanonicalNativeCoinType(TRON): %v", err)
	}

	got, err := r.ForCoinV2(tronCoin)
	if err != nil {
		t.Fatalf("ForCoinV2: %v", err)
	}
	wrapped, ok := got.(*payment.V1AsV2)
	if !ok {
		t.Fatalf("ForCoinV2 returned %T, want *payment.V1AsV2", got)
	}
	if wrapped.Underlying() != v1 {
		t.Fatal("expected underlying V1 strategy")
	}
}

func TestRegistry_ForCoinV2_V2NativeNotWrapped(t *testing.T) {
	r := payment.NewRegistry()
	native := &nativeV2Strategy{model: payment.PaymentModelMonitored}
	r.RegisterV2(iwallet.ChainSolana, native)
	solCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("RequireCanonicalNativeCoinType(SOL): %v", err)
	}

	got, err := r.ForCoinV2(solCoin)
	if err != nil {
		t.Fatalf("ForCoinV2: %v", err)
	}
	if got != native {
		t.Fatalf("ForCoinV2 returned %T, want native V2 strategy", got)
	}
	if _, ok := got.(*payment.V1AsV2); ok {
		t.Fatal("native V2 strategy should not be wrapped")
	}
}

func TestRegistry_RegisterV2Exclusive_RejectsExistingV1(t *testing.T) {
	r := payment.NewRegistry()
	r.Register(iwallet.ChainBitcoin, &mockStrategy{model: payment.PaymentModelMonitored})

	err := r.RegisterV2Exclusive(iwallet.ChainBitcoin, &nativeV2Strategy{model: payment.PaymentModelMonitored})

	if err == nil {
		t.Fatal("RegisterV2Exclusive should reject an existing V1 strategy")
	}
}

func TestRegistry_RegisterV2Exclusive_RegistersUnusedChain(t *testing.T) {
	r := payment.NewRegistry()
	strategy := &nativeV2Strategy{model: payment.PaymentModelMonitored}

	if err := r.RegisterV2Exclusive(iwallet.ChainSolana, strategy); err != nil {
		t.Fatalf("RegisterV2Exclusive: %v", err)
	}
	if !r.HasChain(iwallet.ChainSolana) {
		t.Fatal("HasChain(Solana) = false after exclusive registration")
	}
	got, err := r.ForChainV2(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChainV2(Solana): %v", err)
	}
	if got != strategy {
		t.Fatalf("ForChainV2(Solana) returned %T, want registered strategy", got)
	}
}

func TestRegistry_RegisterV2BatchExclusive_IsAtomic(t *testing.T) {
	r := payment.NewRegistry()
	existing := &nativeV2Strategy{model: payment.PaymentModelMonitored}
	r.RegisterV2(iwallet.ChainBitcoin, existing)

	err := r.RegisterV2BatchExclusive(map[iwallet.ChainType]payment.ChainEscrowV2{
		iwallet.ChainEthereum: &nativeV2Strategy{model: payment.PaymentModelMonitored},
		iwallet.ChainBitcoin:  &nativeV2Strategy{model: payment.PaymentModelMonitored},
	})

	if err == nil {
		t.Fatal("RegisterV2BatchExclusive should reject a batch containing an existing chain")
	}
	if r.HasChain(iwallet.ChainEthereum) {
		t.Fatal("RegisterV2BatchExclusive partially committed a rejected batch")
	}
	got, lookupErr := r.ForChainV2(iwallet.ChainBitcoin)
	if lookupErr != nil {
		t.Fatalf("ForChainV2(Bitcoin): %v", lookupErr)
	}
	if got != existing {
		t.Fatal("RegisterV2BatchExclusive changed the existing strategy")
	}
}
