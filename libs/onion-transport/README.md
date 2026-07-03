# go-onion-transport

This library contains a Tor transport implementation for libp2p (v0.8.3) that uses an embedded Tor client. 

#### Usage

```go

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	oniontransport "github.com/mobazha/mobazha/pkg/libs/onion-transport"
	"github.com/cretz/bine/tor"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipsn/go-libtor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/config"
	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"path"
)

func main() {

	// First create the private key that will be used for the onion address.
	// You will need to persist this key between sessions if you want to use
	// the same onion address each time.
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// Define a directory for the tor client to use. Inside your application's
	// data directory will do fine.
	torDir := path.Join(dataDir, "tor")

	// Create the embedded Tor client.
	torClient, err := tor.Start(nil, &tor.StartConf{
		ProcessCreator:  libtor.Creator,
		DataDir:         torDir,
		NoAutoSocksPort: true,
		EnableNetwork:   true,
		ExtraArgs:       []string{"--DNSPort", "2121"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create the dialer.
	//
	// IMPORTANT: If you are genuinely trying to anonymize your IP you will need to route
	// any non-libp2p traffic through this dialer as well. For example, any HTTP requests
	// you make MUST go through this dialer.
	dialer, err := torClient.Dialer(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create the onion service.
	onionService, err := torClient.Listen(context.Background(), &tor.ListenConf{
		RemotePorts: []int{9003},
		Version3:    true,
		Key:         privateKey,
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// Override the default lip2p DNS resolver. We need this because libp2p address may contain a 
	// DNS hostname that will be resolved before dialing. If we do not configure the resolver to 
	// use Tor we will blow any anonymity we gained by using Tor.
	// 
	// Note you must enter the DNS resolver address that was used when creating the Tor client.
	madns.DefaultResolver = oniontransport.NewTorResover("localhost:2121")

	// If this option is true then the transport will only attempt to dial out to onion
	// addresses. If libp2p requests to dial out on another type of address, TCP for example,
	// it will respond saying it can't dial that address.
	//
	// If your goal is to use this transport in conjunction with other non-Tor transports,
	// for example in a "dual stack" configuration, then you likely only want to route only
	// outgoing connections to onion addresses through this transport and let outgoing TCP
	// connections go through a faster transport. In this case this option should be true.
	// Note that such a configuration would NOT be anonymous.
	//
	// If this option is set to false then the this transport will attempt to dial out to
	// both onion addresses and TCP addresses.
	dialOnionOnly := false

	// Create the libp2p transport option.
	transportOpt := libp2p.Transport(oniontransport.NewOnionTransportC(dialer, onionService, dialOnionOnly))

	// Create address option.
	onionAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/onion3/%s:9003", onionService.ID))
	if err != nil {
		log.Fatal(err)
	}
	addressOpt := libp2p.ListenAddrs(onionAddr)

	// The transport option is passed in along with the FallbackDefaults options. FallbackDefaults
	// only applies the default options if no other option is provided. In this case we provided
	// the Tor transport so this configuration should ONLY use the Tor transport and no other transports.
	//
	// Using libp2p.Defaults instead of libp2p.FallbackDefaults would append the Tor transport to the
	// the existing list of transports.
	peerHost, err := libp2p.New(context.Background(), transportOpt, addressOpt, libp2p.FallbackDefaults)
	if err != nil {
		log.Fatal(err)
	}

	// Libp2p is now configured to use Tor.

	// If you want to configure IPFS to use Tor you will need to create a new hostOption.
	constructPeerHost := func(ctx context.Context, id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
		pkey := ps.PrivKey(id)
		if pkey == nil {
			return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
		}
		options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.Peerstore(ps)}, options...)

		cfg := &config.Config{}
		if err := cfg.Apply(options...); err != nil {
			return nil, err
		}

		cfg.Transports = nil
		if err := transportOpt(cfg); err != nil {
			return nil, err
		}
		return cfg.NewNode(ctx)
	}

	// You can set the swarm onion address in the config if you want. Or set it here.
	ipfsRepo, err := fsrepo.Open(ipfsDir)
	if err != nil {
		log.Fatal(err)
	}

	ipfsConfig, err := ipfsRepo.Config()
	if err != nil {
		log.Fatal(err)
	}

	ipfsConfig.Addresses.Swarm = []string{fmt.Sprintf("/onion3/%s:9003", onionService.ID)}

	// New IPFS build config
	ncfg := &core.BuildCfg{
		Repo: ipfsRepo,
		Host: constructPeerHost,
	}

	// Construct IPFS node.
	ipfsNode, err := core.NewNode(context.Background(), ncfg)
	if err != nil {
		log.Fatal(err)
	}

	// IPFS is now configured to use Tor.
	
	// WARNING: Passing a hostname into an IPNS Resolve will likely route the the DNS name resolution
	// in the clear and not through Tor. It should be a small change to IPFS to use the madns.DefaultResover
	// but we need to get it merged and released first. 
}
```
