package evm

import (
	pkgevm "github.com/mobazha/mobazha3.0/pkg/evm"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
)

var factoryLog = logging.MustGetLogger("evm-factory")

// defaultEVMClientFactory implements pkgevm.EVMClientFactory
type defaultEVMClientFactory struct{}

func init() {
	// Register the factory when this package is imported
	pkgevm.DefaultFactory = &defaultEVMClientFactory{}
}

// CreateClient creates a new EthClient with full RPC connection.
// The returned ChainClient is actually *EthClient, so wallet code can type-assert
// to access EVM-specific methods like GetRecommendedContractVersion().
func (f *defaultEVMClientFactory) CreateClient(cfg pkgevm.EVMClientConfig) (iwallet.ChainClient, error) {
	coinType, err := iwallet.RequireCanonicalNativeCoinType(cfg.ChainType)
	if err != nil {
		return nil, err
	}

	var opts []EthClientOption
	if cfg.EscrowAddress != "" {
		opts = append(opts, WithEscrowAddress(cfg.EscrowAddress))
	}

	client, err := NewEthClient(coinType, cfg.Testnet, cfg.RpcURL, cfg.RegistryAddress, factoryLog, opts...)
	if err != nil {
		return nil, err
	}

	factoryLog.Infof("Created shared EVM client for %s (testnet=%v, rpc=%s)", cfg.ChainType, cfg.Testnet, cfg.RpcURL)
	return client, nil
}
