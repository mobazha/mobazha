package relay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPlatformRelay_ChainTypeFromID_UsesStatusEvmChains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/relay/status" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"data":{"enabled":true,"evmChains":[{"chainType":"weird_key","chainId":999999}]}}`)
	}))
	defer srv.Close()

	c := NewHTTPPlatformRelay(srv.URL, "")

	ct, err := c.ChainTypeForID(999999)
	if err != nil {
		t.Fatalf("ChainTypeForID: %v", err)
	}
	if ct != "weird_key" {
		t.Fatalf("got %q, want weird_key from hosting status", ct)
	}
}

func TestHTTPPlatformRelay_ChainTypeFromID_FallbackWhenEmptyEvmChains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":{"enabled":true,"evmChains":[]}}`)
	}))
	defer srv.Close()

	c := NewHTTPPlatformRelay(srv.URL, "")

	ct, err := c.ChainTypeForID(56)
	if err != nil {
		t.Fatalf("ChainTypeForID: %v", err)
	}
	if ct != "bsc" {
		t.Fatalf("fallback: got %q want bsc", ct)
	}
}

func TestHTTPPlatformRelay_GetSupportedChains_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":{"enabled":true,"supportedChains":["eth","bsc"]}}`)
	}))
	defer srv.Close()

	c := NewHTTPPlatformRelay(srv.URL, "")

	ch := c.GetSupportedChains()
	if len(ch) != 2 || ch[0] != "eth" {
		t.Fatalf("unexpected chains: %v", ch)
	}
}

func TestHTTPPlatformRelay_Execute_ParsesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/relay/execute" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"data":{"success":true,"txHash":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","taskId":"t1"}}`)
	}))
	defer srv.Close()

	c := NewHTTPPlatformRelay(srv.URL, "")

	resp, err := c.Execute(context.Background(), &EVMRelayRequest{
		ChainType: "eth",
		To:        "0x0000000000000000000000000000000000000001",
		Data:      "0x",
		OrderID:   "o1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TxHash != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || resp.TaskID != "t1" {
		t.Fatalf("resp %+v", resp)
	}
}

func TestHTTPPlatformRelay_Execute_RejectsInvalidTxHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":{"success":true,"txHash":"0xabc","taskId":"t1"}}`)
	}))
	defer srv.Close()

	c := NewHTTPPlatformRelay(srv.URL, "")

	_, err := c.Execute(context.Background(), &EVMRelayRequest{
		ChainType: "eth",
		To:        "0x0000000000000000000000000000000000000001",
		Data:      "0x",
		OrderID:   "o1",
	})
	if err == nil {
		t.Fatal("expected invalid txHash error")
	}
}
