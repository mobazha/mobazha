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

	tnet "github.com/libp2p/go-libp2p-testing/net"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
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

func runAPITests(t *testing.T, tests apiTests) {
	identity, err := tnet.RandIdentity()
	if err != nil {
		t.Fatal(err)
	}
	node := &mockNode{
		identityFunc: func() peer.ID {
			return identity.ID()
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
	r := gateway.newV1Router()
	r.Use(gateway.NodeSelectionMiddleware)

	ts := httptest.NewServer(r)
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
