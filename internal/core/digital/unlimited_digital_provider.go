package digital

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

// UnlimitedDigitalProvider represents non-scarce digital files and fixed links.
// It validates that delivery assets exist but never creates stock holds.
type UnlimitedDigitalProvider struct {
	db database.Database
}

func NewUnlimitedDigitalProvider(db database.Database) *UnlimitedDigitalProvider {
	return &UnlimitedDigitalProvider{db: db}
}

func (p *UnlimitedDigitalProvider) Kind() contracts.SupplyKind {
	return contracts.SupplyKindUnlimitedDigital
}

func (p *UnlimitedDigitalProvider) GetAvailability(ctx context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateUnlimitedDigitalSupplyLine(req.Line); err != nil {
		return nil, err
	}

	result := &contracts.AvailabilityResult{
		LineID:     req.Line.LineID,
		SupplyKind: contracts.SupplyKindUnlimitedDigital,
		CheckedAt:  licensePoolCheckedAt(req.CheckedAt),
	}
	configured, err := p.hasUnlimitedDigitalAsset(req.Line.ListingSlug, req.Line.VariantSKU)
	if err != nil {
		return nil, err
	}
	if !configured {
		result.Status = contracts.SupplyAvailabilityManualActionRequired
		result.ManualActionRequired = true
		result.Reason = "digital_asset_missing"
		return result, nil
	}
	result.Status = contracts.SupplyAvailabilityUnlimited
	result.Available = true
	result.Unlimited = true
	return result, nil
}

func (p *UnlimitedDigitalProvider) Reserve(ctx context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateUnlimitedDigitalReserveRequest(req); err != nil {
		return nil, err
	}
	configured, err := p.hasUnlimitedDigitalAsset(req.Line.ListingSlug, req.Line.VariantSKU)
	if err != nil {
		return nil, err
	}
	if !configured {
		return nil, fmt.Errorf("%w: digital asset missing for %q", contracts.ErrSupplyManualActionRequired, req.Line.ListingSlug)
	}
	return unlimitedDigitalReservation(req), nil
}

func (p *UnlimitedDigitalProvider) ReserveTx(ctx context.Context, tx database.Tx, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("unlimited digital: transaction is required")
	}
	if err := validateUnlimitedDigitalReserveRequest(req); err != nil {
		return nil, err
	}
	configured, err := hasUnlimitedDigitalAssetInTx(tx, req.Line.ListingSlug, req.Line.VariantSKU)
	if err != nil {
		return nil, err
	}
	if !configured {
		return nil, fmt.Errorf("%w: digital asset missing for %q", contracts.ErrSupplyManualActionRequired, req.Line.ListingSlug)
	}
	return unlimitedDigitalReservation(req), nil
}

func (p *UnlimitedDigitalProvider) Commit(ctx context.Context, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateUnlimitedDigitalOrderIdentity(req.OrderRef, req.OrderType)
}

func (p *UnlimitedDigitalProvider) CommitTx(ctx context.Context, tx database.Tx, req contracts.CommitSupplyRequest) error {
	if tx == nil {
		return errors.New("unlimited digital: transaction is required")
	}
	return p.Commit(ctx, req)
}

func (p *UnlimitedDigitalProvider) Release(ctx context.Context, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateUnlimitedDigitalOrderIdentity(req.OrderRef, req.OrderType)
}

func (p *UnlimitedDigitalProvider) ReleaseTx(ctx context.Context, tx database.Tx, req contracts.ReleaseSupplyRequest) error {
	if tx == nil {
		return errors.New("unlimited digital: transaction is required")
	}
	return p.Release(ctx, req)
}

func (p *UnlimitedDigitalProvider) hasUnlimitedDigitalAsset(listingSlug string, variantSKU string) (bool, error) {
	var count int64
	err := p.db.View(func(tx database.Tx) error {
		var err error
		count, err = unlimitedDigitalAssetCountInTx(tx, listingSlug, variantSKU)
		return err
	})
	return count > 0, err
}

func hasUnlimitedDigitalAssetInTx(tx database.Tx, listingSlug string, variantSKU string) (bool, error) {
	count, err := unlimitedDigitalAssetCountInTx(tx, listingSlug, variantSKU)
	return count > 0, err
}

func unlimitedDigitalAssetCountInTx(tx database.Tx, listingSlug string, variantSKU string) (int64, error) {
	var count int64
	q := tx.Read().Model(&models.DigitalAsset{}).
		Where("listing_slug = ? AND asset_type IN (?, ?)",
			listingSlug, models.AssetTypeFile, models.AssetTypeLink)
	if variantSKU != "" {
		q = q.Where("variant_sku IN (?, '')", variantSKU)
	} else {
		q = q.Where("variant_sku = ?", "")
	}
	return count, q.Count(&count).Error
}

func unlimitedDigitalReservation(req contracts.ReserveSupplyRequest) *contracts.SupplyReservation {
	return &contracts.SupplyReservation{
		ID:          fmt.Sprintf("noop:%s:%s:%s", req.OrderRef, req.Line.ListingSlug, req.Line.VariantSKU),
		OrderRef:    req.OrderRef,
		OrderType:   req.OrderType,
		LineID:      req.Line.LineID,
		SupplyKind:  contracts.SupplyKindUnlimitedDigital,
		ListingSlug: req.Line.ListingSlug,
		VariantSKU:  req.Line.VariantSKU,
		Quantity:    req.Line.Quantity,
		Status:      contracts.SupplyReservationNoop,
		ExpiresAt:   req.ExpiresAt,
	}
}

func validateUnlimitedDigitalReserveRequest(req contracts.ReserveSupplyRequest) error {
	if err := validateUnlimitedDigitalOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	if req.ExpiresAt.IsZero() {
		return errors.New("unlimited digital: reservation expiry is required")
	}
	return validateUnlimitedDigitalSupplyLine(req.Line)
}

func validateUnlimitedDigitalSupplyLine(line contracts.SupplyLine) error {
	if line.SupplyKind != contracts.SupplyKindUnlimitedDigital {
		return contracts.ErrSupplyKindUnsupported
	}
	if strings.TrimSpace(line.ListingSlug) == "" {
		return errors.New("unlimited digital: listing slug is required")
	}
	if line.Quantity <= 0 {
		return errors.New("unlimited digital: quantity must be positive")
	}
	return nil
}

func validateUnlimitedDigitalOrderIdentity(orderRef string, orderType string) error {
	if strings.TrimSpace(orderRef) == "" {
		return errors.New("unlimited digital: order ref is required")
	}
	if strings.TrimSpace(orderType) == "" {
		return errors.New("unlimited digital: order type is required")
	}
	return nil
}

var _ contracts.SupplyProvider = (*UnlimitedDigitalProvider)(nil)
