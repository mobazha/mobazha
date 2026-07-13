package core

import (
	"context"
	"errors"
	"testing"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

type affiliateTermsReaderStub struct {
	present bool
	err     error
}

func (s affiliateTermsReaderStub) HasSettlementTerms(context.Context, string) (bool, error) {
	return s.present, s.err
}

func TestSellerAffiliatePaymentProvisioningPolicy_FailsClosedForFiat(t *testing.T) {
	policy := sellerAffiliatePaymentProvisioningPolicy{terms: affiliateTermsReaderStub{present: true}}
	err := policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		OrderID: "referred-order", PaymentCoin: "fiat:stripe:USD",
	})
	require.ErrorIs(t, err, models.ErrSellerAffiliatePaymentRailUnsupported)

	err = policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		OrderID: "referred-order", PaymentCoin: "crypto:solana:mainnet:native",
	})
	require.NoError(t, err)
}

func TestSellerAffiliatePaymentProvisioningPolicy_UsesSignedOrderReferralBeforeSellerLedger(t *testing.T) {
	policy := sellerAffiliatePaymentProvisioningPolicy{}
	err := policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		OrderID: "referred-order", PaymentCoin: "fiat:stripe:USD",
		OrderOpen: &pb.OrderOpen{AffiliateReferralSessionID: "signed-referral-session"},
	})
	require.ErrorIs(t, err, models.ErrSellerAffiliatePaymentRailUnsupported)
}

func TestSellerAffiliatePaymentProvisioningPolicy_PropagatesTermsLookupFailure(t *testing.T) {
	want := errors.New("database unavailable")
	policy := sellerAffiliatePaymentProvisioningPolicy{terms: affiliateTermsReaderStub{err: want}}
	err := policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		OrderID: "referred-order", PaymentCoin: "fiat:paypal:USD",
	})
	require.ErrorIs(t, err, want)
}
