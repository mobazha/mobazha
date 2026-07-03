package guest

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/redact"
)

// DistributionManagedEscrowWatcher adapts Core guest-order models to the
// chain-neutral watch registrar implemented by a trusted distribution.
type DistributionManagedEscrowWatcher struct {
	registrar distribution.ManagedEscrowWatchRegistrar
	projector distribution.ManagedEscrowGuestProjector
}

// NewDistributionManagedEscrowWatcher constructs the guest observation bridge.
func NewDistributionManagedEscrowWatcher(
	registrar distribution.ManagedEscrowWatchRegistrar,
	projector distribution.ManagedEscrowGuestProjector,
) *DistributionManagedEscrowWatcher {
	return &DistributionManagedEscrowWatcher{registrar: registrar, projector: projector}
}

// RegisterWatch validates and projects the order before crossing the module boundary.
func (w *DistributionManagedEscrowWatcher) RegisterWatch(ctx context.Context, order *models.GuestOrder) error {
	if w == nil || w.registrar == nil || w.projector == nil {
		return fmt.Errorf("guest managed escrow watch: registrar and projector are required")
	}
	projection, err := ManagedEscrowGuestProjection(order)
	if err != nil {
		return err
	}
	watch, err := w.projector.ProjectManagedEscrowGuestWatch(ctx, projection)
	if err != nil {
		return err
	}
	if err := w.registrar.RegisterManagedEscrowWatch(ctx, watch); err != nil {
		return fmt.Errorf("guest managed escrow watch: register %s: %w", order.OrderToken, err)
	}
	return nil
}

// StopWatch removes an order from the private monitor runtime.
func (w *DistributionManagedEscrowWatcher) StopWatch(orderToken string) {
	if w == nil || w.registrar == nil || strings.TrimSpace(orderToken) == "" {
		return
	}
	if err := w.registrar.StopManagedEscrowWatch(orderToken); err != nil {
		log.Warningf("guest managed escrow watch: stop %s: %v", redact.Token(orderToken), err)
	}
}
