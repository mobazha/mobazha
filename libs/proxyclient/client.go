package proxyclient

import (
	"context"
	"errors"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"sync"
)

var (
	dialerOnce sync.Once
	dialer     proxy.Dialer
)

// SetProxy guards the internal state of this package with a sync.Once. It allows the
// caller to set a socks5 proxy dialer which will be used to instantiate an http client
// on all subsequent calls to NewHttpClient.
func SetProxy(proxyDialer proxy.Dialer) {
	dialerOnce.Do(func() {
		dialer = proxyDialer
	})
}

// DialFunc returns a dial function using the package's proxy dialer or
// an error if the dialer is not set.
func DialFunc() (func(network, addr string) (net.Conn, error), error) {
	if dialer == nil {
		return nil, errors.New("proxy dialer not set")
	}

	return dialer.Dial, nil
}

// DialContextFunc returns a dial function using the package's proxy dialer or
// an error if the dialer is not set.
func DialContextFunc() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if dialer == nil {
		return nil, errors.New("proxy dialer not set")
	}

	return dialContext, nil
}

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if ctx == nil {
		panic("nil context")
	}
	type dialResult struct {
		net.Conn
		error
	}
	results := make(chan dialResult)
	defer close(results)

	go func() {
		conn, err := dialer.Dial(network, addr)
		results <- dialResult{conn, err}
	}()

	select {
	case result := <-results:
		return result.Conn, result.error
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// NewHttpClient returns a new http client. If a proxy is set it will instantiate the
// new client with a proxy dialer. Otherwise it will return a default http client.
func NewHttpClient() *http.Client {
	if dialer == nil {
		return &http.Client{}
	}

	// Make a http.Transport that uses the proxy dialer, and a
	// http.Client that uses the transport.
	tbTransport := &http.Transport{DialContext: dialContext}
	client := &http.Client{Transport: tbTransport}

	return client
}
