package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type localPolicyAITestNode struct {
	contracts.NodeService
	mc    aipkg.MultiConfig
	proxy *aipkg.Proxy
}

func (n *localPolicyAITestNode) AIConfig() aipkg.Config                 { return n.mc.ActiveConfig() }
func (n *localPolicyAITestNode) AIMultiConfig() aipkg.MultiConfig       { return n.mc }
func (n *localPolicyAITestNode) AIProxy() *aipkg.Proxy                  { return n.proxy }
func (n *localPolicyAITestNode) AIRateLimiter() *aipkg.DailyRateLimiter { return nil }
func (n *localPolicyAITestNode) PlatformAIConfig() *aipkg.Config        { return nil }
func (n *localPolicyAITestNode) SaveAIMultiConfig(mc aipkg.MultiConfig) error {
	n.mc = mc
	return nil
}

func newPrivateDistributionAIRequest(t *testing.T, method, target, body string, node contracts.NodeService) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), nodeContextKey, node)
	return req.WithContext(ctx)
}

func TestHandlePUTAIConfig_AllowsTrustedPlainHTTP(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, false, false, false)}
	node := &localPolicyAITestNode{}
	req := newPrivateDistributionAIRequest(t, http.MethodPut, "/v1/settings/ai", `{
		"provider":"custom",
		"base_url":"http://ollama:11434/v1",
		"model":"llama3.2",
		"enabled":true
	}`, node)
	rr := httptest.NewRecorder()

	g.handlePUTAIConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := node.mc.ActiveConfig().BaseURL; got != "http://ollama:11434/v1" {
		t.Fatalf("expected saved base URL, got %q", got)
	}
}

func TestHandlePUTAIConfig_RejectsRemoteEndpoint(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, false, false, false)}
	node := &localPolicyAITestNode{}
	req := newPrivateDistributionAIRequest(t, http.MethodPut, "/v1/settings/ai", `{
		"provider":"openai",
		"base_url":"https://api.openai.com/v1",
		"model":"gpt-4o",
		"enabled":true
	}`, node)
	rr := httptest.NewRecorder()

	g.handlePUTAIConfig(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGETAIStatus_AllowsDockerInternalNoKey(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, false, false, false)}
	node := &localPolicyAITestNode{
		mc: aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {
					Model:   "llama3.2:1b",
					BaseURL: "http://ollama:11434/v1",
				},
			},
		},
		proxy: aipkg.NewProxy(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"capabilities":["completion"]}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}),
	}
	req := newPrivateDistributionAIRequest(t, http.MethodGet, "/v1/ai/status", "", node)
	rr := httptest.NewRecorder()

	g.handleGETAIStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data struct {
			Available      bool   `json:"available"`
			Source         string `json:"source"`
			BYOKConfigured bool   `json:"byok_configured"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Data.Available || resp.Data.Source != "byok" || !resp.Data.BYOKConfigured {
		t.Fatalf("unexpected status response: %+v", resp.Data)
	}
}

func TestHandlePOSTAITestConnection_AllowsLocalNoKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Fatalf("expected no authorization header, got %q", auth)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hi"}},
			},
		})
	}))
	defer server.Close()

	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, false, false, false)}
	node := &localPolicyAITestNode{proxy: aipkg.NewProxy(server.Client())}
	req := newPrivateDistributionAIRequest(t, http.MethodPost, "/v1/settings/ai/test", `{
		"provider":"custom",
		"base_url":"`+server.URL+`",
		"model":"llama3.2"
	}`, node)
	rr := httptest.NewRecorder()

	g.handlePOSTAITestConnection(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
