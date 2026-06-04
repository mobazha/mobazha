//go:build !private_distribution

package api

import (
	"net/http"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

func TestListingSupplySummaryRequiresListingsReadScope(t *testing.T) {
	result := matchRouteScope(http.MethodPost, "/v1/listings/supply-summary", &AuthIdentity{
		Scopes: contracts.NewScopeSet([]contracts.Scope{contracts.ScopeListingsRead}),
	})
	if !result.Allowed {
		t.Fatalf("listings:read should allow supply summary, got denied: %s", result.DenyMsg)
	}
	if result.Scope != contracts.ScopeListingsRead {
		t.Fatalf("scope = %s, want %s", result.Scope, contracts.ScopeListingsRead)
	}
}
