package webhook

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/events"
)

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
	EventChatMessage          = "chat.message"
)

var allWebhookEventTypes = []string{
	EventOrderCreated,
	EventOrderFunded,
	EventOrderPaymentReceived,
	EventOrderConfirmed,
	EventOrderFulfilled,
	EventOrderCompleted,
	EventOrderCancelled,
	EventOrderDeclined,
	EventOrderRefunded,
	EventDisputeOpened,
	EventDisputeClosed,
	EventChatMessage,
}

// AllWebhookEventTypes returns a copy of all supported webhook event types.
func AllWebhookEventTypes() []string {
	cp := make([]string, len(allWebhookEventTypes))
	copy(cp, allWebhookEventTypes)
	return cp
}

// ClassifyEvent maps a Go event value to a webhook event type string.
// Returns empty string for unsupported event types.
func ClassifyEvent(evt interface{}) string {
	switch evt.(type) {
	case events.NewOrder, *events.NewOrder:
		return EventOrderCreated
	case events.OrderFunded, *events.OrderFunded:
		return EventOrderFunded
	case events.OrderPaymentReceived, *events.OrderPaymentReceived:
		return EventOrderPaymentReceived
	case events.OrderConfirmation, *events.OrderConfirmation:
		return EventOrderConfirmed
	case events.OrderFulfillment, *events.OrderFulfillment:
		return EventOrderFulfilled
	case events.OrderCompletion, *events.OrderCompletion:
		return EventOrderCompleted
	case events.OrderCancel, *events.OrderCancel:
		return EventOrderCancelled
	case events.OrderDeclined, *events.OrderDeclined:
		return EventOrderDeclined
	case events.Refund, *events.Refund:
		return EventOrderRefunded
	case events.DisputeOpen, *events.DisputeOpen:
		return EventDisputeOpened
	case events.DisputeClose, *events.DisputeClose:
		return EventDisputeClosed
	case events.ChatMessage, *events.ChatMessage:
		return EventChatMessage
	default:
		return ""
	}
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

// WebhookBusEventTypes returns the Go event struct pointers that the EventBus should
// subscribe to for webhook forwarding. Bridge uses this to avoid duplicating the list.
func WebhookBusEventTypes() []interface{} {
	return []interface{}{
		new(events.NewOrder),
		new(events.OrderFunded),
		new(events.OrderPaymentReceived),
		new(events.OrderConfirmation),
		new(events.OrderFulfillment),
		new(events.OrderCompletion),
		new(events.OrderCancel),
		new(events.OrderDeclined),
		new(events.Refund),
		new(events.DisputeOpen),
		new(events.DisputeClose),
		new(events.ChatMessage),
	}
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
