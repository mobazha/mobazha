// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

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
	PaymentModuleStopped    PaymentModuleState = "stopped"
	PaymentModuleStarting   PaymentModuleState = "starting"
	PaymentModuleReady      PaymentModuleState = "ready"
	PaymentModuleNeedsSetup PaymentModuleState = "needs_setup"
	PaymentModuleDegraded   PaymentModuleState = "degraded"
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

	lifecycleMu     sync.Mutex
	stopMu          sync.Mutex
	mu              sync.RWMutex
	registrations   []paymentModuleRegistration
	health          map[string]PaymentModuleHealth
	active          map[string]bool
	cleanupComplete map[string]bool
	registered      bool
	started         bool
	stopped         bool
	runCancel       context.CancelFunc
	runWG           sync.WaitGroup
	doneOnce        sync.Once
	done            chan struct{}
	onHealth        func(PaymentModuleHealth)
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
		authority:       authority,
		target:          target,
		modules:         ordered,
		health:          health,
		active:          make(map[string]bool, len(ordered)),
		cleanupComplete: make(map[string]bool, len(ordered)),
		done:            make(chan struct{}),
	}, nil
}

// Register prepares modules in dependency order and atomically commits each
// module's complete strategy set according to its activation policy.
func (m *TrustedPaymentModuleManager) Register(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("trusted payment module manager is nil")
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	if m.registered {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are already registered")
	}
	m.mu.Unlock()

	registrations, failures, err := registerPaymentModules(ctx, m.authority, m.target, m.modules...)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.registrations = registrations
	m.registered = true
	for _, registration := range registrations {
		id := registration.descriptor.ID
		health := m.health[id]
		health.Chains = append([]iwallet.ChainType(nil), registration.chains...)
		m.health[id] = health
		m.active[id] = true
	}
	for _, failure := range failures {
		health := m.health[failure.descriptor.ID]
		health.State = unavailablePaymentModuleState(failure.descriptor.Activation)
		health.Error = failure.err.Error()
		m.health[failure.descriptor.ID] = health
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
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	if !m.registered {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are not registered")
	}
	if m.stopped {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are stopped")
	}
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("trusted payment modules are already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.runCancel = cancel
	m.started = true
	m.onHealth = onHealth
	registrations := append([]paymentModuleRegistration(nil), m.registrations...)
	m.mu.Unlock()
	if onHealth != nil {
		for _, health := range m.Health() {
			if health.State == PaymentModuleDegraded || health.State == PaymentModuleNeedsSetup {
				onHealth(health)
			}
		}
	}

	for _, registration := range registrations {
		id := registration.descriptor.ID
		if dependency := m.unavailableDependency(registration.descriptor); dependency != "" {
			cause := fmt.Errorf("payment module %q dependency %q is unavailable", id, dependency)
			m.deactivateCascade(registration, cause)
			if registration.descriptor.Activation == PaymentModuleRequired {
				return m.abortStart(runCtx, cause)
			}
			continue
		}
		m.publish(id, PaymentModuleStarting, "")
		runner, ok := registration.module.(PaymentModuleRunner)
		if !ok {
			m.publish(id, PaymentModuleReady, "")
			continue
		}
		ready := make(chan struct{})
		result := make(chan error, 1)
		var readyOnce sync.Once
		m.runWG.Add(1)
		go func() {
			defer m.runWG.Done()
			result <- runner.Start(runCtx, func() { readyOnce.Do(func() { close(ready) }) })
		}()
		select {
		case <-ready:
			m.publish(id, PaymentModuleReady, "")
			go m.watchRunner(runCtx, registration, result)
		case err := <-result:
			if err == nil {
				err = fmt.Errorf("module returned before reporting readiness")
			}
			m.deactivateCascade(registration, err)
			if registration.descriptor.Activation == PaymentModuleRequired {
				return m.abortStart(runCtx, err)
			}
		case <-runCtx.Done():
			return m.abortStart(runCtx, runCtx.Err())
		}
	}
	go func() {
		select {
		case <-runCtx.Done():
		case <-m.done:
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.Stop(cleanupCtx)
	}()
	return nil
}

func (m *TrustedPaymentModuleManager) watchRunner(ctx context.Context, registration paymentModuleRegistration, result <-chan error) {
	select {
	case err := <-result:
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("module returned before node shutdown")
		}
		m.deactivateCascade(registration, err)
	case <-ctx.Done():
	}
}

func (m *TrustedPaymentModuleManager) deactivateCascade(registration paymentModuleRegistration, cause error) {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()
	affected := map[string]error{registration.descriptor.ID: cause}
	for _, candidate := range m.registrations {
		for _, dependency := range candidate.descriptor.Dependencies {
			if dependencyCause, ok := affected[dependency]; ok {
				affected[candidate.descriptor.ID] = fmt.Errorf("dependency %q failed: %w", dependency, dependencyCause)
				break
			}
		}
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for index := len(m.registrations) - 1; index >= 0; index-- {
		candidate := m.registrations[index]
		candidateCause, ok := affected[candidate.descriptor.ID]
		if !ok {
			continue
		}
		cleanupErr := m.cleanupRegistration(cleanupCtx, candidate, PaymentModuleDegraded, candidateCause.Error())
		if cleanupErr != nil {
			m.publish(candidate.descriptor.ID, PaymentModuleDegraded, errors.Join(candidateCause, cleanupErr).Error())
		}
	}
}

// Stop shuts down active modules in reverse dependency order.
func (m *TrustedPaymentModuleManager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.stop(ctx)
}

func (m *TrustedPaymentModuleManager) abortStart(_ context.Context, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(cause, m.stop(cleanupCtx))
}

func (m *TrustedPaymentModuleManager) stop(ctx context.Context) error {
	m.doneOnce.Do(func() { close(m.done) })
	m.mu.Lock()
	if m.runCancel != nil {
		m.runCancel()
	}
	registrations := append([]paymentModuleRegistration(nil), m.registrations...)
	m.mu.Unlock()

	m.stopMu.Lock()
	var cleanupErrors []error
	for index := len(registrations) - 1; index >= 0; index-- {
		if err := m.cleanupRegistration(ctx, registrations[index], PaymentModuleStopped, ""); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	m.stopMu.Unlock()

	if err := m.waitForRunners(ctx); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	m.mu.Lock()
	allClean := true
	for _, registration := range registrations {
		if !m.cleanupComplete[registration.descriptor.ID] {
			allClean = false
			break
		}
	}
	if allClean {
		m.started = false
		m.stopped = true
	}
	m.mu.Unlock()
	return errors.Join(cleanupErrors...)
}

func (m *TrustedPaymentModuleManager) cleanupRegistration(
	ctx context.Context,
	registration paymentModuleRegistration,
	state PaymentModuleState,
	detail string,
) error {
	id := registration.descriptor.ID
	m.mu.Lock()
	if m.cleanupComplete[id] {
		m.mu.Unlock()
		if state == PaymentModuleStopped {
			m.publish(id, PaymentModuleStopped, "")
		}
		return nil
	}
	active := m.active[id]
	m.active[id] = false
	m.mu.Unlock()

	if active {
		m.target.UnregisterV2Batch(registration.chains)
	}
	var cleanupErrors []error
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
	cleanupErr := errors.Join(cleanupErrors...)
	m.mu.Lock()
	if cleanupErr == nil {
		m.cleanupComplete[id] = true
	}
	m.mu.Unlock()
	publishedState := state
	if cleanupErr != nil {
		publishedState = PaymentModuleDegraded
		if detail == "" {
			detail = cleanupErr.Error()
		} else {
			detail = errors.Join(errors.New(detail), cleanupErr).Error()
		}
	}
	m.publish(id, publishedState, detail)
	return cleanupErr
}

func (m *TrustedPaymentModuleManager) waitForRunners(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.runWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for payment module runners: %w", ctx.Err())
	}
}

func (m *TrustedPaymentModuleManager) unavailableDependency(descriptor PaymentModuleDescriptor) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, dependency := range descriptor.Dependencies {
		if !m.active[dependency] {
			return dependency
		}
	}
	return ""
}

func unavailablePaymentModuleState(activation PaymentModuleActivation) PaymentModuleState {
	if activation == PaymentModuleSetupGated {
		return PaymentModuleNeedsSetup
	}
	return PaymentModuleDegraded
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
		health.Descriptor = clonePaymentModuleDescriptor(health.Descriptor)
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
	snapshot := health
	snapshot.Descriptor = clonePaymentModuleDescriptor(snapshot.Descriptor)
	snapshot.Chains = append([]iwallet.ChainType(nil), snapshot.Chains...)
	m.mu.Unlock()
	if callback != nil {
		callback(snapshot)
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
	descriptor = clonePaymentModuleDescriptor(descriptor)
	descriptor.ID = strings.TrimSpace(descriptor.ID)
	descriptor.Version = strings.TrimSpace(descriptor.Version)
	for index := range descriptor.Dependencies {
		descriptor.Dependencies[index] = strings.TrimSpace(descriptor.Dependencies[index])
	}
	return descriptor
}

func clonePaymentModuleDescriptor(descriptor PaymentModuleDescriptor) PaymentModuleDescriptor {
	descriptor.Rails = append([]PaymentRailKind(nil), descriptor.Rails...)
	descriptor.Capabilities = append([]PaymentModuleCapability(nil), descriptor.Capabilities...)
	descriptor.Dependencies = append([]string(nil), descriptor.Dependencies...)
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
