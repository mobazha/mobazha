//go:build !private_distribution

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
)

type agentChatHTTPTestNode struct {
	*aiStatusTestNode
	proxy *aipkg.Proxy
	store agentstore.Persistence
}

func (n *agentChatHTTPTestNode) AIProxy() *aipkg.Proxy                  { return n.proxy }
func (n *agentChatHTTPTestNode) AgentStore() agentstore.Persistence     { return n.store }
func (n *agentChatHTTPTestNode) ProfileName() string                    { return "Test Store" }
func (n *agentChatHTTPTestNode) ProductCatalog() []aipkg.ListingSummary { return nil }

type agentChatMemoryStore struct {
	thread   *agentstore.Thread
	turns    []*agentstore.Turn
	messages []*agentstore.Message
}

func (s *agentChatMemoryStore) SaveThread(_ context.Context, thread *agentstore.Thread) error {
	cp := *thread
	s.thread = &cp
	return nil
}

func (s *agentChatMemoryStore) SaveTurn(_ context.Context, turn *agentstore.Turn) error {
	cp := *turn
	s.turns = append(s.turns, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveMessage(_ context.Context, msg *agentstore.Message) error {
	cp := *msg
	s.messages = append(s.messages, &cp)
	return nil
}

func (s *agentChatMemoryStore) LoadThread(_ context.Context, _, threadID string) (*agentstore.Thread, error) {
	if s.thread == nil || s.thread.ID != threadID {
		return nil, agentstore.ErrThreadNotFound
	}
	cp := *s.thread
	return &cp, nil
}

func (s *agentChatMemoryStore) ListThreads(context.Context, string, int, int) ([]*agentstore.Thread, error) {
	if s.thread == nil {
		return nil, nil
	}
	cp := *s.thread
	return []*agentstore.Thread{&cp}, nil
}

func (s *agentChatMemoryStore) LoadMessages(_ context.Context, _, threadID string) ([]*agentstore.Message, error) {
	out := make([]*agentstore.Message, 0, len(s.messages))
	for _, msg := range s.messages {
		if msg.ThreadID == threadID {
			cp := *msg
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *agentChatMemoryStore) DeleteThread(context.Context, string, string) error {
	s.thread = nil
	s.turns = nil
	s.messages = nil
	return nil
}

func TestAgentChatConfigFingerprint_ChangesWhenCredentialChanges(t *testing.T) {
	base := aipkg.Config{
		Provider: "openai",
		APIKey:   "sk-old",
		Model:    "gpt-4o-mini",
		BaseURL:  "https://api.openai.com/v1",
		Enabled:  true,
	}
	rotated := base
	rotated.APIKey = "sk-new"

	if agentChatConfigFingerprint(base, "prompt") == agentChatConfigFingerprint(rotated, "prompt") {
		t.Fatal("fingerprint should change when the API key changes")
	}
}

func TestCatalogCacheKey_IncludesTenant(t *testing.T) {
	a := catalogCacheKey("tenant-a", "Official Store")
	b := catalogCacheKey("tenant-b", "Official Store")
	if a == b {
		t.Fatalf("catalog cache key should include tenant identity, got %q", a)
	}
}

func TestHandlePOSTAgentChat_StreamsSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello from agent\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {APIKey: "test-key", Model: "test-model", BaseURL: upstream.URL},
			},
		}, aipkg.PlatformProfile{}),
		proxy: aipkg.NewProxy(upstream.Client()),
		store: &agentChatMemoryStore{},
	}
	cacheKey := "test-node:" + node.ProfileName()
	agentChatRuntimes.Delete(cacheKey)
	defer agentChatRuntimes.Delete(cacheKey)

	body := strings.NewReader(`{"message":"hello","sessionId":"thread-http"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentChat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}
	got := rr.Body.String()
	if !strings.Contains(got, "event: content") || !strings.Contains(got, "Hello from agent") {
		t.Fatalf("expected content SSE event, got %s", got)
	}
	if !strings.Contains(got, "event: done") || !strings.Contains(got, `"sessionId":"thread-http"`) {
		t.Fatalf("expected done SSE event with session id, got %s", got)
	}

	store := node.store.(*agentChatMemoryStore)
	if store.thread == nil || store.thread.Title != "hello" || store.thread.LastActive.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected thread metadata to be persisted, got %#v", store.thread)
	}
}
