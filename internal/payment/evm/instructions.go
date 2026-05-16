package evmpayment

import (
	"encoding/hex"
	"errors"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

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

// BuildEscrowReleaseParams builds EVM-specific escrow release parameters
// (receivers, amounts, signature message) from the given spend info and redeem script.
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

	message, err = ethWal.BuildEthSignatureMessage(redeemScript, receiversAddresses, amounts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build eth signature message: %w", err)
	}

	return receivers, amounts, message, nil
}

// SignEscrowRelease signs an escrow release message using the EVM (secp256k1) key.
// This encapsulates the EVM signing pattern used across CompleteOrder, CloseDispute, and Refund.
func SignEscrowRelease(tos []iwallet.SpendInfo, redeemScript []byte, ethMasterKey *btcec.PrivateKey) ([]iwallet.EscrowSignature, error) {
	_, _, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		return nil, err
	}
	sig, err := signWithEthereumKey(message, ethMasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign escrow release: %w", err)
	}
	return []iwallet.EscrowSignature{{Index: 0, Signature: sig}}, nil
}

func BuildCancelableEscrowReleaseInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	ethMasterKey *btcec.PrivateKey,
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

	currentUserPubkey := ethMasterKey.PubKey().SerializeCompressed()
	message, err := ethWal.BuildEthSignatureMessage(script, []common.Address{common.HexToAddress(receiverAddress)}, amounts)
	if err != nil {
		return nil, fmt.Errorf("failed to build eth signature message: %w", err)
	}
	currentUserSignature, err := signWithEthereumKey(message, ethMasterKey)
	if err != nil {
		return nil, fmt.Errorf("error signing in createmultisig : %v", err)
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

func BuildCompleteEscrowInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	ethMasterKey *btcec.PrivateKey,
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

	buyerPubKeyBytes := ethMasterKey.PubKey().SerializeCompressed()
	buyerSig, err := signWithEthereumKey(message, ethMasterKey)
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

func BuildDisputeReleaseInstructions(
	order *models.Order,
	walletOperator contracts.WalletOperator,
	ethMasterKey *btcec.PrivateKey,
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

	ownKeyBytes := ethMasterKey.PubKey().SerializeCompressed()
	ownSig, err := signWithEthereumKey(message, ethMasterKey)
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
