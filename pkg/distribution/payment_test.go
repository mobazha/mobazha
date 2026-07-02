package distribution

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRuntime_DoesNotExposePrivilegedState(t *testing.T) {
	runtimeType := reflect.TypeOf(PaymentRuntime{})
	require.Equal(t, 4, runtimeType.NumField())
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
	return PaymentModuleDescriptor{
		ID: m.id, Version: "test", Rails: []PaymentRailKind{PaymentRailEscrow}, Activation: PaymentModuleOptional,
	}
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

func registerTestPaymentModules(ctx context.Context, registry PaymentRegistry, modules ...PaymentModule) (*TrustedPaymentModuleManager, error) {
	manager, err := NewTrustedPaymentModuleManager(PaymentRuntimeAuthority{}, registry, modules...)
	if err != nil {
		return nil, err
	}
	return manager, manager.Register(ctx)
}

func TestTrustedPaymentModuleManager_ValidModule_CommitsStrategy(t *testing.T) {
	registry := newTestPaymentRegistry()
	module := &testPaymentModule{
		id:       "commercial.managed",
		chain:    iwallet.ChainEthereum,
		strategy: &testPaymentStrategy{},
	}

	_, err := registerTestPaymentModules(context.Background(), registry, module)

	require.NoError(t, err)
	assert.True(t, module.called)
	assert.True(t, module.activated)
	assert.True(t, registry.HasChain(iwallet.ChainEthereum))
}

func TestTrustedPaymentModuleManager_ActivationFailure_RollsBackStrategies(t *testing.T) {
	registry := newTestPaymentRegistry()
	module := &testPaymentModule{
		id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{},
		activateErr: errors.New("guest runtime unavailable"),
	}

	_, err := registerTestPaymentModules(context.Background(), registry, module)

	require.ErrorContains(t, err, "guest runtime unavailable")
	assert.True(t, module.activated)
	assert.True(t, module.unbound)
	assert.True(t, module.rolledBack)
	assert.Empty(t, registry.strategies)
}

func TestTrustedPaymentModuleManager_LaterBindFailureUnbindsEarlierModule(t *testing.T) {
	registry := newTestPaymentRegistry()
	first := &testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}}
	second := &testPaymentModule{id: "commercial.solana", chain: iwallet.ChainSolana, strategy: &testPaymentStrategy{}, activateErr: errors.New("bind failed")}

	_, err := registerTestPaymentModules(context.Background(), registry, first, second)

	require.ErrorContains(t, err, "bind failed")
	assert.True(t, first.activated)
	assert.True(t, first.unbound)
	assert.True(t, second.unbound)
	assert.True(t, first.rolledBack)
	assert.True(t, second.rolledBack)
	assert.Empty(t, registry.strategies)
}

func TestPaymentRuntime_EnforcesDeclaredCapabilities(t *testing.T) {
	authority := NewPaymentRuntimeAuthority(ManagedEVMRuntime{}, ManagedSolanaRuntime{}, ManagedEscrowGuestRuntimePorts{})
	runtime, err := authority.RuntimeFor(PaymentModuleDescriptor{ID: "managed-evm", Capabilities: []PaymentModuleCapability{CapabilityManagedEVMExecution}})
	require.NoError(t, err)
	_, err = runtime.ManagedEVM()
	require.NoError(t, err)
	_, err = runtime.ManagedEscrowGuest()
	require.ErrorContains(t, err, string(CapabilityManagedEscrowGuest))
	_, err = runtime.ManagedSolana()
	require.ErrorContains(t, err, string(CapabilityManagedSolana))

	solanaRuntime, err := authority.RuntimeFor(PaymentModuleDescriptor{
		ID: "managed-solana", Capabilities: []PaymentModuleCapability{CapabilityManagedSolana},
	})
	require.NoError(t, err)
	_, err = solanaRuntime.ManagedSolana()
	require.NoError(t, err)
	_, err = solanaRuntime.ManagedEVM()
	require.ErrorContains(t, err, string(CapabilityManagedEVMExecution))
}

func TestTrustedPaymentModuleManager_ModuleFailure_LeavesRegistryUnchanged(t *testing.T) {
	registry := newTestPaymentRegistry()
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "commercial.solana", err: errors.New("missing relay")},
	}

	_, err := registerTestPaymentModules(context.Background(), registry, modules...)

	require.ErrorContains(t, err, "missing relay")
	assert.Empty(t, registry.strategies)
	for _, module := range modules {
		assert.True(t, module.(*testPaymentModule).rolledBack)
	}
}

func TestTrustedPaymentModuleManager_DuplicateID_LeavesRegistryUnchanged(t *testing.T) {
	registry := newTestPaymentRegistry()
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainSolana, strategy: &testPaymentStrategy{}},
	}

	_, err := registerTestPaymentModules(context.Background(), registry, modules...)

	require.ErrorContains(t, err, "registered more than once")
	assert.Empty(t, registry.strategies)
}

func TestTrustedPaymentModuleManager_ExistingCoreChain_RejectsWholeSet(t *testing.T) {
	registry := newTestPaymentRegistry()
	registry.strategies[iwallet.ChainBitcoin] = &testPaymentStrategy{}
	modules := []PaymentModule{
		&testPaymentModule{id: "commercial.managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		&testPaymentModule{id: "invalid.override", chain: iwallet.ChainBitcoin, strategy: &testPaymentStrategy{}},
	}

	_, err := registerTestPaymentModules(context.Background(), registry, modules...)

	require.ErrorContains(t, err, "already registered")
	assert.Len(t, registry.strategies, 1)
	assert.False(t, registry.HasChain(iwallet.ChainEthereum))
}

func TestTrustedPaymentModuleManager_TypedNilModule_IsRejected(t *testing.T) {
	registry := newTestPaymentRegistry()
	var module *testPaymentModule

	_, err := registerTestPaymentModules(context.Background(), registry, module)

	require.ErrorContains(t, err, "is nil")
	assert.Empty(t, registry.strategies)
}

type lifecycleTestModule struct {
	*testPaymentModule
	dependencies []string
	stopOrder    *[]string
	stopMu       *sync.Mutex
	stopped      chan struct{}
	started      chan struct{}
	readyGate    <-chan struct{}
}

func (m *lifecycleTestModule) Descriptor() PaymentModuleDescriptor {
	descriptor := m.testPaymentModule.Descriptor()
	descriptor.Dependencies = append([]string(nil), m.dependencies...)
	return descriptor
}

func (m *lifecycleTestModule) Start(ctx context.Context, ready func()) error {
	if m.started != nil {
		close(m.started)
	}
	if m.readyGate != nil {
		select {
		case <-m.readyGate:
		case <-ctx.Done():
			return nil
		}
	}
	ready()
	<-m.stopped
	return nil
}

func (m *lifecycleTestModule) Stop(context.Context) error {
	m.stopMu.Lock()
	*m.stopOrder = append(*m.stopOrder, m.id)
	m.stopMu.Unlock()
	select {
	case <-m.stopped:
	default:
		close(m.stopped)
	}
	return nil
}

func TestTrustedPaymentModuleManager_StopsInReverseDependencyOrder(t *testing.T) {
	registry := newTestPaymentRegistry()
	var stopOrder []string
	var stopMu sync.Mutex
	base := &lifecycleTestModule{
		testPaymentModule: &testPaymentModule{id: "base", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		stopOrder:         &stopOrder, stopMu: &stopMu, stopped: make(chan struct{}),
	}
	dependent := &lifecycleTestModule{
		testPaymentModule: &testPaymentModule{id: "dependent", chain: iwallet.ChainSolana, strategy: &testPaymentStrategy{}},
		dependencies:      []string{"base"}, stopOrder: &stopOrder, stopMu: &stopMu, stopped: make(chan struct{}),
	}
	manager, err := NewTrustedPaymentModuleManager(PaymentRuntimeAuthority{}, registry, dependent, base)
	require.NoError(t, err)
	require.NoError(t, manager.Register(context.Background()))
	require.NoError(t, manager.Start(context.Background(), nil))
	require.NoError(t, manager.Stop(context.Background()))
	require.Equal(t, []string{"dependent", "base"}, stopOrder)
	health := manager.Health()
	require.Len(t, health, 2)
	require.Equal(t, PaymentModuleStopped, health[0].State)
	require.Equal(t, PaymentModuleStopped, health[1].State)
}

func TestTrustedPaymentModuleManager_WaitsForRunnerReadiness(t *testing.T) {
	registry := newTestPaymentRegistry()
	var stopOrder []string
	var stopMu sync.Mutex
	readyGate := make(chan struct{})
	module := &lifecycleTestModule{
		testPaymentModule: &testPaymentModule{id: "managed", chain: iwallet.ChainEthereum, strategy: &testPaymentStrategy{}},
		stopOrder:         &stopOrder,
		stopMu:            &stopMu,
		stopped:           make(chan struct{}),
		started:           make(chan struct{}),
		readyGate:         readyGate,
	}
	manager, err := NewTrustedPaymentModuleManager(PaymentRuntimeAuthority{}, registry, module)
	require.NoError(t, err)
	require.NoError(t, manager.Register(context.Background()))
	require.NoError(t, manager.Start(context.Background(), nil))
	require.Eventually(t, func() bool {
		select {
		case <-module.started:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.Equal(t, PaymentModuleStarting, manager.Health()[0].State)
	close(readyGate)
	require.Eventually(t, func() bool {
		return manager.Health()[0].State == PaymentModuleReady
	}, time.Second, time.Millisecond)
	require.NoError(t, manager.Stop(context.Background()))
}

func TestTrustedPaymentModuleManager_RejectsDependencyCycle(t *testing.T) {
	registry := newTestPaymentRegistry()
	first := &lifecycleTestModule{testPaymentModule: &testPaymentModule{id: "first"}, dependencies: []string{"second"}}
	second := &lifecycleTestModule{testPaymentModule: &testPaymentModule{id: "second"}, dependencies: []string{"first"}}
	_, err := NewTrustedPaymentModuleManager(PaymentRuntimeAuthority{}, registry, first, second)
	require.ErrorContains(t, err, "dependency cycle")
}
