package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aipkg "github.com/mobazha/mobazha/internal/ai"
	agentexec "github.com/mobazha/mobazha/pkg/agent/exec"
)

func TestResolveAgentChatAttachment_SingleAttachment(t *testing.T) {
	attachments := []aipkg.ChatAttachment{{ID: "att_1", Name: "cover.jpg"}}
	got, err := resolveAgentChatAttachment(attachments, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "att_1" {
		t.Fatalf("expected att_1, got %#v", got)
	}
}

func TestResolveAgentChatAttachment_ByID(t *testing.T) {
	attachments := []aipkg.ChatAttachment{
		{ID: "att_a", Name: "a.jpg"},
		{ID: "att_b", Name: "b.jpg"},
	}
	got, err := resolveAgentChatAttachment(attachments, "att_b", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "b.jpg" {
		t.Fatalf("expected b.jpg, got %#v", got)
	}
}

func TestExecuteAgentAttachmentsAnalyze_TextExcerpt(t *testing.T) {
	result, err := executeAgentAttachmentsAnalyze(context.Background(), agentToolContext{
		attachments: []aipkg.ChatAttachment{{
			Name:        "supplier.csv",
			ContentType: "text/csv",
			Text:        "title,price\nLinen Tote,45\n",
		}},
	}, agentexec.ToolCall{
		ID:        "call_1",
		Name:      "agent_attachments_analyze",
		Arguments: `{"question":"Summarize this file"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Data agentAttachmentAnalyzeResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Mode != "text_excerpt" {
		t.Fatalf("expected text_excerpt mode, got %#v", payload.Data)
	}
	if !strings.Contains(payload.Data.Analysis, "Linen Tote") {
		t.Fatalf("expected excerpt in analysis, got %q", payload.Data.Analysis)
	}
}

func TestExecuteAgentAttachmentsAnalyze_ImageUsesVision(t *testing.T) {
	vision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"A canvas tote bag on a table."}}]}`))
	}))
	defer vision.Close()

	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{
			Vision: &aipkg.Config{
				Provider:   "openai",
				APIKey:     "test-key",
				Model:      "gpt-4o",
				BaseURL:    vision.URL,
				Enabled:    true,
				IsPlatform: true,
				DailyLimit: 20,
			},
		}),
		proxy:       aipkg.NewProxy(vision.Client()),
		rateLimiter: aipkg.NewDailyRateLimiter(),
	}
	result, err := executeAgentAttachmentsAnalyze(context.Background(), agentToolContext{
		attachments: []aipkg.ChatAttachment{{
			Name:          "product.jpg",
			ContentType:   "image/jpeg",
			ContentBase64: "/9j/4AAQ",
		}},
		provider: node,
		origin:   "http://127.0.0.1:8080",
		actorID:  "test-node",
	}, agentexec.ToolCall{
		ID:        "call_1",
		Name:      "agent_attachments_analyze",
		Arguments: `{"question":"What is in this image?"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Data agentAttachmentAnalyzeResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Mode != "vision" {
		t.Fatalf("expected vision mode, got %#v", payload.Data)
	}
	if !strings.Contains(payload.Data.Analysis, "canvas tote bag") {
		t.Fatalf("unexpected analysis: %q", payload.Data.Analysis)
	}
}

func TestHandlePOSTAgentAttachmentsAnalyze_TextExcerpt(t *testing.T) {
	vision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("vision should not be called for text excerpt")
	}))
	defer vision.Close()

	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		proxy:            aipkg.NewProxy(vision.Client()),
	}
	body := strings.NewReader(`{"question":"Summarize this file","attachments":[{"name":"supplier.csv","contentType":"text/csv","text":"title,price\nLinen Tote,45\n"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/attachments/analyze", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentAttachmentsAnalyze(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Data agentAttachmentAnalyzeResult `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Mode != "text_excerpt" || !strings.Contains(payload.Data.Analysis, "Linen Tote") {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}
