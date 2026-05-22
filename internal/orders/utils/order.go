package utils

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func GetOrderEscrowInfo(orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, testnet bool) (iwallet.EscrowInfo, error) {
	escrowInfo := iwallet.EscrowInfo{
		Testnet: testnet,
	}

	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("payment_sent missing settlement spec")
	}
	method := spec.GetMethod()
	if payment.MethodIsDirect(method) {
		return iwallet.EscrowInfo{}, nil
	}
	switch method {
	case pb.PaymentSent_CANCELABLE:
		escrowInfo.RequiredSignatures = 1
	case pb.PaymentSent_MODERATED:
		escrowInfo.RequiredSignatures = 2
	default:
		return iwallet.EscrowInfo{}, nil
	}

	escrowInfo.ContractAddress = paymentSent.ContractAddress

	coinType := iwallet.CoinType(paymentSent.Coin)
	if !iwallet.IsValidCoinType(coinType) {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type: %v", coinType)
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("failed to get coin info: %w", err)
	}
	escrowInfo.CoinType = coinType

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode chaincode: %w", err)
	}
	escrowInfo.UniqueId = [20]byte(chainCode[:20])
	escrowInfo.Amount = iwallet.NewAmount(paymentSent.Amount).Uint64()
	escrowInfo.UnlockHours = uint64(paymentSent.EscrowTimeoutHours)

	buyerID := orderOpen.BuyerID
	vendorID := orderOpen.Listings[0].Listing.VendorID
	if coinInfo.Chain == iwallet.ChainSolana {
		payer, err := solana.PublicKeyFromBase58(paymentSent.PayerAddress)
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode payer address: %w", err)
		}
		escrowInfo.PayerAddress = payer.String()
		escrowInfo.BuyerAddress = solana.PublicKeyFromBytes(buyerID.Pubkeys.Solana).String()
		escrowInfo.SellerAddress = solana.PublicKeyFromBytes(vendorID.Pubkeys.Solana).String()
		escrowInfo.ModeratorAddress = paymentSent.ModeratorAddress
	} else if coinInfo.IsEthTypeChain() {
		payer := common.HexToAddress(paymentSent.PayerAddress)
		escrowInfo.PayerAddress = payer.String()
		buyer, err := iwallet.PubKeyBytesToEthAddress(buyerID.Pubkeys.Eth)
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode buyer address: %w", err)
		}
		escrowInfo.BuyerAddress = buyer.String()
		seller, err := iwallet.PubKeyBytesToEthAddress(vendorID.Pubkeys.Eth)
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode seller address: %w", err)
		}
		escrowInfo.SellerAddress = seller.String()
		escrowInfo.ModeratorAddress = paymentSent.ModeratorAddress
	} else {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type: %v", escrowInfo.CoinType)
	}

	return escrowInfo, nil
}
