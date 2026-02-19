package adapters

import (
	"fmt"

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
