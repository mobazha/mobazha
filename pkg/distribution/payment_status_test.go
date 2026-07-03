package distribution

import (
	"context"
	"errors"
	"testing"
	"time"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statusTestPaymentModule struct {
	updates chan paymentModuleStatusUpdate
}

func (*statusTestPaymentModule) Descriptor() PaymentModuleDescriptor {
	return PaymentModuleDescriptor{
		ID:           "private.direct-observed",
		Version:      "test",
		Rails:        []PaymentRailKind{PaymentRailDirectObserved},
		Capabilities: []PaymentModuleCapability{CapabilityDirectObserved},
		Chains:       []iwallet.ChainType{iwallet.ChainMonero},
		Activation:   PaymentModuleSetupGated,
	}
}

func (*statusTestPaymentModule) Register(context.Context, PaymentRuntime, PaymentRegistrar) error {
	return nil
}

func (*statusTestPaymentModule) RollbackRegistration(context.Context) error { return nil }

func (module *statusTestPaymentModule) StartWithStatus(
	ctx context.Context,
	report func(PaymentModuleState, error),
) error {
	for {
		select {
		case update := <-module.updates:
			report(update.state, update.err)
		case <-ctx.Done():
			return nil
		}
	}
}

func (*statusTestPaymentModule) Stop(context.Context) error { return nil }

func TestTrustedPaymentModuleManager_StatusRunnerPreservesActiveDirectRail(t *testing.T) {
	registry := newTestPaymentRegistry()
	module := &statusTestPaymentModule{updates: make(chan paymentModuleStatusUpdate, 3)}
	module.updates <- paymentModuleStatusUpdate{state: PaymentModuleNeedsSetup, err: errors.New("wallet not provisioned")}

	manager, err := NewTrustedPaymentModuleManager(
		NewPaymentRuntimeAuthority(ManagedEVMRuntime{}, ManagedSolanaRuntime{}, ManagedEscrowGuestRuntimePorts{}, DirectObservedRuntimePorts{}),
		registry,
		module,
	)
	require.NoError(t, err)
	require.NoError(t, manager.Register(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, manager.Start(ctx, nil))
	health := manager.Health()[0]
	assert.Equal(t, PaymentModuleNeedsSetup, health.State)
	assert.True(t, health.Active)
	assert.Equal(t, []iwallet.ChainType{iwallet.ChainMonero}, health.Chains)

	module.updates <- paymentModuleStatusUpdate{state: PaymentModuleReady}
	require.Eventually(t, func() bool {
		health := manager.Health()[0]
		return health.State == PaymentModuleReady && health.Active && health.Error == ""
	}, time.Second, time.Millisecond)

	module.updates <- paymentModuleStatusUpdate{state: PaymentModuleDegraded, err: errors.New("daemon unavailable")}
	require.Eventually(t, func() bool {
		health := manager.Health()[0]
		return health.State == PaymentModuleDegraded && health.Active && health.Error == "daemon unavailable"
	}, time.Second, time.Millisecond)

	require.NoError(t, manager.Stop(context.Background()))
	assert.Equal(t, PaymentModuleStopped, manager.Health()[0].State)
}
