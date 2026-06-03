package digital

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// LicenseKeyPoolProvider controls scarce license-key availability. It reserves
// concrete keys but leaves buyer-visible grants to the digital entitlement layer.
type LicenseKeyPoolProvider struct {
	db database.Database
}

var errLicenseKeyAlreadyClaimed = errors.New("license key already claimed")

func NewLicenseKeyPoolProvider(db database.Database) *LicenseKeyPoolProvider {
	return &LicenseKeyPoolProvider{db: db}
}

func (p *LicenseKeyPoolProvider) Kind() contracts.SupplyKind {
	return contracts.SupplyKindLicenseKeyPool
}

func (p *LicenseKeyPoolProvider) GetAvailability(ctx context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateLicenseKeySupplyLine(req.Line); err != nil {
		return nil, err
	}

	result := &contracts.AvailabilityResult{
		LineID:     req.Line.LineID,
		SupplyKind: contracts.SupplyKindLicenseKeyPool,
		CheckedAt:  licensePoolCheckedAt(req.CheckedAt),
	}
	err := p.db.View(func(tx database.Tx) error {
		available, err := availableLicenseKeyCountInTx(tx, req.Line.ListingSlug, req.Line.VariantSKU)
		if err != nil {
			return err
		}
		result.AvailableQuantity = available
		if available >= int64(req.Line.Quantity) {
			result.Available = true
			result.Status = contracts.SupplyAvailabilityAvailable
			return nil
		}
		result.Available = false
		result.Status = contracts.SupplyAvailabilityOutOfStock
		result.Reason = "license_key_pool_exhausted"
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *LicenseKeyPoolProvider) Reserve(ctx context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateLicenseKeyReserveRequest(req); err != nil {
		return nil, err
	}

	var reservation contracts.SupplyReservation
	err := p.db.Update(func(tx database.Tx) error {
		res, err := reserveLicenseKeysInTx(tx, req)
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

func (p *LicenseKeyPoolProvider) ReserveTx(ctx context.Context, tx database.Tx, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("license key pool: transaction is required")
	}
	if err := validateLicenseKeyReserveRequest(req); err != nil {
		return nil, err
	}
	return reserveLicenseKeysInTx(tx, req)
}

func (p *LicenseKeyPoolProvider) Commit(ctx context.Context, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateLicenseKeyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	return p.db.Update(func(tx database.Tx) error {
		return commitLicenseKeysInTx(tx, req.OrderRef, req.OrderType, time.Now())
	})
}

func (p *LicenseKeyPoolProvider) CommitTx(ctx context.Context, tx database.Tx, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("license key pool: transaction is required")
	}
	if err := validateLicenseKeyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	return commitLicenseKeysInTx(tx, req.OrderRef, req.OrderType, time.Now())
}

func (p *LicenseKeyPoolProvider) Release(ctx context.Context, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateLicenseKeyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	return p.db.Update(func(tx database.Tx) error {
		return releaseLicenseKeysInTx(tx, req.OrderRef, req.OrderType)
	})
}

func (p *LicenseKeyPoolProvider) ReleaseTx(ctx context.Context, tx database.Tx, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("license key pool: transaction is required")
	}
	if err := validateLicenseKeyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	return releaseLicenseKeysInTx(tx, req.OrderRef, req.OrderType)
}

func reserveLicenseKeysInTx(tx database.Tx, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	keys, err := licenseKeysForOrderInTx(tx, req.OrderRef, req.OrderType, req.Line.ListingSlug, req.Line.VariantSKU)
	if err != nil {
		return nil, err
	}
	needed := req.Line.Quantity - len(keys)
	newlyReserved := make([]models.DigitalLicenseKey, 0, needed)
	for needed > 0 {
		key, err := reserveOneLicenseKeyWithRetryInTx(tx, req)
		if err != nil {
			releaseErr := releaseLicenseKeyRowsInTx(tx, newlyReserved)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.Join(
					fmt.Errorf("%w for %q: available 0, requested %d",
						contracts.ErrSupplyUnavailable, req.Line.ListingSlug, needed),
					releaseErr,
				)
			}
			return nil, errors.Join(err, releaseErr)
		}
		keys = append(keys, *key)
		newlyReserved = append(newlyReserved, *key)
		needed--
	}
	return licenseKeyReservationFromRows(req, keys), nil
}

func reserveOneLicenseKeyWithRetryInTx(tx database.Tx, req contracts.ReserveSupplyRequest) (*models.DigitalLicenseKey, error) {
	const maxReserveRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxReserveRetries; attempt++ {
		key, err := reserveOneLicenseKeyInTx(tx, req)
		if err == nil {
			return key, nil
		}
		lastErr = err
		if errors.Is(err, errLicenseKeyAlreadyClaimed) {
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

func reserveOneLicenseKeyInTx(tx database.Tx, req contracts.ReserveSupplyRequest) (*models.DigitalLicenseKey, error) {
	var candidate models.DigitalLicenseKey
	if err := tx.Read().
		Where("listing_slug = ? AND variant_sku = ? AND status = ?",
			req.Line.ListingSlug, req.Line.VariantSKU, models.LicenseKeyStatusAvailable).
		Order("id ASC").
		First(&candidate).Error; err != nil {
		return nil, err
	}

	rows, err := tx.UpdateColumns(
		map[string]interface{}{
			"status":        models.LicenseKeyStatusReserved,
			"order_id":      req.OrderRef,
			"order_type":    req.OrderType,
			"buyer_peer_id": req.BuyerPeerID,
		},
		map[string]interface{}{
			"id = ?":     candidate.ID,
			"status = ?": models.LicenseKeyStatusAvailable,
		},
		&models.DigitalLicenseKey{},
	)
	if err != nil {
		return nil, fmt.Errorf("reserve license key %s: %w", candidate.ID, err)
	}
	if rows == 0 {
		return nil, fmt.Errorf("%w: %s", errLicenseKeyAlreadyClaimed, candidate.ID)
	}

	var reserved models.DigitalLicenseKey
	if err := tx.Read().Where("id = ?", candidate.ID).First(&reserved).Error; err != nil {
		return nil, fmt.Errorf("reload reserved license key: %w", err)
	}
	return &reserved, nil
}

func licenseKeysForOrderInTx(tx database.Tx, orderRef string, orderType string, listingSlug string, variantSKU string) ([]models.DigitalLicenseKey, error) {
	var keys []models.DigitalLicenseKey
	if err := tx.Read().
		Where("order_id = ? AND order_type = ? AND listing_slug = ? AND variant_sku = ? AND status IN (?, ?)",
			orderRef, orderType, listingSlug, variantSKU,
			models.LicenseKeyStatusReserved,
			models.LicenseKeyStatusDispensed).
		Order("id ASC").
		Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func licenseKeyReservationFromRows(req contracts.ReserveSupplyRequest, keys []models.DigitalLicenseKey) *contracts.SupplyReservation {
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.ID)
	}
	return &contracts.SupplyReservation{
		ID:             strings.Join(ids, ","),
		OrderRef:       req.OrderRef,
		OrderType:      req.OrderType,
		LineID:         req.Line.LineID,
		SupplyKind:     contracts.SupplyKindLicenseKeyPool,
		ListingSlug:    req.Line.ListingSlug,
		VariantSKU:     req.Line.VariantSKU,
		Quantity:       len(keys),
		ReservationRef: strings.Join(ids, ","),
		Status:         contracts.SupplyReservationReserved,
		ExpiresAt:      req.ExpiresAt,
	}
}

func commitLicenseKeysInTx(tx database.Tx, orderRef string, orderType string, committedAt time.Time) error {
	var keys []models.DigitalLicenseKey
	if err := tx.Read().
		Where("order_id = ? AND order_type = ? AND status = ?", orderRef, orderType, models.LicenseKeyStatusReserved).
		Find(&keys).Error; err != nil {
		return err
	}
	for i := range keys {
		keys[i].Status = models.LicenseKeyStatusDispensed
		keys[i].DispensedAt = committedAt
		if err := tx.Save(&keys[i]); err != nil {
			return err
		}
	}
	return nil
}

func releaseLicenseKeysInTx(tx database.Tx, orderRef string, orderType string) error {
	var keys []models.DigitalLicenseKey
	if err := tx.Read().
		Where("order_id = ? AND order_type = ? AND status = ?", orderRef, orderType, models.LicenseKeyStatusReserved).
		Find(&keys).Error; err != nil {
		return err
	}
	return releaseLicenseKeyRowsInTx(tx, keys)
}

func releaseLicenseKeyRowsInTx(tx database.Tx, keys []models.DigitalLicenseKey) error {
	for i := range keys {
		if keys[i].Status != models.LicenseKeyStatusReserved {
			continue
		}
		keys[i].Status = models.LicenseKeyStatusAvailable
		keys[i].OrderID = ""
		keys[i].OrderType = ""
		keys[i].BuyerPeerID = ""
		if err := tx.Save(&keys[i]); err != nil {
			return err
		}
	}
	return nil
}

func availableLicenseKeyCountInTx(tx database.Tx, listingSlug string, variantSKU string) (int64, error) {
	var count int64
	err := tx.Read().Model(&models.DigitalLicenseKey{}).
		Where("listing_slug = ? AND variant_sku = ? AND status = ?",
			listingSlug, variantSKU, models.LicenseKeyStatusAvailable).
		Count(&count).Error
	return count, err
}

func validateLicenseKeyReserveRequest(req contracts.ReserveSupplyRequest) error {
	if err := validateLicenseKeyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	if req.ExpiresAt.IsZero() {
		return errors.New("license key pool: reservation expiry is required")
	}
	return validateLicenseKeySupplyLine(req.Line)
}

func validateLicenseKeyOrderRef(orderRef string) error {
	if strings.TrimSpace(orderRef) == "" {
		return errors.New("license key pool: order ref is required")
	}
	return nil
}

func validateLicenseKeyOrderIdentity(orderRef string, orderType string) error {
	if err := validateLicenseKeyOrderRef(orderRef); err != nil {
		return err
	}
	if strings.TrimSpace(orderType) == "" {
		return errors.New("license key pool: order type is required")
	}
	return nil
}

func validateLicenseKeySupplyLine(line contracts.SupplyLine) error {
	if line.SupplyKind != contracts.SupplyKindLicenseKeyPool {
		return contracts.ErrSupplyKindUnsupported
	}
	if strings.TrimSpace(line.ListingSlug) == "" {
		return errors.New("license key pool: listing slug is required")
	}
	if line.Quantity <= 0 {
		return errors.New("license key pool: quantity must be positive")
	}
	if line.Quantity > 10000 {
		return fmt.Errorf("license key pool: quantity %s is too large", strconv.Itoa(line.Quantity))
	}
	return nil
}

func licensePoolCheckedAt(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

var _ contracts.SupplyProvider = (*LicenseKeyPoolProvider)(nil)
