//go:build !private_distribution

package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/internal/repo"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type aiChatProvider interface {
	aiConfigProvider
	AgentStore() agentstore.Persistence
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
		host = "127.0.0.1:" + repo.DefaultGatewayPort
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

func getCachedCatalog(tenantID string, p aiChatProvider) string {
	key := catalogCacheKey(tenantID, p.ProfileName())
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

func catalogCacheKey(tenantID, profileName string) string {
	return "catalog-" + tenantID + ":" + profileName
}

// handleGETAgentChatSessions handles GET /v1/agent/chat/sessions.
func (g *Gateway) handleGETAgentChatSessions(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	tenantID := agentChatTenantID(r, p)
	sessions, err := p.AgentStore().ListThreads(r.Context(), tenantID, limit, offset)
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
			Role:      agentChatRole(s.Persona),
			Title:     s.Title,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.LastActive,
		}
	}
	responsePkg.Success(w, summaries)
}

// handleGETAgentChatSession handles GET /v1/agent/chat/{sessionId}.
func (g *Gateway) handleGETAgentChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	tenantID := agentChatTenantID(r, p)
	thread, err := p.AgentStore().LoadThread(r.Context(), tenantID, sessionID)
	if errors.Is(err, agentstore.ErrThreadNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session")
		return
	}
	messages, err := p.AgentStore().LoadMessages(r.Context(), tenantID, sessionID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session messages")
		return
	}
	responsePkg.Success(w, agentChatSessionFromThread(thread, messages))
}

// handleDELETEAgentChatSession handles DELETE /v1/agent/chat/{sessionId}.
func (g *Gateway) handleDELETEAgentChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	tenantID := agentChatTenantID(r, p)
	if err := p.AgentStore().DeleteThread(r.Context(), tenantID, sessionID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to delete session")
		return
	}
	forgetAgentChatThread(tenantID+":"+p.ProfileName(), tenantID, sessionID)
	responsePkg.NoContent(w)
}

func agentChatTenantID(r *http.Request, p aiChatProvider) string {
	nodeID := getIdentityService(r).GetNodeID()
	if nodeID != "" {
		return nodeID
	}
	return p.ProfileName()
}

func agentChatRole(persona string) string {
	if persona != "" {
		return persona
	}
	return string(aipkg.UserRoleSeller)
}

func agentChatSessionFromThread(thread *agentstore.Thread, messages []*agentstore.Message) *aipkg.ChatSession {
	session := &aipkg.ChatSession{
		ID:        thread.ID,
		TenantID:  thread.TenantID,
		Role:      agentChatRole(thread.Persona),
		Title:     thread.Title,
		CreatedAt: thread.CreatedAt,
		UpdatedAt: thread.LastActive,
		Messages:  agentChatVisibleMessages(messages),
	}
	return session
}

func agentChatVisibleMessages(messages []*agentstore.Message) []aipkg.ChatMsg {
	out := make([]aipkg.ChatMsg, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		role := aipkg.ChatRole(msg.Role)
		if role != aipkg.RoleUser && role != aipkg.RoleAssistant {
			continue
		}
		if role == aipkg.RoleAssistant && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		out = append(out, aipkg.ChatMsg{
			Role:    role,
			Content: msg.Content,
		})
	}
	return out
}

func visibleChatSession(session *aipkg.ChatSession) *aipkg.ChatSession {
	if session == nil {
		return nil
	}
	visible := *session
	visible.Messages = visibleChatMessages(session.Messages)
	return &visible
}

func visibleChatMessages(messages []aipkg.ChatMsg) []aipkg.ChatMsg {
	visible := make([]aipkg.ChatMsg, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == aipkg.RoleTool || msg.Role == aipkg.RoleSystem {
			continue
		}
		if msg.Role == aipkg.RoleAssistant && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		msg.ToolCalls = nil
		msg.ToolCallID = ""
		msg.Name = ""
		visible = append(visible, msg)
	}
	return visible
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
