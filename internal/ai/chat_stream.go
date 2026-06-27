package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ToolDefinition is a tool schema passed to the LLM for function calling.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// StreamDelta represents an incremental chunk from the LLM stream.
type StreamDelta struct {
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	Done       bool       `json:"done,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// StreamChat sends a streaming chat completion request to the configured AI provider.
// It returns a channel of StreamDeltas. The caller must drain the channel.
func (p *Proxy) StreamChat(ctx context.Context, cfg Config, messages []ChatMsg, tools []ToolDefinition) (<-chan StreamDelta, error) {
	if !cfg.IsValid() {
		return nil, fmt.Errorf("AI is not configured")
	}
	if IsAnthropicProvider(cfg.Provider) {
		return p.streamAnthropic(ctx, cfg, messages, tools)
	}
	return p.streamOpenAI(ctx, cfg, messages, tools)
}

func (p *Proxy) streamOpenAI(ctx context.Context, cfg Config, messages []ChatMsg, tools []ToolDefinition) (<-chan StreamDelta, error) {
	body := map[string]interface{}{
		"model":      cfg.EffectiveModel(),
		"stream":     true,
		"max_tokens": DefaultMaxTokens,
	}
	if DefaultTemperature > 0 {
		body["temperature"] = DefaultTemperature
	}

	oaiMsgs := convertToOpenAIMessages(messages)
	body["messages"] = oaiMsgs

	if len(tools) > 0 {
		oaiTools := make([]map[string]interface{}, len(tools))
		for i, t := range tools {
			oaiTools[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  json.RawMessage(t.Parameters),
				},
			}
		}
		body["tools"] = oaiTools
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := strings.TrimSuffix(cfg.EffectiveBaseURL(), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := p.clientWithTimeout(StreamTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s", extractErrorMessage(respBody, resp.StatusCode))
	}

	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		parseOpenAIStream(resp.Body, ch)
	}()
	return ch, nil
}

func parseOpenAIStream(body io.Reader, ch chan<- StreamDelta) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	pendingToolCalls := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			flushToolCalls(pendingToolCalls, ch)
			ch <- StreamDelta{Done: true}
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if choice.Delta.Content != "" {
			ch <- StreamDelta{Content: choice.Delta.Content}
		}

		for _, tc := range choice.Delta.ToolCalls {
			existing, ok := pendingToolCalls[tc.Index]
			if !ok {
				existing = &ToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				pendingToolCalls[tc.Index] = existing
			}
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			existing.Function.Arguments += tc.Function.Arguments
		}

		if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
			flushToolCalls(pendingToolCalls, ch)
			if choice.FinishReason == "stop" {
				ch <- StreamDelta{Done: true, StopReason: "stop"}
				return
			}
			ch <- StreamDelta{StopReason: "tool_calls"}
		}
	}
}

func flushToolCalls(pending map[int]*ToolCall, ch chan<- StreamDelta) {
	if len(pending) == 0 {
		return
	}
	var calls []ToolCall
	for _, tc := range pending {
		calls = append(calls, *tc)
	}
	ch <- StreamDelta{ToolCalls: calls}
	for k := range pending {
		delete(pending, k)
	}
}

func (p *Proxy) streamAnthropic(ctx context.Context, cfg Config, messages []ChatMsg, tools []ToolDefinition) (<-chan StreamDelta, error) {
	systemText, anthropicMsgs := convertToAnthropicMessages(messages)

	body := map[string]interface{}{
		"model":      cfg.EffectiveModel(),
		"max_tokens": DefaultMaxTokens,
		"stream":     true,
		"messages":   anthropicMsgs,
	}
	if systemText != "" {
		body["system"] = systemText
	}
	if DefaultTemperature > 0 {
		body["temperature"] = DefaultTemperature
	}

	if len(tools) > 0 {
		anthropicTools := make([]map[string]interface{}, len(tools))
		for i, t := range tools {
			anthropicTools[i] = map[string]interface{}{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": json.RawMessage(t.Parameters),
			}
		}
		body["tools"] = anthropicTools
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := strings.TrimSuffix(cfg.EffectiveBaseURL(), "/") + "/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := p.clientWithTimeout(StreamTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s", extractErrorMessage(respBody, resp.StatusCode))
	}

	ch := make(chan StreamDelta, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		parseAnthropicStream(resp.Body, ch)
	}()
	return ch, nil
}

func parseAnthropicStream(body io.Reader, ch chan<- StreamDelta) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var currentToolID, currentToolName string
	var toolArgsBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				currentToolID = event.ContentBlock.ID
				currentToolName = event.ContentBlock.Name
				toolArgsBuilder.Reset()
			}

		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				ch <- StreamDelta{Content: event.Delta.Text}
			}
			if event.Delta.Type == "input_json_delta" {
				toolArgsBuilder.WriteString(event.Delta.PartialJSON)
			}

		case "content_block_stop":
			if currentToolID != "" {
				ch <- StreamDelta{
					ToolCalls: []ToolCall{{
						ID:   currentToolID,
						Type: "function",
						Function: ToolCallFunc{
							Name:      currentToolName,
							Arguments: toolArgsBuilder.String(),
						},
					}},
				}
				currentToolID = ""
				currentToolName = ""
				toolArgsBuilder.Reset()
			}

		case "message_delta":
			if event.Delta.StopReason == "tool_use" {
				ch <- StreamDelta{StopReason: "tool_calls"}
			} else if event.Delta.StopReason == "end_turn" {
				ch <- StreamDelta{Done: true, StopReason: "stop"}
			}

		case "message_stop":
			ch <- StreamDelta{Done: true}
			return

		case "error":
			ch <- StreamDelta{Error: data}
			return
		}
	}
}

// convertToOpenAIMessages translates ChatMsg to the OpenAI messages format.
func convertToOpenAIMessages(msgs []ChatMsg) []map[string]interface{} {
	var result []map[string]interface{}
	for _, m := range msgs {
		msg := map[string]interface{}{"role": string(m.Role)}

		if len(m.ContentBlocks) > 0 {
			msg["content"] = openAIContentBlocks(m)
		} else if m.Content != "" {
			msg["content"] = m.Content
		}

		if len(m.ToolCalls) > 0 {
			oaiCalls := make([]map[string]interface{}, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				oaiCalls[i] = map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}
			}
			msg["tool_calls"] = oaiCalls
		}

		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		if m.Name != "" {
			msg["name"] = m.Name
		}

		result = append(result, msg)
	}
	return result
}

// convertToAnthropicMessages extracts the system prompt and converts messages
// to Anthropic format, including tool_use and tool_result blocks.
func convertToAnthropicMessages(msgs []ChatMsg) (system string, result []map[string]interface{}) {
	for _, m := range msgs {
		if m.Role == RoleSystem {
			system = m.Content
			continue
		}

		if m.Role == RoleTool {
			result = append(result, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}

		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			var blocks []map[string]interface{}
			if m.Content != "" {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var inputJSON interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputJSON); err != nil {
					inputJSON = map[string]interface{}{}
				}
				blocks = append(blocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": inputJSON,
				})
			}
			result = append(result, map[string]interface{}{
				"role":    "assistant",
				"content": blocks,
			})
			continue
		}

		if len(m.ContentBlocks) > 0 {
			result = append(result, map[string]interface{}{
				"role":    string(m.Role),
				"content": anthropicContentBlocks(m),
			})
			continue
		}

		result = append(result, map[string]interface{}{
			"role":    string(m.Role),
			"content": m.Content,
		})
	}
	return
}

func openAIContentBlocks(msg ChatMsg) []map[string]interface{} {
	blocks := make([]map[string]interface{}, 0, len(msg.ContentBlocks)+1)
	if msg.Content != "" {
		blocks = append(blocks, map[string]interface{}{"type": "text", "text": msg.Content})
	}
	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": block.Text})
			}
		case "image_url":
			if block.ImageURL != nil && strings.TrimSpace(block.ImageURL.URL) != "" {
				image := map[string]string{"url": block.ImageURL.URL}
				if block.ImageURL.Detail != "" {
					image["detail"] = block.ImageURL.Detail
				}
				blocks = append(blocks, map[string]interface{}{"type": "image_url", "image_url": image})
			}
		}
	}
	if len(blocks) == 0 {
		return []map[string]interface{}{{"type": "text", "text": ""}}
	}
	return blocks
}

func anthropicContentBlocks(msg ChatMsg) []map[string]interface{} {
	blocks := make([]map[string]interface{}, 0, len(msg.ContentBlocks)+1)
	if msg.Content != "" {
		blocks = append(blocks, map[string]interface{}{"type": "text", "text": msg.Content})
	}
	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": block.Text})
			}
		case "image_url":
			if block.ImageURL != nil && strings.TrimSpace(block.ImageURL.URL) != "" {
				source := anthropicImageSource(block.ImageURL.URL)
				blocks = append(blocks, map[string]interface{}{
					"type":   "image",
					"source": source,
				})
			}
		}
	}
	if len(blocks) == 0 {
		return []map[string]interface{}{{"type": "text", "text": ""}}
	}
	return blocks
}

func anthropicImageSource(rawURL string) map[string]interface{} {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(strings.ToLower(rawURL), "data:image/") {
		header, data, ok := strings.Cut(rawURL, ",")
		if ok {
			mediaType := strings.TrimPrefix(header, "data:")
			mediaType = strings.TrimSuffix(mediaType, ";base64")
			if mediaType != "" && data != "" {
				return map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				}
			}
		}
	}
	return map[string]interface{}{
		"type": "url",
		"url":  rawURL,
	}
}
