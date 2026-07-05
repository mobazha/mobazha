package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestHumaSecurity_RouteScopeConsistency_Sovereign is the sovereign-build
// counterpart of the same test in huma_security_consistency_test.go.
// Sovereign ships a stripped operation surface (no chat/fiat/fulfillment
// etc.) plus a few sovereign-only endpoints (Monero NodePool, XMR wallet),
// so we re-run the consistency invariant against the registered set.
//
// The invariant is: every Huma operation declaring apiToken in Security
// MUST have at least one matching prefix in routeScopeMap. Otherwise
// an API token holder gets a misleading 403 from the deny-by-default
// branch in matchRouteScope, contradicting the OpenAPI contract. See
// huma_security_consistency_test.go for full background.
func TestHumaSecurity_RouteScopeConsistency_Sovereign(t *testing.T) {
	r := chi.NewMux()
	g := &Gateway{config: &GatewayConfig{ProductSurfacePolicy: restrictedProductSurfacePolicy{}}}
	mustRegisterHumaAPI(t, g, r)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/openapi.json returned %d, want 200", rr.Code)
	}

	type opMeta struct {
		OperationID string                `json:"operationId"`
		Security    []map[string][]string `json:"security"`
	}
	var spec struct {
		Paths map[string]map[string]opMeta `json:"paths"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("Failed to unmarshal OpenAPI spec: %v", err)
	}

	type drift struct {
		opID   string
		method string
		path   string
	}
	var drifts []drift

	for path, methods := range spec.Paths {
		for method, op := range methods {
			methodUpper := strings.ToUpper(method)
			if !securityIncludesAPIToken(op.Security) {
				continue
			}
			if !routeScopeMapCovers(methodUpper, path) {
				drifts = append(drifts, drift{
					opID:   op.OperationID,
					method: methodUpper,
					path:   path,
				})
			}
		}
	}

	if len(drifts) > 0 {
		sort.Slice(drifts, func(i, j int) bool {
			return drifts[i].opID < drifts[j].opID
		})
		var lines []string
		for _, d := range drifts {
			lines = append(lines, "  - "+d.opID+" ("+d.method+" "+d.path+")")
		}
		t.Errorf(
			"%d sovereign Huma operation(s) declare apiToken in Security but "+
				"have no matching routeScopeMap entry. Either switch to "+
				"adminOnlyAuthSecurity or add an explicit routeScope. "+
				"Drifted operations:\n%s",
			len(drifts),
			strings.Join(lines, "\n"),
		)
	}
}
