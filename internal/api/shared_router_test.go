//go:build !private_distribution

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

type stubNodeService struct {
	contracts.NodeService
}

func TestNewSharedRouter_InjectsSkillProvider(t *testing.T) {
	provider := agentskill.NewFSProvider(fstest.MapFS{
		"seller/default/SKILL.md": {Data: []byte("---\nname: default\npersona: seller\n---\nbody")},
	})
	sr, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(r *http.Request) (contracts.NodeService, error) {
			return &stubNodeService{}, nil
		},
		SkillProvider: provider,
	})
	if err != nil {
		t.Fatalf("NewSharedRouter: %v", err)
	}
	if sr.gateway.config.SkillProvider != provider {
		t.Fatal("expected shared router to retain injected skill provider")
	}
}

func TestSharedRouter_ResolverInjectsNode(t *testing.T) {
	mock := &stubNodeService{}
	sr, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(r *http.Request) (contracts.NodeService, error) {
			return mock, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSharedRouter: %v", err)
	}

	// /v1/peers uses getCoreIface → (nil, false) for stub → 501.
	// The key assertion: we do NOT get 401 (resolver succeeded).
	req := httptest.NewRequest("GET", "/v1/peers", nil)
	rr := httptest.NewRecorder()
	sr.ServeHTTP(rr, req)

	if rr.Code == http.StatusUnauthorized {
		t.Fatal("resolver should have injected node into context, got 401")
	}
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 (stub is not CoreIface), got %d", rr.Code)
	}
}

func TestSharedRouter_ResolverError_Returns401(t *testing.T) {
	sr, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(r *http.Request) (contracts.NodeService, error) {
			return nil, errors.New("auth failed")
		},
	})
	if err != nil {
		t.Fatalf("NewSharedRouter: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/peers", nil)
	rr := httptest.NewRecorder()
	sr.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when resolver fails, got %d", rr.Code)
	}
}

func TestSharedRouter_CORS_WhenEnabled(t *testing.T) {
	sr, err := NewSharedRouter(SharedRouterConfig{
		Resolver: func(r *http.Request) (contracts.NodeService, error) {
			return &stubNodeService{}, nil
		},
		AllowCORS: true,
	})
	if err != nil {
		t.Fatalf("NewSharedRouter: %v", err)
	}

	req := httptest.NewRequest("OPTIONS", "/v1/peers", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	sr.ServeHTTP(rr, req)

	if v := rr.Header().Get("Access-Control-Allow-Origin"); v == "" {
		t.Error("expected CORS allow-origin header when AllowCORS=true")
	}
}
