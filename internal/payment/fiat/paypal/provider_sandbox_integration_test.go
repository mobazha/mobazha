//go:build integration

// SPDX-License-Identifier: MPL-2.0

package paypal

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/require"
)

func TestPayPalSandbox_DelayedDisbursementApprovedPartner(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PAYPAL_DELAYED_DISBURSEMENT_SMOKE")) != "1" {
		t.Skip("PAYPAL_DELAYED_DISBURSEMENT_SMOKE=1 is required")
	}
	requiredEnv := func(key string) string {
		t.Helper()
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			t.Fatalf("%s is required for the delayed-disbursement sandbox smoke", key)
		}
		return value
	}

	provider := NewProvider(Config{
		ClientID:             requiredEnv("PAYPAL_CLIENT_ID"),
		ClientSecret:         requiredEnv("PAYPAL_CLIENT_SECRET"),
		Mode:                 ModePartner,
		Sandbox:              true,
		PartnerID:            requiredEnv("PAYPAL_PARTNER_ID"),
		PartnerAttributionID: requiredEnv("PAYPAL_PARTNER_ATTRIBUTION_ID"),
		DelayedDisbursement:  true,
	})
	session, err := provider.CreatePayment(context.Background(), contracts.CreatePaymentParams{
		OrderID:         "mobazha-delayed-disbursement-smoke",
		Amount:          100,
		Currency:        "USD",
		Description:     "Mobazha delayed-disbursement sandbox capability smoke",
		ReturnURL:       "https://example.invalid/paypal/return",
		CancelURL:       "https://example.invalid/paypal/cancel",
		IdempotencyKey:  "mbza-delayed-disbursement-smoke",
		SellerAccountID: requiredEnv("PAYPAL_MERCHANT_ID"),
		ModeratorPeerID: "sandbox-moderator",
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotEmpty(t, session.SessionID)
	require.NotEmpty(t, session.ApproveURL)
	require.Equal(t, contracts.CaptureManual, session.CaptureMode)
}
