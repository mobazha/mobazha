package orders

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// BuildAffiliateOrderFacts derives the immutable seller, buyer, and net
// merchandise facts used both to quote payment-attempt terms and to persist the
// verified-payment attribution later.
func BuildAffiliateOrderFacts(
	orderID string,
	orderOpen *pb.OrderOpen,
	referralSessionID string,
	attributedAt time.Time,
	exchangeRates wallet.ExchangeRateQuerier,
) (models.AffiliateOrderFacts, error) {
	if orderOpen == nil || len(orderOpen.GetItems()) == 0 || orderOpen.GetBuyerID() == nil || attributedAt.IsZero() {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}
	amounts, err := CalculateOrderNetMerchandiseLines(orderOpen, exchangeRates)
	if err != nil {
		return models.AffiliateOrderFacts{}, fmt.Errorf("calculate affiliate merchandise lines: %w", err)
	}
	if len(amounts) != len(orderOpen.GetItems()) {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}

	sellerPeerID := ""
	lines := make([]models.AffiliateOrderLineFact, 0, len(amounts))
	for index, item := range orderOpen.GetItems() {
		listing, err := utils.ExtractListing(item.GetListingHash(), orderOpen.GetListings())
		if err != nil || listing.GetVendorID() == nil {
			return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
		}
		peerID := strings.TrimSpace(listing.GetVendorID().GetPeerID())
		if peerID == "" || (sellerPeerID != "" && sellerPeerID != peerID) {
			return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
		}
		sellerPeerID = peerID
		if amounts[index].Cmp(iwallet.NewAmount(0)) <= 0 {
			continue
		}
		lines = append(lines, models.AffiliateOrderLineFact{
			OrderLineID:          fmt.Sprintf("%s:%d", strings.TrimSpace(orderID), index),
			NetMerchandiseAtomic: amounts[index].String(),
			Currency:             orderOpen.GetPricingCoin(),
		})
	}
	if len(lines) == 0 {
		return models.AffiliateOrderFacts{}, models.ErrInvalidSellerAffiliate
	}
	return models.AffiliateOrderFacts{
		OrderID:           strings.TrimSpace(orderID),
		SellerPeerID:      sellerPeerID,
		BuyerPeerID:       strings.TrimSpace(orderOpen.GetBuyerID().GetPeerID()),
		ReferralSessionID: strings.TrimSpace(referralSessionID),
		AttributedAt:      attributedAt.UTC(),
		Lines:             lines,
	}, nil
}
