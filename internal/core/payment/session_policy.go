package payment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	porderpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
	OrderID     string
	PaymentCoin string
	ExpiresAt   time.Time
	OrderOpen   *porderpb.OrderOpen
}

// CollectibleFirstSaleAuthorizationSignal asks hosting to atomically reserve a
// source-custody collectible for this order before any funding target exists.
// CollectibleFirstSaleAuthorizationSignal is the policy-layer reservation payload.
type CollectibleFirstSaleAuthorizationSignal struct {
	OrderID              string
	HubSlotID            string
	CertNumber           string
	SellerPeerID         string
	PaymentCoin          string
	ReservationExpiresAt time.Time
}

// CollectibleFirstSaleAuthorizationHook authorizes and reserves a managed
// collectible before payment provisioning.
type CollectibleFirstSaleAuthorizationHook func(context.Context, CollectibleFirstSaleAuthorizationSignal) error

type collectibleFirstSaleProvisioningPolicy struct {
	authorize CollectibleFirstSaleAuthorizationHook
}

// NewCollectibleFirstSaleProvisioningPolicy creates the fail-closed policy for
// managed source-custody collectible orders.
func NewCollectibleFirstSaleProvisioningPolicy(authorize CollectibleFirstSaleAuthorizationHook) SessionProvisioningPolicy {
	return &collectibleFirstSaleProvisioningPolicy{authorize: authorize}
}

func (p *collectibleFirstSaleProvisioningPolicy) AuthorizeSessionProvisioning(
	ctx context.Context,
	input SessionProvisioningPolicyInput,
) error {
	if !orderOpenContainsRWA(input.OrderOpen) {
		return nil
	}
	if !models.IsManagedCollectibleFirstSale(input.OrderOpen) {
		return fmt.Errorf("%w", ErrRWAPaymentSessionUnsupported)
	}
	// Source-custody defaults require escrow-native refunds. Provider checkout
	// refunds need a separate provider-refund ledger and are intentionally
	// rejected until that domain workflow exists.
	if strings.HasPrefix(input.PaymentCoin, "fiat:") {
		return fmt.Errorf("%w: managed collectible first sales require an escrow-backed crypto rail", ErrRWAPaymentSessionUnsupported)
	}
	if !strings.HasPrefix(input.PaymentCoin, "crypto:") {
		return fmt.Errorf("%w: unsupported payment rail %q", ErrRWAPaymentSessionUnsupported, input.PaymentCoin)
	}
	if p == nil || p.authorize == nil {
		return fmt.Errorf("%w: hosting source-custody authorizer is unavailable", ErrCollectibleFirstSalePreflight)
	}
	meta, ok := models.CollectibleOrderMetadataFromOrderOpen(input.OrderOpen)
	if !ok {
		return fmt.Errorf("%w: collectible order metadata is incomplete", ErrCollectibleFirstSalePreflight)
	}
	if err := p.authorize(ctx, CollectibleFirstSaleAuthorizationSignal{
		OrderID:              strings.TrimSpace(input.OrderID),
		HubSlotID:            strings.TrimSpace(meta.HubSlotID),
		CertNumber:           strings.TrimSpace(meta.CertNumber),
		SellerPeerID:         strings.TrimSpace(meta.SellerPeerID),
		PaymentCoin:          strings.TrimSpace(input.PaymentCoin),
		ReservationExpiresAt: input.ExpiresAt,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrCollectibleFirstSalePreflight, err)
	}
	return nil
}

func orderOpenContainsRWA(orderOpen *porderpb.OrderOpen) bool {
	if orderOpen == nil {
		return false
	}
	for _, signed := range orderOpen.GetListings() {
		if signed.GetListing().GetMetadata().GetContractType() == porderpb.Listing_Metadata_RWA_TOKEN {
			return true
		}
	}
	return false
}
