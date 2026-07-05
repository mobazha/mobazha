package digital

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

var _ contracts.TransactionalDigitalSupplyLineResolver = (*DigitalAssetAppService)(nil)

// LicenseKeyPoolSupplyLinesForOrderItems resolves order line items whose
// scarcity is controlled by a DigitalLicenseKey pool. It is intentionally
// narrower than full digital supply resolution: file/link-only assets are left
// for UnlimitedDigitalProvider work.
func (s *DigitalAssetAppService) LicenseKeyPoolSupplyLinesForOrderItems(items []OrderLineItem) ([]contracts.SupplyLine, error) {
	if s == nil || len(items) == 0 {
		return nil, nil
	}

	lines := make([]contracts.SupplyLine, 0, len(items))
	for itemIndex, item := range items {
		if item.ListingSlug == "" {
			continue
		}
		assets, err := s.getAssetModelsByListing(item.ListingSlug, item.VariantSKU)
		if err != nil {
			return nil, fmt.Errorf("resolve digital supply assets for %s/%s: %w", item.ListingSlug, item.VariantSKU, err)
		}
		licenseAsset, ok := firstLicenseKeyAssetForVariant(assets, item.VariantSKU)
		if !ok {
			continue
		}
		qty := int(item.Quantity)
		if qty == 0 {
			qty = 1
		}
		lines = append(lines, contracts.SupplyLine{
			LineID:      fmt.Sprintf("digital:%d:%s:%s:license_key_pool", itemIndex, item.ListingSlug, licenseAsset.VariantSKU),
			ListingSlug: item.ListingSlug,
			VariantSKU:  licenseAsset.VariantSKU,
			Quantity:    qty,
			SupplyKind:  contracts.SupplyKindLicenseKeyPool,
		})
	}
	return lines, nil
}

// UnlimitedDigitalSupplyLinesForOrderItems resolves non-scarce file/link
// digital assets. If a line has a license-key asset, the license pool controls
// scarcity and this resolver intentionally skips the item.
func (s *DigitalAssetAppService) UnlimitedDigitalSupplyLinesForOrderItems(items []OrderLineItem) ([]contracts.SupplyLine, error) {
	if s == nil || len(items) == 0 {
		return nil, nil
	}

	lines := make([]contracts.SupplyLine, 0, len(items))
	for itemIndex, item := range items {
		if item.ListingSlug == "" {
			continue
		}
		assets, err := s.getAssetModelsByListing(item.ListingSlug, item.VariantSKU)
		if err != nil {
			return nil, fmt.Errorf("resolve unlimited digital assets for %s/%s: %w", item.ListingSlug, item.VariantSKU, err)
		}
		if _, ok := firstLicenseKeyAssetForVariant(assets, item.VariantSKU); ok {
			continue
		}
		asset, ok := firstUnlimitedDigitalAssetForVariant(assets, item.VariantSKU)
		if !ok {
			continue
		}
		qty := int(item.Quantity)
		if qty == 0 {
			qty = 1
		}
		lines = append(lines, contracts.SupplyLine{
			LineID:      fmt.Sprintf("digital:%d:%s:%s:unlimited_digital", itemIndex, item.ListingSlug, asset.VariantSKU),
			ListingSlug: item.ListingSlug,
			VariantSKU:  asset.VariantSKU,
			Quantity:    qty,
			SupplyKind:  contracts.SupplyKindUnlimitedDigital,
		})
	}
	return lines, nil
}

// SupplyAvailabilityLinesForOrderItems resolves digital supply lines in
// order-item order. Scarce license keys win over unlimited file/link delivery
// when both are configured for the same item. Missing delivery assets still
// produce a manual-action unlimited-digital line so checkout fails closed
// instead of silently skipping the digital item.
func (s *DigitalAssetAppService) SupplyAvailabilityLinesForOrderItems(items []OrderLineItem) ([]contracts.SupplyLine, error) {
	if s == nil {
		return nil, nil
	}
	return supplyAvailabilityLinesForOrderItems(items, s.getAssetModelsByListing)
}

// SupplyAvailabilityLinesForOrderItemsTx resolves the same supply contract
// using an existing database transaction. Standard ORDER_OPEN processing must
// use this path because TenantDB deliberately serializes View and Update; a
// nested View from inside Update would otherwise deadlock cold-start replay.
func (s *DigitalAssetAppService) SupplyAvailabilityLinesForOrderItemsTx(tx database.Tx, items []OrderLineItem) ([]contracts.SupplyLine, error) {
	if s == nil || tx == nil {
		return nil, nil
	}
	return supplyAvailabilityLinesForOrderItems(items, func(listingSlug, variantSKU string) ([]models.DigitalAsset, error) {
		return getAssetModelsByListingTx(tx, listingSlug, variantSKU)
	})
}

func supplyAvailabilityLinesForOrderItems(
	items []OrderLineItem,
	loadAssets func(listingSlug, variantSKU string) ([]models.DigitalAsset, error),
) ([]contracts.SupplyLine, error) {
	if len(items) == 0 {
		return nil, nil
	}

	lines := make([]contracts.SupplyLine, 0, len(items))
	for itemIndex, item := range items {
		if item.ListingSlug == "" {
			continue
		}
		assets, err := loadAssets(item.ListingSlug, item.VariantSKU)
		if err != nil {
			return nil, fmt.Errorf("resolve digital supply assets for %s/%s: %w", item.ListingSlug, item.VariantSKU, err)
		}
		qty := int(item.Quantity)
		if qty == 0 {
			qty = 1
		}
		if asset, ok := firstLicenseKeyAssetForVariant(assets, item.VariantSKU); ok {
			lines = append(lines, contracts.SupplyLine{
				LineID:      fmt.Sprintf("digital:%d:%s:%s:license_key_pool", itemIndex, item.ListingSlug, asset.VariantSKU),
				ListingSlug: item.ListingSlug,
				VariantSKU:  asset.VariantSKU,
				Quantity:    qty,
				SupplyKind:  contracts.SupplyKindLicenseKeyPool,
			})
			continue
		}
		if asset, ok := firstUnlimitedDigitalAssetForVariant(assets, item.VariantSKU); ok {
			lines = append(lines, contracts.SupplyLine{
				LineID:      fmt.Sprintf("digital:%d:%s:%s:unlimited_digital", itemIndex, item.ListingSlug, asset.VariantSKU),
				ListingSlug: item.ListingSlug,
				VariantSKU:  asset.VariantSKU,
				Quantity:    qty,
				SupplyKind:  contracts.SupplyKindUnlimitedDigital,
			})
			continue
		}
		lines = append(lines, missingDigitalAssetSupplyLine(itemIndex, item, qty))
	}
	return lines, nil
}

func missingDigitalAssetSupplyLine(itemIndex int, item OrderLineItem, qty int) contracts.SupplyLine {
	return contracts.SupplyLine{
		LineID:      fmt.Sprintf("digital:%d:%s:%s:missing_digital_asset", itemIndex, item.ListingSlug, item.VariantSKU),
		ListingSlug: item.ListingSlug,
		VariantSKU:  item.VariantSKU,
		Quantity:    qty,
		SupplyKind:  contracts.SupplyKindUnlimitedDigital,
		Metadata: map[string]string{
			"manualActionReason": "digital_asset_missing",
		},
	}
}

func firstLicenseKeyAsset(assets []models.DigitalAsset) (models.DigitalAsset, bool) {
	for _, asset := range assets {
		if asset.AssetType == models.AssetTypeLicenseKey {
			return asset, true
		}
	}
	return models.DigitalAsset{}, false
}

func firstLicenseKeyAssetForVariant(assets []models.DigitalAsset, variantSKU string) (models.DigitalAsset, bool) {
	variantSKU = strings.TrimSpace(variantSKU)
	if variantSKU != "" {
		for _, asset := range assets {
			if asset.AssetType == models.AssetTypeLicenseKey && asset.VariantSKU == variantSKU {
				return asset, true
			}
		}
	}
	return firstLicenseKeyAsset(assets)
}

func firstUnlimitedDigitalAsset(assets []models.DigitalAsset) (models.DigitalAsset, bool) {
	for _, asset := range assets {
		switch asset.AssetType {
		case models.AssetTypeFile, models.AssetTypeLink:
			return asset, true
		}
	}
	return models.DigitalAsset{}, false
}

func firstUnlimitedDigitalAssetForVariant(assets []models.DigitalAsset, variantSKU string) (models.DigitalAsset, bool) {
	variantSKU = strings.TrimSpace(variantSKU)
	if variantSKU != "" {
		for _, asset := range assets {
			switch asset.AssetType {
			case models.AssetTypeFile, models.AssetTypeLink:
				if asset.VariantSKU == variantSKU {
					return asset, true
				}
			}
		}
	}
	return firstUnlimitedDigitalAsset(assets)
}
