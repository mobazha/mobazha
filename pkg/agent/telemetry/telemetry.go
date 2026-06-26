package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Event types emitted by the agent runtime.
const (
	OverflowDetected       = "overflow_detected"
	ReplayShaped           = "replay_shaped"
	CompactionStarted      = "auto_compaction_started"
	CompactionSucceeded    = "auto_compaction_succeeded"
	CompactionFailed       = "auto_compaction_failed"
	ToolCallBatch          = "tool_call_batch"
	PendingRequestCreated  = "pending_request_created"
	PendingRequestResolved = "pending_request_resolved"
	TurnStarted            = "turn_started"
	TurnCompleted          = "turn_completed"
	TurnFailed             = "turn_failed"
	GuardrailBlocked       = "guardrail_blocked"
	LLMRetried             = "llm_retried"
	MemoryRetrieved        = "memory_retrieved"
	MemoryRetrievalFailed  = "memory_retrieval_failed"
	MemorySaved            = "memory_saved"
	MemorySaveFailed       = "memory_save_failed"
	MemoryDeleted          = "memory_deleted"
	MemoryDeleteFailed     = "memory_delete_failed"
)

// Event is a structured telemetry event from the agent runtime.
type Event struct {
	Type     string         `json:"type"`
	TenantID string         `json:"tenant_id,omitempty"`
	ThreadID string         `json:"thread_id,omitempty"`
	Attrs    map[string]any `json:"attrs,omitempty"`
	Time     time.Time      `json:"time"`
}

// Emitter is the interface for recording agent telemetry events.
type Emitter interface {
	Emit(ctx context.Context, event Event)
}

// LogEmitter writes events to the standard logger as JSON.
// Suitable for development, standalone deployments, and MVP.
type LogEmitter struct {
	mu     sync.Mutex
	logger *log.Logger
}

// NewLogEmitter creates an emitter that writes to the given logger.
// If logger is nil, the default standard logger is used.
func NewLogEmitter(logger *log.Logger) *LogEmitter {
	if logger == nil {
		logger = log.Default()
	}
	return &LogEmitter{logger: logger}
}

// Emit writes the event as JSON to the logger.
func (e *LogEmitter) Emit(_ context.Context, event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		e.mu.Lock()
		e.logger.Printf("[agent-telemetry] marshal error: %v, event_type=%s", err, event.Type)
		e.mu.Unlock()
		return
	}
	e.mu.Lock()
	e.logger.Printf("[agent-telemetry] %s", data)
	e.mu.Unlock()
}

// NoopEmitter discards all events silently.
type NoopEmitter struct{}

func (NoopEmitter) Emit(context.Context, Event) {}

// BufferEmitter captures events in memory for testing.
type BufferEmitter struct {
	mu     sync.Mutex
	Events []Event
}

func (b *BufferEmitter) Emit(_ context.Context, event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	b.mu.Lock()
	b.Events = append(b.Events, event)
	b.mu.Unlock()
}

// ByType returns all captured events of the given type.
func (b *BufferEmitter) ByType(eventType string) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []Event
	for _, e := range b.Events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// Count returns the number of captured events.
func (b *BufferEmitter) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.Events)
}
