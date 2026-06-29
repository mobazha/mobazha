package paymentplugin

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// HealthStatus is the supervisor-reported activation state of a plugin.
type HealthStatus string

const (
	HealthStarting  HealthStatus = "starting"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStopped   HealthStatus = "stopped"
)

// Registration is an immutable manifest plus mutable health projection.
type Registration struct {
	Manifest      Manifest     `json:"manifest"`
	Health        HealthStatus `json:"health"`
	HealthMessage string       `json:"healthMessage,omitempty"`
	CheckedAt     time.Time    `json:"checkedAt,omitempty"`
}

// Registry tracks validated plugin manifests and health without granting
// plugins direct access to Core internals or financial state transitions.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Registration
}

// NewRegistry creates an empty payment plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Registration)}
}

// Register validates and inserts a plugin in the starting state. Duplicate
// IDs fail so upgrades must use an explicit drain/unregister/register flow.
func (r *Registry) Register(manifest Manifest) error {
	if r == nil {
		return fmt.Errorf("payment plugin registry is nil")
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[manifest.ID]; exists {
		return fmt.Errorf("payment plugin %q is already registered", manifest.ID)
	}
	r.plugins[manifest.ID] = Registration{Manifest: cloneManifest(manifest), Health: HealthStarting}
	return nil
}

// Unregister removes a plugin after its supervisor has drained and stopped it.
// Core-owned payment facts and order history are intentionally outside this
// registry.
func (r *Registry) Unregister(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, id)
}

// SetHealth updates the supervisor projection for an existing plugin.
func (r *Registry) SetHealth(id string, status HealthStatus, message string, checkedAt time.Time) error {
	if r == nil {
		return fmt.Errorf("payment plugin registry is nil")
	}
	if !status.valid() {
		return fmt.Errorf("payment plugin %q: invalid health status %q", id, status)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	registration, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("payment plugin %q is not registered", id)
	}
	registration.Health = status
	registration.HealthMessage = message
	registration.CheckedAt = checkedAt
	r.plugins[id] = registration
	return nil
}

// Active returns healthy plugin manifests in stable ID order. Degraded or
// unhealthy plugins are not buyer-visible and therefore fail closed.
func (r *Registry) Active() []Manifest {
	if r == nil {
		return []Manifest{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	manifests := make([]Manifest, 0, len(r.plugins))
	for _, registration := range r.plugins {
		if registration.Health == HealthHealthy {
			manifests = append(manifests, cloneManifest(registration.Manifest))
		}
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	return manifests
}

// Snapshot returns all registrations in stable ID order for admin and audit
// surfaces. The returned slice does not alias registry storage.
func (r *Registry) Snapshot() []Registration {
	if r == nil {
		return []Registration{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	registrations := make([]Registration, 0, len(r.plugins))
	for _, registration := range r.plugins {
		registration.Manifest = cloneManifest(registration.Manifest)
		registrations = append(registrations, registration)
	}
	sort.Slice(registrations, func(i, j int) bool {
		return registrations[i].Manifest.ID < registrations[j].Manifest.ID
	})
	return registrations
}

func cloneManifest(manifest Manifest) Manifest {
	clone := manifest
	clone.Chains = make([]Chain, len(manifest.Chains))
	for i, chain := range manifest.Chains {
		clone.Chains[i] = chain
		clone.Chains[i].Assets = append([]string(nil), chain.Assets...)
	}
	clone.Capabilities = append([]Capability(nil), manifest.Capabilities...)
	clone.OptionalCapabilities = append([]Capability(nil), manifest.OptionalCapabilities...)
	clone.Permissions.Network = append([]string(nil), manifest.Permissions.Network...)
	clone.Permissions.Signing = append([]SigningPermission(nil), manifest.Permissions.Signing...)
	return clone
}

func (s HealthStatus) valid() bool {
	switch s {
	case HealthStarting, HealthHealthy, HealthDegraded, HealthUnhealthy, HealthStopped:
		return true
	default:
		return false
	}
}
