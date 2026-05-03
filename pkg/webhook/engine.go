package webhook

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	defaultBatchSize = 50
)

// Engine manages webhook enqueuing and delivery.
// Delivery and cleanup are driven externally via RunDeliveryOnce / RunCleanupOnce
// (shared scheduler in SaaS, or per-node scheduler in standalone).
type Engine struct {
	store  EndpointStore
	cfg    Config
	client *http.Client
}

// NewEngine creates a new Engine. Delivery and cleanup are driven by the caller
// via RunDeliveryOnce / RunCleanupOnce — no internal goroutines are started.
func NewEngine(store EndpointStore, cfg Config) *Engine {
	return &Engine{
		store: store,
		cfg:   cfg,
		client: &http.Client{
			Timeout: cfg.httpTimeout(),
		},
	}
}

// Config returns the engine's current configuration (read-only snapshot).
func (e *Engine) Config() Config {
	return e.cfg
}

// CheckEndpointQuota returns an error if the node has reached its endpoint limit.
// Returns nil if the limit is 0 (unlimited) or if the current count is below the limit.
func (e *Engine) CheckEndpointQuota() error {
	if e.cfg.MaxEndpoints <= 0 {
		return nil
	}
	endpoints, err := e.store.ListActive()
	if err != nil {
		return fmt.Errorf("checking endpoint quota: %w", err)
	}
	if len(endpoints) >= e.cfg.MaxEndpoints {
		return fmt.Errorf("endpoint limit reached (%d/%d)", len(endpoints), e.cfg.MaxEndpoints)
	}
	return nil
}

// Enqueue finds active endpoints matching the event type and creates delivery records.
func (e *Engine) Enqueue(eventType string, payload []byte) {
	endpoints, err := e.store.ListActive()
	if err != nil {
		log.Printf("[webhook] Failed to list active endpoints: %v", err)
		return
	}

	maxRetries := e.cfg.maxRetries()
	var deliveries []Delivery
	now := time.Now()
	for _, ep := range endpoints {
		if !MatchEventFilter(ep.EventTypes, eventType) {
			continue
		}
		deliveries = append(deliveries, Delivery{
			EndpointID:  ep.ID,
			EventType:   eventType,
			Payload:     string(payload),
			Status:      DeliveryStatusPending,
			MaxAttempts: maxRetries,
			CreatedAt:   now,
		})
	}

	if len(deliveries) == 0 {
		return
	}

	if err := e.store.CreateDeliveries(deliveries); err != nil {
		log.Printf("[webhook] Failed to enqueue deliveries for event %s: %v", eventType, err)
	}
}

// RunDeliveryOnce processes pending webhook deliveries in a single pass.
func (e *Engine) RunDeliveryOnce() {
	e.processPendingDeliveries()
}

func (e *Engine) processPendingDeliveries() {
	deliveries, err := e.store.GetPending(defaultBatchSize)
	if err != nil {
		log.Printf("[webhook] Failed to fetch pending deliveries: %v", err)
		return
	}

	for i := range deliveries {
		e.deliver(&deliveries[i])
	}
}

func (e *Engine) deliver(d *Delivery) {
	ep, err := e.store.GetEndpoint(d.EndpointID)
	if err != nil {
		if errors.Is(err, ErrEndpointNotFound) {
			log.Printf("[webhook] Delivery %s: endpoint %s not found, marking failed", d.ID, d.EndpointID)
			_ = e.store.UpdateResult(d.ID, DeliveryResult{Status: DeliveryStatusFailed, Error: "endpoint not found"})
		} else {
			log.Printf("[webhook] Delivery %s: transient error fetching endpoint %s: %v, will retry", d.ID, d.EndpointID, err)
		}
		return
	}

	body := []byte(d.Payload)
	sig, ts := WebhookHeaders(ep.Secret, d.ID, body)

	req, err := http.NewRequest("POST", ep.URL, bytes.NewReader(body))
	if err != nil {
		_ = e.store.UpdateResult(d.ID, DeliveryResult{Status: DeliveryStatusFailed, Error: err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-ID", d.ID)
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, err := e.client.Do(req)
	if err != nil {
		e.markRetryOrFail(d, nil, err.Error())
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	statusCode := resp.StatusCode
	if statusCode >= 200 && statusCode < 300 {
		_ = e.store.UpdateResult(d.ID, DeliveryResult{Status: DeliveryStatusSuccess, StatusCode: &statusCode})
	} else {
		e.markRetryOrFail(d, &statusCode, http.StatusText(statusCode))
	}
}

func (e *Engine) markRetryOrFail(d *Delivery, statusCode *int, errMsg string) {
	nextAttempt := d.Attempts + 1
	if nextAttempt >= d.MaxAttempts {
		_ = e.store.UpdateResult(d.ID, DeliveryResult{Status: DeliveryStatusFailed, StatusCode: statusCode, Error: errMsg})
		return
	}

	nextRetry := time.Now().Add(RetryBackoff(d.Attempts))
	_ = e.store.UpdateResult(d.ID, DeliveryResult{
		Status:     DeliveryStatusPending,
		StatusCode: statusCode,
		Error:      errMsg,
		NextRetry:  &nextRetry,
	})
}

// RunCleanupOnce removes old webhook deliveries in a single pass.
func (e *Engine) RunCleanupOnce() {
	deleted, err := e.store.CleanupOld(e.cfg.retentionAge())
	if err != nil {
		log.Printf("[webhook] Failed to cleanup old deliveries: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[webhook] Cleaned up %d old deliveries", deleted)
	}
}
