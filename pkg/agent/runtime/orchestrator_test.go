package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/store"
	"github.com/mobazha/mobazha3.0/pkg/agent/stream"
	"github.com/mobazha/mobazha3.0/pkg/agent/telemetry"
)

// --- mock LLMClient ---

type mockLLM struct {
	responses []mockLLMResponse
	callIndex int
	captured  []capturedCall
}

type capturedCall struct {
	messages []Message
	tools    []ToolDef
}

type mockLLMResponse struct {
	chunks []stream.Chunk
	err    error
}

type fakePersistence struct {
	thread         *store.Thread
	messages       []*store.Message
	saveMessageErr error
	saveTurnErr    error
	turnSaveCount  int
}

func (p *fakePersistence) SaveThread(_ context.Context, t *store.Thread) error {
	if t == nil {
		return nil
	}
	cp := *t
	p.thread = &cp
	return nil
}

func (p *fakePersistence) SaveTurn(context.Context, *store.Turn) error {
	p.turnSaveCount++
	if p.saveTurnErr != nil && p.turnSaveCount > 1 {
		return p.saveTurnErr
	}
	return nil
}

func (p *fakePersistence) SaveMessage(_ context.Context, m *store.Message) error {
	if p.saveMessageErr != nil {
		return p.saveMessageErr
	}
	cp := *m
	p.messages = append(p.messages, &cp)
	return nil
}

func (p *fakePersistence) LoadThread(_ context.Context, _, threadID string) (*store.Thread, error) {
	if p.thread == nil || p.thread.ID != threadID {
		return nil, store.ErrThreadNotFound
	}
	cp := *p.thread
	return &cp, nil
}

func (p *fakePersistence) ListThreads(context.Context, string, int, int) ([]*store.Thread, error) {
	if p.thread == nil {
		return nil, nil
	}
	cp := *p.thread
	return []*store.Thread{&cp}, nil
}

func (p *fakePersistence) LoadMessages(_ context.Context, _, threadID string) ([]*store.Message, error) {
	out := make([]*store.Message, 0, len(p.messages))
	for _, msg := range p.messages {
		if msg.ThreadID != threadID {
			continue
		}
		cp := *msg
		out = append(out, &cp)
	}
	return out, nil
}

func (p *fakePersistence) DeleteThread(context.Context, string, string) error {
	p.thread = nil
	p.messages = nil
	return nil
}

func (m *mockLLM) ChatStream(_ context.Context, msgs []Message, tools []ToolDef) (stream.Stream, error) {
	if m.callIndex >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	m.captured = append(m.captured, capturedCall{messages: msgs, tools: tools})
	resp := m.responses[m.callIndex]
	m.callIndex++

	if resp.err != nil {
		return nil, resp.err
	}

	buf := stream.NewBuffered(context.Background(), 16)
	go func() {
		for _, c := range resp.chunks {
			buf.Send(c)
		}
		buf.Finish()
	}()
	return buf, nil
}

func newTestOrch(llm *mockLLM, emitter telemetry.Emitter) *Orchestrator {
	if emitter == nil {
		emitter = &telemetry.BufferEmitter{}
	}
	return NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{CallID: c.ID, Content: "ok"}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)
}

// --- tests ---

func TestRunTurn_SimpleTextResponse(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{
				{Delta: "Hello, "},
				{Delta: "world!"},
			}},
		},
	}

	orch := newTestOrch(llm, nil)
	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_1", "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}

	var combined string
	for _, c := range chunks {
		combined += c.Delta
	}
	if combined != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", combined)
	}
}

func TestHydrateThread_SeedsHistoryForNextTurn(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "next"}}},
		},
	}

	orch := newTestOrch(llm, nil)
	orch.HydrateThread("tenant_1", "th_1", []*store.Message{
		{Role: "user", Content: "Previous question"},
		{Role: "assistant", Content: "Previous answer"},
	})

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_1", "Current question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if len(llm.captured) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(llm.captured))
	}
	got := llm.captured[0].messages
	if len(got) != 3 {
		t.Fatalf("expected hydrated history plus current message, got %#v", got)
	}
	if got[0].Content != "Previous question" || got[1].Content != "Previous answer" || got[2].Content != "Current question" {
		t.Fatalf("unexpected message order: %#v", got)
	}
}

func TestForgetThread_RemovesHydratedHistory(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "fresh"}}},
		},
	}
	orch := newTestOrch(llm, nil)
	orch.HydrateThread("tenant_1", "th_deleted", []*store.Message{
		{Role: "user", Content: "Deleted question"},
		{Role: "assistant", Content: "Deleted answer"},
	})

	orch.ForgetThread("tenant_1", "th_deleted")
	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_deleted", "Fresh question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if len(llm.captured) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(llm.captured))
	}
	got := llm.captured[0].messages
	if len(got) != 1 {
		t.Fatalf("expected only current message after forget, got %#v", got)
	}
	if got[0].Content != "Fresh question" {
		t.Fatalf("unexpected message after forget: %#v", got)
	}
}

func TestRunTurn_WithToolCalls(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{
				{Delta: "Let me search...", ToolCalls: []stream.ToolCall{
					{ID: "tc_1", Name: "search", Arguments: `{"q":"trending"}`},
				}},
			}},
			{chunks: []stream.Chunk{
				{Delta: "Based on the search: trending items are X."},
			}},
		},
	}

	toolExecuted := false
	executor := exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
		toolExecuted = true
		return exec.ToolResult{
			CallID:  c.ID,
			Name:    c.Name,
			Content: `{"items":["X","Y"]}`,
		}, nil
	})

	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(executor, 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_2", "What's trending?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}

	if !toolExecuted {
		t.Error("expected tool to be executed")
	}

	var combined string
	for _, c := range chunks {
		combined += c.Delta
	}
	if combined == "" {
		t.Error("expected non-empty output")
	}

	batchEvents := emitter.ByType(telemetry.ToolCallBatch)
	if len(batchEvents) == 0 {
		t.Error("expected tool_call_batch telemetry event")
	}

	turnComplete := emitter.ByType(telemetry.TurnCompleted)
	if len(turnComplete) == 0 {
		t.Error("expected turn_completed telemetry event")
	}
}

func TestRunTurn_EmitsRedactedToolProgress(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{
				{ToolCalls: []stream.ToolCall{
					{ID: "tc_1", Name: "search", Arguments: `{"secret":"hidden"}`},
				}},
			}},
			{chunks: []stream.Chunk{{Delta: "done"}}},
		},
	}
	orch := newTestOrch(llm, nil)

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_progress", "Search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}

	var sawExecuting, sawDone bool
	for _, chunk := range chunks {
		if chunk.ToolEvent == nil {
			continue
		}
		if chunk.ToolEvent.ID != "tc_1" || chunk.ToolEvent.Name != "search" {
			t.Fatalf("unexpected tool event: %#v", chunk.ToolEvent)
		}
		if strings.Contains(fmt.Sprintf("%#v", chunk.ToolEvent), "hidden") {
			t.Fatal("tool event should not expose arguments")
		}
		sawExecuting = sawExecuting || chunk.ToolEvent.Status == "executing"
		sawDone = sawDone || chunk.ToolEvent.Status == "done"
	}
	if !sawExecuting || !sawDone {
		t.Fatalf("expected executing and done tool events, got %#v", chunks)
	}
}

func TestRunTurn_LLMError(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{err: fmt.Errorf("API key expired")},
			{err: fmt.Errorf("API key expired")},
			{err: fmt.Errorf("API key expired")},
		},
	}

	orch := newTestOrch(llm, telemetry.NoopEmitter{})

	result, err := orch.RunTurn(context.Background(), "t1", "th_3", "Hello")
	if err != nil {
		t.Fatalf("RunTurn itself should not error, got: %v", err)
	}

	_, streamErr := stream.Collect(result.Output)
	if streamErr == nil {
		t.Fatal("expected stream error from LLM failure")
	}
}

func TestRunTurn_OverflowDetected(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "ok"}}},
		},
	}

	tinyBudget := budget.NewCalculator(budget.Config{
		MaxContextTokens: 10,
		ReservedOutput:   5,
		CompactThreshold: 0.75,
		ShapeThreshold:   0.60,
	})

	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		tinyBudget,
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	longMsg := ""
	for i := 0; i < 200; i++ {
		longMsg += "word "
	}

	result, err := orch.RunTurn(context.Background(), "t1", "th_4", longMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, streamErr := stream.Collect(result.Output)
	if streamErr == nil {
		t.Fatal("expected overflow error")
	}

	overflows := emitter.ByType(telemetry.OverflowDetected)
	if len(overflows) == 0 {
		t.Error("expected overflow_detected telemetry event")
	}
}

func TestRunTurn_MaxToolRoundsExceeded(t *testing.T) {
	alwaysToolCall := mockLLMResponse{
		chunks: []stream.Chunk{
			{ToolCalls: []stream.ToolCall{{ID: "tc", Name: "loop", Arguments: "{}"}}},
		},
	}
	responses := make([]mockLLMResponse, 15)
	for i := range responses {
		responses[i] = alwaysToolCall
	}
	llm := &mockLLM{responses: responses}

	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{CallID: c.ID, Content: "ok"}, nil
		}), 5*time.Second, 0),
		nil,
		telemetry.NoopEmitter{},
		&Config{MaxToolRounds: 3, TurnTimeout: 10 * time.Second},
	)

	result, err := orch.RunTurn(context.Background(), "t1", "th_5", "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, streamErr := stream.Collect(result.Output)
	if streamErr == nil {
		t.Fatal("expected max rounds error")
	}
}

// --- New tests for P0 features ---

func TestRunTurn_MultiTurnMemory(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "I'm Alice!"}}},
			{chunks: []stream.Chunk{{Delta: "You said you're Bob."}}},
		},
	}

	orch := newTestOrch(llm, nil)

	r1, err := orch.RunTurn(context.Background(), "t1", "th_mem", "My name is Bob")
	if err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	stream.Collect(r1.Output)

	r2, err := orch.RunTurn(context.Background(), "t1", "th_mem", "What's my name?")
	if err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	stream.Collect(r2.Output)

	if len(llm.captured) < 2 {
		t.Fatal("expected at least 2 LLM calls")
	}
	turn2Msgs := llm.captured[1].messages
	if len(turn2Msgs) < 3 {
		t.Fatalf("expected >= 3 messages (history + new user), got %d", len(turn2Msgs))
	}

	foundUserBob := false
	foundAssistantAlice := false
	for _, m := range turn2Msgs {
		if m.Role == "user" && strings.Contains(m.Content, "Bob") {
			foundUserBob = true
		}
		if m.Role == "assistant" && strings.Contains(m.Content, "Alice") {
			foundAssistantAlice = true
		}
	}
	if !foundUserBob {
		t.Error("turn 2 should include prior user message about Bob")
	}
	if !foundAssistantAlice {
		t.Error("turn 2 should include prior assistant response")
	}
}

func TestRunTurn_PersistenceFailureDoesNotPolluteMemory(t *testing.T) {
	persist := &fakePersistence{saveMessageErr: errors.New("disk full")}
	llm := &mockLLM{
		responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "unused"}}}},
	}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		telemetry.NoopEmitter{},
		nil,
	)

	_, err := orch.RunTurn(context.Background(), "tenant_1", "th_fail", "hello")
	if err == nil {
		t.Fatal("expected RunTurn to fail when persistence rejects the user message")
	}
	if got := orch.mem.GetMessages("tenant_1", "th_fail"); len(got) != 0 {
		t.Fatalf("memory should not contain unpersisted messages, got %#v", got)
	}
}

func TestRunTurn_CompletedTurnPersistenceFailureReturnsStreamError(t *testing.T) {
	persist := &fakePersistence{saveTurnErr: errors.New("turn update failed")}
	llm := &mockLLM{
		responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "answer"}}}},
	}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		telemetry.NoopEmitter{},
		nil,
	)

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_turn_fail", "hello")
	if err != nil {
		t.Fatalf("RunTurn should start successfully, got %v", err)
	}
	_, streamErr := stream.Collect(result.Output)
	if streamErr == nil || !strings.Contains(streamErr.Error(), "turn update failed") {
		t.Fatalf("expected completed turn persistence error, got %v", streamErr)
	}
}

func TestRunTurn_LoadsHistoryFromPersistenceAfterRestart(t *testing.T) {
	persist := &fakePersistence{}
	llm1 := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "first answer"}}}}}
	orch1 := NewOrchestrator(
		llm1,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		telemetry.NoopEmitter{},
		nil,
	)
	r1, err := orch1.RunTurn(context.Background(), "tenant_1", "th_restart", "first question")
	if err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	if _, err := stream.Collect(r1.Output); err != nil {
		t.Fatalf("turn 1 stream error: %v", err)
	}

	llm2 := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "second answer"}}}}}
	orch2 := NewOrchestrator(
		llm2,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		telemetry.NoopEmitter{},
		nil,
	)
	r2, err := orch2.RunTurn(context.Background(), "tenant_1", "th_restart", "second question")
	if err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	if _, err := stream.Collect(r2.Output); err != nil {
		t.Fatalf("turn 2 stream error: %v", err)
	}

	if len(llm2.captured) != 1 {
		t.Fatalf("expected one LLM call after restart, got %d", len(llm2.captured))
	}
	got := llm2.captured[0].messages
	if len(got) < 3 {
		t.Fatalf("expected persisted history plus current message, got %#v", got)
	}
	if got[0].Content != "first question" || got[1].Content != "first answer" || got[len(got)-1].Content != "second question" {
		t.Fatalf("unexpected restored history: %#v", got)
	}
}

func TestRunTurn_SystemPrompt(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "Hola!"}}},
		},
	}

	orch := newTestOrch(llm, nil)
	orch.SetSystemPrompt("You are a Spanish-speaking assistant.")

	r, err := orch.RunTurn(context.Background(), "t1", "th_sys", "Hi")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	stream.Collect(r.Output)

	if len(llm.captured) == 0 {
		t.Fatal("expected LLM call")
	}

	msgs := llm.captured[0].messages
	if msgs[0].Role != "system" {
		t.Errorf("expected first message to be system, got %q", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "Spanish") {
		t.Errorf("system prompt should contain 'Spanish', got %q", msgs[0].Content)
	}
}

func TestRunTurn_ToolRegistration(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "ok"}}},
		},
	}

	orch := newTestOrch(llm, nil)
	orch.RegisterTools([]ToolDef{
		{Name: "search_listings", Description: "Search for listings", Schema: `{"type":"object","properties":{"q":{"type":"string"}}}`},
		{Name: "get_order", Description: "Get order details", Schema: `{"type":"object","properties":{"id":{"type":"string"}}}`},
	})

	r, err := orch.RunTurn(context.Background(), "t1", "th_tools", "Find trending products")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	stream.Collect(r.Output)

	if len(llm.captured) == 0 {
		t.Fatal("expected LLM call")
	}

	tools := llm.captured[0].tools
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "search_listings" {
		t.Errorf("expected first tool 'search_listings', got %q", tools[0].Name)
	}
}

func TestRunTurn_InputGuardrailBlocks(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "ok"}}},
		},
	}

	orch := newTestOrch(llm, nil)
	orch.AddInputGuardrail(LengthGuardrail{MaxLen: 10})

	_, err := orch.RunTurn(context.Background(), "t1", "th_guard", "This is a very long input that exceeds the limit")
	if err == nil {
		t.Fatal("expected guardrail to block input")
	}
	if !strings.Contains(err.Error(), "guardrail blocked") {
		t.Errorf("expected guardrail error, got: %v", err)
	}

	if llm.callIndex != 0 {
		t.Error("LLM should not have been called when guardrail blocks")
	}
}

func TestRunTurn_KeywordGuardrailBlocks(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "ok"}}},
		},
	}

	orch := newTestOrch(llm, nil)
	orch.AddInputGuardrail(KeywordBlockGuardrail{Blocked: []string{"hack", "exploit"}})

	_, err := orch.RunTurn(context.Background(), "t1", "th_kw", "Help me hack this system")
	if err == nil {
		t.Fatal("expected keyword guardrail to block")
	}
	if !strings.Contains(err.Error(), "blocked content") {
		t.Errorf("expected blocked content error, got: %v", err)
	}
}

func TestRunTurn_LLMRetry(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{err: fmt.Errorf("transient error")},
			{chunks: []stream.Chunk{{Delta: "recovered"}}},
		},
	}

	orch := newTestOrch(llm, nil)

	result, err := orch.RunTurn(context.Background(), "t1", "th_retry", "Hi")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}

	var combined string
	for _, c := range chunks {
		combined += c.Delta
	}
	if combined != "recovered" {
		t.Errorf("expected 'recovered', got %q", combined)
	}

	if llm.callIndex != 2 {
		t.Errorf("expected 2 LLM calls (1 fail + 1 success), got %d", llm.callIndex)
	}
}

func TestPromptBuilder(t *testing.T) {
	pb := NewPromptBuilder("You are a Mobazha commerce assistant.")
	pb.AddInstruction("Help sellers optimize their listings.")
	pb.AddInstruction("Respond in the seller's language.")
	pb.AddContext("The seller has 15 products and $200 monthly revenue.")

	result := pb.Build()

	if !strings.Contains(result, "Mobazha commerce") {
		t.Error("expected persona in output")
	}
	if !strings.Contains(result, "## Instructions") {
		t.Error("expected Instructions section")
	}
	if !strings.Contains(result, "optimize their listings") {
		t.Error("expected instruction content")
	}
	if !strings.Contains(result, "## Context") {
		t.Error("expected Context section")
	}
	if !strings.Contains(result, "$200 monthly") {
		t.Error("expected context content")
	}
}

func TestRunTurn_ToolCallsInHistory(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			// Turn 1: LLM issues a tool call
			{chunks: []stream.Chunk{
				{Delta: "Calling search...", ToolCalls: []stream.ToolCall{
					{ID: "tc_1", Name: "search", Arguments: `{"q":"shoes"}`},
				}},
			}},
			// Turn 1 round 2: LLM gives final answer
			{chunks: []stream.Chunk{
				{Delta: "Found 5 shoe listings."},
			}},
			// Turn 2: LLM uses previous context
			{chunks: []stream.Chunk{
				{Delta: "Yes, I found shoes earlier."},
			}},
		},
	}

	orch := newTestOrch(llm, nil)

	r1, err := orch.RunTurn(context.Background(), "t1", "th_tc", "Find shoes")
	if err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	stream.Collect(r1.Output)

	r2, err := orch.RunTurn(context.Background(), "t1", "th_tc", "Did you find any?")
	if err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	stream.Collect(r2.Output)

	if len(llm.captured) < 3 {
		t.Fatalf("expected at least 3 LLM calls, got %d", len(llm.captured))
	}

	// The 3rd call (turn 2) should contain the prior assistant message with tool_calls
	turn2Msgs := llm.captured[2].messages
	foundToolCalls := false
	foundToolResult := false
	for _, m := range turn2Msgs {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			foundToolCalls = true
			if m.ToolCalls[0].Name != "search" {
				t.Errorf("expected tool call name 'search', got %q", m.ToolCalls[0].Name)
			}
		}
		if m.Role == "tool" && m.ToolCallID == "tc_1" {
			foundToolResult = true
		}
	}
	if !foundToolCalls {
		t.Error("turn 2 history should include prior assistant message with tool_calls")
	}
	if !foundToolResult {
		t.Error("turn 2 history should include prior tool result message")
	}
}

func TestGuardrailChain(t *testing.T) {
	guards := []InputGuardrail{
		LengthGuardrail{MaxLen: 1000},
		KeywordBlockGuardrail{Blocked: []string{"drop table"}},
	}

	r := RunInputGuardrails(context.Background(), guards, "t1", "th1", "normal input")
	if !r.Passed {
		t.Errorf("expected pass, got blocked: %s", r.Reason)
	}

	r = RunInputGuardrails(context.Background(), guards, "t1", "th1", "please DROP TABLE users")
	if r.Passed {
		t.Error("expected block for SQL injection attempt")
	}
}
