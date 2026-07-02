package evm

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// EVMClientConfig holds configuration for creating a shared EVM chain client.
// Each supported EVM chain (BSC, Ethereum, Polygon, Base, Conflux) needs its own config.
type EVMClientConfig struct {
	ChainType       iwallet.ChainType
	RpcURL          string // HTTPS RPC endpoint (balance, receipt, broadcast)
	WsURL           string // Optional WSS endpoint for eth_subscribe (managed escrow LiveMonitor prefers this)
	RegistryAddress string // ContractManager contract address (for dynamic escrow lookup)
	EscrowAddress   string // Optional pre-resolved escrow address (skips Registry query)
	Testnet         bool
}

// EVMClientFactory creates EVM chain clients.
// This interface allows external packages (like hosting) to create EthClient instances
// without directly importing internal packages.
type EVMClientFactory interface {
	// CreateClient creates a new EVM chain client with RPC connection.
	// The returned ChainClient supports GetTransaction, GetContractAddress (via Registry),
	// and other RPC-dependent operations.
	CreateClient(cfg EVMClientConfig) (iwallet.ChainClient, error)
}

// DefaultFactory is the default EVM client factory implementation.
// It should be set by the internal package during initialization.
var DefaultFactory EVMClientFactory

// NewSharedClient creates an EVM chain client using the DefaultFactory.
// This is a convenience function for hosting to create shared per-chain clients.
// If DefaultFactory is not set, it returns an error.
func NewSharedClient(cfg EVMClientConfig) (iwallet.ChainClient, error) {
	if DefaultFactory == nil {
		return nil, ErrFactoryNotSet
	}
	return DefaultFactory.CreateClient(cfg)
}
