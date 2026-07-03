package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	paypb "github.com/mobazha/mobazha/pkg/payment"
)

// FiatPaymentFacade wraps contracts.FiatService to produce unified
// payment.PaymentSession values for fiat orders.
//
// It sits between PaymentSessionService and FiatPaymentAppService in the
// calling stack:
//
//	PaymentSessionService.CreateSession (fiat branch)
//	  → FiatPaymentFacade.CreateSession
//	       → contracts.FiatService.CreatePayment  (stores metadata)
//	       → PaymentSessionProjector.Project      (builds PaymentSession)
//
// The facade does NOT own any business logic — it maps the existing
// FiatPaymentAppService result into the unified session contract.
//
// Phase PS B3.
type FiatPaymentFacade struct {
	fiatSvc   contracts.FiatService
	db        database.Database
	projector *PaymentSessionProjector
}

// NewFiatPaymentFacade constructs a FiatPaymentFacade.
func NewFiatPaymentFacade(fiatSvc contracts.FiatService, db database.Database) *FiatPaymentFacade {
	return &FiatPaymentFacade{
		fiatSvc:   fiatSvc,
		db:        db,
		projector: NewPaymentSessionProjector(db),
	}
}

// CreateSession provisions a fiat payment session and returns the unified view.
//
// Steps:
//  1. Parse providerID and currency from req.PaymentCoin ("fiat:{provider}:{currency}").
//  2. Call FiatService.CreatePayment, which stores fiat_session_id + fiat_currency
//     in Order.FiatMetadata via FiatPaymentAppService.
//  3. Run PaymentSessionProjector to build a PaymentSession from the updated order.
//     The projector reads FiatMetadata and produces FundingTarget{type=provider_session}.
//
// Provider SDK fields from FiatProviderSession (client secrets, PayPal URLs, …) are merged
// into FundingTarget.ProviderData immediately after creation.
//
// Idempotency: the primary idempotency gate lives in PaymentSessionServiceImpl.CreateSession
// which checks for a non-empty sessionID in the projection before routing here.  FiatPaymentFacade
// is therefore only called when provisioning is genuinely needed.
//
// If the provider returns the same session (e.g. Stripe PI idempotency), CreatePayment is
// expected to handle that transparently and return the existing FiatProviderSession.  Full SDK
// fields (clientSecret, approveURL, etc.) are merged into the response immediately and also
// persisted into FiatMetadata so subsequent GET / payment-session reads and idempotent POST
// retries can recover the same checkout payload without calling the provider again.
//
// KNOWN LIMITATION: contracts.FiatService does not yet expose a RetrieveSession method.
// In the unlikely case that two requests arrive concurrently for the same order (race), the
// second call will also proceed through CreatePayment; providers must tolerate or deduplicate
// this via their own idempotency keys.
func (f *FiatPaymentFacade) CreateSession(
	ctx context.Context,
	req contracts.CreatePaymentSessionRequest,
) (*paypb.PaymentSession, error) {
	if req.FiatAmountCents <= 0 {
		return nil, fmt.Errorf("%w (got %d)", ErrInvalidFiatAmountCents, req.FiatAmountCents)
	}

	input, err := f.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("fiat facade: load order %s: %w", req.OrderID, err)
	}
	if models.BuyerAwaitingPaymentReadiness(input.order) {
		return f.projector.Project(input)
	}

	providerID, currency, err := parseFiatCoin(req.PaymentCoin)
	if err != nil {
		return nil, fmt.Errorf("fiat facade: %w", err)
	}

	params := contracts.CreatePaymentParams{
		OrderID:     req.OrderID,
		Amount:      req.FiatAmountCents,
		Currency:    currency,
		Description: req.FiatDescription,
		ReturnURL:   req.FiatReturnURL,
		CancelURL:   req.FiatCancelURL,
	}

	provSession, err := f.fiatSvc.CreatePayment(ctx, providerID, params)
	if err != nil {
		return nil, fmt.Errorf("fiat facade: create %s payment for order %s: %w", providerID, req.OrderID, err)
	}
	if err := f.persistSessionRecoveryMetadata(req.OrderID, provSession); err != nil {
		return nil, fmt.Errorf("fiat facade: persist recovery metadata for order %s: %w", req.OrderID, err)
	}

	input, err = f.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("fiat facade: project session for order %s after create: %w", req.OrderID, err)
	}
	view, err := f.projector.Project(input)
	if err != nil {
		return nil, err
	}
	mergeFiatProviderSessionIntoView(view, providerID, provSession)
	return view, nil
}

// parseFiatCoin splits a canonical fiat coin "fiat:{provider}:{currency}" into
// its components. Returns an error if the format is not recognised.
func parseFiatCoin(paymentCoin string) (providerID, currency string, err error) {
	// Expected: "fiat:{provider}:{currency}"
	parts := strings.SplitN(paymentCoin, ":", 3)
	if len(parts) != 3 || !strings.EqualFold(parts[0], "fiat") || parts[1] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("invalid fiat coin format %q: want \"fiat:{provider}:{currency}\"", paymentCoin)
	}
	return strings.ToLower(parts[1]), strings.ToUpper(parts[2]), nil
}

func (f *FiatPaymentFacade) persistSessionRecoveryMetadata(orderID string, fs *contracts.FiatProviderSession) error {
	if fs == nil {
		return nil
	}
	return f.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		if err := order.MergeFiatMetadata(buildFiatRecoveryMetadata(fs)); err != nil {
			return err
		}
		return tx.Update("fiat_metadata", order.FiatMetadata,
			map[string]interface{}{"id = ?": orderID},
			&models.Order{})
	})
}
