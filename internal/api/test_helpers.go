package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	tnet "github.com/libp2p/go-libp2p-testing/net"
	peer "github.com/libp2p/go-libp2p/core/peer"
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
			nodes: map[string]coreiface.CoreIface{
				"test_peer_id": node,
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
			t.Errorf("%s: Expected response %s, got %s", test.name, string(expected), string(response))
			continue
		}
	}
}
