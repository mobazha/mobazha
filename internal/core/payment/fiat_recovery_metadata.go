package payment

import (
	"strconv"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
)

const (
	fiatMetaCaptureMode         = "fiat_capture_mode"
	fiatMetaProviderStatus      = "fiat_provider_status"
	fiatMetaExpiresAt           = "fiat_expires_at"
	fiatMetaApproveURL          = "fiat_approve_url"
	fiatMetaCheckoutMode        = "fiat_checkout_mode"
	fiatMetaStripeClientSecret  = "fiat_stripe_client_secret"
	fiatMetaStripePublishable   = "fiat_stripe_publishable_key"
	fiatMetaStripeConnectedAcct = "fiat_stripe_connected_account_id"
	fiatMetaPayPalOrderID       = "fiat_paypal_order_id"
	fiatMetaPayPalClientID      = "fiat_paypal_client_id"
)

func buildFiatRecoveryMetadata(fs *contracts.FiatProviderSession) map[string]string {
	if fs == nil {
		return nil
	}
	meta := map[string]string{}
	if fs.CaptureMode != "" {
		meta[fiatMetaCaptureMode] = string(fs.CaptureMode)
	}
	if fs.Status != "" {
		meta[fiatMetaProviderStatus] = fs.Status
	}
	if !fs.ExpiresAt.IsZero() {
		meta[fiatMetaExpiresAt] = fs.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if fs.ApproveURL != "" {
		meta[fiatMetaApproveURL] = fs.ApproveURL
	}
	if fs.Stripe != nil {
		meta[fiatMetaCheckoutMode] = "embedded"
		if fs.Stripe.ClientSecret != "" {
			meta[fiatMetaStripeClientSecret] = fs.Stripe.ClientSecret
		}
		if fs.Stripe.PublishableKey != "" {
			meta[fiatMetaStripePublishable] = fs.Stripe.PublishableKey
		}
		if fs.Stripe.ConnectedAccountID != "" {
			meta[fiatMetaStripeConnectedAcct] = fs.Stripe.ConnectedAccountID
		}
	}
	if fs.PayPal != nil {
		meta[fiatMetaCheckoutMode] = "redirect"
		if fs.PayPal.OrderID != "" {
			meta[fiatMetaPayPalOrderID] = fs.PayPal.OrderID
		}
		if fs.PayPal.ClientID != "" {
			meta[fiatMetaPayPalClientID] = fs.PayPal.ClientID
		}
	}
	return meta
}

func mergeFiatRecoveryMetadata(providerData map[string]interface{}, meta map[string]string) {
	if providerData == nil || len(meta) == 0 {
		return
	}
	if v := meta[fiatMetaCaptureMode]; v != "" {
		providerData["captureMode"] = v
	}
	if v := meta[fiatMetaProviderStatus]; v != "" {
		providerData["providerStatus"] = v
	}
	if v := meta[fiatMetaExpiresAt]; v != "" {
		if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
			providerData["expiresAt"] = ts.UTC().Format(time.RFC3339Nano)
		}
	}
	if v := meta[fiatMetaApproveURL]; v != "" {
		providerData["approveURL"] = v
	}
	if v := meta[fiatMetaCheckoutMode]; v != "" {
		providerData["checkoutMode"] = v
	}
	if v := meta[fiatMetaStripeClientSecret]; v != "" {
		providerData["clientSecret"] = v
	}
	if v := meta[fiatMetaStripePublishable]; v != "" {
		providerData["publishableKey"] = v
	}
	if v := meta[fiatMetaStripeConnectedAcct]; v != "" {
		providerData["connectedAccountId"] = v
	}
	if v := meta[fiatMetaPayPalOrderID]; v != "" {
		providerData["orderID"] = v
	}
	if v := meta[fiatMetaPayPalClientID]; v != "" {
		providerData["clientID"] = v
	}

	// Preserve future numeric metadata compatibility if needed.
	if v := meta["fiat_amount_cents"]; v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			providerData["amountCents"] = n
		}
	}
}

// shouldExposeFiatRecoveryMetadata returns true only while the order is still
// anchored to the provider-side checkout session (fiat_session_id). Once
// PaymentTransactionID is populated with a distinct provider payment/capture ID,
// the session has advanced beyond checkout and historical clientSecret/approveURL
// data must no longer be re-exposed by the unified session view.
func shouldExposeFiatRecoveryMetadata(paymentTxID string, meta map[string]string) bool {
	if len(meta) == 0 {
		return false
	}
	sessionID := meta["fiat_session_id"]
	if sessionID == "" {
		return false
	}
	if paymentTxID == "" {
		return true
	}
	return paymentTxID == sessionID
}
