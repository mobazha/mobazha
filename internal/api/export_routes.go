package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerExportRoutes mounts the seller data-portability endpoints
// (DG-1.10). Registered outside Huma because the CSV path writes text/csv
// directly without the JSON data envelope. Authentication and scope checks
// match the digital-asset upload stream.
func (g *Gateway) registerExportRoutes(r chi.Router) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return g.AuthenticationMiddleware(g.ScopeEnforcementMiddleware(h))
	}
	r.Method(http.MethodGet, "/v1/exports/listings", wrap(g.handleExportListings))
	r.Method(http.MethodGet, "/v1/exports/sales", wrap(g.handleExportSales))
	r.Method(http.MethodGet, "/v1/exports/customers", wrap(g.handleExportCustomers))
}
