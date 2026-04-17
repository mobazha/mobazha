package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// FeatureAuditLogStore implements contracts.FeatureAuditLogger backed
// by the `feature_flag_audit_logs` GORM table.
//
// Rows are append-only; no Update / Delete APIs are exposed. The
// tenant scope is provided by the wrapped database: on standalone
// nodes the db is scoped to StandaloneTenantID; on hosting SaaS the
// caller is expected to pass a tenant-scoped database. Because
// FeatureFlagAuditLog does not embed TenantMixin, caller must set
// TenantID on the entry explicitly for tenant-scope entries (matches
// the shared schema; see pkg/models/feature_flag_audit_log.go).
//
// See docs/FEATURE_FLAG_ARCHITECTURE.md §5.3.
type FeatureAuditLogStore struct {
	db database.Database
}

// NewFeatureAuditLogStore returns a FeatureAuditLogStore backed by the
// provided database. Callers are responsible for ensuring the database
// has the appropriate tenant scope for tenant-scope audit entries.
func NewFeatureAuditLogStore(db database.Database) *FeatureAuditLogStore {
	return &FeatureAuditLogStore{db: db}
}

var _ contracts.FeatureAuditLogger = (*FeatureAuditLogStore)(nil)

// AppendAudit persists a single audit entry. `entry` must be non-nil
// and carry Scope + FeatureKey + Actor; CreatedAt is filled in by GORM
// via the autoCreateTime / autoUpdateTime tags if left zero.
//
// Writes are best-effort for the caller's perspective: a non-nil error
// must be surfaced so operators can alert, but audit failures should
// never block the underlying feature-flag mutation. Callers are
// expected to log-and-continue, not return the error to the HTTP
// response (see feature_handlers.go for the pattern).
func (s *FeatureAuditLogStore) AppendAudit(ctx context.Context, entry *models.FeatureFlagAuditLog) error {
	if s == nil || s.db == nil {
		return errors.New("feature_audit: nil store")
	}
	if entry == nil {
		return errors.New("feature_audit: nil entry")
	}
	if entry.Scope == "" || entry.FeatureKey == "" {
		return errors.New("feature_audit: scope and feature_key are required")
	}
	return s.db.Update(func(tx database.Tx) error {
		// Bypass the multi-tenant Save() wrapper: FeatureFlagAuditLog
		// does not embed TenantMixin; its TenantID column is a plain
		// field set explicitly by the caller (may be empty for
		// platform_global scope entries).
		if err := tx.Read().Create(entry).Error; err != nil {
			return fmt.Errorf("feature_audit.AppendAudit: %w", err)
		}
		return nil
	})
}

// List returns the most recent audit entries for the given scope and
// tenant (tenantID may be empty for platform_global). Results are
// ordered by CreatedAt DESC, newest first, capped at `limit` rows
// (<=0 → default 100, >1000 → 1000).
func (s *FeatureAuditLogStore) List(ctx context.Context, scope, tenantID string, limit int) ([]models.FeatureFlagAuditLog, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []models.FeatureFlagAuditLog
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Model(&models.FeatureFlagAuditLog{})
		if scope != "" {
			q = q.Where("scope = ?", scope)
		}
		if tenantID != "" {
			q = q.Where("tenant_id = ?", tenantID)
		}
		return q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("feature_audit.List: %w", err)
	}
	return rows, nil
}

// Migrate ensures the feature_flag_audit_logs table exists. Intended
// to be called from MobazhaNode.initFeatureResolver during startup.
func (s *FeatureAuditLogStore) Migrate() error {
	if s == nil || s.db == nil {
		return errors.New("feature_audit: nil store")
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.FeatureFlagAuditLog{})
	})
}
