package webhook

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/events"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
	"github.com/op/go-logging"
)

var sinkLog = logging.MustGetLogger("WHSK")

// WebhookSink is an EventSink that forwards events to the webhook Engine.
// It replaces the previous Bridge pattern that directly subscribed to EventBus.
type WebhookSink struct {
	engine *wh.Engine
	nodeID string
}

// NewWebhookSink creates a new WebhookSink.
func NewWebhookSink(engine *wh.Engine, nodeID string) *WebhookSink {
	return &WebhookSink{engine: engine, nodeID: nodeID}
}

// Name implements events.EventSink.
func (s *WebhookSink) Name() string { return "webhook" }

// Concurrency implements events.ConcurrentSink.
// Webhook delivery benefits from parallel workers for HTTP fan-out.
func (s *WebhookSink) Concurrency() int { return 4 }

// Accept implements events.EventSink.
// Always returns true — Engine.Enqueue filters per-endpoint by EventTypes.
func (s *WebhookSink) Accept(_ events.EventMeta) bool { return true }

// Handle implements events.EventSink.
func (s *WebhookSink) Handle(_ context.Context, meta events.EventMeta, event interface{}) error {
	payload, err := wh.BuildCloudEvent(s.nodeID, meta.Name, event)
	if err != nil {
		sinkLog.Errorf("Failed to build CloudEvent for %s: %v", meta.Name, err)
		return err
	}
	s.engine.Enqueue(meta.Name, payload)
	return nil
}
