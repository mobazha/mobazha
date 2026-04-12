package repo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/natefinch/lumberjack"
)

const (
	defaultConfigFilename = "mobazha.conf"
	defaultLogDirname     = "logs"
	defaultLogFilename    = "mobazha.log"
)

var (
	DefaultHomeDir = AppDataDir("mobazha", false)

	LogLevelMap = map[string]logging.Level{
		"debug":    logging.DEBUG,
		"info":     logging.INFO,
		"notice":   logging.NOTICE,
		"warning":  logging.WARNING,
		"error":    logging.ERROR,
		"critical": logging.CRITICAL,
	}

	DefaultMainnetBootstrapAddrs = []string{
		// "/ip4/23.94.43.104/tcp/4001/p2p/12D3KooWD1GpGf11qVtcDhat8q8rB2du9nohFEFu2DgciUYWY2BC",
		// "/ip4/192.227.231.231/tcp/4001/p2p/12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
		// "/ip4/43.157.46.194/tcp/4001/p2p/12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
		// "/ip4/43.153.84.212/tcp/4001/p2p/12D3KooWC37TxYV9UGrcxwi3kmupGaDNC5YTo1BDL7TrWQHPfh5S",
	}

	DefaultTestnetBootstrapAddrs = []string{
		// "/ip4/23.94.43.104/tcp/4011/p2p/12D3KooWGkqSo8BZh9GMWpgBnFayG99KuAP3k8fNSC6Nc7RwX76y",
		// "/ip4/192.227.231.231/tcp/4011/p2p/12D3KooWAJcabjdM2AQBYn6bNpKPHkoRb6DcutS8z59ZmxyAYZtw",
		// "/ip4/43.157.46.194/tcp/4011/p2p/12D3KooWAREpvFoVdj1G97tHp287okV8srZt8Jmokn1gReNg2nEr",
		// "/ip4/43.153.84.212/tcp/4011/p2p/12D3KooWAmT26qKkGRoWQjLHFbgiRC8wpFLhrGYgSK5a3MCh6ah5",
	}

	DefaultMainnetSNFServers = []string{
		// "12D3KooWD1GpGf11qVtcDhat8q8rB2du9nohFEFu2DgciUYWY2BC",
		// "12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
		// "12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
		// "12D3KooWC37TxYV9UGrcxwi3kmupGaDNC5YTo1BDL7TrWQHPfh5S",
	}

	DefaultTestnetSNFServers = []string{
		// "12D3KooWGkqSo8BZh9GMWpgBnFayG99KuAP3k8fNSC6Nc7RwX76y",
		// "12D3KooWAJcabjdM2AQBYn6bNpKPHkoRb6DcutS8z59ZmxyAYZtw",
		// "12D3KooWAREpvFoVdj1G97tHp287okV8srZt8Jmokn1gReNg2nEr",
		// "12D3KooWAmT26qKkGRoWQjLHFbgiRC8wpFLhrGYgSK5a3MCh6ah5",
	}
)

// Config defines the configuration options for Mobazha.
//
// See loadConfig for details on the configuration load process.
type Config struct {
	ConfigVersion          uint     `long:"configversion" description:"Configuration file version"`
	ShowVersion            bool     `short:"v" long:"version" description:"Display version information and exit"`
	ConfigFile             string   `short:"C" long:"configfile" description:"Path to configuration file"`
	DataDir                string   `short:"d" long:"datadir" description:"Directory to store data"`
	LogDir                 string   `long:"logdir" description:"Directory to log output."`
	LogLevel               string   `short:"l" long:"loglevel" description:"set the logging level [debug, info, notice, warning, error, critical]" default:"info"`
	BoostrapAddrs          []string `long:"bootstrapaddr" description:"Override the default bootstrap addresses with the provided values"`
	SwarmAddrs             []string `long:"swarmaddr" description:"Override the default swarm addresses with the provided values"`
	GatewayAddr            string   `long:"gatewayaddr" description:"Override the default gateway address with the provided value"`
	StoreAndForwardServers []string `long:"snfserver" description:"A peerID of a store and forward server to use for receiving messages while offline."`
	Testnet                bool     `short:"t" long:"testnet" description:"Use the test network"`
	WalletTestnet          bool     `long:"wallettestnet" description:"Use testnet for wallet transactions (coins and chains)"`
	Regtest                bool     `long:"regtest" description:"Use the regtest network for UTXO wallets (generates bcrt1q addresses for BTC)"`
	DisableNATPortMap      bool     `long:"noupnp" description:"Disable use of upnp."`
	UseSSL                 bool     `long:"ssl" description:"Use SSL on the API"`
	SSLCertFile            string   `long:"sslcertfile" description:"Path to the SSL certificate file"`
	SSLKeyFile             string   `long:"sslkeyfile" description:"Path to the SSL key file"`
	APIUsername            string   `short:"u" long:"apiusername" description:"The username to use with the API authentication"`
	APIPassword            string   `short:"P" long:"apipassword" description:"The password to use with the API authentication"`
	APICookie              string   `long:"apicookie" description:"A cookie to use for authentication in addition or in place of the un/pw. If set the cookie must be put in the request header."`
	APIAllowedIPs          []string `long:"allowedip" description:"Only allow API connections from these IP addresses"`
	APIAllowAllOrigins     bool     `long:"apiallowallorigins" description:"Cors option to allow all origins on the API."`
	APIPublicGateway       bool     `long:"publicgateway" description:"When this option is used only public GET methods will be allowed in the API"`
	Profile                string   `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	CPUProfile             string   `long:"cpuprofile" description:"Write CPU profile to the specified file"`
	InfrastructureOnly     bool     `long:"infraonly" description:"Run as infrastructure-only node (P2P host, DHT, SNF) without business services."`
	UserAgentComment       string   `long:"uacomment" description:"Comment to add to the user agent."`
	EnableSNFServer        bool     `long:"enablesnfserver" description:"Enable this node to operate as a store-and-forward server."`
	SNFServerPeers         []string `long:"snfpeer" description:"A list of other store-and-forward servers to replicate snf data to. This is only used when the snf server is enabled."`
	Tor                    bool     `long:"tor" description:"Proxy all incoming and outgoing connections over the Tor network exclusively."`
	DualStack              bool     `long:"dualstack" description:"Listen for incoming connections via Tor in addition to via the clearnet. This mode is not private."`
	DHTClientOnly          bool     `long:"dhtclientonly" description:"Disable participating in serving data in the DHT. This should be used if your node is undialable."`
	NetConfigEndpoint      string   `long:"netconfigendpoint" description:"Override the default net config endpoint with the provided value"`
	NetDBEndpoint          string   `long:"netdbendpoint" description:"Override the default NetDB endpoint for search index sync"`
	RelayAPIURL            string   `long:"relayapiurl" description:"Platform Relay API URL for gas fee payment (EVM/Solana CANCELABLE payments)"`

	// IdentityKey is an optional externally-provided identity key in libp2p marshaled format.
	// When set, the node uses this key instead of generating one from a mnemonic.
	// This is used by mobazha_hosting to inject keys from KeyVault.
	IdentityKey []byte `no-flag:"true" description:"External identity key (libp2p marshaled format)"`

	// SaaSMode marks this node as a SaaS tenant node. When true, the builder:
	//   - Skips full P2P infrastructure (uses minimal libp2p Host for identity only)
	//   - Uses SharedDB for multi-tenant data isolation (TenantDB)
	//   - May use lighter alternatives where available
	// Set by mobazha_hosting for non-default tenant nodes.
	SaaSMode bool `no-flag:"true" description:"SaaS tenant node mode (lightweight P2P, shared DB)"`

	// SharedDB is an optional *gorm.DB connection for multi-tenant shared database.
	// When set, the node uses TenantDB (tenant-scoped wrapper) instead of creating
	// its own SQLite file. Used together with SaaSMode by mobazha_hosting.
	// The value must be a *gorm.DB pointer.
	SharedDB interface{} `no-flag:"true" description:"Shared GORM DB connection for multi-tenant mode"`

	// HTTPProxyTrustedPeers is a list of peer IDs allowed to proxy HTTP
	// requests to this node via the libp2p HTTP proxy protocol. Typically
	// the SaaS default node's peer ID. Only used in standalone (non-SaaS) mode.
	HTTPProxyTrustedPeers []string `long:"httpproxytrustedpeer" description:"Peer IDs trusted to proxy HTTP requests via libp2p"`

	// HTTPProxyLocalAddr is the local API address that the libp2p HTTP proxy
	// handler forwards requests to. Defaults to "http://127.0.0.1:5102".
	HTTPProxyLocalAddr string `long:"httpproxylocaladdr" description:"Local API address for libp2p HTTP proxy forwarding"`

	// SaaSAPIURL is the SaaS platform URL for standalone stores to register
	// and send heartbeats. When set together with StandaloneAPIKey, the node
	// starts a heartbeat sender on boot.
	SaaSAPIURL string `long:"saasapiurl" description:"SaaS platform URL for store registration and heartbeat"`

	// StandaloneAPIKey is the API key obtained during store registration
	// with the SaaS platform, used to authenticate heartbeat requests.
	StandaloneAPIKey string `long:"standaloneapikey" description:"API key for SaaS platform authentication"`

	// StandaloneConnectivity describes how this standalone store is reachable:
	// "public" (direct HTTPS), "tunnel" (Cloudflare Tunnel), or "nat" (libp2p only).
	StandaloneConnectivity string `long:"standaloneconnectivity" description:"Connectivity mode: public, tunnel, or nat"`

	// CasdoorCertificate is the PEM-encoded public certificate of the SaaS
	// Casdoor instance. Used by standalone nodes to verify JWT tokens issued
	// by SaaS Casdoor (for Mini App management via SaaS proxy). When empty,
	// only Basic Auth is available. The certificate can be provided directly
	// in the config file or fetched from SaaSAPIURL on first boot.
	CasdoorCertificate string `long:"casdoorcertificate" description:"PEM certificate from SaaS Casdoor for JWT validation"`

	// OwnerUserID is the Casdoor User ID of the store owner, used for
	// JWT admin authorization in standalone mode. Typically fetched from
	// SaaS store_registry.owner_user_id during startup. When empty,
	// JWT auth falls back to peerID-based comparison (legacy behavior).
	OwnerUserID string `long:"owneruserid" description:"Casdoor User ID of the store owner for JWT admin auth"`

	// Matrix homeserver configuration (injected by hosting in SaaS mode).
	// These override the corresponding NetConfig fields loaded from the remote endpoint.
	MatrixInternalURL        string `no-flag:"true"`
	MatrixServerName         string `no-flag:"true"`
	MatrixRegistrationSecret string `no-flag:"true"`
	MatrixSDKLogLevel        string `no-flag:"true"`

	// MatrixCryptoStore allows hosting (SaaS) to inject a shared PostgreSQL
	// *dbutil.Database for mautrix-go crypto state. When non-nil,
	// mautrixChatService uses this instead of a per-tenant SQLite file.
	// Tenant isolation is via CryptoHelper.DBAccountID = peerID.
	MatrixCryptoStore interface{} `no-flag:"true"`

	// Platform AI Gateway — injected by hosting to provide zero-config AI.
	// When the tenant hasn't configured BYOK, AIConfig() falls back to this.
	PlatformAIProvider   string `no-flag:"true"`
	PlatformAIAPIKey     string `no-flag:"true"`
	PlatformAIModel      string `no-flag:"true"`
	PlatformAIBaseURL    string `no-flag:"true"`
	PlatformAIDailyLimit int    `no-flag:"true"` // per-tenant daily call limit (0 = unlimited)
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//  1. Start with a default config with sane settings
//  2. Pre-parse the command line to check for an alternative config file
//  3. Load configuration file overwriting defaults with any specified options
//  4. Parse CLI options and overwrite/add any specified options
//
// The above results in Mobazha functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func LoadConfig(dataDir string) (*Config, error) {
	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := Config{}
	preParser := flags.NewParser(&preCfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, err
		}
	}

	if preCfg.DataDir != "" {
		preCfg.ConfigFile = filepath.Join(preCfg.DataDir, defaultConfigFilename)
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version.String())
		os.Exit(0)
	}

	// Default config.
	cfg := preCfg
	if dataDir != "" {
		cfg.DataDir = dataDir
		if preCfg.Testnet {
			cfg.DataDir = dataDir + "-testnet"
		}
	} else if preCfg.DataDir == "" {
		cfg.DataDir = DefaultHomeDir
		if preCfg.Testnet {
			cfg.DataDir = DefaultHomeDir + "-testnet"
		}
	}
	if preCfg.ConfigFile == "" {
		cfg.ConfigFile = filepath.Join(cfg.DataDir, defaultConfigFilename)
	}

	// Load additional config from file.
	var configFileError error
	parser := flags.NewParser(&cfg, flags.Default|flags.IgnoreUnknown)
	if _, err := os.Stat(cfg.ConfigFile); os.IsNotExist(err) {
		err := createDefaultConfigFile(cfg.ConfigFile, cfg.Testnet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating a default config file: %v\n", err)
			return nil, err
		}
	}

	err = flags.NewIniParser(parser).ParseFile(cfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, err
		}
		configFileError = err
	}

	checkConfigFileMigration := func() error {
		if cfg.ConfigVersion < 4 {
			err = os.Rename(cfg.ConfigFile, cfg.ConfigFile+"_bak_"+time.Now().Format("2006-01-02"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error backup config file: %v\n", err)
				return err
			}

			err := createDefaultConfigFile(cfg.ConfigFile, cfg.Testnet)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating a default config file: %v\n", err)
				return err
			}

			err = flags.NewIniParser(parser).ParseFile(cfg.ConfigFile)
			if err != nil {
				if _, ok := err.(*os.PathError); !ok {
					fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
					fmt.Fprintln(os.Stderr, usageMessage)
					return err
				}
				configFileError = err
			}
		}
		return nil
	}
	err = checkConfigFileMigration()
	if err != nil {
		return nil, err
	}

	if cfg.Tor && cfg.DualStack {
		return nil, errors.New("tor and dualstack options cannot be used together")
	}

	// Ensure LogLevel has a valid default. When LoadConfig is called from a
	// hosting process (not a standalone node), the pre-parser may not apply
	// struct tag defaults, leaving LogLevel as "".
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	_, ok := LogLevelMap[strings.ToLower(cfg.LogLevel)]
	if !ok {
		return nil, errors.New("invalid log level")
	}

	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	if cfg.LogDir == "" {
		cfg.LogDir = cleanAndExpandPath(path.Join(cfg.DataDir, "logs"))
	}

	// Validate profile port number
	if cfg.Profile != "" {
		profilePort, err := strconv.Atoi(cfg.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			return nil, fmt.Errorf("%d: The profile port must be between 1024 and 65535", profilePort)
		}
	}

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		log.Errorf("%v", configFileError)
	}
	return &cfg, nil
}

// SetupLogging sets up logging for this node
func SetupLogging(logDir, logLevel string) {
	level, ok := LogLevelMap[strings.ToLower(logLevel)]
	if !ok {
		level = logging.INFO
	}

	writers := []io.Writer{os.Stdout}
	if logDir != "" {
		rotator := &lumberjack.Logger{
			Filename:   path.Join(logDir, defaultLogFilename),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
		writers = append(writers, rotator)
	}

	logging.Configure(logging.Config{
		Level:   level,
		Format:  logging.FormatText,
		Writers: writers,
	})
}

// createDefaultConfig copies the sample-bchd.conf content to the given destination path,
// and populates it with some randomly generated RPC username and password.
func createDefaultConfigFile(destinationPath string, testnet bool) error {
	// Create the destination directory if it does not exists
	err := os.MkdirAll(filepath.Dir(destinationPath), 0700)
	if err != nil {
		return err
	}

	sampleBytes, err := Asset("sample-mobazha.conf")
	if err != nil {
		return err
	}
	src := bytes.NewReader(sampleBytes)

	dest, err := os.OpenFile(destinationPath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dest.Close()

	// We copy every line from the sample config file to the destination,
	// only replacing the two lines for rpcuser and rpcpass
	reader := bufio.NewReader(src)
	for err != io.EOF {
		var line string
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		if _, err := dest.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(DefaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}
