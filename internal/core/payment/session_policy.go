package payment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/extensions"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

// SessionProvisioningPolicy is a business-policy boundary that runs before a
// payment provider or crypto funding target is created. Policies must be
// idempotent because CreateSession itself is idempotent and may be retried.
type SessionProvisioningPolicy interface {
	AuthorizeSessionProvisioning(context.Context, SessionProvisioningPolicyInput) error
}

// SessionProvisioningPolicyInput contains the signed order data needed to
// authorize creation of a new payment funding target.
type SessionProvisioningPolicyInput struct {
	OrderID                 string
	PaymentCoin             string
	PaymentSelectionQuoteID string
	SettlementMethod        porderpb.PaymentSent_Method
	SettlementMethodKnown   bool
	ExpiresAt               time.Time
	OrderOpen               *porderpb.OrderOpen
}

// OrderExtensionResolver projects signed order input into a required extension envelope.
type OrderExtensionResolver func(SessionProvisioningPolicyInput) (extensions.OrderExtension, bool, error)

// OrderExtensionReserveFunc invokes a provider's fail-closed reservation capability.
type OrderExtensionReserveFunc func(context.Context, extensions.ReservationRequest) (extensions.Reservation, error)

// OrderExtensionsResolver returns every persisted extension whose funding
// policy must be evaluated before a target is created.
type OrderExtensionsResolver func(SessionProvisioningPolicyInput) ([]extensions.OrderExtension, error)

// OrderExtensionReservationRecorder durably binds a provider reservation to
// the order before Core creates a funding target.
type OrderExtensionReservationRecorder func(extensions.ReservationRequest, extensions.Reservation) error

type orderExtensionProvisioningPolicy struct {
	resolve     OrderExtensionResolver
	resolveMany OrderExtensionsResolver
	reserve     OrderExtensionReserveFunc
	record      OrderExtensionReservationRecorder
}

// NewOrderExtensionProvisioningPolicy creates a product-neutral reservation
// gate for payment target creation.
func NewOrderExtensionProvisioningPolicy(resolve OrderExtensionResolver, reserve OrderExtensionReserveFunc, recorder ...OrderExtensionReservationRecorder) SessionProvisioningPolicy {
	policy := &orderExtensionProvisioningPolicy{resolve: resolve, reserve: reserve}
	if len(recorder) > 0 {
		policy.record = recorder[0]
	}
	return policy
}

// NewOrderExtensionsProvisioningPolicy creates the generic multi-extension
// reservation gate used by node composition.
func NewOrderExtensionsProvisioningPolicy(resolve OrderExtensionsResolver, reserve OrderExtensionReserveFunc, record OrderExtensionReservationRecorder) SessionProvisioningPolicy {
	return &orderExtensionProvisioningPolicy{resolveMany: resolve, reserve: reserve, record: record}
}

func (p *orderExtensionProvisioningPolicy) AuthorizeSessionProvisioning(ctx context.Context, input SessionProvisioningPolicyInput) error {
	if p == nil || (p.resolve == nil && p.resolveMany == nil) {
		return nil
	}
	var required []extensions.OrderExtension
	if p.resolveMany != nil {
		var err error
		required, err = p.resolveMany(input)
		if err != nil {
			return err
		}
	} else {
		extension, needed, err := p.resolve(input)
		if err != nil {
			return err
		}
		if needed {
			required = append(required, extension)
		}
	}
	for _, extension := range required {
		if extension.SettlementPolicy != extensions.SettlementPolicyExtensionAttested {
			continue
		}
		methodKnown := input.SettlementMethodKnown
		method := input.SettlementMethod
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input.PaymentCoin)), "fiat:") {
			methodKnown = true
			method = porderpb.PaymentSent_FIAT
		}
		if methodKnown && method != porderpb.PaymentSent_CANCELABLE {
			return fmt.Errorf("%w: extension %s requires CANCELABLE settlement, got %s", ErrOrderExtensionSettlement, extension.ExtensionID, method.String())
		}
	}
	for _, extension := range required {
		if !extension.ReservationRequired {
			continue
		}
		if err := p.reserveOne(ctx, input, extension); err != nil {
			return err
		}
	}
	return nil
}

func (p *orderExtensionProvisioningPolicy) reserveOne(ctx context.Context, input SessionProvisioningPolicyInput, extension extensions.OrderExtension) error {
	if p.reserve == nil {
		return fmt.Errorf("%w: provider %q is unavailable", ErrOrderExtensionReservation, extension.ProviderID)
	}
	request := extensions.ReservationRequest{
		OrderID:        strings.TrimSpace(input.OrderID),
		Extension:      extension,
		PaymentCoin:    strings.TrimSpace(input.PaymentCoin),
		IdempotencyKey: "reserve:" + extension.ExtensionID + ":" + strings.TrimSpace(input.OrderID),
		ExpiresAt:      input.ExpiresAt,
	}
	if err := request.Validate(time.Now().UTC()); err != nil {
		return fmt.Errorf("%w: %v", ErrOrderExtensionReservation, err)
	}
	reservation, err := p.reserve(ctx, request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOrderExtensionReservation, err)
	}
	if err := reservation.Validate(); err != nil {
		return fmt.Errorf("%w: provider returned invalid reservation: %v", ErrOrderExtensionReservation, err)
	}
	if p.record == nil {
		return fmt.Errorf("%w: reservation persistence is unavailable", ErrOrderExtensionReservation)
	}
	if err := p.record(request, reservation); err != nil {
		return fmt.Errorf("%w: persist reservation binding: %w", ErrOrderExtensionReservation, err)
	}
	return nil
}
