package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	"gorm.io/gorm"
)

// Common errors.
var (
	ErrThreadNotFound        = errors.New("agent: thread not found")
	ErrApprovalNotFound      = errors.New("agent: approval not found")
	ErrApprovalClaimConflict = errors.New("agent: approval apply claim conflict")
	ErrSkillRunNotFound      = errors.New("agent: skill run not found")
	ErrArtifactNotFound      = errors.New("agent: artifact not found")
	ErrMemoryNotFound        = errors.New("agent: memory not found")
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

	// MemoryStatusActive means a memory is available for retrieval.
	MemoryStatusActive = "active"
	// MemoryStatusArchived means a memory is retained but no longer retrieved.
	MemoryStatusArchived = "archived"
)

// Persistence provides durable storage for agent threads, turns, and messages.
type Persistence interface {
	SaveThread(ctx context.Context, t *Thread) error
	SaveTurn(ctx context.Context, t *Turn) error
	SaveMessage(ctx context.Context, m *Message) error
	SaveSkillRun(ctx context.Context, r *SkillRun) error
	SaveArtifact(ctx context.Context, a *Artifact) error
	SaveApproval(ctx context.Context, a *Approval) error
	LoadThread(ctx context.Context, tenantID, threadID string) (*Thread, error)
	ListThreads(ctx context.Context, tenantID string, limit, offset int) ([]*Thread, error)
	LoadMessages(ctx context.Context, tenantID, threadID string) ([]*Message, error)
	LoadSkillRun(ctx context.Context, tenantID, runID string) (*SkillRun, error)
	ListSkillRuns(ctx context.Context, tenantID, skillID, status string, limit, offset int) ([]*SkillRun, error)
	LoadArtifact(ctx context.Context, tenantID, artifactID string) (*Artifact, error)
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

// NewGormPersistence creates a durable agent runtime persistence adapter.
func NewGormPersistence(db pkgdb.Database) *GormPersistence {
	return &GormPersistence{db: db}
}

// MigrateModels creates or updates the agent runtime tables.
func MigrateModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		for _, model := range []interface{}{&Thread{}, &Turn{}, &Message{}, &SkillRun{}, &Artifact{}, &Approval{}, &Memory{}} {
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
	if a == nil {
		return fmt.Errorf("agent store: artifact is nil")
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
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

// SaveApproval persists a pending or decided approval request.
func (p *GormPersistence) SaveApproval(_ context.Context, a *Approval) error {
	if p == nil || p.db == nil {
		return nil
	}
	if a == nil {
		return fmt.Errorf("agent store: approval is nil")
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
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
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
		return tx.Save(t)
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
	m.ToolCalls = sanitizeToolCalls(m.ToolCalls)
	return m
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
		ActorID:   memoryActorID(scope, item.Scope),
		Status:    MemoryStatusActive,
		Content:   truncateStoreText(redact.SanitizeEnvBlock(item.Content), 4000),
		Metadata:  metadata,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
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
