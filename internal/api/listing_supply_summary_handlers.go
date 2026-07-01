package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// handlePOSTListingSupplySummary returns seller-safe advisory supply summaries
// for admin product surfaces.
// POST /v1/listings/supply-summary
func (g *Gateway) handlePOSTListingSupplySummary(w http.ResponseWriter, r *http.Request) {
	orderSvc := getOrderService(r)
	if orderSvc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Supply summary is not available")
		return
	}

	var req contracts.ListingSupplySummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	resp, err := orderSvc.SummarizeListingSupply(r.Context(), req)
	if err != nil {
		status, code, message := classifyCheckoutSupplyQuoteError(err)
		response.Error(w, status, code, message)
		return
	}
	response.Success(w, resp)
}
