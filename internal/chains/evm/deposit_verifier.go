package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/payment"
)

// EVMReceiptFetcher abstracts the go-ethereum ethclient methods needed for
// deposit verification. EthClient satisfies this interface through its
// embedded *ethclient.Client.
type EVMReceiptFetcher interface {
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error)
}

var _ EVMReceiptFetcher = (*EthClient)(nil)

// DepositVerification holds the parameters for verifying an EVM escrow deposit.
type DepositVerification struct {
	TxHash       string
	EscrowHash   common.Hash    // keccak256(redeemScript) — identifies this escrow
	ExpectedAddr common.Address // escrow contract address
	MinAmount    *big.Int       // minimum expected funding amount
}

// Funded(bytes32 indexed scriptHash, address indexed from, uint256 value)
var fundedEventTopic = crypto.Keccak256Hash([]byte("Funded(bytes32,address,uint256)"))

// VerifyDeposit checks that a transaction:
//  1. Has a successful receipt (not reverted)
//  2. Targets or involves the escrow contract
//  3. Emits a Funded event with the matching escrow hash and sufficient amount
func VerifyDeposit(ctx context.Context, fetcher EVMReceiptFetcher, params DepositVerification) error {
	txHash := common.HexToHash(params.TxHash)

	receipt, err := fetcher.TransactionReceipt(ctx, txHash)
	if err != nil {
		return fmt.Errorf("get receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return payment.ErrDepositReverted
	}

	tx, _, err := fetcher.TransactionByHash(ctx, txHash)
	if err != nil {
		return fmt.Errorf("get tx: %w", err)
	}

	// For native ETH deposits, tx.To() is the escrow contract. For ERC20
	// deposits, tx.To() is the token contract; verify via receipt logs instead.
	if tx.To() == nil || *tx.To() != params.ExpectedAddr {
		if !verifyEscrowInLogs(receipt, params.ExpectedAddr) {
			return payment.ErrDepositTargetInvalid
		}
	}

	if !verifyFundedEvent(receipt, params) {
		return payment.ErrDepositEventNotFound
	}

	return nil
}

// verifyFundedEvent scans the receipt logs for a Funded event from the escrow
// contract that matches the expected scriptHash and has sufficient value.
func verifyFundedEvent(receipt *types.Receipt, params DepositVerification) bool {
	for _, lg := range receipt.Logs {
		if lg.Address != params.ExpectedAddr {
			continue
		}
		// Topics: [0]=event sig, [1]=scriptHash (indexed), [2]=from (indexed)
		if len(lg.Topics) < 2 || lg.Topics[0] != fundedEventTopic {
			continue
		}
		if lg.Topics[1] != params.EscrowHash {
			continue
		}
		amount := new(big.Int).SetBytes(lg.Data)
		if amount.Cmp(params.MinAmount) >= 0 {
			return true
		}
	}
	return false
}

// verifyEscrowInLogs confirms the escrow contract address appears in the
// receipt's event logs (for ERC20 deposits where tx.To is the token contract).
func verifyEscrowInLogs(receipt *types.Receipt, escrowAddr common.Address) bool {
	for _, lg := range receipt.Logs {
		if lg.Address == escrowAddr {
			return true
		}
	}
	return false
}
