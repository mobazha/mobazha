package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	"github.com/mobazha/mobazha3.0/pkg/agent/store"
	"github.com/mobazha/mobazha3.0/pkg/agent/stream"
	"github.com/mobazha/mobazha3.0/pkg/agent/telemetry"
	"github.com/mobazha/mobazha3.0/pkg/redact"
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

const runtimeUseSkillToolName = "use_skill_tool"

const (
	toolResultHistoryMaxLen = 2000
	toolResultExcerptMaxLen = 1200
)

// Config holds the orchestrator's tuning parameters.
type Config struct {
	MaxToolRounds  int           // max iterative tool→model rounds per turn (default 10)
	TurnTimeout    time.Duration // overall timeout for a single turn (default 120s)
	MaxHistoryMsgs int           // max messages loaded from thread history (default 50)
	LLMRetries     int           // retry count on transient LLM errors (default 2)
	ShapeKeepMsgs  int           // recent messages retained when replay shaping triggers (default 16)
}

func defaultConfig() Config {
	return Config{
		MaxToolRounds:  10,
		TurnTimeout:    120 * time.Second,
		MaxHistoryMsgs: 50,
		LLMRetries:     2,
		ShapeKeepMsgs:  16,
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
		if cfg.ShapeKeepMsgs > 0 {
			c.ShapeKeepMsgs = cfg.ShapeKeepMsgs
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

// TurnOptions carries per-turn runtime context. Skills and context blocks are
// intentionally per-turn, not global orchestrator state, to avoid cross-thread
// leakage.
type TurnOptions struct {
	SkillProvider   agentskill.Provider
	RequestedSkills []string
	SkillFilter     agentskill.Filter
	ContextBlocks   []string
	ToolCatalog     kernel.ToolCatalog
	MemoryStore     kernel.MemoryStore
	Scope           kernel.Scope
}

type resolvedTurnContext struct {
	availableSkills []string
	activeSkills    []*agentskill.Skill
	grantedTools    map[string][]kernel.ToolMetadata
	baseTools       []ToolDef
	tools           []ToolDef
	toolResultModes map[string]string
	contextBlocks   []string
	skillProvider   agentskill.Provider
	toolCatalog     kernel.ToolCatalog
	memoryStore     kernel.MemoryStore
	scope           kernel.Scope
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
	return o.RunTurnWithOptions(ctx, tenantID, threadID, userMsg, TurnOptions{})
}

// RunTurnWithOptions executes a turn with per-turn dynamic context such as
// runtime-loaded Markdown skills.
func (o *Orchestrator) RunTurnWithOptions(ctx context.Context, tenantID, threadID string, userMsg string, opts TurnOptions) (*TurnResult, error) {
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
	turnStarted := false
	defer func() {
		if !turnStarted {
			cancel()
		}
	}()

	turnID := newTurnID()
	turnStartedAt := time.Now()

	resolved, err := o.resolveTurnContext(turnCtx, opts)
	if err != nil {
		return nil, err
	}
	if resolved.scope.TenantID == "" {
		resolved.scope.TenantID = tenantID
	}
	if resolved.scope.ThreadID == "" {
		resolved.scope.ThreadID = threadID
	}

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

	o.forgetExplicitMemory(turnCtx, resolved, userMsg)

	history, err := o.assembleHistory(ctx, tenantID, threadID, userMsg, resolved)
	if err != nil {
		return nil, err
	}

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
	o.captureExplicitMemory(turnCtx, resolved, userMsg)

	outStream := stream.NewBuffered(ctx, 32)

	go func() {
		defer cancel()
		defer outStream.Finish()
		o.runLoop(turnCtx, tenantID, threadID, turnID, turnStartedAt, history, &resolved, outStream)
	}()
	turnStarted = true

	return &TurnResult{Output: outStream, TurnID: turnID}, nil
}

// assembleHistory builds the full message list for the LLM call:
// [system prompt] + [prior messages from memory] + [current user message].
func (o *Orchestrator) assembleHistory(ctx context.Context, tenantID, threadID, userMsg string, resolved resolvedTurnContext) ([]Message, error) {
	var msgs []Message

	systemPrompt := o.systemPromptWithSkills(resolved)
	if memoryBlock := o.memoryContextBlock(ctx, resolved, userMsg); memoryBlock != "" {
		resolved.contextBlocks = append([]string{memoryBlock}, resolved.contextBlocks...)
		systemPrompt = o.systemPromptWithSkills(resolved)
	}
	if systemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: systemPrompt})
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
	return msgs, nil
}

func (o *Orchestrator) resolveTurnContext(ctx context.Context, opts TurnOptions) (resolvedTurnContext, error) {
	resolved := resolvedTurnContext{
		baseTools:       append([]ToolDef(nil), o.tools...),
		tools:           append([]ToolDef(nil), o.tools...),
		toolResultModes: map[string]string{},
		contextBlocks:   append([]string(nil), opts.ContextBlocks...),
		grantedTools:    map[string][]kernel.ToolMetadata{},
		skillProvider:   opts.SkillProvider,
		toolCatalog:     opts.ToolCatalog,
		memoryStore:     opts.MemoryStore,
		scope:           opts.Scope,
	}
	if opts.SkillProvider == nil {
		return resolved, nil
	}
	available, err := opts.SkillProvider.List(ctx, opts.SkillFilter)
	if err != nil {
		return resolvedTurnContext{}, err
	}
	sort.Strings(available)
	resolved.availableSkills = available
	resolved.tools = appendRuntimeSkillTool(nil)

	for _, requested := range opts.RequestedSkills {
		s, err := opts.SkillProvider.Load(ctx, requested)
		if err != nil {
			return resolvedTurnContext{}, err
		}
		if !containsSkillID(resolved.availableSkills, s.ID) {
			return resolvedTurnContext{}, fmt.Errorf("skill %q is not available for this turn", s.ID)
		}
		resolved.activeSkills = append(resolved.activeSkills, s)
	}
	if opts.ToolCatalog == nil || len(resolved.activeSkills) == 0 {
		if opts.ToolCatalog != nil && len(resolved.activeSkills) == 0 {
			if err := recalculateInitialTools(ctx, &resolved); err != nil {
				return resolvedTurnContext{}, err
			}
		}
		return resolved, nil
	}
	if err := recalculateGrantedTools(ctx, &resolved); err != nil {
		return resolvedTurnContext{}, err
	}
	return resolved, nil
}

func recalculateInitialTools(ctx context.Context, resolved *resolvedTurnContext) error {
	if resolved == nil || resolved.toolCatalog == nil || resolved.skillProvider == nil {
		return nil
	}
	catalogTools, err := resolved.toolCatalog.List(ctx, resolved.scope)
	if err != nil {
		return err
	}
	allowedNames := map[string]struct{}{}
	for _, tool := range catalogTools {
		if initialToolAllowed(tool) {
			allowedNames[tool.Name] = struct{}{}
			rememberToolResultMode(resolved, tool)
		}
	}
	resolved.tools = appendRuntimeSkillTool(filterToolDefs(resolved.baseTools, allowedNames))
	return nil
}

func initialToolAllowed(tool kernel.ToolMetadata) bool {
	return len(tool.AllowedSkills) == 0 &&
		len(tool.Capabilities) == 0 &&
		tool.Risk == kernel.RiskRead &&
		tool.Approval == kernel.ApprovalNone &&
		tool.SideEffect == kernel.SideEffectNone
}

func rememberToolResultMode(resolved *resolvedTurnContext, tool kernel.ToolMetadata) {
	if resolved == nil || tool.Name == "" || tool.ResultMode == "" {
		return
	}
	if resolved.toolResultModes == nil {
		resolved.toolResultModes = map[string]string{}
	}
	resolved.toolResultModes[tool.Name] = tool.ResultMode
}

func toolResultMode(resolved *resolvedTurnContext, toolName string) string {
	if resolved == nil || resolved.toolResultModes == nil {
		return ""
	}
	return resolved.toolResultModes[toolName]
}

func (o *Orchestrator) memoryContextBlock(ctx context.Context, resolved resolvedTurnContext, userMsg string) string {
	if resolved.memoryStore == nil || resolved.scope.TenantID == "" {
		return ""
	}
	query := kernel.MemoryQuery{
		Scope: resolved.scope,
		Types: []kernel.MemoryScope{
			kernel.MemoryUser,
			kernel.MemoryStoreScope,
			kernel.MemoryTenant,
			kernel.MemoryThread,
			kernel.MemorySkill,
		},
		Limit: 5,
	}
	if text := memoryQueryText(userMsg); text != "" {
		query.Query = text
	}
	items, err := resolved.memoryStore.Search(ctx, query)
	if err == nil && len(items) == 0 && query.Query != "" {
		query.Query = ""
		items, err = resolved.memoryStore.Search(ctx, query)
	}
	if err != nil || len(items) == 0 {
		if err != nil {
			o.emitter.Emit(ctx, telemetry.Event{
				Type:     telemetry.MemoryRetrievalFailed,
				TenantID: resolved.scope.TenantID,
				Attrs:    map[string]any{"error": err.Error()},
			})
		}
		return ""
	}
	lines := []string{
		"Relevant memory for this turn:",
		"Use memory as preference/context only. If memory conflicts with current user input or tool results, prefer current input and tools.",
	}
	for _, item := range items {
		content := compactToolResultRaw(item.Content, 320)
		if content == "" {
			continue
		}
		subject := item.Subject
		if subject == "" {
			subject = "general"
		}
		lines = append(lines, fmt.Sprintf("- [%s/%s] %s", item.Scope, subject, content))
	}
	if len(lines) <= 2 {
		return ""
	}
	o.emitter.Emit(ctx, telemetry.Event{
		Type:     telemetry.MemoryRetrieved,
		TenantID: resolved.scope.TenantID,
		Attrs:    map[string]any{"count": len(lines) - 2},
	})
	return strings.Join(lines, "\n")
}

func memoryQueryText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 160 {
		return string(runes[:160])
	}
	return text
}

func (o *Orchestrator) captureExplicitMemory(ctx context.Context, resolved resolvedTurnContext, userMsg string) {
	if resolved.memoryStore == nil || resolved.scope.TenantID == "" {
		return
	}
	content, ok := explicitMemoryContent(userMsg)
	if !ok {
		return
	}
	item := kernel.MemoryItem{
		ID:      explicitMemoryID(resolved.scope, content),
		Scope:   explicitMemoryScope(resolved.scope),
		Subject: explicitMemorySubject(content),
		Content: content,
		Metadata: map[string]string{
			"source": "explicit_user_message",
		},
	}
	if err := resolved.memoryStore.Save(ctx, resolved.scope, item); err != nil {
		o.emitter.Emit(ctx, telemetry.Event{
			Type:     telemetry.MemorySaveFailed,
			TenantID: resolved.scope.TenantID,
			Attrs: map[string]any{
				"error": err.Error(),
				"scope": item.Scope,
			},
		})
		return
	}
	o.emitter.Emit(ctx, telemetry.Event{
		Type:     telemetry.MemorySaved,
		TenantID: resolved.scope.TenantID,
		Attrs: map[string]any{
			"scope":   item.Scope,
			"subject": item.Subject,
		},
	})
}

func (o *Orchestrator) forgetExplicitMemory(ctx context.Context, resolved resolvedTurnContext, userMsg string) {
	if resolved.memoryStore == nil || resolved.scope.TenantID == "" || resolved.scope.ActorID == "" {
		return
	}
	queryText, ok := explicitMemoryForgetQuery(userMsg)
	if !ok {
		return
	}
	items, err := resolved.memoryStore.Search(ctx, kernel.MemoryQuery{
		Scope: resolved.scope,
		Types: []kernel.MemoryScope{kernel.MemoryUser},
		Query: queryText,
		Limit: 5,
	})
	if err != nil {
		o.emitter.Emit(ctx, telemetry.Event{
			Type:     telemetry.MemoryDeleteFailed,
			TenantID: resolved.scope.TenantID,
			Attrs: map[string]any{
				"error": err.Error(),
				"scope": kernel.MemoryUser,
			},
		})
		return
	}
	deleted := 0
	for _, item := range items {
		if item.ID == "" || item.Scope != kernel.MemoryUser {
			continue
		}
		if err := resolved.memoryStore.Delete(ctx, resolved.scope, item.ID); err != nil {
			o.emitter.Emit(ctx, telemetry.Event{
				Type:     telemetry.MemoryDeleteFailed,
				TenantID: resolved.scope.TenantID,
				Attrs: map[string]any{
					"error": err.Error(),
					"scope": kernel.MemoryUser,
				},
			})
			continue
		}
		deleted++
	}
	if deleted > 0 {
		o.emitter.Emit(ctx, telemetry.Event{
			Type:     telemetry.MemoryDeleted,
			TenantID: resolved.scope.TenantID,
			Attrs: map[string]any{
				"scope": kernel.MemoryUser,
				"count": deleted,
			},
		})
	}
}

func explicitMemoryContent(text string) (string, bool) {
	normalized := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if normalized == "" {
		return "", false
	}
	lower := strings.ToLower(normalized)
	if strings.Contains(lower, "don't remember") ||
		strings.Contains(lower, "do not remember") ||
		strings.Contains(normalized, "不要记住") ||
		strings.Contains(normalized, "别记住") {
		return "", false
	}
	candidates := []string{
		"请帮我记住",
		"请记住",
		"帮我记住",
		"记住",
	}
	for _, marker := range candidates {
		if strings.HasPrefix(normalized, marker) {
			if content := cleanExplicitMemoryContent(normalized[len(marker):]); content != "" {
				return content, true
			}
		}
	}
	prefixes := []string{
		"please remember that ",
		"please remember ",
		"remember that ",
		"remember: ",
		"remember ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			if content := cleanExplicitMemoryContent(normalized[len(prefix):]); content != "" {
				return content, true
			}
		}
	}
	return "", false
}

func explicitMemoryForgetQuery(text string) (string, bool) {
	normalized := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if normalized == "" {
		return "", false
	}
	markers := []string{
		"请帮我忘记",
		"请忘记",
		"帮我忘记",
		"不要再记住",
		"别再记住",
		"不要记住",
		"别记住",
		"删除记忆",
		"清除记忆",
		"移除记忆",
		"忘记",
	}
	for _, marker := range markers {
		if strings.HasPrefix(normalized, marker) {
			if query := cleanExplicitMemoryContent(normalized[len(marker):]); managed_escrowForgetQuery(query) {
				return query, true
			}
			return "", false
		}
	}
	lower := strings.ToLower(normalized)
	prefixes := []string{
		"please forget that ",
		"please forget ",
		"forget that ",
		"forget: ",
		"forget ",
		"delete memory that ",
		"delete memory ",
		"remove memory that ",
		"remove memory ",
		"don't remember that ",
		"don't remember ",
		"do not remember that ",
		"do not remember ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			if query := cleanExplicitMemoryContent(normalized[len(prefix):]); managed_escrowForgetQuery(query) {
				return query, true
			}
			return "", false
		}
	}
	return "", false
}

func managed_escrowForgetQuery(query string) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return false
	}
	switch query {
	case "all", "everything", "anything", "全部", "所有", "全部内容", "所有内容":
		return false
	default:
		return true
	}
}

func cleanExplicitMemoryContent(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, ":：,，.。;； ")
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > 600 {
		text = string(runes[:600])
	}
	return text
}

func explicitMemoryScope(scope kernel.Scope) kernel.MemoryScope {
	if scope.ActorID != "" {
		return kernel.MemoryUser
	}
	return kernel.MemoryTenant
}

func explicitMemorySubject(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(content, "偏好") ||
		strings.Contains(content, "默认") ||
		strings.Contains(content, "喜欢") ||
		strings.Contains(lower, "prefer") ||
		strings.Contains(lower, "preference") ||
		strings.Contains(lower, "default") {
		return "preference"
	}
	return "user_note"
}

func explicitMemoryID(scope kernel.Scope, content string) string {
	key := strings.Join([]string{
		scope.TenantID,
		scope.StoreID,
		scope.ActorID,
		string(explicitMemoryScope(scope)),
		explicitMemorySubject(content),
		content,
	}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return "mem_" + hex.EncodeToString(sum[:])[:24]
}

func recalculateGrantedTools(ctx context.Context, resolved *resolvedTurnContext) error {
	if resolved == nil || resolved.toolCatalog == nil || len(resolved.activeSkills) == 0 {
		return nil
	}
	catalogTools, err := resolved.toolCatalog.List(ctx, resolved.scope)
	if err != nil {
		return err
	}
	allowedNames := map[string]struct{}{}
	resolved.grantedTools = map[string][]kernel.ToolMetadata{}
	for _, s := range resolved.activeSkills {
		if s == nil {
			continue
		}
		grant := kernel.ToolGrant{
			SkillID:      kernel.SkillID(s.ID),
			Capabilities: skillCapabilities(s),
			Persona:      turnPersona(resolved.scope, s),
		}
		granted := kernel.FilterToolsForGrant(catalogTools, grant)
		resolved.grantedTools[s.ID] = granted
		for _, tool := range granted {
			allowedNames[tool.Name] = struct{}{}
			rememberToolResultMode(resolved, tool)
		}
	}
	if len(allowedNames) == 0 {
		resolved.tools = appendRuntimeSkillTool(nil)
		return nil
	}
	resolved.tools = appendRuntimeSkillTool(filterToolDefs(resolved.baseTools, allowedNames))
	return nil
}

func compactToolResultForHistory(toolName, content, resultMode string, isError bool) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if isError {
		return compactToolResultRaw(content, toolResultExcerptMaxLen)
	}
	switch resultMode {
	case "redacted":
		return marshalCompactToolResult(map[string]any{
			"tool":       toolName,
			"resultMode": "redacted",
			"summary":    "Tool completed; result omitted from chat history because the catalog marks it redacted.",
		})
	case "summary":
		return summarizeToolResult(toolName, content, resultMode)
	}
	if len(content) > toolResultHistoryMaxLen {
		return summarizeToolResult(toolName, content, "summary")
	}
	return compactToolResultRaw(content, toolResultHistoryMaxLen)
}

func summarizeToolResult(toolName, content, resultMode string) string {
	summary := map[string]any{
		"tool":       toolName,
		"resultMode": resultMode,
		"summary":    "Tool result compacted for chat history; use an artifact/read tool if exact payload is required.",
	}
	var decoded any
	if err := json.Unmarshal([]byte(content), &decoded); err == nil {
		switch v := decoded.(type) {
		case map[string]any:
			redacted, _ := redactToolResultValue(v).(map[string]any)
			summary["type"] = "object"
			summary["keys"] = sortedMapKeys(redacted, 12)
			addToolResultDataShape(summary, redacted["data"])
			if compact, err := json.Marshal(redacted); err == nil {
				summary["excerpt"] = compactToolResultRaw(string(compact), toolResultExcerptMaxLen)
			}
		case []any:
			v, _ = redactToolResultValue(v).([]any)
			summary["type"] = "array"
			summary["itemCount"] = len(v)
			if compact, err := json.Marshal(v); err == nil {
				summary["excerpt"] = compactToolResultRaw(string(compact), toolResultExcerptMaxLen)
			}
		default:
			summary["type"] = "scalar"
			if compact, err := json.Marshal(decoded); err == nil {
				summary["excerpt"] = compactToolResultRaw(string(compact), toolResultExcerptMaxLen)
			}
		}
	} else {
		summary["type"] = "text"
		summary["excerpt"] = compactToolResultRaw(content, toolResultExcerptMaxLen)
	}
	return marshalCompactToolResult(summary)
}

func redactToolResultValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if redact.IsSensitiveKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactToolResultValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactToolResultValue(item)
		}
		return out
	default:
		return v
	}
}

func addToolResultDataShape(summary map[string]any, data any) {
	switch v := data.(type) {
	case []any:
		summary["dataType"] = "array"
		summary["dataItemCount"] = len(v)
	case map[string]any:
		summary["dataType"] = "object"
		summary["dataKeys"] = sortedMapKeys(v, 12)
	case nil:
	default:
		summary["dataType"] = fmt.Sprintf("%T", data)
	}
}

func sortedMapKeys(m map[string]any, limit int) []string {
	if len(m) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

func compactToolResultRaw(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || limit <= 0 {
		return ""
	}
	raw = redact.SanitizeEnvBlock(raw)
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.Join(strings.Fields(raw), " ")
	if len(raw) <= limit {
		return raw
	}
	if limit <= 3 {
		return raw[:limit]
	}
	return raw[:limit-3] + "..."
}

func marshalCompactToolResult(value map[string]any) string {
	out, err := json.Marshal(value)
	if err != nil {
		return `{"summary":"Tool result compacted for chat history."}`
	}
	return string(out)
}

func (o *Orchestrator) systemPromptWithSkills(resolved resolvedTurnContext) string {
	prompt := o.systemPrompt
	skillPrompt := agentskill.BuildPromptContextWithOptions(agentskill.PromptContextOptions{
		Available:    resolved.availableSkills,
		Active:       resolved.activeSkills,
		GrantedTools: promptToolsBySkill(resolved.grantedTools),
	})
	if skillPrompt == "" {
		return promptWithTurnContext(prompt, resolved.contextBlocks)
	}
	if prompt == "" {
		return promptWithTurnContext(skillPrompt, resolved.contextBlocks)
	}
	return promptWithTurnContext(prompt+"\n\n"+skillPrompt, resolved.contextBlocks)
}

func promptWithTurnContext(prompt string, blocks []string) string {
	if len(blocks) == 0 {
		return prompt
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block != "" {
			parts = append(parts, block)
		}
	}
	if len(parts) == 0 {
		return prompt
	}
	context := "## Turn Context\n" + strings.Join(parts, "\n\n")
	if strings.TrimSpace(prompt) == "" {
		return context
	}
	return prompt + "\n\n" + context
}

func skillCapabilities(s *agentskill.Skill) []kernel.Capability {
	if s == nil {
		return nil
	}
	raw := s.Capabilities()
	out := make([]kernel.Capability, 0, len(raw))
	for _, item := range raw {
		if item == "" {
			continue
		}
		out = append(out, kernel.Capability(item))
	}
	return out
}

func turnPersona(scope kernel.Scope, s *agentskill.Skill) kernel.Persona {
	if scope.ActingPersona != "" {
		return scope.ActingPersona
	}
	if s != nil && s.Persona() != "" {
		return kernel.Persona(s.Persona())
	}
	return ""
}

func filterToolDefs(tools []ToolDef, allowedNames map[string]struct{}) []ToolDef {
	out := make([]ToolDef, 0, len(tools))
	for _, tool := range tools {
		if _, ok := allowedNames[tool.Name]; ok {
			out = append(out, tool)
		}
	}
	return out
}

func promptToolsBySkill(granted map[string][]kernel.ToolMetadata) map[string][]agentskill.PromptTool {
	if len(granted) == 0 {
		return nil
	}
	out := make(map[string][]agentskill.PromptTool, len(granted))
	for skillID, tools := range granted {
		items := make([]agentskill.PromptTool, 0, len(tools))
		for _, tool := range tools {
			items = append(items, agentskill.PromptTool{
				Name:        tool.Name,
				Description: tool.Description,
				Risk:        string(tool.Risk),
				Approval:    string(tool.Approval),
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		out[skillID] = items
	}
	return out
}

func toolDefNameSet(tools []ToolDef) map[string]struct{} {
	out := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		out[tool.Name] = struct{}{}
	}
	return out
}

func appendRuntimeSkillTool(tools []ToolDef) []ToolDef {
	for _, tool := range tools {
		if tool.Name == runtimeUseSkillToolName {
			return tools
		}
	}
	return append(tools, ToolDef{
		Name:        runtimeUseSkillToolName,
		Description: "Load one available skill for the current response. Use when the user request clearly matches a skill or explicitly asks for a skill.",
		Schema:      `{"type":"object","properties":{"skill":{"type":"string","description":"Skill identifier, name, or path to load."}},"required":["skill"],"additionalProperties":false}`,
	})
}

func activateRuntimeSkillTool(ctx context.Context, resolved *resolvedTurnContext, arguments string) (string, error) {
	if resolved == nil || resolved.skillProvider == nil {
		return "", fmt.Errorf("skill provider is not available")
	}
	var req struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal([]byte(arguments), &req); err != nil {
		return "", fmt.Errorf("parse use_skill_tool arguments: %w", err)
	}
	req.Skill = strings.TrimSpace(req.Skill)
	if req.Skill == "" {
		return "", fmt.Errorf("skill is required")
	}
	s, err := resolved.skillProvider.Load(ctx, req.Skill)
	if err != nil {
		return "", err
	}
	if !containsSkillID(resolved.availableSkills, s.ID) {
		return "", fmt.Errorf("skill is not available for this turn")
	}
	if !hasActiveSkill(resolved.activeSkills, s.ID) {
		resolved.activeSkills = append(resolved.activeSkills, s)
	}
	if err := recalculateGrantedTools(ctx, resolved); err != nil {
		return "", err
	}
	return agentskill.BuildPromptContextWithOptions(agentskill.PromptContextOptions{
		Active:       []*agentskill.Skill{s},
		GrantedTools: promptToolsBySkill(resolved.grantedTools),
	}), nil
}

func hasActiveSkill(skills []*agentskill.Skill, id string) bool {
	for _, s := range skills {
		if s != nil && s.ID == id {
			return true
		}
	}
	return false
}

func containsSkillID(ids []string, id string) bool {
	for _, item := range ids {
		if item == id {
			return true
		}
	}
	return false
}

func (o *Orchestrator) runLoop(
	ctx context.Context,
	tenantID, threadID, turnID string,
	turnStartedAt time.Time,
	history []Message,
	resolved *resolvedTurnContext,
	out *stream.Buffered,
) {
	for round := 0; round < o.cfg.MaxToolRounds; round++ {
		tools := []ToolDef(nil)
		if resolved != nil {
			tools = resolved.tools
		}
		allowedTools := toolDefNameSet(tools)
		tokens := 0
		for _, m := range history {
			tokens += budget.EstimateTokens(m.Content)
		}
		decision := o.budget.Decide(tokens)
		if decision.ShouldShape {
			shaped := shapeReplayHistory(history, o.cfg.ShapeKeepMsgs)
			if len(shaped) < len(history) {
				history = shaped
				tokens = estimateMessagesTokens(history)
				decision = o.budget.Decide(tokens)
				o.emitter.Emit(ctx, telemetry.Event{
					Type:     telemetry.ReplayShaped,
					TenantID: tenantID,
					ThreadID: threadID,
					Attrs: map[string]any{
						"estimated": decision.Estimated,
						"kept_msgs": len(history),
					},
				})
			}
		}
		if decision.ShouldCompact {
			o.emitter.Emit(ctx, telemetry.Event{
				Type:     telemetry.CompactionStarted,
				TenantID: tenantID,
				ThreadID: threadID,
				Attrs: map[string]any{
					"estimated": decision.Estimated,
				},
			})
			compacted := compactReplayHistory(history, o.cfg.ShapeKeepMsgs)
			if len(compacted) < len(history) {
				history = compacted
				tokens = estimateMessagesTokens(history)
				decision = o.budget.Decide(tokens)
				o.emitter.Emit(ctx, telemetry.Event{
					Type:     telemetry.CompactionSucceeded,
					TenantID: tenantID,
					ThreadID: threadID,
					Attrs: map[string]any{
						"estimated": decision.Estimated,
						"kept_msgs": len(history),
					},
				})
			} else {
				o.emitter.Emit(ctx, telemetry.Event{
					Type:     telemetry.CompactionFailed,
					TenantID: tenantID,
					ThreadID: threadID,
					Attrs:    map[string]any{"reason": "no compactable replay messages"},
				})
			}
		}

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

		llmStream, err := o.callLLMWithRetry(ctx, history, tools)
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

		skillRoutingBatch := hasRuntimeSkillToolCall(toolCalls)
		execCalls := make([]exec.ToolCall, 0, len(toolCalls))
		toolNames := make(map[string]string, len(toolCalls))
		for _, tc := range toolCalls {
			if tc.Name == runtimeUseSkillToolName {
				out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
					ID:     tc.ID,
					Name:   tc.Name,
					Status: "executing",
				}})
				content, err := activateRuntimeSkillTool(ctx, resolved, tc.Arguments)
				status := "done"
				if err != nil {
					status = "error"
					content = fmt.Sprintf(`{"error":%q}`, err.Error())
				}
				out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
					ID:     tc.ID,
					Name:   tc.Name,
					Status: status,
				}})
				history = append(history, Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
				})
				if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
					ID:         newMessageID(),
					TenantID:   tenantID,
					ThreadID:   threadID,
					TurnID:     turnID,
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
					Tokens:     budget.EstimateTokens(content),
					Bytes:      len(content),
					CreatedAt:  time.Now(),
				}); err != nil {
					out.SendError(err)
					return
				}
				continue
			}
			if skillRoutingBatch {
				out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
					ID:     tc.ID,
					Name:   tc.Name,
					Status: "error",
				}})
				content := fmt.Sprintf(`{"error":%q}`, "ordinary tools must be retried after skill routing completes")
				history = append(history, Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
				})
				if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
					ID:         newMessageID(),
					TenantID:   tenantID,
					ThreadID:   threadID,
					TurnID:     turnID,
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
					Tokens:     budget.EstimateTokens(content),
					Bytes:      len(content),
					CreatedAt:  time.Now(),
				}); err != nil {
					out.SendError(err)
					return
				}
				continue
			}
			if _, ok := allowedTools[tc.Name]; !ok {
				out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
					ID:     tc.ID,
					Name:   tc.Name,
					Status: "error",
				}})
				content := fmt.Sprintf(`{"error":%q}`, "tool is not authorized for this turn")
				history = append(history, Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
				})
				if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
					ID:         newMessageID(),
					TenantID:   tenantID,
					ThreadID:   threadID,
					TurnID:     turnID,
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
					Tokens:     budget.EstimateTokens(content),
					Bytes:      len(content),
					CreatedAt:  time.Now(),
				}); err != nil {
					out.SendError(err)
					return
				}
				continue
			}
			if content, approval, ok, err := approvalRequiredToolResult(resolved, tenantID, threadID, turnID, tc); ok {
				status := "approval_required"
				if err != nil {
					status = "error"
					content = fmt.Sprintf(`{"error":%q}`, err.Error())
				} else if err := o.saveApproval(ctx, approval); err != nil {
					status = "error"
					content = fmt.Sprintf(`{"error":%q}`, err.Error())
				}
				toolEvent := &stream.ToolEvent{
					ID:     tc.ID,
					Name:   tc.Name,
					Status: status,
				}
				if status == "approval_required" {
					toolEvent.Result = json.RawMessage(content)
				}
				out.Send(stream.Chunk{ToolEvent: toolEvent})
				history = append(history, Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
				})
				if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
					ID:         newMessageID(),
					TenantID:   tenantID,
					ThreadID:   threadID,
					TurnID:     turnID,
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
					Tokens:     budget.EstimateTokens(content),
					Bytes:      len(content),
					CreatedAt:  time.Now(),
				}); err != nil {
					out.SendError(err)
					return
				}
				continue
			}
			execCalls = append(execCalls, exec.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
			toolNames[tc.ID] = tc.Name
			out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
				ID:     tc.ID,
				Name:   tc.Name,
				Status: "executing",
			}})
		}
		if len(execCalls) == 0 {
			continue
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
				"count":       len(execCalls),
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
			content := compactToolResultForHistory(toolName, r.Content, toolResultMode(resolved, toolName), r.IsError)
			out.Send(stream.Chunk{ToolEvent: &stream.ToolEvent{
				ID:     r.CallID,
				Name:   toolName,
				Status: status,
			}})
			history = append(history, Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: r.CallID,
			})
			if err := o.saveMessage(ctx, tenantID, threadID, &store.Message{
				ID:         newMessageID(),
				TenantID:   tenantID,
				ThreadID:   threadID,
				TurnID:     turnID,
				Role:       "tool",
				Content:    content,
				ToolCallID: r.CallID,
				Tokens:     budget.EstimateTokens(content),
				Bytes:      len(content),
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

func estimateMessagesTokens(messages []Message) int {
	tokens := 0
	for _, m := range messages {
		tokens += budget.EstimateTokens(m.Content)
	}
	return tokens
}

func shapeReplayHistory(history []Message, keepLast int) []Message {
	if keepLast <= 0 || len(history) <= keepLast {
		return history
	}
	system := firstSystemMessage(history)
	tailStart := managed_escrowReplayTailStart(history, len(history)-keepLast)
	tail := append([]Message(nil), history[tailStart:]...)
	if system == nil || (len(tail) > 0 && tail[0].Role == "system") {
		return tail
	}
	return append([]Message{*system}, tail...)
}

func compactReplayHistory(history []Message, keepLast int) []Message {
	if keepLast <= 0 || len(history) <= keepLast+2 {
		return history
	}
	system := firstSystemMessage(history)
	start := 0
	if system != nil {
		start = 1
	}
	prefixEnd := managed_escrowReplayTailStart(history, len(history)-keepLast)
	if prefixEnd <= start {
		return history
	}
	summary := summarizeReplayMessages(history[start:prefixEnd])
	tail := append([]Message(nil), history[prefixEnd:]...)
	out := make([]Message, 0, len(tail)+2)
	if system != nil {
		out = append(out, *system)
	}
	out = append(out, Message{
		Role:    "system",
		Content: summary,
	})
	out = append(out, tail...)
	return out
}

func managed_escrowReplayTailStart(history []Message, desiredStart int) int {
	if desiredStart <= 0 {
		return 0
	}
	if desiredStart >= len(history) {
		return len(history)
	}
	start := desiredStart
	for start > 0 && history[start].Role == "tool" {
		start--
	}
	if start > 0 && history[start-1].Role == "assistant" && len(history[start-1].ToolCalls) > 0 {
		start--
	}
	return start
}

func firstSystemMessage(history []Message) *Message {
	if len(history) == 0 || history[0].Role != "system" {
		return nil
	}
	msg := history[0]
	return &msg
}

func summarizeReplayMessages(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(messages)+1)
	parts = append(parts, fmt.Sprintf("Earlier conversation compacted deterministically (%d messages). Recent messages below are authoritative.", len(messages)))
	for i, msg := range messages {
		if i >= 12 {
			parts = append(parts, fmt.Sprintf("... %d more compacted messages omitted.", len(messages)-i))
			break
		}
		content := compactToolResultRaw(msg.Content, 220)
		if content == "" && len(msg.ToolCalls) > 0 {
			content = fmt.Sprintf("%d tool call(s)", len(msg.ToolCalls))
		}
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("- %s: %s", msg.Role, content))
	}
	return strings.Join(parts, "\n")
}

func approvalRequiredToolResult(resolved *resolvedTurnContext, tenantID, threadID, turnID string, call stream.ToolCall) (string, *store.Approval, bool, error) {
	tool, skillID, ok := grantedToolMetadata(resolved, call.Name)
	if !ok || tool.Approval != kernel.ApprovalExplicit {
		return "", nil, false, nil
	}
	payload := toolCallPayload(call.Arguments)
	artifactIDs := approvalArtifactIDsFromPayload(payload)
	artifactIDsJSON := marshalApprovalArtifactIDs(artifactIDs)
	createdAt := time.Now()
	req := kernel.ApprovalRequest{
		ID:             newApprovalID(),
		SkillID:        kernel.SkillID(skillID),
		Scope:          resolved.scope,
		Risk:           tool.Risk,
		Action:         call.Name,
		Summary:        fmt.Sprintf("Approval required to run %s", call.Name),
		Payload:        payload,
		IdempotencyKey: fmt.Sprintf("%s:%s:%s", threadID, turnID, call.ID),
		CreatedAt:      createdAt,
	}
	hash, err := kernel.ComputeApprovalHash(req)
	if err != nil {
		return "", nil, true, fmt.Errorf("compute approval hash: %w", err)
	}
	req.RequestHash = hash
	approval := &store.Approval{
		ID:             req.ID,
		TenantID:       tenantID,
		ThreadID:       threadID,
		TurnID:         turnID,
		ToolCallID:     call.ID,
		SkillID:        string(req.SkillID),
		StoreID:        req.Scope.StoreID,
		ActorID:        req.Scope.ActorID,
		ActingPersona:  string(req.Scope.ActingPersona),
		Risk:           string(req.Risk),
		Action:         req.Action,
		Summary:        req.Summary,
		Payload:        string(payload),
		ArtifactIDs:    artifactIDsJSON,
		RequestHash:    req.RequestHash,
		IdempotencyKey: req.IdempotencyKey,
		Status:         store.ApprovalStatusPending,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	data, err := json.Marshal(struct {
		Status      string                 `json:"status"`
		Message     string                 `json:"message"`
		ArtifactIDs []string               `json:"artifactIds,omitempty"`
		Approval    kernel.ApprovalRequest `json:"approval"`
	}{
		Status:      "approval_required",
		Message:     "This tool requires explicit approval before execution.",
		ArtifactIDs: artifactIDs,
		Approval:    req,
	})
	if err != nil {
		return "", nil, true, fmt.Errorf("marshal approval request: %w", err)
	}
	return string(data), approval, true, nil
}

func grantedToolMetadata(resolved *resolvedTurnContext, toolName string) (kernel.ToolMetadata, string, bool) {
	if resolved == nil {
		return kernel.ToolMetadata{}, "", false
	}
	var fallback kernel.ToolMetadata
	var fallbackSkill string
	var found bool
	for skillID, tools := range resolved.grantedTools {
		for _, tool := range tools {
			if tool.Name != toolName {
				continue
			}
			if tool.Approval == kernel.ApprovalExplicit {
				return tool, skillID, true
			}
			if !found {
				fallback = tool
				fallbackSkill = skillID
				found = true
			}
		}
	}
	return fallback, fallbackSkill, found
}

func toolCallPayload(arguments string) json.RawMessage {
	args := strings.TrimSpace(arguments)
	if args == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(args)) {
		return json.RawMessage(args)
	}
	data, err := json.Marshal(map[string]string{"arguments": arguments})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

func approvalArtifactIDsFromPayload(payload json.RawMessage) []string {
	if len(payload) == 0 || !json.Valid(payload) {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil
	}
	var out []string
	collectApprovalArtifactIDs(decoded, "", &out)
	return uniqueStringList(out, 20)
}

func collectApprovalArtifactIDs(value any, key string, out *[]string) {
	switch v := value.(type) {
	case map[string]any:
		for childKey, childValue := range v {
			collectApprovalArtifactIDs(childValue, childKey, out)
		}
	case []any:
		if isArtifactIDKey(key) {
			for _, item := range v {
				if s, ok := item.(string); ok {
					*out = append(*out, s)
				}
			}
			return
		}
		for _, item := range v {
			collectApprovalArtifactIDs(item, key, out)
		}
	case string:
		if isArtifactIDKey(key) {
			*out = append(*out, v)
		}
	}
}

func isArtifactIDKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	switch normalized {
	case "artifactid",
		"artifactids",
		"sourceartifactid",
		"sourceartifactids",
		"proposalartifactid",
		"proposalartifactids":
		return true
	default:
		return false
	}
}

func uniqueStringList(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func marshalApprovalArtifactIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(data)
}

func hasRuntimeSkillToolCall(toolCalls []stream.ToolCall) bool {
	for _, tc := range toolCalls {
		if tc.Name == runtimeUseSkillToolName {
			return true
		}
	}
	return false
}

// callLLMWithRetry wraps the LLM call with simple retry logic for transient errors.
func (o *Orchestrator) callLLMWithRetry(ctx context.Context, history []Message, tools []ToolDef) (stream.Stream, error) {
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
		s, err := o.llm.ChatStream(ctx, history, tools)
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

func (o *Orchestrator) saveApproval(ctx context.Context, approval *store.Approval) error {
	if approval == nil || o.persist == nil {
		return nil
	}
	if err := o.persist.SaveApproval(ctx, approval); err != nil {
		return fmt.Errorf("agent runtime: save approval: %w", err)
	}
	return nil
}

func newTurnID() string {
	return "turn_" + uuid.NewString()
}

func newMessageID() string {
	return "msg_" + uuid.NewString()
}

func newApprovalID() string {
	return "appr_" + uuid.NewString()
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
