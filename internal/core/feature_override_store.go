package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FeatureOverrideStore implements config.TenantFeatureStore backed by
// the `feature_overrides` GORM table.
//
// The tenant scope is provided by the wrapped database: on standalone
// nodes the db is scoped to StandaloneTenantID; on hosting SaaS the
// caller is expected to pass a tenant-scoped database (TenantDB). The
// `tenantID` argument in interface methods is therefore informational
// — it must match the db scope, which callers guarantee via request
// routing. See docs/FEATURE_FLAG_ARCHITECTURE.md §5.2.
type FeatureOverrideStore struct {
	db database.Database
}

// NewFeatureOverrideStore returns a config.TenantFeatureStore backed
// by the provided database. Callers are responsible for ensuring the
// database is scoped to the tenant whose overrides should be served.
func NewFeatureOverrideStore(db database.Database) *FeatureOverrideStore {
	return &FeatureOverrideStore{db: db}
}

var _ config.TenantFeatureStore = (*FeatureOverrideStore)(nil)

// Get returns the tenant-layer override for the given feature. When
// no row exists, configured=false and the resolver falls back to
// feature.DefaultValue.
func (s *FeatureOverrideStore) Get(ctx context.Context, tenantID, key string) (bool, bool, error) {
	if s == nil || s.db == nil {
		return false, false, nil
	}
	var ov models.FeatureOverride
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("tenant_id = ? AND feature_key = ?", tenantID, key).
			First(&ov).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("feature_override.Get %q: %w", key, err)
	}
	return ov.Enabled, true, nil
}

// Set upserts the per-tenant override. `actor` is recorded on the
// row for lightweight auditing; the full audit log is written by the
// resolver caller (see Phase FF-4 audit work).
func (s *FeatureOverrideStore) Set(ctx context.Context, tenantID, key string, value bool, actor string) error {
	if s == nil || s.db == nil {
		return errors.New("feature_override: nil store")
	}
	if key == "" {
		return errors.New("feature_override: empty key")
	}
	return s.db.Update(func(tx database.Tx) error {
		// Composite PK (tenant_id, feature_key) with both string columns
		// — use an explicit OnConflict upsert. GORM's Save() can flip to
		// INSERT when any PK field is its zero value (empty string), and
		// UPDATE detection across composite PKs is unreliable, so we
		// always INSERT with DO UPDATE on conflict. tenant_id is written
		// via the TenantID argument to keep the store usable regardless
		// of whether the caller supplied a tenant-scoped Tx wrapper.
		ov := models.FeatureOverride{
			TenantMixin: models.TenantMixin{TenantID: tenantID},
			FeatureKey:  key,
			Enabled:     value,
			UpdatedBy:   actor,
		}
		return tx.Read().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "feature_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_by", "updated_at"}),
		}).Create(&ov).Error
	})
}

// List returns all per-tenant overrides as key → enabled map.
func (s *FeatureOverrideStore) List(ctx context.Context, tenantID string) (map[string]bool, error) {
	if s == nil || s.db == nil {
		return map[string]bool{}, nil
	}
	var rows []models.FeatureOverride
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("tenant_id = ?", tenantID).Find(&rows).Error
	}); err != nil {
		return nil, fmt.Errorf("feature_override.List: %w", err)
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.FeatureKey] = r.Enabled
	}
	return out, nil
}

// Migrate ensures the feature_overrides table exists. Intended to be
// called from MobazhaNode.initFeatureResolver during startup.
func (s *FeatureOverrideStore) Migrate() error {
	if s == nil || s.db == nil {
		return errors.New("feature_override: nil store")
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.FeatureOverride{})
	})
}
