//go:build !private_distribution

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestFindMinVariantCost(t *testing.T) {
	svc := &SupplyChainAppService{}

	tests := []struct {
		name     string
		variants []contracts.StoreSyncVariant
		expected float64
	}{
		{
			name:     "empty variants",
			variants: nil,
			expected: 0,
		},
		{
			name: "single variant",
			variants: []contracts.StoreSyncVariant{
				{RetailPrice: "29.99"},
			},
			expected: 2999,
		},
		{
			name: "multiple variants",
			variants: []contracts.StoreSyncVariant{
				{RetailPrice: "29.99"},
				{RetailPrice: "19.50"},
				{RetailPrice: "49.00"},
			},
			expected: 1950,
		},
		{
			name: "invalid price skipped",
			variants: []contracts.StoreSyncVariant{
				{RetailPrice: "invalid"},
				{RetailPrice: "25.00"},
			},
			expected: 2500,
		},
		{
			name: "zero price skipped",
			variants: []contracts.StoreSyncVariant{
				{RetailPrice: "0"},
				{RetailPrice: "15.00"},
			},
			expected: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.findMinVariantCost(tt.variants)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestAutoActionRuleTypes(t *testing.T) {
	assert.Equal(t, models.RuleTrigger("stock_out"), models.RuleTriggerStockOut)
	assert.Equal(t, models.RuleTrigger("stock_back"), models.RuleTriggerStockBack)
	assert.Equal(t, models.RuleTrigger("price_drift"), models.RuleTriggerPriceDrift)
	assert.Equal(t, models.RuleTrigger("product_cost_changed"), models.RuleTriggerProductCostChanged)
	assert.Equal(t, models.RuleTrigger("product_discontinued"), models.RuleTriggerProductDiscontinued)

	assert.Equal(t, models.RuleAction("hide_listing"), models.RuleActionHideListing)
	assert.Equal(t, models.RuleAction("show_listing"), models.RuleActionShowListing)
	assert.Equal(t, models.RuleAction("pause_listing"), models.RuleActionPauseListing)
	assert.Equal(t, models.RuleAction("notify_only"), models.RuleActionNotifyOnly)
	assert.Equal(t, models.RuleAction("auto_delist"), models.RuleActionAutoDelist)
}

func TestAlertTypes(t *testing.T) {
	assert.Equal(t, models.AlertType("stock_out"), models.AlertTypeStockOut)
	assert.Equal(t, models.AlertType("stock_back"), models.AlertTypeStockBack)
	assert.Equal(t, models.AlertType("price_drift"), models.AlertTypePriceDrift)
	assert.Equal(t, models.AlertType("rule_action"), models.AlertTypeRuleAction)
	assert.Equal(t, models.AlertType("product_changed"), models.AlertTypeProductChanged)
	assert.Equal(t, models.AlertType("product_discontinued"), models.AlertTypeProductDiscontinued)
}

func TestDetectProductChanges_CostIncrease(t *testing.T) {
	svc := &SupplyChainAppService{}
	mapping := models.SyncedProductMapping{
		SupplierCost: "2000", // 2000 cents = $20.00
	}
	product := &contracts.StoreSyncProduct{
		Variants: []contracts.StoreSyncVariant{
			{RetailPrice: "25.00"}, // $25.00 = 2500 cents
		},
	}

	changes := svc.detectProductChanges(mapping, product)
	assert.Len(t, changes, 1)
	assert.Equal(t, models.RuleTriggerProductCostChanged, changes[0].trigger)
	assert.InDelta(t, 2000, changes[0].storedCost, 0.01)
	assert.InDelta(t, 2500, changes[0].newCost, 0.01)
	assert.InDelta(t, 25.0, changes[0].driftPct, 0.01)
}

func TestDetectProductChanges_CostDecrease(t *testing.T) {
	svc := &SupplyChainAppService{}
	mapping := models.SyncedProductMapping{
		SupplierCost: "3000", // $30.00
	}
	product := &contracts.StoreSyncProduct{
		Variants: []contracts.StoreSyncVariant{
			{RetailPrice: "20.00"}, // $20.00 = 2000 cents
		},
	}

	changes := svc.detectProductChanges(mapping, product)
	assert.Len(t, changes, 1)
	assert.Equal(t, models.RuleTriggerProductCostChanged, changes[0].trigger)
	assert.InDelta(t, 33.33, changes[0].driftPct, 0.1)
}

func TestDetectProductChanges_NoCostChange(t *testing.T) {
	svc := &SupplyChainAppService{}
	mapping := models.SyncedProductMapping{
		SupplierCost: "2999", // $29.99
	}
	product := &contracts.StoreSyncProduct{
		Variants: []contracts.StoreSyncVariant{
			{RetailPrice: "29.99"}, // $29.99 = 2999 cents
		},
	}

	changes := svc.detectProductChanges(mapping, product)
	assert.Len(t, changes, 0)
}

func TestDetectProductChanges_Discontinued(t *testing.T) {
	svc := &SupplyChainAppService{}
	mapping := models.SyncedProductMapping{
		SupplierCost: "2000",
	}
	product := &contracts.StoreSyncProduct{
		Variants: []contracts.StoreSyncVariant{},
	}

	changes := svc.detectProductChanges(mapping, product)
	assert.Len(t, changes, 1)
	assert.Equal(t, models.RuleTriggerProductDiscontinued, changes[0].trigger)
}

func TestDetectProductChanges_MissingStoredCost(t *testing.T) {
	svc := &SupplyChainAppService{}
	mapping := models.SyncedProductMapping{
		SupplierCost: "",
	}
	product := &contracts.StoreSyncProduct{
		Variants: []contracts.StoreSyncVariant{
			{RetailPrice: "29.99"},
		},
	}

	changes := svc.detectProductChanges(mapping, product)
	assert.Len(t, changes, 0, "no changes when stored cost is empty (baseline not set)")
}

func TestSupplyChainMonitorServiceInterface(t *testing.T) {
	var _ contracts.SupplyChainService = (*SupplyChainAppService)(nil)
	_ = context.Background()
}
