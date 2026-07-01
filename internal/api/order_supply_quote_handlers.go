package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// handlePOSTOrderSupplyQuote returns a buyer-safe advisory supply quote for
// authenticated standard checkout.
// POST /v1/orders/supply-quote
func (g *Gateway) handlePOSTOrderSupplyQuote(w http.ResponseWriter, r *http.Request) {
	orderSvc := getOrderService(r)
	if orderSvc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Order service is not available")
		return
	}

	var req contracts.QuoteCheckoutSupplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	if len(req.Items) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "At least one item is required")
		return
	}

	resp, err := orderSvc.QuoteCheckoutSupply(r.Context(), req)
	if err != nil {
		status, code, message := classifyCheckoutSupplyQuoteError(err)
		response.Error(w, status, code, message)
		return
	}
	response.Success(w, resp)
}
