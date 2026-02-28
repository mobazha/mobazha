package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

const maxWebhookBodySize = 512 * 1024 // 512 KB

// getFiatService extracts the FiatService from the request's NodeService
// via the FiatPaymentProviderAccessor type assertion. Returns nil when
// the node does not support fiat payments.
func getFiatService(r *http.Request) (contracts.FiatService, bool) {
	ns := getNodeService(r)
	fp, ok := ns.(contracts.FiatPaymentProviderAccessor)
	if !ok {
		return nil, false
	}
	svc := fp.Fiat()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func (g *Gateway) handleGETFiatProviders(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providers, err := svc.EnabledProviders(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, providers)
}

func (g *Gateway) handlePOSTFiatPayment(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	var req struct {
		OrderID     string `json:"orderID"`
		Amount      int64  `json:"amount"`
		Currency    string `json:"currency"`
		Description string `json:"description"`
		ReturnURL   string `json:"returnURL"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.OrderID == "" || req.Currency == "" || req.Amount <= 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "orderID, currency (non-empty) and amount (>0) are required")
		return
	}

	params := contracts.CreatePaymentParams{
		OrderID:     req.OrderID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		ReturnURL:   req.ReturnURL,
	}

	session, err := svc.CreatePayment(r.Context(), providerID, params)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Created(w, session)
}

func (g *Gateway) handlePOSTFiatCapture(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	sessionID := mux.Vars(r)["sessionID"]
	if providerID == "" || sessionID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID and sessionID are required")
		return
	}

	result, err := svc.CapturePayment(r.Context(), providerID, sessionID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, result)
}

func (g *Gateway) handleGETFiatPayment(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	paymentID := mux.Vars(r)["paymentID"]
	if providerID == "" || paymentID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID and paymentID are required")
		return
	}

	detail, err := svc.GetPayment(r.Context(), providerID, paymentID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, detail)
}

func (g *Gateway) handlePOSTFiatWebhook(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "failed to read request body")
		return
	}

	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := svc.HandleWebhook(r.Context(), providerID, payload, headers); err != nil {
		if errors.Is(err, contracts.ErrWebhookSignature) {
			responsePkg.Error(w, http.StatusUnauthorized, responsePkg.CodeUnauthorized, "Invalid webhook signature")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Webhook processing failed")
		return
	}

	responsePkg.NoContent(w)
}

// --- Seller-side Fiat Provider Management ---

func (g *Gateway) handleGETFiatProviderStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	status, err := svc.GetProviderStatus(r.Context(), providerID)
	if err != nil {
		if errors.Is(err, contracts.ErrProviderNotFound) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Provider not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, status)
}

func (g *Gateway) handleGETFiatProviderConfig(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	cfg, err := svc.GetProviderConfig(providerID)
	if err != nil {
		if errors.Is(err, contracts.ErrProviderNotFound) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Provider config not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, cfg)
}

func (g *Gateway) handlePUTFiatProviderConfig(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	var input contracts.ProviderConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if input.SecretKey == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "secretKey is required")
		return
	}

	if err := svc.SaveProviderConfig(providerID, input); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	view, err := svc.GetProviderConfig(providerID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.Success(w, view)
}

func (g *Gateway) handlePOSTFiatProviderVerify(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	if err := svc.VerifyProviderConfig(providerID); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Provider verification failed: "+err.Error())
		return
	}

	responsePkg.Success(w, map[string]bool{"verified": true})
}

func (g *Gateway) handleDELETEFiatProviderConfig(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := mux.Vars(r)["providerID"]
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	if err := svc.DeleteProviderConfig(providerID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.NoContent(w)
}
