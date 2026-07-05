package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

const maxWebhookBodySize = 512 * 1024 // 512 KB

// sanitizeProviderError strips API keys, tokens and URLs with credentials
// from provider error messages to produce a safe detail string.
func sanitizeProviderError(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"sk_live_", "sk_test_", "rk_live_", "rk_test_", "pk_live_", "pk_test_"} {
		searchFrom := 0
		for searchFrom < len(msg) {
			idx := strings.Index(msg[searchFrom:], prefix)
			if idx < 0 {
				break
			}
			idx += searchFrom
			end := idx + len(prefix)
			for end < len(msg) && msg[end] != ' ' && msg[end] != '"' && msg[end] != '\'' && msg[end] != ',' {
				end++
			}
			replacement := prefix + "***"
			msg = msg[:idx] + replacement + msg[end:]
			searchFrom = idx + len(replacement)
		}
	}
	return msg
}

func requestWebhookURL(r *http.Request, providerID string) string {
	return fmt.Sprintf("%s/v1/fiat/%s/webhooks", publicRequestOrigin(r), providerID)
}

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
		log.Warningf("Failed to list fiat providers: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load payment providers")
		return
	}

	responsePkg.Success(w, providers)
}

func (g *Gateway) handlePOSTFiatPayment(w http.ResponseWriter, r *http.Request) {
	// Phase PS / B5: programmatic fiat provisioning with canonical paymentCoin is available via
	// POST /v1/orders/{orderID}/payment-session (PaymentSessionService). This route remains the
	// provider-scoped REST surface for Stripe/PayPal SDK compatibility (returns FiatProviderSession).
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := chi.URLParam(r, "providerID")
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
		CancelURL   string `json:"cancelURL"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.OrderID == "" || req.Currency == "" || req.Amount <= 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "orderID, currency (non-empty) and amount (>0) are required")
		return
	}

	orderSvc := getOrderService(r)
	if orderSvc != nil {
		order, orderErr := orderSvc.GetOrder(req.OrderID)
		if orderErr != nil {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Order not found")
			return
		}
		if models.BuyerAwaitingPaymentReadiness(order) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "Payment is not ready yet; waiting for seller to receive the order")
			return
		}
	}

	params := contracts.CreatePaymentParams{
		OrderID:     req.OrderID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		ReturnURL:   req.ReturnURL,
		CancelURL:   req.CancelURL,
	}

	session, err := svc.CreatePayment(r.Context(), providerID, params)
	if err != nil {
		if errors.Is(err, corepayment.ErrRWAPaymentSessionUnsupported) ||
			errors.Is(err, corepayment.ErrOrderExtensionReservation) ||
			errors.Is(err, corepayment.ErrOrderExtensionSettlement) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "This order requires an escrow-backed crypto payment")
			return
		}
		log.Warningf("Fiat payment creation failed for %s: %v", providerID, err)
		responsePkg.ErrorWithDetail(w, http.StatusInternalServerError, responsePkg.CodeProviderError,
			"Payment creation failed. Please try again.",
			fmt.Sprintf("%s create payment: %v", providerID, sanitizeProviderError(err)))
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

	providerID := chi.URLParam(r, "providerID")
	sessionID := chi.URLParam(r, "sessionID")
	if providerID == "" || sessionID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID and sessionID are required")
		return
	}

	result, err := svc.CapturePayment(r.Context(), providerID, sessionID)
	if err != nil {
		log.Warningf("Fiat capture failed for %s/%s: %v", providerID, sessionID, err)
		responsePkg.ErrorWithDetail(w, http.StatusInternalServerError, responsePkg.CodeProviderError,
			"Payment capture failed. Please try again.",
			fmt.Sprintf("%s capture: %v", providerID, sanitizeProviderError(err)))
		return
	}

	responsePkg.Success(w, result)
}

func (g *Gateway) handlePOSTFiatRefund(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := chi.URLParam(r, "providerID")
	paymentID := chi.URLParam(r, "paymentID")
	if providerID == "" || paymentID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID and paymentID are required")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Idempotency-Key header is required")
		return
	}

	var body struct {
		Amount   *int64 `json:"amount"`
		Currency string `json:"currency"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}

	params := contracts.RefundParams{
		PaymentID: paymentID, IdempotencyKey: idempotencyKey,
		Amount: body.Amount, Currency: body.Currency, Reason: body.Reason,
	}

	result, err := svc.RefundPayment(r.Context(), providerID, params)
	if err != nil {
		log.Errorf("fiat refund failed for %s/%s: %v", providerID, paymentID, err)
		if errors.Is(err, contracts.ErrActionIntentConflict) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "Idempotency-Key conflicts with an existing refund intent")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "refund request failed")
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

	providerID := chi.URLParam(r, "providerID")
	paymentID := chi.URLParam(r, "paymentID")
	if providerID == "" || paymentID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID and paymentID are required")
		return
	}

	detail, err := svc.GetPayment(r.Context(), providerID, paymentID)
	if err != nil {
		log.Warningf("Failed to get fiat payment %s/%s: %v", providerID, paymentID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to retrieve payment details")
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

	providerID := chi.URLParam(r, "providerID")
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
		var retryErr *contracts.RetryableError
		if errors.As(err, &retryErr) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryErr.RetryAfter.Seconds())))
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, retryErr.Error())
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

	providerID := chi.URLParam(r, "providerID")
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
		log.Warningf("Failed to get provider status %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to retrieve provider status")
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

	providerID := chi.URLParam(r, "providerID")
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
		log.Warningf("Failed to get provider config %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to load provider configuration")
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

	providerID := chi.URLParam(r, "providerID")
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
	if (providerID == "stripe" || providerID == "paypal") && strings.TrimSpace(input.PublicKey) == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "publicKey is required")
		return
	}

	if err := svc.SaveProviderConfig(providerID, input); err != nil {
		log.Warningf("Failed to save provider config %s: %v", providerID, err)
		responsePkg.ErrorWithDetail(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to save provider configuration",
			sanitizeProviderError(err))
		return
	}

	if input.WebhookSecret == "" {
		webhookURL := requestWebhookURL(r, providerID)
		if _, err := svc.SetupWebhook(r.Context(), providerID, webhookURL); err != nil {
			log.Warningf("auto-webhook setup for %s: %v", providerID, err)
		}
	}

	view, err := svc.GetProviderConfig(providerID)
	if err != nil {
		log.Warningf("Failed to reload provider config %s after save: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Configuration saved but failed to reload")
		return
	}

	responsePkg.Success(w, view)
}

func (g *Gateway) handlePOSTFiatSetupWebhook(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	webhookURL := requestWebhookURL(r, providerID)
	result, err := svc.SetupWebhook(r.Context(), providerID, webhookURL)
	if err != nil {
		log.Warningf("Webhook setup failed for %s: %v", providerID, err)
		responsePkg.ErrorWithDetail(w, http.StatusBadRequest, responsePkg.CodeProviderError,
			"Webhook auto-configuration failed. Please set up the webhook manually.",
			fmt.Sprintf("%s webhook setup: %v", providerID, sanitizeProviderError(err)))
		return
	}

	responsePkg.Success(w, result)
}

func (g *Gateway) handlePOSTFiatProviderVerify(w http.ResponseWriter, r *http.Request) {
	svc, ok := getFiatService(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Fiat payments not available")
		return
	}

	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	if err := svc.VerifyProviderConfig(providerID); err != nil {
		log.Warningf("Provider verification failed for %s: %v", providerID, err)
		responsePkg.ErrorWithDetail(w, http.StatusBadRequest, responsePkg.CodeProviderError,
			"Provider verification failed. Please check your API keys.",
			fmt.Sprintf("%s verification: %v", providerID, sanitizeProviderError(err)))
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

	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "providerID is required")
		return
	}

	if err := svc.DisconnectProvider(r.Context(), providerID); err != nil {
		if errors.Is(err, contracts.ErrActiveOrdersExist) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "Cannot disconnect provider with active orders")
			return
		}
		log.Warningf("Failed to disconnect provider %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to disconnect provider")
		return
	}

	responsePkg.NoContent(w)
}
