package wallet_interface

import "errors"

var ErrChainClientStarted = errors.New("chain client has already been started")
var ErrChainClientStopped = errors.New("chain client has already been stopped")

type BlockSubscription struct {
	Out   chan BlockInfo
	Close func()
}

type TransactionSubscription struct {
	Out         chan Transaction
	Subscribe   chan []AddressEx
	Unsubscribe chan []AddressEx
	Close       func()
}

type EstimateFeeRes struct {
	FeePerTx   Amount `json:"feePerTx,omitempty"`
	FeePerUnit Amount `json:"feePerUnit,omitempty"`
	FeeLimit   Amount `json:"feeLimit,omitempty"`
}

type ChainClient interface {
	GetBlockchainInfo() (BlockInfo, error)

	GetAddressTransactions(addr AddressEx, fromHeight uint64) ([]Transaction, error)

	GetTransaction(id TransactionID, coinType CoinType) (*Transaction, error)

	IsBlockInMainChain(block BlockInfo) (bool, error)

	EstimateFee(txsize int) (map[FeeLevel]EstimateFeeRes, error)

	SubscribeTransactions(addrs []AddressEx) (*TransactionSubscription, error)

	SubscribeBlocks() (*BlockSubscription, error)

	Broadcast(serializedTx []byte) error

	Open() error

	Close() error
}
