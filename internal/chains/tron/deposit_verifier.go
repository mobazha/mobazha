package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/payment"
)

// Funded(bytes32 indexed scriptHash, address indexed from, uint256 value)
var fundedEventTopic = hex.EncodeToString(crypto.Keccak256([]byte("Funded(bytes32,address,uint256)")))

// DepositVerification holds parameters for TRON escrow deposit verification.
type DepositVerification struct {
	TxHash           string
	EscrowHash       [32]byte // keccak256(redeemScript)
	ExpectedContract string   // hex address (41-prefixed) of the escrow contract
	MinAmount        *big.Int
}

// VerifyDeposit checks that a TRON transaction:
//  1. Has a successful receipt (not reverted)
//  2. Emits a Funded event with the matching escrow hash
//  3. Has sufficient deposit amount
//  4. Has enough confirmations (19 solid blocks)
func VerifyDeposit(ctx context.Context, client *TronClient, params DepositVerification) error {
	info, err := client.GetTransactionInfo(ctx, params.TxHash)
	if err != nil {
		return fmt.Errorf("get transaction info: %w", err)
	}

	if info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS" {
		return payment.ErrDepositReverted
	}
	if info.Result == "FAILED" {
		return payment.ErrDepositReverted
	}

	if !verifyFundedEvent(info, params) {
		return payment.ErrDepositEventNotFound
	}

	nowBlock, err := client.GetNowBlock(ctx)
	if err != nil {
		return fmt.Errorf("get current block: %w", err)
	}
	confirmations := nowBlock.BlockHeader.RawData.Number - info.BlockNumber
	if confirmations < solidBlockConfirms {
		return fmt.Errorf("insufficient confirmations: %d < %d", confirmations, solidBlockConfirms)
	}

	return nil
}

// verifyFundedEvent scans the transaction logs for a matching Funded event.
func verifyFundedEvent(info *TronTransactionInfo, params DepositVerification) bool {
	expectedContract := strings.ToLower(strings.TrimPrefix(params.ExpectedContract, "0x"))
	escrowHashHex := hex.EncodeToString(params.EscrowHash[:])

	for _, lg := range info.Log {
		logAddr := strings.ToLower(strings.TrimPrefix(lg.Address, "0x"))
		if logAddr != expectedContract {
			continue
		}
		// Topics: [0]=event sig, [1]=scriptHash (indexed), [2]=from (indexed)
		if len(lg.Topics) < 2 {
			continue
		}
		if strings.ToLower(lg.Topics[0]) != fundedEventTopic {
			continue
		}
		if strings.ToLower(lg.Topics[1]) != escrowHashHex {
			continue
		}

		if params.MinAmount != nil && params.MinAmount.Sign() > 0 {
			dataBytes, err := hex.DecodeString(strings.TrimPrefix(lg.Data, "0x"))
			if err != nil || len(dataBytes) < 32 {
				continue
			}
			amount := new(big.Int).SetBytes(dataBytes[:32])
			if amount.Cmp(params.MinAmount) < 0 {
				continue
			}
		}
		return true
	}
	return false
}
