package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
)

// SupplyAvailabilityAppService is the order-facing aggregate boundary for
// provider-neutral supply quote, reserve, commit, and release operations.
type SupplyAvailabilityAppService struct {
	providers map[contracts.SupplyKind]contracts.SupplyProvider
}

func NewSupplyAvailabilityAppService(providers ...contracts.SupplyProvider) (*SupplyAvailabilityAppService, error) {
	s := &SupplyAvailabilityAppService{
		providers: make(map[contracts.SupplyKind]contracts.SupplyProvider, len(providers)),
	}
	for _, provider := range providers {
		if err := s.RegisterProvider(provider); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *SupplyAvailabilityAppService) RegisterProvider(provider contracts.SupplyProvider) error {
	if provider == nil {
		return errors.New("supply availability: provider is nil")
	}
	kind := provider.Kind()
	if !kind.IsValid() {
		return fmt.Errorf("%w: %s", contracts.ErrSupplyKindUnsupported, kind)
	}
	if _, exists := s.providers[kind]; exists {
		return fmt.Errorf("supply availability: duplicate provider for %s", kind)
	}
	s.providers[kind] = provider
	return nil
}

func (s *SupplyAvailabilityAppService) Quote(ctx context.Context, req contracts.SupplyQuoteRequest) (*contracts.SupplyQuoteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lines, err := aggregateSupplyLines(req.Lines)
	if err != nil {
		return nil, err
	}

	result := &contracts.SupplyQuoteResult{
		Results: make([]contracts.AvailabilityResult, 0, len(lines)),
		CanSell: true,
	}
	for _, line := range lines {
		provider, err := s.providerFor(line.SupplyKind)
		if err != nil {
			return nil, err
		}
		availability, err := provider.GetAvailability(ctx, contracts.AvailabilityRequest{
			Line:        line,
			BuyerPeerID: req.BuyerPeerID,
			CheckedAt:   time.Now(),
		})
		if err != nil {
			return nil, err
		}
		if availability == nil {
			return nil, errors.New("supply availability: provider returned nil availability")
		}
		result.Results = append(result.Results, *availability)
		if availability.ManualActionRequired || availability.Status == contracts.SupplyAvailabilityManualActionRequired {
			result.ManualActionRequired = true
		}
		if !availability.Available || availability.ManualActionRequired {
			result.CanSell = false
		}
	}
	if result.ManualActionRequired {
		result.Reason = "manual_action_required"
	} else if !result.CanSell {
		result.Reason = "supply_unavailable"
	}
	return result, nil
}

func (s *SupplyAvailabilityAppService) ReserveOrder(ctx context.Context, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.OrderRef == "" {
		return nil, errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return nil, errors.New("supply availability: order type is required")
	}
	if req.ExpiresAt.IsZero() {
		return nil, errors.New("supply availability: reservation expiry is required")
	}
	lines, err := aggregateSupplyLines(req.Lines)
	if err != nil {
		return nil, err
	}

	result := &contracts.ReserveOrderSupplyResult{
		Reservations: make([]contracts.SupplyReservation, 0, len(lines)),
	}
	reservedKinds := make(map[contracts.SupplyKind]struct{})
	for _, line := range lines {
		provider, err := s.providerFor(line.SupplyKind)
		if err != nil {
			return nil, err
		}
		reservation, err := provider.Reserve(ctx, contracts.ReserveSupplyRequest{
			OrderRef:    req.OrderRef,
			OrderType:   req.OrderType,
			Line:        line,
			BuyerPeerID: req.BuyerPeerID,
			ExpiresAt:   req.ExpiresAt,
		})
		if err != nil {
			releaseErr := s.releaseReservedKinds(ctx, req.OrderRef, req.OrderType, "reserve_failed", reservedKinds)
			return nil, errors.Join(fmt.Errorf("reserve supply line %q: %w", line.LineID, err), releaseErr)
		}
		if reservation == nil {
			releaseErr := s.releaseReservedKinds(ctx, req.OrderRef, req.OrderType, "reserve_failed", reservedKinds)
			return nil, errors.Join(errors.New("supply availability: provider returned nil reservation"), releaseErr)
		}
		result.Reservations = append(result.Reservations, *reservation)
		reservedKinds[line.SupplyKind] = struct{}{}
		if reservation.Status == contracts.SupplyReservationFailed {
			result.Reason = "reservation_failed"
		}
	}
	return result, nil
}

func (s *SupplyAvailabilityAppService) ReserveOrderTx(ctx context.Context, tx database.Tx, req contracts.ReserveOrderSupplyRequest) (*contracts.ReserveOrderSupplyResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("supply availability: transaction is required")
	}
	if req.OrderRef == "" {
		return nil, errors.New("supply availability: order ref is required")
	}
	if req.OrderType == "" {
		return nil, errors.New("supply availability: order type is required")
	}
	if req.ExpiresAt.IsZero() {
		return nil, errors.New("supply availability: reservation expiry is required")
	}
	lines, err := aggregateSupplyLines(req.Lines)
	if err != nil {
		return nil, err
	}

	result := &contracts.ReserveOrderSupplyResult{
		Reservations: make([]contracts.SupplyReservation, 0, len(lines)),
	}
	reservedKinds := make(map[contracts.SupplyKind]struct{})
	for _, line := range lines {
		provider, err := s.transactionalProviderFor(line.SupplyKind)
		if err != nil {
			releaseErr := s.releaseReservedKindsTx(ctx, tx, req.OrderRef, req.OrderType, "reserve_failed", reservedKinds)
			return nil, errors.Join(err, releaseErr)
		}
		reservation, err := provider.ReserveTx(ctx, tx, contracts.ReserveSupplyRequest{
			OrderRef:    req.OrderRef,
			OrderType:   req.OrderType,
			Line:        line,
			BuyerPeerID: req.BuyerPeerID,
			ExpiresAt:   req.ExpiresAt,
		})
		if err != nil {
			releaseErr := s.releaseReservedKindsTx(ctx, tx, req.OrderRef, req.OrderType, "reserve_failed", reservedKinds)
			return nil, errors.Join(fmt.Errorf("reserve supply line %q: %w", line.LineID, err), releaseErr)
		}
		if reservation == nil {
			releaseErr := s.releaseReservedKindsTx(ctx, tx, req.OrderRef, req.OrderType, "reserve_failed", reservedKinds)
			return nil, errors.Join(errors.New("supply availability: provider returned nil reservation"), releaseErr)
		}
		result.Reservations = append(result.Reservations, *reservation)
		reservedKinds[line.SupplyKind] = struct{}{}
		if reservation.Status == contracts.SupplyReservationFailed {
			result.Reason = "reservation_failed"
		}
	}
	return result, nil
}

func (s *SupplyAvailabilityAppService) CommitOrder(ctx context.Context, orderRef string, orderType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if orderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if orderType == "" {
		return errors.New("supply availability: order type is required")
	}

	var errs []error
	for _, provider := range s.providersInOrder() {
		if err := provider.Commit(ctx, contracts.CommitSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SupplyAvailabilityAppService) CommitOrderTx(ctx context.Context, tx database.Tx, orderRef string, orderType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("supply availability: transaction is required")
	}
	if orderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if orderType == "" {
		return errors.New("supply availability: order type is required")
	}

	var errs []error
	for _, kind := range s.providerKindsInOrder() {
		provider, err := s.transactionalProviderFor(kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := provider.CommitTx(ctx, tx, contracts.CommitSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SupplyAvailabilityAppService) ReleaseOrder(ctx context.Context, orderRef string, orderType string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if orderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if orderType == "" {
		return errors.New("supply availability: order type is required")
	}

	var errs []error
	for _, provider := range s.providersInOrder() {
		if err := provider.Release(ctx, contracts.ReleaseSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
			Reason:    reason,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SupplyAvailabilityAppService) ReleaseOrderTx(ctx context.Context, tx database.Tx, orderRef string, orderType string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("supply availability: transaction is required")
	}
	if orderRef == "" {
		return errors.New("supply availability: order ref is required")
	}
	if orderType == "" {
		return errors.New("supply availability: order type is required")
	}

	var errs []error
	for _, kind := range s.providerKindsInOrder() {
		provider, err := s.transactionalProviderFor(kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := provider.ReleaseTx(ctx, tx, contracts.ReleaseSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
			Reason:    reason,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type transactionalSupplyProvider interface {
	contracts.SupplyProvider
	ReserveTx(context.Context, database.Tx, contracts.ReserveSupplyRequest) (*contracts.SupplyReservation, error)
	CommitTx(context.Context, database.Tx, contracts.CommitSupplyRequest) error
	ReleaseTx(context.Context, database.Tx, contracts.ReleaseSupplyRequest) error
}

func (s *SupplyAvailabilityAppService) providerFor(kind contracts.SupplyKind) (contracts.SupplyProvider, error) {
	if !kind.IsValid() {
		return nil, fmt.Errorf("%w: %s", contracts.ErrSupplyKindUnsupported, kind)
	}
	provider, ok := s.providers[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", contracts.ErrSupplyKindUnsupported, kind)
	}
	return provider, nil
}

func (s *SupplyAvailabilityAppService) transactionalProviderFor(kind contracts.SupplyKind) (transactionalSupplyProvider, error) {
	provider, err := s.providerFor(kind)
	if err != nil {
		return nil, err
	}
	txProvider, ok := provider.(transactionalSupplyProvider)
	if !ok {
		return nil, fmt.Errorf("supply availability: provider %s does not support transactional reserve", kind)
	}
	return txProvider, nil
}

func (s *SupplyAvailabilityAppService) releaseReservedKinds(ctx context.Context, orderRef string, orderType string, reason string, kinds map[contracts.SupplyKind]struct{}) error {
	var errs []error
	for _, kind := range sortedSupplyKinds(kinds) {
		provider, ok := s.providers[kind]
		if !ok {
			continue
		}
		if err := provider.Release(ctx, contracts.ReleaseSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
			Reason:    reason,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SupplyAvailabilityAppService) releaseReservedKindsTx(ctx context.Context, tx database.Tx, orderRef string, orderType string, reason string, kinds map[contracts.SupplyKind]struct{}) error {
	var errs []error
	for _, kind := range sortedSupplyKinds(kinds) {
		provider, err := s.transactionalProviderFor(kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := provider.ReleaseTx(ctx, tx, contracts.ReleaseSupplyRequest{
			OrderRef:  orderRef,
			OrderType: orderType,
			Reason:    reason,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SupplyAvailabilityAppService) providersInOrder() []contracts.SupplyProvider {
	orderedKinds := s.providerKindsInOrder()
	providers := make([]contracts.SupplyProvider, 0, len(orderedKinds))
	for _, kind := range orderedKinds {
		providers = append(providers, s.providers[kind])
	}
	return providers
}

func (s *SupplyAvailabilityAppService) providerKindsInOrder() []contracts.SupplyKind {
	kinds := make(map[contracts.SupplyKind]struct{}, len(s.providers))
	for kind := range s.providers {
		kinds[kind] = struct{}{}
	}
	return sortedSupplyKinds(kinds)
}

func sortedSupplyKinds(kinds map[contracts.SupplyKind]struct{}) []contracts.SupplyKind {
	ordered := make([]contracts.SupplyKind, 0, len(kinds))
	for kind := range kinds {
		ordered = append(ordered, kind)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i] < ordered[j]
	})
	return ordered
}

type supplyLineBucketKey struct {
	supplyKind   contracts.SupplyKind
	listingSlug  string
	variantHash  string
	variantSKU   string
	stockTracked bool
	stockLimit   int64
	providerID   string
	providerRef  string
}

func aggregateSupplyLines(lines []contracts.SupplyLine) ([]contracts.SupplyLine, error) {
	if len(lines) == 0 {
		return nil, errors.New("supply availability: at least one line is required")
	}

	byBucket := make(map[supplyLineBucketKey]int, len(lines))
	aggregated := make([]contracts.SupplyLine, 0, len(lines))
	for _, line := range lines {
		if line.Quantity <= 0 {
			return nil, errors.New("supply availability: quantity must be positive")
		}
		if !line.SupplyKind.IsValid() {
			return nil, fmt.Errorf("%w: %s", contracts.ErrSupplyKindUnsupported, line.SupplyKind)
		}
		if strings.TrimSpace(line.ListingSlug) == "" {
			return nil, errors.New("supply availability: listing slug is required")
		}
		key := supplyLineBucketKey{
			supplyKind:   line.SupplyKind,
			listingSlug:  line.ListingSlug,
			variantHash:  line.VariantHash,
			variantSKU:   line.VariantSKU,
			stockTracked: line.StockTracked,
			stockLimit:   line.StockLimit,
			providerID:   line.ProviderID,
			providerRef:  line.ProviderRef,
		}
		idx, exists := byBucket[key]
		if !exists {
			byBucket[key] = len(aggregated)
			aggregated = append(aggregated, line)
			continue
		}
		aggregated[idx].Quantity += line.Quantity
		if aggregated[idx].LineID == "" {
			aggregated[idx].LineID = line.LineID
		}
	}
	return aggregated, nil
}

var _ contracts.SupplyAvailabilityService = (*SupplyAvailabilityAppService)(nil)
