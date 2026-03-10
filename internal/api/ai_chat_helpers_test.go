package api

import (
	"strings"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
)

func TestBuildLLMMessages(t *testing.T) {
	history := []aipkg.ChatMsg{
		{Role: aipkg.RoleUser, Content: "Previous question"},
		{Role: aipkg.RoleAssistant, Content: "Previous answer"},
	}
	msgs := buildLLMMessages("System prompt", history, "Current message")

	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (system + 2 history + current), got %d", len(msgs))
	}
	if msgs[0].Role != aipkg.RoleSystem || msgs[0].Content != "System prompt" {
		t.Error("first message should be system prompt")
	}
	if msgs[1].Content != "Previous question" {
		t.Error("second message should be history[0]")
	}
	if msgs[3].Role != aipkg.RoleUser || msgs[3].Content != "Current message" {
		t.Error("last message should be current user message")
	}
}

func TestBuildLLMMessages_EmptyHistory(t *testing.T) {
	msgs := buildLLMMessages("System", nil, "Hello")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
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
