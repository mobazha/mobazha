package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/store"
	"github.com/mobazha/mobazha3.0/pkg/agent/stream"
	"github.com/mobazha/mobazha3.0/pkg/agent/telemetry"
)

// LLMClient abstracts the model inference call.
// Implementations bridge to OpenAI / Anthropic / Platform AI Gateway.
type LLMClient interface {
	ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (stream.Stream, error)
}

// Message is an agent conversation message sent to/from the LLM.
type Message struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []stream.ToolCall `json:"tool_calls,omitempty"`
}

// ToolDef describes a tool the LLM can invoke.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

// Config holds the orchestrator's tuning parameters.
type Config struct {
	MaxToolRounds  int           // max iterative tool→model rounds per turn (default 10)
	TurnTimeout    time.Duration // overall timeout for a single turn (default 120s)
	MaxHistoryMsgs int           // max messages loaded from thread history (default 50)
	LLMRetries     int           // retry count on transient LLM errors (default 2)
}

func defaultConfig() Config {
	return Config{
		MaxToolRounds:  10,
		TurnTimeout:    120 * time.Second,
		MaxHistoryMsgs: 50,
		LLMRetries:     2,
	}
}

// Orchestrator coordinates a single agent turn: user input → LLM →
// (optional tool calls → LLM)* → final output streamed back.
//
// Supports:
//   - Multi-turn memory (loads/saves thread message history)
//   - System prompt injection via PromptBuilder
//   - Tool registration (ToolDefs passed to ChatStream)
//   - Input/output guardrails
//   - LLM retry on transient errors
type Orchestrator struct {
	llm       LLMClient
	budget    *budget.Calculator
	batchExec *exec.BatchExecutor
	persist   store.Persistence
	mem       *store.RuntimeStore
	emitter   telemetry.Emitter
	cfg       Config

	systemPrompt     string
	tools            []ToolDef
	inputGuardrails  []InputGuardrail
	outputGuardrails []OutputGuardrail
}

// NewOrchestrator creates an orchestrator with required dependencies.
func NewOrchestrator(
	llm LLMClient,
	budgetCalc *budget.Calculator,
	batchExec *exec.BatchExecutor,
	persist store.Persistence,
	emitter telemetry.Emitter,
	cfg *Config,
) *Orchestrator {
	c := defaultConfig()
	if cfg != nil {
		if cfg.MaxToolRounds > 0 {
			c.MaxToolRounds = cfg.MaxToolRounds
		}
		if cfg.TurnTimeout > 0 {
			c.TurnTimeout = cfg.TurnTimeout
		}
		if cfg.MaxHistoryMsgs > 0 {
			c.MaxHistoryMsgs = cfg.MaxHistoryMsgs
		}
		if cfg.LLMRetries > 0 {
			c.LLMRetries = cfg.LLMRetries
		}
	}
	if emitter == nil {
		emitter = telemetry.NoopEmitter{}
	}
	return &Orchestrator{
		llm:       llm,
		budget:    budgetCalc,
		batchExec: batchExec,
		persist:   persist,
		mem:       store.NewRuntimeStore(),
		emitter:   emitter,
		cfg:       c,
	}
}

// SetSystemPrompt sets the system prompt for all turns.
func (o *Orchestrator) SetSystemPrompt(prompt string) {
	o.systemPrompt = prompt
}

// RegisterTools sets the tool definitions available for LLM invocation.
func (o *Orchestrator) RegisterTools(tools []ToolDef) {
	o.tools = tools
}

// HydrateThread seeds runtime memory from durable history when a thread is
// resumed after process restart or cache eviction. Existing in-memory history
// wins to avoid duplicating messages during active conversations.
func (o *Orchestrator) HydrateThread(tenantID, threadID string, messages []*store.Message) {
	if tenantID == "" || threadID == "" {
		return
	}
	if len(o.mem.GetMessages(tenantID, threadID)) > 0 {
		return
	}
	if _, ok := o.mem.GetThread(tenantID, threadID); !ok {
		now := time.Now()
		o.mem.UpdateThread(&store.Thread{
			ID:         threadID,
			TenantID:   tenantID,
			CreatedAt:  now,
			LastActive: now,
		})
	}
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		cp := *msg
		cp.TenantID = tenantID
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = time.Now()
		}
		o.mem.AppendMessage(tenantID, threadID, &cp)
	}
}

// ForgetThread removes the in-memory copy of a thread. Durable persistence is
// owned by the store adapter; callers should delete persistent rows separately.
func (o *Orchestrator) ForgetThread(tenantID, threadID string) {
	if o == nil || tenantID == "" || threadID == "" {
		return
	}
	o.mem.RemoveThread(tenantID, threadID)
}

// AddInputGuardrail adds an input validation guardrail.
func (o *Orchestrator) AddInputGuardrail(g InputGuardrail) {
	o.inputGuardrails = append(o.inputGuardrails, g)
}

// AddOutputGuardrail adds an output validation guardrail.
func (o *Orchestrator) AddOutputGuardrail(g OutputGuardrail) {
	o.outputGuardrails = append(o.outputGuardrails, g)
}

// TurnResult holds the outcome of RunTurn.
type TurnResult struct {
	Output stream.Stream
	TurnID string
}

// RunTurn executes a single conversational turn:
//  1. Validate input via guardrails
//  2. Load or create thread, load message history
//  3. Assemble messages: system prompt + history + user message
//  4. Loop: send to LLM → if tool_calls, execute tools, append results, repeat
//  5. Validate output via guardrails
//  6. Save messages to runtime memory and durable persistence
//  7. Stream final assistant output
func (o *Orchestrator) RunTurn(ctx context.Context, tenantID, threadID string, userMsg string) (*TurnResult, error) {
	if len(o.inputGuardrails) > 0 {
		result := RunInputGuardrails(ctx, o.inputGuardrails, tenantID, threadID, userMsg)
		if !result.Passed {
			return nil, fmt.Errorf("input guardrail blocked: %s", result.Reason)
		}
		if result.Rewrite != "" {
			userMsg = result.Rewrite
		}
	}

	turnCtx, cancel := context.WithTimeout(ctx, o.cfg.TurnTimeout)

	turnID := newTurnID()
	turnStartedAt := time.Now()

	o.emitter.Emit(ctx, telemetry.Event{
		Type:     telemetry.TurnStarted,
		TenantID: tenantID,
		ThreadID: threadID,
		Attrs:    map[string]any{"turn_id": turnID},
	})

	if _, err := o.ensureThread(ctx, tenantID, threadID); err != nil {
		return nil, err
	}
	if err := o.saveTurn(ctx, &store.Turn{
		ID:        turnID,
		TenantID:  tenantID,
		ThreadID:  threadID,
		StartedAt: turnStartedAt,
		Completed: false,
	}); err != nil {
		return nil, err
	}

	history := o.assembleHistory(tenantID, threadID, userMsg)

	if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
		ID:        newMessageID(),
		TenantID:  tenantID,
		ThreadID:  threadID,
		TurnID:    turnID,
		Role:      "user",
		Content:   userMsg,
		Tokens:    budget.EstimateTokens(userMsg),
		Bytes:     len(userMsg),
		CreatedAt: time.Now(),
	}); err != nil {
		return nil, err
	}

	outStream := stream.NewBuffered(ctx, 32)

	go func() {
		defer cancel()
		defer outStream.Finish()
		o.runLoop(turnCtx, tenantID, threadID, turnID, turnStartedAt, history, outStream)
	}()

	return &TurnResult{Output: outStream, TurnID: turnID}, nil
}

// assembleHistory builds the full message list for the LLM call:
// [system prompt] + [prior messages from memory] + [current user message].
func (o *Orchestrator) assembleHistory(tenantID, threadID, userMsg string) []Message {
	var msgs []Message

	if o.systemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: o.systemPrompt})
	}

	priorMessages := o.mem.GetMessages(tenantID, threadID)
	if len(priorMessages) > o.cfg.MaxHistoryMsgs {
		priorMessages = priorMessages[len(priorMessages)-o.cfg.MaxHistoryMsgs:]
	}
	for _, m := range priorMessages {
		msg := Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		if m.ToolCalls != "" {
			_ = json.Unmarshal([]byte(m.ToolCalls), &msg.ToolCalls)
		}
		msgs = append(msgs, msg)
	}

	msgs = append(msgs, Message{Role: "user", Content: userMsg})
	return msgs
}

func (o *Orchestrator) runLoop(
	ctx context.Context,
	tenantID, threadID, turnID string,
	turnStartedAt time.Time,
	history []Message,
	out *stream.Buffered,
) {
	for round := 0; round < o.cfg.MaxToolRounds; round++ {
		tokens := 0
		for _, m := range history {
			tokens += budget.EstimateTokens(m.Content)
		}
		decision := o.budget.Decide(tokens)

		if decision.Overflow {
			o.emitter.Emit(ctx, telemetry.Event{
				Type:     telemetry.OverflowDetected,
				TenantID: tenantID,
				ThreadID: threadID,
				Attrs: map[string]any{
					"estimated": decision.Estimated,
					"available": decision.Available,
				},
			})
			out.SendError(fmt.Errorf("context overflow: estimated %d tokens, 0 available", decision.Estimated))
			return
		}

		llmStream, err := o.callLLMWithRetry(ctx, history)
		if err != nil {
			out.SendError(fmt.Errorf("LLM call failed: %w", err))
			return
		}

		chunks, toolCalls, assistantText, streamErr := o.drainLLMStream(llmStream, out)
		_ = chunks

		if streamErr != nil {
			out.SendError(fmt.Errorf("LLM stream error: %w", streamErr))
			return
		}

		if len(toolCalls) == 0 {
			if err := o.saveAssistantMessage(ctx, tenantID, threadID, turnID, assistantText); err != nil {
				out.SendError(err)
				return
			}

			// Output guardrails run post-stream as audit/telemetry only.
			// In streaming mode, content is already delivered to the consumer —
			// blocking is not possible without buffering the full response first.
			// Future: add a buffered mode for high-trust scenarios where output
			// must be validated before delivery (at the cost of TTFB latency).
			if len(o.outputGuardrails) > 0 {
				result := RunOutputGuardrails(ctx, o.outputGuardrails, tenantID, threadID, assistantText)
				if !result.Passed {
					o.emitter.Emit(ctx, telemetry.Event{
						Type:     telemetry.GuardrailBlocked,
						TenantID: tenantID,
						ThreadID: threadID,
						Attrs:    map[string]any{"stage": "output", "reason": result.Reason},
					})
				}
			}

			o.emitter.Emit(ctx, telemetry.Event{
				Type:     telemetry.TurnCompleted,
				TenantID: tenantID,
				ThreadID: threadID,
				Attrs: map[string]any{
					"turn_id": turnID,
					"rounds":  round + 1,
				},
			})
			completedAt := time.Now()
			if err := o.saveTurn(ctx, &store.Turn{
				ID:          turnID,
				TenantID:    tenantID,
				ThreadID:    threadID,
				StartedAt:   turnStartedAt,
				CompletedAt: &completedAt,
				Completed:   true,
			}); err != nil {
				out.SendError(err)
			}
			return
		}

		history = append(history, Message{Role: "assistant", Content: assistantText, ToolCalls: toolCalls})

		toolCallsJSON, _ := json.Marshal(toolCalls)
		if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
			ID:        newMessageID(),
			TenantID:  tenantID,
			ThreadID:  threadID,
			TurnID:    turnID,
			Role:      "assistant",
			Content:   assistantText,
			ToolCalls: string(toolCallsJSON),
			Tokens:    budget.EstimateTokens(assistantText),
			Bytes:     len(assistantText),
			CreatedAt: time.Now(),
		}); err != nil {
			out.SendError(err)
			return
		}

		execCalls := make([]exec.ToolCall, len(toolCalls))
		toolNames := make(map[string]string, len(toolCalls))
		for i, tc := range toolCalls {
			execCalls[i] = exec.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
			toolNames[tc.ID] = tc.Name
			out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
				ID:     tc.ID,
				Name:   tc.Name,
				Status: "executing",
			}})
		}

		start := time.Now()
		results, execErr := o.batchExec.Execute(ctx, execCalls, exec.Parallel)
		duration := time.Since(start)

		errCount := 0
		for _, r := range results {
			if r.IsError {
				errCount++
			}
		}

		o.emitter.Emit(ctx, telemetry.Event{
			Type:     telemetry.ToolCallBatch,
			TenantID: tenantID,
			ThreadID: threadID,
			Attrs: map[string]any{
				"mode":        "parallel",
				"count":       len(toolCalls),
				"duration_ms": duration.Milliseconds(),
				"error_count": errCount,
			},
		})

		for _, r := range results {
			status := "done"
			if r.IsError {
				status = "error"
			}
			toolName := r.Name
			if toolName == "" {
				toolName = toolNames[r.CallID]
			}
			out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
				ID:     r.CallID,
				Name:   toolName,
				Status: status,
			}})
			history = append(history, Message{
				Role:       "tool",
				Content:    r.Content,
				ToolCallID: r.CallID,
			})
			if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
				ID:         newMessageID(),
				TenantID:   tenantID,
				ThreadID:   threadID,
				TurnID:     turnID,
				Role:       "tool",
				Content:    r.Content,
				ToolCallID: r.CallID,
				Tokens:     budget.EstimateTokens(r.Content),
				Bytes:      len(r.Content),
				CreatedAt:  time.Now(),
			}); err != nil {
				out.SendError(err)
				return
			}
		}

		if execErr != nil && errCount == len(results) {
			out.SendError(fmt.Errorf("all tool calls failed: %w", execErr))
			return
		}
	}

	out.SendError(fmt.Errorf("exceeded max tool rounds (%d)", o.cfg.MaxToolRounds))
}

// callLLMWithRetry wraps the LLM call with simple retry logic for transient errors.
func (o *Orchestrator) callLLMWithRetry(ctx context.Context, history []Message) (stream.Stream, error) {
	var lastErr error
	for attempt := 0; attempt <= o.cfg.LLMRetries; attempt++ {
		if attempt > 0 {
			o.emitter.Emit(ctx, telemetry.Event{
				Type:  telemetry.LLMRetried,
				Attrs: map[string]any{"attempt": attempt, "error": lastErr.Error()},
			})
			backoff := time.Duration(attempt*500) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		s, err := o.llm.ChatStream(ctx, history, o.tools)
		if err == nil {
			return s, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (o *Orchestrator) saveTurn(ctx context.Context, turn *store.Turn) error {
	if o.persist == nil {
		return nil
	}
	if err := o.persist.SaveTurn(ctx, turn); err != nil {
		return fmt.Errorf("agent runtime: save turn: %w", err)
	}
	return nil
}

func (o *Orchestrator) saveMessage(ctx context.Context, tenantID, threadID string, msg *store.Message) error {
	if msg == nil {
		return nil
	}
	if o.persist != nil {
		if err := o.persist.SaveMessage(ctx, msg); err != nil {
			return fmt.Errorf("agent runtime: save message: %w", err)
		}
	}
	o.mem.AppendMessage(tenantID, threadID, msg)
	return nil
}

func newTurnID() string {
	return "turn_" + uuid.NewString()
}

func newMessageID() string {
	return "msg_" + uuid.NewString()
}

// saveAssistantMessage persists the assistant's response to runtime memory and durable store.
func (o *Orchestrator) saveAssistantMessage(ctx context.Context, tenantID, threadID, turnID, text string) error {
	return o.saveMessage(ctx, tenantID, threadID, &store.Message{
		ID:        newMessageID(),
		TenantID:  tenantID,
		ThreadID:  threadID,
		TurnID:    turnID,
		Role:      "assistant",
		Content:   text,
		Tokens:    budget.EstimateTokens(text),
		Bytes:     len(text),
		CreatedAt: time.Now(),
	})
}

// drainLLMStream reads all chunks from the LLM stream, forwarding text
// deltas to the output stream and collecting tool calls.
// Returns any error from the LLM stream (e.g. SSE disconnect mid-response).
func (o *Orchestrator) drainLLMStream(
	llmStream stream.Stream,
	out *stream.Buffered,
) ([]stream.Chunk, []stream.ToolCall, string, error) {
	var (
		chunks    []stream.Chunk
		toolCalls []stream.ToolCall
		text      string
	)

	for {
		c := llmStream.Next()
		if c == nil {
			break
		}
		chunks = append(chunks, *c)

		if c.Delta != "" {
			text += c.Delta
			out.Send(stream.Chunk{Delta: c.Delta})
		}
		if len(c.ToolCalls) > 0 {
			toolCalls = append(toolCalls, c.ToolCalls...)
		}
	}

	return chunks, toolCalls, text, llmStream.Err()
}

func (o *Orchestrator) ensureThread(ctx context.Context, tenantID, threadID string) (*store.Thread, error) {
	if t, ok := o.mem.GetThread(tenantID, threadID); ok {
		o.mem.TouchThread(tenantID, threadID)
		t.LastActive = time.Now()
		if o.persist != nil {
			if err := o.persist.SaveThread(ctx, t); err != nil {
				return nil, fmt.Errorf("agent runtime: touch thread: %w", err)
			}
		}
		return t, nil
	}

	if o.persist != nil {
		t, err := o.persist.LoadThread(ctx, tenantID, threadID)
		if err != nil && !errors.Is(err, store.ErrThreadNotFound) {
			return nil, fmt.Errorf("agent runtime: load thread: %w", err)
		}
		if t != nil {
			t.LastActive = time.Now()
			o.mem.UpdateThread(t)
			messages, err := o.persist.LoadMessages(ctx, tenantID, threadID)
			if err != nil {
				return nil, fmt.Errorf("agent runtime: load messages: %w", err)
			}
			o.HydrateThread(tenantID, threadID, messages)
			if err := o.persist.SaveThread(ctx, t); err != nil {
				return nil, fmt.Errorf("agent runtime: update thread: %w", err)
			}
			return t, nil
		}
	}

	t := &store.Thread{
		ID:         threadID,
		TenantID:   tenantID,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}
	o.mem.UpdateThread(t)
	if o.persist != nil {
		if err := o.persist.SaveThread(ctx, t); err != nil {
			return nil, fmt.Errorf("agent runtime: create thread: %w", err)
		}
	}
	return t, nil
}
