// huma_security_consistency_test.go — TD-117 contract guard.
//
// The Huma per-operation `Security` declaration is the OpenAPI contract:
// API consumers consult it to decide which credentials to send. The
// runtime gate `operationAcceptsAPIToken` honours this declaration —
// any operation that omits the apiToken scheme will refuse mbz_ tokens
// at the middleware layer with a 401, even before the scope check runs.
//
// This test catches the failure mode that motivated TD-117: an endpoint
// declares `nodeAuthSecurity` (basic + JWT + apiToken) so OpenAPI clients
// happily craft an mbz_ token, send it, and get a confusing 403 from the
// scope-enforcement layer because no `routeScopeMap` entry covers the
// path. Either the spec must be tightened to `adminOnlyAuthSecurity`,
// OR a `routeScopeMap` entry must be added so the token is honoured.
//
// The test enforces the invariant: every operation declaring apiToken in
// its Security MUST have at least one matching prefix in routeScopeMap.
// New endpoints that violate this fail closed — pick adminOnlyAuthSecurity
// (preferred for admin-only writes) or add an explicit scope mapping.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// TestHumaSecurity_RouteScopeConsistency scans every huma operation in
// the full-build OpenAPI spec and asserts that any operation declaring
// the apiToken scheme has a matching entry in routeScopeMap. Without a
// mapping the runtime denies API tokens by default, contradicting the
// OpenAPI contract.
func TestHumaSecurity_RouteScopeConsistency(t *testing.T) {
	r := chi.NewMux()

	cfg := huma.DefaultConfig(nodeHumaAPITitle, nodeHumaAPIVersion)
	cfg.OpenAPIPath = "/v1/openapi"
	cfg.DocsPath = "/v1/docs"
	cfg.SchemasPath = "/v1/schemas"
	installNodeHumaEnvelope(&cfg)
	api := humachi.New(r, cfg)

	g := &Gateway{}
	// Public operations.
	g.registerNodeHumaSmokeRoutes(api)
	g.registerNodeHumaListingPublicOperations(api)
	g.registerNodeHumaMediaPublicOperations(api)
	g.registerNodeHumaProfilePublicOperations(api)
	g.registerNodeHumaDiscountPublicOperations(api)
	g.registerNodeHumaCollectionPublicOperations(api)
	g.registerNodeHumaSystemPublicOperations(api)
	g.registerNodeHumaMiscPublicOperations(api)
	g.registerNodeHumaSocialPublicOperations(api)
	g.registerNodeHumaOrderPublicOperations(api)
	g.registerNodeHumaFiatPublicOperations(api)
	g.registerNodeHumaFulfillmentPublicOperations(api)
	g.registerNodeHumaSettingsPublicOperations(api)
	g.registerNodeHumaAuthPublicOperations(api)
	// Admin operations.
	g.registerNodeHumaListingAdminOperations(api)
	g.registerNodeHumaMediaAdminOperations(api)
	g.registerNodeHumaProfileAdminOperations(api)
	g.registerNodeHumaDiscountAdminOperations(api)
	g.registerNodeHumaCollectionAdminOperations(api)
	g.registerNodeHumaSystemAdminOperations(api)
	g.registerNodeHumaMiscAdminOperations(api)
	g.registerNodeHumaSocialAdminOperations(api)
	g.registerNodeHumaOrderAdminOperations(api)
	g.registerNodeHumaFiatAdminOperations(api)
	g.registerNodeHumaFulfillmentAdminOperations(api)
	g.registerNodeHumaSettingsAdminOperations(api)
	g.registerNodeHumaAuthAdminOperations(api)
	g.registerNodeHumaWalletOperations(api)
	g.registerNodeHumaChatOperations(api)
	g.registerNodeHumaDisputeOperations(api)
	g.registerNodeHumaCartOperations(api)
	g.registerNodeHumaNotificationOperations(api)
	g.registerNodeHumaWebhookOperations(api)
	g.registerAIHTTPCapabilities(api)
	g.registerNodeHumaShippingOperations(api)

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
			"%d Huma operation(s) declare apiToken in Security but have no matching "+
				"routeScopeMap entry. Either:\n"+
				"  (a) switch the operation's Security to adminOnlyAuthSecurity "+
				"(preferred for admin-only writes — matches the runtime which already "+
				"denies tokens for these paths via deny-by-default in matchRouteScope),\n"+
				"  (b) add an explicit routeScope entry granting an appropriate scope.\n\n"+
				"Drifted operations:\n%s",
			len(drifts),
			strings.Join(lines, "\n"),
		)
	}
}

// helpers (securityIncludesAPIToken / routeScopeMapCovers) live in
// huma_security_consistency_helpers_test.go so each distribution-profile
// snapshot can share them.
