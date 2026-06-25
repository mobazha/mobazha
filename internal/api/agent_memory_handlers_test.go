//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
)

type agentMemoryAPITestSave struct {
	scope kernel.Scope
	item  kernel.MemoryItem
}

type agentMemoryAPITestStore struct {
	agentChatMemoryStore
	searchItems   []kernel.MemoryItem
	searchQueries []kernel.MemoryQuery
	saves         []agentMemoryAPITestSave
	deleteScopes  []kernel.Scope
	deletedIDs    []string
	deleteErr     error
}

func (s *agentMemoryAPITestStore) Search(_ context.Context, query kernel.MemoryQuery) ([]kernel.MemoryItem, error) {
	s.searchQueries = append(s.searchQueries, query)
	out := make([]kernel.MemoryItem, len(s.searchItems))
	copy(out, s.searchItems)
	return out, nil
}

func (s *agentMemoryAPITestStore) Save(_ context.Context, scope kernel.Scope, item kernel.MemoryItem) error {
	s.saves = append(s.saves, agentMemoryAPITestSave{scope: scope, item: item})
	return nil
}

func (s *agentMemoryAPITestStore) Delete(_ context.Context, scope kernel.Scope, id string) error {
	s.deleteScopes = append(s.deleteScopes, scope)
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteErr
}

func TestHandlePOSTAgentMemory_DefaultsToUserScope(t *testing.T) {
	store := &agentMemoryAPITestStore{}
	req := newAgentMemoryAPIRequest(t, http.MethodPost, "/v1/agent/memories", `{"subject":"preferences","content":"Prefers concise checkout guidance.","metadata":{"source":"manual"}}`, store)
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentMemory(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.saves) != 1 {
		t.Fatalf("expected 1 saved memory, got %d", len(store.saves))
	}
	save := store.saves[0]
	if save.scope.TenantID != "test-node" || save.scope.ActorID != "test-node" || save.scope.StoreID != "Test Store" {
		t.Fatalf("unexpected write scope: %+v", save.scope)
	}
	if save.item.Scope != kernel.MemoryUser {
		t.Fatalf("expected user memory scope, got %q", save.item.Scope)
	}
	if save.item.ID == "" || !strings.HasPrefix(save.item.ID, "mem_") {
		t.Fatalf("expected generated memory id, got %q", save.item.ID)
	}
	if save.item.Subject != "preferences" || save.item.Content != "Prefers concise checkout guidance." {
		t.Fatalf("unexpected saved item: %+v", save.item)
	}

	var envelope struct {
		Data kernel.MemoryItem `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != save.item.ID || envelope.Data.Scope != kernel.MemoryUser {
		t.Fatalf("unexpected response data: %+v", envelope.Data)
	}
}

func TestHandleGETAgentMemories_SearchesVisibleScope(t *testing.T) {
	store := &agentMemoryAPITestStore{
		searchItems: []kernel.MemoryItem{{
			ID:      "mem_1",
			Scope:   kernel.MemoryUser,
			Subject: "preferences",
			Content: "Prefers espresso.",
		}},
	}
	req := newAgentMemoryAPIRequest(t, http.MethodGet, "/v1/agent/memories?scope=user,tenant&q=espresso&limit=7&subject=preferences&threadId=thread_1&skillId=product.import", "", store)
	rr := httptest.NewRecorder()

	(&Gateway{}).handleGETAgentMemories(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.searchQueries) != 1 {
		t.Fatalf("expected 1 memory query, got %d", len(store.searchQueries))
	}
	query := store.searchQueries[0]
	if query.Scope.TenantID != "test-node" || query.Scope.ActorID != "test-node" || query.Scope.StoreID != "Test Store" {
		t.Fatalf("unexpected visible scope: %+v", query.Scope)
	}
	if query.Scope.ThreadID != "thread_1" || query.Scope.SkillID != "product.import" {
		t.Fatalf("expected thread and skill filters, got %+v", query.Scope)
	}
	if query.Query != "espresso" || query.Subject != "preferences" || query.Limit != 7 {
		t.Fatalf("unexpected query fields: %+v", query)
	}
	if len(query.Types) != 2 || query.Types[0] != kernel.MemoryUser || query.Types[1] != kernel.MemoryTenant {
		t.Fatalf("unexpected memory scope filters: %+v", query.Types)
	}

	var envelope struct {
		Data []kernel.MemoryItem `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].ID != "mem_1" {
		t.Fatalf("unexpected response data: %+v", envelope.Data)
	}
}

func TestHandlePOSTAgentMemory_RequiresThreadIDForThreadScope(t *testing.T) {
	store := &agentMemoryAPITestStore{}
	req := newAgentMemoryAPIRequest(t, http.MethodPost, "/v1/agent/memories", `{"scope":"thread","content":"Use this only in the thread."}`, store)
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentMemory(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.saves) != 0 {
		t.Fatalf("expected no memory save, got %d", len(store.saves))
	}
}

func TestHandleDELETEAgentMemory_ArchivesVisibleMemory(t *testing.T) {
	store := &agentMemoryAPITestStore{}
	req := newAgentMemoryAPIRequest(t, http.MethodDelete, "/v1/agent/memories/mem_1?threadId=thread_1", "", store)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("memoryId", "mem_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&Gateway{}).handleDELETEAgentMemory(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.deletedIDs) != 1 || store.deletedIDs[0] != "mem_1" {
		t.Fatalf("unexpected deleted ids: %+v", store.deletedIDs)
	}
	if len(store.deleteScopes) != 1 || store.deleteScopes[0].ThreadID != "thread_1" || store.deleteScopes[0].ActorID != "test-node" {
		t.Fatalf("unexpected delete scope: %+v", store.deleteScopes)
	}
}

func newAgentMemoryAPIRequest(t *testing.T, method, target, body string, store *agentMemoryAPITestStore) *http.Request {
	t.Helper()
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
}
