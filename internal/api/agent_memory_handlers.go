//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

const (
	agentMemoryDefaultLimit = 20
	agentMemoryMaxLimit     = 100
)

type agentMemoryCreateRequest struct {
	ID       string            `json:"id,omitempty"`
	Scope    string            `json:"scope,omitempty"`
	Subject  string            `json:"subject,omitempty"`
	Content  string            `json:"content"`
	StoreID  string            `json:"storeId,omitempty"`
	ThreadID string            `json:"threadId,omitempty"`
	SkillID  string            `json:"skillId,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func (g *Gateway) handleGETAgentMemories(w http.ResponseWriter, r *http.Request) {
	p, memoryStore, ok := agentMemoryProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Agent memory is not available in this mode")
		return
	}
	scope := agentMemoryVisibleScope(r, p)
	types, ok := agentMemoryQueryScopes(r.URL.Query().Get("scope"))
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "Invalid memory scope")
		return
	}
	limit, ok := agentMemoryQueryLimit(r.URL.Query().Get("limit"))
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "Invalid limit")
		return
	}
	query := kernel.MemoryQuery{
		Scope:   scope,
		Types:   types,
		Subject: strings.TrimSpace(r.URL.Query().Get("subject")),
		Query:   strings.TrimSpace(firstNonEmptyAgentMemoryQuery(r.URL.Query().Get("q"), r.URL.Query().Get("query"))),
		Limit:   limit,
	}
	items, err := memoryStore.Search(r.Context(), query)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to list agent memories")
		return
	}
	responsePkg.Success(w, items)
}

func (g *Gateway) handlePOSTAgentMemory(w http.ResponseWriter, r *http.Request) {
	p, memoryStore, ok := agentMemoryProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Agent memory is not available in this mode")
		return
	}
	var req agentMemoryCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "Memory content is required")
		return
	}
	scope, memoryScope, err := agentMemoryWriteScope(r, p, req)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = "mem_" + uuid.NewString()
	}
	item := kernel.MemoryItem{
		ID:       id,
		Scope:    memoryScope,
		Subject:  strings.TrimSpace(req.Subject),
		Content:  req.Content,
		Metadata: req.Metadata,
	}
	if err := memoryStore.Save(r.Context(), scope, item); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to save agent memory")
		return
	}
	responsePkg.Created(w, item)
}

func (g *Gateway) handleDELETEAgentMemory(w http.ResponseWriter, r *http.Request) {
	p, memoryStore, ok := agentMemoryProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Agent memory is not available in this mode")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "memoryId"))
	if id == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "Memory id is required")
		return
	}
	scope := agentMemoryVisibleScope(r, p)
	if err := memoryStore.Delete(r.Context(), scope, id); err != nil {
		if errors.Is(err, agentstore.ErrMemoryNotFound) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Agent memory not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to delete agent memory")
		return
	}
	responsePkg.NoContent(w)
}

func agentMemoryProvider(r *http.Request) (aiChatProvider, kernel.MemoryStore, bool) {
	p, ok := getAIChatProvider(r)
	if !ok {
		return nil, nil, false
	}
	memoryStore := agentChatKernelMemoryStore(p.AgentStore())
	if memoryStore == nil {
		return nil, nil, false
	}
	return p, memoryStore, true
}

func agentMemoryVisibleScope(r *http.Request, p aiChatProvider) kernel.Scope {
	if p == nil {
		if provider, ok := getAIChatProvider(r); ok {
			p = provider
		}
	}
	scope := kernel.Scope{}
	if p != nil {
		scope.TenantID = agentChatTenantID(r, p)
		scope.StoreID = p.ProfileName()
		scope.ActorID = agentApprovalDecisionActor(r, p)
	}
	q := r.URL.Query()
	if storeID := strings.TrimSpace(q.Get("storeId")); storeID != "" {
		scope.StoreID = storeID
	}
	scope.ThreadID = strings.TrimSpace(q.Get("threadId"))
	scope.SkillID = strings.TrimSpace(q.Get("skillId"))
	return scope
}

func agentMemoryWriteScope(r *http.Request, p aiChatProvider, req agentMemoryCreateRequest) (kernel.Scope, kernel.MemoryScope, error) {
	memoryScope, ok := agentMemoryScope(strings.TrimSpace(req.Scope), kernel.MemoryUser)
	if !ok {
		return kernel.Scope{}, "", errors.New("invalid memory scope")
	}
	scope := kernel.Scope{
		TenantID: agentChatTenantID(r, p),
		StoreID:  p.ProfileName(),
		ActorID:  agentApprovalDecisionActor(r, p),
		ThreadID: strings.TrimSpace(req.ThreadID),
		SkillID:  strings.TrimSpace(req.SkillID),
	}
	if storeID := strings.TrimSpace(req.StoreID); storeID != "" {
		scope.StoreID = storeID
	}
	switch memoryScope {
	case kernel.MemoryUser:
		if scope.ActorID == "" {
			return kernel.Scope{}, "", errors.New("user memory requires an actor")
		}
	case kernel.MemoryStoreScope:
		if scope.StoreID == "" {
			return kernel.Scope{}, "", errors.New("store memory requires a storeId")
		}
	case kernel.MemoryThread:
		if scope.ThreadID == "" {
			return kernel.Scope{}, "", errors.New("thread memory requires a threadId")
		}
	case kernel.MemorySkill:
		if scope.SkillID == "" {
			return kernel.Scope{}, "", errors.New("skill memory requires a skillId")
		}
	case kernel.MemoryTenant:
		if scope.TenantID == "" {
			return kernel.Scope{}, "", errors.New("tenant memory requires a tenant")
		}
	}
	return scope, memoryScope, nil
}

func agentMemoryScope(raw string, fallback kernel.MemoryScope) (kernel.MemoryScope, bool) {
	if raw == "" {
		return fallback, true
	}
	switch kernel.MemoryScope(raw) {
	case kernel.MemoryUser,
		kernel.MemoryStoreScope,
		kernel.MemoryTenant,
		kernel.MemoryThread,
		kernel.MemorySkill:
		return kernel.MemoryScope(raw), true
	default:
		return "", false
	}
}

func agentMemoryQueryScopes(raw string) ([]kernel.MemoryScope, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	types := make([]kernel.MemoryScope, 0, len(parts))
	seen := map[kernel.MemoryScope]bool{}
	for _, part := range parts {
		scope, ok := agentMemoryScope(strings.TrimSpace(part), "")
		if !ok || scope == "" {
			return nil, false
		}
		if !seen[scope] {
			seen[scope] = true
			types = append(types, scope)
		}
	}
	return types, true
}

func agentMemoryQueryLimit(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return agentMemoryDefaultLimit, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, false
	}
	if limit > agentMemoryMaxLimit {
		limit = agentMemoryMaxLimit
	}
	return limit, true
}

func firstNonEmptyAgentMemoryQuery(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
