package core

import (
	"context"
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
func (p *P2PInfra) Close() error {
	if p.DHT != nil {
		_ = p.DHT.Close()
	}
	if p.Host != nil {
		_ = p.Host.Close()
	}
	if p.SNFStore != nil {
		if c, ok := p.SNFStore.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
	if p.DHTStore != nil {
		if c, ok := p.DHTStore.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
	if p.Cancel != nil {
		p.Cancel()
	}
	return nil
}

// P2PConfig holds parameters for constructing a P2PInfra.
type P2PConfig struct {
	PrivKey           crypto.PrivKey
	SwarmAddrs        []string
	BootstrapAddrs    []string // multiaddr strings (parsed internally)
	DataDir           string   // persistent data directory for SNF store
	Testnet           bool
	DHTClientOnly     bool
	IsDefaultNode     bool
	DisableNATPortMap bool
	EnableSNFServer   bool
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
		libp2p.DefaultTransports,
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
	}

	if !pcfg.DisableNATPortMap && !pcfg.Tor {
		hostOpts = append(hostOpts, libp2p.NATPortMap())
	}

	if pcfg.NATConnectivity == "nat" {
		hostOpts = append(hostOpts, libp2p.EnableRelay(), libp2p.EnableHolePunching())
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
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		if !seen[pi.ID] {
			seen[pi.ID] = true
			peers = append(peers, *pi)
		}
	}
	return peers, nil
}
