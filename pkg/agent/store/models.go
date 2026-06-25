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

// SkillRun represents one durable execution of a business skill.
type SkillRun struct {
	ID            string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID      string     `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_skill_runs_tenant_skill_status,priority:1" json:"tenant_id"`
	ThreadID      string     `gorm:"column:thread_id;type:varchar(64);index" json:"thread_id,omitempty"`
	TurnID        string     `gorm:"column:turn_id;type:varchar(64);index" json:"turn_id,omitempty"`
	SkillID       string     `gorm:"column:skill_id;type:varchar(128);index:idx_agent_skill_runs_tenant_skill_status,priority:2" json:"skill_id"`
	StoreID       string     `gorm:"column:store_id;type:varchar(255);index" json:"store_id,omitempty"`
	ActorID       string     `gorm:"column:actor_id;type:varchar(255)" json:"actor_id,omitempty"`
	ActingPersona string     `gorm:"column:acting_persona;type:varchar(64)" json:"acting_persona,omitempty"`
	Status        string     `gorm:"column:status;type:varchar(32);index:idx_agent_skill_runs_tenant_skill_status,priority:3" json:"status"`
	Input         string     `gorm:"column:input;type:text" json:"input,omitempty"`
	Output        string     `gorm:"column:output;type:text" json:"output,omitempty"`
	Error         string     `gorm:"column:error;type:text" json:"error,omitempty"`
	StartedAt     time.Time  `gorm:"column:started_at;index" json:"started_at"`
	CompletedAt   *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Artifact represents a durable skill artifact such as source material,
// extracted product candidates, drafts, validation reports, or previews.
type Artifact struct {
	ID          string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_artifacts_tenant_run_kind,priority:1" json:"tenant_id"`
	ThreadID    string    `gorm:"column:thread_id;type:varchar(64);index" json:"thread_id,omitempty"`
	TurnID      string    `gorm:"column:turn_id;type:varchar(64);index" json:"turn_id,omitempty"`
	SkillRunID  string    `gorm:"column:skill_run_id;type:varchar(64);index:idx_agent_artifacts_tenant_run_kind,priority:2" json:"skill_run_id,omitempty"`
	SkillID     string    `gorm:"column:skill_id;type:varchar(128);index" json:"skill_id,omitempty"`
	Kind        string    `gorm:"column:kind;type:varchar(64);index:idx_agent_artifacts_tenant_run_kind,priority:3" json:"kind"`
	Status      string    `gorm:"column:status;type:varchar(32);index" json:"status"`
	Name        string    `gorm:"column:name;type:varchar(256)" json:"name,omitempty"`
	ContentType string    `gorm:"column:content_type;type:varchar(128)" json:"content_type,omitempty"`
	SourceURI   string    `gorm:"column:source_uri;type:text" json:"source_uri,omitempty"`
	SourceName  string    `gorm:"column:source_name;type:varchar(256)" json:"source_name,omitempty"`
	SourceHash  string    `gorm:"column:source_hash;type:varchar(128);index" json:"source_hash,omitempty"`
	Summary     string    `gorm:"column:summary;type:text" json:"summary,omitempty"`
	Data        string    `gorm:"column:data;type:text" json:"data,omitempty"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Approval represents a pending or decided human approval request.
type Approval struct {
	ID             string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_approvals_tenant_status_created,priority:1" json:"tenant_id"`
	ThreadID       string     `gorm:"column:thread_id;type:varchar(64);index" json:"thread_id,omitempty"`
	TurnID         string     `gorm:"column:turn_id;type:varchar(64);index" json:"turn_id,omitempty"`
	ToolCallID     string     `gorm:"column:tool_call_id;type:varchar(128)" json:"tool_call_id,omitempty"`
	SkillID        string     `gorm:"column:skill_id;type:varchar(128)" json:"skill_id,omitempty"`
	StoreID        string     `gorm:"column:store_id;type:varchar(255);index" json:"store_id,omitempty"`
	ActorID        string     `gorm:"column:actor_id;type:varchar(255)" json:"actor_id,omitempty"`
	ActingPersona  string     `gorm:"column:acting_persona;type:varchar(64)" json:"acting_persona,omitempty"`
	Risk           string     `gorm:"column:risk;type:varchar(64)" json:"risk"`
	Action         string     `gorm:"column:action;type:varchar(128)" json:"action"`
	Summary        string     `gorm:"column:summary;type:text" json:"summary"`
	Payload        string     `gorm:"column:payload;type:text" json:"payload,omitempty"`
	ArtifactIDs    string     `gorm:"column:artifact_ids;type:text" json:"artifact_ids,omitempty"`
	RequestHash    string     `gorm:"column:request_hash;type:varchar(128);index" json:"request_hash"`
	IdempotencyKey string     `gorm:"column:idempotency_key;type:varchar(255);index" json:"idempotency_key,omitempty"`
	Status         string     `gorm:"column:status;type:varchar(32);index:idx_agent_approvals_tenant_status_created,priority:2" json:"status"`
	DecisionBy     string     `gorm:"column:decision_by;type:varchar(255)" json:"decision_by,omitempty"`
	DecisionAt     *time.Time `gorm:"column:decision_at" json:"decision_at,omitempty"`
	AppliedBy      string     `gorm:"column:applied_by;type:varchar(255)" json:"applied_by,omitempty"`
	AppliedAt      *time.Time `gorm:"column:applied_at" json:"applied_at,omitempty"`
	ApplyResult    string     `gorm:"column:apply_result;type:text" json:"apply_result,omitempty"`
	ApplyError     string     `gorm:"column:apply_error;type:text" json:"apply_error,omitempty"`
	CreatedAt      time.Time  `gorm:"index:idx_agent_approvals_tenant_status_created,priority:3,sort:desc" json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Memory stores explicit long-lived agent memory scoped to a user, store,
// tenant, thread, or skill. Real-time commerce facts still belong in tools.
type Memory struct {
	ID         string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	TenantID   string     `gorm:"column:tenant_id;type:varchar(255);primaryKey;index:idx_agent_memories_scope_subject,priority:1" json:"tenant_id"`
	Scope      string     `gorm:"column:scope;type:varchar(32);index:idx_agent_memories_scope_subject,priority:2" json:"scope"`
	Subject    string     `gorm:"column:subject;type:varchar(128);index:idx_agent_memories_scope_subject,priority:3" json:"subject,omitempty"`
	StoreID    string     `gorm:"column:store_id;type:varchar(255);index" json:"store_id,omitempty"`
	ActorID    string     `gorm:"column:actor_id;type:varchar(255);index" json:"actor_id,omitempty"`
	Status     string     `gorm:"column:status;type:varchar(32);index" json:"status"`
	Content    string     `gorm:"column:content;type:text" json:"content"`
	Metadata   string     `gorm:"column:metadata;type:text" json:"metadata,omitempty"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
}

func (Thread) TableName() string { return "agent_threads" }

func (Turn) TableName() string { return "agent_turns" }

func (Message) TableName() string { return "agent_messages" }

func (SkillRun) TableName() string { return "agent_skill_runs" }

func (Artifact) TableName() string { return "agent_artifacts" }

func (Approval) TableName() string { return "agent_approvals" }

func (Memory) TableName() string { return "agent_memories" }
