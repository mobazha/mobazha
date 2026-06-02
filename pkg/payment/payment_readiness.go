package payment

import (
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// PaymentReadinessStatus is the seller-receipt gate state for buyer-side
// payment sessions. Funding progress remains on SessionStatus / FundingState.
type PaymentReadinessStatus string

const (
	PaymentReadinessAwaitingSellerReceipt PaymentReadinessStatus = "awaiting_seller_receipt"
	PaymentReadinessReadyToPay            PaymentReadinessStatus = "ready_to_pay"
)

const defaultPaymentReadinessRetrySeconds = 2

// PaymentReadinessView describes whether a buyer may receive actionable
// payment instructions. It is layered on top of the canonical order FSM.
type PaymentReadinessView struct {
	Status                 PaymentReadinessStatus `json:"status"`
	ReadyAt                *time.Time             `json:"readyAt,omitempty"`
	RetryAfterSeconds      int                    `json:"retryAfterSeconds,omitempty"`
	SellerReceiptTimeoutAt *time.Time             `json:"sellerReceiptTimeoutAt,omitempty"`
}

// DerivePaymentReadiness builds the readiness view for an order row.
func DerivePaymentReadiness(order *models.Order, sellerReceiptTimeoutAt time.Time) PaymentReadinessView {
	if order == nil {
		return PaymentReadinessView{Status: PaymentReadinessAwaitingSellerReceipt}
	}
	if order.Role() != models.RoleBuyer {
		return PaymentReadinessView{Status: PaymentReadinessReadyToPay}
	}
	if models.IsPaymentReady(order) {
		view := PaymentReadinessView{Status: PaymentReadinessReadyToPay}
		if order.PaymentReadyAt != nil {
			t := *order.PaymentReadyAt
			view.ReadyAt = &t
		}
		return view
	}
	timeout := sellerReceiptTimeoutAt
	return PaymentReadinessView{
		Status:                 PaymentReadinessAwaitingSellerReceipt,
		RetryAfterSeconds:      defaultPaymentReadinessRetrySeconds,
		SellerReceiptTimeoutAt: &timeout,
	}
}

// ApplyBuyerPaymentReadinessGate strips actionable funding instructions when
// the buyer-side order is not yet payment-ready.
func ApplyBuyerPaymentReadinessGate(session *PaymentSession) {
	if session == nil || session.PaymentReadiness.Status == PaymentReadinessReadyToPay {
		return
	}

	ft := &session.FundingTarget
	ft.Address = ""
	ft.QRPayload = ""
	ft.MemoOrTag = ""
	ft.DisplayInstructions = nil
	ft.NetworkFeeHints = nil

	if len(ft.ProviderData) > 0 {
		safe := make(map[string]interface{}, 1)
		if providerID, ok := ft.ProviderData["providerID"].(string); ok && providerID != "" {
			safe["providerID"] = providerID
		}
		ft.ProviderData = safe
	}

	session.UserActionRequest = nil
}
