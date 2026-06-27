//go:build !private_distribution

package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
)

func TestGetLocalAPIURL_PrefersServerLocalAddr(t *testing.T) {
	req := httptestRequestWithHost("http://localhost:18080/v1/agent/chat", "localhost:18080")
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 8080,
	}))

	got := getLocalAPIURL(req)
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("expected local listener address for server-side tool calls, got %q", got)
	}
}

func httptestRequestWithHost(rawURL, host string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, rawURL, nil)
	req.Host = host
	return req
}

func TestVisibleChatMessages_HidesInternalToolTraffic(t *testing.T) {
	messages := []aipkg.ChatMsg{
		{Role: aipkg.RoleSystem, Content: "system prompt"},
		{Role: aipkg.RoleUser, Content: "What should I focus on?"},
		{
			Role:    aipkg.RoleAssistant,
			Content: "Let me inspect the internal data.",
			ToolCalls: []aipkg.ToolCall{{
				ID: "call_1",
				Function: aipkg.ToolCallFunc{
					Name:      "sales_list",
					Arguments: `{"limit":20}`,
				},
			}},
		},
		{
			Role:       aipkg.RoleTool,
			Content:    `{"data":[{"orderID":"QmSecret"}]}`,
			ToolCallID: "call_1",
			Name:       "sales_list",
		},
		{Role: aipkg.RoleAssistant, Content: "Start with unread customer messages."},
	}

	visible := visibleChatMessages(messages)
	if len(visible) != 2 {
		t.Fatalf("expected only user and final assistant messages, got %d: %#v", len(visible), visible)
	}
	if visible[0].Role != aipkg.RoleUser || visible[0].Content != "What should I focus on?" {
		t.Fatalf("unexpected first visible message: %#v", visible[0])
	}
	if visible[1].Role != aipkg.RoleAssistant || visible[1].Content != "Start with unread customer messages." {
		t.Fatalf("unexpected assistant message: %#v", visible[1])
	}
	if visible[1].ToolCalls != nil || visible[1].ToolCallID != "" || visible[1].Name != "" {
		t.Fatalf("assistant internals should be stripped: %#v", visible[1])
	}
}

func TestAgentChatVisibleMessages_PreservesAttachmentDisplay(t *testing.T) {
	messages := []*agentstore.Message{
		{
			Role:              "user",
			Content:           "Import from this image",
			AttachmentDisplay: `[{"artifactId":"art_img","name":"cover.jpg","contentType":"image/jpeg"}]`,
		},
	}

	visible := agentChatVisibleMessages(messages)
	if len(visible) != 1 {
		t.Fatalf("expected one visible message, got %d", len(visible))
	}
	if len(visible[0].AttachmentDisplay) != 1 {
		t.Fatalf("expected attachment display metadata, got %#v", visible[0].AttachmentDisplay)
	}
	got := visible[0].AttachmentDisplay[0]
	if got.ArtifactID != "art_img" || got.Name != "cover.jpg" || got.ContentType != "image/jpeg" {
		t.Fatalf("unexpected attachment display: %#v", got)
	}
}

func TestAgentChatVisibleMessages_PreservesDeliveryOnlyAssistantMessage(t *testing.T) {
	messages := []*agentstore.Message{
		{Role: "user", Content: "Import these products"},
		{
			Role:       "assistant",
			Deliveries: `[{"state":"needs_review","skillId":"product.import","skillRunId":"run_1","messageKey":"product_import.needs_review","data":{"reviewableCount":2}}]`,
		},
	}

	visible := agentChatVisibleMessages(messages)
	if len(visible) != 2 {
		t.Fatalf("expected user and delivery messages, got %d: %#v", len(visible), visible)
	}
	deliveries := visible[1].Deliveries
	if len(deliveries) != 1 {
		t.Fatalf("expected one persisted delivery, got %#v", deliveries)
	}
	if deliveries[0].SkillRunID != "run_1" || deliveries[0].MessageKey != "product_import.needs_review" {
		t.Fatalf("unexpected delivery: %#v", deliveries[0])
	}
	if string(deliveries[0].Data) != `{"reviewableCount":2}` {
		t.Fatalf("unexpected delivery data: %s", deliveries[0].Data)
	}
}

func TestAgentChatVisibleMessages_HidesAssistantToolCallContent(t *testing.T) {
	messages := []*agentstore.Message{
		{Role: "user", Content: "Import this image"},
		{
			Role:      "assistant",
			Content:   "Let me inspect the proposal details.",
			ToolCalls: `[{"id":"call_1","name":"agent_artifacts_get","arguments":"{}"}]`,
		},
		{Role: "tool", Content: `{"data":{"summary":"internal"}}`, ToolCallID: "call_1"},
		{Role: "assistant", Content: "The import proposal is ready for review."},
	}

	visible := agentChatVisibleMessages(messages)
	if len(visible) != 2 {
		t.Fatalf("expected user plus final assistant only, got %d: %#v", len(visible), visible)
	}
	if visible[1].Content != "The import proposal is ready for review." {
		t.Fatalf("unexpected final assistant message: %#v", visible[1])
	}
}

func TestVisibleChatSession_DoesNotMutateStoredSession(t *testing.T) {
	session := &aipkg.ChatSession{
		ID: "session-1",
		Messages: []aipkg.ChatMsg{
			{Role: aipkg.RoleUser, Content: "Hello"},
			{Role: aipkg.RoleTool, Content: `{"data":[]}`, Name: "profile_get"},
		},
	}

	visible := visibleChatSession(session)
	if visible == session {
		t.Fatal("visibleChatSession should return a copy")
	}
	if len(visible.Messages) != 1 {
		t.Fatalf("expected one visible message, got %d", len(visible.Messages))
	}
	if len(session.Messages) != 2 {
		t.Fatalf("stored session should remain unchanged, got %d messages", len(session.Messages))
	}
}

func TestGenerateSessionTitle_Short(t *testing.T) {
	got := generateSessionTitle("How do I create a listing?")
	if got != "How do I create a listing?" {
		t.Errorf("short message should remain unchanged, got %q", got)
	}
}

func TestGenerateSessionTitle_Long(t *testing.T) {
	longMsg := strings.Repeat("word ", 30)
	got := generateSessionTitle(longMsg)
	if len(got) > 84 { // 80 + "..."
		t.Errorf("title should be truncated, got length %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("truncated title should end with ...")
	}
}

func TestGenerateSessionTitle_MultiLine(t *testing.T) {
	got := generateSessionTitle("Line one\nLine two\nLine three")
	if strings.Contains(got, "\n") {
		t.Error("title should not contain newlines")
	}
	if got != "Line one Line two Line three" {
		t.Errorf("unexpected title: %q", got)
	}
}

func TestGenerateSessionTitle_Empty(t *testing.T) {
	got := generateSessionTitle("")
	if got != "" {
		t.Errorf("empty message should produce empty title, got %q", got)
	}
}

func TestTrimSessionMessages_UnderLimit(t *testing.T) {
	session := &aipkg.ChatSession{
		Messages: make([]aipkg.ChatMsg, 10),
	}
	trimSessionMessages(session)
	if len(session.Messages) != 10 {
		t.Errorf("should not trim when under limit: got %d", len(session.Messages))
	}
}

func TestTrimSessionMessages_OverLimit(t *testing.T) {
	msgs := make([]aipkg.ChatMsg, 50)
	for i := range msgs {
		msgs[i] = aipkg.ChatMsg{Role: aipkg.RoleUser, Content: strings.Repeat("m", i)}
	}
	session := &aipkg.ChatSession{Messages: msgs}
	trimSessionMessages(session)
	if len(session.Messages) != aipkg.MaxSessionMessages {
		t.Errorf("expected %d messages after trim, got %d", aipkg.MaxSessionMessages, len(session.Messages))
	}
	if session.Messages[0].Content != strings.Repeat("m", 10) {
		t.Error("should keep most recent messages (tail)")
	}
}

func TestTrimSessionMessages_AtLimit(t *testing.T) {
	session := &aipkg.ChatSession{
		Messages: make([]aipkg.ChatMsg, aipkg.MaxSessionMessages),
	}
	trimSessionMessages(session)
	if len(session.Messages) != aipkg.MaxSessionMessages {
		t.Errorf("should not trim at exactly the limit")
	}
}
