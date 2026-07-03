package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

// ExternalSupplyProvider checks supplier-managed listings without pretending
// the current fulfillment APIs can hold external stock.
type ExternalSupplyProvider struct {
	db       database.Database
	registry contracts.FulfillmentProviderRegistry
}

func NewExternalSupplyProvider(db database.Database, registry contracts.FulfillmentProviderRegistry) *ExternalSupplyProvider {
	return &ExternalSupplyProvider{db: db, registry: registry}
}

func (p *ExternalSupplyProvider) Kind() contracts.SupplyKind {
	return contracts.SupplyKindExternalSupply
}

func (p *ExternalSupplyProvider) GetAvailability(ctx context.Context, req contracts.AvailabilityRequest) (*contracts.AvailabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateExternalSupplyLine(req.Line); err != nil {
		return nil, err
	}

	result := &contracts.AvailabilityResult{
		LineID:      req.Line.LineID,
		SupplyKind:  contracts.SupplyKindExternalSupply,
		ProviderID:  req.Line.ProviderID,
		ProviderRef: req.Line.ProviderRef,
		CheckedAt:   externalSupplyCheckedAt(req.CheckedAt),
	}

	var mapping models.SyncedProductMapping
	err := p.db.View(func(tx database.Tx) error {
		q := tx.Read().Where("listing_slug = ?", req.Line.ListingSlug)
		if req.Line.ProviderID != "" {
			q = q.Where("provider_id = ?", req.Line.ProviderID)
		}
		if req.Line.ProviderRef != "" {
			q = q.Where("(sync_product_id = ? OR external_id = ? OR id = ?)",
				req.Line.ProviderRef, req.Line.ProviderRef, req.Line.ProviderRef)
		}
		return q.Order("last_sync_at DESC").First(&mapping).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		result.Status = contracts.SupplyAvailabilityManualActionRequired
		result.ManualActionRequired = true
		result.Reason = "external_mapping_missing"
		return result, nil
	}
	if err != nil {
		return nil, err
	}

	result.ProviderID = mapping.ProviderID
	result.ProviderRef = externalSupplyProviderRef(mapping)

	switch strings.TrimSpace(mapping.Status) {
	case "", "synced":
		if p.registry == nil {
			result.Status = contracts.SupplyAvailabilitySupplierUnavailable
			result.Reason = "supplier_registry_unavailable"
			return result, nil
		}
		if _, err := p.registry.ForProvider(mapping.ProviderID); err != nil {
			result.Status = contracts.SupplyAvailabilitySupplierUnavailable
			result.Reason = "supplier_provider_unavailable"
			return result, nil
		}
		result.Available = true
		result.Status = contracts.SupplyAvailabilityAvailable
		result.AvailableQuantity = int64(req.Line.Quantity)
		return result, nil
	default:
		result.Status = contracts.SupplyAvailabilitySupplierUnavailable
		result.Reason = "supplier_mapping_" + mapping.Status
		return result, nil
	}
}

func (p *ExternalSupplyProvider) Reserve(ctx context.Context, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateExternalSupplyReserveRequest(req); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: external provider hold is not supported", contracts.ErrSupplyManualActionRequired)
}

func (p *ExternalSupplyProvider) ReserveTx(ctx context.Context, tx database.Tx, req contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("external supply: transaction is required")
	}
	if err := validateExternalSupplyReserveRequest(req); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: external provider hold is not supported", contracts.ErrSupplyManualActionRequired)
}

func (p *ExternalSupplyProvider) Commit(ctx context.Context, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateExternalSupplyOrderIdentity(req.OrderRef, req.OrderType)
}

func (p *ExternalSupplyProvider) CommitTx(ctx context.Context, tx database.Tx, req contracts.CommitSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("external supply: transaction is required")
	}
	return validateExternalSupplyOrderIdentity(req.OrderRef, req.OrderType)
}

func (p *ExternalSupplyProvider) Release(ctx context.Context, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateExternalSupplyOrderIdentity(req.OrderRef, req.OrderType)
}

func (p *ExternalSupplyProvider) ReleaseTx(ctx context.Context, tx database.Tx, req contracts.ReleaseSupplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("external supply: transaction is required")
	}
	return validateExternalSupplyOrderIdentity(req.OrderRef, req.OrderType)
}

func validateExternalSupplyReserveRequest(req contracts.ReserveSupplyRequest) error {
	if err := validateExternalSupplyOrderIdentity(req.OrderRef, req.OrderType); err != nil {
		return err
	}
	if req.ExpiresAt.IsZero() {
		return errors.New("external supply: reservation expiry is required")
	}
	return validateExternalSupplyLine(req.Line)
}

func validateExternalSupplyOrderIdentity(orderRef string, orderType string) error {
	if strings.TrimSpace(orderRef) == "" {
		return errors.New("external supply: order ref is required")
	}
	if strings.TrimSpace(orderType) == "" {
		return errors.New("external supply: order type is required")
	}
	return nil
}

func validateExternalSupplyLine(line contracts.SupplyLine) error {
	if line.SupplyKind != contracts.SupplyKindExternalSupply {
		return contracts.ErrSupplyKindUnsupported
	}
	if strings.TrimSpace(line.ListingSlug) == "" {
		return errors.New("external supply: listing slug is required")
	}
	if line.Quantity <= 0 {
		return errors.New("external supply: quantity must be positive")
	}
	return nil
}

func externalSupplyProviderRef(mapping models.SyncedProductMapping) string {
	if mapping.SyncProductID != "" {
		return mapping.SyncProductID
	}
	if mapping.ExternalID != "" {
		return mapping.ExternalID
	}
	return mapping.ID
}

func externalSupplyCheckedAt(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

var _ contracts.SupplyProvider = (*ExternalSupplyProvider)(nil)
var _ transactionalSupplyProvider = (*ExternalSupplyProvider)(nil)
