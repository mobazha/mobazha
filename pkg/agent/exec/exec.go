package exec

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/stream"
)

// ToolCall is a type alias to the canonical definition in stream package.
type ToolCall = stream.ToolCall

// ToolResult holds the output of a single tool execution.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// Mode controls how tool calls in a batch are dispatched.
type Mode int

const (
	Parallel Mode = iota
	Serial
)

// ToolExecutor is the interface for executing a single tool call.
// Implementations bridge to MCP, HTTP, or test mocks.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

// BatchExecutor dispatches a batch of tool calls with concurrency control.
type BatchExecutor struct {
	executor ToolExecutor
	timeout  time.Duration
	maxPar   int // max parallel goroutines (0 = unlimited)
}

// NewBatchExecutor creates a batch executor with the given settings.
func NewBatchExecutor(executor ToolExecutor, timeout time.Duration, maxParallel int) *BatchExecutor {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &BatchExecutor{
		executor: executor,
		timeout:  timeout,
		maxPar:   maxParallel,
	}
}

// Execute dispatches all tool calls and returns results in the same order.
// In Parallel mode, calls run concurrently (bounded by maxPar).
// In Serial mode, calls run one at a time, stopping on first error.
func (b *BatchExecutor) Execute(ctx context.Context, calls []ToolCall, mode Mode) ([]ToolResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	if mode == Serial {
		return b.executeSerial(ctx, calls)
	}
	return b.executeParallel(ctx, calls)
}

func (b *BatchExecutor) executeSerial(ctx context.Context, calls []ToolCall) ([]ToolResult, error) {
	results := make([]ToolResult, len(calls))
	for i, call := range calls {
		result, err := b.executor.Execute(ctx, call)
		if err != nil {
			results[i] = ToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: fmt.Sprintf("tool execution error: %v", err),
				IsError: true,
			}
			return results[:i+1], err
		}
		results[i] = result
	}
	return results, nil
}

func (b *BatchExecutor) executeParallel(ctx context.Context, calls []ToolCall) ([]ToolResult, error) {
	results := make([]ToolResult, len(calls))
	var (
		wg      sync.WaitGroup
		errOnce sync.Once
		firstErr error
		sem     chan struct{}
	)

	if b.maxPar > 0 {
		sem = make(chan struct{}, b.maxPar)
	}

	for i, call := range calls {
		wg.Add(1)
		if sem != nil {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				wg.Done()
				errOnce.Do(func() { firstErr = ctx.Err() })
				continue
			}
		}
		go func(idx int, c ToolCall) {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}

			result, err := b.executor.Execute(ctx, c)
			if err != nil {
				results[idx] = ToolResult{
					CallID:  c.ID,
					Name:    c.Name,
					Content: fmt.Sprintf("tool execution error: %v", err),
					IsError: true,
				}
				errOnce.Do(func() { firstErr = err })
				return
			}
			results[idx] = result
		}(i, call)
	}
	wg.Wait()
	return results, firstErr
}

// ToolExecutorFunc is a convenience adapter for single-function executors.
type ToolExecutorFunc func(ctx context.Context, call ToolCall) (ToolResult, error)

func (f ToolExecutorFunc) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	return f(ctx, call)
}
