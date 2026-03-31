package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mobazha/ethereum-watcher/rpc"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"

	contract "github.com/mobazha/mobazha3.0/internal/chains/evm/contract"
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
	// EtherScanAPIKey is needed for all Eherscan requests
	EtherScanAPIKey = "REDACTED"
	// BscScanAPIKey is needed for all Bscscan requests
	BscScanAPIKey = "REDACTED"
	PolyScanAPIKey = "REDACTED"
	maxGasLimit    = 3000000

	EscrowContractName = "escrow"
)

// EthClient represents the eth client
type EthClient struct {
	CoinType iwallet.CoinType
	Testnet  bool

	*ethclient.Client
	rpcUrl string
	logger *logging.Logger

	rpc *rpc.EthBlockChainRPCWithRetry

	registry *contract.Registry

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

	// Skip Registry if escrow address is pre-resolved
	if client.escrowAddr != nil {
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
		return struct {
			VersionName    string
			Status         uint8
			BugLevel       uint8
			Implementation common.Address
			DateAdded      *big.Int
		}{}, errors.New("no registry available and no pre-resolved escrow address")
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
