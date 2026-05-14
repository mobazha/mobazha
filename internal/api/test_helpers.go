//go:build !private_distribution

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	tnet "github.com/libp2p/go-libp2p-testing/net"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type apiTests []apiTest

type apiTest struct {
	name             string
	path             string
	method           string
	body             []byte
	setNodeMethods   func(n *mockNode)
	statusCode       int
	expectedResponse func() ([]byte, error)
}

// wrapErrorMessage returns legacy error format. Kept for tests not yet migrated to Phase G.
func wrapErrorMessage(reason string) string {
	reason = strings.Replace(reason, `"`, `'`, -1)
	result, _ := marshalAndSanitizeJSON(APIError{false, reason})
	return string(result)
}

// wrapPhaseGError returns Phase G error envelope: {"error":{"code":"<CODE>","message":"<msg>"}}\n
func wrapPhaseGError(statusCode int, message string) string {
	code := responsePkg.HttpStatusToCode(statusCode)
	env := map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(env)
	return string(b) + "\n"
}

// wrapDataInEnvelope wraps data in Phase G success envelope: {"data": <raw>}
func wrapDataInEnvelope(data interface{}) ([]byte, error) {
	inner, err := marshalAndSanitizeJSON(data)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(`{"data": `), inner...), '}'), nil
}

// wrapRawJSONInEnvelope wraps pre-marshaled JSON bytes in Phase G envelope.
// Use for protobuf responses where sanitizeProtobuf (protojson) must be used.
func wrapRawJSONInEnvelope(raw []byte) ([]byte, error) {
	return append(append([]byte(`{"data": `), raw...), '}'), nil
}

func runAPITests(t *testing.T, tests apiTests) {
	identity, err := tnet.RandIdentity()
	if err != nil {
		t.Fatal(err)
	}
	node := &mockNode{
		identityFunc: func() peer.ID {
			return identity.ID()
		},
		nodeIDFunc: func() string {
			return identity.ID().String()
		},
	}
	gateway := &Gateway{
		nodeManager: &mockNodeManager{
			nodes: map[string]contracts.NodeService{
				"test_user_id": node,
			},
		},
		config: &GatewayConfig{},
	}
	outer := chi.NewMux()
	// Test-only auth shim: handler-level tests in this file exercise the
	// business logic of owner-only routes, not the auth pipeline. Inject a
	// pre-resolved admin AuthIdentity (just like the SaaS SharedRouter does
	// in production) so nodeHumaAuthMiddleware's "already authenticated"
	// short-circuit (huma_middleware.go) takes effect and individual cases
	// can focus on their own status code expectations. Auth coverage lives
	// in TestGateway_AuthenticationMiddleware / TestGateway_JWTAuth /
	// TestNodeBridgeRequestWithOptionalAuth_*.
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithAuthIdentity(r.Context(), &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	outer.Mount("/", gateway.newV1Router(false, false))

	ts := httptest.NewServer(outer)
	defer ts.Close()

	for _, test := range tests {
		test.setNodeMethods(node)
		req, err := http.NewRequest(test.method, fmt.Sprintf("%s%s", ts.URL, test.path), bytes.NewReader(test.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Mobazha-Node", "test_user_id")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != test.statusCode {
			t.Errorf("%s. Expected status code %d, got %d", test.name, test.statusCode, res.StatusCode)
			continue
		}
		response, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
		expected, err := test.expectedResponse()
		if err != nil {
			log.Fatal(err)
		}
		if expected != nil && !bytes.Equal(response, expected) {
			// Normalize non-breaking spaces (U+00A0) to regular spaces for comparison.
			// protojson error messages use NBSP which differs from regular space in test expectations.
			normalizedExpected := strings.ReplaceAll(string(expected), "\u00a0", " ")
			normalizedResponse := strings.ReplaceAll(string(response), "\u00a0", " ")
			if normalizedExpected == normalizedResponse {
				continue
			}
			// 尝试 JSON 语义比较（忽略空白和 key 顺序差异）
			var expectedJSON, actualJSON interface{}
			if json.Unmarshal([]byte(normalizedExpected), &expectedJSON) == nil && json.Unmarshal([]byte(normalizedResponse), &actualJSON) == nil {
				if !reflect.DeepEqual(expectedJSON, actualJSON) {
					t.Errorf("%s: Expected response %q, got %q", test.name, normalizedExpected, normalizedResponse)
					continue
				}
			} else {
				t.Errorf("%s: Expected response %q, got %q", test.name, normalizedExpected, normalizedResponse)
				continue
			}
		}
	}
}
