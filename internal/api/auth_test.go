//go:build !private_distribution

package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
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

	outer := chi.NewMux()
	outer.Use(gateway.AuthenticationMiddleware)
	outer.Mount("/", gateway.newV1Router(false, false))

	ts := httptest.NewServer(outer)
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
		gateway.auth = authState{
			username:     test.config.Username,
			passwordHash: test.config.Password,
		}
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/profiles", ts.URL), nil)
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
		isRejected := resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized
		if test.forbidden && !isRejected {
			t.Errorf("Test %d: expected 401/403, got %d", i, resp.StatusCode)
			continue
		}
		if !test.forbidden && isRejected {
			t.Errorf("Test %d: unexpected rejection status %d", i, resp.StatusCode)
			continue
		}
	}
}

func TestGateway_JWTAuth(t *testing.T) {
	const localPeerID = "QmLocalPeerID1234567890ABCDEF1234567890"

	certPEM, privKey := generateTestRSACert()

	validator, err := NewJWTValidator(certPEM, localPeerID, "")
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	gateway := &Gateway{
		nodeManager: &mockNodeManager{
			nodes: map[string]contracts.NodeService{
				"test_peer_id": &mockNode{
					getMyProfileFunc: func() (*models.Profile, error) { return nil, nil },
				},
			},
		},
		config:       &GatewayConfig{},
		jwtValidator: validator,
	}

	outer := chi.NewMux()
	outer.Use(gateway.AuthenticationMiddleware)
	outer.Mount("/", gateway.newV1Router(false, false))
	ts := httptest.NewServer(outer)
	defer ts.Close()

	t.Run("ValidJWT_AdminPeerID", func(t *testing.T) {
		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Name:       "seller1",
			Properties: map[string]string{"peerID": localPeerID},
		}, privKey)

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("Expected auth success, got %d", resp.StatusCode)
		}
	})

	t.Run("ValidJWT_WrongPeerID_FallsThrough", func(t *testing.T) {
		// Configure Basic Auth so the fallthrough triggers 401 instead of passthrough
		gateway.auth = authState{
			username:     "admin",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		defer func() { gateway.auth = authState{} }()

		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Name:       "another-user",
			Properties: map[string]string{"peerID": "QmWrongPeerIDDoesNotMatchLocalNode00000"},
		}, privKey)

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		// JWT valid but peerID mismatch → not admin → falls to Basic Auth → 401
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 (JWT peerID mismatch, falls to Basic Auth), got %d", resp.StatusCode)
		}
	})

	t.Run("ExpiredJWT_FallsThrough", func(t *testing.T) {
		gateway.auth = authState{
			username:     "admin",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		defer func() { gateway.auth = authState{} }()

		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
			Name:       "seller1",
			Properties: map[string]string{"peerID": localPeerID},
		}, privKey)

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for expired JWT (falls to Basic Auth), got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidBearerToken_FallsThrough", func(t *testing.T) {
		gateway.auth = authState{
			username:     "admin",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		defer func() { gateway.auth = authState{} }()

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for invalid Bearer (falls to Basic Auth), got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidBearerToken_RateLimited", func(t *testing.T) {
		gateway.auth = authState{
			username:     "admin",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		gateway.authLimiter = newAuthRateLimiter()
		defer func() {
			gateway.auth = authState{}
			gateway.authLimiter.stop()
			gateway.authLimiter = nil
		}()

		var resp *http.Response
		for i := 0; i < authRateLimitMaxFailures; i++ {
			req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
			req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
			req.Header.Set("X-Mobazha-Node", "test_user_id")

			var err error
			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("invalid bearer request %d: %v", i, err)
			}
			resp.Body.Close()
		}
		if resp == nil {
			t.Fatal("expected response from invalid Bearer request")
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected invalid Bearer attempts to hit auth rate limiter, got %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Retry-After"); got != "900" {
			t.Fatalf("Retry-After = %q, want 900", got)
		}
	})

	t.Run("NoValidator_JWTIgnored", func(t *testing.T) {
		gateway.jwtValidator = nil
		defer func() { gateway.jwtValidator = validator }()

		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			},
			Properties: map[string]string{"peerID": localPeerID},
		}, privKey)

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		// No validator → JWT ignored → no Basic Auth configured → passes through
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("Should not be forbidden when validator is nil, got %d", resp.StatusCode)
		}
	})

	t.Run("CorrectBasicAuthSucceedsAfterPriorFailures", func(t *testing.T) {
		// Regression: in Docker / NAT deployments multiple clients share
		// one source IP. The previous design banned the IP up front, so
		// even the real operator's correct password was rejected with
		// 429 once any (e.g. SPA-init) request had 401'd 5 times. New
		// behaviour: correct credentials always reset the counter.
		// passwordHash below is sha256("letmein").
		gateway.auth = authState{
			username:     "alice",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		gateway.authLimiter = newAuthRateLimiter()
		defer func() {
			gateway.auth = authState{}
			gateway.authLimiter.stop()
			gateway.authLimiter = nil
		}()

		// Burn through the failure budget with the wrong password.
		for i := 0; i < authRateLimitMaxFailures; i++ {
			req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
			req.Header.Set("X-Mobazha-Node", "test_user_id")
			req.SetBasicAuth("alice", "wrong-password")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("priming failure %d: %v", i, err)
			}
			resp.Body.Close()
		}

		// Wrong password again -> now 429 (limiter has tipped).
		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("X-Mobazha-Node", "test_user_id")
		req.SetBasicAuth("alice", "wrong-password")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post-budget wrong-cred request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429 once budget burned, got %d", resp.StatusCode)
		}

		// Correct password should now succeed and reset the counter,
		// even though the IP was technically "blocked". This is the
		// key behavioural change.
		req, _ = http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("X-Mobazha-Node", "test_user_id")
		req.SetBasicAuth("alice", "letmein")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("correct-cred request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("correct credentials should pass even after prior failures, got %d", resp.StatusCode)
		}

		// And a second correct request right after also succeeds —
		// proves resetIP actually fired.
		req, _ = http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("X-Mobazha-Node", "test_user_id")
		req.SetBasicAuth("alice", "letmein")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("follow-up correct-cred request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			// 404 is acceptable because the mockNode has no real profile.
			t.Fatalf("follow-up correct-cred request unexpectedly rejected: %d", resp.StatusCode)
		}
	})

	t.Run("ValidJWT_WebSocketProtocol", func(t *testing.T) {
		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Properties: map[string]string{"peerID": localPeerID},
		}, privKey)

		encoded := base64.RawURLEncoding.EncodeToString([]byte(token))
		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Sec-WebSocket-Protocol", "mbz.auth.v1, mbz.auth.b64."+encoded)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("Expected auth success via WebSocket protocol, got %d", resp.StatusCode)
		}
	})

	t.Run("ValidJWT_AdminPeerID_WithBasicAuthConfigured", func(t *testing.T) {
		gateway.auth = authState{
			username:     "alice",
			passwordHash: "1c8bfe8f801d79745c4631d09fff36c82aa37fc4cce4fc946683d7b336b63032",
		}
		defer func() { gateway.auth = authState{} }()

		token := signToken(&JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Properties: map[string]string{"peerID": localPeerID},
		}, privKey)

		req, _ := http.NewRequest("GET", ts.URL+"/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Mobazha-Node", "test_user_id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		// JWT admin match should succeed even when Basic Auth is also configured
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("JWT admin should bypass Basic Auth, got %d", resp.StatusCode)
		}
	})
}
