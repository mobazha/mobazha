package distribution

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PaymentModuleState is the manager-owned lifecycle state of one trusted
// in-process payment module.
type PaymentModuleState string

const (
	PaymentModuleStopped  PaymentModuleState = "stopped"
	PaymentModuleStarting PaymentModuleState = "starting"
	PaymentModuleReady    PaymentModuleState = "ready"
	PaymentModuleDegraded PaymentModuleState = "degraded"
)

// PaymentModuleHealth is an immutable manager snapshot suitable for product
// capability projection and operational diagnostics.
type PaymentModuleHealth struct {
	Descriptor PaymentModuleDescriptor
	State      PaymentModuleState
	Chains     []iwallet.ChainType
	Error      string
}

// TrustedPaymentModuleManager owns descriptor validation, dependency order,
// atomic registration, per-module contributions, lifecycle, and health.
type TrustedPaymentModuleManager struct {
	authority PaymentRuntimeAuthority
	target    PaymentRegistry
	modules   []PaymentModule

	mu            sync.RWMutex
	registrations []paymentModuleRegistration
	health        map[string]PaymentModuleHealth
	active        map[string]bool
	registered    bool
	started       bool
	stopOnce      sync.Once
	done          chan struct{}
	onHealth      func(PaymentModuleHealth)
}

// NewTrustedPaymentModuleManager validates descriptors and establishes a
// stable dependency order before Core resources are exposed to modules.
func NewTrustedPaymentModuleManager(
	authority PaymentRuntimeAuthority,
	target PaymentRegistry,
	modules ...PaymentModule,
) (*TrustedPaymentModuleManager, error) {
	if target == nil {
		return nil, fmt.Errorf("payment module registry is required")
	}
	ordered, err := orderPaymentModules(modules)
	if err != nil {
		return nil, err
	}
	health := make(map[string]PaymentModuleHealth, len(ordered))
	for _, module := range ordered {
		descriptor := normalizedPaymentModuleDescriptor(module.Descriptor())
		health[descriptor.ID] = PaymentModuleHealth{Descriptor: descriptor, State: PaymentModuleStopped}
	}
	return &TrustedPaymentModuleManager{
		authority: authority,
		target:    target,
		modules:   ordered,
		health:    health,
		active:    make(map[string]bool, len(ordered)),
		done:      make(chan struct{}),
	}, nil
}

// Register prepares every module and atomically commits the full strategy set.
func (m *TrustedPaymentModuleManager) Register(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("trusted payment module manager is nil")
	}
	m.mu.Lock()
	if m.registered {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are already registered")
	}
	m.mu.Unlock()

	registrations, err := registerPaymentModules(ctx, m.authority, m.target, m.modules...)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.registrations = registrations
	m.registered = true
	for _, registration := range registrations {
		id := strings.TrimSpace(registration.module.Descriptor().ID)
		health := m.health[id]
		health.Chains = append([]iwallet.ChainType(nil), registration.chains...)
		m.health[id] = health
		m.active[id] = true
	}
	m.mu.Unlock()
	return nil
}

// Start launches module runners in dependency order. A runtime failure removes
// only that module's live contribution; unrelated modules remain active.
func (m *TrustedPaymentModuleManager) Start(ctx context.Context, onHealth func(PaymentModuleHealth)) error {
	if m == nil {
		return fmt.Errorf("trusted payment module manager is nil")
	}
	m.mu.Lock()
	if !m.registered {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are not registered")
	}
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are already started")
	}
	m.started = true
	m.onHealth = onHealth
	registrations := append([]paymentModuleRegistration(nil), m.registrations...)
	m.mu.Unlock()

	for _, registration := range registrations {
		id := strings.TrimSpace(registration.module.Descriptor().ID)
		m.publish(id, PaymentModuleStarting, "")
		runner, ok := registration.module.(PaymentModuleRunner)
		if !ok {
			m.publish(id, PaymentModuleReady, "")
			continue
		}
		m.publish(id, PaymentModuleReady, "")
		go m.run(ctx, registration, runner)
	}
	go func() {
		select {
		case <-ctx.Done():
		case <-m.done:
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.Stop(cleanupCtx)
	}()
	return nil
}

func (m *TrustedPaymentModuleManager) run(ctx context.Context, registration paymentModuleRegistration, runner PaymentModuleRunner) {
	err := runner.Start(ctx)
	if ctx.Err() != nil {
		return
	}
	if err == nil {
		err = fmt.Errorf("module returned before node shutdown")
	}
	m.deactivate(registration, err)
}

func (m *TrustedPaymentModuleManager) deactivate(registration paymentModuleRegistration, cause error) {
	id := strings.TrimSpace(registration.module.Descriptor().ID)
	m.mu.Lock()
	if !m.active[id] {
		m.mu.Unlock()
		return
	}
	m.active[id] = false
	m.mu.Unlock()

	m.target.UnregisterV2Batch(registration.chains)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cleanupErrors := []error{cause}
	if runner, ok := registration.module.(PaymentModuleRunner); ok {
		if err := runner.Stop(cleanupCtx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stop module %q: %w", id, err))
		}
	}
	if binder, ok := registration.module.(PaymentModuleBinder); ok {
		if err := binder.Unbind(cleanupCtx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("unbind module %q: %w", id, err))
		}
	}
	if err := registration.module.RollbackRegistration(cleanupCtx); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("rollback module %q: %w", id, err))
	}
	m.publish(id, PaymentModuleDegraded, errors.Join(cleanupErrors...).Error())
}

// Stop shuts down active modules in reverse dependency order.
func (m *TrustedPaymentModuleManager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var result error
	m.stopOnce.Do(func() {
		close(m.done)
		m.mu.RLock()
		registrations := append([]paymentModuleRegistration(nil), m.registrations...)
		m.mu.RUnlock()
		var cleanupErrors []error
		for index := len(registrations) - 1; index >= 0; index-- {
			registration := registrations[index]
			id := strings.TrimSpace(registration.module.Descriptor().ID)
			m.mu.Lock()
			active := m.active[id]
			m.active[id] = false
			m.mu.Unlock()
			if !active {
				continue
			}
			m.target.UnregisterV2Batch(registration.chains)
			if runner, ok := registration.module.(PaymentModuleRunner); ok {
				if err := runner.Stop(ctx); err != nil {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("stop module %q: %w", id, err))
				}
			}
			if binder, ok := registration.module.(PaymentModuleBinder); ok {
				if err := binder.Unbind(ctx); err != nil {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("unbind module %q: %w", id, err))
				}
			}
			if err := registration.module.RollbackRegistration(ctx); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("rollback module %q: %w", id, err))
			}
			m.publish(id, PaymentModuleStopped, "")
		}
		result = errors.Join(cleanupErrors...)
	})
	return result
}

// Health returns stable, ID-sorted snapshots.
func (m *TrustedPaymentModuleManager) Health() []PaymentModuleHealth {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	result := make([]PaymentModuleHealth, 0, len(m.health))
	for _, health := range m.health {
		health.Chains = append([]iwallet.ChainType(nil), health.Chains...)
		result = append(result, health)
	}
	m.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Descriptor.ID < result[j].Descriptor.ID })
	return result
}

func (m *TrustedPaymentModuleManager) publish(id string, state PaymentModuleState, detail string) {
	m.mu.Lock()
	health := m.health[id]
	health.State = state
	health.Error = detail
	m.health[id] = health
	callback := m.onHealth
	m.mu.Unlock()
	if callback != nil {
		callback(health)
	}
}

func orderPaymentModules(modules []PaymentModule) ([]PaymentModule, error) {
	byID := make(map[string]PaymentModule, len(modules))
	descriptors := make(map[string]PaymentModuleDescriptor, len(modules))
	order := make([]string, 0, len(modules))
	for index, module := range modules {
		if isNilInterface(module) {
			return nil, fmt.Errorf("payment module at index %d is nil", index)
		}
		descriptor := normalizedPaymentModuleDescriptor(module.Descriptor())
		if err := validatePaymentModuleDescriptor(descriptor); err != nil {
			return nil, err
		}
		if _, exists := byID[descriptor.ID]; exists {
			return nil, fmt.Errorf("payment module ID %q is registered more than once", descriptor.ID)
		}
		byID[descriptor.ID] = module
		descriptors[descriptor.ID] = descriptor
		order = append(order, descriptor.ID)
	}
	for id, descriptor := range descriptors {
		for _, dependency := range descriptor.Dependencies {
			if _, exists := byID[dependency]; !exists {
				return nil, fmt.Errorf("payment module %q requires missing dependency %q", id, dependency)
			}
		}
	}

	state := make(map[string]uint8, len(modules))
	ordered := make([]PaymentModule, 0, len(modules))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("payment module dependency cycle includes %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, dependency := range descriptors[id].Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = 2
		ordered = append(ordered, byID[id])
		return nil
	}
	for _, id := range order {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func normalizedPaymentModuleDescriptor(descriptor PaymentModuleDescriptor) PaymentModuleDescriptor {
	descriptor.ID = strings.TrimSpace(descriptor.ID)
	descriptor.Version = strings.TrimSpace(descriptor.Version)
	for index := range descriptor.Dependencies {
		descriptor.Dependencies[index] = strings.TrimSpace(descriptor.Dependencies[index])
	}
	return descriptor
}

func validatePaymentModuleDescriptor(descriptor PaymentModuleDescriptor) error {
	if descriptor.ID == "" {
		return fmt.Errorf("payment module descriptor ID is required")
	}
	if descriptor.Version == "" {
		return fmt.Errorf("payment module %q version is required", descriptor.ID)
	}
	if len(descriptor.Rails) == 0 {
		return fmt.Errorf("payment module %q must declare at least one rail", descriptor.ID)
	}
	for _, rail := range descriptor.Rails {
		switch rail {
		case PaymentRailEscrow, PaymentRailDirectObserved, PaymentRailProviderSession:
		default:
			return fmt.Errorf("payment module %q declares unknown rail %q", descriptor.ID, rail)
		}
	}
	switch descriptor.Activation {
	case PaymentModuleRequired, PaymentModuleOptional, PaymentModuleSetupGated:
	default:
		return fmt.Errorf("payment module %q activation requirement is invalid", descriptor.ID)
	}
	for _, dependency := range descriptor.Dependencies {
		if dependency == "" || dependency == descriptor.ID {
			return fmt.Errorf("payment module %q has invalid dependency %q", descriptor.ID, dependency)
		}
	}
	return nil
}
