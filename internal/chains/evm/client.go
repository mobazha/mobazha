package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mobazha/ethereum-watcher/rpc"
	"github.com/mobazha/mobazha/pkg/logging"
	"github.com/mobazha/mobazha/pkg/redact"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"

	contract "github.com/mobazha/mobazha/internal/chains/evm/contract"
)

/*
	!! Important URL information from Infura
	Mainnet	JSON-RPC over HTTPs	https://mainnet.infura.io/v3/YOUR-PROJECT-ID
	Mainnet	JSON-RPC over websockets	wss://mainnet.infura.io/ws/v3/YOUR-PROJECT-ID
	Ropsten	JSON-RPC over HTTPS	https://ropsten.infura.io/v3/YOUR-PROJECT-ID
	Ropsten	JSON-RPC over websockets	wss://ropsten.infura.io/ws/v3/YOUR-PROJECT-ID
	Rinkeby	JSON-RPC over HTTPS	https://rinkeby.infura.io/v3/YOUR-PROJECT-ID
	Rinkeby	JSON-RPC over websockets	wss://rinkeby.infura.io/ws/v3/YOUR-PROJECT-ID
	Kovan	JSON-RPC over HTTPS	https://kovan.infura.io/v3/YOUR-PROJECT-ID
	Kovan	JSON-RPC over websockets	wss://kovan.infura.io/ws/v3/YOUR-PROJECT-ID
	Görli	JSON-RPC over HTTPS	https://goerli.infura.io/v3/YOUR-PROJECT-ID
	Görli	JSON-RPC over websockets	wss://goerli.infura.io/ws/v3/YOUR-PROJECT-ID
*/

const (
	maxGasLimit = 3000000

	EscrowContractName = "escrow"
)

// ErrV1ChainNotSupported is returned by GetRecommendedContractVersion
// when the chain has no V1 ContractManager Registry deployed (Phase
// managed EVM v0.3.0 Sprint 1 D8 — promoted EVM L2 set: Arbitrum, Optimism,
// Avalanche, Gnosis, Celo, Mantle, zkSync Era, Scroll, Linea). On these
// chains order creation MUST route through the V2 managed EVM adapter; hitting
// V1 is a routing bug, not a recoverable fault. Callers map this error
// to CHAIN_NOT_SUPPORTED at the user-visible boundary.
//
// Detection happens at NewEthClient time: a zero-address registry
// argument skips the Registry ABI binding entirely, leaving
// EthClient.registry == nil. Subsequent V1 lookups then trip this
// sentinel instead of dispatching an eth_call against the zero
// address (which would return empty data and surface as a confusing
// generic decode error).
var ErrV1ChainNotSupported = errors.New("chain has no V1 ContractManager Registry — V1 escrow paths must not be used on this chain (route via managed EVM adapter V2 instead)")

// EthClient represents the eth client
type EthClient struct {
	CoinType iwallet.CoinType
	Testnet  bool

	*ethclient.Client
	rpcUrl string
	wsUrl  string
	logger *logging.Logger

	rpc *rpc.EthBlockChainRPCWithRetry

	registry *contract.Registry

	// wsClient is an optional separate WebSocket connection used for
	// eth_subscribe (log subscriptions). When set, SubscribeFilterLogs
	// uses this client instead of the primary HTTP-based Client. This
	// allows managed EVM live monitor to use WSS for real-time events while
	// the primary client handles RPC reads/writes over HTTPS.
	wsClient *ethclient.Client

	// escrowAddr is a pre-resolved escrow contract address.
	// When set, GetRecommendedContractVersion() returns this address
	// without querying the Registry contract via RPC.
	escrowAddr *common.Address
}

// NewEthClient returns a new eth client with RPC connection.
// If WithEscrowAddress is set, the Registry binding is skipped (escrow address is pre-resolved).
func NewEthClient(coinType iwallet.CoinType, testnet bool, rpcUrl string, registryAddress string, logger *logging.Logger, opts ...EthClientOption) (*EthClient, error) {
	client := &EthClient{
		CoinType: coinType,
		Testnet:  testnet,
		rpcUrl:   rpcUrl,
		logger:   logger,
	}
	for _, opt := range opts {
		opt(client)
	}

	// Create RPC connection
	conn, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return nil, fmt.Errorf("dial ETH client failed, coinType: %v, rpcUrl: %v, error: %v", coinType, rpcUrl, err)
	}
	retryRpc, err := rpc.NewEthRPCWithRetry(rpcUrl, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry RPC, %v", err)
	}
	client.Client = conn
	client.rpc = retryRpc

	// Optionally dial a separate WSS connection for log subscriptions.
	// Failure is non-fatal: LiveMonitor will fallback to HTTP polling.
	if client.wsUrl != "" {
		wsConn, wsErr := ethclient.Dial(client.wsUrl)
		if wsErr != nil {
			if logger != nil {
				logger.Warningf("[%s] WSS dial failed (non-fatal, will use HTTP polling): %v", coinType, wsErr)
			}
		} else {
			client.wsClient = wsConn
			if logger != nil {
				logger.Infof("[%s] WSS connection established for log subscriptions: %s", coinType, redact.URL(client.wsUrl))
			}
		}
	}

	// Skip Registry if escrow address is pre-resolved
	if client.escrowAddr != nil {
		return client, nil
	}

	// Phase managed EVM v0.3.0 Sprint 1 D8 — chains without a deployed
	// V1 ContractManager Registry (zero-address sentinel from
	// pkg/evm/defaults.go) skip the Registry binding entirely.
	// Subsequent V1 lookups via GetRecommendedContractVersion fail
	// closed with ErrV1ChainNotSupported. We treat the empty string
	// as equivalent to zero (defensive: legacy configs may not have
	// migrated to the explicit zero literal yet) so neither produces
	// a Registry binding pointing at the zero address.
	if registryAddress == "" || common.HexToAddress(registryAddress) == (common.Address{}) {
		if logger != nil {
			logger.Infof("[%s] V1 ContractManager Registry not deployed (zero-address sentinel) — V1 escrow paths will fail closed; managed EVM adapter V2 is the supported route", coinType)
		}
		return client, nil
	}

	// Create Registry binding for escrow contract lookup
	reg, err := contract.NewRegistry(common.HexToAddress(registryAddress), conn)
	if err != nil {
		if logger != nil {
			logger.Errorf("error initializing Registry contract: %s", err.Error())
		}
		return nil, err
	}
	client.registry = reg

	return client, nil
}

// EthClientOption configures an EthClient.
type EthClientOption func(*EthClient)

// WithEscrowAddress sets a pre-resolved escrow contract address,
// avoiding the need for RPC connection and Registry lookup.
func WithEscrowAddress(addr string) EthClientOption {
	return func(c *EthClient) {
		if addr != "" && common.IsHexAddress(addr) {
			a := common.HexToAddress(addr)
			c.escrowAddr = &a
		}
	}
}

// WithWsURL configures a separate WebSocket connection for eth_subscribe.
// When set, SubscribeFilterLogs uses the WSS endpoint for real-time log
// subscriptions while the primary client handles HTTP RPC operations.
// If dialing fails, the client is still usable (subscriptions fallback
// to the primary connection or HTTP polling via LiveMonitor).
func WithWsURL(wsURL string) EthClientOption {
	return func(c *EthClient) {
		if wsURL != "" {
			c.wsUrl = wsURL
		}
	}
}

// SubscribeFilterLogs overrides the embedded ethclient.Client's method to
// prefer the dedicated WSS connection when available. This enables managed EVM
// LiveMonitor to get real-time log events over WebSocket while the primary
// HTTPS client handles balance queries, receipt lookups, and broadcasts.
func (client *EthClient) SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	if client.wsClient != nil {
		sub, err := client.wsClient.SubscribeFilterLogs(ctx, q, ch)
		if err == nil {
			return sub, nil
		}
		if client.logger != nil {
			client.logger.Warningf("[%s] WSS SubscribeFilterLogs failed, falling back to primary: %v", client.CoinType, err)
		}
	}
	return client.Client.SubscribeFilterLogs(ctx, q, ch)
}

// Close releases both the primary HTTP/RPC and optional WSS connections.
func (client *EthClient) Close() {
	if client.wsClient != nil {
		client.wsClient.Close()
		client.wsClient = nil
	}
	if client.Client != nil {
		client.Client.Close()
	}
}

func (client *EthClient) buildTxFromTxAndReceipt(coinType iwallet.CoinType, transaction *types.Transaction, receipt *types.Receipt) (*iwallet.Transaction, error) {
	if transaction == nil {
		return nil, nil
	}
	if receipt == nil {
		return nil, nil
	}
	blockInfo, err := client.BlockByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		return nil, err
	}

	// Accumulate all ERC20 Transfer events from the transaction receipt.
	// This is important for transactions that trigger multiple transfers,
	// such as RWA instant buy which transfers to both seller and platform fee recipient.
	var resultTxn *iwallet.Transaction
	var totalValue iwallet.Amount

	for _, receiptLog := range receipt.Logs {
		txn, err := client.buildTxFromReceiptLog(coinType, receiptLog, blockInfo)
		if err != nil {
			client.logger.Errorf("buildTxFromReceiptLog failed, %v", err)
			continue
		}

		if txn != nil {
			if resultTxn == nil {
				// First recognized transfer, initialize the result
				resultTxn = txn
				totalValue = txn.Value
			} else {
				// Subsequent transfers, append To entries and accumulate value
				resultTxn.To = append(resultTxn.To, txn.To...)
				totalValue = totalValue.Add(txn.Value)
			}
		}
	}

	// If we found ERC20 transfers, return the accumulated result
	if resultTxn != nil {
		resultTxn.Value = totalValue
		resultTxn.BlockInfo = &iwallet.BlockInfo{
			Height:    receipt.BlockNumber.Uint64(),
			BlockID:   iwallet.BlockID(receipt.BlockHash.Hex()),
			PrevBlock: iwallet.BlockID(blockInfo.ParentHash().Hex()),
			BlockTime: time.Unix(int64(blockInfo.Time()), 0),
		}
		return resultTxn, nil
	}

	// Fallback: No ERC20 transfers found, build from raw transaction (for native ETH transfers)
	from, err := types.Sender(types.LatestSignerForChainID(transaction.ChainId()), transaction)
	if err != nil {
		client.logger.Errorf("get sender from tx %s failed, %v", transaction.Hash().Hex(), err)
		return nil, err
	}

	// For contract deployment, the To address is empty
	to := ""
	if transaction.To() != nil {
		to = EnsureCorrectPrefix(transaction.To().Hex())
	}

	txn := &iwallet.Transaction{
		ID: iwallet.TransactionID(EnsureCorrectPrefix(string(transaction.Hash().Hex()))),
		From: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(EnsureCorrectPrefix(from.Hex()), coinType),
				Amount:  iwallet.NewAmount(transaction.Value()),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(to, coinType),
				Amount:  iwallet.NewAmount(transaction.Value()),
			},
		},
		Value:     iwallet.NewAmount(transaction.Value()),
		Timestamp: time.Unix(int64(blockInfo.Time()), 0),
	}

	txn.Height = receipt.BlockNumber.Uint64()
	txn.BlockInfo = &iwallet.BlockInfo{
		Height:    receipt.BlockNumber.Uint64(),
		BlockID:   iwallet.BlockID(receipt.BlockHash.Hex()),
		PrevBlock: iwallet.BlockID(blockInfo.ParentHash().Hex()),
		BlockTime: time.Unix(int64(blockInfo.Time()), 0),
	}

	return txn, nil
}

// GetTransaction - returns a eth txn for the specified hash
func (client *EthClient) GetTransaction(txid iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	txHash := common.HexToHash(EnsureCorrectPrefix(txid.String()))

	tx, _, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return nil, fmt.Errorf("get transaction by id %s failed, %v", txid, err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return nil, fmt.Errorf("wait receipt by id %s failed, %v", txid, err)
	}

	return client.buildTxFromTxAndReceipt(coinType, tx, receipt)
}

func (client *EthClient) EstimateFee(txsize int) (map[iwallet.FeeLevel]iwallet.EstimateFeeRes, error) {
	return nil, errors.New("not implemented")
}

func (client *EthClient) Broadcast(serializedTx []byte) error {
	return nil
}

func (client *EthClient) GetRecommendedContractVersion() (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	// Use pre-resolved address if available (SaaS mode / config-driven)
	if client.escrowAddr != nil {
		return struct {
			VersionName    string
			Status         uint8
			BugLevel       uint8
			Implementation common.Address
			DateAdded      *big.Int
		}{
			VersionName:    "config",
			Status:         1,
			Implementation: *client.escrowAddr,
		}, nil
	}
	if client.registry == nil {
		// Phase managed EVM v0.3.0 Sprint 1 D8 — promoted EVM L2 set
		// has no V1 ContractManager Registry, so registry is nil
		// here and any caller still routing through V1 (e.g.,
		// EVMChainOps strategies that haven't been migrated to
		// managed EVM adapter V2) gets a clear ErrV1ChainNotSupported
		// instead of a generic empty-call decode error. Routing
		// the order via managed EVM adapter V2 is the supported path.
		return struct {
			VersionName    string
			Status         uint8
			BugLevel       uint8
			Implementation common.Address
			DateAdded      *big.Int
		}{}, ErrV1ChainNotSupported
	}
	return client.registry.GetRecommendedVersion(nil, EscrowContractName)
}

func (client *EthClient) buildTxFromReceiptLog(coinType iwallet.CoinType, receipt *types.Log, blockInfo *types.Block) (*iwallet.Transaction, error) {
	coinInfo, _ := coinType.CoinInfo()
	// Check if token transfer event
	if !coinInfo.IsNative {
		contractAddress := coinInfo.ContractAddress(client.Testnet)
		erc20Token, err := contract.NewToken(common.HexToAddress(contractAddress), client)
		if err != nil {
			client.logger.Errorf("Initilaizing erc20 token failed: %s", err.Error())
			return nil, err
		}

		tokenTransfer, err := erc20Token.ParseTransfer(*receipt)
		if err == nil {
			return &iwallet.Transaction{
				ID: iwallet.TransactionID(EnsureCorrectPrefix(receipt.TxHash.Hex())),
				From: []iwallet.SpendInfo{
					{
						Address: iwallet.NewAddress(EnsureCorrectPrefix(tokenTransfer.From.Hex()), coinType),
						Amount:  iwallet.Amount(*tokenTransfer.Value),
					},
				},
				To: []iwallet.SpendInfo{
					{
						Address: iwallet.NewAddress(EnsureCorrectPrefix(tokenTransfer.To.Hex()), coinType),
						Amount:  iwallet.NewAmount(*tokenTransfer.Value),
					},
				},
				Value:     iwallet.NewAmount(*tokenTransfer.Value),
				Height:    receipt.BlockNumber,
				Timestamp: time.Unix(int64(blockInfo.Time()), 0),
			}, nil
		}
	}

	escrow, err := contract.NewEscrow(receipt.Address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow, %v", err)
	}

	if event, err := escrow.ParseExecuted(*receipt); err == nil {
		client.logger.Infof("[%s] Event Executed, tx: %s -> block: %d", client.CoinType, receipt.TxHash.Hex(), receipt.BlockNumber)
		if len(event.Destinations) != len(event.Amounts) {
			client.logger.Error("destinations and amounts sizes are not equal")
		}

		minLen := len(event.Destinations)
		if len(event.Amounts) < minLen {
			minLen = len(event.Amounts)
		}

		tos := []iwallet.SpendInfo{}
		total := iwallet.NewAmount(0)
		for i := 0; i < minLen; i++ {
			tos = append(tos, iwallet.SpendInfo{
				Address: iwallet.NewAddress(EnsureCorrectPrefix(event.Destinations[i].Hex()), coinType),
				Amount:  iwallet.NewAmount(event.Amounts[i]),
			})
			total = total.Add(iwallet.NewAmount(event.Amounts[i]))
		}

		return &iwallet.Transaction{
			ID: iwallet.TransactionID(EnsureCorrectPrefix(receipt.TxHash.Hex())),
			From: []iwallet.SpendInfo{
				{
					Address: iwallet.NewAddress(EnsureCorrectPrefix(common.Bytes2Hex(event.ScriptHash[:])), coinType),
					Amount:  total,
				},
			},
			To:        tos,
			Value:     total,
			Height:    receipt.BlockNumber,
			Timestamp: time.Unix(int64(blockInfo.Time()), 0),
		}, nil
	}

	if event, err := escrow.ParseFunded(*receipt); err == nil {
		client.logger.Infof("[%s]Event Funded, tx: %s, from: %s, value: %s -> block: %d", client.CoinType, receipt.TxHash.Hex(), event.From, event.Value, receipt.BlockNumber)

		return &iwallet.Transaction{
			ID: iwallet.TransactionID(EnsureCorrectPrefix(receipt.TxHash.Hex())),
			From: []iwallet.SpendInfo{
				{
					Address: iwallet.NewAddress(EnsureCorrectPrefix(event.From.Hex()), coinType),
					Amount:  iwallet.NewAmount(event.Value),
				},
			},
			To: []iwallet.SpendInfo{
				{
					Address: iwallet.NewAddress(EnsureCorrectPrefix(common.Bytes2Hex(event.ScriptHash[:])), coinType),
					Amount:  iwallet.NewAmount(event.Value),
				},
			},
			Value:     iwallet.NewAmount(event.Value),
			Height:    receipt.BlockNumber,
			Timestamp: time.Unix(int64(blockInfo.Time()), 0),
		}, nil
	}

	return nil, nil
}
