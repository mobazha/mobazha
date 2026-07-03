package order

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha/internal/core/checkoutsupply"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

// SetCheckoutSupplyQuoter wires the shared checkout supply quote service.
func (s *OrderAppService) SetCheckoutSupplyQuoter(quoter *checkoutsupply.CheckoutSupplyQuoteService) {
	if s == nil {
		return
	}
	s.checkoutSupplyQuoter = quoter
	if s.digitalSupplyLines != nil && quoter != nil {
		quoter.SetDigitalSupplyLineResolver(s.digitalSupplyLines)
	}
}

// QuoteCheckoutSupply performs a buyer-safe advisory supply preflight for
// authenticated standard checkout without creating an order or holding inventory.
func (s *OrderAppService) QuoteCheckoutSupply(ctx context.Context, req contracts.QuoteCheckoutSupplyRequest) (*contracts.CheckoutSupplyQuoteResponse, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}
	if s == nil || s.checkoutSupplyQuoter == nil {
		return nil, fmt.Errorf("checkout supply quote service not configured")
	}
	return s.checkoutSupplyQuoter.Quote(ctx, models.OrderTypeStandard, "checkout_quote", req.Items)
}

// SummarizeListingSupply performs a seller-safe advisory supply summary for
// admin product surfaces without creating inventory holds.
func (s *OrderAppService) SummarizeListingSupply(ctx context.Context, req contracts.ListingSupplySummaryRequest) (*contracts.ListingSupplySummaryResponse, error) {
	if s == nil || s.checkoutSupplyQuoter == nil {
		return nil, fmt.Errorf("checkout supply quote service not configured")
	}
	return s.checkoutSupplyQuoter.SellerSummary(ctx, req)
}
