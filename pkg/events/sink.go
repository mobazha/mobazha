package events

import "context"

// EventSink is the interface implemented by event consumers.
// The Dispatcher calls Accept to filter, then Handle in a dedicated goroutine.
type EventSink interface {
	// Name returns a stable identifier for logging and metrics.
	Name() string

	// Accept returns true if this sink is interested in the event.
	Accept(meta EventMeta) bool

	// Handle processes the event. Called in a goroutine managed by the Dispatcher.
	Handle(ctx context.Context, meta EventMeta, event interface{}) error
}
