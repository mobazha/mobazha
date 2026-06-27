package ai

import (
	"encoding/json"
	"time"
)

// ChatRole represents a message sender role.
type ChatRole string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
	RoleTool      ChatRole = "tool"
)

// ChatMsg is a single message in a conversation, stored in DB and sent to LLM.
type ChatMsg struct {
	Role              ChatRole                `json:"role"`
	Content           string                  `json:"content,omitempty"`
	ContentBlocks     []ChatContentBlock      `json:"contentBlocks,omitempty"`
	AttachmentDisplay []ChatAttachmentDisplay `json:"attachmentDisplay,omitempty"`
	Deliveries        []ChatDelivery          `json:"deliveries,omitempty"`
	ToolCalls         []ToolCall              `json:"tool_calls,omitempty"`
	ToolCallID        string                  `json:"tool_call_id,omitempty"`
	Name              string                  `json:"name,omitempty"`
}

// ChatDelivery is a persisted structured business outcome rendered by chat clients.
type ChatDelivery struct {
	State      string          `json:"state"`
	SkillID    string          `json:"skillId,omitempty"`
	SkillRunID string          `json:"skillRunId,omitempty"`
	MessageKey string          `json:"messageKey"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// ChatAttachmentDisplay is safe UI metadata for rendering user-turn
// attachments in conversation history. It intentionally excludes file bytes.
type ChatAttachmentDisplay struct {
	ArtifactID  string `json:"artifactId,omitempty"`
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
	PreviewURL  string `json:"previewUrl,omitempty"`
}

// ChatContentBlock is a multimodal chat block for providers that support
// text+image input.
type ChatContentBlock struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ChatImageURL `json:"image_url,omitempty"`
}

// ChatImageURL is an OpenAI-compatible image_url block payload.
type ChatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ToolCall represents a function call requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc is the function name + arguments within a tool call.
type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatSession holds a conversation's metadata and history.
type ChatSession struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title,omitempty"`
	Messages  []ChatMsg `json:"messages,omitempty"`
}

// ChatRequest is the incoming request from the frontend.
type ChatRequest struct {
	SessionID string       `json:"sessionId,omitempty"`
	Message   string       `json:"message"`
	Context   *ChatContext `json:"context,omitempty"`
}

// ChatContext carries optional UI hints (not used for authz).
type ChatContext struct {
	CurrentPage       string           `json:"currentPage,omitempty"`
	SelectedListSlug  string           `json:"selectedListingSlug,omitempty"`
	SelectedOrderID   string           `json:"selectedOrderId,omitempty"`
	Locale            string           `json:"locale,omitempty"`
	LatestUserMessage string           `json:"-"`
	ArtifactIDs       []string         `json:"artifactIds,omitempty"`
	SkillRunIDs       []string         `json:"skillRunIds,omitempty"`
	Attachments       []ChatAttachment `json:"attachments,omitempty"`
}

// ChatAttachment describes a file the user attached to the current chat turn.
// Binary payloads may be present for tool handoff, but prompts should only use
// bounded metadata/excerpts.
type ChatAttachment struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	URL           string `json:"url,omitempty"`
	SourceURI     string `json:"sourceUri,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Text          string `json:"text,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
}

// SSEEvent is a server-sent event payload for the chat stream.
type SSEEvent struct {
	Type       string      `json:"type"`
	Content    string      `json:"content,omitempty"`
	Tool       string      `json:"tool,omitempty"`
	ToolID     string      `json:"toolId,omitempty"`
	Args       interface{} `json:"args,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	State      string      `json:"state,omitempty"`
	SkillID    string      `json:"skillId,omitempty"`
	SkillRunID string      `json:"skillRunId,omitempty"`
	MessageKey string      `json:"messageKey,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	SessionID  string      `json:"sessionId,omitempty"`
	Error      string      `json:"error,omitempty"`
}

const (
	SSETypeThinking   = "thinking"
	SSETypeContent    = "content"
	SSETypeToolCall   = "tool_call"
	SSETypeToolResult = "tool_result"
	SSETypeDelivery   = "delivery"
	SSETypeDone       = "done"
	SSETypeError      = "error"
)

const (
	MaxSessionMessages = 40
	MaxUserMessageLen  = 8000
	DefaultMaxTokens   = 4096
	DefaultTemperature = 0.7
	StreamTimeout      = 120 * time.Second
)
