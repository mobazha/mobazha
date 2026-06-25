package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// SkillID is the stable dot-style identifier used by runtime, docs, and UI.
type SkillID string

const (
	SkillProductImport SkillID = "product.import"
)

// Persona is the role the user is acting as for the current agent turn.
// A user may hold multiple roles, but the runtime should pick one acting
// persona per turn to avoid leaking cross-role skills or tools.
type Persona string

const (
	PersonaSeller    Persona = "seller"
	PersonaBuyer     Persona = "buyer"
	PersonaModerator Persona = "moderator"
	PersonaOperator  Persona = "operator"
)

// Capability is an abstract business permission requested by a skill and
// provided by one or more concrete tools. Capabilities keep skill manifests
// stable when tool names or implementations change.
type Capability string

const (
	CapabilityListingRead               Capability = "listing.read"
	CapabilityListingDraftWrite         Capability = "listing.draft_write"
	CapabilityListingApplyAfterApproval Capability = "listing.apply_after_approval"
	CapabilityOrderRead                 Capability = "order.read"
	CapabilityOrderWrite                Capability = "order.write"
	CapabilityOrderFulfillmentWrite     Capability = "order.fulfillment_write"
	CapabilityOrderFinancial            Capability = "order.financial"
	CapabilityDiscountWrite             Capability = "discount.write"
	CapabilityCollectionRead            Capability = "collection.read"
	CapabilityCollectionWrite           Capability = "collection.write"
	CapabilityExchangeRatesRead         Capability = "exchange.rates.read"
	CapabilityProfileWrite              Capability = "profile.write"
	CapabilityChatRead                  Capability = "chat.read"
	CapabilityChatWrite                 Capability = "chat.write"
	CapabilityAgentArtifactRead         Capability = "agent.artifact.read"
	CapabilityAgentArtifactWrite        Capability = "agent.artifact.write"
)

// Scope identifies the actor and commerce boundary for an agent operation.
type Scope struct {
	TenantID      string    `json:"tenantId"`
	StoreID       string    `json:"storeId,omitempty"`
	ActorID       string    `json:"actorId,omitempty"`
	ActorRole     string    `json:"actorRole,omitempty"`
	ActorRoles    []Persona `json:"actorRoles,omitempty"`
	ActingPersona Persona   `json:"actingPersona,omitempty"`
}

// Risk describes the highest impact level of a tool or approval request.
type Risk string

const (
	RiskRead      Risk = "read"
	RiskDraft     Risk = "draft"
	RiskWrite     Risk = "write"
	RiskFinancial Risk = "financial"
	RiskDangerous Risk = "dangerous"
)

// ApprovalMode describes whether an action can run automatically.
type ApprovalMode string

const (
	ApprovalNone     ApprovalMode = "none"
	ApprovalReview   ApprovalMode = "review"
	ApprovalExplicit ApprovalMode = "explicit"
)

// SideEffect describes whether a tool mutates external state.
type SideEffect string

const (
	SideEffectNone       SideEffect = "none"
	SideEffectIdempotent SideEffect = "idempotent"
	SideEffectMutable    SideEffect = "mutable"
)

// ToolMetadata is the catalog contract used by skills and policy enforcement.
// Runtime can enforce the generic fields without knowing commerce semantics.
type ToolMetadata struct {
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace,omitempty"`
	Version         string          `json:"version,omitempty"`
	Description     string          `json:"description,omitempty"`
	InputSchema     json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema    json.RawMessage `json:"outputSchema,omitempty"`
	Risk            Risk            `json:"risk"`
	Approval        ApprovalMode    `json:"approval"`
	SideEffect      SideEffect      `json:"sideEffect"`
	Idempotent      bool            `json:"idempotent"`
	Parallelizable  bool            `json:"parallelizable"`
	Timeout         time.Duration   `json:"timeout,omitempty"`
	Capabilities    []Capability    `json:"capabilities,omitempty"`
	AllowedSkills   []SkillID       `json:"allowedSkills,omitempty"`
	AllowedPersonas []Persona       `json:"allowedPersonas,omitempty"`
	ResultMode      string          `json:"resultMode,omitempty"`
}

// ToolCatalog resolves tool metadata for runtime policy and skill planning.
type ToolCatalog interface {
	List(ctx context.Context, scope Scope) ([]ToolMetadata, error)
	Resolve(ctx context.Context, scope Scope, name string) (*ToolMetadata, error)
}

// ApprovalRequest is the stable record for human-in-the-loop writes.
type ApprovalRequest struct {
	ID             string          `json:"id"`
	SkillID        SkillID         `json:"skillId"`
	Scope          Scope           `json:"scope"`
	Risk           Risk            `json:"risk"`
	Action         string          `json:"action"`
	Summary        string          `json:"summary"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	RequestHash    string          `json:"requestHash"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// ComputeApprovalHash returns a deterministic hash for replay and approval
// binding. Runtime-only identity fields are excluded so callers can recompute it
// for the same approved action.
func ComputeApprovalHash(req ApprovalRequest) (string, error) {
	req.ID = ""
	req.RequestHash = ""
	req.CreatedAt = time.Time{}
	req.Scope.ActorRole = ""
	req.Scope.ActorRoles = nil
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// SkillRun records one execution of a business skill.
type SkillRun struct {
	ID          string          `json:"id"`
	SkillID     SkillID         `json:"skillId"`
	Scope       Scope           `json:"scope"`
	ThreadID    string          `json:"threadId,omitempty"`
	TurnID      string          `json:"turnId,omitempty"`
	Status      string          `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Artifacts   []ArtifactRef   `json:"artifacts,omitempty"`
	Approvals   []string        `json:"approvals,omitempty"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
}

// ArtifactRef points to a durable artifact without forcing storage details into
// the kernel package.
type ArtifactRef struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	ContentType string `json:"contentType,omitempty"`
	Name        string `json:"name,omitempty"`
	URI         string `json:"uri,omitempty"`
}

// ContextBudget captures token allocation and compression decisions per turn.
type ContextBudget struct {
	ModelContextTokens int            `json:"modelContextTokens"`
	ReservedOutput     int            `json:"reservedOutput"`
	Buckets            map[string]int `json:"buckets,omitempty"`
	ShouldCompact      bool           `json:"shouldCompact"`
	Reason             string         `json:"reason,omitempty"`
}

// MemoryScope controls who can read a memory item.
type MemoryScope string

const (
	MemoryUser       MemoryScope = "user"
	MemoryStoreScope MemoryScope = "store"
	MemoryTenant     MemoryScope = "tenant"
	MemoryThread     MemoryScope = "thread"
	MemorySkill      MemoryScope = "skill"
)

// MemoryItem is the durable memory contract. Ranking and personalization
// policy may be injected by closed-source providers.
type MemoryItem struct {
	ID        string            `json:"id"`
	Scope     MemoryScope       `json:"scope"`
	Subject   string            `json:"subject"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type MemoryQuery struct {
	Scope   Scope         `json:"scope"`
	Types   []MemoryScope `json:"types,omitempty"`
	Subject string        `json:"subject,omitempty"`
	Query   string        `json:"query,omitempty"`
	Limit   int           `json:"limit,omitempty"`
}

type MemoryStore interface {
	Search(ctx context.Context, q MemoryQuery) ([]MemoryItem, error)
	Save(ctx context.Context, scope Scope, item MemoryItem) error
	Delete(ctx context.Context, scope Scope, id string) error
}
