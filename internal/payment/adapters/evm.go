package adapters

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	eth "github.com/mobazha/mobazha3.0/internal/multiwallet/coins/eth"
	evmpayment "github.com/mobazha/mobazha3.0/internal/payment/evm"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// EVMChainOps implements the ChainOps interface for EVM chains
// (ETH, BSC, MATIC, BASE, CFX). Only the truly chain-specific
// operations are defined here; all shared orchestration lives
// in ClientSignedAdapter (client_signed.go).
//
// Dependencies are injected at construction time — no direct
// reference to MobazhaNode.
type EVMChainOps struct {
	Keys            contracts.KeyProvider
	Multiwallet     contracts.WalletOperator
	BuildReleaseTxn evmpayment.BuildReleaseTransactionFn
	OnAutoConfirm   func(event *events.CancelablePaymentReady, chainType string)
}

// AutoConfirm delegates to OnAutoConfirm callback with chain type.
func (o *EVMChainOps) AutoConfirm(event *events.CancelablePaymentReady) error {
	coinType := iwallet.CoinType(event.Coin)
	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		return fmt.Errorf("unknown coin %s: %w", event.Coin, err)
	}
	o.OnAutoConfirm(event, string(coinInfo.Chain))
	return nil
}

// SignEscrowRelease signs the escrow release using EVM master key.
func (o *EVMChainOps) SignEscrowRelease(params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	ethKey, err := o.Keys.EVMMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM master key: %w", err)
	}
	sigs, err := evmpayment.SignEscrowRelease(params.Transaction.To, params.Script, ethKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign escrow release (evm): %w", err)
	}
	return sigs, nil
}

// BuildCancelableRelease builds cancelable escrow release instructions.
func (o *EVMChainOps) BuildCancelableRelease(order *models.Order, initiator, receiver string) (any, error) {
	ethKey, err := o.Keys.EVMMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM master key: %w", err)
	}
	return evmpayment.BuildCancelableEscrowReleaseInstructions(order, o.Multiwallet, ethKey, initiator, receiver)
}

// BuildCompleteEscrow builds complete escrow instructions.
func (o *EVMChainOps) BuildCompleteEscrow(order *models.Order, initiator string, release *pb.EscrowRelease) (any, error) {
	ethKey, err := o.Keys.EVMMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM master key: %w", err)
	}
	return evmpayment.BuildCompleteEscrowInstructions(order, o.Multiwallet, ethKey, initiator, release)
}

// BuildDisputeRelease builds dispute release instructions.
func (o *EVMChainOps) BuildDisputeRelease(order *models.Order, initiator string) (any, error) {
	ethKey, err := o.Keys.EVMMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM master key: %w", err)
	}
	return evmpayment.BuildDisputeReleaseInstructions(order, o.Multiwallet, ethKey, initiator, o.BuildReleaseTxn)
}

// VerifyDeposit checks the buyer's EVM deposit on-chain:
// receipt status, Funded event, escrow hash match, and minimum amount.
func (o *EVMChainOps) VerifyDeposit(ctx context.Context, params payment.DepositVerifyParams) error {
	coinInfo, err := iwallet.CoinInfoFromCoinType(params.CoinType)
	if err != nil || !coinInfo.IsEthTypeChain() {
		return nil
	}

	wallet, ok := o.Multiwallet.WalletForChain(coinInfo.Chain)
	if !ok {
		return nil
	}

	ethWallet, ok := wallet.(*eth.ETHWallet)
	if !ok || ethWallet.ChainClient == nil {
		return nil
	}

	fetcher, ok := ethWallet.ChainClient.(eth.EVMReceiptFetcher)
	if !ok {
		return nil
	}

	if params.Script == "" {
		return nil
	}
	scriptBytes, err := hex.DecodeString(params.Script)
	if err != nil {
		return fmt.Errorf("decode script hex: %w", err)
	}

	script, err := eth.DeserializeEthScript(scriptBytes)
	if err != nil {
		return fmt.Errorf("deserialize escrow script: %w", err)
	}

	escrowHash, _, err := eth.CalculateRedeemScriptHash(script)
	if err != nil {
		return fmt.Errorf("calculate escrow hash: %w", err)
	}

	amount := new(big.Int)
	if params.OrderAmount != "" {
		amount.SetString(params.OrderAmount, 10)
	}

	expectedAddr := common.HexToAddress(params.ContractAddr)

	return eth.VerifyDeposit(ctx, fetcher, eth.DepositVerification{
		TxHash:       params.TxHash,
		EscrowHash:   escrowHash,
		ExpectedAddr: expectedAddr,
		MinAmount:    amount,
	})
}
