//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"fmt"
	"path"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	lcfg "github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	libp2pwebrtc "github.com/libp2p/go-libp2p/p2p/transport/webrtc"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/fx"
)

// P2PInfra encapsulates the libp2p networking stack (Host, DHT, datastores).
// Full (default/standalone) nodes use this; lightweight SaaS tenant nodes
// create a minimal host directly.
type P2PInfra struct {
	Host     host.Host
	PeerID   peer.ID
	PrivKey  crypto.PrivKey
	DHT      *dual.DHT
	DHTStore ds.Batching // in-memory, used only by DHT
	SNFStore ds.Batching // LevelDB persistent store; nil when SNF server is disabled
	Ctx      context.Context
	Cancel   context.CancelFunc
}

// Close tears down the P2P infrastructure in reverse dependency order.
// Errors are aggregated so that all components are closed even if one fails.
func (p *P2PInfra) Close() error {
	var errs []error

	if p.DHT != nil {
		if err := p.DHT.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close DHT: %w", err))
		}
	}
	if p.Host != nil {
		if err := p.Host.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close Host: %w", err))
		}
	}
	if p.SNFStore != nil {
		if c, ok := p.SNFStore.(interface{ Close() error }); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close SNFStore: %w", err))
			}
		}
	}
	if p.DHTStore != nil {
		if c, ok := p.DHTStore.(interface{ Close() error }); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close DHTStore: %w", err))
			}
		}
	}
	if p.Cancel != nil {
		p.Cancel()
	}
	return errors.Join(errs...)
}

// P2PConfig holds parameters for constructing a P2PInfra.
type P2PConfig struct {
	PrivKey           crypto.PrivKey
	SwarmAddrs        []string
	AnnounceAddrs     []string // extra multiaddrs to advertise (e.g. public IP in Docker)
	BootstrapAddrs    []string // multiaddr strings (parsed internally)
	StaticRelayPeers  []string // peer ID strings of known relay servers (for AutoRelay)
	DataDir           string   // persistent data directory for SNF store
	Testnet           bool
	DHTClientOnly     bool
	IsDefaultNode     bool
	DisableNATPortMap bool
	DisableReuseport  bool
	EnableSNFServer   bool
	EnableRelayServer bool // run circuit-relay v2 service (for infra / public nodes)
	Tor               bool
	DualStack         bool
	TransportOpt      libp2p.Option // Tor/dual-stack transport (may be nil)
	NATConnectivity   string        // "nat" enables relay client + hole punching
}

// createP2PInfra constructs a full libp2p Host with Kad-DHT directly,
// replacing the previous Kubo core.NewNode() call.
func createP2PInfra(parentCtx context.Context, pcfg *P2PConfig) (*P2PInfra, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()

	peerID, err := peer.IDFromPrivateKey(pcfg.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("createP2PInfra: derive peer ID: %w", err)
	}

	// ── Build libp2p Host ──────────────────────────────────────────────
	hostOpts := []libp2p.Option{
		libp2p.Identity(pcfg.PrivKey),
		libp2p.ListenAddrStrings(pcfg.SwarmAddrs...),
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
	}

	// Explicit transports (replaces DefaultTransports) so we can
	// conditionally disable TCP reuseport. On macOS, SO_REUSEPORT causes
	// outbound dials from the listener port (0.0.0.0:4001) to time out
	// because SYN-ACK packets are misrouted to the listener socket.
	if pcfg.DisableReuseport {
		hostOpts = append(hostOpts, libp2p.Transport(tcp.NewTCPTransport, tcp.DisableReuseport()))
	} else {
		hostOpts = append(hostOpts, libp2p.Transport(tcp.NewTCPTransport))
	}
	hostOpts = append(hostOpts,
		libp2p.Transport(quic.NewTransport),
		libp2p.Transport(ws.New),
		libp2p.Transport(webtransport.New),
		libp2p.Transport(libp2pwebrtc.New),
	)

	if !pcfg.DisableNATPortMap && !pcfg.Tor {
		hostOpts = append(hostOpts, libp2p.NATPortMap())
	}

	// Circuit Relay v2: always enable the relay transport (client side)
	// and hole punching so NAT'd nodes can be reached via relay peers.
	hostOpts = append(hostOpts, libp2p.EnableRelay(), libp2p.EnableHolePunching())

	// Public infrastructure nodes run the relay *service* so they can
	// relay traffic on behalf of NAT'd peers.
	if pcfg.EnableRelayServer {
		hostOpts = append(hostOpts,
			libp2p.EnableRelayService(),
			libp2p.ForceReachabilityPublic(),
		)
	}

	// AutoRelay: when static relay peers are provided, the node will
	// automatically reserve relay slots and advertise relay addresses
	// when AutoNAT detects it is behind NAT.
	if len(pcfg.StaticRelayPeers) > 0 {
		var relayInfos []peer.AddrInfo
		for _, s := range pcfg.StaticRelayPeers {
			maddr, err := ma.NewMultiaddr(s)
			if err != nil {
				log.Warningf("skipping invalid static relay addr %q: %v", s, err)
				continue
			}
			pi, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Warningf("skipping static relay addr %q: %v", s, err)
				continue
			}
			relayInfos = append(relayInfos, *pi)
		}
		if len(relayInfos) > 0 {
			hostOpts = append(hostOpts, libp2p.EnableAutoRelayWithStaticRelays(relayInfos))
		}
	}

	// AnnounceAddrs: extra multiaddrs to advertise (e.g. public IP for
	// Docker-hosted nodes that only see internal container addresses).
	if len(pcfg.AnnounceAddrs) > 0 {
		var extraAddrs []ma.Multiaddr
		for _, s := range pcfg.AnnounceAddrs {
			maddr, err := ma.NewMultiaddr(s)
			if err != nil {
				log.Warningf("skipping invalid announce addr %q: %v", s, err)
				continue
			}
			extraAddrs = append(extraAddrs, maddr)
		}
		if len(extraAddrs) > 0 {
			hostOpts = append(hostOpts, libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
				return append(addrs, extraAddrs...)
			}))
		}
	}

	// Apply options to low-level config for Tor/DualStack transport handling.
	// Tor mode: replace all transports with the onion transport.
	// DualStack mode: add the onion transport alongside defaults.
	config := &lcfg.Config{}
	if err := config.Apply(hostOpts...); err != nil {
		return nil, fmt.Errorf("createP2PInfra: apply host options: %w", err)
	}
	config.DisableMetrics = true

	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, fmt.Errorf("createP2PInfra: new peerstore: %w", err)
	}
	config.Peerstore = ps

	if pcfg.Tor && pcfg.TransportOpt != nil {
		config.Transports = []fx.Option{}
		if err := pcfg.TransportOpt(config); err != nil {
			return nil, fmt.Errorf("createP2PInfra: apply Tor transport: %w", err)
		}
	} else if pcfg.DualStack && pcfg.TransportOpt != nil {
		if err := pcfg.TransportOpt(config); err != nil {
			return nil, fmt.Errorf("createP2PInfra: apply DualStack transport: %w", err)
		}
	}

	h, err := config.NewNode()
	if err != nil {
		return nil, fmt.Errorf("createP2PInfra: new host: %w", err)
	}

	// ── DHT ────────────────────────────────────────────────────────────
	dhtStore := dsync.MutexWrap(ds.NewMapDatastore())

	dhtMode := dht.ModeClient
	if pcfg.DHTClientOnly {
		// explicit client-only, keep default
	} else if pcfg.IsDefaultNode {
		dhtMode = dht.ModeServer
	}

	bootstrapPeers, err := parseBootstrapPeers(pcfg.BootstrapAddrs)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("createP2PInfra: parse bootstrap peers: %w", err)
	}

	dhtOpts := []dht.Option{
		dht.Concurrency(20),
		dht.Mode(dhtMode),
		dht.Datastore(dhtStore),
		dht.ProtocolPrefix(ProtocolDHT),
		dht.MaxRecordAge(maxRecordAge),
		dht.AddressFilter(nil),
	}
	wanOptions := []dht.Option{
		dht.BootstrapPeers(bootstrapPeers...),
	}

	dualDHT, err := dual.New(
		ctx, h,
		dual.DHTOption(dhtOpts...),
		dual.WanDHTOption(wanOptions...),
	)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("createP2PInfra: new DHT: %w", err)
	}

	// ── SNF store (persistent, LevelDB) ────────────────────────────────
	var snfStore ds.Batching
	if pcfg.EnableSNFServer {
		snfPath := path.Join(pcfg.DataDir, "snf-datastore")
		snfStore, err = leveldb.NewDatastore(snfPath, nil)
		if err != nil {
			dualDHT.Close()
			h.Close()
			return nil, fmt.Errorf("createP2PInfra: new SNF store: %w", err)
		}
	}

	success = true
	return &P2PInfra{
		Host:     h,
		PeerID:   peerID,
		PrivKey:  pcfg.PrivKey,
		DHT:      dualDHT,
		DHTStore: dhtStore,
		SNFStore: snfStore,
		Ctx:      ctx,
		Cancel:   cancel,
	}, nil
}

// parseBootstrapPeers converts multiaddr strings into peer.AddrInfo objects
// for DHT bootstrap. Invalid addresses are silently skipped.
func parseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	var peers []peer.AddrInfo
	seen := make(map[peer.ID]bool)
	for _, s := range addrs {
		maddr, err := ma.NewMultiaddr(s)
		if err != nil {
			log.Warningf("skipping invalid bootstrap address %q: %v", s, err)
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Warningf("skipping bootstrap address %q: %v", s, err)
			continue
		}
		if !seen[pi.ID] {
			seen[pi.ID] = true
			peers = append(peers, *pi)
		}
	}
	if len(addrs) > 0 && len(peers) == 0 {
		return nil, fmt.Errorf("all %d bootstrap addresses were invalid", len(addrs))
	}
	return peers, nil
}
