package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOpenAIStream_ContentOnly(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":" World"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		parseOpenAIStream(strings.NewReader(sseData), ch)
	}()

	var content strings.Builder
	var gotDone bool
	for d := range ch {
		content.WriteString(d.Content)
		if d.Done {
			gotDone = true
		}
	}
	if content.String() != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", content.String())
	}
	if !gotDone {
		t.Error("expected done signal")
	}
}

func TestParseOpenAIStream_ToolCalls(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"listings_list_mine","arguments":""}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"limit"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\": 10}"}}]},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`
	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		parseOpenAIStream(strings.NewReader(sseData), ch)
	}()

	var toolCalls []ToolCall
	for d := range ch {
		toolCalls = append(toolCalls, d.ToolCalls...)
	}
	if len(toolCalls) == 0 {
		t.Fatal("expected tool calls")
	}
	tc := toolCalls[0]
	if tc.Function.Name != "listings_list_mine" {
		t.Errorf("expected tool name 'listings_list_mine', got %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"limit": 10}` {
		t.Errorf("expected accumulated arguments, got %q", tc.Function.Arguments)
	}
}

func TestParseAnthropicStream_ContentOnly(t *testing.T) {
	sseData := `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" Anthropic"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

data: {"type":"message_stop"}

`
	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		parseAnthropicStream(strings.NewReader(sseData), ch)
	}()

	var content strings.Builder
	var gotDone bool
	for d := range ch {
		content.WriteString(d.Content)
		if d.Done {
			gotDone = true
		}
	}
	if content.String() != "Hello Anthropic" {
		t.Errorf("expected 'Hello Anthropic', got %q", content.String())
	}
	if !gotDone {
		t.Error("expected done signal")
	}
}

func TestParseAnthropicStream_ToolUse(t *testing.T) {
	sseData := `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"profile_get"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}

data: {"type":"message_stop"}

`
	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		parseAnthropicStream(strings.NewReader(sseData), ch)
	}()

	var toolCalls []ToolCall
	var gotToolCallsStop bool
	for d := range ch {
		toolCalls = append(toolCalls, d.ToolCalls...)
		if d.StopReason == "tool_calls" {
			gotToolCallsStop = true
		}
	}
	if len(toolCalls) == 0 {
		t.Fatal("expected tool calls")
	}
	if toolCalls[0].Function.Name != "profile_get" {
		t.Errorf("expected 'profile_get', got %q", toolCalls[0].Function.Name)
	}
	if !gotToolCallsStop {
		t.Error("expected tool_calls stop reason")
	}
}

func TestConvertToOpenAIMessages_Basic(t *testing.T) {
	msgs := []ChatMsg{
		{Role: RoleSystem, Content: "System prompt"},
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
	}
	result := convertToOpenAIMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0]["role"] != "system" {
		t.Error("first message should be system")
	}
	if result[1]["content"] != "Hello" {
		t.Error("second message content mismatch")
	}
}

func TestConvertToOpenAIMessages_WithToolCalls(t *testing.T) {
	msgs := []ChatMsg{
		{
			Role:    RoleAssistant,
			Content: "Let me check",
			ToolCalls: []ToolCall{{
				ID: "call_1",
				Function: ToolCallFunc{
					Name:      "profile_get",
					Arguments: "{}",
				},
			}},
		},
		{
			Role:       RoleTool,
			Content:    `{"name":"My Store"}`,
			ToolCallID: "call_1",
			Name:       "profile_get",
		},
	}
	result := convertToOpenAIMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	toolCalls, ok := result[0]["tool_calls"].([]map[string]interface{})
	if !ok || len(toolCalls) != 1 {
		t.Fatal("first message should have tool_calls")
	}
	if result[1]["tool_call_id"] != "call_1" {
		t.Error("tool result should have tool_call_id")
	}
}

func TestConvertToAnthropicMessages_ExtractsSystem(t *testing.T) {
	msgs := []ChatMsg{
		{Role: RoleSystem, Content: "System prompt here"},
		{Role: RoleUser, Content: "Hello"},
	}
	system, result := convertToAnthropicMessages(msgs)
	if system != "System prompt here" {
		t.Errorf("expected system prompt, got %q", system)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 message (user only, system extracted), got %d", len(result))
	}
	if result[0]["role"] != "user" {
		t.Error("remaining message should be user")
	}
}

func TestConvertToAnthropicMessages_ToolResults(t *testing.T) {
	msgs := []ChatMsg{
		{
			Role:       RoleTool,
			Content:    `{"result":"ok"}`,
			ToolCallID: "toolu_1",
			Name:       "profile_get",
		},
	}
	_, result := convertToAnthropicMessages(msgs)
	if len(result) != 1 {
		t.Fatal("expected 1 message")
	}
	if result[0]["role"] != "user" {
		t.Error("tool result should be mapped to user role in Anthropic format")
	}
	content, ok := result[0]["content"].([]map[string]interface{})
	if !ok || len(content) != 1 {
		t.Fatal("content should be array with one tool_result block")
	}
	if content[0]["type"] != "tool_result" {
		t.Error("block type should be tool_result")
	}
}

func TestConvertToAnthropicMessages_AssistantToolUse(t *testing.T) {
	msgs := []ChatMsg{
		{
			Role:    RoleAssistant,
			Content: "Checking...",
			ToolCalls: []ToolCall{{
				ID:   "toolu_1",
				Type: "function",
				Function: ToolCallFunc{
					Name:      "listings_list_mine",
					Arguments: `{"limit":5}`,
				},
			}},
		},
	}
	_, result := convertToAnthropicMessages(msgs)
	if len(result) != 1 {
		t.Fatal("expected 1 message")
	}
	content, ok := result[0]["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("assistant with tool_calls should have content array")
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 blocks (text + tool_use), got %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Error("first block should be text")
	}
	if content[1]["type"] != "tool_use" {
		t.Error("second block should be tool_use")
	}
	input := content[1]["input"]
	inputJSON, _ := json.Marshal(input)
	if !strings.Contains(string(inputJSON), "limit") {
		t.Errorf("tool_use input should contain parsed arguments, got %s", string(inputJSON))
	}
}
