package checkoutsupply

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

type sellerSummaryReadModel struct {
	listings         map[string]*pb.SignedListing
	reservedByBucket map[checkoutInventoryBucketKey]int64
	heldBySlug       map[string]int64
	licenseAvailable map[string]int64
	licenseReserved  map[string]int64
	digitalAssets    map[string]sellerSummaryDigitalAssets
	externalMappings map[string]models.SyncedProductMapping
}

type sellerSummaryDigitalAssets struct {
	hasLicenseKey bool
	hasUnlimited  bool
}

type sellerSummaryReservationRow struct {
	ListingSlug string `gorm:"column:listing_slug"`
	VariantHash string `gorm:"column:variant_hash"`
	Quantity    int64  `gorm:"column:quantity"`
}

type sellerSummaryLicenseCountRow struct {
	ListingSlug string `gorm:"column:listing_slug"`
	Status      string `gorm:"column:status"`
	Count       int64  `gorm:"column:count"`
}

func (s *CheckoutSupplyQuoteService) canUseSellerSummaryReadModel() bool {
	if s == nil || s.db == nil {
		return false
	}
	return true
}

func (s *CheckoutSupplyQuoteService) sellerSummaryFromReadModel(
	ctx context.Context,
	slugs []string,
) ([]contracts.ListingSupplySummaryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	model := sellerSummaryReadModel{
		listings:         make(map[string]*pb.SignedListing, len(slugs)),
		reservedByBucket: make(map[checkoutInventoryBucketKey]int64),
		heldBySlug:       make(map[string]int64),
		licenseAvailable: make(map[string]int64),
		licenseReserved:  make(map[string]int64),
		digitalAssets:    make(map[string]sellerSummaryDigitalAssets),
		externalMappings: make(map[string]models.SyncedProductMapping),
	}

	err := s.db.View(func(tx database.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		foundSlugs := make([]string, 0, len(slugs))
		for _, slug := range slugs {
			sl, err := tx.GetListing(slug)
			if err != nil || sl == nil || sl.GetListing() == nil {
				continue
			}
			model.listings[slug] = sl
			foundSlugs = append(foundSlugs, slug)
		}
		if len(foundSlugs) == 0 {
			return nil
		}
		if err := loadSellerSummaryReservationsInTx(tx, foundSlugs, model.reservedByBucket, model.heldBySlug); err != nil {
			return err
		}
		if err := loadSellerSummaryLicenseCountsInTx(tx, foundSlugs, model.licenseAvailable, model.licenseReserved); err != nil {
			return err
		}
		if err := loadSellerSummaryDigitalAssetsInTx(tx, foundSlugs, model.digitalAssets); err != nil {
			return err
		}

		externalItems := make([]models.GuestOrderItem, 0, len(foundSlugs))
		for _, slug := range foundSlugs {
			if listingContractType(model.listings[slug]) != pb.Listing_Metadata_PHYSICAL_GOOD {
				continue
			}
			externalItems = append(externalItems, models.GuestOrderItem{
				ListingSlug:  slug,
				ContractType: pb.Listing_Metadata_PHYSICAL_GOOD.String(),
			})
		}
		mappings, err := checkoutExternalSupplyMappingsForItemsInTx(tx, externalItems)
		if err != nil {
			return err
		}
		model.externalMappings = mappings
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load seller supply summary read model: %w", err)
	}

	items := make([]contracts.ListingSupplySummaryItem, 0, len(slugs))
	for _, slug := range slugs {
		sl := model.listings[slug]
		if sl == nil {
			items = append(items, s.sellerSummaryItemViaQuote(ctx, slug))
			continue
		}
		items = append(items, sellerSummaryItemFromReadModel(slug, sl, model))
	}
	return items, nil
}

func loadSellerSummaryReservationsInTx(
	tx database.Tx,
	slugs []string,
	reservedByBucket map[checkoutInventoryBucketKey]int64,
	heldBySlug map[string]int64,
) error {
	var rows []sellerSummaryReservationRow
	err := tx.Read().Model(&models.InventoryReservation{}).
		Where("listing_slug IN ? AND released_at IS NULL", slugs).
		Select("listing_slug, variant_hash, COALESCE(SUM(quantity), 0) AS quantity").
		Group("listing_slug, variant_hash").
		Scan(&rows).Error
	if ignoreMissingSellerSummaryTable(err, models.InventoryReservation{}.TableName()) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, row := range rows {
		reservedByBucket[checkoutInventoryBucketKey{Slug: row.ListingSlug, VariantHash: row.VariantHash}] = row.Quantity
		heldBySlug[row.ListingSlug] += row.Quantity
	}
	return nil
}

func loadSellerSummaryLicenseCountsInTx(
	tx database.Tx,
	slugs []string,
	availableBySlug map[string]int64,
	reservedBySlug map[string]int64,
) error {
	var rows []sellerSummaryLicenseCountRow
	err := tx.Read().Model(&models.DigitalLicenseKey{}).
		Where("listing_slug IN ? AND status IN ?", slugs, []string{
			models.LicenseKeyStatusAvailable,
			models.LicenseKeyStatusReserved,
		}).
		Select("listing_slug, status, COUNT(*) AS count").
		Group("listing_slug, status").
		Scan(&rows).Error
	if ignoreMissingSellerSummaryTable(err, models.DigitalLicenseKey{}.TableName()) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, row := range rows {
		switch row.Status {
		case models.LicenseKeyStatusAvailable:
			availableBySlug[row.ListingSlug] += row.Count
		case models.LicenseKeyStatusReserved:
			reservedBySlug[row.ListingSlug] += row.Count
		}
	}
	return nil
}

func loadSellerSummaryDigitalAssetsInTx(
	tx database.Tx,
	slugs []string,
	assetsBySlug map[string]sellerSummaryDigitalAssets,
) error {
	var assets []models.DigitalAsset
	err := tx.Read().
		Where("listing_slug IN ?", slugs).
		Find(&assets).Error
	if ignoreMissingSellerSummaryTable(err, models.DigitalAsset{}.TableName()) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, asset := range assets {
		flags := assetsBySlug[asset.ListingSlug]
		switch asset.AssetType {
		case models.AssetTypeLicenseKey:
			flags.hasLicenseKey = true
		case models.AssetTypeFile, models.AssetTypeLink:
			flags.hasUnlimited = true
		}
		assetsBySlug[asset.ListingSlug] = flags
	}
	return nil
}

func sellerSummaryItemFromReadModel(
	slug string,
	sl *pb.SignedListing,
	model sellerSummaryReadModel,
) contracts.ListingSupplySummaryItem {
	switch listingContractType(sl) {
	case pb.Listing_Metadata_DIGITAL_GOOD:
		return sellerSummaryDigitalItemFromReadModel(slug, model)
	default:
		if mapping, ok := model.externalMappings[slug]; ok {
			return sellerSummaryExternalItemFromReadModel(slug, mapping)
		}
		return sellerSummaryTrackedStockItemFromReadModel(slug, sl, model)
	}
}

func sellerSummaryTrackedStockItemFromReadModel(
	slug string,
	sl *pb.SignedListing,
	model sellerSummaryReadModel,
) contracts.ListingSupplySummaryItem {
	summary := contracts.ListingSupplySummaryItem{
		ListingSlug: slug,
		SupplyMode:  contracts.ListingSupplyModeTrackedStock,
		Status:      contracts.SupplyAvailabilityUnlimited,
	}
	item := sl.GetListing().GetItem()
	if item == nil {
		return summary
	}

	var onHand int64
	var totalAvailable int64
	var hasTrackedStock bool
	var hasOutOfStockVariant bool
	for _, sku := range item.GetSkus() {
		qty := strings.TrimSpace(sku.GetQuantity())
		if qty == "" || qty == "-1" {
			continue
		}
		parsed, err := strconv.ParseInt(qty, 10, 64)
		if err != nil {
			return unknownSellerSupplySummaryItem(slug, "quote_unavailable")
		}
		if parsed < 0 {
			continue
		}

		hasTrackedStock = true
		onHand += parsed
		bucket := checkoutInventoryBucketKey{
			Slug:        slug,
			VariantHash: computeCheckoutVariantHashFromSku(sku),
		}
		available := parsed - model.reservedByBucket[bucket]
		if available < 0 {
			available = 0
		}
		if available == 0 {
			hasOutOfStockVariant = true
		}
		totalAvailable += available
	}
	if !hasTrackedStock {
		return summary
	}

	summary.OnHandQuantity = int64Ptr(onHand)
	summary.AvailableQuantity = int64Ptr(totalAvailable)
	if held := model.heldBySlug[slug]; held > 0 {
		summary.HeldQuantity = int64Ptr(held)
	}
	switch {
	case totalAvailable == 0:
		summary.Status = contracts.SupplyAvailabilityOutOfStock
	case hasOutOfStockVariant:
		summary.Status = contracts.SupplyAvailabilityLowStock
	default:
		summary.Status = contracts.SupplyAvailabilityAvailable
	}
	return summary
}

func sellerSummaryDigitalItemFromReadModel(
	slug string,
	model sellerSummaryReadModel,
) contracts.ListingSupplySummaryItem {
	assets := model.digitalAssets[slug]
	if assets.hasLicenseKey {
		available := model.licenseAvailable[slug]
		reserved := model.licenseReserved[slug]
		summary := contracts.ListingSupplySummaryItem{
			ListingSlug:       slug,
			SupplyMode:        contracts.ListingSupplyModeLicenseCodes,
			AvailableQuantity: int64Ptr(available),
		}
		if available > 0 {
			summary.Status = contracts.SupplyAvailabilityAvailable
		} else {
			summary.Status = contracts.SupplyAvailabilityOutOfStock
			summary.Reason = "license_key_pool_exhausted"
		}
		if available+reserved > 0 {
			summary.OnHandQuantity = int64Ptr(available + reserved)
		}
		if reserved > 0 {
			summary.HeldQuantity = int64Ptr(reserved)
		}
		return summary
	}
	if assets.hasUnlimited {
		return contracts.ListingSupplySummaryItem{
			ListingSlug: slug,
			SupplyMode:  contracts.ListingSupplyModeInstantDownload,
			Status:      contracts.SupplyAvailabilityUnlimited,
		}
	}
	return contracts.ListingSupplySummaryItem{
		ListingSlug:          slug,
		SupplyMode:           contracts.ListingSupplyModeInstantDownload,
		Status:               contracts.SupplyAvailabilityManualActionRequired,
		ManualActionRequired: true,
		Reason:               "digital_asset_missing",
	}
}

func sellerSummaryExternalItemFromReadModel(
	slug string,
	mapping models.SyncedProductMapping,
) contracts.ListingSupplySummaryItem {
	status := strings.TrimSpace(mapping.Status)
	if status == "" || status == "synced" {
		return contracts.ListingSupplySummaryItem{
			ListingSlug: slug,
			SupplyMode:  contracts.ListingSupplyModeSupplierFulfilled,
			Status:      contracts.SupplyAvailabilityAvailable,
		}
	}
	return contracts.ListingSupplySummaryItem{
		ListingSlug: slug,
		SupplyMode:  contracts.ListingSupplyModeSupplierFulfilled,
		Status:      contracts.SupplyAvailabilitySupplierUnavailable,
		Reason:      "supplier_mapping_" + status,
	}
}

func listingContractType(sl *pb.SignedListing) pb.Listing_Metadata_ContractType {
	if sl == nil || sl.GetListing() == nil || sl.GetListing().GetMetadata() == nil {
		return pb.Listing_Metadata_PHYSICAL_GOOD
	}
	return sl.GetListing().GetMetadata().GetContractType()
}

func int64Ptr(v int64) *int64 {
	return &v
}

func ignoreMissingSellerSummaryTable(err error, table string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, table) &&
		(strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist"))
}
