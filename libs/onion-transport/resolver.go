package oniontransport

import (
	"context"
	madns "github.com/multiformats/go-multiaddr-dns"
	"net"
	"time"
)

// NewTorResover returns a no madns.Resolver that will resolve
// IP addresses over Tor.
//
// TODO: This does not seem to work for TXT records. Look into if
// Tor can resolve TXT records.
func NewTorResover(proxy string) *madns.Resolver {
	netResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, proxy)
		},
	}
	r, _ := madns.NewResolver(madns.WithDefaultResolver(netResolver))
	return r
}
