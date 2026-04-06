package proxyclient

import (
	"io"
	"strings"
	"testing"

	"golang.org/x/net/proxy"
)

func TestNewHttpClient(t *testing.T) {
	t.Skip("skipping: requires network access and a running Tor proxy on 127.0.0.1:9150")
	// No proxy
	client := NewHttpClient()

	resp, err := client.Get("http://check.torproject.org")
	if err != nil {
		t.Fatalf("Failed to issue GET request: %v\n", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read the body: %v\n", err)
	}
	if strings.Contains(string(body), "Congratulations. This browser is configured to use Tor.") {
		t.Error("Connected through proxy when we should not have")
	}

	// With Proxy
	dialer, err = proxy.SOCKS5("tcp", "127.0.0.1:9150", nil, proxy.Direct)
	if err != nil {
		t.Fatal(err)
	}

	SetProxy(dialer)
	client = NewHttpClient()

	resp, err = client.Get("http://check.torproject.org")
	if err != nil {
		t.Fatalf("Failed to issue GET request: %v\n", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read the body: %v\n", err)
	}
	if !strings.Contains(string(body), "Congratulations. This browser is configured to use Tor.") {
		t.Error("Failed to connect through Tor")
	}
}
