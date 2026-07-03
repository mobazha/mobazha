package checkoutsupply

import (
	"strconv"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func (s *CheckoutSupplyQuoteService) enrichSellerSummaryQuantities(
	summary *contracts.ListingSupplySummaryItem,
	sl *pb.SignedListing,
) {
	if s == nil || summary == nil {
		return
	}
	switch summary.SupplyMode {
	case contracts.ListingSupplyModeTrackedStock:
		s.enrichTrackedStockSummaryQuantities(summary, sl)
	case contracts.ListingSupplyModeLicenseCodes:
		s.enrichLicensePoolSummaryQuantities(summary)
	}
}

func (s *CheckoutSupplyQuoteService) enrichTrackedStockSummaryQuantities(
	summary *contracts.ListingSupplySummaryItem,
	sl *pb.SignedListing,
) {
	onHand := listingSkuOnHandQuantity(sl)
	if onHand >= 0 {
		q := onHand
		summary.OnHandQuantity = &q
	}
	if s.db == nil {
		return
	}
	held, err := s.listingSkuHeldQuantity(summary.ListingSlug)
	if err != nil || held <= 0 {
		return
	}
	summary.HeldQuantity = &held
}

func (s *CheckoutSupplyQuoteService) enrichLicensePoolSummaryQuantities(
	summary *contracts.ListingSupplySummaryItem,
) {
	if s.db == nil || summary.ListingSlug == "" {
		return
	}
	available, reserved, err := s.listingLicenseKeyPoolCounts(summary.ListingSlug)
	if err != nil {
		return
	}
	if available+reserved > 0 {
		onHand := available + reserved
		summary.OnHandQuantity = &onHand
	}
	if reserved > 0 {
		summary.HeldQuantity = &reserved
	}
}

func listingSkuOnHandQuantity(sl *pb.SignedListing) int64 {
	if sl == nil || sl.GetListing() == nil || sl.GetListing().GetItem() == nil {
		return -1
	}
	var total int64
	var hasQty bool
	for _, sku := range sl.GetListing().GetItem().GetSkus() {
		qty := strings.TrimSpace(sku.GetQuantity())
		if qty == "" || qty == "-1" {
			continue
		}
		parsed, err := strconv.ParseInt(qty, 10, 64)
		if err != nil || parsed < 0 {
			continue
		}
		total += parsed
		hasQty = true
	}
	if !hasQty {
		return -1
	}
	return total
}

func (s *CheckoutSupplyQuoteService) listingSkuHeldQuantity(listingSlug string) (int64, error) {
	var held int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.InventoryReservation{}).
			Where("listing_slug = ? AND released_at IS NULL", listingSlug).
			Select("COALESCE(SUM(quantity), 0)").Scan(&held).Error
	})
	return held, err
}

func (s *CheckoutSupplyQuoteService) listingLicenseKeyPoolCounts(listingSlug string) (available, reserved int64, err error) {
	err = s.db.View(func(tx database.Tx) error {
		if e := tx.Read().Model(&models.DigitalLicenseKey{}).
			Where("listing_slug = ? AND status = ?", listingSlug, models.LicenseKeyStatusAvailable).
			Count(&available).Error; e != nil {
			return e
		}
		return tx.Read().Model(&models.DigitalLicenseKey{}).
			Where("listing_slug = ? AND status = ?", listingSlug, models.LicenseKeyStatusReserved).
			Count(&reserved).Error
	})
	return available, reserved, err
}
