//go:build !private_distribution

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	agentexec "github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentruntime "github.com/mobazha/mobazha3.0/pkg/agent/runtime"
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	agentstream "github.com/mobazha/mobazha3.0/pkg/agent/stream"
	"github.com/mobazha/mobazha3.0/pkg/agent/telemetry"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type agentChatRuntimeCacheEntry struct {
	fingerprint string
	orch        *agentruntime.Orchestrator
}

type agentToolContext struct {
	baseURL   string
	authToken string
}

type agentToolContextKey struct{}

var agentChatRuntimes sync.Map

// handlePOSTAgentChat handles POST /v1/agent/chat — the Orchestrator-backed
// seller AI chat endpoint.
func (g *Gateway) handlePOSTAgentChat(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available in this mode")
		return
	}

	var req aipkg.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "message is required")
		return
	}
	if len(req.Message) > aipkg.MaxUserMessageLen {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			fmt.Sprintf("message too long (max %d characters)", aipkg.MaxUserMessageLen))
		return
	}

	cfg, err := p.AIConfigForChat(nil)
	if err != nil {
		if errors.Is(err, aipkg.ErrVisionNotConfigured) || errors.Is(err, aipkg.ErrVisionUnsupported) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "AI chat model is not configured for this input")
			return
		}
		aiLog.Warningf("Agent chat config resolution failed: %v", err)
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "AI is not configured")
		return
	}
	if !cfg.IsValid() {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "AI is not configured. Please set up an AI provider in Settings > Integrations.")
		return
	}

	nodeID := getIdentityService(r).GetNodeID()
	if cfg.IsPlatform {
		if rl := p.AIRateLimiter(); rl != nil {
			if ok, _ := rl.Allow(nodeID, cfg.DailyLimit); !ok {
				responsePkg.Error(w, http.StatusTooManyRequests, "RATE_LIMITED",
					"Daily AI limit reached. Configure your own API key in Settings > Integrations for unlimited usage.")
				return
			}
		}
	}

	tenantID := nodeID
	if tenantID == "" {
		tenantID = p.ProfileName()
	}

	streamKey := "agent-chat-" + tenantID + ":" + p.ProfileName()
	if _, loaded := activeAIStreams.LoadOrStore(streamKey, true); loaded {
		responsePkg.Error(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Another AI chat request is still in progress. Please wait.")
		return
	}
	defer activeAIStreams.Delete(streamKey)

	threadID := req.SessionID
	if threadID == "" {
		threadID = uuid.New().String()
	}

	persist := p.AgentStore()

	systemPrompt := aipkg.BuildSystemPrompt(aipkg.UserRoleSeller, p.ProfileName(), req.Context)
	if catalogCtx := getCachedCatalog(tenantID, p); catalogCtx != "" {
		systemPrompt += "\n\n" + catalogCtx
	}

	orch := getAgentChatOrchestrator(tenantID+":"+p.ProfileName(), cfg, systemPrompt, p.AIProxy(), persist)

	flusher, ok := w.(http.Flusher)
	if !ok {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	emitSSE := func(event aipkg.SSEEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		eventType := event.Type
		if eventType == "" {
			eventType = "message"
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
		flusher.Flush()
	}

	turnCtx := context.WithValue(r.Context(), agentToolContextKey{}, agentToolContext{
		baseURL:   getLocalAPIURL(r),
		authToken: getAuthToken(r),
	})
	turnOptions, err := agentChatTurnOptions(turnCtx, req, tenantID, nodeID, p.ProfileName())
	if err != nil {
		aiLog.Warningf("Agent chat skill routing failed: %v", err)
		emitSSE(aipkg.SSEEvent{Type: aipkg.SSETypeError, Error: agentChatRouteErrorMessage(err)})
		return
	}
	result, err := orch.RunTurnWithOptions(turnCtx, tenantID, threadID, req.Message, turnOptions)
	if err != nil {
		aiLog.Warningf("Agent chat turn failed before streaming: %v", err)
		emitSSE(aipkg.SSEEvent{Type: aipkg.SSETypeError, Error: "AI assistant failed to start the request"})
		return
	}

	for {
		chunk := result.Output.Next()
		if chunk == nil {
			break
		}
		if chunk.Error != nil {
			aiLog.Warningf("Agent chat stream error: %v", chunk.Error)
			emitSSE(aipkg.SSEEvent{Type: aipkg.SSETypeError, Error: "AI assistant failed to complete the request"})
			return
		}
		if chunk.ToolEvent != nil {
			eventType := aipkg.SSETypeToolResult
			if chunk.ToolEvent.Status == "executing" {
				eventType = aipkg.SSETypeToolCall
			}
			event := aipkg.SSEEvent{
				Type:   eventType,
				Tool:   chunk.ToolEvent.Name,
				ToolID: chunk.ToolEvent.ID,
			}
			if chunk.ToolEvent.Status == "error" {
				event.Error = "tool execution failed"
			}
			if len(chunk.ToolEvent.Result) > 0 {
				var result any
				if err := json.Unmarshal(chunk.ToolEvent.Result, &result); err == nil {
					event.Result = result
				}
			}
			emitSSE(event)
		}
		if chunk.Delta != "" {
			emitSSE(aipkg.SSEEvent{Type: aipkg.SSETypeContent, Content: chunk.Delta})
		}
	}
	if err := result.Output.Err(); err != nil {
		aiLog.Warningf("Agent chat stream error: %v", err)
		emitSSE(aipkg.SSEEvent{Type: aipkg.SSETypeError, Error: "AI assistant failed to complete the request"})
		return
	}

	if cfg.IsPlatform {
		if rl := p.AIRateLimiter(); rl != nil {
			rl.Increment(nodeID)
		}
	}

	if err := updateAgentChatThreadMetadata(r.Context(), persist, tenantID, threadID, req.Message); err != nil {
		aiLog.Errorf("agent chat update thread metadata: %s", err)
	}

	emitSSE(aipkg.SSEEvent{
		Type:      aipkg.SSETypeDone,
		SessionID: threadID,
	})
}

func updateAgentChatThreadMetadata(ctx context.Context, persist agentstore.Persistence, tenantID, threadID, firstMessage string) error {
	if persist == nil {
		return nil
	}
	thread, err := persist.LoadThread(ctx, tenantID, threadID)
	if err != nil {
		return err
	}
	thread.Persona = string(aipkg.UserRoleSeller)
	if thread.Title == "" {
		thread.Title = generateSessionTitle(firstMessage)
	}
	thread.LastActive = time.Now()
	return persist.SaveThread(ctx, thread)
}

func agentChatTurnOptions(ctx context.Context, req aipkg.ChatRequest, tenantID, actorID, storeID string) (agentruntime.TurnOptions, error) {
	skillProvider, err := agentChatSkillProvider(ctx)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	skillFilter := agentskill.Filter{Persona: string(kernel.PersonaSeller)}
	requestedSkills, err := requestedAgentSkills(ctx, skillProvider, req, skillFilter)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	return agentruntime.TurnOptions{
		SkillProvider:   skillProvider,
		RequestedSkills: requestedSkills,
		SkillFilter:     skillFilter,
		ToolCatalog:     kernel.NewStaticToolCatalog(aipkg.SellerToolMetadata()),
		Scope: kernel.Scope{
			TenantID:      tenantID,
			StoreID:       storeID,
			ActorID:       actorID,
			ActorRoles:    []kernel.Persona{kernel.PersonaSeller},
			ActingPersona: kernel.PersonaSeller,
		},
	}, nil
}

func agentChatRouteErrorMessage(err error) string {
	if err == nil {
		return "AI assistant failed to route the request"
	}
	if strings.Contains(err.Error(), "MOBAZHA_AGENT_SKILLS_DIR") {
		return "AI assistant requires private skill configuration (MOBAZHA_AGENT_SKILLS_DIR)"
	}
	return "AI assistant failed to route the request"
}

func agentChatSkillProvider(ctx context.Context) (agentskill.Provider, error) {
	dir := strings.TrimSpace(os.Getenv("MOBAZHA_AGENT_SKILLS_DIR"))
	if dir == "" {
		return nil, fmt.Errorf("MOBAZHA_AGENT_SKILLS_DIR is required for agent chat")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("MOBAZHA_AGENT_SKILLS_DIR is not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("MOBAZHA_AGENT_SKILLS_DIR must point to a directory")
	}
	provider := agentskill.NewFilesystemProvider(dir)
	ids, err := provider.List(ctx, agentskill.Filter{Persona: string(kernel.PersonaSeller)})
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("MOBAZHA_AGENT_SKILLS_DIR has no seller skills")
	}
	return provider, nil
}

func requestedAgentSkills(ctx context.Context, provider agentskill.Provider, req aipkg.ChatRequest, filter agentskill.Filter) ([]string, error) {
	if provider == nil {
		return nil, nil
	}
	text := req.Message
	if req.Context != nil {
		text += "\n" + req.Context.CurrentPage
	}
	decision, err := agentskill.NewSkillRouter(provider).Route(ctx, agentskill.RouteInput{
		Text:   text,
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}
	return decision.RequestedSkills, nil
}

func getAgentChatOrchestrator(cacheKey string, cfg aipkg.Config, systemPrompt string, proxy *aipkg.Proxy, persist agentstore.Persistence) *agentruntime.Orchestrator {
	fingerprint := agentChatConfigFingerprint(cfg, systemPrompt)
	if cached, ok := agentChatRuntimes.Load(cacheKey); ok {
		entry := cached.(*agentChatRuntimeCacheEntry)
		if entry.fingerprint == fingerprint {
			return entry.orch
		}
	}

	orch := agentruntime.NewOrchestrator(
		agentChatLLMClient{proxy: proxy, cfg: cfg},
		budget.NewCalculator(budget.DefaultConfig()),
		agentexec.NewBatchExecutor(agentChatToolExecutor{}, 30*time.Second, 4),
		persist,
		telemetry.NewLogEmitter(nil),
		&agentruntime.Config{MaxToolRounds: 5, TurnTimeout: aipkg.StreamTimeout, MaxHistoryMsgs: aipkg.MaxSessionMessages},
	)
	orch.SetSystemPrompt(systemPrompt)
	orch.RegisterTools(agentToolDefs(aipkg.SellerTools()))

	entry := &agentChatRuntimeCacheEntry{fingerprint: fingerprint, orch: orch}
	agentChatRuntimes.Store(cacheKey, entry)
	return orch
}

func forgetAgentChatThread(cacheKey, tenantID, threadID string) {
	if cached, ok := agentChatRuntimes.Load(cacheKey); ok {
		if entry, ok := cached.(*agentChatRuntimeCacheEntry); ok && entry.orch != nil {
			entry.orch.ForgetThread(tenantID, threadID)
		}
	}
}

func agentChatConfigFingerprint(cfg aipkg.Config, systemPrompt string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(cfg.Provider))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.EffectiveModel()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.EffectiveBaseURL()))
	_, _ = h.Write([]byte{0})
	keyHash := sha256.Sum256([]byte(cfg.APIKey))
	_, _ = h.Write(keyHash[:])
	_, _ = h.Write([]byte{0})
	if cfg.Enabled {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
	if cfg.IsPlatform {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte(fmt.Sprintf("%d", cfg.DailyLimit)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(systemPrompt))
	return hex.EncodeToString(h.Sum(nil))
}

type agentChatLLMClient struct {
	proxy *aipkg.Proxy
	cfg   aipkg.Config
}

func (c agentChatLLMClient) ChatStream(ctx context.Context, messages []agentruntime.Message, tools []agentruntime.ToolDef) (agentstream.Stream, error) {
	deltas, err := c.proxy.StreamChat(ctx, c.cfg, agentChatMessages(messages), agentChatTools(tools))
	if err != nil {
		return nil, err
	}

	out := agentstream.NewBuffered(ctx, 64)
	go func() {
		defer out.Finish()
		for delta := range deltas {
			if delta.Error != "" {
				out.SendError(fmt.Errorf("%s", delta.Error))
				return
			}
			chunk := agentstream.Chunk{Delta: delta.Content}
			if len(delta.ToolCalls) > 0 {
				chunk.ToolCalls = agentToolCalls(delta.ToolCalls)
			}
			if chunk.Delta != "" || len(chunk.ToolCalls) > 0 {
				out.Send(chunk)
			}
			if delta.Done {
				return
			}
		}
	}()
	return out, nil
}

type agentChatToolExecutor struct{}

func (agentChatToolExecutor) Execute(ctx context.Context, call agentexec.ToolCall) (agentexec.ToolResult, error) {
	toolCtx, _ := ctx.Value(agentToolContextKey{}).(agentToolContext)
	executor := aipkg.NewToolExecutor(toolCtx.baseURL, toolCtx.authToken)
	result, err := executor.Execute(ctx, call.Name, call.Arguments)
	if err != nil {
		return agentexec.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf(`{"error":%q}`, "tool execution failed"),
			IsError: true,
		}, err
	}
	return agentexec.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: result,
	}, nil
}

func agentToolDefs(tools []aipkg.ToolDefinition) []agentruntime.ToolDef {
	out := make([]agentruntime.ToolDef, len(tools))
	for i, tool := range tools {
		out[i] = agentruntime.ToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			Schema:      string(tool.Parameters),
		}
	}
	return out
}

func agentChatTools(tools []agentruntime.ToolDef) []aipkg.ToolDefinition {
	out := make([]aipkg.ToolDefinition, len(tools))
	for i, tool := range tools {
		out[i] = aipkg.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  json.RawMessage(tool.Schema),
		}
	}
	return out
}

func agentChatMessages(messages []agentruntime.Message) []aipkg.ChatMsg {
	out := make([]aipkg.ChatMsg, len(messages))
	for i, msg := range messages {
		out[i] = aipkg.ChatMsg{
			Role:       aipkg.ChatRole(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  agentChatToolCalls(msg.ToolCalls),
		}
	}
	return out
}

func agentChatToolCalls(calls []agentstream.ToolCall) []aipkg.ToolCall {
	out := make([]aipkg.ToolCall, len(calls))
	for i, call := range calls {
		out[i] = aipkg.ToolCall{
			ID:   call.ID,
			Type: "function",
			Function: aipkg.ToolCallFunc{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		}
	}
	return out
}

func agentToolCalls(calls []aipkg.ToolCall) []agentstream.ToolCall {
	out := make([]agentstream.ToolCall, len(calls))
	for i, call := range calls {
		out[i] = agentstream.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		}
	}
	return out
}
