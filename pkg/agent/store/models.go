package store

import "time"

// Thread represents a multi-turn conversation session.
type Thread struct {
	ID         string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID   string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_threads_tenant_active,priority:1" json:"tenant_id"`
	Persona    string    `gorm:"column:persona;type:varchar(64)" json:"persona"`
	Title      string    `gorm:"column:title;type:varchar(256)" json:"title,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	LastActive time.Time `gorm:"column:last_active;index:idx_agent_threads_tenant_active,priority:2,sort:desc" json:"last_active"`
}

// Turn represents a single user→model→(tool)→model cycle within a thread.
type Turn struct {
	ID          string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_turns_tenant_thread,priority:1" json:"tenant_id"`
	ThreadID    string     `gorm:"column:thread_id;type:varchar(64);index:idx_agent_turns_tenant_thread,priority:2" json:"thread_id"`
	TurnIndex   int        `json:"turn_index"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Completed   bool       `json:"completed"`
}

// Message is a single message within a turn.
type Message struct {
	ID         string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID   string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_messages_tenant_thread_created,priority:1" json:"tenant_id"`
	ThreadID   string    `gorm:"column:thread_id;type:varchar(64);index:idx_agent_messages_tenant_thread_created,priority:2" json:"thread_id"`
	TurnID     string    `gorm:"column:turn_id;type:varchar(64);index" json:"turn_id"`
	Role       string    `json:"role"`
	Content    string    `gorm:"type:text" json:"content"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolCalls  string    `gorm:"type:text" json:"tool_calls,omitempty"` // JSON-serialized []ToolCall for assistant messages
	Tokens     int       `json:"tokens"`
	Bytes      int       `json:"bytes"`
	CreatedAt  time.Time `gorm:"index:idx_agent_messages_tenant_thread_created,priority:3" json:"created_at"`
}

func (Thread) TableName() string { return "agent_threads" }

func (Turn) TableName() string { return "agent_turns" }

func (Message) TableName() string { return "agent_messages" }
