//go:build tor
// +build tor

package net

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"path"
	"strings"

	"github.com/cretz/bine/tor"
	"github.com/ipsn/go-libtor"
	"github.com/libp2p/go-libp2p"
	oniontransport "github.com/mobazha/mobazha3.0/pkg/onion-transport"
	"golang.org/x/net/proxy"
)

// SetupTor is a constructor that initializes the embedded Tor client. The reason we have it here in this package
// rather than core/builder.go is because we want to be able to control whether or not to build the tor C library
// using build tags. This file is not the default and the build tag needs to be used to use Tor. If the `tor` build
// tag is not used it will not build the Tor client and will error if the config options try to enable it.
func SetupTor(ctx context.Context, key ed25519.PrivateKey, dataDir string, dualstackMode bool) (string, proxy.Dialer, libp2p.Option, func() error, error) {
	embeddedTorClient, err := tor.Start(nil, &tor.StartConf{
		ProcessCreator:  libtor.Creator,
		DataDir:         path.Join(dataDir, "tor"),
		EnableNetwork:   true,
		NoAutoSocksPort: true,
		ExtraArgs:       []string{"--DNSPort", strings.Split(TorDNSResover, ":")[1]},
	})
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to start tor: %v", err)
	}
	dialer, err := embeddedTorClient.Dialer(context.Background(), nil)
	if err != nil {
		return "", nil, nil, nil, err
	}

	onion, err := embeddedTorClient.Listen(ctx, &tor.ListenConf{
		RemotePorts: []int{9003},
		Version3:    true,
		Key:         key,
	})
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to create onion service: %v", err)
	}

	transportOpt := libp2p.Transport(oniontransport.NewOnionTransportC(dialer, onion, dualstackMode))

	return onion.ID, dialer, transportOpt, onion.Close, nil
}
