package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/agent/kernel"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/redact"
	"gorm.io/gorm"
)

// Common errors.
var (
	ErrThreadNotFound           = errors.New("agent: thread not found")
	ErrApprovalNotFound         = errors.New("agent: approval not found")
	ErrApprovalClaimConflict    = errors.New("agent: approval apply claim conflict")
	ErrArtifactApprovalConflict = errors.New("agent: artifact approval changed concurrently")
	ErrArtifactVersionConflict  = errors.New("agent: artifact changed concurrently")
	ErrSkillRunNotFound         = errors.New("agent: skill run not found")
	ErrArtifactNotFound         = errors.New("agent: artifact not found")
	ErrArtifactContentNotFound  = errors.New("agent: artifact content not found")
	ErrMemoryNotFound           = errors.New("agent: memory not found")
)

const (
	// SkillRunStatusCreated means the skill run has been created but not started.
	SkillRunStatusCreated = "created"
	// SkillRunStatusRunning means the skill run is actively producing artifacts.
	SkillRunStatusRunning = "running"
	// SkillRunStatusWaitingForReview means the skill run is waiting for user review.
	SkillRunStatusWaitingForReview = "waiting_for_review"
	// SkillRunStatusWaitingForApproval means the skill run is waiting for approval.
	SkillRunStatusWaitingForApproval = "waiting_for_approval"
	// SkillRunStatusCompleted means the skill run completed successfully.
	SkillRunStatusCompleted = "completed"
	// SkillRunStatusFailed means the skill run failed.
	SkillRunStatusFailed = "failed"

	// ArtifactKindSourceMaterial stores uploaded or pasted source material metadata.
	ArtifactKindSourceMaterial = "source_material"
	// ArtifactKindCandidate stores extracted candidates for a skill-specific task.
	ArtifactKindCandidate = "candidate"
	// ArtifactKindProposal stores reviewable proposals before a business write.
	ArtifactKindProposal = "proposal"
	// ArtifactKindValidationReport stores validation results.
	ArtifactKindValidationReport = "validation_report"

	// ArtifactStatusNew means the artifact has just been created.
	ArtifactStatusNew = "new"
	// ArtifactStatusNeedsReview means the artifact needs human review.
	ArtifactStatusNeedsReview = "needs_review"
	// ArtifactStatusReady means the artifact is ready for approval or apply.
	ArtifactStatusReady = "ready"
	// ArtifactStatusSkipped means the artifact was intentionally skipped.
	ArtifactStatusSkipped = "skipped"
	// ArtifactStatusApplied means the artifact has been applied to commerce state.
	ArtifactStatusApplied = "applied"

	// ApprovalStatusPending means a tool call is waiting for human approval.
	ApprovalStatusPending = "pending"
	// ApprovalStatusApproved means a human approved the pending tool call.
	ApprovalStatusApproved = "approved"
	// ApprovalStatusRejected means a human rejected the pending tool call.
	ApprovalStatusRejected = "rejected"
	// ApprovalStatusApplying means an approved tool call is currently applying.
	ApprovalStatusApplying = "applying"
	// ApprovalStatusApplied means the approved tool call has been applied.
	ApprovalStatusApplied = "applied"
	// ApprovalStatusApplyFailed means the approved tool call failed while applying.
	ApprovalStatusApplyFailed = "apply_failed"
	// ApprovalStatusSuperseded means the underlying proposal changed and requires a fresh approval.
	ApprovalStatusSuperseded = "superseded"

	// MemoryStatusActive means a memory is available for retrieval.
	MemoryStatusActive = "active"
	// MemoryStatusArchived means a memory is retained but no longer retrieved.
	MemoryStatusArchived = "archived"

	// TurnStatusRunning means the turn has started but has not reached a terminal state.
	TurnStatusRunning = "running"
	// TurnStatusCompleted means the turn produced a final assistant response.
	TurnStatusCompleted = "completed"
	// TurnStatusFailed means the turn reached a terminal error after it was created.
	TurnStatusFailed = "failed"
)

// Persistence provides durable storage for agent threads, turns, and messages.
type Persistence interface {
	SaveThread(ctx context.Context, t *Thread) error
	SaveTurn(ctx context.Context, t *Turn) error
	SaveMessage(ctx context.Context, m *Message) error
	SaveCompactionCheckpoint(ctx context.Context, checkpoint CompactionCheckpoint) (bool, error)
	FinalizeTurn(ctx context.Context, t *Turn, messages []*Message) error
	RecoverStaleTurns(ctx context.Context, tenantID, threadID string, staleBefore time.Time) (int64, error)
	SaveSkillRun(ctx context.Context, r *SkillRun) error
	SaveArtifact(ctx context.Context, a *Artifact) error
	SaveArtifactWithContent(ctx context.Context, a *Artifact, content *ArtifactContent) error
	SaveArtifactAndRefreshApproval(ctx context.Context, a *Artifact, toolCallID string, expectedUpdatedAt time.Time, replacement *Approval) error
	SaveApproval(ctx context.Context, a *Approval) error
	LoadThread(ctx context.Context, tenantID, threadID string) (*Thread, error)
	ListThreads(ctx context.Context, tenantID string, limit, offset int) ([]*Thread, error)
	LoadMessages(ctx context.Context, tenantID, threadID string) ([]*Message, error)
	LoadRecentMessages(ctx context.Context, tenantID, threadID string, limit int) ([]*Message, error)
	LoadSkillRun(ctx context.Context, tenantID, runID string) (*SkillRun, error)
	ListSkillRuns(ctx context.Context, tenantID, skillID, status string, limit, offset int) ([]*SkillRun, error)
	LoadArtifact(ctx context.Context, tenantID, artifactID string) (*Artifact, error)
	LoadArtifactContent(ctx context.Context, tenantID, artifactID string) (*ArtifactContent, error)
	ListArtifacts(ctx context.Context, tenantID, skillRunID, kind, status string, limit, offset int) ([]*Artifact, error)
	LoadApproval(ctx context.Context, tenantID, approvalID string) (*Approval, error)
	ListApprovals(ctx context.Context, tenantID, status string, limit, offset int) ([]*Approval, error)
	UpdateApprovalStatus(ctx context.Context, tenantID, approvalID, status, actorID string) (*Approval, error)
	ClaimApprovalForApply(ctx context.Context, tenantID, approvalID, actorID string) (*Approval, error)
	MarkApprovalApplied(ctx context.Context, tenantID, approvalID, result, actorID string) (*Approval, error)
	MarkApprovalApplyFailed(ctx context.Context, tenantID, approvalID, applyErr, actorID string) (*Approval, error)
	DeleteThread(ctx context.Context, tenantID, threadID string) error
}

// GormPersistence stores agent runtime state in the tenant-scoped node DB.
type GormPersistence struct {
	db pkgdb.Database
}

var _ Persistence = (*GormPersistence)(nil)

// MemoryUpdate describes mutable user-managed memory fields.
type MemoryUpdate struct {
	Subject  *string
	Content  *string
	Metadata *map[string]string
}

// NewGormPersistence creates a durable agent runtime persistence adapter.
func NewGormPersistence(db pkgdb.Database) *GormPersistence {
	return &GormPersistence{db: db}
}

// TenantID returns the tenant scope enforced by the underlying database.
func (p *GormPersistence) TenantID() string {
	if p == nil || p.db == nil {
		return ""
	}
	type tenantIDProvider interface {
		TenantID() string
	}
	if scoped, ok := p.db.(tenantIDProvider); ok {
		return scoped.TenantID()
	}
	return ""
}

// MigrateModels creates or updates the agent runtime tables.
func MigrateModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		for _, model := range []interface{}{&Thread{}, &Turn{}, &Message{}, &SkillRun{}, &Artifact{}, &ArtifactContent{}, &Approval{}, &Memory{}} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	})
}

// Save stores or updates an explicit memory item for the given actor scope.
func (p *GormPersistence) Save(_ context.Context, scope kernel.Scope, item kernel.MemoryItem) error {
	if p == nil || p.db == nil {
		return nil
	}
	memory, err := memoryFromKernel(scope, item)
	if err != nil {
		return err
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(memory)
	})
}

// Search returns active memories visible to the provided agent scope.
func (p *GormPersistence) Search(_ context.Context, q kernel.MemoryQuery) ([]kernel.MemoryItem, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}
	var records []Memory
	err := p.db.View(func(tx pkgdb.Tx) error {
		query := tx.Read().
			Where("tenant_id = ? AND status = ?", q.Scope.TenantID, MemoryStatusActive)
		if len(q.Types) > 0 {
			scopes := make([]string, 0, len(q.Types))
			for _, s := range q.Types {
				scopes = append(scopes, string(s))
			}
			query = query.Where("scope IN (?)", scopes)
		}
		if q.Subject != "" {
			query = query.Where("subject = ?", q.Subject)
		}
		query = visibleMemoryQuery(query, q.Scope)
		if q.Query != "" {
			like := "%" + q.Query + "%"
			query = query.Where("(content LIKE ? OR subject LIKE ?)", like, like)
		}
		return query.
			Order("updated_at DESC").
			Limit(limit).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	if err := p.touchRetrievedMemories(records); err != nil {
		return nil, err
	}
	out := make([]kernel.MemoryItem, 0, len(records))
	for _, record := range records {
		out = append(out, record.toKernel())
	}
	return out, nil
}

// Delete archives a memory visible to the provided scope.
func (p *GormPersistence) Delete(_ context.Context, scope kernel.Scope, id string) error {
	if p == nil || p.db == nil {
		return nil
	}
	now := time.Now()
	return p.db.Update(func(tx pkgdb.Tx) error {
		var memory Memory
		query := tx.Read().Where("tenant_id = ? AND id = ?", scope.TenantID, id)
		if err := visibleMemoryQuery(query, scope).First(&memory).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemoryNotFound
			}
			return err
		}
		where := map[string]interface{}{
			"tenant_id = ?": scope.TenantID,
			"id = ?":        id,
		}
		rows, err := tx.UpdateColumns(
			map[string]interface{}{
				"status":     MemoryStatusArchived,
				"updated_at": now,
			},
			where,
			&Memory{},
		)
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrMemoryNotFound
		}
		return nil
	})
}

// UpdateMemory updates mutable fields on a memory visible to the provided scope.
func (p *GormPersistence) UpdateMemory(_ context.Context, scope kernel.Scope, id string, update MemoryUpdate) (*kernel.MemoryItem, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrMemoryNotFound
	}
	values, err := memoryUpdateColumns(update)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	values["updated_at"] = now
	var memory Memory
	err = p.db.Update(func(tx pkgdb.Tx) error {
		query := tx.Read().Where("tenant_id = ? AND id = ? AND status = ?", scope.TenantID, id, MemoryStatusActive)
		if err := visibleMemoryQuery(query, scope).First(&memory).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemoryNotFound
			}
			return err
		}
		rows, err := tx.UpdateColumns(values, map[string]interface{}{
			"tenant_id = ?": scope.TenantID,
			"id = ?":        id,
			"status = ?":    MemoryStatusActive,
		}, &Memory{})
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrMemoryNotFound
		}
		applyMemoryUpdateColumns(&memory, values)
		memory.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	item := memory.toKernel()
	return &item, nil
}

// SaveSkillRun persists a durable skill run.
func (p *GormPersistence) SaveSkillRun(_ context.Context, r *SkillRun) error {
	if p == nil || p.db == nil {
		return nil
	}
	if r == nil {
		return fmt.Errorf("agent store: skill run is nil")
	}
	cp := *r
	now := time.Now()
	if cp.StartedAt.IsZero() {
		cp.StartedAt = now
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = now
	}
	if cp.Status == "" {
		cp.Status = SkillRunStatusCreated
	}
	cp.Input = sanitizeJSONText(cp.Input)
	cp.Output = sanitizeJSONText(cp.Output)
	cp.Error = truncateStoreText(cp.Error, 2000)
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

// SaveArtifact persists a durable skill artifact.
func (p *GormPersistence) SaveArtifact(_ context.Context, a *Artifact) error {
	if p == nil || p.db == nil {
		return nil
	}
	cp, err := prepareArtifactForSave(a)
	if err != nil {
		return err
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

// SaveArtifactWithContent atomically persists artifact metadata and its private
// binary content in the tenant database.
func (p *GormPersistence) SaveArtifactWithContent(_ context.Context, a *Artifact, content *ArtifactContent) error {
	if p == nil || p.db == nil {
		return nil
	}
	cp, err := prepareArtifactForSave(a)
	if err != nil {
		return err
	}
	if content == nil || len(content.Data) == 0 {
		return p.db.Update(func(tx pkgdb.Tx) error {
			return tx.Save(&cp)
		})
	}
	contentCopy := *content
	if contentCopy.ArtifactID == "" {
		contentCopy.ArtifactID = cp.ID
	}
	if contentCopy.TenantID == "" {
		contentCopy.TenantID = cp.TenantID
	}
	if contentCopy.ArtifactID != cp.ID || contentCopy.TenantID != cp.TenantID {
		return fmt.Errorf("agent store: artifact content identity mismatch")
	}
	contentCopy.ThreadID = cp.ThreadID
	if contentCopy.ContentType == "" {
		contentCopy.ContentType = cp.ContentType
	}
	contentCopy.Bytes = int64(len(contentCopy.Data))
	now := time.Now()
	if contentCopy.CreatedAt.IsZero() {
		contentCopy.CreatedAt = now
	}
	if contentCopy.UpdatedAt.IsZero() {
		contentCopy.UpdatedAt = now
	}
	cp.ContentBytes = contentCopy.Bytes
	a.ContentBytes = contentCopy.Bytes
	return p.db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Save(&cp); err != nil {
			return err
		}
		return tx.Save(&contentCopy)
	})
}

// SaveArtifactAndRefreshApproval atomically saves proposal changes, supersedes
// any active approval for the same tool call, and persists a fresh pending
// approval when one is supplied. A proposal already being applied is immutable.
func (p *GormPersistence) SaveArtifactAndRefreshApproval(_ context.Context, a *Artifact, toolCallID string, expectedUpdatedAt time.Time, replacement *Approval) error {
	if p == nil || p.db == nil {
		return nil
	}
	artifact, err := prepareArtifactForSave(a)
	if err != nil {
		return err
	}
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return fmt.Errorf("agent store: approval tool call ID is required")
	}
	if expectedUpdatedAt.IsZero() {
		return fmt.Errorf("agent store: artifact expected update time is required")
	}

	var preparedReplacement *Approval
	if replacement != nil {
		cp, err := prepareApprovalForSave(replacement)
		if err != nil {
			return err
		}
		if cp.TenantID != artifact.TenantID || cp.ToolCallID != toolCallID || cp.Status != ApprovalStatusPending {
			return fmt.Errorf("agent store: replacement approval scope mismatch")
		}
		preparedReplacement = &cp
	}

	return p.db.Update(func(tx pkgdb.Tx) error {
		var current Artifact
		if err := tx.Read().
			Where("tenant_id = ? AND id = ?", artifact.TenantID, artifact.ID).
			First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArtifactNotFound
			}
			return err
		}
		if !current.UpdatedAt.Equal(expectedUpdatedAt) {
			return ErrArtifactVersionConflict
		}

		var active []Approval
		if err := tx.Read().
			Where("tenant_id = ? AND tool_call_id = ? AND status IN (?)", artifact.TenantID, toolCallID, []string{
				ApprovalStatusPending,
				ApprovalStatusApproved,
				ApprovalStatusApplying,
				ApprovalStatusApplyFailed,
				ApprovalStatusApplied,
			}).
			Find(&active).Error; err != nil {
			return err
		}
		for _, approval := range active {
			if approval.Status == ApprovalStatusApplying || approval.Status == ApprovalStatusApplied {
				return ErrArtifactApprovalConflict
			}
		}

		if len(active) > 0 {
			now := time.Now()
			rows, err := tx.UpdateColumns(map[string]interface{}{
				"status":      ApprovalStatusSuperseded,
				"decision_by": "",
				"decision_at": nil,
				"apply_error": "",
				"updated_at":  now,
			}, map[string]interface{}{
				"tenant_id = ?":    artifact.TenantID,
				"tool_call_id = ?": toolCallID,
				"status IN (?)": []string{
					ApprovalStatusPending,
					ApprovalStatusApproved,
					ApprovalStatusApplyFailed,
				},
			}, &Approval{})
			if err != nil {
				return err
			}
			if rows != int64(len(active)) {
				return ErrArtifactApprovalConflict
			}
		}

		if err := tx.Save(&artifact); err != nil {
			return err
		}
		if len(active) > 0 && preparedReplacement != nil {
			return tx.Save(preparedReplacement)
		}
		return nil
	})
}

func prepareArtifactForSave(a *Artifact) (Artifact, error) {
	if a == nil {
		return Artifact{}, fmt.Errorf("agent store: artifact is nil")
	}
	cp := *a
	now := time.Now()
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = now
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = now
	}
	if cp.Status == "" {
		cp.Status = ArtifactStatusNew
	}
	cp.Data = sanitizeJSONText(cp.Data)
	cp.Summary = truncateStoreText(cp.Summary, 4000)
	return cp, nil
}

// SaveApproval persists a pending or decided approval request.
func (p *GormPersistence) SaveApproval(_ context.Context, a *Approval) error {
	if p == nil || p.db == nil {
		return nil
	}
	cp, err := prepareApprovalForSave(a)
	if err != nil {
		return err
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

func prepareApprovalForSave(a *Approval) (Approval, error) {
	if a == nil {
		return Approval{}, fmt.Errorf("agent store: approval is nil")
	}
	cp := *a
	now := time.Now()
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = now
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = now
	}
	if cp.Status == "" {
		cp.Status = ApprovalStatusPending
	}
	cp.ArtifactIDs = sanitizeJSONText(cp.ArtifactIDs)
	return cp, nil
}

// SaveThread persists an agent thread.
func (p *GormPersistence) SaveThread(_ context.Context, t *Thread) error {
	if p == nil || p.db == nil {
		return nil
	}
	if t == nil {
		return fmt.Errorf("agent store: thread is nil")
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.LastActive.IsZero() {
		t.LastActive = now
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		var existing Thread
		err := tx.Read().Where("tenant_id = ? AND id = ?", t.TenantID, t.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Save(t)
		}
		if err != nil {
			return err
		}
		_, err = tx.UpdateColumns(map[string]interface{}{
			"persona":     t.Persona,
			"title":       t.Title,
			"last_active": t.LastActive,
		}, map[string]interface{}{
			"tenant_id = ?": t.TenantID,
			"id = ?":        t.ID,
		}, &Thread{})
		return err
	})
}

// SaveTurn persists an agent turn.
func (p *GormPersistence) SaveTurn(_ context.Context, t *Turn) error {
	if p == nil || p.db == nil {
		return nil
	}
	if t == nil {
		return fmt.Errorf("agent store: turn is nil")
	}
	if t.StartedAt.IsZero() {
		t.StartedAt = time.Now()
	}
	if t.Status == "" {
		if t.Completed {
			t.Status = TurnStatusCompleted
		} else {
			t.Status = TurnStatusRunning
		}
	}
	t.Error = truncateStoreText(sanitizeJSONText(redact.SanitizeEnvBlock(t.Error)), 2000)
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(t)
	})
}

// SaveMessage persists a redacted agent message.
func (p *GormPersistence) SaveMessage(_ context.Context, m *Message) error {
	if p == nil || p.db == nil {
		return nil
	}
	if m == nil {
		return fmt.Errorf("agent store: message is nil")
	}
	cp := sanitizeMessage(*m)
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	if cp.Bytes == 0 {
		cp.Bytes = len(cp.Content)
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

// SaveCompactionCheckpoint advances a thread's durable replay summary.
func (p *GormPersistence) SaveCompactionCheckpoint(_ context.Context, checkpoint CompactionCheckpoint) (bool, error) {
	if p == nil || p.db == nil {
		return true, nil
	}
	if checkpoint.TenantID == "" || checkpoint.ThreadID == "" || checkpoint.ThroughMessageID == "" || checkpoint.ThroughCreatedAt.IsZero() {
		return false, fmt.Errorf("agent store: invalid compaction checkpoint boundary")
	}
	checkpoint.Summary = truncateStoreText(redact.SanitizeEnvBlock(strings.TrimSpace(checkpoint.Summary)), 8000)
	checkpoint.SourceHash = truncateStoreText(strings.TrimSpace(checkpoint.SourceHash), 64)
	if checkpoint.Summary == "" || checkpoint.SourceHash == "" {
		return false, fmt.Errorf("agent store: invalid compaction checkpoint content")
	}

	applied := false
	err := p.db.Update(func(tx pkgdb.Tx) error {
		for attempt := 0; attempt < 3; attempt++ {
			var thread Thread
			err := tx.Read().
				Where("tenant_id = ? AND id = ?", checkpoint.TenantID, checkpoint.ThreadID).
				First(&thread).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrThreadNotFound
			}
			if err != nil {
				return err
			}
			if !compactionBoundaryBefore(thread.CompactionThroughCreatedAt, thread.CompactionThroughMessageID, checkpoint.ThroughCreatedAt, checkpoint.ThroughMessageID) {
				return nil
			}
			rows, err := tx.UpdateColumns(map[string]interface{}{
				"compaction_summary":            checkpoint.Summary,
				"compaction_source_hash":        checkpoint.SourceHash,
				"compaction_through_message_id": checkpoint.ThroughMessageID,
				"compaction_through_created_at": checkpoint.ThroughCreatedAt,
			}, map[string]interface{}{
				"tenant_id = ?":              checkpoint.TenantID,
				"id = ?":                     checkpoint.ThreadID,
				"compaction_source_hash = ?": thread.CompactionSourceHash,
			}, &Thread{})
			if err != nil {
				return err
			}
			if rows > 0 {
				applied = true
				return nil
			}
		}
		return fmt.Errorf("agent store: compaction checkpoint update conflicted")
	})
	return applied, err
}

// FinalizeTurn atomically persists final messages and the terminal turn state.
func (p *GormPersistence) FinalizeTurn(_ context.Context, t *Turn, messages []*Message) error {
	if p == nil || p.db == nil {
		return nil
	}
	if t == nil {
		return fmt.Errorf("agent store: turn is nil")
	}
	if !t.Completed || (t.Status != TurnStatusCompleted && t.Status != TurnStatusFailed) {
		return fmt.Errorf("agent store: turn %s is not terminal", t.ID)
	}
	turn := *t
	if turn.StartedAt.IsZero() {
		turn.StartedAt = time.Now()
	}
	if turn.CompletedAt == nil {
		completedAt := time.Now()
		turn.CompletedAt = &completedAt
	}
	turn.Error = truncateStoreText(sanitizeJSONText(redact.SanitizeEnvBlock(turn.Error)), 2000)

	prepared := make([]Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if message.TenantID != turn.TenantID || message.ThreadID != turn.ThreadID || message.TurnID != turn.ID {
			return fmt.Errorf("agent store: final message scope does not match turn %s", turn.ID)
		}
		cp := sanitizeMessage(*message)
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = time.Now()
		}
		if cp.Bytes == 0 {
			cp.Bytes = len(cp.Content)
		}
		prepared = append(prepared, cp)
	}

	return p.db.Update(func(tx pkgdb.Tx) error {
		for i := range prepared {
			if err := tx.Save(&prepared[i]); err != nil {
				return err
			}
		}
		return tx.Save(&turn)
	})
}

// RecoverStaleTurns marks interrupted running turns as failed.
func (p *GormPersistence) RecoverStaleTurns(_ context.Context, tenantID, threadID string, staleBefore time.Time) (int64, error) {
	if p == nil || p.db == nil || tenantID == "" || staleBefore.IsZero() {
		return 0, nil
	}
	now := time.Now()
	var rows int64
	err := p.db.Update(func(tx pkgdb.Tx) error {
		where := map[string]interface{}{
			"tenant_id = ?":  tenantID,
			"status = ?":     TurnStatusRunning,
			"started_at < ?": staleBefore,
		}
		if threadID != "" {
			where["thread_id = ?"] = threadID
		}
		updated, err := tx.UpdateColumns(map[string]interface{}{
			"status":       TurnStatusFailed,
			"error":        "agent runtime interrupted before completion",
			"completed":    true,
			"completed_at": now,
		}, where, &Turn{})
		rows = updated
		return err
	})
	return rows, err
}

// LoadThread loads a tenant-scoped agent thread.
func (p *GormPersistence) LoadThread(_ context.Context, tenantID, threadID string) (*Thread, error) {
	if p == nil || p.db == nil {
		return nil, ErrThreadNotFound
	}
	var thread Thread
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, threadID).First(&thread).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}
	return &thread, nil
}

// ListThreads returns recent tenant-scoped agent threads.
func (p *GormPersistence) ListThreads(_ context.Context, tenantID string, limit, offset int) ([]*Thread, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var records []Thread
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().
			Where("tenant_id = ?", tenantID).
			Order("last_active DESC").
			Limit(limit).
			Offset(offset).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Thread, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// LoadMessages loads a tenant-scoped thread history, oldest first.
func (p *GormPersistence) LoadMessages(_ context.Context, tenantID, threadID string) ([]*Message, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	var records []Message
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().
			Where("tenant_id = ? AND thread_id = ?", tenantID, threadID).
			Order("created_at ASC, id ASC").
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Message, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// LoadRecentMessages loads replay messages after the durable checkpoint. Before
// the first checkpoint it returns the full history so compaction never silently
// drops an older prefix after a process restart.
func (p *GormPersistence) LoadRecentMessages(_ context.Context, tenantID, threadID string, _ int) ([]*Message, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	var thread Thread
	var records []Message
	err := p.db.View(func(tx pkgdb.Tx) error {
		threadErr := tx.Read().
			Where("tenant_id = ? AND id = ?", tenantID, threadID).
			First(&thread).Error
		if threadErr != nil && !errors.Is(threadErr, gorm.ErrRecordNotFound) {
			return threadErr
		}
		query := tx.Read().
			Where("tenant_id = ? AND thread_id = ?", tenantID, threadID).
			Order("created_at DESC, id DESC")
		if validThreadCompactionCheckpoint(&thread) {
			query = query.Where(
				"created_at > ? OR (created_at = ? AND id > ?)",
				*thread.CompactionThroughCreatedAt,
				*thread.CompactionThroughCreatedAt,
				thread.CompactionThroughMessageID,
			)
		}
		return query.Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	extra := 0
	if validThreadCompactionCheckpoint(&thread) {
		extra = 1
	}
	out := make([]*Message, len(records)+extra)
	if extra == 1 {
		out[0] = threadCompactionMessage(&thread)
	}
	for i := range records {
		out[extra+len(records)-1-i] = &records[i]
	}
	return out, nil
}

func compactionBoundaryBefore(currentAt *time.Time, currentID string, nextAt time.Time, nextID string) bool {
	if currentAt == nil || currentAt.IsZero() || currentID == "" {
		return true
	}
	if currentAt.Before(nextAt) {
		return true
	}
	return currentAt.Equal(nextAt) && currentID < nextID
}

func validThreadCompactionCheckpoint(thread *Thread) bool {
	return thread != nil &&
		strings.TrimSpace(thread.CompactionSummary) != "" &&
		strings.TrimSpace(thread.CompactionSourceHash) != "" &&
		thread.CompactionThroughMessageID != "" &&
		thread.CompactionThroughCreatedAt != nil &&
		!thread.CompactionThroughCreatedAt.IsZero()
}

func threadCompactionMessage(thread *Thread) *Message {
	return &Message{
		ID:         "checkpoint:" + thread.CompactionSourceHash,
		TenantID:   thread.TenantID,
		ThreadID:   thread.ID,
		Role:       "system",
		Content:    thread.CompactionSummary,
		Tokens:     0,
		Bytes:      len(thread.CompactionSummary),
		CreatedAt:  *thread.CompactionThroughCreatedAt,
		Checkpoint: true,
	}
}

// LoadSkillRun loads a single tenant-scoped skill run.
func (p *GormPersistence) LoadSkillRun(_ context.Context, tenantID, runID string) (*SkillRun, error) {
	if p == nil || p.db == nil {
		return nil, ErrSkillRunNotFound
	}
	var run SkillRun
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, runID).First(&run).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillRunNotFound
		}
		return nil, err
	}
	return &run, nil
}

// ListSkillRuns returns recent tenant-scoped skill runs.
func (p *GormPersistence) ListSkillRuns(_ context.Context, tenantID, skillID, status string, limit, offset int) ([]*SkillRun, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var records []SkillRun
	err := p.db.View(func(tx pkgdb.Tx) error {
		query := tx.Read().Where("tenant_id = ?", tenantID)
		if skillID != "" {
			query = query.Where("skill_id = ?", skillID)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}
		return query.
			Order("started_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*SkillRun, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// LoadArtifact loads a single tenant-scoped artifact.
func (p *GormPersistence) LoadArtifact(_ context.Context, tenantID, artifactID string) (*Artifact, error) {
	if p == nil || p.db == nil {
		return nil, ErrArtifactNotFound
	}
	var artifact Artifact
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, artifactID).First(&artifact).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, err
	}
	return &artifact, nil
}

// LoadArtifactContent loads private binary content using explicit tenant and
// artifact predicates.
func (p *GormPersistence) LoadArtifactContent(_ context.Context, tenantID, artifactID string) (*ArtifactContent, error) {
	if p == nil || p.db == nil {
		return nil, ErrArtifactContentNotFound
	}
	var content ArtifactContent
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("tenant_id = ? AND artifact_id = ?", tenantID, artifactID).First(&content).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArtifactContentNotFound
		}
		return nil, err
	}
	return &content, nil
}

// ListArtifacts returns tenant-scoped skill artifacts.
func (p *GormPersistence) ListArtifacts(_ context.Context, tenantID, skillRunID, kind, status string, limit, offset int) ([]*Artifact, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	var records []Artifact
	err := p.db.View(func(tx pkgdb.Tx) error {
		query := tx.Read().Where("tenant_id = ?", tenantID)
		if skillRunID != "" {
			query = query.Where("skill_run_id = ?", skillRunID)
		}
		if kind != "" {
			query = query.Where("kind = ?", kind)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}
		return query.
			Order("created_at ASC, id ASC").
			Limit(limit).
			Offset(offset).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Artifact, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// LoadApproval loads a single tenant-scoped approval request.
func (p *GormPersistence) LoadApproval(_ context.Context, tenantID, approvalID string) (*Approval, error) {
	if p == nil || p.db == nil {
		return nil, ErrApprovalNotFound
	}
	var approval Approval
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, approvalID).First(&approval).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// ListApprovals returns recent tenant-scoped approval requests.
func (p *GormPersistence) ListApprovals(_ context.Context, tenantID, status string, limit, offset int) ([]*Approval, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var records []Approval
	err := p.db.View(func(tx pkgdb.Tx) error {
		query := tx.Read().Where("tenant_id = ?", tenantID)
		if status != "" {
			query = query.Where("status = ?", status)
		}
		return query.
			Order("created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Approval, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// UpdateApprovalStatus records a human approval decision.
func (p *GormPersistence) UpdateApprovalStatus(_ context.Context, tenantID, approvalID, status, actorID string) (*Approval, error) {
	if p == nil || p.db == nil {
		return nil, ErrApprovalNotFound
	}
	if status != ApprovalStatusApproved && status != ApprovalStatusRejected {
		return nil, fmt.Errorf("agent store: invalid approval status %q", status)
	}
	var approval Approval
	now := time.Now()
	err := p.db.Update(func(tx pkgdb.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{
				"status":      status,
				"decision_by": actorID,
				"decision_at": now,
				"updated_at":  now,
			},
			map[string]interface{}{
				"id = ?":     approvalID,
				"status = ?": ApprovalStatusPending,
			},
			&Approval{},
		)
		if err != nil {
			return err
		}
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, approvalID).First(&approval).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// ClaimApprovalForApply atomically moves an approved approval into applying.
func (p *GormPersistence) ClaimApprovalForApply(_ context.Context, tenantID, approvalID, actorID string) (*Approval, error) {
	if p == nil || p.db == nil {
		return nil, ErrApprovalNotFound
	}
	var approval Approval
	now := time.Now()
	err := p.db.Update(func(tx pkgdb.Tx) error {
		rows, err := tx.UpdateColumns(
			map[string]interface{}{
				"status":      ApprovalStatusApplying,
				"applied_by":  actorID,
				"apply_error": "",
				"updated_at":  now,
			},
			map[string]interface{}{
				"id = ?":        approvalID,
				"status IN (?)": []string{ApprovalStatusApproved, ApprovalStatusApplyFailed},
			},
			&Approval{},
		)
		if err != nil {
			return err
		}
		if err := tx.Read().Where("tenant_id = ? AND id = ?", tenantID, approvalID).First(&approval).Error; err != nil {
			return err
		}
		if rows == 0 {
			return ErrApprovalClaimConflict
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// MarkApprovalApplied records a successful apply result.
func (p *GormPersistence) MarkApprovalApplied(_ context.Context, tenantID, approvalID, result, actorID string) (*Approval, error) {
	if p == nil || p.db == nil {
		return nil, ErrApprovalNotFound
	}
	var approval Approval
	now := time.Now()
	err := p.db.Update(func(tx pkgdb.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{
				"status":       ApprovalStatusApplied,
				"applied_by":   actorID,
				"applied_at":   now,
				"apply_result": sanitizeJSONText(result),
				"apply_error":  "",
				"updated_at":   now,
			},
			map[string]interface{}{
				"id = ?":     approvalID,
				"status = ?": ApprovalStatusApplying,
			},
			&Approval{},
		)
		if err != nil {
			return err
		}
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, approvalID).First(&approval).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// MarkApprovalApplyFailed records a failed apply attempt for later review.
func (p *GormPersistence) MarkApprovalApplyFailed(_ context.Context, tenantID, approvalID, applyErr, actorID string) (*Approval, error) {
	if p == nil || p.db == nil {
		return nil, ErrApprovalNotFound
	}
	var approval Approval
	now := time.Now()
	err := p.db.Update(func(tx pkgdb.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{
				"status":      ApprovalStatusApplyFailed,
				"applied_by":  actorID,
				"apply_error": truncateStoreText(applyErr, 2000),
				"updated_at":  now,
			},
			map[string]interface{}{
				"id = ?":     approvalID,
				"status = ?": ApprovalStatusApplying,
			},
			&Approval{},
		)
		if err != nil {
			return err
		}
		return tx.Read().Where("tenant_id = ? AND id = ?", tenantID, approvalID).First(&approval).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// DeleteThread removes a thread and all its turn/message rows.
func (p *GormPersistence) DeleteThread(_ context.Context, tenantID, threadID string) error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		where := map[string]interface{}{"tenant_id": tenantID}
		if err := tx.Delete("thread_id", threadID, where, &Message{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, where, &Turn{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, where, &SkillRun{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, where, &ArtifactContent{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, where, &Artifact{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, where, &Approval{}); err != nil {
			return err
		}
		return tx.Delete("id", threadID, where, &Thread{})
	})
}

func sanitizeMessage(m Message) Message {
	m.Content = sanitizeJSONText(m.Content)
	m.AttachmentDisplay = sanitizeAttachmentDisplay(m.AttachmentDisplay)
	m.Deliveries = sanitizeDeliveries(m.Deliveries)
	m.ToolCalls = sanitizeToolCalls(m.ToolCalls)
	return m
}

type messageAttachmentDisplay struct {
	ArtifactID  string `json:"artifactId,omitempty"`
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
	PreviewURL  string `json:"previewUrl,omitempty"`
}

func sanitizeAttachmentDisplay(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var items []messageAttachmentDisplay
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return ""
	}
	out := make([]messageAttachmentDisplay, 0, min(len(items), 10))
	for _, item := range items {
		item.ArtifactID = truncateStoreText(redact.SanitizeEnvBlock(strings.TrimSpace(item.ArtifactID)), 128)
		item.Name = truncateStoreText(redact.SanitizeEnvBlock(strings.TrimSpace(item.Name)), 256)
		item.ContentType = truncateStoreText(redact.SanitizeEnvBlock(strings.TrimSpace(item.ContentType)), 128)
		item.PreviewURL = sanitizeAttachmentPreviewURL(item.PreviewURL)
		if item.Name == "" && item.ArtifactID == "" {
			continue
		}
		out = append(out, item)
		if len(out) >= 10 {
			break
		}
	}
	if len(out) == 0 {
		return ""
	}
	data, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(data)
}

func sanitizeAttachmentPreviewURL(raw string) string {
	raw = truncateStoreText(redact.SanitizeEnvBlock(strings.TrimSpace(raw)), 2048)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return raw
	default:
		return ""
	}
}

func sanitizeJSONText(content string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		return redact.RedactMapJSON(obj)
	}

	var arr []map[string]any
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		for i := range arr {
			arr[i] = redact.RedactMap(arr[i])
		}
		if data, err := json.Marshal(arr); err == nil {
			return string(data)
		}
	}
	return content
}

func sanitizeToolCalls(raw string) string {
	if raw == "" {
		return ""
	}
	var calls []map[string]any
	if err := json.Unmarshal([]byte(raw), &calls); err != nil {
		return raw
	}
	for _, call := range calls {
		if args, ok := call["arguments"].(string); ok {
			call["arguments"] = sanitizeJSONText(args)
		}
	}
	data, err := json.Marshal(calls)
	if err != nil {
		return raw
	}
	return string(data)
}

func sanitizeDeliveries(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var deliveries []any
	if err := json.Unmarshal([]byte(raw), &deliveries); err != nil {
		return ""
	}
	for i := range deliveries {
		deliveries[i] = redact.RedactValue(deliveries[i])
	}
	data, err := json.Marshal(deliveries)
	if err != nil {
		return ""
	}
	return string(data)
}

func truncateStoreText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "...(truncated)"
}

// SanitizeApprovalForAPI returns a copy safe for external API responses.
// Execution paths must use the persisted Approval directly so hash checks and
// tool replay keep the original payload.
func SanitizeApprovalForAPI(a *Approval) *Approval {
	if a == nil {
		return nil
	}
	cp := *a
	if cp.Payload != "" {
		cp.Payload = sanitizeJSONText(cp.Payload)
	}
	if cp.ApplyResult != "" {
		cp.ApplyResult = sanitizeJSONText(cp.ApplyResult)
	}
	return &cp
}

// SanitizeApprovalsForAPI sanitizes a list of approvals for API responses.
func SanitizeApprovalsForAPI(items []*Approval) []*Approval {
	if len(items) == 0 {
		return items
	}
	out := make([]*Approval, 0, len(items))
	for _, item := range items {
		out = append(out, SanitizeApprovalForAPI(item))
	}
	return out
}

func memoryFromKernel(scope kernel.Scope, item kernel.MemoryItem) (*Memory, error) {
	if scope.TenantID == "" {
		return nil, fmt.Errorf("agent store: memory tenant scope is required")
	}
	if item.ID == "" {
		return nil, fmt.Errorf("agent store: memory id is required")
	}
	if item.Scope == "" {
		return nil, fmt.Errorf("agent store: memory scope is required")
	}
	if item.Scope == kernel.MemoryUser && scope.ActorID == "" {
		return nil, fmt.Errorf("agent store: user memory actor scope is required")
	}
	if item.Scope == kernel.MemoryStoreScope && scope.StoreID == "" {
		return nil, fmt.Errorf("agent store: store memory store scope is required")
	}
	if item.Scope == kernel.MemoryThread && scope.ThreadID == "" {
		return nil, fmt.Errorf("agent store: thread memory thread scope is required")
	}
	if item.Scope == kernel.MemorySkill && scope.SkillID == "" {
		return nil, fmt.Errorf("agent store: skill memory skill scope is required")
	}
	if item.Content == "" {
		return nil, fmt.Errorf("agent store: memory content is required")
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	metadata := ""
	if len(item.Metadata) > 0 {
		data, err := json.Marshal(item.Metadata)
		if err != nil {
			return nil, fmt.Errorf("agent store: marshal memory metadata: %w", err)
		}
		metadata = sanitizeJSONText(string(data))
	}
	return &Memory{
		ID:        item.ID,
		TenantID:  scope.TenantID,
		Scope:     string(item.Scope),
		Subject:   item.Subject,
		StoreID:   memoryStoreID(scope, item.Scope),
		ThreadID:  memoryThreadID(scope, item.Scope),
		SkillID:   memorySkillID(scope, item.Scope),
		ActorID:   memoryActorID(scope, item.Scope),
		Status:    MemoryStatusActive,
		Content:   truncateStoreText(redact.SanitizeEnvBlock(item.Content), 4000),
		Metadata:  metadata,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

func memoryUpdateColumns(update MemoryUpdate) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	if update.Subject != nil {
		values["subject"] = truncateStoreText(strings.TrimSpace(*update.Subject), 128)
	}
	if update.Content != nil {
		content := strings.TrimSpace(*update.Content)
		if content == "" {
			return nil, fmt.Errorf("agent store: memory content is required")
		}
		values["content"] = truncateStoreText(redact.SanitizeEnvBlock(content), 4000)
	}
	if update.Metadata != nil {
		metadata := ""
		if len(*update.Metadata) > 0 {
			data, err := json.Marshal(*update.Metadata)
			if err != nil {
				return nil, fmt.Errorf("agent store: marshal memory metadata: %w", err)
			}
			metadata = sanitizeJSONText(string(data))
		}
		values["metadata"] = metadata
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("agent store: memory update is empty")
	}
	return values, nil
}

func applyMemoryUpdateColumns(memory *Memory, values map[string]interface{}) {
	if memory == nil {
		return
	}
	if value, ok := values["subject"].(string); ok {
		memory.Subject = value
	}
	if value, ok := values["content"].(string); ok {
		memory.Content = value
	}
	if value, ok := values["metadata"].(string); ok {
		memory.Metadata = value
	}
}

func memoryStoreID(scope kernel.Scope, memoryScope kernel.MemoryScope) string {
	if memoryScope == kernel.MemoryStoreScope {
		return scope.StoreID
	}
	return ""
}

func memoryActorID(scope kernel.Scope, memoryScope kernel.MemoryScope) string {
	if memoryScope == kernel.MemoryUser {
		return scope.ActorID
	}
	return ""
}

func memoryThreadID(scope kernel.Scope, memoryScope kernel.MemoryScope) string {
	if memoryScope == kernel.MemoryThread {
		return scope.ThreadID
	}
	return ""
}

func memorySkillID(scope kernel.Scope, memoryScope kernel.MemoryScope) string {
	if memoryScope == kernel.MemorySkill {
		return scope.SkillID
	}
	return ""
}

func (p *GormPersistence) touchRetrievedMemories(records []Memory) error {
	if len(records) == 0 {
		return nil
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		if record.ID != "" {
			ids = append(ids, record.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	return p.db.Update(func(tx pkgdb.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{
				"last_used_at": now,
			},
			map[string]interface{}{
				"id IN (?)":  ids,
				"status = ?": MemoryStatusActive,
			},
			&Memory{},
		)
		return err
	})
}

func visibleMemoryQuery(query *gorm.DB, scope kernel.Scope) *gorm.DB {
	clauses := []string{"scope = ?"}
	args := []interface{}{string(kernel.MemoryTenant)}
	if scope.ActorID != "" {
		clauses = append(clauses, "(scope = ? AND actor_id = ?)")
		args = append(args, string(kernel.MemoryUser), scope.ActorID)
	}
	if scope.StoreID != "" {
		clauses = append(clauses, "(scope = ? AND store_id = ?)")
		args = append(args, string(kernel.MemoryStoreScope), scope.StoreID)
	}
	if scope.ThreadID != "" {
		clauses = append(clauses, "(scope = ? AND thread_id = ?)")
		args = append(args, string(kernel.MemoryThread), scope.ThreadID)
	}
	if scope.SkillID != "" {
		clauses = append(clauses, "(scope = ? AND skill_id = ?)")
		args = append(args, string(kernel.MemorySkill), scope.SkillID)
	}
	return query.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func (m Memory) toKernel() kernel.MemoryItem {
	metadata := map[string]string{}
	if m.Metadata != "" {
		_ = json.Unmarshal([]byte(m.Metadata), &metadata)
	}
	return kernel.MemoryItem{
		ID:        m.ID,
		Scope:     kernel.MemoryScope(m.Scope),
		Subject:   m.Subject,
		Content:   m.Content,
		Metadata:  metadata,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// RuntimeStore is an in-memory cache for active thread state and messages.
// Provides fast reads during a turn without hitting the database.
// Keys are composite (tenantID, threadID) to prevent cross-tenant leakage.
type RuntimeStore struct {
	mu       sync.RWMutex
	threads  map[string]*Thread    // key: threadKey(tenantID, threadID)
	messages map[string][]*Message // key: threadKey(tenantID, threadID)
}

// threadKey builds a composite map key that prevents cross-tenant collisions.
func threadKey(tenantID, threadID string) string {
	return tenantID + "\x00" + threadID
}

// NewRuntimeStore creates an empty in-memory store.
func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{
		threads:  make(map[string]*Thread),
		messages: make(map[string][]*Message),
	}
}

// GetThread returns a defensive copy of the thread to prevent callers
// from mutating internal state without holding the lock.
func (r *RuntimeStore) GetThread(tenantID, threadID string) (*Thread, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.threads[threadKey(tenantID, threadID)]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

func (r *RuntimeStore) UpdateThread(t *Thread) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.threads[threadKey(t.TenantID, t.ID)] = t
}

// TouchThread atomically updates the thread's LastActive timestamp.
func (r *RuntimeStore) TouchThread(tenantID, threadID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.threads[threadKey(tenantID, threadID)]; ok {
		t.LastActive = time.Now()
	}
}

func (r *RuntimeStore) RemoveThread(tenantID, threadID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	delete(r.threads, k)
	delete(r.messages, k)
}

// CleanupIdle removes threads that have been inactive for longer than maxIdle.
func (r *RuntimeStore) CleanupIdle(maxIdle time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-maxIdle)
	removed := 0
	for id, t := range r.threads {
		if t.LastActive.Before(cutoff) {
			delete(r.threads, id)
			delete(r.messages, id)
			removed++
		}
	}
	return removed
}

func (r *RuntimeStore) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.threads)
}

// AppendMessage adds a message to the thread's in-memory history.
func (r *RuntimeStore) AppendMessage(tenantID, threadID string, m *Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	r.messages[k] = append(r.messages[k], m)
}

// GetMessages returns all in-memory messages for a thread, oldest first.
// Returns deep copies to prevent callers from mutating internal state.
func (r *RuntimeStore) GetMessages(tenantID, threadID string) []*Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msgs := r.messages[threadKey(tenantID, threadID)]
	out := make([]*Message, len(msgs))
	for i, m := range msgs {
		cp := *m
		out[i] = &cp
	}
	return out
}

// ReplaceMessages atomically replaces a thread's in-memory replay history.
func (r *RuntimeStore) ReplaceMessages(tenantID, threadID string, messages []*Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		cp := *message
		out = append(out, &cp)
	}
	r.messages[threadKey(tenantID, threadID)] = out
}

// TruncateMessages keeps only the last n messages for a thread (budget shaping).
func (r *RuntimeStore) TruncateMessages(tenantID, threadID string, keepLast int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	msgs := r.messages[k]
	if len(msgs) > keepLast {
		r.messages[k] = msgs[len(msgs)-keepLast:]
	}
}
