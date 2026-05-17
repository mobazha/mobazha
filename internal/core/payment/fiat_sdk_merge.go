//go:build !private_distribution

package payment

import (
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	sesspb "github.com/mobazha/mobazha3.0/pkg/payment"
)

// mergeFiatProviderSessionIntoView enriches FundingTarget.ProviderData with
// provider SDK fields aligned with UNIFIED_PAYMENT_SESSION_ARCHITECTURE §6.2
// / payment_session.go comments (clientSecret, publishableKey, etc.).
func mergeFiatProviderSessionIntoView(
	view *sesspb.PaymentSession,
	providerID string,
	fs *contracts.FiatProviderSession,
) {
	if view == nil || fs == nil {
		return
	}
	fd := view.FundingTarget.ProviderData
	if fd == nil {
		fd = make(map[string]interface{})
	}
	fd["providerID"] = providerID
	if fs.SessionID != "" {
		fd["sessionID"] = fs.SessionID
	}
	fd["captureMode"] = string(fs.CaptureMode)
	if fs.Status != "" {
		fd["providerStatus"] = fs.Status
	}
	if !fs.ExpiresAt.IsZero() {
		fd["expiresAt"] = fs.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if fs.ApproveURL != "" {
		fd["approveURL"] = fs.ApproveURL
	}
	if fs.Stripe != nil {
		fd["checkoutMode"] = "embedded"
		if fs.Stripe.ClientSecret != "" {
			fd["clientSecret"] = fs.Stripe.ClientSecret
		}
		if fs.Stripe.PublishableKey != "" {
			fd["publishableKey"] = fs.Stripe.PublishableKey
		}
		if fs.Stripe.ConnectedAccountID != "" {
			fd["connectedAccountId"] = fs.Stripe.ConnectedAccountID
		}
	}
	if fs.PayPal != nil {
		fd["checkoutMode"] = "redirect"
		if fs.PayPal.OrderID != "" {
			fd["orderID"] = fs.PayPal.OrderID
		}
		if fs.PayPal.ClientID != "" {
			fd["clientID"] = fs.PayPal.ClientID
		}
	}
	view.FundingTarget.ProviderData = fd
}
