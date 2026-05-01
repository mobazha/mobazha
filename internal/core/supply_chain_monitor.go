package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

const (
	inventoryMonitorInterval = 5 * time.Minute
	priceDriftInterval       = 30 * time.Minute
	defaultPriceDriftPct     = 10.0 // 10% threshold
)

// ---------------------------------------------------------------------------
// M6.1: Inventory Monitor Worker
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) inventoryMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(inventoryMonitorInterval)
	defer ticker.Stop()
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: inventory monitor started (interval: %s)", inventoryMonitorInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.checkInventory(ctx)
		}
	}
}

func (s *SupplyChainAppService) checkInventory(ctx context.Context) {
	var mappings []models.SyncedProductMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("status IN ?", []string{"synced", "hidden"}).Find(&mappings).Error
	})
	if len(mappings) == 0 {
		return
	}

	providerMappings := make(map[string][]models.SyncedProductMapping)
	for _, m := range mappings {
		providerMappings[m.ProviderID] = append(providerMappings[m.ProviderID], m)
	}

	for providerID, pMappings := range providerMappings {
		provider, err := s.registry.ForProvider(providerID)
		if err != nil {
			continue
		}
		ssp, ok := provider.(contracts.FulfillmentStoreSyncProvider)
		if !ok {
			continue
		}
		s.checkProviderInventory(ctx, ssp, providerID, pMappings)
	}
}

func (s *SupplyChainAppService) checkProviderInventory(
	ctx context.Context,
	ssp contracts.FulfillmentStoreSyncProvider,
	providerID string,
	mappings []models.SyncedProductMapping,
) {
	for _, mapping := range mappings {
		if mapping.SyncProductID == "" {
			continue
		}
		product, err := ssp.GetStoreSyncProduct(ctx, mapping.SyncProductID)
		if err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: inventory check failed for %s: %v", mapping.ListingSlug, err)
			continue
		}

		allOutOfStock := true
		anyOutOfStock := false
		for _, v := range product.Variants {
			if v.InStock {
				allOutOfStock = false
			} else {
				anyOutOfStock = true
			}
		}

		if allOutOfStock && len(product.Variants) > 0 {
			s.handleStockOut(ctx, providerID, mapping)
		} else if anyOutOfStock {
			logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: partial OOS for %s (some variants unavailable)", mapping.ListingSlug)
		} else {
			s.handleStockBack(ctx, providerID, mapping)
		}
	}
}

func (s *SupplyChainAppService) handleStockOut(ctx context.Context, providerID string, mapping models.SyncedProductMapping) {
	existing := s.findActiveAlert(mapping.ListingSlug, models.AlertTypeStockOut)
	if existing != nil {
		return
	}

	alert := &models.SupplyChainAlert{
		ID:          uuid.NewString(),
		ProviderID:  providerID,
		ListingSlug: mapping.ListingSlug,
		AlertType:   models.AlertTypeStockOut,
		Severity:    models.AlertSeverityCritical,
		Title:       fmt.Sprintf("Out of Stock: %s", mapping.ListingSlug),
		Message:     "All variants are out of stock at the supplier. Consider hiding this listing.",
	}

	_ = s.db.Update(func(tx database.Tx) error {
		return tx.Save(alert)
	})

	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: OOS alert created for %s", mapping.ListingSlug)

	s.evaluateRules(ctx, models.RuleTriggerStockOut, providerID, mapping.ListingSlug, 0)
}

func (s *SupplyChainAppService) handleStockBack(ctx context.Context, providerID string, mapping models.SyncedProductMapping) {
	existing := s.findActiveAlert(mapping.ListingSlug, models.AlertTypeStockOut)
	if existing == nil {
		return
	}

	_ = s.db.Update(func(tx database.Tx) error {
		return tx.Update("dismissed", true, map[string]interface{}{"id": existing.ID}, &models.SupplyChainAlert{})
	})

	backAlert := &models.SupplyChainAlert{
		ID:          uuid.NewString(),
		ProviderID:  providerID,
		ListingSlug: mapping.ListingSlug,
		AlertType:   models.AlertTypeStockBack,
		Severity:    models.AlertSeverityInfo,
		Title:       fmt.Sprintf("Back in Stock: %s", mapping.ListingSlug),
		Message:     "Product is back in stock at the supplier.",
	}
	_ = s.db.Update(func(tx database.Tx) error {
		return tx.Save(backAlert)
	})

	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: stock restored for %s", mapping.ListingSlug)

	s.evaluateRules(ctx, models.RuleTriggerStockBack, providerID, mapping.ListingSlug, 0)
}

func (s *SupplyChainAppService) findActiveAlert(listingSlug string, alertType models.AlertType) *models.SupplyChainAlert {
	var alert models.SupplyChainAlert
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("listing_slug = ? AND alert_type = ? AND dismissed = ?", listingSlug, alertType, false).First(&alert).Error
	})
	if err != nil {
		return nil
	}
	return &alert
}

// ---------------------------------------------------------------------------
// M6.2: Price Drift Detector Worker
// ---------------------------------------------------------------------------

func (s *SupplyChainAppService) priceDriftDetectorLoop(ctx context.Context) {
	ticker := time.NewTicker(priceDriftInterval)
	defer ticker.Stop()
	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: price drift detector started (interval: %s)", priceDriftInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.detectPriceDrifts(ctx)
		}
	}
}

func (s *SupplyChainAppService) detectPriceDrifts(ctx context.Context) {
	var mappings []models.SyncedProductMapping
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("status = ? AND supplier_cost != ''", "synced").Find(&mappings).Error
	})
	if len(mappings) == 0 {
		return
	}

	providerMappings := make(map[string][]models.SyncedProductMapping)
	for _, m := range mappings {
		providerMappings[m.ProviderID] = append(providerMappings[m.ProviderID], m)
	}

	for providerID, pMappings := range providerMappings {
		provider, err := s.registry.ForProvider(providerID)
		if err != nil {
			continue
		}
		ssp, ok := provider.(contracts.FulfillmentStoreSyncProvider)
		if !ok {
			continue
		}
		s.checkProviderPrices(ctx, ssp, providerID, pMappings)
	}
}

func (s *SupplyChainAppService) checkProviderPrices(
	ctx context.Context,
	ssp contracts.FulfillmentStoreSyncProvider,
	providerID string,
	mappings []models.SyncedProductMapping,
) {
	threshold := s.getPriceDriftThreshold(providerID)

	for _, mapping := range mappings {
		if mapping.SyncProductID == "" || mapping.SupplierCost == "" {
			continue
		}
		product, err := ssp.GetStoreSyncProduct(ctx, mapping.SyncProductID)
		if err != nil {
			continue
		}

		storedCost, err := strconv.ParseFloat(mapping.SupplierCost, 64)
		if err != nil || storedCost == 0 {
			continue
		}

		currentMinCost := s.findMinVariantCost(product.Variants)
		if currentMinCost == 0 {
			continue
		}

		driftPct := math.Abs(currentMinCost-storedCost) / storedCost * 100
		if driftPct >= threshold {
			s.handlePriceDrift(ctx, providerID, mapping, storedCost, currentMinCost, driftPct)
		}
	}
}

// getPriceDriftThreshold returns the MINIMUM threshold across all applicable
// price_drift rules for a provider. This ensures an alert is created as soon as
// the lowest-threshold rule fires (e.g. 10% notify), while each rule's own
// Threshold is re-checked during evaluateRules to gate its specific action.
// Applicable rules: provider-specific (ProviderID == providerID) + global
// (ProviderID == ""). Falls back to hardcoded default if no rules exist.
func (s *SupplyChainAppService) getPriceDriftThreshold(providerID string) float64 {
	var rules []models.AutoActionRule
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("trigger = ? AND enabled = ? AND threshold > 0",
			models.RuleTriggerPriceDrift, true).Find(&rules).Error
	})

	minThreshold := 0.0
	for _, r := range rules {
		if r.ProviderID != "" && r.ProviderID != providerID {
			continue
		}
		if minThreshold == 0 || r.Threshold < minThreshold {
			minThreshold = r.Threshold
		}
	}
	if minThreshold > 0 {
		return minThreshold
	}
	return defaultPriceDriftPct
}

// findMinVariantCost returns the minimum cost across variants in CENTS (uint64
// stored as float64 for comparison). The provider returns prices as USD dollar
// strings (e.g. "29.99"), so we convert to cents using parseUSDDollarsToCents
// to match the unit stored in SyncedProductMapping.SupplierCost.
func (s *SupplyChainAppService) findMinVariantCost(variants []contracts.StoreSyncVariant) float64 {
	var minCents uint64
	for _, v := range variants {
		cents, ok := parseUSDDollarsToCents(v.RetailPrice)
		if !ok || cents == 0 {
			continue
		}
		if minCents == 0 || cents < minCents {
			minCents = cents
		}
	}
	return float64(minCents)
}

func (s *SupplyChainAppService) handlePriceDrift(
	ctx context.Context,
	providerID string,
	mapping models.SyncedProductMapping,
	storedCost, currentCost, driftPct float64,
) {
	existing := s.findActiveAlert(mapping.ListingSlug, models.AlertTypePriceDrift)
	if existing != nil {
		return
	}

	direction := "increased"
	if currentCost < storedCost {
		direction = "decreased"
	}

	// Convert cents to dollars for user-facing display.
	storedDollars := storedCost / 100
	currentDollars := currentCost / 100

	metadata, _ := json.Marshal(map[string]interface{}{
		"storedCostCents":  storedCost,
		"currentCostCents": currentCost,
		"storedCost":       storedDollars,
		"currentCost":      currentDollars,
		"driftPct":         driftPct,
		"direction":        direction,
	})

	alert := &models.SupplyChainAlert{
		ID:          uuid.NewString(),
		ProviderID:  providerID,
		ListingSlug: mapping.ListingSlug,
		AlertType:   models.AlertTypePriceDrift,
		Severity:    models.AlertSeverityWarning,
		Title:       fmt.Sprintf("Price %s %.1f%%: %s", direction, driftPct, mapping.ListingSlug),
		Message:     fmt.Sprintf("Supplier cost %s from $%.2f to $%.2f (%.1f%% drift). Consider updating your retail price.", direction, storedDollars, currentDollars, driftPct),
		Metadata:    metadata,
	}

	_ = s.db.Update(func(tx database.Tx) error {
		return tx.Save(alert)
	})

	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: price drift alert for %s (%.1f%%)", mapping.ListingSlug, driftPct)

	s.evaluateRules(ctx, models.RuleTriggerPriceDrift, providerID, mapping.ListingSlug, driftPct)
}

// ---------------------------------------------------------------------------
// M6.3: Auto-Action Rule Engine
// ---------------------------------------------------------------------------

// evaluateRules checks all enabled rules for the given trigger/provider pair.
// For price_drift triggers, driftPct allows per-rule threshold gating: a rule
// only fires if the observed drift exceeds that rule's own Threshold. For
// stock-based triggers, pass driftPct=0 (threshold check is skipped).
func (s *SupplyChainAppService) evaluateRules(ctx context.Context, trigger models.RuleTrigger, providerID, listingSlug string, driftPct float64) {
	var rules []models.AutoActionRule
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("trigger = ? AND enabled = ?", trigger, true).Find(&rules).Error
	})

	for _, rule := range rules {
		if rule.ProviderID != "" && rule.ProviderID != providerID {
			continue
		}
		if trigger == models.RuleTriggerPriceDrift && rule.Threshold > 0 && driftPct < rule.Threshold {
			continue
		}
		s.executeRuleAction(ctx, rule, listingSlug)
	}
}

func (s *SupplyChainAppService) executeRuleAction(ctx context.Context, rule models.AutoActionRule, listingSlug string) {
	switch rule.Action {
	case models.RuleActionHideListing:
		s.setListingVisibility(ctx, listingSlug, false)
		s.recordRuleAction(rule, listingSlug, "hidden")
	case models.RuleActionShowListing:
		s.setListingVisibility(ctx, listingSlug, true)
		s.recordRuleAction(rule, listingSlug, "shown")
	case models.RuleActionPauseListing:
		s.setListingVisibility(ctx, listingSlug, false)
		s.recordRuleAction(rule, listingSlug, "paused")
	case models.RuleActionNotifyOnly:
		logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: rule %s triggered notify_only for %s", rule.ID, listingSlug)
	}
}

func (s *SupplyChainAppService) setListingVisibility(ctx context.Context, listingSlug string, visible bool) {
	if !visible {
		// Hiding: record the current listing status so we can restore correctly.
		var currentStatus string
		if s.listingOps != nil {
			currentStatus = s.getListingCurrentStatus(listingSlug)
		}
		_ = s.db.Update(func(tx database.Tx) error {
			var m models.SyncedProductMapping
			if err := tx.Read().Where("listing_slug = ?", listingSlug).First(&m).Error; err != nil {
				return err
			}
			m.Status = "hidden"
			m.PreviousListingStatus = currentStatus
			return tx.Save(&m)
		})
		if s.listingOps != nil {
			if err := s.listingOps.SetListingStatus(listingSlug, models.ListingStatusDraft); err != nil {
				logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to hide listing %s: %v", listingSlug, err)
			}
		}
	} else {
		// Showing: only restore to "published" if the listing was published before
		// automation hid it. This prevents auto-publishing seller drafts.
		var mapping models.SyncedProductMapping
		_ = s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("listing_slug = ?", listingSlug).First(&mapping).Error
		})

		_ = s.db.Update(func(tx database.Tx) error {
			return tx.Update("status", "synced", map[string]interface{}{"listing_slug": listingSlug}, &models.SyncedProductMapping{})
		})

		if s.listingOps != nil && mapping.PreviousListingStatus == models.ListingStatusPublished {
			if err := s.listingOps.SetListingStatus(listingSlug, models.ListingStatusPublished); err != nil {
				logger.LogErrorWithIDf(log, s.nodeID, "SupplyChain: failed to show listing %s: %v", listingSlug, err)
			}
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "SupplyChain: listing %s visibility set to %v", listingSlug, visible)
}

// getListingCurrentStatus retrieves the current listing status via listingOps.
func (s *SupplyChainAppService) getListingCurrentStatus(listingSlug string) string {
	if s.listingOps == nil {
		return ""
	}
	sl, err := s.listingOps.GetListingStatus(listingSlug)
	if err != nil {
		return ""
	}
	return sl
}

func (s *SupplyChainAppService) recordRuleAction(rule models.AutoActionRule, listingSlug, action string) {
	alert := &models.SupplyChainAlert{
		ID:          uuid.NewString(),
		ProviderID:  rule.ProviderID,
		ListingSlug: listingSlug,
		AlertType:   models.AlertTypeRuleAction,
		Severity:    models.AlertSeverityInfo,
		Title:       fmt.Sprintf("Auto-action: %s on %s", action, listingSlug),
		Message:     fmt.Sprintf("Rule '%s → %s' triggered. Listing %s.", rule.Trigger, rule.Action, action),
		ActionTaken: string(rule.Action),
	}
	_ = s.db.Update(func(tx database.Tx) error {
		return tx.Save(alert)
	})
}

// ---------------------------------------------------------------------------
// Alert & Rule CRUD (called by handlers)
// ---------------------------------------------------------------------------

// ListAlerts returns active alerts, newest first.
func (s *SupplyChainAppService) ListAlerts(ctx context.Context, dismissed bool, limit int) ([]contracts.SupplyChainAlert, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var dbAlerts []models.SupplyChainAlert
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Where("dismissed = ?", dismissed).Order("created_at DESC").Limit(limit)
		return q.Find(&dbAlerts).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]contracts.SupplyChainAlert, len(dbAlerts))
	for i, a := range dbAlerts {
		result[i] = contracts.SupplyChainAlert{
			ID:          a.ID,
			ProviderID:  a.ProviderID,
			ListingSlug: a.ListingSlug,
			AlertType:   string(a.AlertType),
			Severity:    string(a.Severity),
			Title:       a.Title,
			Message:     a.Message,
			Dismissed:   a.Dismissed,
			ActionTaken: a.ActionTaken,
			CreatedAt:   a.CreatedAt,
		}
	}
	return result, nil
}

// DismissAlert marks an alert as dismissed.
func (s *SupplyChainAppService) DismissAlert(ctx context.Context, alertID string) error {
	return s.db.Update(func(tx database.Tx) error {
		var alert models.SupplyChainAlert
		if err := tx.Read().Where("id = ?", alertID).First(&alert).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("alert not found")
			}
			return err
		}
		return tx.Update("dismissed", true, map[string]interface{}{"id": alertID}, &models.SupplyChainAlert{})
	})
}

// ListRules returns all auto-action rules.
func (s *SupplyChainAppService) ListRules(ctx context.Context) ([]contracts.AutoActionRule, error) {
	var dbRules []models.AutoActionRule
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Order("created_at DESC").Find(&dbRules).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]contracts.AutoActionRule, len(dbRules))
	for i, r := range dbRules {
		enabled := r.Enabled
		result[i] = contracts.AutoActionRule{
			ID:         r.ID,
			ProviderID: r.ProviderID,
			Trigger:    string(r.Trigger),
			Action:     string(r.Action),
			Threshold:  r.Threshold,
			Enabled:    &enabled,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		}
	}
	return result, nil
}

// CreateRule creates a new auto-action rule. Enabled defaults to true when
// the caller does not explicitly set it (Go zero-value false is treated as
// "not specified" since the API DTO uses a plain bool).
func (s *SupplyChainAppService) CreateRule(ctx context.Context, rule *contracts.AutoActionRule) error {
	enabled := true
	if rule.Enabled != nil {
		enabled = *rule.Enabled
	}
	dbRule := &models.AutoActionRule{
		ID:         uuid.NewString(),
		ProviderID: rule.ProviderID,
		Trigger:    models.RuleTrigger(rule.Trigger),
		Action:     models.RuleAction(rule.Action),
		Threshold:  rule.Threshold,
		Enabled:    enabled,
	}
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(dbRule)
	})
	if err != nil {
		return err
	}
	rule.ID = dbRule.ID
	return nil
}

// DeleteRule removes an auto-action rule.
func (s *SupplyChainAppService) DeleteRule(ctx context.Context, ruleID string) error {
	return s.db.Update(func(tx database.Tx) error {
		var rule models.AutoActionRule
		if err := tx.Read().Where("id = ?", ruleID).First(&rule).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("rule not found")
			}
			return err
		}
		return tx.Delete("id", ruleID, nil, &models.AutoActionRule{})
	})
}
