// +build !tor

package net

import (
	"context"
	"crypto/ed25519"
	"errors"
	"github.com/libp2p/go-libp2p"
	"golang.org/x/net/proxy"
)

// SetupTor is used by default. It will not build the tor C library.
func SetupTor(ctx context.Context, key ed25519.PrivateKey, dataDir string, dualstackMode bool) (string, proxy.Dialer, libp2p.Option, func() error, error) {
	return "", nil, nil, nil, errors.New("openbazaar was built with the notor build tag")
}
