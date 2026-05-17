//go:build !private_distribution

package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	corePmt "github.com/mobazha/mobazha3.0/internal/core/payment"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
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
//       - awaiting_funds + empty fundingTarget.address → order exists but payment not yet set up;
//         call CreateSession (POST /v1/orders/{orderID}/payment-session) to provision.
//       - awaiting_funds + non-empty fundingTarget.address → payment provisioned, awaiting incoming funds.
//       - other statuses → see SessionStatus enum documentation.
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
