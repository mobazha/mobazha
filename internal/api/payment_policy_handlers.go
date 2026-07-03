package api

import (
	"encoding/json"
	"net/http"

	"github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/response"
)

type storePaymentPolicyInput struct {
	UtxoConfirmationPolicy string `json:"utxoConfirmationPolicy"`
}

// handleGETStorePaymentPolicy returns store-level payment policy settings.
// GET /v1/settings/payment-policy
func (g *Gateway) handleGETStorePaymentPolicy(w http.ResponseWriter, r *http.Request) {
	db, ok := getNodeDB(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Payment policy settings are not available for this node")
		return
	}

	cfg, err := payment.GetStorePaymentSettings(db)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, map[string]string{
		"utxoConfirmationPolicy": cfg.UtxoConfirmationPolicy,
	})
}

// handlePUTStorePaymentPolicy updates store-level payment policy settings.
// PUT /v1/settings/payment-policy
func (g *Gateway) handlePUTStorePaymentPolicy(w http.ResponseWriter, r *http.Request) {
	db, ok := getNodeDB(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Payment policy settings are not available for this node")
		return
	}

	var input storePaymentPolicyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"Invalid request body: "+err.Error())
		return
	}

	if err := models.ValidatePaymentConfirmationPolicy(input.UtxoConfirmationPolicy); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	policy := models.NormalizePaymentConfirmationPolicy(input.UtxoConfirmationPolicy)
	cfg, err := payment.SaveStorePaymentSettings(db, policy)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, map[string]string{
		"utxoConfirmationPolicy": cfg.UtxoConfirmationPolicy,
	})
}
