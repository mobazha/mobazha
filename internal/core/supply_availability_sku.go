package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// SkuQuantityProvider implements the existing InventoryReservation semantics
// behind the SupplyAvailability provider boundary.
type SkuQuantityProvider struct {
	db database.Database
}

func NewSkuQuantityProvider(db database.Database) *SkuQuantityProvider {
	return &SkuQuantityProvider{db: db}
}

func (p *SkuQuantityProvider) Kind() contracts.SupplyKind {
	return contracts.SupplyKindSkuQuantity
}

func (p *SkuQuantityProvider) GetAvailability(ctx context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateSkuSupplyLine(req.Line); err != nil {
		return nil, err
	}

	result := baseSkuAvailabilityResult(req.Line)
	if !req.Line.StockTracked {
		result.Status = contracts.SupplyAvailabilityUnlimited
		result.Available = true
		result.Unlimited = true
		result.CheckedAt = checkedAt(req.CheckedAt)
		return result, nil
	}

	err := p.db.View(func(tx database.Tx) error {
		reserved, err := skuReservedQuantity(tx, req.Line.ListingSlug, req.Line.VariantHash)
		if err != nil {
			return err
		}
		applySkuStockAvailability(result, req.Line, reserved)
		result.CheckedAt = checkedAt(req.CheckedAt)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *SkuQuantityProvider) Reserve(ctx context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateSkuReserveRequest(req); err != nil {
		return nil, err
	}

	var reservation contracts.SupplyReservation
	err := p.db.Update(func(tx database.Tx) error {
		res, err := reserveSkuQuantityInTx(tx, req, time.Now())
		if err != nil {
			return err
		}
		reservation = *res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (p *SkuQuantityProvider) ReserveTx(ctx context.Context, tx database.Tx, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("supply availability: transaction is required")
	}
	if err := validateSkuReserveRequest(req); err != nil {
		return nil, err
	}
	return reserveSkuQuantityInTx(tx, req, time.Now())
}

func (p *SkuQuantityProvider) Commit(ctx context.Context, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.OrderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return errors.New("supply availability: order type is required")
	}

	return p.db.Update(func(tx database.Tx) error {
		return commitSkuQuantityInTx(tx, req)
	})
}

func (p *SkuQuantityProvider) CommitTx(ctx context.Context, tx database.Tx, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("supply availability: transaction is required")
	}
	if req.OrderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return errors.New("supply availability: order type is required")
	}
	return commitSkuQuantityInTx(tx, req)
}

func (p *SkuQuantityProvider) Release(ctx context.Context, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.OrderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return errors.New("supply availability: order type is required")
	}

	return p.db.Update(func(tx database.Tx) error {
		return releaseSkuQuantityInTx(tx, req, time.Now())
	})
}

func (p *SkuQuantityProvider) ReleaseTx(ctx context.Context, tx database.Tx, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("supply availability: transaction is required")
	}
	if req.OrderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return errors.New("supply availability: order type is required")
	}
	return releaseSkuQuantityInTx(tx, req, time.Now())
}

func reserveSkuQuantityInTx(tx database.Tx, req contracts.ReserveSupplyRequest, reservedAt time.Time) (*contracts.SupplyReservation, error) {
	existing, found, err := existingSkuReservationInTx(tx, req)
	if err != nil {
		return nil, err
	}
	if found {
		return skuReservationFromRow(existing, req.Line), nil
	}

	if req.Line.StockTracked {
		reserved, err := skuReservedQuantity(tx, req.Line.ListingSlug, req.Line.VariantHash)
		if err != nil {
			return nil, fmt.Errorf("check reserved quantity for %s: %w", req.Line.ListingSlug, err)
		}
		if reserved+int64(req.Line.Quantity) > req.Line.StockLimit {
			available := req.Line.StockLimit - reserved
			if available < 0 {
				available = 0
			}
			return nil, fmt.Errorf("%w for %q (variant %q): available %d, requested %d",
				contracts.ErrSupplyUnavailable, req.Line.ListingSlug, req.Line.VariantHash,
				available, req.Line.Quantity)
		}
	}

	reservationID, err := nextInventoryReservationID(tx)
	if err != nil {
		return nil, err
	}
	row := models.InventoryReservation{
		ID:          reservationID,
		OrderRef:    req.OrderRef,
		OrderType:   req.OrderType,
		ListingSlug: req.Line.ListingSlug,
		VariantHash: req.Line.VariantHash,
		Quantity:    req.Line.Quantity,
		ReservedAt:  reservedAt,
		ExpiresAt:   req.ExpiresAt,
	}
	if err := tx.Save(&row); err != nil {
		return nil, fmt.Errorf("reserve inventory for %s: %w", req.Line.ListingSlug, err)
	}

	return skuReservationFromRow(row, req.Line), nil
}

func skuReservationFromRow(row models.InventoryReservation, line contracts.SupplyLine) *contracts.SupplyReservation {
	return &contracts.SupplyReservation{
		ID:          strconv.Itoa(row.ID),
		OrderRef:    row.OrderRef,
		OrderType:   row.OrderType,
		LineID:      line.LineID,
		SupplyKind:  contracts.SupplyKindSkuQuantity,
		ListingSlug: row.ListingSlug,
		VariantHash: row.VariantHash,
		VariantSKU:  line.VariantSKU,
		Quantity:    row.Quantity,
		Status:      contracts.SupplyReservationReserved,
		ExpiresAt:   row.ExpiresAt,
	}
}

func existingSkuReservationInTx(tx database.Tx, req contracts.ReserveSupplyRequest) (models.InventoryReservation, bool, error) {
	var row models.InventoryReservation
	err := tx.Read().
		Where("order_ref = ? AND order_type = ? AND listing_slug = ? AND variant_hash = ? AND released_at IS NULL",
			req.OrderRef, req.OrderType, req.Line.ListingSlug, req.Line.VariantHash).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.InventoryReservation{}, false, nil
	}
	if err != nil {
		return models.InventoryReservation{}, false, err
	}
	return row, true, nil
}

func releaseSkuQuantityInTx(tx database.Tx, req contracts.ReleaseSupplyRequest, releasedAt time.Time) error {
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL",
		req.OrderRef, req.OrderType).Find(&reservations).Error; err != nil {
		return err
	}
	for i := range reservations {
		if reservations[i].Confirmed {
			continue
		}
		reservations[i].ReleasedAt = &releasedAt
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func commitSkuQuantityInTx(tx database.Tx, req contracts.CommitSupplyRequest) error {
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL AND confirmed = ?",
		req.OrderRef, req.OrderType, false).Find(&reservations).Error; err != nil {
		return err
	}
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range reservations {
		reservations[i].Confirmed = true
		reservations[i].ExpiresAt = farFuture
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateSkuReserveRequest(req contracts.ReserveSupplyRequest) error {
	if req.OrderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return errors.New("supply availability: order type is required")
	}
	if req.ExpiresAt.IsZero() {
		return errors.New("supply availability: reservation expiry is required")
	}
	return validateSkuSupplyLine(req.Line)
}

func validateSkuSupplyLine(line contracts.SupplyLine) error {
	if line.SupplyKind != contracts.SupplyKindSkuQuantity {
		return contracts.ErrSupplyKindUnsupported
	}
	if line.ListingSlug == "" {
		return errors.New("supply availability: listing slug is required")
	}
	if line.Quantity <= 0 {
		return errors.New("supply availability: quantity must be positive")
	}
	if line.StockTracked && line.StockLimit < 0 {
		return errors.New("supply availability: stock limit cannot be negative")
	}
	return nil
}

func skuReservedQuantity(tx database.Tx, listingSlug, variantHash string) (int64, error) {
	var total int64
	err := tx.Read().Model(&models.InventoryReservation{}).
		Where("listing_slug = ? AND variant_hash = ? AND released_at IS NULL",
			listingSlug, variantHash).
		Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error
	return total, err
}

func nextInventoryReservationID(tx database.Tx) (int, error) {
	var maxID int
	err := tx.Read().Model(&models.InventoryReservation{}).
		Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error
	return maxID + 1, err
}

func baseSkuAvailabilityResult(line contracts.SupplyLine) *contracts.AvailabilityResult {
	return &contracts.AvailabilityResult{
		LineID:      line.LineID,
		SupplyKind:  contracts.SupplyKindSkuQuantity,
		ProviderID:  line.ProviderID,
		ProviderRef: line.ProviderRef,
	}
}

func applySkuStockAvailability(result *contracts.AvailabilityResult, line contracts.SupplyLine, reserved int64) {
	availableQuantity := line.StockLimit - reserved
	if availableQuantity < 0 {
		availableQuantity = 0
	}
	result.AvailableQuantity = availableQuantity
	result.Available = availableQuantity >= int64(line.Quantity)
	switch {
	case availableQuantity <= 0:
		result.Status = contracts.SupplyAvailabilityOutOfStock
	case result.Available:
		result.Status = contracts.SupplyAvailabilityAvailable
	default:
		result.Status = contracts.SupplyAvailabilityLowStock
	}
}

func checkedAt(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}
