package webhook

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/events"
)

// Event type constants kept for convenience. Values match events.EventMeta.Name.
const (
	EventOrderCreated         = "order.created"
	EventOrderFunded          = "order.funded"
	EventOrderPaymentReceived = "order.payment_received"
	EventOrderConfirmed       = "order.confirmed"
	EventOrderFulfilled       = "order.fulfilled"
	EventOrderCompleted       = "order.completed"
	EventOrderCancelled       = "order.cancelled"
	EventOrderDeclined        = "order.declined"
	EventOrderRefunded        = "order.refunded"
	EventDisputeOpened        = "dispute.opened"
	EventDisputeClosed        = "dispute.closed"
)

// AllWebhookEventTypes returns all registered event names that webhooks can subscribe to.
func AllWebhookEventTypes() []string {
	return events.AllEventNames()
}

// ClassifyEvent maps a Go event value to a dot-separated event type string
// using the Event Registry. Returns empty string for unregistered event types.
func ClassifyEvent(evt interface{}) string {
	meta := events.LookupEvent(evt)
	if meta == nil {
		return ""
	}
	return meta.Name
}

// CloudEvent represents a CloudEvents v1.0 structured-mode envelope.
type CloudEvent struct {
	SpecVersion     string      `json:"specversion"`
	ID              string      `json:"id"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	Time            string      `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data"`
}

// BuildCloudEvent wraps a classified event into a CloudEvents v1.0 JSON payload.
func BuildCloudEvent(tenantID, eventType string, data interface{}) ([]byte, error) {
	ce := CloudEvent{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Type:            "com.mobazha." + eventType,
		Source:          "/tenants/" + tenantID,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            data,
	}
	return json.Marshal(ce)
}

// MatchEventFilter checks whether eventType matches a comma-separated filter string.
// Supports exact match ("order.created") and wildcard ("order.*").
func MatchEventFilter(filter, eventType string) bool {
	parts := strings.Split(filter, ",")
	for _, part := range parts {
		pattern := strings.TrimSpace(part)
		if pattern == "" {
			continue
		}
		if pattern == eventType {
			return true
		}
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(eventType, prefix) {
				return true
			}
		}
	}
	return false
}
