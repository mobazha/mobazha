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
	"sort"
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
	"github.com/mobazha/mobazha3.0/pkg/redact"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type agentChatRuntimeCacheEntry struct {
	fingerprint string
	orch        *agentruntime.Orchestrator
}

type agentToolContext struct {
	baseURL     string
	authToken   string
	attachments []aipkg.ChatAttachment
	provider    aiChatProvider
	origin      string
	tenantID    string
	actorID     string
}

type agentToolContextKey struct{}

var agentChatRuntimes sync.Map

const (
	agentChatMaxContextArtifacts   = 10
	agentChatMaxContextSkillRuns   = 3
	agentChatMaxContextAttachments = 10
	agentChatSkillRunArtifactMax   = 15
	agentChatArtifactDataMaxLen    = 1200
	agentChatSkillRunDataMaxLen    = 480
	agentChatAttachmentTextMaxLen  = 1200
	agentChatMaxRequestBytes       = 16 << 20
)

const agentChatThreadCompactionPrompt = `Summarize the earlier part of this agent conversation for future context replay.

Rules:
- Preserve durable user preferences, store facts, task state, decisions, open questions, and constraints.
- Preserve references to tools, artifacts, approvals, listings, orders, and skill runs when they affect future turns.
- Do not invent facts, do not execute actions, and do not include secrets.
- Output only the summary in concise plain text.`

// handlePOSTAgentChat handles POST /v1/agent/chat — the Orchestrator-backed
// seller AI chat endpoint.
func (g *Gateway) handlePOSTAgentChat(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available in this mode")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, agentChatMaxRequestBytes)

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

	origin := publicRequestOrigin(r)
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
		baseURL:     getLocalAPIURL(r),
		authToken:   getAuthToken(r),
		attachments: agentChatContextAttachments(req.Context),
		provider:    p,
		origin:      origin,
		tenantID:    tenantID,
		actorID:     nodeID,
	})
	turnOptions, err := agentChatTurnOptions(turnCtx, persist, req, tenantID, nodeID, p.ProfileName())
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
			emitSSE(agentChatSSEEventFromToolEvent(chunk.ToolEvent))
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

func agentChatSSEEventFromToolEvent(toolEvent *agentstream.ToolEvent) aipkg.SSEEvent {
	eventType := aipkg.SSETypeToolResult
	if toolEvent.Status == "executing" {
		eventType = aipkg.SSETypeToolCall
	}
	event := aipkg.SSEEvent{
		Type:   eventType,
		Tool:   toolEvent.Name,
		ToolID: toolEvent.ID,
	}
	if len(toolEvent.Result) > 0 {
		var result any
		if err := json.Unmarshal(toolEvent.Result, &result); err == nil {
			event.Result = result
		}
	}
	if toolEvent.Status == "error" && event.Result == nil {
		event.Result = map[string]any{"error": "tool execution failed"}
	}
	return event
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

func agentChatTurnOptions(ctx context.Context, persist agentstore.Persistence, req aipkg.ChatRequest, tenantID, actorID, storeID string) (agentruntime.TurnOptions, error) {
	skillProvider, err := agentChatSkillProvider(ctx)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	skillFilter := agentskill.Filter{Persona: string(kernel.PersonaSeller)}
	requestedSkills, err := requestedAgentSkills(ctx, skillProvider, req, skillFilter)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	referencedRuns, err := agentChatReferencedSkillRuns(ctx, persist, tenantID, req.Context)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	requestedSkills = agentChatRequestedSkillsWithRuns(requestedSkills, referencedRuns)
	contextBlocks, err := agentChatContextBlocks(ctx, persist, tenantID, req.Context, referencedRuns)
	if err != nil {
		return agentruntime.TurnOptions{}, err
	}
	return agentruntime.TurnOptions{
		SkillProvider:   skillProvider,
		RequestedSkills: requestedSkills,
		SkillFilter:     skillFilter,
		ContextBlocks:   contextBlocks,
		ToolCatalog:     kernel.NewStaticToolCatalog(aipkg.SellerToolMetadata()),
		MemoryStore:     agentChatKernelMemoryStore(persist),
		Scope: kernel.Scope{
			TenantID:      tenantID,
			StoreID:       storeID,
			ActorID:       actorID,
			ActorRoles:    []kernel.Persona{kernel.PersonaSeller},
			ActingPersona: kernel.PersonaSeller,
		},
	}, nil
}

func agentChatKernelMemoryStore(persist agentstore.Persistence) kernel.MemoryStore {
	if memoryStore, ok := persist.(kernel.MemoryStore); ok {
		return memoryStore
	}
	return nil
}

func agentChatReferencedSkillRuns(ctx context.Context, persist agentstore.Persistence, tenantID string, chatCtx *aipkg.ChatContext) ([]*agentstore.SkillRun, error) {
	runIDs, err := agentChatContextSkillRunIDs(chatCtx)
	if err != nil {
		return nil, err
	}
	if len(runIDs) == 0 {
		return nil, nil
	}
	if persist == nil {
		return nil, fmt.Errorf("skill run context requires agent store")
	}
	runs := make([]*agentstore.SkillRun, 0, len(runIDs))
	for _, runID := range runIDs {
		run, err := persist.LoadSkillRun(ctx, tenantID, runID)
		if errors.Is(err, agentstore.ErrSkillRunNotFound) {
			return nil, fmt.Errorf("skill run %q not found", runID)
		}
		if err != nil {
			return nil, fmt.Errorf("load skill run %q: %w", runID, err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func agentChatRequestedSkillsWithRuns(requested []string, runs []*agentstore.SkillRun) []string {
	out := append([]string(nil), requested...)
	seen := map[string]struct{}{}
	for _, skillID := range out {
		skillID = strings.TrimSpace(skillID)
		if skillID != "" {
			seen[skillID] = struct{}{}
		}
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		skillID := strings.TrimSpace(run.SkillID)
		if skillID == "" {
			continue
		}
		if _, ok := seen[skillID]; ok {
			continue
		}
		seen[skillID] = struct{}{}
		out = append(out, skillID)
	}
	return out
}

func agentChatContextBlocks(ctx context.Context, persist agentstore.Persistence, tenantID string, chatCtx *aipkg.ChatContext, referencedRuns []*agentstore.SkillRun) ([]string, error) {
	var blocks []string
	artifactBlocks, err := agentChatArtifactContextBlocks(ctx, persist, tenantID, chatCtx)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, artifactBlocks...)
	attachmentBlocks, err := agentChatAttachmentContextBlocks(chatCtx)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, attachmentBlocks...)
	runBlocks, err := agentChatSkillRunContextBlocks(ctx, persist, tenantID, referencedRuns)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, runBlocks...)
	return blocks, nil
}

func agentChatAttachmentContextBlocks(chatCtx *aipkg.ChatContext) ([]string, error) {
	if chatCtx == nil || len(chatCtx.Attachments) == 0 {
		return nil, nil
	}
	if len(chatCtx.Attachments) > agentChatMaxContextAttachments {
		return nil, fmt.Errorf("too many attachments (max %d)", agentChatMaxContextAttachments)
	}
	lines := make([]string, 0, len(chatCtx.Attachments)*7+3)
	lines = append(lines, "Attached files for this turn:")
	lines = append(lines, "The user attached these files with the current message. Attachment presence is reliable; do not say no file or image was attached. Text excerpts in context are truncated. For image visual understanding (describe, compare, read labels, listing copy ideas), call agent_attachments_analyze with the attachment id/name and a focused question. For product.import ingest or review workflows, call agent_product_import_ingest instead of manually analyzing images.")
	for i, attachment := range chatCtx.Attachments {
		lines = append(lines, formatAgentChatAttachmentContextBlock(i+1, attachment))
	}
	return []string{strings.Join(lines, "\n")}, nil
}

func formatAgentChatAttachmentContextBlock(index int, attachment aipkg.ChatAttachment) string {
	parts := []string{fmt.Sprintf("- Attachment %d:", index)}
	if value := artifactContextPromptValue(attachment.ID, 120); value != "" {
		parts = append(parts, "id="+value)
	}
	if value := artifactContextPromptValue(attachment.Name, 160); value != "" {
		parts = append(parts, "name="+value)
	}
	if value := artifactContextPromptValue(attachment.ContentType, 100); value != "" {
		parts = append(parts, "contentType="+value)
	}
	if attachment.Size > 0 {
		parts = append(parts, fmt.Sprintf("size=%d", attachment.Size))
	}
	lines := []string{strings.Join(parts, " ")}
	if attachment.URL != "" {
		lines = append(lines, "  url: "+redact.URL(attachment.URL))
	}
	if attachment.SourceURI != "" {
		lines = append(lines, "  sourceUri: "+redact.URL(attachment.SourceURI))
	}
	if text := artifactContextPromptValue(attachment.Text, agentChatAttachmentTextMaxLen); text != "" {
		lines = append(lines, "  textExcerpt(redacted/truncated): "+text)
	}
	if strings.TrimSpace(attachment.ContentBase64) != "" {
		lines = append(lines, "  inlineBinary: available")
	}
	return strings.Join(lines, "\n")
}

func agentChatAttachmentImageURL(attachment aipkg.ChatAttachment, origin string) string {
	contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if encoded := strings.TrimSpace(attachment.ContentBase64); encoded != "" {
		return "data:" + contentType + ";base64," + encoded
	}
	rawURL := strings.TrimSpace(attachment.URL)
	if rawURL == "" {
		rawURL = strings.TrimSpace(attachment.SourceURI)
	}
	if rawURL == "" {
		return ""
	}
	return aipkg.ResolveImageURLs([]string{rawURL}, origin)[0]
}

func agentChatArtifactContextBlocks(ctx context.Context, persist agentstore.Persistence, tenantID string, chatCtx *aipkg.ChatContext) ([]string, error) {
	if persist == nil || chatCtx == nil || len(chatCtx.ArtifactIDs) == 0 {
		return nil, nil
	}
	ids := uniqueTrimmedStrings(chatCtx.ArtifactIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > agentChatMaxContextArtifacts {
		return nil, fmt.Errorf("too many artifactIds (max %d)", agentChatMaxContextArtifacts)
	}
	lines := make([]string, 0, len(ids)*6+2)
	lines = append(lines, "Referenced artifacts for this turn:")
	lines = append(lines, "Use these artifacts as bounded context. dataExcerpt values are redacted and may be truncated; fetch the artifact by id when exact source payload is required.")
	for i, id := range ids {
		artifact, err := persist.LoadArtifact(ctx, tenantID, id)
		if errors.Is(err, agentstore.ErrArtifactNotFound) {
			return nil, fmt.Errorf("artifact %q not found", id)
		}
		if err != nil {
			return nil, fmt.Errorf("load artifact %q: %w", id, err)
		}
		lines = append(lines, formatAgentChatArtifactContextBlock(i+1, artifact))
	}
	return []string{strings.Join(lines, "\n")}, nil
}

func agentChatSkillRunContextBlocks(ctx context.Context, persist agentstore.Persistence, tenantID string, runs []*agentstore.SkillRun) ([]string, error) {
	if len(runs) == 0 {
		return nil, nil
	}
	if persist == nil {
		return nil, fmt.Errorf("skill run context requires agent store")
	}
	lines := make([]string, 0, len(runs)*8+3)
	lines = append(lines, "Referenced skill runs for this turn:")
	lines = append(lines, "Continue from these skill runs and their artifacts. Do not ask the user to paste the same sources again. Fetch artifacts by id when exact data is required before creating or applying changes.")
	for i, run := range runs {
		if run == nil {
			continue
		}
		artifacts, err := persist.ListArtifacts(ctx, tenantID, run.ID, "", "", agentChatSkillRunArtifactMax, 0)
		if err != nil {
			return nil, fmt.Errorf("list artifacts for skill run %q: %w", run.ID, err)
		}
		lines = append(lines, formatAgentChatSkillRunContextBlock(i+1, run, artifacts))
	}
	return []string{strings.Join(lines, "\n")}, nil
}

func agentChatContextSkillRunIDs(chatCtx *aipkg.ChatContext) ([]string, error) {
	if chatCtx == nil || len(chatCtx.SkillRunIDs) == 0 {
		return nil, nil
	}
	ids := uniqueTrimmedStrings(chatCtx.SkillRunIDs)
	if len(ids) > agentChatMaxContextSkillRuns {
		return nil, fmt.Errorf("too many skillRunIds (max %d)", agentChatMaxContextSkillRuns)
	}
	return ids, nil
}

func formatAgentChatSkillRunContextBlock(index int, run *agentstore.SkillRun, artifacts []*agentstore.Artifact) string {
	if run == nil {
		return fmt.Sprintf("- SkillRun %d: <nil>", index)
	}
	parts := []string{
		"id=" + run.ID,
		"skillId=" + run.SkillID,
		"status=" + run.Status,
	}
	if run.ThreadID != "" {
		parts = append(parts, "threadId="+run.ThreadID)
	}
	if run.StoreID != "" {
		parts = append(parts, "storeId="+run.StoreID)
	}
	if run.ActingPersona != "" {
		parts = append(parts, "actingPersona="+run.ActingPersona)
	}
	if !run.UpdatedAt.IsZero() {
		parts = append(parts, "updatedAt="+run.UpdatedAt.UTC().Format(time.RFC3339))
	}
	lines := []string{fmt.Sprintf("- SkillRun %d: %s", index, strings.Join(parts, "; "))}
	if excerpt := compactArtifactDataExcerpt(run.Input, agentChatSkillRunDataMaxLen); excerpt != "" {
		lines = append(lines, "  inputExcerpt(redacted/truncated): "+excerpt)
	}
	if excerpt := compactArtifactDataExcerpt(run.Output, agentChatSkillRunDataMaxLen); excerpt != "" {
		lines = append(lines, "  outputExcerpt(redacted/truncated): "+excerpt)
	}
	if value := artifactContextPromptValue(run.Error, 240); value != "" {
		lines = append(lines, "  error(redacted/truncated): "+value)
	}
	if counts := formatAgentChatArtifactCounts(artifacts); counts != "" {
		lines = append(lines, "  artifactCountsShown: "+counts)
	}
	if len(artifacts) == 0 {
		lines = append(lines, "  artifacts: none")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, fmt.Sprintf("  artifactsShown: %d", len(artifacts)))
	for i, artifact := range artifacts {
		for _, line := range strings.Split(formatAgentChatArtifactContextBlockWithDataLimit(i+1, artifact, agentChatSkillRunDataMaxLen), "\n") {
			lines = append(lines, "  "+line)
		}
	}
	return strings.Join(lines, "\n")
}

func formatAgentChatArtifactCounts(artifacts []*agentstore.Artifact) string {
	if len(artifacts) == 0 {
		return ""
	}
	counts := make(map[string]int)
	keys := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		key := strings.TrimSpace(artifact.Kind)
		if key == "" {
			key = "unknown"
		}
		if artifact.Status != "" {
			key += "." + artifact.Status
		}
		if _, ok := counts[key]; !ok {
			keys = append(keys, key)
		}
		counts[key]++
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

func formatAgentChatArtifactContextBlock(index int, artifact *agentstore.Artifact) string {
	return formatAgentChatArtifactContextBlockWithDataLimit(index, artifact, agentChatArtifactDataMaxLen)
}

func formatAgentChatArtifactContextBlockWithDataLimit(index int, artifact *agentstore.Artifact, dataLimit int) string {
	if artifact == nil {
		return fmt.Sprintf("- Artifact %d: <nil>", index)
	}
	parts := []string{
		"id=" + artifact.ID,
		"kind=" + artifact.Kind,
		"status=" + artifact.Status,
	}
	if artifact.ThreadID != "" {
		parts = append(parts, "threadId="+artifact.ThreadID)
	}
	if artifact.SkillRunID != "" {
		parts = append(parts, "skillRunId="+artifact.SkillRunID)
	}
	if artifact.SkillID != "" {
		parts = append(parts, "skillId="+artifact.SkillID)
	}
	if !artifact.UpdatedAt.IsZero() {
		parts = append(parts, "updatedAt="+artifact.UpdatedAt.UTC().Format(time.RFC3339))
	}
	lines := []string{fmt.Sprintf("- Artifact %d: %s", index, strings.Join(parts, "; "))}
	if value := artifactContextPromptValue(artifact.Name, 160); value != "" {
		lines = append(lines, "  name: "+value)
	}
	if value := artifactContextPromptValue(artifact.ContentType, 80); value != "" {
		lines = append(lines, "  contentType: "+value)
	}
	if value := artifactContextPromptValue(artifact.SourceName, 160); value != "" {
		lines = append(lines, "  sourceName: "+value)
	}
	if artifact.SourceURI != "" {
		lines = append(lines, "  sourceUri: "+redact.URL(artifact.SourceURI))
	}
	if value := artifactContextPromptValue(artifact.Summary, 320); value != "" {
		lines = append(lines, "  summary: "+value)
	}
	if excerpt := compactArtifactDataExcerpt(artifact.Data, dataLimit); excerpt != "" {
		lines = append(lines, "  dataExcerpt(redacted/truncated): "+excerpt)
	}
	return strings.Join(lines, "\n")
}

func artifactContextPromptValue(raw string, limit int) string {
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

func compactArtifactDataExcerpt(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || limit <= 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		switch v := decoded.(type) {
		case map[string]any:
			raw = redact.RedactMapJSON(v)
		case []any:
			for i, item := range v {
				if obj, ok := item.(map[string]any); ok {
					v[i] = redact.RedactMap(obj)
				}
			}
			if compact, err := json.Marshal(v); err == nil {
				raw = string(compact)
			}
		default:
			if compact, err := json.Marshal(decoded); err == nil {
				raw = string(compact)
			}
		}
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

func uniqueTrimmedStrings(items []string) []string {
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
	return out
}

func agentChatRouteErrorMessage(err error) string {
	if err == nil {
		return "AI assistant failed to route the request"
	}
	if strings.Contains(err.Error(), "MOBAZHA_AGENT_SKILLS_DIR") {
		return "AI assistant requires private skill configuration (MOBAZHA_AGENT_SKILLS_DIR)"
	}
	if strings.Contains(err.Error(), "skill run") || strings.Contains(err.Error(), "skillRunIds") {
		return "Referenced skill run is not available"
	}
	if strings.Contains(err.Error(), "artifact") {
		return "Referenced artifact is not available"
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
		for _, attachment := range req.Context.Attachments {
			text += "\n" + attachment.Name + " " + attachment.ContentType
		}
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
		&agentruntime.Config{MaxToolRounds: 10, TurnTimeout: aipkg.StreamTimeout, MaxHistoryMsgs: aipkg.MaxSessionMessages},
	)
	orch.SetSystemPrompt(systemPrompt)
	orch.RegisterTools(agentToolDefs(aipkg.SellerTools()))
	orch.SetThreadCompactor(agentChatThreadCompactor{proxy: proxy, cfg: cfg})

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

type agentChatThreadCompactor struct {
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

func (c agentChatThreadCompactor) CompactThread(ctx context.Context, req agentruntime.ThreadCompactionRequest) (string, error) {
	if c.proxy == nil {
		return "", fmt.Errorf("AI proxy is not configured")
	}
	messages := make([]aipkg.ChatMsg, 0, len(req.Messages)+1)
	messages = append(messages, aipkg.ChatMsg{
		Role:    aipkg.RoleSystem,
		Content: agentChatThreadCompactionPrompt,
	})
	messages = append(messages, agentChatMessages(req.Messages)...)
	deltas, err := c.proxy.StreamChat(ctx, c.cfg, messages, nil)
	if err != nil {
		return "", err
	}
	var summary strings.Builder
	for delta := range deltas {
		if delta.Error != "" {
			return "", fmt.Errorf("%s", delta.Error)
		}
		if delta.Content != "" {
			summary.WriteString(delta.Content)
		}
		if delta.Done {
			break
		}
	}
	out := strings.TrimSpace(summary.String())
	if out == "" {
		return "", fmt.Errorf("thread compactor returned empty summary")
	}
	return out, nil
}

type agentChatToolExecutor struct{}

func (agentChatToolExecutor) Execute(ctx context.Context, call agentexec.ToolCall) (agentexec.ToolResult, error) {
	toolCtx, _ := ctx.Value(agentToolContextKey{}).(agentToolContext)
	executor := aipkg.NewToolExecutor(toolCtx.baseURL, toolCtx.authToken)
	if call.Name == "agent_attachments_analyze" {
		arguments, err := agentChatAttachmentsAnalyzeArgumentsWithAttachments(call.Arguments, toolCtx.attachments)
		if err != nil {
			return agentexec.ToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: fmt.Sprintf(`{"error":%q}`, "invalid attachment analyze arguments"),
				IsError: true,
			}, err
		}
		call.Arguments = arguments
	}
	if call.Name == "agent_product_import_ingest" {
		arguments, err := agentChatProductImportIngestArgumentsWithAttachments(call.Arguments, toolCtx.attachments)
		if err != nil {
			return agentexec.ToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: fmt.Sprintf(`{"error":%q}`, "invalid product import arguments"),
				IsError: true,
			}, err
		}
		call.Arguments = arguments
	}
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

func agentChatContextAttachments(chatCtx *aipkg.ChatContext) []aipkg.ChatAttachment {
	if chatCtx == nil || len(chatCtx.Attachments) == 0 {
		return nil
	}
	return append([]aipkg.ChatAttachment(nil), chatCtx.Attachments...)
}

func agentChatAttachmentsAnalyzeArgumentsWithAttachments(argsJSON string, attachments []aipkg.ChatAttachment) (string, error) {
	if len(attachments) == 0 {
		return argsJSON, nil
	}
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse attachment analyze arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	if _, ok := args["attachments"]; !ok {
		out := make([]aipkg.ChatAttachment, len(attachments))
		copy(out, attachments)
		args["attachments"] = out
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal attachment analyze arguments: %w", err)
	}
	return string(data), nil
}

func agentChatProductImportIngestArgumentsWithAttachments(argsJSON string, attachments []aipkg.ChatAttachment) (string, error) {
	if len(attachments) == 0 {
		return argsJSON, nil
	}
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse product import ingest arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	attachmentSources := agentChatProductImportAttachmentSources(attachments)
	if len(attachmentSources) == 0 {
		return argsJSON, nil
	}
	args["sources"] = mergeAgentChatProductImportSources(args["sources"], attachmentSources)
	data, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal product import ingest arguments: %w", err)
	}
	return string(data), nil
}

func agentChatProductImportAttachmentSources(attachments []aipkg.ChatAttachment) []map[string]any {
	sources := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		source := agentChatProductImportAttachmentSource(attachment)
		if len(source) > 0 {
			sources = append(sources, source)
		}
	}
	return sources
}

func agentChatProductImportAttachmentSource(attachment aipkg.ChatAttachment) map[string]any {
	name := strings.TrimSpace(attachment.Name)
	if name == "" {
		name = strings.TrimSpace(attachment.ID)
	}
	if name == "" {
		return nil
	}
	source := map[string]any{"sourceName": name}
	if id := strings.TrimSpace(attachment.ID); id != "" {
		source["attachmentId"] = id
	}
	if contentType := strings.TrimSpace(attachment.ContentType); contentType != "" {
		source["contentType"] = contentType
	}
	if text := strings.TrimSpace(attachment.Text); text != "" {
		source["text"] = text
		return source
	}
	if encoded := strings.TrimSpace(attachment.ContentBase64); encoded != "" {
		source["contentBase64"] = encoded
		return source
	}
	return nil
}

func mergeAgentChatProductImportSources(rawSources any, attachmentSources []map[string]any) []any {
	sources, ok := rawSources.([]any)
	if !ok || len(sources) == 0 {
		out := make([]any, 0, len(attachmentSources))
		for _, source := range attachmentSources {
			out = append(out, cloneMapAny(source))
		}
		return out
	}
	out := make([]any, 0, len(sources))
	for _, raw := range sources {
		source, ok := raw.(map[string]any)
		if !ok {
			out = append(out, raw)
			continue
		}
		enriched := cloneMapAny(source)
		if !agentChatProductImportSourceHasContent(enriched) {
			if idx := matchAgentChatProductImportAttachmentSource(enriched, attachmentSources); idx >= 0 {
				copyMissingAgentChatSourceFields(enriched, attachmentSources[idx])
			}
		}
		out = append(out, enriched)
	}
	return out
}

func agentChatProductImportSourceHasContent(source map[string]any) bool {
	return stringFromAny(source["text"]) != "" || stringFromAny(source["contentBase64"]) != ""
}

func matchAgentChatProductImportAttachmentSource(source map[string]any, attachments []map[string]any) int {
	if len(attachments) == 1 {
		return 0
	}
	for i, attachment := range attachments {
		if sameNonEmptyString(source["attachmentId"], attachment["attachmentId"]) ||
			sameNonEmptyString(source["id"], attachment["attachmentId"]) ||
			sameNonEmptyString(source["sourceName"], attachment["sourceName"]) ||
			sameNonEmptyString(source["name"], attachment["sourceName"]) {
			return i
		}
	}
	return -1
}

func sameNonEmptyString(a, b any) bool {
	left := stringFromAny(a)
	right := stringFromAny(b)
	return left != "" && right != "" && left == right
}

func copyMissingAgentChatSourceFields(dst map[string]any, src map[string]any) {
	for _, key := range []string{"attachmentId", "sourceName", "contentType", "text", "contentBase64"} {
		if stringFromAny(dst[key]) == "" && stringFromAny(src[key]) != "" {
			dst[key] = src[key]
		}
	}
}

func cloneMapAny(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
			Role:          aipkg.ChatRole(msg.Role),
			Content:       msg.Content,
			ContentBlocks: agentChatContentBlocks(msg.ContentBlocks),
			ToolCallID:    msg.ToolCallID,
			ToolCalls:     agentChatToolCalls(msg.ToolCalls),
		}
	}
	return out
}

func agentChatContentBlocks(blocks []agentruntime.MessageContentBlock) []aipkg.ChatContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]aipkg.ChatContentBlock, 0, len(blocks))
	for _, block := range blocks {
		item := aipkg.ChatContentBlock{
			Type: block.Type,
			Text: block.Text,
		}
		if block.ImageURL != nil {
			item.ImageURL = &aipkg.ChatImageURL{
				URL:    block.ImageURL.URL,
				Detail: block.ImageURL.Detail,
			}
		}
		out = append(out, item)
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
