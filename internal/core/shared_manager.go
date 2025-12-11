package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"sync"

	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/corehttp"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/api"
	mcfg "github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// SharedManager Manages shared resources
type SharedManager struct {
	ExchangeRateProvider *wallet.ExchangeRateProvider
	mu                   sync.RWMutex
	clients              map[string]coreiface.CoreIface

	// httpGateway is the Mobazha API.
	httpGateway *api.Gateway

	SNFServers []peer.ID

	NetConfig *mcfg.NetConfig
}

var (
	SharedManagerInstance *SharedManager
	once                  sync.Once
)

// NewSharedManager creates a new SharedManager instance
func NewSharedManager(ctx context.Context, cfg *repo.Config) (*SharedManager, error) {
	once.Do(func() {
		repo.SetupLogging(cfg.LogDir, cfg.LogLevel)

		// Profiling
		if cfg.Profile != "" {
			go func() {
				listenAddr := net.JoinHostPort("", cfg.Profile)
				log.Infof("Profile server listening on %s", listenAddr)
				profileRedirect := http.RedirectHandler("/debug/pprof",
					http.StatusSeeOther)
				http.Handle("/", profileRedirect)
				log.Errorf("%v", http.ListenAndServe(listenAddr, nil))
			}()
		}

		// Write cpu profile if requested.
		if cfg.CPUProfile != "" {
			f, err := os.Create(cfg.CPUProfile)
			if err != nil {
				log.Errorf("Unable to create cpu profile: %v", err)
			} else {
				pprof.StartCPUProfile(f)
				defer f.Close()
				defer pprof.StopCPUProfile()
			}
		}

		endpoint := netConfigEndpoint
		if cfg.NetConfigEndpoint != "" {
			endpoint = cfg.NetConfigEndpoint
		}
		netConfig, err := mcfg.LoadNetConfig(endpoint)
		if err != nil {
			log.Infof("Failed to load net config: %s", err)
		}

		// Store and forward client and server
		snfServers := func() []peer.ID {
			// Merge the snf server addresses from the config with the ones from the net config.
			servers := append(netConfig.StoreAndForwardServers, cfg.StoreAndForwardServers...)
			if cfg.Testnet {
				servers = append(servers, repo.DefaultTestnetSNFServers...)
			} else {
				servers = append(servers, repo.DefaultMainnetSNFServers...)
			}

			serverMap := make(map[string]bool)
			for _, server := range servers {
				serverMap[server] = true
			}

			addrs := []peer.ID{}
			for serverStr := range serverMap {
				server, err := peer.Decode(serverStr)
				if err != nil {
					log.Errorf("Error parsing snf server %s: %s", serverStr, err)
					continue
				}
				addrs = append(addrs, server)
			}
			return addrs
		}()

		erp := wallet.NewExchangeRateProvider(netConfig.GetExchangeRateProviders())

		SharedManagerInstance = &SharedManager{
			ExchangeRateProvider: erp,
			SNFServers:           snfServers,
			NetConfig:            netConfig,
			clients:              make(map[string]coreiface.CoreIface),
		}
	})
	return SharedManagerInstance, nil
}

func (im *SharedManager) GetDefaultNode() coreiface.CoreIface {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.clients[repo.DefaultNodeID]
}

func (im *SharedManager) GetIPFSNode() *core.IpfsNode {
	im.mu.RLock()
	defer im.mu.RUnlock()

	obNode, ok := im.clients[repo.DefaultNodeID]
	if !ok {
		return nil
	}

	return obNode.IPFSNode()
}

func (im *SharedManager) GetHTTPGateway() *api.Gateway {
	return im.httpGateway
}

func (im *SharedManager) AddNode(nodeID string, node coreiface.CoreIface) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.clients[nodeID] = node
}

func (im *SharedManager) RemoveNode(nodeID string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	delete(im.clients, nodeID)
}

func (im *SharedManager) GetNode(nodeID string) (coreiface.CoreIface, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	node, ok := im.clients[nodeID]
	return node, ok
}

func (im *SharedManager) GetNodes() map[string]coreiface.CoreIface {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.clients
}

// GetMaxImportZipSize returns the maximum size for batch import ZIP files.
func (im *SharedManager) GetMaxImportZipSize() int64 {
	if im.NetConfig != nil {
		return im.NetConfig.GetMaxImportZipSize()
	}
	return 300 << 20 // 300MB default
}

// GetMaxImportVideoSize returns the maximum size for individual video files in batch import.
func (im *SharedManager) GetMaxImportVideoSize() int64 {
	if im.NetConfig != nil {
		return im.NetConfig.GetMaxImportVideoSize()
	}
	return 15 << 20 // 15MB default
}

func (im *SharedManager) initHTTPGateway(cfg *repo.Config) (*api.Gateway, error) {
	ipfsNode := im.GetIPFSNode()
	if ipfsNode == nil {
		return nil, fmt.Errorf("ipfs node not found")
	}

	// Get API configuration
	ipfsConf, err := ipfsNode.Repo.Config()
	if err != nil {
		return nil, err
	}

	// Create a network listener
	gatewayMaddr, err := ma.NewMultiaddr(ipfsConf.Addresses.Gateway[0])
	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: invalid gateway address: %q (err: %s)", ipfsConf.Addresses.Gateway, err)
	}
	var gwLis manet.Listener
	if cfg.UseSSL {
		netAddr, err := manet.ToNetAddr(gatewayMaddr)
		if err != nil {
			return nil, err
		}
		gwLis, err = manet.WrapNetListener(&dummyListener{netAddr})
		if err != nil {
			return nil, err
		}
	} else {
		gwLis, err = manet.Listen(gatewayMaddr)
		if err != nil {
			return nil, fmt.Errorf("newHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
		}
	}

	// We might have listened to /tcp/0 - let's see what we are listing on
	gatewayMaddr = gwLis.Multiaddr()

	// Setup an options slice
	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.VersionOption(),
		corehttp.HostnameOption(),
		corehttp.GatewayOption("/ipfs", "/ipns"),
	}

	if len(ipfsConf.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", ipfsConf.Gateway.RootRedirect))
	}

	allowedIPs := make(map[string]bool)
	for _, ip := range cfg.APIAllowedIPs {
		allowedIPs[ip] = true
	}

	config := &api.GatewayConfig{
		Listener:        manet.NetListener(gwLis),
		AllowAllOrigins: cfg.APIAllowAllOrigins,
		UseSSL:          cfg.UseSSL,
		SSLCert:         cfg.SSLCertFile,
		SSLKey:          cfg.SSLKeyFile,
		Username:        cfg.APIUsername,
		Password:        cfg.APIPassword,
		Cookie:          cfg.APICookie,
		PublicOnly:      cfg.APIPublicGateway,
		AllowedIPs:      allowedIPs,
	}

	im.httpGateway, err = api.NewGateway(im, config, opts...)
	if err != nil {
		return nil, err
	}

	return im.httpGateway, nil
}

func (im *SharedManager) Start() {
	if im.httpGateway == nil {
		return
	}
	go im.httpGateway.Serve()
}

func (im *SharedManager) Stop() {
	if im.httpGateway == nil {
		return
	}
	im.httpGateway.Close()
}
