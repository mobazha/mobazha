package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/distribution"
)

type restrictedProductSurfacePolicy struct{}

func (restrictedProductSurfacePolicy) ExternalExchangeRatesEnabled() bool { return false }
func (restrictedProductSurfacePolicy) MCPToolCatalog() string {
	return distribution.MCPToolCatalogRestricted
}
func (restrictedProductSurfacePolicy) CoreAPISurface() string {
	return distribution.CoreAPISurfaceRestricted
}

func TestHandleGETExchangeRatesRestrictedPolicyDoesNotRequireProvider(t *testing.T) {

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/exchange-rates", nil)
	(&Gateway{config: &GatewayConfig{
		ProductSurfacePolicy: restrictedProductSurfacePolicy{},
	}}).handleGETExchangeRates(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode rates: %v", err)
	}
	if len(response.Data) != 0 {
		t.Fatalf("restricted rates = %v, want empty map", response.Data)
	}
}
