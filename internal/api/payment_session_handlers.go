//go:build !private_distribution

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	corePmt "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// handleGETOrderPaymentSession returns the unified payment session view for a given order.
//
// GET /v1/orders/{orderID}/payment-session
//
// The session is built by PaymentSessionProjector from existing order, payment, and
// fiat metadata — no new DB table is required (Phase B Step 1 projection-first design).
//
// Returns:
//   - 200 with PaymentSession JSON when the order exists, regardless of provisioning state.
//     Clients MUST inspect session.Status to distinguish:
//   - awaiting_funds + empty fundingTarget.address → order exists but payment not yet set up;
//     call CreateSession (POST /v1/orders/{orderID}/payment-session) to provision.
//   - awaiting_funds + non-empty fundingTarget.address → payment provisioned, awaiting incoming funds.
//   - other statuses → see SessionStatus enum documentation.
//   - 404 if the order record itself does not exist in the database.
//   - 503 if the PaymentSession subsystem is not available on this build.
func (g *Gateway) handleGETOrderPaymentSession(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	svc := getNodeService(r).PaymentSession()
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"payment session subsystem not available")
		return
	}

	session, err := svc.GetSession(r.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"order not found: "+orderID)
		case errors.Is(err, corePmt.ErrProvisioningNotImplemented):
			// Phase B Step 1: provisioning not yet wired — should not happen on GET,
			// but guard defensively.
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
				"payment session provisioning not yet available — use the existing payment initialisation path")
		default:
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		}
		return
	}

	responsePkg.Success(w, session)
}

// handlePOSTOrderPaymentSession provisions (or idempotently re-reads) the unified
// payment session for an order.
//
// POST /v1/orders/{orderID}/payment-session
//
// Body JSON (camelCase):
//   - paymentCoin (required): canonical coin after ingress normalization;
//     legacy native tickers are accepted and normalized server-side.
//   - refundAddress (optional for crypto); payerAddress / moderator forwarded to escrow setup where applicable.
//   - fiatAmountCents, fiatDescription, fiatReturnURL, fiatCancelURL: required for
//     fiat:{provider}:{currency} when provisioning a new provider checkout session.
//
// Phase PS / B5: primary programmatic alternative to POST /v1/fiat/{providerID}/payments
// for clients that already use canonical paymentCoin.
func (g *Gateway) handlePOSTOrderPaymentSession(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "orderID is required")
		return
	}

	svc := getNodeService(r).PaymentSession()
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
			"payment session subsystem not available")
		return
	}

	var payload struct {
		PaymentCoin     string `json:"paymentCoin"`
		RefundAddress   string `json:"refundAddress"`
		BuyerPeerID     string `json:"buyerPeerID"`
		PayerAddress    string `json:"payerAddress"`
		Moderator       string `json:"moderator"`
		FiatAmountCents int64  `json:"fiatAmountCents"`
		FiatDescription string `json:"fiatDescription"`
		FiatReturnURL   string `json:"fiatReturnURL"`
		FiatCancelURL   string `json:"fiatCancelURL"`
	}
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&payload); err != nil {
			if !errors.Is(err, io.EOF) {
				responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
				return
			}
		}
		var trailing json.RawMessage
		if err := dec.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
			return
		}
		if len(trailing) > 0 {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "unexpected trailing JSON in request body")
			return
		}
	}

	if strings.TrimSpace(payload.PaymentCoin) == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "paymentCoin is required")
		return
	}

	normalizedCoin, err := iwallet.NormalizePaymentCoinIngress(payload.PaymentCoin)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}

	req := contracts.CreatePaymentSessionRequest{
		OrderID:         orderID,
		PaymentCoin:     string(normalizedCoin),
		RefundAddress:   payload.RefundAddress,
		BuyerPeerID:     payload.BuyerPeerID,
		PayerAddress:    payload.PayerAddress,
		Moderator:       payload.Moderator,
		FiatAmountCents: payload.FiatAmountCents,
		FiatDescription: payload.FiatDescription,
		FiatReturnURL:   payload.FiatReturnURL,
		FiatCancelURL:   payload.FiatCancelURL,
	}

	if strings.HasPrefix(strings.ToLower(req.PaymentCoin), "fiat:") && req.FiatAmountCents <= 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation,
			"fiatAmountCents must be > 0 when paymentCoin is a fiat canonical id")
		return
	}

	session, err := svc.CreateSession(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"order not found: "+orderID)
		case errors.Is(err, corePmt.ErrProvisioningNotImplemented):
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
				"crypto payment session provisioning is not yet available on this node — use POST /v1/orders/{orderID}/instructions/payment")
		case errors.Is(err, corePmt.ErrFiatFacadeNotWired):
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
				"fiat payment session provisioning is not available on this node")
		case errors.Is(err, corePmt.ErrInvalidFiatAmountCents):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		case errors.Is(err, coreiface.ErrCoinSwitchRequiresConfirmation):
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict,
				"cannot switch coin with existing partial payment")
		case errors.Is(err, corePmt.ErrPaymentCoinMismatch):
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, err.Error())
		case errors.Is(err, corePmt.ErrExchangeRateUnavailable):
			responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail,
				"exchange rate service unavailable — cross-currency crypto payment cannot be calculated")
		case errors.Is(err, corePmt.ErrRWAPaymentUseLegacyInstructions):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		case errors.Is(err, coreiface.ErrBadRequest):
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		default:
			msg := err.Error()
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, msg)
		}
		return
	}

	responsePkg.Success(w, session)
}
