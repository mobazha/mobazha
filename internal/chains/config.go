package chains

import (
	"fmt"
	"path"

	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/repo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
)

var (
	DefaultHomeDir = repo.AppDataDir("multiwallet", false)
	DefaultLogDir  = path.Join(DefaultHomeDir, "logs")
)

// Option is a multiwallet option type.
type Option func(*Config) error

type Config struct {
	NodeID     string
	Chains     []iwallet.ChainType
	ChainAPIs  map[iwallet.ChainType]APIUrls
	UseTestnet bool
	UseRegtest bool
	DataDir    string
	LogDir     string
	LogLevel   logging.Level
	NetConfig  *config.NetConfig
}

type APIUrls struct {
	MainnetRpc             []string // HTTPS API
	MainnetWss             []string // WebSocket API
	MainnetRegistryAddress string   // ContractManager contract address
	MainnetEscrowAddress   string   // Pre-resolved escrow contract address (optional, avoids Registry RPC)
	TestnetRpc             []string
	TestnetWss             []string
	TestnetRegistryAddress string // ContractManager contract address
	TestnetEscrowAddress   string // Pre-resolved escrow contract address (optional, avoids Registry RPC)
}

// Defaults are the default options. This option will be automatically
// prepended to any options you pass to the constructor.
var Defaults = func(cfg *Config) error {
	cfg.Chains = iwallet.GetAllSupportedChainTypes()
	cfg.ChainAPIs = map[iwallet.ChainType]APIUrls{
		iwallet.ChainBitcoin: {
			MainnetWss: []string{
				"https://btc1.trezor.io",
				"https://btc2.trezor.io",
				"https://btc3.trezor.io",
				"https://btc4.trezor.io",
				"https://btc5.trezor.io",
				"https://btc1.mobazha.info",
			},
			TestnetWss: []string{
				"https://tbtc1.trezor.io",
				"https://tbtc2.trezor.io",
			},
		},
		iwallet.ChainBitcoinCash: {
			MainnetWss: []string{
				"https://bch1.trezor.io",
				"https://bch2.trezor.io",
				"https://bch3.trezor.io",
				"https://bch4.trezor.io",
				"https://bch5.trezor.io",
				"https://bch1.mobazha.info",
			},
			// bchd.greyh.at:8335
			TestnetWss: []string{"bchd-testnet.greyh.at:18335"},
		},
		iwallet.ChainLitecoin: {
			MainnetWss: []string{
				"https://ltc1.trezor.io",
				"https://ltc2.trezor.io",
				"https://ltc3.trezor.io",
				"https://ltc4.trezor.io",
				"https://ltc5.trezor.io",
				"https://ltc1.mobazha.info",
			},
			TestnetWss: []string{"https://tltc1.mobazha.info"},
		},
		iwallet.ChainZCash: {
			MainnetWss: []string{
				"https://zec1.trezor.io",
				"https://zec2.trezor.io",
				"https://zec3.trezor.io",
				"https://zec4.trezor.io",
				"https://zec5.trezor.io",
				"https://zec1.mobazha.info",
			},
			TestnetWss: []string{"https://tzec.blockbook.api.openbazaar.org"},
		},
		iwallet.ChainEthereum: {
			MainnetRpc:             []string{"https://sepolia.drpc.org", "https://eth-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetWss:             []string{"wss://eth-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetRegistryAddress: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
			TestnetRpc:             []string{"https://sepolia.drpc.org", "https://eth-sepolia.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			TestnetWss:             []string{"wss://eth-sepolia.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			TestnetRegistryAddress: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
		},
		iwallet.ChainBase: {
			MainnetRpc:             []string{"https://mainnet.base.org", "https://base-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetWss:             []string{"wss://base-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetRegistryAddress: "0x4c1a1b21c4471ca57145ee08404cbaf9c8b83991",
			TestnetRpc:             []string{"https://sepolia.base.org", "https://base-sepolia.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			TestnetWss:             []string{"wss://base-sepolia.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			TestnetRegistryAddress: "0x24C0f3049Cc8188631bF74A9DE944223C9772156",
		},
		iwallet.ChainBSC: {
			MainnetRpc:             []string{"https://56.rpc.thirdweb.com", "https://bsc-dataseed.binance.org"},
			TestnetRpc:             []string{"https://data-seed-prebsc-2-s2.binance.org:8545"},
			MainnetRegistryAddress: "0x4c1a1b21c4471ca57145ee08404cbaf9c8b83991",
			TestnetRegistryAddress: "0x8EC3ec712fd24b11c2Ce369a04249e7C439CB339",
		},
		iwallet.ChainPolygon: {
			MainnetRpc:             []string{"https://polygon-rpc.com", "https://polygon-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetWss:             []string{"wss://polygon-mainnet.g.alchemy.com/v2/H12dBz723H5BWOjtRxt79gx3QRrpw4hd"},
			MainnetRegistryAddress: "0x97EA76863a5843408E2727DCacCDa6A499BA747F",
			// TestnetRpc: "https://polygon-mainnet.infura.io/v3/88f4e1bc34ca40b995af9f2fd8ecaf87",
			// TestnetWss: "wss://polygon-mumbai.infura.io/ws/v3/88f4e1bc34ca40b995af9f2fd8ecaf87",
			TestnetRpc:             []string{"https://rpc-amoy.polygon.technology"},
			TestnetWss:             []string{"wss://polygon-mumbai.g.alchemy.com/v2/JaCwts18MYqD0wHQ8VjSSophnEND_yfO"},
			TestnetRegistryAddress: "0xb46a91f9546b6650453F2B54705E0e8e25C85247",
		},
		iwallet.ChainConflux: {
			MainnetRpc:             []string{"https://evm.confluxrpc.com/GGna2h7aru3XSNFpeLrfT2ahYqM3YFeiX4FfgCgChdSfM9CbMXPik9762LBpKrzbC4c7kENDz2ikAYdyHQWjGiDvJ"},
			MainnetWss:             []string{"wss://evm.confluxrpc.org/ws"},
			MainnetRegistryAddress: "0x17ebC8FeE90E7556E1E12Aa42604477D6A243324",
			// TestnetRpc: "https://polygon-mainnet.infura.io/v3/88f4e1bc34ca40b995af9f2fd8ecaf87",
			// TestnetWss: "wss://polygon-mumbai.infura.io/ws/v3/88f4e1bc34ca40b995af9f2fd8ecaf87",
			TestnetRpc:             []string{"https://evmtestnet.confluxrpc.com"},
			TestnetWss:             []string{"wss://evmtestnet.confluxrpc.org/ws"},
			TestnetRegistryAddress: "0x93ecc969ff6C9e822F4AFD80acb59848eB9b9bf7",
		},
		iwallet.ChainSolana: {
			MainnetRpc:             []string{"https://api.devnet.solana.com"},
			MainnetRegistryAddress: "6LmWMjAMAfVdc8mpgPjHvFLa2sbcudiLiJT3bAGRYMMD",
			TestnetRpc:             []string{"https://api.devnet.solana.com"},
			TestnetRegistryAddress: "6LmWMjAMAfVdc8mpgPjHvFLa2sbcudiLiJT3bAGRYMMD",
		},
		iwallet.ChainTRON: {
			MainnetRpc: []string{"https://api.trongrid.io", "https://api.tronstack.io"},
			TestnetRpc: []string{"https://api.shasta.trongrid.io"},
		},
	}
	cfg.LogLevel = logging.INFO
	cfg.DataDir = DefaultHomeDir
	cfg.LogDir = DefaultLogDir
	return nil
}

// Apply applies the given options to this Option
func (cfg *Config) Apply(opts ...Option) error {
	for i, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("multiwallet option %d failed: %s", i, err)
		}
	}
	return nil
}

// NodeID configures the multiwallet to use the provided node ID.
func NodeID(nodeID string) Option {
	return func(cfg *Config) error {
		cfg.NodeID = nodeID
		return nil
	}
}

// DataDir configures the multiwallet to use the provided data directory
//
// Defaults to a multiwallet directory inside the os-specific home directory.
func DataDir(dataDir string) Option {
	return func(cfg *Config) error {
		cfg.DataDir = dataDir
		return nil
	}
}

// LogDir configures the multiwallet to use the provided log directory
//
// Defaults to a log directory inside the default home directory.
func LogDir(logDir string) Option {
	return func(cfg *Config) error {
		cfg.LogDir = logDir
		return nil
	}
}

// Testnet configures the multiwallet to use the testnet for all coins.
//
// Defaults to false which will use mainnet.
func Testnet(testnet bool) Option {
	return func(cfg *Config) error {
		cfg.UseTestnet = testnet
		return nil
	}
}

// Regtest configures the multiwallet to use the regtest network for UTXO coins.
// When true, UTXO wallets generate regtest-compatible addresses (e.g. bcrt1q for BTC).
func Regtest(regtest bool) Option {
	return func(cfg *Config) error {
		cfg.UseRegtest = regtest
		return nil
	}
}

// Wallets configures the multiwallet to use the provided wallets.
//
// Defaults to all implemented wallets.
func Chains(chains []iwallet.ChainType) Option {
	return func(cfg *Config) error {
		cfg.Chains = chains
		return nil
	}
}

// ChainAPIs configures the multiwallet to use the provided wallet API urls.
// The provided map will override existing config options. If the map does not
// contain a specific key, it will not override the default.
//
// Defaults to all default APIs.
func ChainAPIs(apis map[iwallet.ChainType]APIUrls) Option {
	return func(cfg *Config) error {
		for ct, api := range apis {
			cfg.ChainAPIs[ct] = api
		}
		return nil
	}
}

// LogLevel sets the log level for the wallet.
//
// Defaults to INFO.
func LogLevel(level logging.Level) Option {
	return func(cfg *Config) error {
		cfg.LogLevel = level
		return nil
	}
}

// NetConfig provides other configurations from the network.
func NetConfig(data *config.NetConfig) Option {
	return func(cfg *Config) error {
		cfg.NetConfig = data
		return nil
	}
}


// EscrowAddresses configures pre-resolved escrow contract addresses for chains.
// When set, chain clients skip Registry RPC queries for contract address lookup.
// This is used in SaaS mode where contract addresses are stable and configurable.
func EscrowAddresses(addrs map[iwallet.ChainType]string) Option {
	return func(cfg *Config) error {
		for chain, addr := range addrs {
			api := cfg.ChainAPIs[chain]
			api.MainnetEscrowAddress = addr
			api.TestnetEscrowAddress = addr
			cfg.ChainAPIs[chain] = api
		}
		return nil
	}
}
