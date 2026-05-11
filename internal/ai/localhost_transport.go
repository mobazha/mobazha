package ai

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewLocalhostOnlyClient returns an *http.Client whose transport refuses to
// dial any address that does not resolve to a loopback IP. This is a
// defense-in-depth measure for PrivateDistribution mode: even if handler-level URL
// validation is bypassed (e.g. DNS rebinding), the network layer blocks
// outbound connections to non-loopback hosts.
func NewLocalhostOnlyClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("localhost-only: invalid address %q: %w", addr, err)
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("localhost-only: DNS resolve %q: %w", host, err)
			}

			for _, ip := range ips {
				if !ip.IP.IsLoopback() {
					return nil, fmt.Errorf("localhost-only: %q resolved to non-loopback %s — blocked", host, ip.IP)
				}
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}
