package tronpayment

import (
	"encoding/hex"
	"errors"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethWal "github.com/mobazha/mobazha/internal/chains/evm"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// BuildReleaseTransactionFn matches the signature used for dispute release.
type BuildReleaseTransactionFn func(releaseInfo *pb.DisputeClose_ModeratedEscrowRelease, paymentSent *pb.PaymentSent) (iwallet.Transaction, error)

func signWithEthereumKey(message []byte, key *btcec.PrivateKey) ([]byte, error) {
	ecdsaKey, err := crypto.ToECDSA(key.Serialize())
	if err != nil {
		return nil, fmt.Errorf("failed to convert secp256k1 key for ethereum signing: %w", err)
	}
	sig, err := crypto.Sign(message, ecdsaKey)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// BuildEscrowReleaseParams builds TRON-specific escrow release parameters.
// TRON uses the same Escrow.sol contract as EVM, so the signature message
// format is identical. The key difference is that TRONMasterKey is used
// for signing instead of EVMMasterKey.
func BuildEscrowReleaseParams(tos []iwallet.SpendInfo, redeemScript []byte) (receivers [][]byte, amounts []uint64, message []byte, err error) {
	receivers = make([][]byte, 0, len(tos))
	amounts = make([]uint64, 0, len(tos))
	receiversAddresses := make([]common.Address, 0, len(tos))

	for _, to := range tos {
		amounts = append(amounts, uint64(to.Amount.Int64()))
		address := common.HexToAddress(to.Address.String())
		receiversAddresses = append(receiversAddresses, address)
		receivers = append(receivers, address.Bytes())
	}

	// Reuse EVM signature message format — the Escrow.sol contract is identical
	message, err = ethWal.BuildEthSignatureMessage(redeemScript, receiversAddresses, amounts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build signature message: %w", err)
	}

	return receivers, amounts, message, nil
}

// SignEscrowRelease signs an escrow release message using the TRON (secp256k1) key.
func SignEscrowRelease(tos []iwallet.SpendInfo, redeemScript []byte, tronMasterKey *btcec.PrivateKey) ([]iwallet.EscrowSignature, error) {
	_, _, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		return nil, err
	}
	sig, err := signWithEthereumKey(message, tronMasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign escrow release: %w", err)
	}
	return []iwallet.EscrowSignature{{Index: 0, Signature: sig}}, nil
}

// BuildCancelableEscrowReleaseInstructions builds a cancelable (buyer-only)
// escrow release for TRON. The pattern mirrors EVM but uses TRONMasterKey.
func BuildCancelableEscrowReleaseInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	tronMasterKey *btcec.PrivateKey,
	initiatorAddress string,
	receiverAddress string,
) (any, error) {
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, err
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	wallet, err := walletOperator.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, err
	}
	escrowProcessor, ok := wallet.(iwallet.EscrowProcessor)
	if !ok {
		return nil, errors.New("wallet does not support escrow processor")
	}

	amounts := []uint64{iwallet.NewAmount(paymentSent.Amount).Uint64()}
	receiverPubkey := common.HexToAddress(receiverAddress).Bytes()

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to decode script: %w", err)
	}

	currentUserPubkey := tronMasterKey.PubKey().SerializeCompressed()
	message, err := ethWal.BuildEthSignatureMessage(script, []common.Address{common.HexToAddress(receiverAddress)}, amounts)
	if err != nil {
		return nil, fmt.Errorf("failed to build signature message: %w", err)
	}
	currentUserSignature, err := signWithEthereumKey(message, tronMasterKey)
	if err != nil {
		return nil, fmt.Errorf("error signing release: %w", err)
	}

	escrowInfo, err := utils.GetOrderEscrowInfo(orderOpen, paymentSent, wallet.IsTestnet())
	if err != nil {
		return nil, err
	}

	releaseParams := iwallet.ReleaseEscrowParams{
		InitiatorAddress: initiatorAddress,
		Message:          message,
		PublicKeys:       [][]byte{currentUserPubkey},
		Signatures:       [][]byte{currentUserSignature},
		Amounts:          amounts,
		Recipients:       [][]byte{receiverPubkey},
	}

	return escrowProcessor.BuildReleaseEscrowInstructions(escrowInfo, releaseParams)
}

// BuildCompleteEscrowInstructions builds complete escrow instructions (buyer + vendor).
func BuildCompleteEscrowInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	tronMasterKey *btcec.PrivateKey,
	initiatorAddress string,
	releaseInfo *pb.EscrowRelease,
) (any, error) {
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to get order open message: %w", err)
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to get payment sent message: %w", err)
	}
	coinType := iwallet.CoinType(paymentSent.Coin)
	wallet, err := walletOperator.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, err
	}
	escrowProcessor, ok := wallet.(iwallet.EscrowProcessor)
	if !ok {
		return nil, errors.New("wallet does not support escrow processor")
	}

	vendorID, err := order.VendorID()
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor ID: %w", err)
	}

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(releaseInfo.ToAddress, coinType),
		Amount:  iwallet.NewAmount(releaseInfo.ToAmount),
	}}
	if iwallet.NewAmount(releaseInfo.PlatformAmount).Cmp(iwallet.NewAmount(0)) > 0 {
		tos = append(tos, iwallet.SpendInfo{
			Address: iwallet.NewAddress(releaseInfo.PlatformAddress, coinType),
			Amount:  iwallet.NewAmount(releaseInfo.PlatformAmount),
		})
	}

	redeemScript, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to decode script: %w", err)
	}

	receivers, amounts, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		return nil, err
	}

	buyerPubKeyBytes := tronMasterKey.PubKey().SerializeCompressed()
	buyerSig, err := signWithEthereumKey(message, tronMasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	vendorPubKeyBytes := vendorID.Pubkeys.Eth
	vendorSig := releaseInfo.EscrowSignatures[0].Signature

	escrowInfo, err := utils.GetOrderEscrowInfo(orderOpen, paymentSent, wallet.IsTestnet())
	if err != nil {
		return nil, fmt.Errorf("failed to get order escrow info: %w", err)
	}
	params := iwallet.ReleaseEscrowParams{
		InitiatorAddress: initiatorAddress,
		Message:          message,
		PublicKeys:       [][]byte{buyerPubKeyBytes, vendorPubKeyBytes},
		Signatures:       [][]byte{buyerSig, vendorSig},
		Recipients:       receivers,
		Amounts:          amounts,
	}
	return escrowProcessor.BuildReleaseEscrowInstructions(escrowInfo, params)
}

// BuildDisputeReleaseInstructions builds dispute release instructions.
func BuildDisputeReleaseInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	tronMasterKey *btcec.PrivateKey,
	initiatorAddress string,
	buildReleaseTransaction BuildReleaseTransactionFn,
) (any, error) {
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, err
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}
	disputeClose, err := order.DisputeClosedMessage()
	if err != nil {
		return nil, err
	}

	txn, err := buildReleaseTransaction(disputeClose.ReleaseInfo, paymentSent)
	if err != nil {
		return nil, err
	}

	wallet, err := walletOperator.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, fmt.Errorf("cannot validate order. coin not supported by moderator:%s, %w", paymentSent.Coin, err)
	}
	escrowProcessor, ok := wallet.(iwallet.EscrowProcessor)
	if !ok {
		return nil, errors.New("wallet does not support escrow")
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payment script: %w", err)
	}
	receivers, amounts, message, err := BuildEscrowReleaseParams(txn.To, script)
	if err != nil {
		return nil, err
	}

	ownKeyBytes := tronMasterKey.PubKey().SerializeCompressed()
	ownSig, err := signWithEthereumKey(message, tronMasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate moderator signature: %w", err)
	}
	moderatorPubKeyBytes := common.HexToAddress(paymentSent.ModeratorAddress).Bytes()
	modSig := disputeClose.ReleaseInfo.EscrowSignatures[0].Signature

	escrowInfo, err := utils.GetOrderEscrowInfo(orderOpen, paymentSent, wallet.IsTestnet())
	if err != nil {
		return nil, fmt.Errorf("failed to get order escrow info: %w", err)
	}
	params := iwallet.ReleaseEscrowParams{
		InitiatorAddress: initiatorAddress,
		Message:          message,
		PublicKeys:       [][]byte{ownKeyBytes, moderatorPubKeyBytes},
		Signatures:       [][]byte{ownSig, modSig},
		Recipients:       receivers,
		Amounts:          amounts,
	}
	return escrowProcessor.BuildReleaseEscrowInstructions(escrowInfo, params)
}
