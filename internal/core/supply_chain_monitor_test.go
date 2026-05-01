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

	assert.Equal(t, models.RuleAction("hide_listing"), models.RuleActionHideListing)
	assert.Equal(t, models.RuleAction("show_listing"), models.RuleActionShowListing)
	assert.Equal(t, models.RuleAction("pause_listing"), models.RuleActionPauseListing)
	assert.Equal(t, models.RuleAction("notify_only"), models.RuleActionNotifyOnly)
}

func TestAlertTypes(t *testing.T) {
	assert.Equal(t, models.AlertType("stock_out"), models.AlertTypeStockOut)
	assert.Equal(t, models.AlertType("stock_back"), models.AlertTypeStockBack)
	assert.Equal(t, models.AlertType("price_drift"), models.AlertTypePriceDrift)
	assert.Equal(t, models.AlertType("rule_action"), models.AlertTypeRuleAction)
}

func TestSupplyChainMonitorServiceInterface(t *testing.T) {
	var _ contracts.SupplyChainService = (*SupplyChainAppService)(nil)
	_ = context.Background()
}
