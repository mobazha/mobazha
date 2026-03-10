package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type aiChatProvider interface {
	aiConfigProvider
	ChatStore() *aipkg.ChatStore
	ProfileName() string
	ProductCatalog() []aipkg.ListingSummary
}

func getAIChatProvider(r *http.Request) (aiChatProvider, bool) {
	node := getNodeService(r)
	if node == nil {
		return nil, false
	}
	if p, ok := node.(aiChatProvider); ok {
		return p, true
	}
	return nil, false
}

// getLocalAPIURL constructs the local API URL for tool execution.
func getLocalAPIURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "127.0.0.1:4002"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// getAuthToken extracts the raw Authorization header value from the request.
func getAuthToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		return auth
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return "Bearer " + token
	}
	return ""
}

// activeAIStreams enforces max 1 concurrent AI chat stream per tenant.
var activeAIStreams sync.Map

// catalogCache stores formatted product catalog text per provider with a short TTL
// to avoid reading the full ListingIndex from DB on every chat message.
var catalogCache sync.Map

type catalogCacheEntry struct {
	text      string
	expiresAt time.Time
}

const catalogCacheTTL = 30 * time.Second

func getCachedCatalog(p aiChatProvider) string {
	key := "catalog-" + p.ProfileName()
	if v, ok := catalogCache.Load(key); ok {
		entry := v.(*catalogCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.text
		}
	}
	catalog := p.ProductCatalog()
	text := ""
	if len(catalog) > 0 {
		text = aipkg.FormatProductCatalog(catalog)
	}
	catalogCache.Store(key, &catalogCacheEntry{
		text:      text,
		expiresAt: time.Now().Add(catalogCacheTTL),
	})
	return text
}

// handlePOSTAIChat handles POST /v1/ai/chat — streaming AI conversation.
func (g *Gateway) handlePOSTAIChat(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available in this mode")
		return
	}

	cfg := p.AIConfig()
	if !cfg.IsValid() {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "AI is not configured. Please set up an AI provider in Settings > Integrations.")
		return
	}

	store := p.ChatStore()
	streamKey := "ai-chat-" + p.ProfileName()
	if _, loaded := activeAIStreams.LoadOrStore(streamKey, true); loaded {
		responsePkg.Error(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Another AI chat request is still in progress. Please wait.")
		return
	}
	defer activeAIStreams.Delete(streamKey)

	var req aipkg.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "message is required")
		return
	}
	if len(req.Message) > aipkg.MaxUserMessageLen {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			fmt.Sprintf("message too long (max %d characters)", aipkg.MaxUserMessageLen))
		return
	}

	role := aipkg.UserRoleSeller

	var session *aipkg.ChatSession
	if req.SessionID != "" {
		existing, err := store.GetSession(req.SessionID)
		if err != nil {
			aiLog.Errorf("get session: %s", err)
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session")
			return
		}
		session = existing
	}

	if session == nil {
		session = &aipkg.ChatSession{
			ID:        uuid.New().String(),
			Role:      string(role),
			CreatedAt: time.Now(),
		}
	}

	systemPrompt := aipkg.BuildSystemPrompt(role, p.ProfileName(), req.Context)
	if catalogCtx := getCachedCatalog(p); catalogCtx != "" {
		systemPrompt += "\n\n" + catalogCtx
	}
	messages := buildLLMMessages(systemPrompt, session.Messages, req.Message)

	localURL := getLocalAPIURL(r)
	authToken := getAuthToken(r)
	executor := aipkg.NewToolExecutor(localURL, authToken)
	tools := aipkg.SellerTools()

	engine := aipkg.NewChatEngine(p.AIProxy(), cfg, executor, tools)

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

	session.Messages = append(session.Messages, aipkg.ChatMsg{
		Role:    aipkg.RoleUser,
		Content: req.Message,
	})

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

	newMsgs, err := engine.RunStream(r.Context(), messages, emitSSE)
	if err != nil {
		emitSSE(aipkg.SSEEvent{
			Type:  aipkg.SSETypeError,
			Error: err.Error(),
		})
		return
	}

	session.Messages = append(session.Messages, newMsgs...)

	if session.Title == "" && len(session.Messages) >= 2 {
		session.Title = generateSessionTitle(req.Message)
	}

	trimSessionMessages(session)

	if err := store.CreateOrUpdateSession(session); err != nil {
		aiLog.Errorf("save session: %s", err)
	}

	emitSSE(aipkg.SSEEvent{
		Type:      aipkg.SSETypeDone,
		SessionID: session.ID,
	})
}

// handleGETAIChatSessions handles GET /v1/ai/chat/sessions.
func (g *Gateway) handleGETAIChatSessions(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	sessions, err := p.ChatStore().ListSessions(limit, offset)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to list sessions")
		return
	}

	type sessionSummary struct {
		ID        string    `json:"id"`
		Role      string    `json:"role"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	summaries := make([]sessionSummary, len(sessions))
	for i, s := range sessions {
		summaries[i] = sessionSummary{
			ID:        s.ID,
			Role:      s.Role,
			Title:     s.Title,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		}
	}
	responsePkg.Success(w, summaries)
}

// handleGETAIChatSession handles GET /v1/ai/chat/{sessionId}.
func (g *Gateway) handleGETAIChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := mux.Vars(r)["sessionId"]
	session, err := p.ChatStore().GetSession(sessionID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session")
		return
	}
	if session == nil {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "session not found")
		return
	}
	responsePkg.Success(w, session)
}

// handleDELETEAIChatSession handles DELETE /v1/ai/chat/{sessionId}.
func (g *Gateway) handleDELETEAIChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := mux.Vars(r)["sessionId"]
	if err := p.ChatStore().DeleteSession(sessionID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to delete session")
		return
	}
	responsePkg.NoContent(w)
}

func buildLLMMessages(systemPrompt string, history []aipkg.ChatMsg, currentMessage string) []aipkg.ChatMsg {
	msgs := []aipkg.ChatMsg{{Role: aipkg.RoleSystem, Content: systemPrompt}}
	msgs = append(msgs, history...)
	msgs = append(msgs, aipkg.ChatMsg{Role: aipkg.RoleUser, Content: currentMessage})
	return msgs
}

// generateSessionTitle extracts a clean title from the user's first message.
func generateSessionTitle(msg string) string {
	s := strings.TrimSpace(msg)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 80 {
		return s
	}
	truncated := s[:80]
	if idx := strings.LastIndexAny(truncated, " ,.;!?。，；！？"); idx > 40 {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}

func trimSessionMessages(session *aipkg.ChatSession) {
	if len(session.Messages) <= aipkg.MaxSessionMessages {
		return
	}
	session.Messages = session.Messages[len(session.Messages)-aipkg.MaxSessionMessages:]
}
