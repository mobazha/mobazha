package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestGateway_AuthenticationMiddleware(t *testing.T) {
	gateway := &Gateway{
		nodeManager: &mockNodeManager{
			nodes: map[string]contracts.NodeService{
				"test_peer_id": &mockNode{
					getMyProfileFunc: func() (*models.Profile, error) { return nil, nil },
				},
			},
		},
		config: &GatewayConfig{},
	}

	r := gateway.newV1Router()
	r.Use(gateway.AuthenticationMiddleware)
	r.Use(gateway.NodeSelectionMiddleware)

	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		config    *GatewayConfig
		setup     func(req *http.Request)
		forbidden bool
	}{
		{
			config: &GatewayConfig{
				AllowedIPs: map[string]bool{
					"127.0.0.1": true,
				},
			},
			forbidden: false,
		},
		{
			config: &GatewayConfig{
				AllowedIPs: map[string]bool{
					"197.2.18.3": true,
				},
			},
			forbidden: true,
		},
		{
			config: &GatewayConfig{
				Cookie: "cookie_monster",
			},
			setup: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  AuthCookieName,
					Value: "cookie_monster",
				})
			},
			forbidden: false,
		},
		{
			config: &GatewayConfig{
				Cookie: "cookie_monster",
			},
			setup: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  AuthCookieName,
					Value: "asdfasdf",
				})
			},
			forbidden: true,
		},
		{
			config: &GatewayConfig{
				Username: "alice",
				Password: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
			},
			setup: func(req *http.Request) {
				req.SetBasicAuth("alice", "letmein")
			},
			forbidden: false,
		},
		{
			config: &GatewayConfig{
				Username: "alice",
				Password: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
			},
			setup: func(req *http.Request) {
				req.SetBasicAuth("alice", "asdf")
			},
			forbidden: true,
		},
	}
	for i, test := range tests {
		gateway.config = test.config
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/ob/profile", ts.URL), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Mobazha-Node", "test_user_id")
		if test.setup != nil {
			test.setup(req)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if test.forbidden && resp.StatusCode != http.StatusForbidden {
			t.Errorf("Test %d: expected status forbidden, got %d", i, resp.StatusCode)
			continue
		}
		if !test.forbidden && resp.StatusCode == http.StatusForbidden {
			t.Errorf("Test %d: unexpected forbidden status", i)
			continue
		}
	}
}
