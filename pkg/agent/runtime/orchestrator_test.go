package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
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
	turns          []*store.Turn
	messages       []*store.Message
	skillRuns      []*store.SkillRun
	artifacts      []*store.Artifact
	approvals      []*store.Approval
	saveMessageErr error
	saveTurnErr    error
	turnSaveCount  int
}

type fakeMemoryStore struct {
	items       []kernel.MemoryItem
	err         error
	saveErr     error
	deleteErr   error
	queries     []kernel.MemoryQuery
	savedScopes []kernel.Scope
	savedItems  []kernel.MemoryItem
	deletedIDs  []string
	missQueries bool
}

func (s *fakeMemoryStore) Search(_ context.Context, q kernel.MemoryQuery) ([]kernel.MemoryItem, error) {
	s.queries = append(s.queries, q)
	if s.err != nil {
		return nil, s.err
	}
	if s.missQueries && q.Query != "" {
		return nil, nil
	}
	return s.items, nil
}

func (s *fakeMemoryStore) Save(_ context.Context, scope kernel.Scope, item kernel.MemoryItem) error {
	s.savedScopes = append(s.savedScopes, scope)
	s.savedItems = append(s.savedItems, item)
	if s.saveErr != nil {
		return s.saveErr
	}
	return nil
}

func (s *fakeMemoryStore) Delete(_ context.Context, _ kernel.Scope, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedIDs = append(s.deletedIDs, id)
	for i, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			break
		}
	}
	return nil
}

func (p *fakePersistence) SaveThread(_ context.Context, t *store.Thread) error {
	if t == nil {
		return nil
	}
	cp := *t
	p.thread = &cp
	return nil
}

func (p *fakePersistence) SaveTurn(_ context.Context, t *store.Turn) error {
	p.turnSaveCount++
	if p.saveTurnErr != nil && p.turnSaveCount > 1 {
		return p.saveTurnErr
	}
	cp := *t
	p.turns = append(p.turns, &cp)
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

func (p *fakePersistence) SaveSkillRun(_ context.Context, r *store.SkillRun) error {
	if r == nil {
		return nil
	}
	cp := *r
	p.skillRuns = append(p.skillRuns, &cp)
	return nil
}

func (p *fakePersistence) SaveArtifact(_ context.Context, a *store.Artifact) error {
	if a == nil {
		return nil
	}
	cp := *a
	p.artifacts = append(p.artifacts, &cp)
	return nil
}

func (p *fakePersistence) SaveApproval(_ context.Context, a *store.Approval) error {
	if a == nil {
		return nil
	}
	cp := *a
	p.approvals = append(p.approvals, &cp)
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

func (p *fakePersistence) LoadSkillRun(_ context.Context, tenantID, runID string) (*store.SkillRun, error) {
	for _, run := range p.skillRuns {
		if run.TenantID == tenantID && run.ID == runID {
			cp := *run
			return &cp, nil
		}
	}
	return nil, store.ErrSkillRunNotFound
}

func (p *fakePersistence) ListSkillRuns(_ context.Context, tenantID, skillID, status string, _, _ int) ([]*store.SkillRun, error) {
	out := make([]*store.SkillRun, 0, len(p.skillRuns))
	for _, run := range p.skillRuns {
		if run.TenantID != tenantID {
			continue
		}
		if skillID != "" && run.SkillID != skillID {
			continue
		}
		if status != "" && run.Status != status {
			continue
		}
		cp := *run
		out = append(out, &cp)
	}
	return out, nil
}

func (p *fakePersistence) LoadArtifact(_ context.Context, tenantID, artifactID string) (*store.Artifact, error) {
	for _, artifact := range p.artifacts {
		if artifact.TenantID == tenantID && artifact.ID == artifactID {
			cp := *artifact
			return &cp, nil
		}
	}
	return nil, store.ErrArtifactNotFound
}

func (p *fakePersistence) ListArtifacts(_ context.Context, tenantID, skillRunID, kind, status string, _, _ int) ([]*store.Artifact, error) {
	out := make([]*store.Artifact, 0, len(p.artifacts))
	for _, artifact := range p.artifacts {
		if artifact.TenantID != tenantID {
			continue
		}
		if skillRunID != "" && artifact.SkillRunID != skillRunID {
			continue
		}
		if kind != "" && artifact.Kind != kind {
			continue
		}
		if status != "" && artifact.Status != status {
			continue
		}
		cp := *artifact
		out = append(out, &cp)
	}
	return out, nil
}

func (p *fakePersistence) LoadApproval(_ context.Context, tenantID, approvalID string) (*store.Approval, error) {
	for _, approval := range p.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			cp := *approval
			return &cp, nil
		}
	}
	return nil, store.ErrApprovalNotFound
}

func (p *fakePersistence) ListApprovals(_ context.Context, tenantID, status string, _, _ int) ([]*store.Approval, error) {
	out := make([]*store.Approval, 0, len(p.approvals))
	for _, approval := range p.approvals {
		if approval.TenantID != tenantID {
			continue
		}
		if status != "" && approval.Status != status {
			continue
		}
		cp := *approval
		out = append(out, &cp)
	}
	return out, nil
}

func (p *fakePersistence) UpdateApprovalStatus(_ context.Context, tenantID, approvalID, status, actorID string) (*store.Approval, error) {
	for _, approval := range p.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			if approval.Status == "" || approval.Status == store.ApprovalStatusPending {
				now := time.Now()
				approval.Status = status
				approval.DecisionBy = actorID
				approval.DecisionAt = &now
				approval.UpdatedAt = now
			}
			cp := *approval
			return &cp, nil
		}
	}
	return nil, store.ErrApprovalNotFound
}

func (p *fakePersistence) ClaimApprovalForApply(_ context.Context, tenantID, approvalID, actorID string) (*store.Approval, error) {
	for _, approval := range p.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			if approval.Status == store.ApprovalStatusApproved || approval.Status == store.ApprovalStatusApplyFailed {
				approval.Status = store.ApprovalStatusApplying
				approval.AppliedBy = actorID
				approval.ApplyError = ""
				approval.UpdatedAt = time.Now()
			}
			cp := *approval
			return &cp, nil
		}
	}
	return nil, store.ErrApprovalNotFound
}

func (p *fakePersistence) MarkApprovalApplied(_ context.Context, tenantID, approvalID, result, actorID string) (*store.Approval, error) {
	for _, approval := range p.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			if approval.Status == store.ApprovalStatusApplying {
				now := time.Now()
				approval.Status = store.ApprovalStatusApplied
				approval.AppliedBy = actorID
				approval.AppliedAt = &now
				approval.ApplyResult = result
				approval.ApplyError = ""
				approval.UpdatedAt = now
			}
			cp := *approval
			return &cp, nil
		}
	}
	return nil, store.ErrApprovalNotFound
}

func (p *fakePersistence) MarkApprovalApplyFailed(_ context.Context, tenantID, approvalID, applyErr, actorID string) (*store.Approval, error) {
	for _, approval := range p.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			if approval.Status == store.ApprovalStatusApplying {
				approval.Status = store.ApprovalStatusApplyFailed
				approval.AppliedBy = actorID
				approval.ApplyError = applyErr
				approval.UpdatedAt = time.Now()
			}
			cp := *approval
			return &cp, nil
		}
	}
	return nil, store.ErrApprovalNotFound
}

func (p *fakePersistence) DeleteThread(context.Context, string, string) error {
	p.thread = nil
	p.messages = nil
	p.skillRuns = nil
	p.artifacts = nil
	p.approvals = nil
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

func TestRunTurn_PersistsCompletedTurnStatus(t *testing.T) {
	persist := &fakePersistence{}
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "done"}}}}}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		emitter,
		nil,
	)

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_status_done", "hello")
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(persist.turns) != 2 {
		t.Fatalf("expected running and completed turn saves, got %#v", persist.turns)
	}
	if persist.turns[0].Status != store.TurnStatusRunning || persist.turns[0].Completed {
		t.Fatalf("unexpected initial turn state: %#v", persist.turns[0])
	}
	final := persist.turns[len(persist.turns)-1]
	if final.Status != store.TurnStatusCompleted || !final.Completed || final.CompletedAt == nil || final.Error != "" {
		t.Fatalf("unexpected completed turn state: %#v", final)
	}
	if len(emitter.ByType(telemetry.TurnCompleted)) != 1 {
		t.Fatalf("expected turn_completed telemetry, got %#v", emitter.Events)
	}
}

func TestRunTurn_PersistsFailedTurnStatus(t *testing.T) {
	persist := &fakePersistence{}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{err: errors.New("provider unavailable")},
			{err: errors.New("provider unavailable")},
			{err: errors.New("provider unavailable")},
		},
	}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		persist,
		emitter,
		nil,
	)

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_status_failed", "hello")
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err == nil {
		t.Fatal("expected stream error")
	}
	final := persist.turns[len(persist.turns)-1]
	if final.Status != store.TurnStatusFailed || !final.Completed || final.CompletedAt == nil {
		t.Fatalf("unexpected failed turn state: %#v", final)
	}
	if !strings.Contains(final.Error, "provider unavailable") {
		t.Fatalf("expected persisted failure reason, got %q", final.Error)
	}
	failures := emitter.ByType(telemetry.TurnFailed)
	if len(failures) != 1 {
		t.Fatalf("expected turn_failed telemetry, got %#v", emitter.Events)
	}
	if failures[0].Attrs["reason"] != "llm_call_failed" {
		t.Fatalf("unexpected failure telemetry attrs: %#v", failures[0].Attrs)
	}
}

func TestRunTurnWithOptions_LoadsActiveMarkdownSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "private", "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read, listing.draft_write, listing.apply_after_approval
tool_hints: listings_get_template, listings_create
---

# Product Import

Always create reviewable proposals before apply.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{Delta: "ok"}}},
		},
	}
	orch := newTestOrch(llm, nil)
	orch.SetSystemPrompt("Base prompt.")
	orch.RegisterTools([]ToolDef{
		{Name: "listings_get_template", Description: "Get listing template", Schema: `{}`},
		{Name: "listings_create", Description: "Create listing", Schema: `{}`},
		{Name: "orders_refund", Description: "Refund order", Schema: `{}`},
	})
	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_1", "import products", TurnOptions{
		SkillProvider:   agentskill.NewFilesystemProvider(dir),
		RequestedSkills: []string{"product.import"},
		ToolCatalog: kernel.NewStaticToolCatalog([]kernel.ToolMetadata{
			{
				Name:            "listings_get_template",
				Description:     "Get listing template",
				Risk:            kernel.RiskRead,
				Approval:        kernel.ApprovalNone,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingRead},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "listings_create",
				Description:     "Create listing",
				Risk:            kernel.RiskWrite,
				Approval:        kernel.ApprovalExplicit,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingDraftWrite},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "orders_refund",
				Description:     "Refund order",
				Risk:            kernel.RiskFinancial,
				Approval:        kernel.ApprovalExplicit,
				Capabilities:    []kernel.Capability{kernel.CapabilityOrderFinancial},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
		}),
		Scope: kernel.Scope{ActingPersona: kernel.PersonaSeller},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(llm.captured) != 1 || len(llm.captured[0].messages) == 0 {
		t.Fatalf("expected captured LLM call")
	}
	system := llm.captured[0].messages[0]
	if system.Role != "system" {
		t.Fatalf("expected system message, got %#v", system)
	}
	for _, want := range []string{"Base prompt.", "## Available Skills", "## Runtime-Injected Active Skills", "required capabilities", "granted tools for this turn", "Always create reviewable proposals before apply."} {
		if !strings.Contains(system.Content, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system.Content)
		}
	}
	tools := llm.captured[0].tools
	if len(tools) != 3 {
		t.Fatalf("expected two granted tools plus use_skill_tool, got %#v", tools)
	}
	if tools[0].Name != "listings_get_template" || tools[1].Name != "listings_create" || tools[2].Name != runtimeUseSkillToolName {
		t.Fatalf("unexpected granted tool set: %#v", tools)
	}
	if strings.Contains(system.Content, "orders_refund") {
		t.Fatalf("system prompt should not expose ungranted refund tool:\n%s", system.Content)
	}
}

func TestRunTurnWithOptions_MissingSkillDoesNotSaveTurn(t *testing.T) {
	llm := &mockLLM{}
	persist := &fakePersistence{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{CallID: c.ID, Content: "ok"}, nil
		}), 5*time.Second, 0),
		persist,
		&telemetry.BufferEmitter{},
		nil,
	)

	_, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_1", "import products", TurnOptions{
		SkillProvider:   agentskill.NewFilesystemProvider(t.TempDir()),
		RequestedSkills: []string{"product.import"},
	})
	if !errors.Is(err, agentskill.ErrSkillNotFound) {
		t.Fatalf("expected missing skill error, got %v", err)
	}
	if persist.turnSaveCount != 0 {
		t.Fatalf("turn should not be saved when skill resolution fails, got %d saves", persist.turnSaveCount)
	}
	if len(llm.captured) != 0 {
		t.Fatal("LLM should not be called when skill resolution fails")
	}
}

func TestRunTurnWithOptions_RequestedSkillRespectsFilter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
---

# Product Import
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{}
	persist := &fakePersistence{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{CallID: c.ID, Content: "ok"}, nil
		}), 5*time.Second, 0),
		persist,
		&telemetry.BufferEmitter{},
		nil,
	)

	_, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_filtered_skill", "import products", TurnOptions{
		SkillProvider:   agentskill.NewFilesystemProvider(dir),
		RequestedSkills: []string{"product.import"},
		SkillFilter:     agentskill.Filter{Persona: string(kernel.PersonaBuyer)},
	})
	if err == nil || !strings.Contains(err.Error(), "not available for this turn") {
		t.Fatalf("expected requested skill to respect filter, got %v", err)
	}
	if persist.turnSaveCount != 0 {
		t.Fatalf("turn should not be saved when requested skill is unavailable, got %d saves", persist.turnSaveCount)
	}
	if len(llm.captured) != 0 {
		t.Fatal("LLM should not be called when requested skill is filtered out")
	}
}

func TestRunTurnWithOptions_UseSkillToolLoadsSkillAndRestrictsNextTools(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read, listing.draft_write
---

# Product Import

Always create reviewable proposals before apply.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{
				ToolCalls: []stream.ToolCall{{
					ID:        "skill_call_1",
					Name:      runtimeUseSkillToolName,
					Arguments: `{"skill":"product.import"}`,
				}},
			}}},
			{chunks: []stream.Chunk{{Delta: "skill loaded"}}},
		},
	}
	orch := newTestOrch(llm, nil)
	orch.RegisterTools([]ToolDef{
		{Name: "listings_get_template", Description: "Get listing template", Schema: `{}`},
		{Name: "listings_create", Description: "Create listing", Schema: `{}`},
		{Name: "orders_refund", Description: "Refund order", Schema: `{}`},
	})
	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_use_skill", "import products", TurnOptions{
		SkillProvider: agentskill.NewFilesystemProvider(dir),
		ToolCatalog: kernel.NewStaticToolCatalog([]kernel.ToolMetadata{
			{
				Name:            "listings_get_template",
				Description:     "Get listing template",
				Risk:            kernel.RiskRead,
				Approval:        kernel.ApprovalNone,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingRead},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "listings_create",
				Description:     "Create listing",
				Risk:            kernel.RiskWrite,
				Approval:        kernel.ApprovalExplicit,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingDraftWrite},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "orders_refund",
				Description:     "Refund order",
				Risk:            kernel.RiskFinancial,
				Approval:        kernel.ApprovalExplicit,
				Capabilities:    []kernel.Capability{kernel.CapabilityOrderFinancial},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
		}),
		Scope: kernel.Scope{ActingPersona: kernel.PersonaSeller},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(llm.captured) != 2 {
		t.Fatalf("expected two LLM calls, got %d", len(llm.captured))
	}
	firstTools := toolDefNames(llm.captured[0].tools)
	if len(firstTools) != 1 || firstTools[0] != runtimeUseSkillToolName {
		t.Fatalf("first turn should expose only use_skill_tool before skill activation, got %#v", firstTools)
	}
	secondTools := toolDefNames(llm.captured[1].tools)
	for _, want := range []string{runtimeUseSkillToolName, "listings_get_template", "listings_create"} {
		if !containsToolDef(secondTools, want) {
			t.Fatalf("second turn missing %s, got %#v", want, secondTools)
		}
	}
	if containsToolDef(secondTools, "orders_refund") {
		t.Fatalf("second turn should not expose refund after product.import activation: %#v", secondTools)
	}
}

func TestRunTurnWithOptions_UseSkillToolDefersMixedOrdinaryTools(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read, listing.draft_write
---

# Product Import

Always create reviewable proposals before apply.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{
				ToolCalls: []stream.ToolCall{
					{
						ID:        "skill_call_1",
						Name:      runtimeUseSkillToolName,
						Arguments: `{"skill":"product.import"}`,
					},
					{
						ID:        "refund_call_1",
						Name:      "orders_refund",
						Arguments: `{}`,
					},
				},
			}}},
			{chunks: []stream.Chunk{{Delta: "skill loaded"}}},
		},
	}
	executed := false
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			executed = true
			return exec.ToolResult{CallID: c.ID, Name: c.Name, Content: "ok"}, nil
		}), 5*time.Second, 0),
		nil,
		&telemetry.BufferEmitter{},
		nil,
	)
	orch.RegisterTools([]ToolDef{
		{Name: "listings_get_template", Description: "Get listing template", Schema: `{}`},
		{Name: "listings_create", Description: "Create listing", Schema: `{}`},
		{Name: "orders_refund", Description: "Refund order", Schema: `{}`},
	})
	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_use_skill_mixed", "import products", TurnOptions{
		SkillProvider: agentskill.NewFilesystemProvider(dir),
		ToolCatalog: kernel.NewStaticToolCatalog([]kernel.ToolMetadata{
			{
				Name:            "listings_get_template",
				Description:     "Get listing template",
				Risk:            kernel.RiskRead,
				Approval:        kernel.ApprovalNone,
				SideEffect:      kernel.SideEffectNone,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingRead},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "listings_create",
				Description:     "Create listing",
				Risk:            kernel.RiskWrite,
				Approval:        kernel.ApprovalExplicit,
				SideEffect:      kernel.SideEffectMutable,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingDraftWrite},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "orders_refund",
				Description:     "Refund order",
				Risk:            kernel.RiskFinancial,
				Approval:        kernel.ApprovalExplicit,
				SideEffect:      kernel.SideEffectMutable,
				Capabilities:    []kernel.Capability{kernel.CapabilityOrderFinancial},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
		}),
		Scope: kernel.Scope{ActingPersona: kernel.PersonaSeller},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}
	if executed {
		t.Fatal("ordinary tool in same batch as use_skill_tool should not be executed")
	}
	if len(llm.captured) != 2 {
		t.Fatalf("expected second LLM call after skill routing, got %d", len(llm.captured))
	}
	secondTools := toolDefNames(llm.captured[1].tools)
	if containsToolDef(secondTools, "orders_refund") {
		t.Fatalf("refund should not be exposed after product.import activation: %#v", secondTools)
	}
	var sawRefundRejected bool
	for _, chunk := range chunks {
		if chunk.ToolEvent != nil && chunk.ToolEvent.ID == "refund_call_1" && chunk.ToolEvent.Status == "error" {
			sawRefundRejected = true
		}
	}
	if !sawRefundRejected {
		t.Fatalf("expected mixed ordinary tool to be rejected, got %#v", chunks)
	}
}

func TestRunTurnWithOptions_ExplicitApprovalToolRequiresApproval(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read, listing.draft_write
---

# Product Import
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{
				ToolCalls: []stream.ToolCall{{
					ID:        "create_call_1",
					Name:      "listings_create",
					Arguments: `{"sourceArtifactIds":["art_proposal_1","art_proposal_1"],"listing":{"title":"Draft Shirt","proposalArtifactId":"art_proposal_2"}}`,
				}},
			}}},
			{chunks: []stream.Chunk{{Delta: "Approval required"}}},
		},
	}
	persist := &fakePersistence{}
	executed := false
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			executed = true
			return exec.ToolResult{CallID: c.ID, Name: c.Name, Content: "ok"}, nil
		}), 5*time.Second, 0),
		persist,
		&telemetry.BufferEmitter{},
		nil,
	)
	orch.RegisterTools([]ToolDef{
		{Name: "listings_get_template", Description: "Get listing template", Schema: `{}`},
		{Name: "listings_create", Description: "Create listing", Schema: `{}`},
	})
	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_approval_required", "import products", TurnOptions{
		SkillProvider:   agentskill.NewFilesystemProvider(dir),
		RequestedSkills: []string{"product.import"},
		ToolCatalog: kernel.NewStaticToolCatalog([]kernel.ToolMetadata{
			{
				Name:            "listings_get_template",
				Description:     "Get listing template",
				Risk:            kernel.RiskRead,
				Approval:        kernel.ApprovalNone,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingRead},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
			{
				Name:            "listings_create",
				Description:     "Create listing",
				Risk:            kernel.RiskWrite,
				Approval:        kernel.ApprovalExplicit,
				SideEffect:      kernel.SideEffectMutable,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingDraftWrite},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
			},
		}),
		Scope: kernel.Scope{
			TenantID:      "tenant_1",
			StoreID:       "store_1",
			ActorID:       "seller_1",
			ActingPersona: kernel.PersonaSeller,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}
	if executed {
		t.Fatal("approval-explicit tool should not be executed directly")
	}
	var sawToolResult bool
	for _, chunk := range chunks {
		if chunk.ToolEvent != nil && chunk.ToolEvent.ID == "create_call_1" && chunk.ToolEvent.Status == "approval_required" {
			sawToolResult = true
			if len(chunk.ToolEvent.Result) == 0 || !strings.Contains(string(chunk.ToolEvent.Result), `"status":"approval_required"`) {
				t.Fatalf("expected structured approval result payload, got %#v", chunk.ToolEvent.Result)
			}
			if !strings.Contains(string(chunk.ToolEvent.Result), `"artifactIds":["art_proposal_1","art_proposal_2"]`) {
				t.Fatalf("expected artifact ids in approval result, got %s", string(chunk.ToolEvent.Result))
			}
		}
	}
	if !sawToolResult {
		t.Fatalf("expected approval-required tool result event, got %#v", chunks)
	}
	var approvalMessage *store.Message
	for _, msg := range persist.messages {
		if msg.Role == "tool" && msg.ToolCallID == "create_call_1" {
			approvalMessage = msg
			break
		}
	}
	if approvalMessage == nil {
		t.Fatal("expected approval-required tool message to be saved")
	}
	if len(persist.approvals) != 1 {
		t.Fatalf("expected one durable approval, got %#v", persist.approvals)
	}
	approval := persist.approvals[0]
	if approval.Status != store.ApprovalStatusPending || approval.TenantID != "tenant_1" || approval.StoreID != "store_1" || approval.Action != "listings_create" {
		t.Fatalf("unexpected persisted approval: %#v", approval)
	}
	if approval.RequestHash == "" || approval.ToolCallID != "create_call_1" || approval.SkillID != string(kernel.SkillProductImport) {
		t.Fatalf("persisted approval missing identity/hash fields: %#v", approval)
	}
	if approval.ArtifactIDs != `["art_proposal_1","art_proposal_2"]` {
		t.Fatalf("persisted approval should reference proposal artifacts, got %q", approval.ArtifactIDs)
	}
	for _, want := range []string{`"status":"approval_required"`, `"action":"listings_create"`, `"requestHash":`, `"id":"appr_`} {
		if !strings.Contains(approvalMessage.Content, want) {
			t.Fatalf("approval message missing %s: %s", want, approvalMessage.Content)
		}
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
	orch.RegisterTools([]ToolDef{{Name: "search", Description: "Search", Schema: `{}`}})

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

func TestRunTurn_CompactsSummaryToolResultsInHistory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "product.import"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "product.import", "SKILL.md"), []byte(`---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read
tool_hints: search
---

# Product Import
`), 0o644); err != nil {
		t.Fatal(err)
	}
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{{
				ToolCalls: []stream.ToolCall{{
					ID:        "tc_1",
					Name:      "search",
					Arguments: `{"q":"caps"}`,
				}},
			}}},
			{chunks: []stream.Chunk{{Delta: "Found candidates."}}},
		},
	}
	persist := &fakePersistence{}
	executor := exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
		return exec.ToolResult{
			CallID:  c.ID,
			Name:    c.Name,
			Content: `{"data":[{"title":"Cap","api_key":"secret-value"}],"meta":{"page":1}}`,
		}, nil
	})
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(executor, 5*time.Second, 0),
		persist,
		&telemetry.BufferEmitter{},
		nil,
	)
	orch.RegisterTools([]ToolDef{{Name: "search", Description: "Search", Schema: `{}`}})

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "th_summary_tool", "import caps", TurnOptions{
		SkillProvider:   agentskill.NewFilesystemProvider(dir),
		RequestedSkills: []string{"product.import"},
		ToolCatalog: kernel.NewStaticToolCatalog([]kernel.ToolMetadata{
			{
				Name:            "search",
				Description:     "Search",
				Risk:            kernel.RiskRead,
				Approval:        kernel.ApprovalNone,
				SideEffect:      kernel.SideEffectNone,
				Capabilities:    []kernel.Capability{kernel.CapabilityListingRead},
				AllowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
				AllowedPersonas: []kernel.Persona{kernel.PersonaSeller},
				ResultMode:      "summary",
			},
		}),
		Scope: kernel.Scope{ActingPersona: kernel.PersonaSeller},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	var toolContent string
	for _, msg := range persist.messages {
		if msg.Role == "tool" && msg.ToolCallID == "tc_1" {
			toolContent = msg.Content
			break
		}
	}
	if toolContent == "" {
		t.Fatalf("missing persisted tool message: %#v", persist.messages)
	}
	for _, want := range []string{"Tool result compacted", `"resultMode":"summary"`, `"dataItemCount":1`, "[REDACTED]"} {
		if !strings.Contains(toolContent, want) {
			t.Fatalf("compacted tool content missing %q:\n%s", want, toolContent)
		}
	}
	if strings.Contains(toolContent, "secret-value") {
		t.Fatalf("tool content should redact sensitive values:\n%s", toolContent)
	}
}

func TestRunTurn_RedactsRedactedToolResultsInHistory(t *testing.T) {
	content := compactToolResultForHistory("listings_create", `{"id":"listing_1","title":"Cap"}`, "redacted", false)
	for _, want := range []string{`"tool":"listings_create"`, `"resultMode":"redacted"`, "result omitted"} {
		if !strings.Contains(content, want) {
			t.Fatalf("redacted result missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "listing_1") || strings.Contains(content, "Cap") {
		t.Fatalf("redacted result leaked content:\n%s", content)
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
	orch.RegisterTools([]ToolDef{{Name: "search", Description: "Search", Schema: `{}`}})

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

func TestRunTurn_RejectsUnauthorizedToolCall(t *testing.T) {
	llm := &mockLLM{
		responses: []mockLLMResponse{
			{chunks: []stream.Chunk{
				{ToolCalls: []stream.ToolCall{{ID: "tc_1", Name: "refund", Arguments: `{}`}}},
			}},
			{chunks: []stream.Chunk{{Delta: "I cannot run that tool."}}},
		},
	}
	executed := false
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(_ context.Context, c exec.ToolCall) (exec.ToolResult, error) {
			executed = true
			return exec.ToolResult{CallID: c.ID, Content: "ok"}, nil
		}), 5*time.Second, 0),
		nil,
		&telemetry.BufferEmitter{},
		nil,
	)
	orch.RegisterTools([]ToolDef{{Name: "search", Description: "Search", Schema: `{}`}})

	result, err := orch.RunTurn(context.Background(), "tenant_1", "th_unauthorized", "refund")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunks, streamErr := stream.Collect(result.Output)
	if streamErr != nil {
		t.Fatalf("stream error: %v", streamErr)
	}
	if executed {
		t.Fatal("unauthorized tool should not be executed")
	}
	var sawRejected bool
	for _, chunk := range chunks {
		if chunk.ToolEvent != nil && chunk.ToolEvent.Name == "refund" && chunk.ToolEvent.Status == "error" {
			sawRejected = true
		}
	}
	if !sawRejected {
		t.Fatalf("expected rejected tool event, got %#v", chunks)
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

func TestRunTurn_ReplayShapingKeepsRecentMessages(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.Config{
			MaxContextTokens: 1000,
			ReservedOutput:   1,
			ShapeThreshold:   0.01,
			CompactThreshold: 0.99,
		}),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		&Config{ShapeKeepMsgs: 3},
	)
	for i := 0; i < 20; i++ {
		orch.mem.AppendMessage("tenant_1", "thread_1", &store.Message{
			ID:      fmt.Sprintf("msg_%d", i),
			Role:    "user",
			Content: fmt.Sprintf("old message %d", i),
		})
	}

	result, err := orch.RunTurn(context.Background(), "tenant_1", "thread_1", "current request")
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(emitter.ByType(telemetry.ReplayShaped)) == 0 {
		t.Fatal("expected replay_shaped telemetry")
	}
	if len(llm.captured) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(llm.captured))
	}
	for _, msg := range llm.captured[0].messages {
		if strings.Contains(msg.Content, "old message 0") {
			t.Fatalf("oldest message should have been shaped out: %#v", llm.captured[0].messages)
		}
	}
}

func TestRunTurn_DeterministicCompactionAddsSummary(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.Config{
			MaxContextTokens: 1000,
			ReservedOutput:   1,
			ShapeThreshold:   0.99,
			CompactThreshold: 0.01,
		}),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		&Config{ShapeKeepMsgs: 3},
	)
	for i := 0; i < 12; i++ {
		orch.mem.AppendMessage("tenant_1", "thread_2", &store.Message{
			ID:      fmt.Sprintf("msg_%d", i),
			Role:    "assistant",
			Content: fmt.Sprintf("detailed historical answer %d", i),
		})
	}

	result, err := orch.RunTurn(context.Background(), "tenant_1", "thread_2", "current request")
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(emitter.ByType(telemetry.CompactionSucceeded)) == 0 {
		t.Fatal("expected compaction success telemetry")
	}
	foundSummary := false
	for _, msg := range llm.captured[0].messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Earlier conversation compacted deterministically") {
			foundSummary = true
		}
	}
	if !foundSummary {
		t.Fatalf("expected deterministic compaction summary in prompt: %#v", llm.captured[0].messages)
	}
}

func TestReplayShapingPreservesToolCallGroup(t *testing.T) {
	history := []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "older question"},
		{Role: "assistant", Content: "older answer"},
		{Role: "user", Content: "search this"},
		{Role: "assistant", ToolCalls: []stream.ToolCall{{ID: "tc_1", Name: "search", Arguments: `{}`}}},
		{Role: "tool", Content: "search result", ToolCallID: "tc_1"},
		{Role: "user", Content: "current request"},
	}

	shaped := shapeReplayHistory(history, 2)
	if len(shaped) != 4 {
		t.Fatalf("expected system plus intact tool-call group, got %#v", shaped)
	}
	if shaped[0].Role != "system" {
		t.Fatalf("expected system message to be retained, got %#v", shaped)
	}
	if shaped[1].Role != "assistant" || len(shaped[1].ToolCalls) != 1 || shaped[1].ToolCalls[0].ID != "tc_1" {
		t.Fatalf("expected assistant tool call before tool result, got %#v", shaped)
	}
	if shaped[2].Role != "tool" || shaped[2].ToolCallID != "tc_1" {
		t.Fatalf("expected matching tool result after assistant call, got %#v", shaped)
	}
}

func TestReplayCompactionPreservesToolCallGroup(t *testing.T) {
	history := []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "older question"},
		{Role: "assistant", Content: "older answer"},
		{Role: "user", Content: "search this"},
		{Role: "assistant", ToolCalls: []stream.ToolCall{{ID: "tc_1", Name: "search", Arguments: `{}`}}},
		{Role: "tool", Content: "search result", ToolCallID: "tc_1"},
		{Role: "user", Content: "current request"},
	}

	compacted := compactReplayHistory(history, 2)
	if len(compacted) != 5 {
		t.Fatalf("expected compacted summary plus intact tool-call group, got %#v", compacted)
	}
	if compacted[0].Role != "system" {
		t.Fatalf("expected system message to be retained, got %#v", compacted)
	}
	if compacted[1].Role != "system" || !strings.Contains(compacted[1].Content, "Earlier conversation compacted deterministically") {
		t.Fatalf("expected deterministic summary, got %#v", compacted)
	}
	if compacted[2].Role != "assistant" || len(compacted[2].ToolCalls) != 1 || compacted[2].ToolCalls[0].ID != "tc_1" {
		t.Fatalf("expected assistant tool call before tool result, got %#v", compacted)
	}
	if compacted[3].Role != "tool" || compacted[3].ToolCallID != "tc_1" {
		t.Fatalf("expected matching tool result after assistant call, got %#v", compacted)
	}
}

func TestRunTurnWithOptions_InjectsRetrievedMemory(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{items: []kernel.MemoryItem{
		{
			ID:      "mem_1",
			Scope:   kernel.MemoryUser,
			Subject: "language",
			Content: "用户偏好中文回答。",
		},
	}}
	orch := newTestOrch(llm, nil)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory", "hello", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
			StoreID:  "store_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(llm.captured) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(llm.captured))
	}
	found := false
	for _, msg := range llm.captured[0].messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Relevant memory for this turn") && strings.Contains(msg.Content, "用户偏好中文回答") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected memory context in system prompt: %#v", llm.captured[0].messages)
	}
}

func TestRunTurnWithOptions_MemoryFallsBackWhenQueryMisses(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{
		missQueries: true,
		items: []kernel.MemoryItem{
			{
				ID:      "mem_1",
				Scope:   kernel.MemoryTenant,
				Subject: "policy",
				Content: "人工确认高风险操作。",
			},
		},
	}
	orch := newTestOrch(llm, nil)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_fallback", "   please review risk   ", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
			StoreID:  "store_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(memory.queries) != 2 {
		t.Fatalf("expected query search plus fallback search, got %#v", memory.queries)
	}
	if memory.queries[0].Query != "please review risk" {
		t.Fatalf("expected normalized user query, got %q", memory.queries[0].Query)
	}
	if memory.queries[0].Scope.ThreadID != "thread_memory_fallback" {
		t.Fatalf("expected turn thread scope, got %#v", memory.queries[0].Scope)
	}
	if memory.queries[1].Query != "" {
		t.Fatalf("expected fallback query to be empty, got %q", memory.queries[1].Query)
	}
	found := false
	for _, msg := range llm.captured[0].messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "人工确认高风险操作") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fallback memory context in system prompt: %#v", llm.captured[0].messages)
	}
}

func TestRunTurnWithOptions_SavesExplicitMemory(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_save", "请记住：我喜欢中文回答", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
			StoreID:  "store_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(memory.savedItems) != 1 {
		t.Fatalf("expected one saved memory, got %#v", memory.savedItems)
	}
	if memory.savedScopes[0].ThreadID != "thread_memory_save" {
		t.Fatalf("expected saved scope to include turn thread id, got %#v", memory.savedScopes[0])
	}
	saved := memory.savedItems[0]
	if saved.Scope != kernel.MemoryUser {
		t.Fatalf("expected user-scoped memory, got %q", saved.Scope)
	}
	if saved.Subject != "preference" {
		t.Fatalf("expected preference subject, got %q", saved.Subject)
	}
	if saved.Content != "我喜欢中文回答" {
		t.Fatalf("expected cleaned memory content, got %q", saved.Content)
	}
	if saved.ID == "" || saved.Metadata["source"] != "explicit_user_message" {
		t.Fatalf("expected stable id and source metadata, got %#v", saved)
	}
	if len(emitter.ByType(telemetry.MemorySaved)) != 1 {
		t.Fatalf("expected memory_saved telemetry, got %#v", emitter.Events)
	}
}

func TestRunTurnWithOptions_DoesNotSaveNegatedExplicitMemory(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{}
	orch := newTestOrch(llm, nil)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_skip", "不要记住这个临时偏好", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(memory.savedItems) != 0 {
		t.Fatalf("expected no saved memories, got %#v", memory.savedItems)
	}
}

func TestRunTurnWithOptions_MemorySaveFailureDoesNotFailTurn(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{saveErr: errors.New("memory unavailable")}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_save_failed", "remember that default language is Chinese", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(memory.savedItems) != 1 {
		t.Fatalf("expected attempted memory save, got %#v", memory.savedItems)
	}
	if len(emitter.ByType(telemetry.MemorySaveFailed)) != 1 {
		t.Fatalf("expected memory_save_failed telemetry, got %#v", emitter.Events)
	}
}

func TestRunTurnWithOptions_DeletesExplicitForgetMemory(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{items: []kernel.MemoryItem{{
		ID:      "mem_1",
		Scope:   kernel.MemoryUser,
		Subject: "preference",
		Content: "我喜欢中文回答",
	}}}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_forget", "忘记：我喜欢中文回答", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(memory.deletedIDs) != 1 || memory.deletedIDs[0] != "mem_1" {
		t.Fatalf("expected memory deletion, got %#v", memory.deletedIDs)
	}
	if len(memory.savedItems) != 0 {
		t.Fatalf("expected no memory save for forget request, got %#v", memory.savedItems)
	}
	if len(memory.queries) == 0 || memory.queries[0].Query != "我喜欢中文回答" {
		t.Fatalf("expected explicit forget search query first, got %#v", memory.queries)
	}
	if len(memory.queries[0].Types) != 1 || memory.queries[0].Types[0] != kernel.MemoryUser {
		t.Fatalf("expected forget to search user memories only, got %#v", memory.queries[0].Types)
	}
	if len(emitter.ByType(telemetry.MemoryDeleted)) != 1 {
		t.Fatalf("expected memory_deleted telemetry, got %#v", emitter.Events)
	}
}

func TestRunTurnWithOptions_MemoryDeleteFailureDoesNotFailTurn(t *testing.T) {
	llm := &mockLLM{responses: []mockLLMResponse{{chunks: []stream.Chunk{{Delta: "ok"}}}}}
	memory := &fakeMemoryStore{
		items: []kernel.MemoryItem{{
			ID:      "mem_1",
			Scope:   kernel.MemoryUser,
			Content: "我喜欢中文回答",
		}},
		deleteErr: errors.New("memory delete unavailable"),
	}
	emitter := &telemetry.BufferEmitter{}
	orch := NewOrchestrator(
		llm,
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(exec.ToolExecutorFunc(func(context.Context, exec.ToolCall) (exec.ToolResult, error) {
			return exec.ToolResult{}, nil
		}), 5*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	result, err := orch.RunTurnWithOptions(context.Background(), "tenant_1", "thread_memory_delete_failed", "忘记：我喜欢中文回答", TurnOptions{
		MemoryStore: memory,
		Scope: kernel.Scope{
			TenantID: "tenant_1",
			ActorID:  "actor_1",
		},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if _, err := stream.Collect(result.Output); err != nil {
		t.Fatalf("collect stream: %v", err)
	}
	if len(emitter.ByType(telemetry.MemoryDeleteFailed)) != 1 {
		t.Fatalf("expected memory_delete_failed telemetry, got %#v", emitter.Events)
	}
}

func TestExplicitMemoryContent_IsConservative(t *testing.T) {
	content, ok := explicitMemoryContent("你还记住我喜欢中文回答吗")
	if ok || content != "" {
		t.Fatalf("expected question about memory not to be captured, got %q", content)
	}
	content, ok = explicitMemoryContent("记住：我喜欢中文回答")
	if !ok || content != "我喜欢中文回答" {
		t.Fatalf("expected explicit memory content, got ok=%v content=%q", ok, content)
	}
}

func TestExplicitMemoryForgetQuery_IsConservative(t *testing.T) {
	query, ok := explicitMemoryForgetQuery("你会忘记我喜欢中文回答吗")
	if ok || query != "" {
		t.Fatalf("expected forget question not to delete memory, got %q", query)
	}
	query, ok = explicitMemoryForgetQuery("忘记：我喜欢中文回答")
	if !ok || query != "我喜欢中文回答" {
		t.Fatalf("expected explicit forget query, got ok=%v query=%q", ok, query)
	}
	query, ok = explicitMemoryForgetQuery("forget everything")
	if ok || query != "" {
		t.Fatalf("expected broad forget request to be rejected, got %q", query)
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
	orch.RegisterTools([]ToolDef{{Name: "search", Description: "Search", Schema: `{}`}})

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

func toolDefNames(tools []ToolDef) []string {
	out := make([]string, len(tools))
	for i, tool := range tools {
		out[i] = tool.Name
	}
	return out
}

func containsToolDef(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
