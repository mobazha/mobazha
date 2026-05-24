//go:build !private_distribution

package api

import (
	"math/big"

	ordercalc "github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

type orderPricingBreakdownResp struct {
	Subtotal  string `json:"subtotal"`
	Shipping  string `json:"shipping"`
	Discounts string `json:"discounts"`
	Taxes     string `json:"taxes"`
	Total     string `json:"total"`
	Currency  string `json:"currency"`
}

type orderSettlementBreakdownLineResp struct {
	Type    string `json:"type"`
	Amount  string `json:"amount"`
	Address string `json:"address,omitempty"`
}

type orderSettlementBreakdownResp struct {
	Source           string                             `json:"source,omitempty"`
	Currency         string                             `json:"currency,omitempty"`
	EscrowedAmount   string                             `json:"escrowedAmount,omitempty"`
	SellerAmount     string                             `json:"sellerAmount,omitempty"`
	SellerAddress    string                             `json:"sellerAddress,omitempty"`
	BuyerAmount      string                             `json:"buyerAmount,omitempty"`
	BuyerAddress     string                             `json:"buyerAddress,omitempty"`
	ModeratorAmount  string                             `json:"moderatorAmount,omitempty"`
	ModeratorAddress string                             `json:"moderatorAddress,omitempty"`
	PlatformAmount   string                             `json:"platformAmount,omitempty"`
	PlatformAddress  string                             `json:"platformAddress,omitempty"`
	TransactionFee   string                             `json:"transactionFee,omitempty"`
	TxHash           string                             `json:"txHash,omitempty"`
	Lines            []orderSettlementBreakdownLineResp `json:"lines,omitempty"`
}

func buildOrderPricingBreakdown(order *models.Order, erp wallet.ExchangeRateQuerier) *orderPricingBreakdownResp {
	if order == nil {
		return nil
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil || orderOpen == nil {
		return nil
	}
	totals, err := ordercalc.CalculateOrderTotal(orderOpen, erp)
	if err != nil {
		return nil
	}
	return &orderPricingBreakdownResp{
		Subtotal:  totals.Subtotal.String(),
		Shipping:  totals.Shipping.String(),
		Discounts: totals.Discounts.String(),
		Taxes:     totals.Taxes.String(),
		Total:     totals.Total.String(),
		Currency:  orderOpen.GetPricingCoin(),
	}
}

func buildOrderSettlementBreakdown(order *models.Order) *orderSettlementBreakdownResp {
	if order == nil {
		return nil
	}
	currency := settlementCurrency(order)
	if orderComplete, err := order.OrderCompleteMessage(); err == nil && orderComplete.GetReleaseInfo() != nil {
		return settlementFromEscrowRelease("complete", "seller", currency, orderComplete.GetReleaseInfo())
	}
	if shipments, err := order.OrderShipmentMessages(); err == nil {
		for i := len(shipments) - 1; i >= 0; i-- {
			if shipments[i].GetReleaseInfo() != nil {
				return settlementFromEscrowRelease("shipment", "seller", currency, shipments[i].GetReleaseInfo())
			}
		}
	}
	if refunds, err := order.Refunds(); err == nil {
		for i := len(refunds) - 1; i >= 0; i-- {
			if refunds[i].GetReleaseInfo() != nil {
				return settlementFromEscrowRelease("refund", "buyer", currency, refunds[i].GetReleaseInfo())
			}
		}
	}
	if disputeClose, err := order.DisputeClosedMessage(); err == nil && disputeClose.GetReleaseInfo() != nil {
		return settlementFromDisputeRelease(currency, disputeClose.GetReleaseInfo())
	}
	return nil
}

func settlementCurrency(order *models.Order) string {
	if paymentSent, err := order.PaymentSentMessage(); err == nil && paymentSent != nil && paymentSent.GetCoin() != "" {
		return paymentSent.GetCoin()
	}
	if orderOpen, err := order.OrderOpenMessage(); err == nil && orderOpen != nil {
		return orderOpen.GetPricingCoin()
	}
	return ""
}

func settlementFromEscrowRelease(source, recipientType, currency string, release *pb.EscrowRelease) *orderSettlementBreakdownResp {
	if release == nil {
		return nil
	}
	out := &orderSettlementBreakdownResp{
		Source:          source,
		Currency:        currency,
		EscrowedAmount:  escrowedAmountFromOutpoints(release.GetOutpoints(), release.GetToAmount(), release.GetPlatformAmount(), release.GetTransactionFee()),
		PlatformAmount:  release.GetPlatformAmount(),
		PlatformAddress: release.GetPlatformAddress(),
		TransactionFee:  release.GetTransactionFee(),
		TxHash:          release.GetTxid(),
	}
	switch recipientType {
	case "buyer":
		out.BuyerAmount = release.GetToAmount()
		out.BuyerAddress = release.GetToAddress()
		out.addLine("buyer", release.GetToAmount(), release.GetToAddress())
	default:
		out.SellerAmount = release.GetToAmount()
		out.SellerAddress = release.GetToAddress()
		out.addLine("seller", release.GetToAmount(), release.GetToAddress())
	}
	out.addLine("platform", release.GetPlatformAmount(), release.GetPlatformAddress())
	out.addLine("network_fee", release.GetTransactionFee(), "")
	return out
}

func settlementFromDisputeRelease(currency string, release *pb.DisputeClose_ModeratedEscrowRelease) *orderSettlementBreakdownResp {
	if release == nil {
		return nil
	}
	out := &orderSettlementBreakdownResp{
		Source:           "dispute",
		Currency:         currency,
		EscrowedAmount:   escrowedAmountFromOutpoints(release.GetOutpoints(), release.GetBuyerAmount(), release.GetVendorAmount(), release.GetModeratorAmount(), release.GetTransactionFee()),
		SellerAmount:     release.GetVendorAmount(),
		SellerAddress:    release.GetVendorAddress(),
		BuyerAmount:      release.GetBuyerAmount(),
		BuyerAddress:     release.GetBuyerAddress(),
		ModeratorAmount:  release.GetModeratorAmount(),
		ModeratorAddress: release.GetModeratorAddress(),
		TransactionFee:   release.GetTransactionFee(),
	}
	out.addLine("seller", release.GetVendorAmount(), release.GetVendorAddress())
	out.addLine("buyer", release.GetBuyerAmount(), release.GetBuyerAddress())
	out.addLine("moderator", release.GetModeratorAmount(), release.GetModeratorAddress())
	out.addLine("network_fee", release.GetTransactionFee(), "")
	return out
}

func (b *orderSettlementBreakdownResp) addLine(lineType, amount, address string) {
	if b == nil || amount == "" || amount == "0" {
		return
	}
	b.Lines = append(b.Lines, orderSettlementBreakdownLineResp{
		Type:    lineType,
		Amount:  amount,
		Address: address,
	})
}

func escrowedAmountFromOutpoints(outpoints []*pb.Outpoint, fallbackParts ...string) string {
	total := big.NewInt(0)
	for _, outpoint := range outpoints {
		addDecimalString(total, outpoint.GetValue())
	}
	if total.Sign() > 0 {
		return total.String()
	}
	for _, part := range fallbackParts {
		addDecimalString(total, part)
	}
	return total.String()
}

func addDecimalString(total *big.Int, raw string) {
	if total == nil || raw == "" {
		return
	}
	v, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return
	}
	total.Add(total, v)
}
