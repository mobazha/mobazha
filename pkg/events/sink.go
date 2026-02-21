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

// ConcurrentSink is an optional interface that EventSink implementations
// can implement to specify the desired number of worker goroutines.
// If not implemented, the Dispatcher uses a default of 2.
type ConcurrentSink interface {
	Concurrency() int
}
