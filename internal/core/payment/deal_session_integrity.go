package payment

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// validateDealSessionProvisioning fails closed before a provider session or
// crypto funding target is created. Deal orders must carry an immutable Hosting
// fee-quote reference. Any order whose pricing currency differs from the
// selected payment asset must also bind an immutable payment-selection quote
// before conversion is admitted — including standard (non-Deal) checkout.
func validateDealSessionProvisioning(
	orderOpen *porderpb.OrderOpen,
	req contracts.CreatePaymentSessionRequest,
	selectionQuote *models.PaymentSelectionQuote,
) error {
	ref, err := models.DealTermsSnapshotRefFromOrderOpen(orderOpen)
	if err != nil {
		return fmt.Errorf("%w: invalid signed deal reference: %v", ErrDealPaymentAmountIntegrity, err)
	}
	if strings.TrimSpace(req.PaymentCoin) == "" {
		return nil
	}
	if ref != nil && ref.FeeQuoteID == "" {
		return fmt.Errorf("%w: dealLinkID=%s revision=%d", ErrDealPaymentQuoteRequired, ref.DealLinkID, ref.Revision)
	}

	pricingCode, err := dealPricingCurrencyCode(orderOpen.GetPricingCoin())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDealPaymentAmountIntegrity, err)
	}
	paymentCode, err := iwallet.CoinType(req.PaymentCoin).PricingCurrencyCode()
	if err != nil {
		return fmt.Errorf("%w: resolve payment currency: %v", ErrDealPaymentAmountIntegrity, err)
	}
	feeQuoteID := ""
	if ref != nil {
		feeQuoteID = ref.FeeQuoteID
	}
	if !strings.EqualFold(pricingCode, paymentCode) && selectionQuote == nil {
		return fmt.Errorf(
			"%w: feeQuoteID=%s pricingCurrency=%s paymentCurrency=%s",
			ErrDealPaymentConversionQuoteRequired,
			feeQuoteID,
			pricingCode,
			paymentCode,
		)
	}

	orderAmount, ok := new(big.Int).SetString(strings.TrimSpace(orderOpen.GetAmount()), 10)
	if !ok || orderAmount.Sign() <= 0 {
		return fmt.Errorf("%w: signed order amount must be a positive integer", ErrDealPaymentAmountIntegrity)
	}
	if strings.HasPrefix(strings.ToLower(req.PaymentCoin), "fiat:") && selectionQuote == nil {
		if !orderAmount.IsInt64() || req.FiatAmountCents != orderAmount.Int64() {
			return fmt.Errorf(
				"%w: feeQuoteID=%s signedAmount=%s requestedFiatAmount=%d",
				ErrDealPaymentAmountIntegrity,
				feeQuoteID,
				orderAmount.String(),
				req.FiatAmountCents,
			)
		}
	}
	return nil
}

func dealPricingCurrencyCode(pricingCoin string) (string, error) {
	trimmed := strings.TrimSpace(pricingCoin)
	if trimmed == "" {
		return "", fmt.Errorf("signed order pricingCoin is required")
	}
	coin := iwallet.CoinType(trimmed)
	if coin.IsCanonicalPaymentCoin() {
		return coin.PricingCurrencyCode()
	}
	return strings.ToUpper(trimmed), nil
}

// validateDealPaymentSession verifies the actionable projection returned after
// provisioning. NetworkFeeHints are intentionally excluded: they are advisory
// gas-fee metadata, not a numeric charge included in the signed order total.
func validateDealPaymentSession(
	orderOpen *porderpb.OrderOpen,
	req contracts.CreatePaymentSessionRequest,
	selectionQuote *models.PaymentSelectionQuote,
	session *paypb.PaymentSession,
) error {
	ref, err := models.DealTermsSnapshotRefFromOrderOpen(orderOpen)
	if err != nil {
		return fmt.Errorf("%w: invalid signed deal reference: %v", ErrDealPaymentAmountIntegrity, err)
	}
	if ref == nil && selectionQuote == nil {
		return nil
	}
	if session == nil {
		return fmt.Errorf("%w: payment session is nil", ErrDealPaymentAmountIntegrity)
	}
	// Settlement authorization is asynchronous. While the buyer is waiting for
	// the seller receipt, the readiness gate deliberately strips the actionable
	// target (including PaymentCoin-derived address data). Validate the immutable
	// quote before starting the ceremony, then defer projection equality checks
	// until the session is ready to pay.
	if session.PaymentReadiness.Status == paypb.PaymentReadinessAwaitingSellerReceipt {
		return nil
	}

	expectedAmount := orderOpen.GetAmount()
	if selectionQuote != nil {
		expectedAmount = selectionQuote.BuyerPaymentTotal
	}
	expected := paypb.FormatSessionAmount(expectedAmount, req.PaymentCoin)
	feeQuoteID := ""
	if ref != nil {
		feeQuoteID = ref.FeeQuoteID
	} else if selectionQuote != nil {
		feeQuoteID = selectionQuote.FeeQuoteID
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{name: "paymentCoin", got: session.PaymentCoin, want: req.PaymentCoin},
		{name: "expectedAmount", got: session.ExpectedAmount, want: expected},
		{name: "fundingTarget.assetID", got: session.FundingTarget.AssetID, want: req.PaymentCoin},
		{name: "fundingTarget.amount", got: session.FundingTarget.Amount, want: expected},
		{name: "paymentProgress.requiredAmount", got: session.PaymentProgress.RequiredAmount, want: expected},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf(
				"%w: feeQuoteID=%s field=%s got=%q want=%q",
				ErrDealPaymentAmountIntegrity,
				feeQuoteID,
				check.name,
				check.got,
				check.want,
			)
		}
	}
	return nil
}

func applyPaymentSelectionQuote(session *paypb.PaymentSession, quote *models.PaymentSelectionQuote) {
	if session != nil && quote != nil {
		session.PaymentSelectionQuoteID = quote.QuoteID
	}
}
