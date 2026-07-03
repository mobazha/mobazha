package database

import (
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

// MigrateFulfillmentModels creates/updates supply chain tables and their
// composite indexes that cannot be expressed via GORM struct tags
// (TenantMixin embeds tenant_id as part of the composite PK, but extra
// unique indexes require explicit SQL).
func MigrateFulfillmentModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		allModels := []interface{}{
			&models.FulfillmentProviderConfig{},
			&models.FulfillmentLocation{},
			&models.SyncedProductMapping{},
			&models.FulfillmentOrderMapping{},
			&models.ProcessedFulfillmentEvent{},
			&models.SupplyChainAlert{},
			&models.AutoActionRule{},
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

			// FulfillmentLocation: one external_key per (tenant, provider)
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_fl_tenant_provider_key
				ON fulfillment_locations (tenant_id, provider_id, external_key)`,

			// SyncedProductMapping: one listing per tenant
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_spm_tenant_slug
				ON synced_product_mappings (tenant_id, listing_slug)`,
			`CREATE INDEX IF NOT EXISTS idx_spm_tenant_provider
				ON synced_product_mappings (tenant_id, provider_id)`,

			// FulfillmentOrderMapping: one group per (tenant, order, group_key).
			// Drop legacy 1:1 index if it still exists (pre-FF-3).
			`DROP INDEX IF EXISTS idx_fom_tenant_order`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_fom_tenant_order_group
				ON fulfillment_order_mappings (tenant_id, mobazha_order_id, fulfillment_group_key)`,

			// ProcessedFulfillmentEvent: dedup key per (tenant, provider, event)
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_pfe_tenant_provider_event
				ON processed_fulfillment_events (tenant_id, provider_id, event_id)`,
			`CREATE INDEX IF NOT EXISTS idx_pfe_tenant_order
				ON processed_fulfillment_events (tenant_id, order_id)`,

			// SupplyChainAlert: query by tenant + active (non-dismissed)
			`CREATE INDEX IF NOT EXISTS idx_sca_tenant_active
				ON supply_chain_alerts (tenant_id, dismissed, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_sca_tenant_listing
				ON supply_chain_alerts (tenant_id, listing_slug)`,

			// AutoActionRule: one rule per (tenant, provider, trigger, action).
			// Drop old narrow index if it exists (pre provider-specific support).
			`DROP INDEX IF EXISTS idx_aar_tenant_trigger_action`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_aar_tenant_provider_trigger_action
				ON auto_action_rules (tenant_id, provider_id, trigger, action)`,
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
