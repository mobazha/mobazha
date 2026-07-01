package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
)

type aiStatusTestIdentity struct{}

func (aiStatusTestIdentity) GetNodeID() string  { return "test-node" }
func (aiStatusTestIdentity) Identity() peer.ID  { return "" }
func (aiStatusTestIdentity) UsingTestnet() bool { return true }
func (aiStatusTestIdentity) SignMessage([]byte) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (aiStatusTestIdentity) IsGlobalBanned(peer.ID) bool { return false }

type aiStatusTestNode struct {
	contracts.NodeService
	identity contracts.IdentityService
	mc       aipkg.MultiConfig
	profile  aipkg.PlatformProfile
}

func newAIStatusTestNode(mc aipkg.MultiConfig, profile aipkg.PlatformProfile) *aiStatusTestNode {
	return &aiStatusTestNode{
		identity: aiStatusTestIdentity{},
		mc:       mc,
		profile:  profile,
	}
}

func (n *aiStatusTestNode) IdentityInfo() contracts.IdentityService { return n.identity }
func (n *aiStatusTestNode) AIConfig() aipkg.Config {
	cfg, _ := n.AIConfigForChat(nil)
	return cfg
}
func (n *aiStatusTestNode) AIConfigForGenerate(req aipkg.GenerateRequest) (aipkg.Config, error) {
	return n.profile.ForGenerate(n.mc.ActiveConfig(), req)
}
func (n *aiStatusTestNode) AIConfigForChat(messages []aipkg.ChatMsg) (aipkg.Config, error) {
	return n.profile.ForChat(n.mc.ActiveConfig(), messages)
}
func (n *aiStatusTestNode) AIMultiConfig() aipkg.MultiConfig             { return n.mc }
func (n *aiStatusTestNode) SaveAIMultiConfig(mc aipkg.MultiConfig) error { n.mc = mc; return nil }
func (n *aiStatusTestNode) AIProxy() *aipkg.Proxy                        { return nil }
func (n *aiStatusTestNode) AIRateLimiter() *aipkg.DailyRateLimiter       { return nil }
func (n *aiStatusTestNode) PlatformAIConfig() *aipkg.Config {
	if n.profile.TextAvailable() {
		return n.profile.Text
	}
	if n.profile.VisionAvailable() {
		return n.profile.Vision
	}
	return nil
}
func (n *aiStatusTestNode) PlatformAIProfile() aipkg.PlatformProfile { return n.profile }

func newAIStatusRequest(t *testing.T, node contracts.NodeService) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/ai/status", strings.NewReader(""))
	return req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
}

func TestHandleGETAIStatus_PlatformTextVisionAvailability(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, true, true, true)}
	text := &aipkg.Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true, DailyLimit: 50}
	vision := &aipkg.Config{Provider: "qwen", APIKey: "vision-key", Model: "qwen3-vl-flash", Enabled: true, IsPlatform: true, DailyLimit: 50}
	node := newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{Text: text, Vision: vision})

	rr := httptest.NewRecorder()
	g.handleGETAIStatus(rr, newAIStatusRequest(t, node))

	resp := decodeAIStatusResponse(t, rr)
	if !resp.Available || resp.Source != "platform" || resp.BYOKConfigured {
		t.Fatalf("unexpected platform status: %+v", resp)
	}
	if !resp.TextAvailable || !resp.VisionAvailable {
		t.Fatalf("expected text and vision routes available: %+v", resp)
	}
	if resp.DailyLimit != 50 {
		t.Fatalf("expected daily limit 50, got %d", resp.DailyLimit)
	}
}

func TestHandleGETAIStatus_CommunityIgnoresPlatformFallback(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, true, false, true)}
	text := &aipkg.Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true}
	node := newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{Text: text})

	rr := httptest.NewRecorder()
	g.handleGETAIStatus(rr, newAIStatusRequest(t, node))

	resp := decodeAIStatusResponse(t, rr)
	if resp.Available || resp.Source != "none" {
		t.Fatalf("community policy exposed platform fallback: %+v", resp)
	}
}

func TestHandleGETAIStatus_BYOKNonVisionOverridesPlatformVision(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, true, true, true)}
	vision := &aipkg.Config{Provider: "qwen", APIKey: "vision-key", Model: "qwen3-vl-flash", Enabled: true, IsPlatform: true}
	node := newAIStatusTestNode(aipkg.MultiConfig{
		Enabled:        true,
		ActiveProvider: "deepseek",
		Providers: map[string]aipkg.ProviderCredential{
			"deepseek": {APIKey: "user-key", Model: "deepseek-v4-flash"},
		},
	}, aipkg.PlatformProfile{Vision: vision})

	rr := httptest.NewRecorder()
	g.handleGETAIStatus(rr, newAIStatusRequest(t, node))

	resp := decodeAIStatusResponse(t, rr)
	if !resp.Available || resp.Source != "byok" || !resp.BYOKConfigured {
		t.Fatalf("unexpected BYOK status: %+v", resp)
	}
	if !resp.TextAvailable {
		t.Fatalf("expected BYOK text route available: %+v", resp)
	}
	if resp.VisionAvailable {
		t.Fatalf("expected BYOK non-vision model to hide vision even when platform vision exists: %+v", resp)
	}
}

func TestHandleGETAIStatus_BYOKVisionModelReportsVision(t *testing.T) {
	g := &Gateway{aiHTTPPolicy: distribution.NewAIHTTPPolicy(true, true, true, true)}
	node := newAIStatusTestNode(aipkg.MultiConfig{
		Enabled:        true,
		ActiveProvider: "qwen",
		Providers: map[string]aipkg.ProviderCredential{
			"qwen": {APIKey: "user-key", Model: "qwen3-vl-flash"},
		},
	}, aipkg.PlatformProfile{})

	rr := httptest.NewRecorder()
	g.handleGETAIStatus(rr, newAIStatusRequest(t, node))

	resp := decodeAIStatusResponse(t, rr)
	if !resp.Available || resp.Source != "byok" || !resp.BYOKConfigured {
		t.Fatalf("unexpected BYOK status: %+v", resp)
	}
	if !resp.TextAvailable || !resp.VisionAvailable {
		t.Fatalf("expected BYOK vision model to report both text and vision: %+v", resp)
	}
}

type aiStatusResponse struct {
	Available       bool   `json:"available"`
	Source          string `json:"source"`
	DailyLimit      int    `json:"daily_limit"`
	DailyUsed       int    `json:"daily_used"`
	BYOKConfigured  bool   `json:"byok_configured"`
	TextAvailable   bool   `json:"text_available"`
	VisionAvailable bool   `json:"vision_available"`
}

func decodeAIStatusResponse(t *testing.T, rr *httptest.ResponseRecorder) aiStatusResponse {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data aiStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.Data
}
