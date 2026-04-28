package database

import (
	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// MigrateFulfillmentModels creates/updates supply chain tables and their
// composite indexes that cannot be expressed via GORM struct tags
// (TenantMixin embeds tenant_id as part of the composite PK, but extra
// unique indexes require explicit SQL).
func MigrateFulfillmentModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		allModels := []interface{}{
			&models.FulfillmentProviderConfig{},
			&models.SyncedProductMapping{},
			&models.FulfillmentOrderMapping{},
			&models.ProcessedFulfillmentEvent{},
		}
		for _, m := range allModels {
			if err := tx.Migrate(m); err != nil {
				return err
			}
		}

		migrationSQL := []string{
			// FulfillmentProviderConfig: one provider per tenant
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_fpc_tenant_provider
				ON fulfillment_provider_configs (tenant_id, provider_id)`,

			// SyncedProductMapping: one listing per tenant
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_spm_tenant_slug
				ON synced_product_mappings (tenant_id, listing_slug)`,
			`CREATE INDEX IF NOT EXISTS idx_spm_tenant_provider
				ON synced_product_mappings (tenant_id, provider_id)`,

			// FulfillmentOrderMapping: one mapping per (tenant, mobazha_order_id)
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_fom_tenant_order
				ON fulfillment_order_mappings (tenant_id, mobazha_order_id)`,

			// ProcessedFulfillmentEvent: dedup key per (tenant, provider, event)
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_pfe_tenant_provider_event
				ON processed_fulfillment_events (tenant_id, provider_id, event_id)`,
			`CREATE INDEX IF NOT EXISTS idx_pfe_tenant_order
				ON processed_fulfillment_events (tenant_id, order_id)`,
		}

		gormDB := tx.Read()
		for _, sql := range migrationSQL {
			if err := gormDB.Exec(sql).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
