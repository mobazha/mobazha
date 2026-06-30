package distribution

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRuntime_DoesNotExposePrivilegedState(t *testing.T) {
	runtimeType := reflect.TypeOf(PaymentRuntime{})
	require.Equal(t, 3, runtimeType.NumField())
	for index := 0; index < runtimeType.NumField(); index++ {
		field := runtimeType.Field(index)
		assert.False(t, field.IsExported(), "runtime field %s must not be exported", field.Name)
		typeName := field.Type.String()
		assert.False(t, strings.Contains(typeName, "KeyProvider"), "runtime field %s exposes raw keys", field.Name)
		assert.False(t, strings.Contains(typeName, "WalletOperator"), "runtime field %s exposes the whole wallet operator", field.Name)
		assert.False(t, strings.Contains(typeName, "MobazhaNode"), "runtime field %s exposes the node", field.Name)
	}
}

type testPaymentStrategy struct{}

func (*testPaymentStrategy) Model() payment.PaymentModel { return payment.PaymentModelMonitored }
func (*testPaymentStrategy) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (*testPaymentStrategy) SetupPayment(context.Context, payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (*testPaymentStrategy) Confirm(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (*testPaymentStrategy) Cancel(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (*testPaymentStrategy) Complete(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (*testPaymentStrategy) DisputeRelease(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (*testPaymentStrategy) GetActionStatus(context.Context, string) (*payment.ActionStatus, error) {
	return nil, payment.ErrActionNotFound
}
func (*testPaymentStrategy) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (*testPaymentStrategy) SignEscrowRelease(context.Context, payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (*testPaymentStrategy) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (*testPaymentStrategy) VerifyDeposit(context.Context, payment.DepositVerifyParams) error {
	return nil
}
func (*testPaymentStrategy) ValidatePaymentMessage(payment.PaymentMessageParams) error { return nil }
func (*testPaymentStrategy) VerifyPreRelease(context.Context, payment.PreReleaseParams) error {
	return nil
}

type testPaymentRegistry struct {
	strategies map[iwallet.ChainType]payment.ChainEscrowV2
}

func newTestPaymentRegistry() *testPaymentRegistry {
	return &testPaymentRegistry{strategies: make(map[iwallet.ChainType]payment.ChainEscrowV2)}
}

func (r *testPaymentRegistry) RegisterV2(chain iwallet.ChainType, strategy payment.ChainEscrowV2) error {
	if r.HasChain(chain) {
		return fmt.Errorf("duplicate chain %s", chain)
	}
	r.strategies[chain] = strategy
	return nil
}

func (r *testPaymentRegistry) RegisterV2BatchExclusive(strategies map[iwallet.ChainType]payment.ChainEscrowV2) error {
	for chain := range strategies {
		if r.HasChain(chain) {
			return fmt.Errorf("payment strategy already registered for chain %s", chain)
		}
	}
	for chain, strategy := range strategies {
		r.strategies[chain] = strategy
	}
	return nil
}

func (r *testPaymentRegistry) UnregisterV2Batch(chains []iwallet.ChainType) {
	for _, chain := range chains {
		delete(r.strategies, chain)
	}
}

func (r *testPaymentRegistry) HasChain(chain iwallet.ChainType) bool {
	_, exists := r.strategies[chain]
	return exists
}

type testPaymentModule struct {
	id          string
	chain       iwallet.ChainType
	strategy    payment.ChainEscrowV2
	err         error
	called      bool
	activated   bool
	activateErr error
	unbound     bool
	rolledBack  bool
}

func (m *testPaymentModule) Bind(context.Context) error {
	m.activated = true
	return m.activateErr
}

func (m *testPaymentModule) Unbind(context.Context) error { m.unbound = true; return nil }

func (m *testPaymentModule) RollbackRegistration(context.Context) error {
	m.rolledBack = true
	return nil
}

func (m *testPaymentModule) Descriptor() PaymentModuleDescriptor {
	return PaymentModuleDescriptor{ID: m.id}
}

func (m *testPaymentModule) Register(_ context.Context, _ PaymentRuntime, registrar PaymentRegistrar) error {
	m.called = true
	if m.err != nil {
		return m.err
	}
	if m.chain == "" {
		return nil
	}
	return registrar.RegisterV2(m.chain, m.strategy)
}

func TestRegisterPaymentModules_ValidModule_CommitsStrategy(t *testing.T) {
	registry := newTestPaymentRegistry()
	module := &testPaymentModule{
		id:       "commercial.managed",
		chain:    iwallet.ChainEthereum,
		strategy: &testPaymentStrategy{},
	}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, module)

	require.NoError(t, err)
	assert.True(t, module.called)
	assert.True(t, module.activated)
	assert.True(t, registry.HasChain(iwallet.ChainEthereum))
}

func TestRegisterPaymentModules_ActivationFailure_RollsBackStrategies(t *testing.T) {
	registry := newTestPaymentRegistry()
	module := &testPaymentModule{
		id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{},
		activateErr: errors.New("guest runtime unavailable"),
	}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, module)

	require.ErrorContains(t, err, "guest runtime unavailable")
	assert.True(t, module.activated)
	assert.True(t, module.unbound)
	assert.True(t, module.rolledBack)
	assert.Empty(t, registry.strategies)
}

func TestRegisterPaymentModules_LaterBindFailureUnbindsEarlierModule(t *testing.T) {
	registry := newTestPaymentRegistry()
	first := &testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}}
	second := &testPaymentModule{id: "commercial.solana", chain: iwallet.ChainSolana, strategy: &testPaymentStrategy{}, activateErr: errors.New("bind failed")}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, first, second)

	require.ErrorContains(t, err, "bind failed")
	assert.True(t, first.activated)
	assert.True(t, first.unbound)
	assert.True(t, second.unbound)
	assert.True(t, first.rolledBack)
	assert.True(t, second.rolledBack)
	assert.Empty(t, registry.strategies)
}

func TestPaymentRuntime_EnforcesDeclaredCapabilities(t *testing.T) {
	authority := NewPaymentRuntimeAuthority(ManagedEVMRuntime{}, ManagedEscrowGuestRuntimePorts{})
	runtime, err := authority.RuntimeFor(PaymentModuleDescriptor{ID: "managed-evm", Capabilities: []PaymentModuleCapability{CapabilityManagedEVMExecution}})
	require.NoError(t, err)
	_, err = runtime.ManagedEVM()
	require.NoError(t, err)
	_, err = runtime.ManagedEscrowGuest()
	require.ErrorContains(t, err, string(CapabilityManagedEscrowGuest))
}

func TestRegisterPaymentModules_ModuleFailure_LeavesRegistryUnchanged(t *testing.T) {
	registry := newTestPaymentRegistry()
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "commercial.solana", err: errors.New("missing relay")},
	}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, modules...)

	require.ErrorContains(t, err, "missing relay")
	assert.Empty(t, registry.strategies)
	for _, module := range modules {
		assert.True(t, module.(*testPaymentModule).rolledBack)
	}
}

func TestRegisterPaymentModules_DuplicateID_LeavesRegistryUnchanged(t *testing.T) {
	registry := newTestPaymentRegistry()
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainSolana, strategy: &testPaymentStrategy{}},
	}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, modules...)

	require.ErrorContains(t, err, "registered more than once")
	assert.Empty(t, registry.strategies)
}

func TestRegisterPaymentModules_ExistingCoreChain_RejectsWholeSet(t *testing.T) {
	registry := newTestPaymentRegistry()
	registry.strategies[iwallet.ChainBitcoin] = &testPaymentStrategy{}
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "invalid.override", chain: iwallet.ChainBitcoin, strategy: &testPaymentStrategy{}},
	}

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, modules...)

	require.ErrorContains(t, err, "already registered")
	assert.Len(t, registry.strategies, 1)
	assert.False(t, registry.HasChain(iwallet.ChainEthereum))
}

func TestRegisterPaymentModules_TypedNilModule_IsRejected(t *testing.T) {
	registry := newTestPaymentRegistry()
	var module *testPaymentModule

	err := RegisterPaymentModules(context.Background(), PaymentRuntimeAuthority{}, registry, module)

	require.ErrorContains(t, err, "is nil")
	assert.Empty(t, registry.strategies)
}
