package telemetry

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

func TestBufferEmitter_CaptureAndFilter(t *testing.T) {
	buf := &BufferEmitter{}
	ctx := context.Background()

	buf.Emit(ctx, Event{Type: OverflowDetected, TenantID: "t1", Attrs: map[string]any{"estimated": 100000}})
	buf.Emit(ctx, Event{Type: TurnStarted, ThreadID: "th1"})
	buf.Emit(ctx, Event{Type: OverflowDetected, TenantID: "t2"})

	if buf.Count() != 3 {
		t.Fatalf("expected 3 events, got %d", buf.Count())
	}

	overflows := buf.ByType(OverflowDetected)
	if len(overflows) != 2 {
		t.Fatalf("expected 2 overflow events, got %d", len(overflows))
	}

	turns := buf.ByType(TurnStarted)
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn event, got %d", len(turns))
	}
}

func TestBufferEmitter_AutoTimestamp(t *testing.T) {
	buf := &BufferEmitter{}
	buf.Emit(context.Background(), Event{Type: TurnCompleted})
	if buf.Events[0].Time.IsZero() {
		t.Error("expected auto-filled timestamp")
	}
}

func TestLogEmitter_WritesJSON(t *testing.T) {
	var out bytes.Buffer
	logger := log.New(&out, "", 0)
	emitter := NewLogEmitter(logger)

	emitter.Emit(context.Background(), Event{
		Type:     ToolCallBatch,
		TenantID: "tenant1",
		Attrs:    map[string]any{"count": 3, "duration_ms": 150},
	})

	output := out.String()
	if !strings.Contains(output, "[agent-telemetry]") {
		t.Errorf("expected telemetry prefix in output: %s", output)
	}
	if !strings.Contains(output, `"tool_call_batch"`) {
		t.Errorf("expected event type in output: %s", output)
	}
	if !strings.Contains(output, `"tenant1"`) {
		t.Errorf("expected tenant_id in output: %s", output)
	}
}

func TestNoopEmitter_DoesNotPanic(t *testing.T) {
	noop := NoopEmitter{}
	noop.Emit(context.Background(), Event{Type: CompactionStarted})
}

func TestAllEventConstants(t *testing.T) {
	constants := []string{
		OverflowDetected, ReplayShaped, CompactionStarted,
		CompactionSucceeded, CompactionFailed, ToolCallBatch,
		PendingRequestCreated, PendingRequestResolved,
		TurnStarted, TurnCompleted, GuardrailBlocked, LLMRetried,
		MemoryRetrieved, MemoryRetrievalFailed,
		MemorySaved, MemorySaveFailed,
	}
	seen := make(map[string]bool)
	for _, c := range constants {
		if c == "" {
			t.Error("empty event constant")
		}
		if seen[c] {
			t.Errorf("duplicate event constant: %s", c)
		}
		seen[c] = true
	}
}
