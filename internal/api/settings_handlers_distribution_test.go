package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/deploy"
)

func TestHandleGETExchangeRatesPrivateDistributionDoesNotRequireProvider(t *testing.T) {
	defer deploy.SetProcessMode(deploy.Standalone)
	deploy.SetProcessMode(deploy.PrivateDistribution)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/exchange-rates", nil)
	new(Gateway).handleGETExchangeRates(recorder, request)

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
		t.Fatalf("PrivateDistribution rates = %v, want empty map", response.Data)
	}
}
