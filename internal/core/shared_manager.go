//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/internal/api"
	mcfg "github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/embedded/frontend"
	obnet "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// SharedManager Manages shared resources.
// clients stores contracts.NodeService to support both MobazhaNode and TenantService.
type SharedManager struct {
	ExchangeRateProvider *wallet.ExchangeRateProvider
	mu                   sync.RWMutex
	clients              map[string]contracts.NodeService

	// httpGateway is the Mobazha API.
	httpGateway *api.Gateway

	SNFServers []peer.ID

	// snfProxy is the shared SNF proxy for multi-node hosting
	snfProxy *storeandforward.SNFProxy

	// testnet indicates if running on testnet
	testnet bool

	// saasAPIURL is the SaaS platform URL for standalone → SaaS calls
	saasAPIURL string

	// standaloneAPIKey is the API key for SaaS platform authentication
	standaloneAPIKey string

	// appDataDir is the top-level application data directory (cfg.DataDir).
	// Platform connection files (saas_api_key, owner_user_id, casdoor_certificate)
	// are persisted here so they are available at startup before any node is initialized.
	appDataDir string

	// ctx is the context for the manager
	ctx context.Context

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
			log.Infof("Failed to load net config from %s: %s, using defaults", endpoint, err)
		}
		netConfig.Testnet = cfg.Testnet
		applyInjectedManagedEscrowPaymentConfig(netConfig, cfg)

		if aiJSON := netConfig.GetAIProviders(); aiJSON != "" {
			if err := ai.LoadRemoteProviders(aiJSON); err != nil {
				log.Warningf("Failed to load remote AI providers: %s", err)
			}
		}

		// Store and forward client and server
		snfServers := func() []peer.ID {
			// Merge the snf server addresses from the config with the ones from the net config.
			servers := append(netConfig.StoreAndForwardServers, cfg.StoreAndForwardServers...)
			// Only add hardcoded default servers when no explicit servers are configured.
			// This allows E2E tests and custom deployments to use only their own SNF servers
			// without the overhead of trying to dial unreachable production servers.
			if len(servers) == 0 {
				if cfg.Testnet {
					servers = append(servers, repo.DefaultTestnetSNFServers...)
				} else {
					servers = append(servers, repo.DefaultMainnetSNFServers...)
				}
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

		// Hosting injects Matrix config via repo.Config; override remote NetConfig values.
		if cfg.MatrixInternalURL != "" {
			netConfig.MatrixInternalURL = cfg.MatrixInternalURL
		}
		if cfg.MatrixServerName != "" {
			netConfig.MatrixServerName = cfg.MatrixServerName
		}
		if cfg.MatrixRegistrationSecret != "" {
			netConfig.MatrixRegistrationSecret = cfg.MatrixRegistrationSecret
		}
		if cfg.MatrixSDKLogLevel != "" {
			netConfig.MatrixSDKLogLevel = cfg.MatrixSDKLogLevel
		}

		// Fallback: info-api wraps nodeConfig in {"data": ...} which maps
		// to NetConfig.Data (map[string]string) instead of struct fields.
		if netConfig.MatrixInternalURL == "" {
			if v, ok := netConfig.Data["matrixInternalURL"]; ok && v != "" {
				netConfig.MatrixInternalURL = v
			}
		}
		if netConfig.MatrixServerName == "" {
			if v, ok := netConfig.Data["matrixServerName"]; ok && v != "" {
				netConfig.MatrixServerName = v
			}
		}
		if netConfig.MatrixRegistrationSecret == "" {
			if v, ok := netConfig.Data["matrixRegistrationSecret"]; ok && v != "" {
				netConfig.MatrixRegistrationSecret = v
			}
		}
		// Standalone nodes default to the public SaaS URL so Matrix
		// provisioning, heartbeat, and exchange rates work out of the box.
		// Hosting's infrastructure-only default node must not consume remote
		// rates from itself (hairpin via public URL).
		if !cfg.SaaSMode && cfg.SaaSAPIURL == "" && !cfg.InfrastructureOnly {
			cfg.SaaSAPIURL = "https://app.mobazha.org"
		}

		// Load persisted API key from disk if not provided via CLI.
		if !cfg.SaaSMode && cfg.StandaloneAPIKey == "" && cfg.DataDir != "" {
			cfg.StandaloneAPIKey = loadPersistedAPIKey(cfg.DataDir)
		}

		if !cfg.SaaSMode && cfg.SaaSAPIURL != "" && !cfg.InfrastructureOnly {
			mcfg.GetGlobalExchangeRateConfig().SetRemoteSaaSURL(cfg.SaaSAPIURL)
		}

		erp := wallet.NewExchangeRateProvider(nil)

		// Auto-configure HTTP proxy trusted peers from NetConfig so that
		// native binary nodes accept LibP2P API proxy requests from SaaS
		// without requiring a manual --httpproxytrustedpeer CLI flag.
		if !cfg.SaaSMode && len(cfg.HTTPProxyTrustedPeers) == 0 {
			if pid, ok := netConfig.GetConfig("saasDefaultPeerID"); ok && pid != "" {
				cfg.HTTPProxyTrustedPeers = []string{pid}
				log.Infof("Auto-configured HTTP proxy trusted peer from NetConfig: %s", pid)
			} else if len(netConfig.StoreAndForwardServers) > 0 {
				cfg.HTTPProxyTrustedPeers = netConfig.StoreAndForwardServers
				log.Infof("Auto-configured HTTP proxy trusted peers from SNF servers: %v", netConfig.StoreAndForwardServers)
			}
		}

		SharedManagerInstance = &SharedManager{
			ExchangeRateProvider: erp,
			SNFServers:           snfServers,
			NetConfig:            netConfig,
			clients:              make(map[string]contracts.NodeService),
			testnet:              cfg.Testnet,
			saasAPIURL:           cfg.SaaSAPIURL,
			standaloneAPIKey:     cfg.StandaloneAPIKey,
			appDataDir:           cfg.DataDir,
			ctx:                  ctx,
		}
	})
	return SharedManagerInstance, nil
}

func applyInjectedManagedEscrowPaymentConfig(netConfig *mcfg.NetConfig, cfg *repo.Config) {
	if netConfig == nil || cfg == nil {
		return
	}
	if len(cfg.ManagedEscrowPlatformAddrs) > 0 {
		for rawChain, addr := range cfg.ManagedEscrowPlatformAddrs {
			chain := iwallet.ChainType(strings.TrimSpace(rawChain))
			if !chain.IsValid() || strings.TrimSpace(addr) == "" {
				continue
			}
			netConfig.SetPlatformAddr(chain, strings.TrimSpace(addr))
		}
	}
	if len(cfg.ManagedEscrowReleaseFeeUSDCents) > 0 {
		for rawChain, fee := range cfg.ManagedEscrowReleaseFeeUSDCents {
			chain := iwallet.ChainType(strings.TrimSpace(rawChain))
			if !chain.IsValid() {
				continue
			}
			netConfig.SetConfig(mcfg.ManagedEscrowGasReleaseFeeUSDCentsKey(chain), strconv.FormatUint(fee, 10))
		}
	}
}

// InitSNFProxy initializes the shared SNF proxy using the default node's host.
// This should be called after the default node is created.
func (im *SharedManager) InitSNFProxy(transportHost host.Host) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.snfProxy != nil {
		return nil // Already initialized
	}

	snfServerProtocol := obnet.ProtocolStoreAndForwardMainnet_Server
	snfClientProtocol := obnet.ProtocolStoreAndForwardMainnet_Client
	if im.testnet {
		snfServerProtocol = obnet.ProtocolStoreAndForwardTestnet_Server
		snfClientProtocol = obnet.ProtocolStoreAndForwardTestnet_Client
	}

	proxy, err := storeandforward.NewSNFProxy(im.ctx, &storeandforward.ProxyConfig{
		TransportHost:        transportHost,
		Servers:              im.SNFServers,
		ServerProtocol:       protocol.ID(snfServerProtocol),
		ClientProtocol:       protocol.ID(snfClientProtocol),
		RegistrationDuration: 0, // Use default
	})
	if err != nil {
		return fmt.Errorf("failed to create SNF proxy: %w", err)
	}

	im.snfProxy = proxy
	log.Info("SNF Proxy initialized successfully")
	return nil
}

// GetSNFProxy returns the shared SNF proxy
func (im *SharedManager) GetSNFProxy() *storeandforward.SNFProxy {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.snfProxy
}

// HasSNFProxy returns true if the SNF proxy is initialized
func (im *SharedManager) HasSNFProxy() bool {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.snfProxy != nil
}

func (im *SharedManager) GetDefaultNode() coreiface.CoreIface {
	im.mu.RLock()
	defer im.mu.RUnlock()
	node, ok := im.clients[repo.DefaultNodeID]
	if !ok {
		return nil
	}
	// Default node is always a MobazhaNode (CoreIface).
	ci, ok := node.(coreiface.CoreIface)
	if !ok {
		return nil
	}
	return ci
}

func (im *SharedManager) GetHTTPGateway() *api.Gateway {
	return im.httpGateway
}

// SetHTTPGateway sets the HTTP gateway. Used by hosting to inject the
// SharedRouter's Gateway so that builder.go's NotifyWebsockets integration
// reaches the correct WebSocket hubs.
func (im *SharedManager) SetHTTPGateway(gw *api.Gateway) {
	im.httpGateway = gw
}

func (im *SharedManager) AddNode(nodeID string, node contracts.NodeService) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.clients[nodeID] = node
}

func (im *SharedManager) RemoveNode(nodeID string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	delete(im.clients, nodeID)
}

func (im *SharedManager) GetNode(nodeID string) (contracts.NodeService, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	node, ok := im.clients[nodeID]
	return node, ok
}

func (im *SharedManager) GetNodes() map[string]contracts.NodeService {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.clients
}

// GetNodesSnapshot returns a race-free copy of all active NodeService
// instances. Callers may iterate the returned slice concurrently with
// AddNode / RemoveNode / GetNode.
//
// This is the implementation of contracts.NodeRegistry consumed by the
// shared scheduler (Phase AH-3): the scheduler iterates tenants on every
// tick without holding im.mu, while AddNode/RemoveNode continue to mutate
// im.clients safely under the write lock.
func (im *SharedManager) GetNodesSnapshot() []contracts.NodeService {
	im.mu.RLock()
	defer im.mu.RUnlock()
	out := make([]contracts.NodeService, 0, len(im.clients))
	for _, n := range im.clients {
		out = append(out, n)
	}
	return out
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

// GetExchangeRateService returns the shared ExchangeRateProvider as contracts.ExchangeRateService.
// The ExchangeRateProvider is a singleton shared across all nodes/tenants.
func (im *SharedManager) GetExchangeRateService() contracts.ExchangeRateService {
	return im.ExchangeRateProvider
}

func (im *SharedManager) initHTTPGateway(cfg *repo.Config) (*api.Gateway, error) {
	// Resolve gateway listen address: CLI override > default.
	gatewayAddr := cfg.GatewayAddr
	if gatewayAddr == "" {
		gatewayAddr = repo.DefaultGatewayMultiaddr
	}

	gatewayMaddr, err := ma.NewMultiaddr(gatewayAddr)
	if err != nil {
		return nil, fmt.Errorf("newHTTPGateway: invalid gateway address: %q (err: %s)", gatewayAddr, err)
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

	allowedIPs := make(map[string]bool)
	for _, ip := range cfg.APIAllowedIPs {
		allowedIPs[ip] = true
	}

	// Credential priority chain:
	//   1. Hash file on disk (survives password changes via API)
	//   2. Config file / CLI flags (apiusername + apipassword)
	//   3. Auto-generate (non-SaaS nodes with no auth at all)
	username, passwordHash := api.LoadCredentials(cfg.DataDir, cfg.APIUsername, cfg.APIPassword)

	noBasicAuth := username == "" || passwordHash == ""
	noCookieAuth := cfg.APICookie == ""
	if !cfg.SaaSMode && cfg.DataDir != "" && noBasicAuth && noCookieAuth {
		var err error
		username, passwordHash, err = api.EnsureStandaloneAuth(cfg.DataDir)
		if err != nil {
			return nil, fmt.Errorf("initializing standalone auth: %w", err)
		}
		plainPath := api.AdminPasswordPlaintextPath(cfg.DataDir)
		if _, statErr := os.Stat(plainPath); statErr == nil {
			if frontend.HasContent() {
				log.Infof("Admin credentials generated — set your password in the Web UI setup wizard.")
			} else {
				log.Warningf("Admin password saved to %s — read it from the file and change after first login", plainPath)
			}
		}
	}

	// Resolve LocalPeerID from the default node.
	var localPeerID string
	if defaultNode := im.GetDefaultNode(); defaultNode != nil {
		localPeerID = defaultNode.IdentityInfo().Identity().String()
	}

	casdoorCert := cfg.CasdoorCertificate
	ownerUserID := cfg.OwnerUserID
	if !cfg.SaaSMode && cfg.DataDir != "" {
		persistedCert, persistedOwner := api.LoadPersistedPlatformConfig(cfg.DataDir)
		if casdoorCert == "" && persistedCert != "" {
			casdoorCert = persistedCert
			log.Infof("Loaded Casdoor certificate from data directory")
		}
		if ownerUserID == "" && persistedOwner != "" {
			ownerUserID = persistedOwner
			log.Infof("Loaded owner user ID from data directory: %s", persistedOwner)
		}
	}

	gwConfig := &api.GatewayConfig{
		Listener:               manet.NetListener(gwLis),
		AllowAllOrigins:        cfg.APIAllowAllOrigins,
		UseSSL:                 cfg.UseSSL,
		SSLCert:                cfg.SSLCertFile,
		SSLKey:                 cfg.SSLKeyFile,
		Username:               username,
		Password:               passwordHash,
		Cookie:                 cfg.APICookie,
		PublicOnly:             cfg.APIPublicGateway,
		AllowedIPs:             allowedIPs,
		HashFile:               api.AdminPasswordHashPath(cfg.DataDir),
		PlainFile:              api.AdminPasswordPlaintextPath(cfg.DataDir),
		CasdoorCertificate:     casdoorCert,
		LocalPeerID:            localPeerID,
		OwnerUserID:            ownerUserID,
		SaaSAPIURL:             cfg.SaaSAPIURL,
		StandaloneAPIKey:       cfg.StandaloneAPIKey,
		StandaloneConnectivity: cfg.StandaloneConnectivity,
		DataDir:                cfg.DataDir,
	}

	im.httpGateway, err = api.NewGateway(im, gwConfig)
	if err != nil {
		return nil, err
	}

	// Auto-fetch Casdoor certificate on standalone startup when not yet available.
	// This enables buyer login immediately after deployment without manual admin action.
	if !cfg.SaaSMode && cfg.SaaSAPIURL != "" && casdoorCert == "" && cfg.DataDir != "" {
		go im.autoFetchCasdoorCert(cfg.SaaSAPIURL, cfg.DataDir, localPeerID, ownerUserID)
	}

	return im.httpGateway, nil
}

// autoFetchCasdoorCert fetches the Casdoor signing certificate from the SaaS
// platform and enables JWT authentication on the gateway. Uses exponential
// backoff (5s → 10s → 20s → 40s → 60s cap, up to 5 retries).
func (im *SharedManager) autoFetchCasdoorCert(saasURL, dataDir, localPeerID, ownerUserID string) {
	const maxRetries = 5
	backoff := 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		certPEM, err := obnet.FetchCasdoorCertificate(context.Background(), saasURL)
		if err != nil {
			log.Warningf("Auto-fetch Casdoor certificate attempt %d/%d failed: %v", attempt, maxRetries, err)
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
			continue
		}

		if err := os.WriteFile(api.CertFilePath(dataDir), []byte(certPEM), 0600); err != nil {
			log.Errorf("Failed to persist auto-fetched Casdoor certificate: %v", err)
			return
		}

		if im.httpGateway != nil {
			if err := im.httpGateway.EnableJWTAuth(certPEM, localPeerID, ownerUserID); err != nil {
				log.Errorf("Failed to enable JWT auth with auto-fetched certificate: %v", err)
				return
			}
		}

		log.Infof("Auto-fetched Casdoor certificate from %s — buyer login enabled", saasURL)
		return
	}

	log.Warningf("Failed to auto-fetch Casdoor certificate after %d attempts (buyers cannot log in until admin connects platform manually)", maxRetries)
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

const standaloneAPIKeyFile = "saas_api_key"

// loadPersistedAPIKey reads a previously saved SaaS API key from disk.
func loadPersistedAPIKey(dataDir string) string {
	keyPath := filepath.Join(dataDir, standaloneAPIKeyFile)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	if key := strings.TrimSpace(string(data)); key != "" {
		log.Infof("Loaded SaaS API key from %s", keyPath)
		return key
	}
	return ""
}

// PersistAPIKey saves the API key to the data directory for future startups.
func PersistAPIKey(dataDir, apiKey string) error {
	return os.WriteFile(filepath.Join(dataDir, standaloneAPIKeyFile), []byte(apiKey), 0600)
}
