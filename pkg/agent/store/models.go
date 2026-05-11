package store

import "time"

// Thread represents a multi-turn conversation session.
type Thread struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	TenantID   string    `gorm:"index;not null" json:"tenant_id"`
	Persona    string    `json:"persona"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

// Turn represents a single user→model→(tool)→model cycle within a thread.
type Turn struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	ThreadID  string    `gorm:"index" json:"thread_id"`
	TenantID  string    `gorm:"index;not null" json:"tenant_id"`
	TurnIndex int       `json:"turn_index"`
	StartedAt time.Time `json:"started_at"`
	Completed bool      `json:"completed"`
}

// Message is a single message within a turn.
type Message struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	TurnID     string    `gorm:"index" json:"turn_id"`
	TenantID   string    `gorm:"index;not null" json:"tenant_id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolCalls  string    `json:"tool_calls,omitempty"` // JSON-serialized []ToolCall for assistant messages
	Tokens     int       `json:"tokens"`
	Bytes      int       `json:"bytes"`
	CreatedAt  time.Time `json:"created_at"`
}
