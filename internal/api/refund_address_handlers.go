//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// handlePOSTOrderRefundAddress persists the buyer-controlled crypto refund
// destination for an order once the payment coin is known.
//
// POST /v1/orders/{orderID}/refund-address
//
// Body JSON: { "refundAddress": "0x...", "paymentCoin": "optional canonical coin" }
func (g *Gateway) handlePOSTOrderRefundAddress(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	var payload struct {
		RefundAddress string `json:"refundAddress"`
		PaymentCoin   string `json:"paymentCoin"`
	}
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&payload); err != nil {
			if !errors.Is(err, io.EOF) {
				responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
				return
			}
		}
	}

	orderSvc := getOrderService(r)
	order, err := orderSvc.GetOrder(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "order not found: "+orderID)
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, wrapError(err))
		return
	}
	if order == nil {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "order not found: "+orderID)
		return
	}
	if order.Role() != models.RoleBuyer {
		responsePkg.Error(w, http.StatusForbidden, responsePkg.CodeForbidden,
			"only the buyer may set the refund address for this order")
		return
	}

	coin, err := resolveRefundAddressPaymentCoin(order, payload.PaymentCoin)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if coin.IsFiatPayment() {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"refund address is not applicable to fiat orders")
		return
	}

	if err := orderSvc.SetOrderRefundAddressForPayment(r.Context(), orderID, coin, payload.RefundAddress); err != nil {
		switch {
		case errors.Is(err, models.ErrRefundAddressRequired):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeRefundAddressRequired, err.Error())
		case errors.Is(err, coreiface.ErrBadRequest),
			errors.Is(err, models.ErrRefundAddressInvalid):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		default:
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, wrapError(err))
		}
		return
	}

	responsePkg.Success(w, map[string]interface{}{
		"orderID":       orderID,
		"refundAddress": strings.TrimSpace(payload.RefundAddress),
		"paymentCoin":   string(coin),
	})
}

func resolveRefundAddressPaymentCoin(order *models.Order, paymentCoin string) (iwallet.CoinType, error) {
	if coin := strings.TrimSpace(paymentCoin); coin != "" {
		normalized, err := iwallet.NormalizePaymentCoinIngress(coin)
		if err != nil {
			return "", err
		}
		return normalized, nil
	}
	coin, err := order.GetPaymentCoinType()
	if err != nil {
		return "", err
	}
	if coin == "" {
		return "", errors.New("paymentCoin is required when the order has no selected payment coin yet")
	}
	return coin, nil
}
