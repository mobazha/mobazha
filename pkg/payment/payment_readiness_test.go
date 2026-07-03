package payment

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestDerivePaymentReadiness_BuyerNotReady(t *testing.T) {
	timeout := time.Now().Add(5 * time.Minute)
	order := &models.Order{MyRole: string(models.RoleBuyer)}

	view := DerivePaymentReadiness(order, timeout)
	require.Equal(t, PaymentReadinessAwaitingSellerReceipt, view.Status)
	require.Equal(t, 2, view.RetryAfterSeconds)
	require.NotNil(t, view.SellerReceiptTimeoutAt)
}

func TestDerivePaymentReadiness_BuyerReady(t *testing.T) {
	readyAt := time.Now().Add(-time.Minute)
	order := &models.Order{
		MyRole:         string(models.RoleBuyer),
		PaymentReadyAt: &readyAt,
	}

	view := DerivePaymentReadiness(order, time.Now().Add(time.Hour))
	require.Equal(t, PaymentReadinessReadyToPay, view.Status)
	require.NotNil(t, view.ReadyAt)
}

func TestDerivePaymentReadiness_VendorAlwaysReady(t *testing.T) {
	order := &models.Order{MyRole: string(models.RoleVendor)}

	view := DerivePaymentReadiness(order, time.Now().Add(time.Hour))
	require.Equal(t, PaymentReadinessReadyToPay, view.Status)
}

func TestApplyBuyerPaymentReadinessGate_StripsFundingTarget(t *testing.T) {
	session := &PaymentSession{
		PaymentReadiness: PaymentReadinessView{Status: PaymentReadinessAwaitingSellerReceipt},
		FundingTarget: FundingTargetView{
			Type:      FundingTargetTypeAddress,
			Address:   "0xabc",
			AssetID:   "crypto:ethereum:mainnet:native",
			Amount:    "1.0",
			QRPayload: "ethereum:0xabc",
			ProviderData: map[string]interface{}{
				"providerID":   "stripe",
				"clientSecret": "sec_test",
				"approveURL":   "https://example.com/pay",
			},
		},
	}

	ApplyBuyerPaymentReadinessGate(session)
	require.Empty(t, session.FundingTarget.Address)
	require.Empty(t, session.FundingTarget.QRPayload)
	require.Equal(t, "1.0", session.FundingTarget.Amount)
	require.Equal(t, "stripe", session.FundingTarget.ProviderData["providerID"])
	require.NotContains(t, session.FundingTarget.ProviderData, "clientSecret")
}
