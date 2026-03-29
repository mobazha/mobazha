package payment_test

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// mockStrategy implements PaymentStrategy for testing.
type mockStrategy struct {
	model payment.PaymentModel
}

func (m *mockStrategy) Model() payment.PaymentModel { return m.model }
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

	// BNB coin resolves to BSC chain
	r.Register(iwallet.ChainBSC, s)

	got, err := r.ForCoin(iwallet.CtBNB)
	if err != nil {
		t.Fatalf("ForCoin(BNB): unexpected error: %v", err)
	}
	if got.Model() != payment.PaymentModelClientSigned {
		t.Errorf("Model() = %s, want %s", got.Model(), payment.PaymentModelClientSigned)
	}
}

func TestRegistry_ForCoin_TokensResolveToPlatformChain(t *testing.T) {
	r := payment.NewRegistry()
	bscStrategy := &mockStrategy{model: payment.PaymentModelClientSigned}

	r.Register(iwallet.ChainBSC, bscStrategy)

	// BEP20 tokens should resolve to BSC chain
	for _, coin := range []iwallet.CoinType{iwallet.CtBNB, iwallet.CtBEP20USDT, iwallet.CtBEP20USDC} {
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
