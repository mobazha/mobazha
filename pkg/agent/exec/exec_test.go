package exec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func echoExecutor() ToolExecutorFunc {
	return func(_ context.Context, call ToolCall) (ToolResult, error) {
		return ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("result of %s(%s)", call.Name, call.Arguments),
		}, nil
	}
}

func TestBatchExecutor_Parallel_Basic(t *testing.T) {
	be := NewBatchExecutor(echoExecutor(), 5*time.Second, 0)
	calls := []ToolCall{
		{ID: "1", Name: "search", Arguments: `{"q":"test"}`},
		{ID: "2", Name: "list", Arguments: `{}`},
	}

	results, err := be.Execute(context.Background(), calls, Parallel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.CallID != calls[i].ID {
			t.Errorf("result[%d] callID mismatch: %s vs %s", i, r.CallID, calls[i].ID)
		}
		if r.IsError {
			t.Errorf("result[%d] unexpected error: %s", i, r.Content)
		}
	}
}

func TestBatchExecutor_Serial_Basic(t *testing.T) {
	be := NewBatchExecutor(echoExecutor(), 5*time.Second, 0)
	calls := []ToolCall{
		{ID: "1", Name: "a", Arguments: "{}"},
		{ID: "2", Name: "b", Arguments: "{}"},
		{ID: "3", Name: "c", Arguments: "{}"},
	}

	results, err := be.Execute(context.Background(), calls, Serial)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestBatchExecutor_Serial_ReportsErrorAndSkipsRemainingCalls(t *testing.T) {
	callCount := 0
	executor := ToolExecutorFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
		callCount++
		if call.Name == "fail" {
			return ToolResult{}, errors.New("intentional failure")
		}
		return ToolResult{CallID: call.ID, Name: call.Name, Content: "ok"}, nil
	})

	be := NewBatchExecutor(executor, 5*time.Second, 0)
	calls := []ToolCall{
		{ID: "1", Name: "ok1"},
		{ID: "2", Name: "fail"},
		{ID: "3", Name: "ok2"},
	}

	results, err := be.Execute(context.Background(), calls, Serial)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(results) != 3 {
		t.Errorf("expected 3 correlated results, got %d", len(results))
	}
	if callCount != 2 {
		t.Errorf("expected execution to stop after 2 calls, got %d", callCount)
	}
	if !results[1].IsError || !results[2].IsError || !strings.Contains(results[2].Content, "skipped") {
		t.Errorf("unexpected serial results: %#v", results)
	}
}

func TestBatchExecutor_Parallel_ErrorReported(t *testing.T) {
	executor := ToolExecutorFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
		if call.Name == "bad" {
			return ToolResult{}, errors.New("bad tool")
		}
		return ToolResult{CallID: call.ID, Name: call.Name, Content: "ok"}, nil
	})

	be := NewBatchExecutor(executor, 5*time.Second, 0)
	calls := []ToolCall{
		{ID: "1", Name: "good"},
		{ID: "2", Name: "bad"},
	}

	results, err := be.Execute(context.Background(), calls, Parallel)
	if err == nil {
		t.Fatal("expected error from parallel batch")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[1].IsError {
		t.Error("expected result[1] to be error")
	}
}

func TestBatchExecutor_Parallel_BoundedConcurrency(t *testing.T) {
	var running int32
	var maxSeen int32

	executor := ToolExecutorFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if cur <= old || atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return ToolResult{CallID: call.ID, Name: call.Name, Content: "ok"}, nil
	})

	be := NewBatchExecutor(executor, 5*time.Second, 2)
	calls := make([]ToolCall, 6)
	for i := range calls {
		calls[i] = ToolCall{ID: fmt.Sprintf("%d", i), Name: "work"}
	}

	_, err := be.Execute(context.Background(), calls, Parallel)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&maxSeen) > 2 {
		t.Errorf("max concurrency exceeded: %d (limit 2)", maxSeen)
	}
}

func TestBatchExecutor_ContextCancellation(t *testing.T) {
	executor := ToolExecutorFunc(func(ctx context.Context, call ToolCall) (ToolResult, error) {
		select {
		case <-time.After(5 * time.Second):
			return ToolResult{CallID: call.ID, Content: "done"}, nil
		case <-ctx.Done():
			return ToolResult{}, ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	be := NewBatchExecutor(executor, 10*time.Second, 0)
	_, err := be.Execute(ctx, []ToolCall{{ID: "1", Name: "slow"}}, Parallel)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestBatchExecutor_ParallelCancellationCorrelatesUnstartedCalls(t *testing.T) {
	executor := ToolExecutorFunc(func(ctx context.Context, call ToolCall) (ToolResult, error) {
		<-ctx.Done()
		return ToolResult{}, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	be := NewBatchExecutor(executor, time.Second, 1)
	calls := []ToolCall{
		{ID: "1", Name: "first"},
		{ID: "2", Name: "second"},
		{ID: "3", Name: "third"},
	}
	results, err := be.Execute(ctx, calls, Parallel)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if len(results) != len(calls) {
		t.Fatalf("expected %d correlated results, got %d", len(calls), len(results))
	}
	for i, result := range results {
		if result.CallID != calls[i].ID || result.Name != calls[i].Name || !result.IsError {
			t.Fatalf("result %d is not correlated to its cancelled call: %#v", i, result)
		}
	}
}

func TestBatchExecutor_UsesPerCallTimeout(t *testing.T) {
	var remaining []time.Duration
	executor := ToolExecutorFunc(func(ctx context.Context, call ToolCall) (ToolResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("tool %s did not receive a deadline", call.Name)
		}
		remaining = append(remaining, time.Until(deadline))
		return ToolResult{CallID: call.ID, Name: call.Name, Content: "ok"}, nil
	})

	be := NewBatchExecutor(executor, 5*time.Second, 0)
	_, err := be.Execute(context.Background(), []ToolCall{
		{ID: "1", Name: "slow", Timeout: 50 * time.Millisecond},
		{ID: "2", Name: "default"},
	}, Serial)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 observed deadlines, got %d", len(remaining))
	}
	if remaining[0] <= 0 || remaining[0] > 500*time.Millisecond {
		t.Fatalf("per-call timeout was not applied: %v", remaining[0])
	}
	if remaining[1] < 4*time.Second {
		t.Fatalf("default timeout was not preserved: %v", remaining[1])
	}
}

func TestBatchExecutor_EmptyCalls(t *testing.T) {
	be := NewBatchExecutor(echoExecutor(), 5*time.Second, 0)
	results, err := be.Execute(context.Background(), nil, Parallel)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty calls, got %v", results)
	}
}
