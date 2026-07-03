package adapters

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	evm "github.com/mobazha/mobazha/internal/chains/evm"
	tronchain "github.com/mobazha/mobazha/internal/chains/tron"
	"github.com/mobazha/mobazha/internal/orders/utils"
	tronpayment "github.com/mobazha/mobazha/internal/payment/tron"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// TRONChainOps implements the ChainOps interface for TRON.
// Only the truly chain-specific operations are defined here;
// all shared orchestration lives in ClientSignedAdapter.
//
// Dependencies are injected at construction time — no direct
// reference to MobazhaNode.
type TRONChainOps struct {
	Keys            contracts.KeyProvider
	Multiwallet     contracts.WalletOperator
	BuildReleaseTxn tronpayment.BuildReleaseTransactionFn
	OnAutoConfirm   func(event *events.CancelablePaymentReady)
	TronClient      *tronchain.TronClient
	NodeID          string
}

// AutoConfirm delegates to OnAutoConfirm callback for TRON relay handling.
func (o *TRONChainOps) AutoConfirm(event *events.CancelablePaymentReady) error {
	if o.OnAutoConfirm != nil {
		o.OnAutoConfirm(event)
		return nil
	}
	log.Infof("TRON CANCELABLE payment for order %s - no relay handler configured", event.OrderID)
	return nil
}

// SignEscrowRelease signs the escrow release using TRON master key.
func (o *TRONChainOps) SignEscrowRelease(params payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	tronKey, err := o.Keys.TRONMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON master key: %w", err)
	}
	sigs, err := tronpayment.SignEscrowRelease(params.Transaction.To, params.Script, tronKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign escrow release (tron): %w", err)
	}
	return sigs, nil
}

// BuildCancelableRelease builds cancelable escrow release instructions.
func (o *TRONChainOps) BuildCancelableRelease(order *models.Order, initiator, receiver string) (any, error) {
	tronKey, err := o.Keys.TRONMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON master key: %w", err)
	}
	return tronpayment.BuildCancelableEscrowReleaseInstructions(order, o.Multiwallet, tronKey, initiator, receiver)
}

// BuildCompleteEscrow builds complete escrow instructions.
func (o *TRONChainOps) BuildCompleteEscrow(order *models.Order, initiator string, release *pb.EscrowRelease) (any, error) {
	tronKey, err := o.Keys.TRONMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON master key: %w", err)
	}
	return tronpayment.BuildCompleteEscrowInstructions(order, o.Multiwallet, tronKey, initiator, release)
}

// BuildDisputeRelease builds dispute release instructions.
func (o *TRONChainOps) BuildDisputeRelease(order *models.Order, initiator string) (any, error) {
	tronKey, err := o.Keys.TRONMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON master key: %w", err)
	}
	return tronpayment.BuildDisputeReleaseInstructions(order, o.Multiwallet, tronKey, initiator, o.BuildReleaseTxn)
}

// VerifyDeposit checks the buyer's TRON deposit on-chain.
func (o *TRONChainOps) VerifyDeposit(ctx context.Context, params payment.DepositVerifyParams) error {
	coinInfo, err := payment.SettlementCoinInfoForCoin(params.CoinType)
	if err != nil || coinInfo.Chain != iwallet.ChainTRON {
		return nil
	}

	if o.TronClient == nil {
		return nil
	}

	if params.Script == "" {
		return nil
	}

	scriptBytes, err := hex.DecodeString(params.Script)
	if err != nil {
		return fmt.Errorf("decode script hex: %w", err)
	}

	script, err := evm.DeserializeEthScript(scriptBytes)
	if err != nil {
		return fmt.Errorf("deserialize escrow script: %w", err)
	}

	escrowHash, _, err := evm.CalculateRedeemScriptHash(script)
	if err != nil {
		return fmt.Errorf("calculate escrow hash: %w", err)
	}

	amount := new(big.Int)
	if params.PaymentAmount != "" {
		amount.SetString(params.PaymentAmount, 10)
	}

	contractHex, err := tronchain.Base58ToHex(params.ContractAddr)
	if err != nil {
		contractHex = params.ContractAddr
	}

	return tronchain.VerifyDeposit(ctx, o.TronClient, tronchain.DepositVerification{
		TxHash:           params.TxHash,
		EscrowHash:       escrowHash,
		ExpectedContract: contractHex,
		MinAmount:        amount,
	})
}

// ValidatePaymentMessage validates TRON PaymentSent against OrderOpen.
func (o *TRONChainOps) ValidatePaymentMessage(params payment.PaymentMessageParams) error {
	orderOpen, paymentSent, err := assertPaymentMessageParams(params)
	if err != nil {
		return err
	}

	if orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
		return nil
	}

	if err := validatePaymentMessageAmount(params); err != nil {
		return err
	}

	coinType := iwallet.CoinType(paymentSent.Coin)
	coinInfo, err := payment.SettlementCoinInfoForCoin(coinType)
	if err != nil {
		return fmt.Errorf("unknown coin %s: %w", paymentSent.Coin, err)
	}
	wallet, ok := o.Multiwallet.WalletForChain(coinInfo.Chain)
	if !ok {
		return fmt.Errorf("no wallet for chain %s", coinInfo.Chain)
	}

	escrowInfo, err := utils.GetOrderEscrowInfo(orderOpen, paymentSent, wallet.IsTestnet())
	if err != nil {
		return fmt.Errorf("get escrow info: %w", err)
	}
	escrowWallet, ok := wallet.(iwallet.EscrowProcessor)
	if !ok {
		return fmt.Errorf("wallet does not support escrow processing")
	}
	address, err := escrowWallet.CreateEscrowAddress(escrowInfo)
	if err != nil {
		return fmt.Errorf("create escrow address: %w", err)
	}
	if paymentSent.ToAddress != address.String() {
		return fmt.Errorf("invalid escrow payment address")
	}

	return nil
}

// VerifyPreRelease verifies the deposit before escrow release.
func (o *TRONChainOps) VerifyPreRelease(ctx context.Context, params payment.PreReleaseParams) error {
	if params.TxHash == "" {
		return nil
	}
	return o.VerifyDeposit(ctx, payment.DepositVerifyParams{
		CoinType:     params.CoinType,
		TxHash:       params.TxHash,
		Script:       params.Script,
		ContractAddr: params.ContractAddr,
	})
}
