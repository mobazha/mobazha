package utxo

import (
	"encoding/hex"

	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// UTXOChainClient implements iwallet.ChainClient using ChainOperations interface
// This works with both *Monitor (standalone mode) and UTXOMonitorService (shared mode)
type UTXOChainClient struct {
	ops   pkgutxo.ChainOperations
	chain iwallet.ChainType
}

// NewUTXOChainClient creates a new ChainClient backed by ChainOperations
// ops can be either *Monitor or UTXOMonitorService
func NewUTXOChainClient(ops pkgutxo.ChainOperations, chain iwallet.ChainType) *UTXOChainClient {
	return &UTXOChainClient{
		ops:   ops,
		chain: chain,
	}
}

// GetTransaction gets a transaction by ID
func (c *UTXOChainClient) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	return c.ops.GetTransaction(c.chain, id.String())
}

// EstimateFee estimates fees for different fee levels
func (c *UTXOChainClient) EstimateFee(txsize int) (map[iwallet.FeeLevel]iwallet.EstimateFeeRes, error) {
	result := make(map[iwallet.FeeLevel]iwallet.EstimateFeeRes)

	// Get fee estimates for different target blocks
	priorityFee := c.ops.GetFeeEstimate(c.chain, 1)  // 1 block - priority
	normalFee := c.ops.GetFeeEstimate(c.chain, 6)    // 6 blocks - normal
	economicFee := c.ops.GetFeeEstimate(c.chain, 36) // 36 blocks - economic

	result[iwallet.FlPriority] = iwallet.EstimateFeeRes{
		FeePerUnit: iwallet.NewAmount(int64(priorityFee)),
		FeePerTx:   iwallet.NewAmount(int64(priorityFee) * int64(txsize)),
	}
	result[iwallet.FlNormal] = iwallet.EstimateFeeRes{
		FeePerUnit: iwallet.NewAmount(int64(normalFee)),
		FeePerTx:   iwallet.NewAmount(int64(normalFee) * int64(txsize)),
	}
	result[iwallet.FlEconomic] = iwallet.EstimateFeeRes{
		FeePerUnit: iwallet.NewAmount(int64(economicFee)),
		FeePerTx:   iwallet.NewAmount(int64(economicFee) * int64(txsize)),
	}

	return result, nil
}

// Broadcast broadcasts a serialized transaction
func (c *UTXOChainClient) Broadcast(serializedTx []byte) error {
	txHex := hex.EncodeToString(serializedTx)
	_, err := c.ops.BroadcastTransaction(c.chain, txHex)
	return err
}

// IsConnected returns true if at least one source is healthy
func (c *UTXOChainClient) IsConnected() bool {
	return c.ops.IsHealthy(c.chain)
}

// BlockchainInfo returns basic blockchain info (stub implementation)
func (c *UTXOChainClient) BlockchainInfo() (iwallet.BlockInfo, error) {
	// This is a stub - we don't need detailed blockchain info for most operations
	return iwallet.BlockInfo{
		Height: 0,
	}, nil
}

// GetAddressTransactions gets transactions for an address
func (c *UTXOChainClient) GetAddressTransactions(address string, scriptPubKey []byte) ([]iwallet.Transaction, error) {
	return c.ops.GetAddressTransactions(c.chain, address, scriptPubKey)
}

// SubscribeTransactions returns nil - use callback mechanism for notifications
func (c *UTXOChainClient) SubscribeTransactions() <-chan iwallet.Transaction {
	return nil
}

// Close closes the client (no-op, ops manages lifecycle)
func (c *UTXOChainClient) Close() error {
	return nil
}

// Ensure UTXOChainClient implements ChainClient
var _ iwallet.ChainClient = (*UTXOChainClient)(nil)
