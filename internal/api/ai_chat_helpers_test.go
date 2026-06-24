//go:build !private_distribution

package api

import (
	"strings"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
)

func TestVisibleChatMessages_HidesInternalToolTraffic(t *testing.T) {
	messages := []aipkg.ChatMsg{
		{Role: aipkg.RoleSystem, Content: "system prompt"},
		{Role: aipkg.RoleUser, Content: "What should I focus on?"},
		{
			Role: aipkg.RoleAssistant,
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
