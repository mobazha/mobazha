package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

const poolOpsTokenHeader = "X-Pool-Ops-Token"

func (g *Gateway) registerBillingHoldRoutes(r chi.Router) {
	authWrap := func(h http.HandlerFunc) http.Handler {
		return g.AuthenticationMiddleware(g.ScopeEnforcementMiddleware(h))
	}
	r.Method(http.MethodGet, "/v1/system/billing-hold", authWrap(g.handleGETBillingHold))
	// PUT is pool-operator only — seller admin Basic Auth must not clear billing hold.
	r.Method(http.MethodPut, "/v1/system/billing-hold", g.poolOpsTokenMiddleware(g.handlePUTBillingHold))
}

func (g *Gateway) poolOpsTokenMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		want := os.Getenv("MOBAZHA_POOL_OPS_TOKEN")
		if want == "" {
			response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
				"pool ops token not configured")
			return
		}
		got := r.Header.Get(poolOpsTokenHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			response.Error(w, http.StatusForbidden, response.CodeForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

// handleGETBillingHold returns current L1 billing grace state.
// GET /v1/system/billing-hold
func (g *Gateway) handleGETBillingHold(w http.ResponseWriter, r *http.Request) {
	node := getNodeService(r)
	prefsSvc := node.Preferences()
	if prefsSvc == nil {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail, "preferences not available")
		return
	}
	prefs, err := prefsSvc.GetPreferences()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to load preferences")
		return
	}
	hold, err := prefs.GetBillingHold()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.Success(w, hold)
}

// handlePUTBillingHold sets or clears L1 billing grace (pool ops host agent only).
// PUT /v1/system/billing-hold  {"active":true,"reason":"grace_expiry"}
// Header: X-Pool-Ops-Token (MOBAZHA_POOL_OPS_TOKEN env — not seller admin password).
func (g *Gateway) handlePUTBillingHold(w http.ResponseWriter, r *http.Request) {
	var body models.BillingHold
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid JSON body")
		return
	}
	node := getNodeService(r)
	prefsSvc := node.Preferences()
	if prefsSvc == nil {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail, "preferences not available")
		return
	}
	if err := prefsSvc.SetBillingHold(body); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.Success(w, body)
}
