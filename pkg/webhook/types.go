package webhook

import (
	"errors"
	"time"
)

var ErrEndpointNotFound = errors.New("webhook endpoint not found")

const (
	DeliveryStatusPending = "pending"
	DeliveryStatusSuccess = "success"
	DeliveryStatusFailed  = "failed"
)

// Endpoint represents a registered webhook URL.
type Endpoint struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Secret     string    `json:"-"`
	EventTypes string    `json:"event_types"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Delivery tracks an individual delivery attempt for a webhook event.
type Delivery struct {
	ID          string
	EndpointID  string
	EventType   string
	Payload     string
	Status      string // pending / success / failed
	Attempts    int
	MaxAttempts int
	NextRetryAt *time.Time
	CreatedAt   time.Time
}

// DeliveryResult holds the outcome of a single delivery attempt.
type DeliveryResult struct {
	Status     string
	StatusCode *int
	Error      string
	NextRetry  *time.Time
}

// EndpointStore abstracts webhook endpoint and delivery persistence.
// Standalone nodes inject a SQLite implementation; SaaS injects a multi-tenant GORM one.
// The interface is tenant-agnostic: tenant scoping is handled inside each implementation.
type EndpointStore interface {
	// ListActive returns all enabled endpoints.
	ListActive() ([]Endpoint, error)

	// GetEndpoint returns a single endpoint by ID.
	GetEndpoint(id string) (*Endpoint, error)

	// CreateEndpoint inserts a new endpoint, generating ID and secret if empty.
	CreateEndpoint(ep *Endpoint) error

	// UpdateEndpoint applies partial updates to an endpoint.
	UpdateEndpoint(id string, updates map[string]interface{}) error

	// DeleteEndpoint removes an endpoint and its deliveries.
	DeleteEndpoint(id string) error

	// ListEndpoints returns all endpoints (active and inactive).
	ListEndpoints() ([]Endpoint, error)

	// CountEndpoints returns the total number of endpoints.
	CountEndpoints() (int64, error)

	// CreateDeliveries batch-inserts delivery records.
	CreateDeliveries(deliveries []Delivery) error

	// GetPending returns deliveries ready for (re)delivery.
	GetPending(limit int) ([]Delivery, error)

	// UpdateResult records the outcome of a delivery attempt.
	UpdateResult(id string, result DeliveryResult) error

	// CleanupOld removes completed deliveries older than the given duration.
	CleanupOld(olderThan time.Duration) (int64, error)

	// ListDeliveries returns deliveries for a specific endpoint with pagination.
	ListDeliveries(endpointID string, status string, limit, offset int) ([]Delivery, int64, error)
}
