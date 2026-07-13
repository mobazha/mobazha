package core

import (
	"context"
	"fmt"
	"strings"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/models"
)

type sellerAffiliateSettlementTermsReader interface {
	HasSettlementTerms(context.Context, string) (bool, error)
}

// sellerAffiliatePaymentProvisioningPolicy prevents a referred order from
// entering a rail that cannot execute its frozen commission output. Provider
// payments currently have no atomic multi-party payout primitive, so Stripe
// and PayPal must fail before an external payment session is created.
type sellerAffiliatePaymentProvisioningPolicy struct {
	terms sellerAffiliateSettlementTermsReader
}

func (p sellerAffiliatePaymentProvisioningPolicy) AuthorizeSessionProvisioning(ctx context.Context, input corepayment.SessionProvisioningPolicyInput) error {
	if strings.TrimSpace(input.OrderID) == "" ||
		!strings.HasPrefix(strings.ToLower(strings.TrimSpace(input.PaymentCoin)), "fiat:") {
		return nil
	}
	if input.OrderOpen != nil && strings.TrimSpace(input.OrderOpen.GetAffiliateReferralSessionID()) != "" {
		return fmt.Errorf("%w: Stripe and PayPal cannot execute the frozen promoter payout", models.ErrSellerAffiliatePaymentRailUnsupported)
	}
	if p.terms == nil {
		return nil
	}
	hasTerms, err := p.terms.HasSettlementTerms(ctx, strings.TrimSpace(input.OrderID))
	if err != nil {
		return fmt.Errorf("authorize seller affiliate payment rail: %w", err)
	}
	if hasTerms {
		return fmt.Errorf("%w: Stripe and PayPal cannot execute the frozen promoter payout", models.ErrSellerAffiliatePaymentRailUnsupported)
	}
	return nil
}
