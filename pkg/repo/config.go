package repo

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
	DisableNATPortMap      bool     `long:"noupnp" description:"Disable use of upnp."`
	IPNSQuorum             uint     `long:"ipnsquorum" description:"The size of the IPNS quorum to use. Smaller is faster but less up-to-date." default:"2"`
	NoIPNSPubsub           bool     `long:"noipnsps" description:"Disable use of IPNS pubsub."`
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
	IPFSOnly               bool     `long:"ipfsonly" description:"Disable all Mobazha functionality except the IPFS networking."`
	EnabledWallets         []string `long:"enabledwallet" description:"Only enable wallets in this list. Available wallets: [BTC, BCH, LTC, ZEC, ETH, BNB, MATICUSDT, MATICUSDC]"`
	UserAgentComment       string   `long:"uacomment" description:"Comment to add to the user agent."`
	EnableSNFServer        bool     `long:"enablesnfserver" description:"Enable this node to operate as a store-and-forward server."`
	SNFServerPeers         []string `long:"snfpeer" description:"A list of other store-and-forward servers to replicate snf data to. This is only used when the snf server is enabled."`
	Tor                    bool     `long:"tor" description:"Proxy all incoming and outgoing connections over the Tor network exclusively."`
	DualStack              bool     `long:"dualstack" description:"Listen for incoming connections via Tor in addition to via the clearnet. This mode is not private."`
	DHTClientOnly          bool     `long:"dhtclientonly" description:"Disable participating in serving data in the DHT. This should be used if your node is undialable."`
}
