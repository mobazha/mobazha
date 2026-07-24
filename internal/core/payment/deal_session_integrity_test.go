package payment

import (
	"errors"
	"strings"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

func TestValidateDealSessionProvisioning(t *testing.T) {
	dealOpen := func(pricingCoin, amount, feeQuoteID string) *porderpb.OrderOpen {
		return &porderpb.OrderOpen{
			PricingCoin:  pricingCoin,
			Amount:       amount,
			DealLinkID:   "deal-123",
			DealRevision: 2,
			TermsHash:    strings.Repeat("a", 64),
			FeeQuoteID:   feeQuoteID,
		}
	}

	tests := []struct {
		name    string
		open    *porderpb.OrderOpen
		req     contracts.CreatePaymentSessionRequest
		wantErr error
	}{
		{
			name: "standard same currency remains compatible without quote",
			open: &porderpb.OrderOpen{PricingCoin: "ETH", Amount: "1000000000000000000"},
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin: "crypto:eip155:1:native",
			},
		},
		{
			name: "standard cross currency requires immutable conversion quote",
			open: &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"},
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "crypto:eip155:1:native",
				FiatAmountCents: 1,
			},
			wantErr: ErrDealPaymentConversionQuoteRequired,
		},
		{
			name: "standard cross-currency fiat missing quote is rejected",
			open: &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"},
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:EUR",
				FiatAmountCents: 1,
			},
			wantErr: ErrDealPaymentConversionQuoteRequired,
		},
		{
			name: "standard same-currency fiat amount must match signed order",
			open: &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"},
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:USD",
				FiatAmountCents: 1,
			},
			wantErr: ErrDealPaymentAmountIntegrity,
		},
		{
			name: "deal requires fee quote",
			open: dealOpen("USD", "4900", ""),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:USD",
				FiatAmountCents: 4900,
			},
			wantErr: ErrDealPaymentQuoteRequired,
		},
		{
			name: "cross currency requires immutable conversion quote",
			open: dealOpen("USD", "4900", "fq-123"),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin: "crypto:eip155:1:native",
			},
			wantErr: ErrDealPaymentConversionQuoteRequired,
		},
		{
			name: "same currency fiat amount matches signed order",
			open: dealOpen("USD", "4900", "fq-123"),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:USD",
				FiatAmountCents: 4900,
			},
		},
		{
			name: "canonical pricing coin resolves to same currency",
			open: dealOpen("crypto:eip155:1:native", "1000000000000000000", "fq-123"),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin: "crypto:eip155:1:native",
			},
		},
		{
			name: "fiat request amount cannot override signed order",
			open: dealOpen("USD", "4900", "fq-123"),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:USD",
				FiatAmountCents: 1,
			},
			wantErr: ErrDealPaymentAmountIntegrity,
		},
		{
			name: "signed amount must be positive integer",
			open: dealOpen("USD", "not-an-amount", "fq-123"),
			req: contracts.CreatePaymentSessionRequest{
				PaymentCoin:     "fiat:stripe:USD",
				FiatAmountCents: 4900,
			},
			wantErr: ErrDealPaymentAmountIntegrity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDealSessionProvisioning(tt.open, tt.req, nil)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidateDealPaymentSession(t *testing.T) {
	open := &porderpb.OrderOpen{
		PricingCoin:  "USD",
		Amount:       "4900",
		DealLinkID:   "deal-123",
		DealRevision: 2,
		TermsHash:    strings.Repeat("b", 64),
		FeeQuoteID:   "fq-123",
	}
	req := contracts.CreatePaymentSessionRequest{
		PaymentCoin:     "fiat:stripe:USD",
		FiatAmountCents: 4900,
	}
	valid := func() *paypb.PaymentSession {
		return &paypb.PaymentSession{
			PaymentCoin:    req.PaymentCoin,
			ExpectedAmount: "49",
			PaymentReadiness: paypb.PaymentReadinessView{
				Status: paypb.PaymentReadinessReadyToPay,
			},
			FundingTarget: paypb.FundingTargetView{
				AssetID:         req.PaymentCoin,
				Amount:          "49",
				NetworkFeeHints: &paypb.NetworkFeeHints{FeePayer: "buyer", Asset: "crypto:eip155:1:native"},
			},
			PaymentProgress: paypb.PaymentProgressView{RequiredAmount: "49"},
		}
	}

	require.NoError(t, validateDealPaymentSession(open, req, nil, valid()),
		"advisory network fee hints must not change the signed order amount")
	require.NoError(t, validateDealPaymentSession(open, req, nil, &paypb.PaymentSession{
		PaymentReadiness: paypb.PaymentReadinessView{Status: paypb.PaymentReadinessAwaitingSellerReceipt},
	}), "non-actionable ceremony drafts are validated when they become ready")

	tests := []struct {
		name   string
		mutate func(*paypb.PaymentSession)
	}{
		{name: "payment coin mismatch", mutate: func(s *paypb.PaymentSession) { s.PaymentCoin = "fiat:paypal:USD" }},
		{name: "expected amount mismatch", mutate: func(s *paypb.PaymentSession) { s.ExpectedAmount = "48.99" }},
		{name: "funding asset mismatch", mutate: func(s *paypb.PaymentSession) { s.FundingTarget.AssetID = "fiat:paypal:USD" }},
		{name: "funding amount mismatch", mutate: func(s *paypb.PaymentSession) { s.FundingTarget.Amount = "50" }},
		{name: "required amount mismatch", mutate: func(s *paypb.PaymentSession) { s.PaymentProgress.RequiredAmount = "48" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := valid()
			tt.mutate(session)
			err := validateDealPaymentSession(open, req, nil, session)
			require.True(t, errors.Is(err, ErrDealPaymentAmountIntegrity), "error = %v", err)
		})
	}
}

func TestValidateDealSessionProvisioning_AcceptsBoundCrossCurrencyQuote(t *testing.T) {
	open := &porderpb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", DealLinkID: "deal-123", DealRevision: 2,
		TermsHash: strings.Repeat("c", 64), FeeQuoteID: "fq-123",
	}
	req := contracts.CreatePaymentSessionRequest{
		PaymentCoin: "crypto:eip155:1:native", PaymentSelectionQuoteID: "psq-123",
		AuthorizedPaymentAmount: "19600000000000000",
	}
	quote := &models.PaymentSelectionQuote{
		QuoteID: "psq-123", FeeQuoteID: "fq-123", PaymentCoin: req.PaymentCoin,
		BuyerPaymentTotal: req.AuthorizedPaymentAmount,
	}
	require.NoError(t, validateDealSessionProvisioning(open, req, quote))

	expected := paypb.FormatSessionAmount(quote.BuyerPaymentTotal, req.PaymentCoin)
	session := &paypb.PaymentSession{
		PaymentCoin: req.PaymentCoin, ExpectedAmount: expected,
		PaymentReadiness: paypb.PaymentReadinessView{Status: paypb.PaymentReadinessReadyToPay},
		FundingTarget:    paypb.FundingTargetView{AssetID: req.PaymentCoin, Amount: expected},
		PaymentProgress:  paypb.PaymentProgressView{RequiredAmount: expected},
	}
	require.NoError(t, validateDealPaymentSession(open, req, quote, session))
}

func TestValidateDealSessionProvisioning_AcceptsStandardCrossCurrencyFiatQuote(t *testing.T) {
	open := &porderpb.OrderOpen{PricingCoin: "USD", Amount: "4900"}
	req := contracts.CreatePaymentSessionRequest{
		PaymentCoin: "fiat:stripe:EUR", PaymentSelectionQuoteID: "psq-standard",
		// Client-supplied amount must be ignored once a quote is bound.
		FiatAmountCents: 1,
	}
	quote := &models.PaymentSelectionQuote{
		QuoteID: "psq-standard", PaymentCoin: req.PaymentCoin, BuyerPaymentTotal: "4500",
	}
	require.NoError(t, validateDealSessionProvisioning(open, req, quote))
}
